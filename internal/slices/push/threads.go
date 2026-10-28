package push

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The gh api graphql mutations that answer a review thread, and the two
// questions push threads and push thread put (slice 201): nodeQuery asks for a
// node by its id, a review thread's, with where it is, its repository and pull
// request and its comments; pullThreadsQuery for a pull request's review
// threads, with its number and address. Each is on a single line like
// reviewQuery.
const (
	replyMutation = `mutation($threadId: ID!, $body: String!) { addPullRequestReviewThreadReply(input: ` +
		`{pullRequestReviewThreadId: $threadId, body: $body}) { comment { url } } }`
	resolveMutation = `mutation($threadId: ID!) { resolveReviewThread(input: {threadId: $threadId}) { thread { isResolved } } }`
	nodeQuery       = `query($id: ID!) { node(id: $id) { ... on PullRequestReviewThread { id isResolved path line ` +
		`repository { nameWithOwner } pullRequest { number url } comments(first: 100) { nodes { author { login } body } } } } }`
	pullThreadsQuery = `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) { ` +
		`pullRequest(number: $number) { number url reviewThreads(first: 100) { nodes { id isResolved path line ` +
		`comments(first: 1) { nodes { author { login } body } } } } } } }`
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

// ThreadResult is --json's thread of push thread answering a thread: whether
// the reply was posted and whether the thread was resolved.
type ThreadResult struct {
	ID       string `json:"id"`
	Replied  bool   `json:"replied"`
	Resolved bool   `json:"resolved"`
}

// ThreadView is --json's thread of push thread showing one: its place (no line
// when outdated), whether it is resolved and its comments, oldest first.
type ThreadView struct {
	ID       string        `json:"id"`
	Path     string        `json:"path"`
	Line     int           `json:"line,omitempty"`
	Resolved bool          `json:"resolved"`
	Comments []CommentView `json:"comments"`
}

// CommentView is one of a ThreadView's comments: its author's login, ghost for
// an author GitHub no longer has, and its body as GitHub gives it.
type CommentView struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// threadNode is a review thread as GraphQL gives it.
type threadNode struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	Path       string `json:"path"`
	Line       *int   `json:"line"`
	Comments   struct {
		Nodes []struct {
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

// nodeThread is the node push thread asks for: a thread with its repository
// and pull request. A node that is no review thread has no id.
type nodeThread struct {
	threadNode
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	PullRequest PullRef `json:"pullRequest"`
}

// line is the thread's line, 0 when GitHub gives none (an outdated thread).
func (t threadNode) line() int {
	if t.Line == nil {
		return 0
	}
	return *t.Line
}

// open is the thread as push threads lists it: its first comment's author, ""
// when GitHub no longer has it, and the first line of that comment.
func (t threadNode) open() openThread {
	o := openThread{id: t.ID, path: t.Path, line: t.line()}
	if len(t.Comments.Nodes) > 0 {
		c := t.Comments.Nodes[0]
		first, _, _ := strings.Cut(c.Body, "\n")
		o.first = strings.TrimSpace(first)
		if c.Author != nil {
			o.author = c.Author.Login
		}
	}
	return o
}

// place is a thread's path, and :line after it when it has a line.
func place(path string, line int) string {
	if line > 0 {
		return path + ":" + strconv.Itoa(line)
	}
	return path
}

// reviewThreads is push threads and push thread (slices 191 and 201). Nothing is
// fetched, rebased or pushed. Both are refused, before any gh call, outside
// push.mode pr. push thread asks GitHub for the node its id names
// (threadByNode) and push threads --pr <n> for the pull request by its number
// (pullByNumber), from any branch; push threads alone is the branch's
// (branchThreads). threads makes ghFailed say no more than the failure, since
// the branch is not pushed by it.
func (r *pushRun) reviewThreads(ctx context.Context, q Query) Answer {
	r.threads = true
	if !r.prMode(ctx) {
		return r.report(policy, "threads-without-pr-mode",
			"itos push: review threads are an item's pull request's, and this config does not set push.mode: pr; nothing was posted.")
	}
	switch {
	case q.Thread != "":
		return r.threadByNode(ctx, q)
	case q.PullNumber > 0:
		return r.pullByNumber(ctx, q.PullNumber)
	}
	return r.branchThreads(ctx)
}

// repositoryNamed is the GitHub repository push names for the remote; else the
// no-github-repository report (exit 3): no repository to find what is asked
// about (subject) in.
func (r pushRun) repositoryNamed(ctx context.Context, remote, subject string) (string, Answer, bool) {
	repository := r.ports.GitHubRepository(ctx, r.cfg, remote)
	if repository == "" {
		return "", r.report(missing, "no-github-repository",
			fmt.Sprintf("itos push: %s names no GitHub repository to find %s in; nothing was posted.", remote, subject)), true
	}
	return repository, Answer{}, false
}

// branchThreads is push threads with no --pr: the open threads of the branch's
// pull request. It is refused, exit 1, on a detached HEAD and on main (asked as
// push asks, defaultBranch), which have no pull request; else gh finds the
// open pull request (no-pull-request when there is none) and its threads (ask).
func (r *pushRun) branchThreads(ctx context.Context) Answer {
	branch := r.ports.Branch(ctx)
	if branch == "" {
		return r.report(policy, "detached",
			"itos push: HEAD is not on a branch; check out the item's branch first, or name the pull request with --pr <n>. Nothing was posted.")
	}
	remote := r.namedRemote(ctx)
	r.remote = remote
	main, a, done := r.defaultBranch(ctx)
	r.remote = ""
	if done {
		return a
	}
	if branch == main {
		return r.report(policy, "threads-on-main",
			fmt.Sprintf("itos push: HEAD is on %s, which has no pull request; run it from the item's branch, or name the pull request with --pr <n>. Nothing was posted.", main))
	}
	repository, a, done := r.repositoryNamed(ctx, remote, "the pull request of "+branch)
	if done {
		return a
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
	return r.listThreads(pull, state.unresolved)
}

// pullByNumber is push threads --pr: the open threads of the repository's pull
// request n, asked for with its address (pullThreadsQuery), so no gh pr list.
func (r pushRun) pullByNumber(ctx context.Context, number int) Answer {
	repository, a, done := r.repositoryNamed(ctx, r.namedRemote(ctx), "pull request "+strconv.Itoa(number))
	if done {
		return a
	}
	owner, name, _ := strings.Cut(repository, "/")
	answered, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+pullThreadsQuery, "-f", "owner="+owner, "-f", "name="+name,
		"-F", "number="+strconv.Itoa(number))
	if failed {
		return a
	}
	var got struct {
		Data struct {
			Repository struct {
				PullRequest *struct {
					PullRef
					ReviewThreads *struct {
						Nodes []threadNode `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	err := json.Unmarshal([]byte(answered), &got)
	pull := got.Data.Repository.PullRequest
	if err != nil || pull == nil || pull.ReviewThreads == nil || pull.URL == "" {
		return r.ghFailed("gh api graphql", "its answer is not a pull request's review threads")
	}
	var unresolved []openThread
	for _, t := range pull.ReviewThreads.Nodes {
		if !t.IsResolved {
			unresolved = append(unresolved, t.open())
		}
	}
	return r.listThreads(&pull.PullRef, unresolved)
}

// threadByNode is push thread: the node the id names, asked for by that id from
// any branch, a thread's id naming its own pull request. An id GitHub has no
// node for, a node that is no review thread and a thread of another repository
// than the one push names (compared regardless of case) are refused
// (no-such-thread) before anything is posted; else the thread is shown, or
// answered when there is a reply or a resolve.
func (r pushRun) threadByNode(ctx context.Context, q Query) Answer {
	repository, a, done := r.repositoryNamed(ctx, r.namedRemote(ctx), "review thread "+q.Thread)
	if done {
		return a
	}
	node, a, done := r.askNode(ctx, q.Thread)
	if done {
		return a
	}
	if node.ID == "" || !strings.EqualFold(node.Repository.NameWithOwner, repository) {
		return r.report(policy, "no-such-thread",
			fmt.Sprintf("itos push: %s is not a review thread of %s; itos push threads lists a pull request's open ones. Nothing was posted.", q.Thread, repository))
	}
	if q.Reply == "" && !q.Resolve {
		return r.showThread(node)
	}
	return r.answerThread(ctx, q, &node.PullRequest)
}

// askNode puts nodeQuery to GitHub about the id: the node, which has no id
// when GitHub has none. gh fails for an id it has no node for, its stdout still
// the body with an error of type NOT_FOUND on the path node; any other failure,
// or an answer that is no JSON, ends it with the report (exit 3).
func (r pushRun) askNode(ctx context.Context, id string) (nodeThread, Answer, bool) {
	answered, a, failed := r.gh(ctx, "api", "graphql", "-f", "query="+nodeQuery, "-f", "id="+id)
	var got struct {
		Data struct {
			Node nodeThread `json:"node"`
		} `json:"data"`
		Errors []struct {
			Type string `json:"type"`
			Path []any  `json:"path"`
		} `json:"errors"`
	}
	err := json.Unmarshal([]byte(answered), &got)
	if failed {
		for _, e := range got.Errors {
			if e.Type == "NOT_FOUND" && len(e.Path) == 1 && e.Path[0] == "node" {
				return nodeThread{}, Answer{}, false
			}
		}
		return nodeThread{}, a, true
	}
	if err != nil {
		return nodeThread{}, r.ghFailed("gh api graphql", "its answer is not a review thread's"), true
	}
	return got.Data.Node, Answer{}, false
}

// showThread is push thread with neither --reply nor --resolve: the thread's
// place and whether it is resolved, then each comment, oldest first, its
// author on a line and its body indented below, a \r\n read as a \n.
func (r pushRun) showThread(node nodeThread) Answer {
	pull := node.PullRequest
	view := &ThreadView{ID: node.ID, Path: node.Path, Line: node.line(), Resolved: node.IsResolved, Comments: []CommentView{}}
	state := "unresolved"
	if node.IsResolved {
		state = "resolved"
	}
	lines := []string{fmt.Sprintf("%s %s on %s, %s:", node.ID, place(node.Path, view.Line), pull.URL, state)}
	for _, c := range node.Comments.Nodes {
		author := "ghost"
		if c.Author != nil {
			author = c.Author.Login
		}
		view.Comments = append(view.Comments, CommentView{Author: author, Body: c.Body})
		lines = append(lines, author+":")
		for _, line := range strings.Split(strings.ReplaceAll(c.Body, "\r\n", "\n"), "\n") {
			lines = append(lines, "  "+line)
		}
	}
	a := r.report(0, "thread-shown", lines...)
	a.Pull, a.Thread = &pull, view
	return a
}

// listThreads is push threads' end: the open threads, one line each.
func (r pushRun) listThreads(pull *PullRef, unresolved []openThread) Answer {
	threads := []ThreadInfo{}
	lines := []string{pull.URL + " has no unresolved review threads."}
	if len(unresolved) > 0 {
		lines = []string{fmt.Sprintf("%s has %d unresolved review threads:", pull.URL, len(unresolved))}
	}
	for _, t := range unresolved {
		threads = append(threads, ThreadInfo{ID: t.id, Path: t.path, Line: t.line, Author: t.author, Comment: t.first})
		lines = append(lines, fmt.Sprintf("  %s %s %s: %s", t.id, place(t.path, t.line), t.author, t.first))
	}
	a := r.report(0, "threads-listed", lines...)
	a.Pull, a.Threads = pull, threads
	return a
}

// answerThread is push thread's end, once the thread is known to be this
// repository's: the reply when there is text, then the resolve when asked. A
// call that fails ends it with what was done said before the failure.
func (r pushRun) answerThread(ctx context.Context, q Query, pull *PullRef) Answer {
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
