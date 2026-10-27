package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/glob"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// PullRequest is the pull request of an item's branch under push.mode pr, as
// --json gives it: its address and number, whether this push opened it,
// whether it asked GitHub to rebase it, whether it is a draft once the push is
// done and whether this push turned on auto-merge (slice 179).
type PullRequest struct {
	URL       string `json:"url"`
	Number    int    `json:"number"`
	Created   bool   `json:"created"`
	Updated   bool   `json:"updated"`
	Draft     bool   `json:"draft"`
	AutoMerge bool   `json:"auto_merge"`
}

// prMode is whether the config says push.mode pr, keeping the config for the
// run. A push with no config, or one that cannot be read, is a trunk push.
func (r *pushRun) prMode(ctx context.Context) bool {
	if !r.ports.ConfigFound(ctx) {
		return false
	}
	cfg, err := r.ports.LoadConfig(ctx)
	if err != nil || cfg.Push.Mode != "pr" {
		return false
	}
	r.cfg = cfg
	return true
}

// defaultBranch is main under pr, and the branch a push with no upstream goes
// to under trunk (trunkDefault): the branch the remote's HEAD names, which
// the remote has. done, with the outcome reported, when the remote cannot be
// asked (fetch-failed, by the kind of git's message) or answers without such
// a branch (no-default-branch).
func (r pushRun) defaultBranch(ctx context.Context) (main string, a Answer, done bool) {
	talk := gitrun.Talking(r.origin, nil, io.Discard, nil)
	main, err := r.ports.RemoteHead(ctx, r.remote, talk)
	if err != nil {
		return "", r.askFailed(err, "reading "+r.remote+"'s HEAD failed"), true
	}
	if main != "" {
		has, err := r.ports.RemoteRefs(ctx, r.remote, []string{"refs/heads/" + main}, talk)
		if err != nil {
			return "", r.askFailed(err, "listing "+r.remote+"'s "+main+" failed"), true
		}
		if len(has) > 0 {
			return main, Answer{}, false
		}
	}
	return "", r.noDefault(), true
}

// noDefault is the no-default-branch refusal (exit 3): under pr there is no
// main to put the work behind; under trunk, a branch with no upstream has no
// default branch to go to (r.cfg is set only under pr).
func (r pushRun) noDefault() Answer {
	if r.cfg == nil {
		return r.report(missing, "no-default-branch",
			fmt.Sprintf("itos push: %s has no upstream, and %s's HEAD names no branch %s has, so there is no default branch to push it to; nothing was fetched or pushed.", r.branch, r.remote, r.remote),
			"Set the remote's default branch, then itos push again; or push the branch to its own name with itos push --ref "+r.branch+".")
	}
	return r.report(missing, "no-default-branch",
		fmt.Sprintf("itos push: %s's HEAD names no branch %s has, so there is no main to put the work behind; nothing was fetched or pushed.", r.remote, r.remote),
		"Set the remote's default branch, then itos push again.")
}

// askFailed is the fetch-failed outcome of a question put to the remote that
// failed: the words it gave, then the exit by their kind.
func (r pushRun) askFailed(err error, what string) Answer {
	io.WriteString(r.o.Stderr, words(err))
	failure := remoteKind(err)
	return r.report(remoteExit(failure), "fetch-failed", r.remoteLines(what, failure)...)
}

// judgeRefs judges --ref under pr: one naming main is a push from main,
// refused when it adds a commit that touches more than the registry,
// against the remote's main. done with that refusal, or the failure of a
// question put to the remote; not done for a --ref that names no main (a
// remote with no main has none to name), or a branch the clone lacks, which
// pushNamed reports.
func (r *pushRun) judgeRefs(ctx context.Context, refs []string) (Answer, bool) {
	r.remote = r.namedRemote(ctx)
	main, a, done := r.defaultBranch(ctx)
	if done && a.Outcome == "no-default-branch" {
		return Answer{}, false
	}
	if done || !slices.Contains(refs, main) {
		return a, done
	}
	r.branch = main
	local, err := r.ports.Ref(ctx, "refs/heads/"+main)
	if err != nil {
		return Answer{}, false
	}
	onto, _, failure, failed := r.fetch(ctx, r.remote, "refs/heads/"+main)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("fetching "+r.upstream()+" failed", failure)...), true
	}
	if oldest, adds := r.addsWork(ctx, onto, local); adds {
		return r.workOnMain(ctx, oldest), true
	}
	return Answer{}, false
}

