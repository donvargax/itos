package push

import (
	"context"
	"fmt"
	"slices"
	"strconv"
)

// replyMutation and resolveMutation are the two gh api graphql mutations that
// answer a review thread, each on a single line like reviewQuery.
const (
	replyMutation = `mutation($threadId: ID!, $body: String!) { addPullRequestReviewThreadReply(input: ` +
		`{pullRequestReviewThreadId: $threadId, body: $body}) { comment { url } } }`
	resolveMutation = `mutation($threadId: ID!) { resolveReviewThread(input: {threadId: $threadId}) { thread { isResolved } } }`
)

// PullRef is --json's pull_request of push threads and push thread.
type PullRef struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}

// ThreadInfo is one of --json's threads: an open review thread, its place (no
// line when outdated), its first comment's author and that comment's first
// line.
type ThreadInfo struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Author  string `json:"author"`
	Comment string `json:"comment"`
}

// ThreadResult is --json's thread of push thread: whether the reply was posted
// and whether the thread was resolved.
type ThreadResult struct {
	ID       string `json:"id"`
	Replied  bool   `json:"replied"`
	Resolved bool   `json:"resolved"`
}

// reviewThreads is push threads, which lists the open review threads of the
// branch's pull request, and push thread, which replies to one and resolves it
// (slice 191). Nothing is fetched, rebased or pushed. They refuse, before any
// gh call, outside push.mode pr, on a detached HEAD and on main (asked as push
// asks, defaultBranch); then gh finds the open pull request (no-pull-request
// when there is none) and its threads (ask, which also gives their node ids). An
// id that is none of them is refused (no-such-thread) before anything is posted;
// else the reply is posted and then the resolve, two calls, so a reply that
// fails leaves the thread open. threads makes ghFailed say no more than the
// failure, since the branch is not pushed by it.
func (r *pushRun) reviewThreads(ctx context.Context, q Query) Answer {
	r.threads = true
	v := r.ports
	if !r.prMode(ctx) {
		return r.report(policy, "threads-without-pr-mode",
			"itos push: review threads are an item's pull request's, and this config does not set push.mode: pr; nothing was posted.")
	}
	branch := v.Branch(ctx)
	if branch == "" {
		return r.report(policy, "detached", "itos push: HEAD is not on a branch; check out the item's branch first. Nothing was posted.")
	}
	remote, _, _ := v.Upstream(ctx, branch)
	r.remote = remote
	main, a, done := r.defaultBranch(ctx)
	r.remote = ""
	if done {
		return a
	}
	if branch == main {
		return r.report(policy, "threads-on-main",
			fmt.Sprintf("itos push: HEAD is on %s, which has no pull request; run it from the item's branch. Nothing was posted.", main))
	}
	repository := v.GitHubRepository(ctx, r.cfg, remote)
	if repository == "" {
		return r.report(missing, "no-github-repository",
			fmt.Sprintf("itos push: %s names no GitHub repository to find the pull request of %s in; nothing was posted.", remote, branch))
	}
	open, a, failed := r.openPulls(ctx, repository, branch)
	if failed {
		return a
	}
	if len(open) == 0 {
		return r.report(policy, "no-pull-request",
			fmt.Sprintf("itos push: %s has no open pull request; itos push opens it. Nothing was posted.", branch))
	}
	pull := &PullRef{URL: open[0].URL, Number: open[0].Number}
	state, a, failed := r.ask(ctx, repository, pull.Number)
	if failed {
		return a
	}
	if q.Thread == "" {
		return r.listThreads(pull, state)
	}
	if !slices.Contains(state.ids, q.Thread) {
		a := r.report(policy, "no-such-thread",
			fmt.Sprintf("itos push: %s is not a review thread of %s; itos push threads lists its open ones.", q.Thread, pull.URL))
		a.Pull = pull
		return a
	}
	return r.answerThread(ctx, q, repository, pull)
}

// listThreads is push threads' end: the open threads, one line each.
func (r pushRun) listThreads(pull *PullRef, state pullState) Answer {
	threads := []ThreadInfo{}
	lines := []string{pull.URL + " has no unresolved review threads."}
	if len(state.unresolved) > 0 {
		lines = []string{fmt.Sprintf("%s has %d unresolved review threads:", pull.URL, len(state.unresolved))}
	}
	for _, t := range state.unresolved {
		where := t.path
		if t.line > 0 {
			where += ":" + strconv.Itoa(t.line)
		}
		threads = append(threads, ThreadInfo{ID: t.id, Path: t.path, Line: t.line, Author: t.author, Comment: t.first})
		lines = append(lines, fmt.Sprintf("  %s %s %s: %s", t.id, where, t.author, t.first))
	}
	a := r.report(0, "threads-listed", lines...)
	a.Pull, a.Threads = pull, threads
	return a
}

// answerThread is push thread's end, once the thread is known to be the pull
// request's: the reply when there is text, then the resolve when asked. A call
// that fails ends it with what was done said before the failure.
func (r pushRun) answerThread(ctx context.Context, q Query, repository string, pull *PullRef) Answer {
	result := &ThreadResult{ID: q.Thread}
	var lines []string
	end := func(a Answer) Answer {
		a.Pull, a.Thread = pull, result
		a.Lines = append(lines, a.Lines...)
		return a
	}
	if q.Reply != "" {
		if _, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+replyMutation, "-f", "threadId="+q.Thread, "-f", "body="+q.Reply); failed {
			return end(a)
		}
		result.Replied = true
		lines = append(lines, fmt.Sprintf("Replied to %s on %s.", q.Thread, pull.URL))
	}
	if q.Resolve {
		if _, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+resolveMutation, "-f", "threadId="+q.Thread); failed {
			return end(a)
		}
		result.Resolved = true
		lines = append(lines, fmt.Sprintf("Resolved %s on %s.", q.Thread, pull.URL))
	}
	return end(r.report(0, "thread-answered"))
}