// addsWork is whether the commits onto..sha include one that is neither the
// registry's (registryCommit) nor one push.pr.direct sends straight to main
// (direct), each judged by itself, and the oldest such commit; commits that
// cannot be listed count as work, with no commit named.
func (r pushRun) addsWork(ctx context.Context, onto, sha string) (oldest string, adds bool) {
	ahead, err := r.ports.Commits(ctx, vcsport.Range{From: onto, To: sha})
	if err != nil {
		return "", true
	}
	i := slices.IndexFunc(ahead, func(c string) bool { return !r.registryCommit(ctx, c) && !r.direct(ctx, c) })
	if i < 0 {
		return "", false
	}
	return ahead[i], true
}

// direct is whether the commit is one push.pr.direct sends straight to main
// (slice 186): its type is one of push.pr.direct.types and a commit type too
// (message.Typed, since the header lint that judges a type against
// commits.types is optional), and no path it changes against its first parent, renames off, merged-in history left out, matches a push.pr.direct.except
// glob; a glob that cannot be read matches.
func (r pushRun) direct(ctx context.Context, commit string) bool {
	d := r.cfg.Push.PR.Direct
	if len(d.Types) == 0 {
		return false
	}
	entry, err := r.ports.Show(ctx, commit)
	typ := message.Type(entry.Message)
	if err != nil || !slices.Contains(d.Types, typ) || !message.Typed(r.cfg, typ) {
		return false
	}
	files, err := r.ports.Diff(ctx, r.ports.Parent(ctx, commit), commit)
	if err != nil {
		return false
	}
	for _, f := range files {
		if matched, err := glob.MatchesAny(path.Clean(f), d.Except); matched || err != nil {
			return false
		}
	}
	return true
}

// registryCommit is whether the commit, against its first parent, is the
// registry's for pr's two judgements (slice 177): see registryPaths.
func (r pushRun) registryCommit(ctx context.Context, commit string) bool {
	files, err := r.ports.Touched(ctx, vcsport.Range{From: r.ports.Parent(ctx, commit), To: commit}, true)
	return registryPaths(r.cfg, files, err)
}

// registryPaths is whether the paths a commit touches are the registry's: the
// work registry, and no path but ledger files besides, which work done, task
// add and work promote edit beside it. With no ledger section no path is a
// ledger file, and the rule is touchesOnlyRegistry's.
func registryPaths(cfg *config.Loaded, files []string, err error) bool {
	if err != nil {
		return false
	}
	registry := path.Clean(filepath.ToSlash(cfg.Work.Registry))
	layout, noLedger := ledger.LayoutOf(cfg)
	touched := false
	for _, f := range files {
		f = path.Clean(f)
		switch {
		case f == registry:
			touched = true
		case noLedger == nil && path.Dir(f) == layout.Dir && layout.File.MatchString(path.Base(f)):
		default:
			return false
		}
	}
	return touched
}

// workOnMain is the refusal of a push to main that adds work (outcome
// work-on-main): under pr only the registry's commits, and those
// push.pr.direct names, go straight to main. With push.pr.direct.types set it
// names the oldest commit that is neither, when it can be told.
func (r pushRun) workOnMain(ctx context.Context, oldest string) Answer {
	hint := "Carry the work to its item's branch with git switch -c <item>, then itos push again from there."
	if oldest != "" && len(r.cfg.Push.PR.Direct.Types) > 0 {
		if entry, err := r.ports.Show(ctx, oldest); err == nil {
			return r.report(policy, "work-on-main",
				fmt.Sprintf("itos push: push.mode is pr, and this push adds %s %s to %s, which is neither the registry's nor a commit push.pr.direct sends straight to main; nothing was pushed.",
					oldest[:min(len(oldest), 7)], entry.Subject, r.upstream()), hint)
		}
	}
	return r.report(policy, "work-on-main",
		fmt.Sprintf("itos push: push.mode is pr, and this push adds commits to %s that touch more than %s; nothing was pushed.",
			r.upstream(), r.cfg.Work.Registry), hint)
}

// pushItem is a push from a branch other than main under pr (ADR-0039): the
// branch is named for its item, which the registry must list; it is rebased
// onto the remote's item branch when there is one, else onto main, and the
// registry's commits directly above main go to main, the rest to the item's
// branch, in one atomic push; then the pull request is found or opened.
func (r *pushRun) pushItem(ctx context.Context, item, main string) Answer {
	v := r.ports
	registry := r.cfg.Work.Registry
	r.branch = item
	if ids, err := work.IDsAt(r.cfg.Trees, registry, ""); err != nil || !ids[item] {
		return r.report(policy, "not-item-branch",
			fmt.Sprintf("itos push: push.mode is pr, so a branch is named for its item, and %s is no item of %s; nothing was fetched or pushed.", item, registry),
			"Rename the branch with git branch -m <item>, then itos push again.")
	}
	theirMain, _, failure, failed := r.fetch(ctx, r.remote, "refs/heads/"+main)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("fetching "+r.remote+"/"+main+" failed", failure)...)
	}
	tip, exists, failure, failed := r.fetch(ctx, r.remote, "refs/heads/"+item)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("fetching "+r.upstream()+" failed", failure)...)
	}
	onto := theirMain
	r.ontoName = r.remote + "/" + main
	if exists {
		onto, r.ontoName = tip, r.upstream()
	}
	if !v.IsAncestor(ctx, onto, "HEAD") {
		if a, done := r.rebase(ctx, onto); done {
			return a
		}
	}
	if a, failed := r.resolveHead(ctx); failed {
		return a
	}
	ahead, err := v.Commits(ctx, vcsport.Range{From: theirMain, To: r.sha})
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot list the commits %s adds to %s/%s: %s\n", item, r.remote, main, err)
		return Answer{Code: missing}
	}
	if len(ahead) == 0 {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s adds nothing to %s/%s.", item, r.remote, main))
	}
	toMain := r.registryPrefix(ctx, theirMain, ahead, tip)
	if toMain == len(ahead) {
		return r.pushRegistry(ctx, main, theirMain)
	}
	repository := v.GitHubRepository(ctx, r.cfg, r.remote)
	if repository == "" {
		return r.report(missing, "no-github-repository",
			fmt.Sprintf("itos push: %s has work for a pull request, and %s names no GitHub repository to open it in; nothing was pushed.", item, r.remote),
			"Set "+r.cfg.CI.Range.GitHub.RepositoryEnv+" to owner/name, or point the remote at a GitHub repository, then itos push again.")
	}
	pushing := !exists || tip != r.sha
	if pushing {
		specs := []string{r.sha + ":refs/heads/" + item}
		if toMain > 0 {
			specs = append([]string{ahead[toMain-1] + ":refs/heads/" + main}, specs...)
		}
		if a, failed := r.send(ctx, specs, true); failed {
			return a
		}
	}
	return r.pullRequest(ctx, repository, main, theirMain, ahead[toMain], pushing, !v.IsAncestor(ctx, theirMain, r.sha))
}

// registryPrefix is how many of the commits above main, oldest first, go to
// main: those that start at main's head, each directly on the one before,
// are the registry's (registryCommit) and are not on the item's branch yet
// (theirs, its tip, "" when the remote has none).
func (r pushRun) registryPrefix(ctx context.Context, main string, ahead []string, theirs string) int {
	v := r.ports
	parent := main
	for i, c := range ahead {
		if v.Parent(ctx, c) != parent || theirs != "" && v.IsAncestor(ctx, c, theirs) {
			return i
		}
		if !r.registryCommit(ctx, c) {
			return i
		}
		parent = c
	}
	return len(ahead)
}

// pushRegistry sends a branch whose commits are all the registry's to main
// alone: the item's branch is not pushed, no pull request is opened and no
// run waited for, as for registry commits pushed from main.
func (r *pushRun) pushRegistry(ctx context.Context, main, onto string) Answer {
	r.branch = main
	if a, failed := r.send(ctx, []string{r.sha + ":refs/heads/" + main}, true); failed {
		return a
	}
	pushed := r.pushedLine(ctx, "")
	if r.noWait {
		return r.report(0, "pushed", pushed)
	}
	return r.wait(ctx, r.remote, onto, pushed)
}

// pullRequest finds the item's open pull request or opens one. Under
// push.pr.draft a plain push opens it as a draft, if it opens it, and does
// nothing more: no auto-merge, no rebase, no wait. --ready opens it ready or
// marks the draft ready (gh pr ready), then finishes (ready.go). Otherwise it
// turns on auto-merge by rebase. In every case but a plain draft push, when
// main has a commit the pushed head lacks (behind), it asks GitHub to rebase
// the pull request rather than forcing the branch; the wait then watches the
// run of the head GitHub makes (rebasedHead), not the one pushed. theirMain is
// main's head as the push fetched it. pushed is whether this run pushed the
// item's branch.
func (r *pushRun) pullRequest(ctx context.Context, repository, main, theirMain, first string, pushed, behind bool) Answer {
	item := r.branch
	draft := r.cfg.Push.PR.Draft
	listed, a, failed := r.gh(ctx, "pr", "list", "--repo", repository, "--head", item, "--state", "open", "--json", "number,url,isDraft,title")
	if failed {
		return a
	}
	var open []struct {
		Number  int    `json:"number"`
		URL     string `json:"url"`
		IsDraft bool   `json:"isDraft"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal([]byte(listed), &open); err != nil {
		return r.ghFailed("gh pr list", "its answer is not a list of pull requests: "+err.Error())
	}
	if len(open) > 0 {
		r.pr = &PullRequest{URL: open[0].URL, Number: open[0].Number, Draft: open[0].IsDraft}
		if a, failed := r.retitle(ctx, repository, open[0].Title, first); failed {
			return a
		}
	} else {
		title, body := r.describe(item)
		create := []string{"pr", "create", "--repo", repository, "--base", main, "--head", item, "--title=" + title, "--body=" + body}
		asDraft := draft && !r.markReady
		if asDraft {
			create = append(create, "--draft")
		}
		created, a, failed := r.gh(ctx, create...)
		if failed {
			return a
		}
		url := lastLine(created)
		if url == "" {
			return r.ghFailed("gh pr create", "it printed no address")
		}
		r.pr = &PullRequest{URL: url, Number: pullNumber(url), Created: true, Draft: asDraft}
	}
	switch {
	case !draft:
		if _, a, failed := r.gh(ctx, "pr", "merge", item, "--repo", repository, "--auto", "--rebase"); failed {
			return a
		}
		r.pr.AutoMerge = true
	case r.markReady && r.pr.Draft:
		if _, a, failed := r.gh(ctx, "pr", "ready", item, "--repo", repository); failed {
			return a
		}
		r.pr.Draft = false
	}
	if behind && (!draft || r.markReady) {
		asked := now()
		if _, a, failed := r.gh(ctx, "pr", "update-branch", item, "--repo", repository, "--rebase"); failed {
			return a
		}
		r.pr.Updated = true
		r.rebased = &rebased{repository: repository, onto: theirMain, ontoName: r.remote + "/" + main, since: asked}
	}
	outcome, line := "pushed", ""
	if pushed {
		line = r.pushedLine(ctx, r.pr.URL)
	} else {
		outcome, line = "nothing-to-push", fmt.Sprintf("Nothing to push: %s is at %s. Pull request: %s", item, r.upstream(), r.pr.URL)
	}
	switch {
	case draft && !r.markReady:
		return r.report(0, outcome, line, draftNote(r.pr.Draft))
	case r.markReady && r.noWait:
		return r.report(0, outcome, line, r.pr.URL+" is ready for review; itos push --ready again waits for its checks and reviews, then turns on auto-merge.")
	case r.markReady:
		return r.finish(ctx, repository, outcome, line)
	case !pushed || r.noWait:
		return r.report(0, outcome, line)
	}
	return r.wait(ctx, r.remote, "", line)
}

// draftNote is what a plain push under push.pr.draft says of the pull request,
// a draft or already ready, which it leaves as it is.
func draftNote(draft bool) string {
	if draft {
		return "The pull request is a draft: nothing judges or merges it until itos push --ready marks it ready."
	}
	return "The pull request is ready: itos push --ready waits for its checks and reviews, then turns on auto-merge."
}

// rebased is the rebase GitHub was asked for (gh pr update-branch): the
// repository gh names, main's head as the push fetched it (onto) and its name
// as people write it, and when the request was made, from which
// ci.watch.timeout counts.
type rebased struct {
	repository, onto, ontoName string
	since                      time.Time
}

// sleep and now are how the wait for the rebased head pauses and tells the
// time; a test makes them a clock that only sleep moves.
var (
	sleep = time.Sleep
	now   = time.Now
)

// fetchTimeout is how long the quiet fetch of a head the clone lacks may take.
const fetchTimeout = 20 * time.Second

// rebasedHead is the head GitHub made when it rebased the pull request, found
// by asking gh pr view for the item's head at once and each ci.watch.interval,
// and taking the first that has main's head as an ancestor: until GitHub has
// rebased, gh pr view answers the head pushed or the previous push's, neither
// of which has it. lost is whether ci.watch.timeout passed first, counted from
// the request, with the outcome left to the caller; failed, with the outcome
// reported, when gh pr view does.
func (r *pushRun) rebasedHead(ctx context.Context, cfg *config.Loaded) (head string, lost bool, a Answer, failed bool) {
	rb := r.rebased
	interval := time.Duration(*cfg.CI.Watch.Interval * float64(time.Second))
	timeout := *cfg.CI.Watch.Timeout
	deadline := rb.since.Add(time.Duration(timeout * float64(time.Second)))
	for {
		viewed, ended, bad := r.gh(ctx, "pr", "view", r.branch, "--repo", rb.repository, "--json", "headRefOid")
		if bad {
			return "", false, ended, true
		}
		var view struct {
			HeadRefOid string `json:"headRefOid"`
		}
		if err := json.Unmarshal([]byte(viewed), &view); err != nil {
			return "", false, r.ghFailed("gh pr view", "its answer is not a pull request's head: "+err.Error()), true
		}
		if r.hasCommit(ctx, view.HeadRefOid) && r.ports.IsAncestor(ctx, rb.onto, view.HeadRefOid) {
			return view.HeadRefOid, false, Answer{}, false
		}
		left := deadline.Sub(now())
		if left <= 0 {
			fmt.Fprintf(r.o.Stderr, "itos: GitHub has not rebased the pull request onto %s after %s seconds (ci.watch.timeout), so no CI run was watched; its checks are at %s.\n",
				rb.ontoName, value.Number(timeout), r.pr.URL)
			return "", true, Answer{}, false
		}
		sleep(min(interval, left))
	}
}

// hasCommit is whether the clone has the commit rev names, after one quiet
// fetch of the item's branch from the remote when it does not (the watch
// fetches a head it lacks the same way); an empty rev names none.
func (r *pushRun) hasCommit(ctx context.Context, rev string) bool {
	if rev == "" {
		return false
	}
	if _, err := r.ports.Resolve(ctx, rev); err == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	_, _ = r.ports.Fetch(ctx, r.remote, "refs/heads/"+r.branch, vcsport.Talk{NoPrompt: true})
	_, err := r.ports.Resolve(ctx, rev)
	return err == nil
}

// gh runs gh with the arguments: failed, with pull-request-failed reported,
// when it is not installed or fails. The branch is pushed already.
func (r pushRun) gh(ctx context.Context, args ...string) (string, Answer, bool) {
	stdout, err := r.ports.Gh(ctx, args...)
	if err == nil {
		return stdout, Answer{}, false
	}
	command := "gh " + strings.Join(args[:2], " ")
	if errors.Is(err, providers.ErrNoGh) {
		return "", r.ghFailed(command, "gh is not on the PATH"), true
	}
	reason := err.Error()
	var failed *providers.GhFailed
	if errors.As(err, &failed) && strings.TrimSpace(failed.Words) != "" {
		reason = strings.TrimSpace(failed.Words)
	}
	return "", r.ghFailed(command, reason), true
}

// ghFailed is the pull-request-failed outcome (exit 3): the command that
// failed and why, and that itos push again finishes the pull request.
func (r pushRun) ghFailed(command, reason string) Answer {
	again := "Fix that, then itos push again: it opens the pull request (gh pr create) and turns on auto-merge (gh pr merge --auto --rebase)."
	switch {
	case r.markReady:
		again = "Fix that, then itos push --ready again: it opens the pull request ready or marks it ready, waits for its checks and reviews, then turns on auto-merge."
	case r.cfg != nil && r.cfg.Push.PR.Draft:
		again = "Fix that, then itos push again: it opens the pull request as a draft (gh pr create --draft)."
	}
	return r.report(missing, "pull-request-failed",
		fmt.Sprintf("itos push: %s is pushed, but %s failed: %s", r.upstream(), command, reason), again)
}

// lastLine is the last line of the text that is not empty.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

var pullPath = regexp.MustCompile(`/pull/(\d+)`)

// pullNumber is the number in a pull request's address, 0 when it has none.
func pullNumber(url string) int {
	m := pullPath.FindStringSubmatch(url)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
