package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/kind"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// pushRun is one itos push: its ports, where its report goes and what it
// says under --json.
type pushRun struct {
	ports          Ports
	o              Out
	origin         string    // the folder git runs in (Query.Origin)
	git            io.Writer // where git's stdout goes: stderr under --json
	remote, branch string
	tags, refs     []string         // what --tag and --ref named, when they did (slice 131)
	sha            string           // the commit pushed, resolved once before the push
	noWait         bool             // --no-wait: push and return, whatever ci.watch says
	watched        *ciwatch.Watched // the CI run waited for after the push, if one was
	cfg            *config.Loaded   // the config, when push.mode is pr (prMode)
	ontoName       string           // what the branch is rebased onto, when it is not the upstream (pr.go)
	pr             *PullRequest     // the pull request of the item's branch, once found or opened (pr.go)
	noUpstream     string           // the local branch, when it has no upstream and goes to the remote's default branch (trunkDefault)
}

// report ends the run: Run prints the outcome under --json, else the lines
// on stdout for a success, on stderr for anything else.
func (r pushRun) report(code int, outcome string, lines ...string) Answer {
	return Answer{Code: code, Outcome: outcome, Lines: append(lines, r.noUpstreamNote()...), Remote: r.remote, Branch: r.branch, Tags: r.tags,
		Refs: r.refs, Commit: r.sha, Watched: r.watched, PullRequest: r.pr}
}

// upstream is the upstream's name as people write it: origin/main.
func (r pushRun) upstream() string { return r.remote + "/" + r.branch }

// noUpstreamNote is the report's line for a branch with no upstream that went
// to the remote's default branch: that it has none, and how to push it to
// its own name; nothing otherwise.
func (r pushRun) noUpstreamNote() []string {
	if r.noUpstream == "" {
		return nil
	}
	return []string{fmt.Sprintf("%s has no upstream, so itos push used %s; itos push --ref %s pushes it to its own name.",
		r.noUpstream, r.upstream(), r.noUpstream)}
}

// trunkDefault points a trunk push of a branch with no upstream at the
// remote's default branch (bug 136), found as pr finds main (defaultBranch):
// the branch is then fetched and rebased onto, and pushed to, as an upstream
// is, and the report says so (noUpstreamNote). A remote with no branch at all
// has none to name and keeps the branch's own name, which the push makes; one
// with branches and no default is refused, and so is one that cannot be asked.
// No upstream is written: each push asks the remote again.
func (r *pushRun) trunkDefault(ctx context.Context, branch string) (Answer, bool) {
	main, a, done := r.defaultBranch(ctx)
	if !done {
		r.branch, r.noUpstream = main, branch
		return Answer{}, false
	}
	if a.Outcome != "no-default-branch" {
		return a, true
	}
	refs, err := r.ports.RemoteRefs(ctx, r.remote, nil, gitrun.Talking(r.origin, nil, io.Discard, nil))
	if err != nil {
		return r.askFailed(err, "listing "+r.remote+"'s branches failed"), true
	}
	for ref := range refs {
		if strings.HasPrefix(ref, "refs/heads/") {
			return a, true
		}
	}
	return Answer{}, false
}

// base is what the branch is rebased onto, as people write it: the upstream,
// else what a pr-mode push chose (ontoName).
func (r pushRun) base() string {
	if r.ontoName != "" {
		return r.ontoName
	}
	return r.upstream()
}

// ready is whether the working tree can be pulled into: no rebase in
// progress, no conflict left and no uncommitted change to a tracked file;
// done when it cannot, with the refusal reported.
func (r pushRun) ready(ctx context.Context) (Answer, bool) {
	v := r.ports
	if v.Rebasing(ctx) || len(v.Conflicted(ctx)) > 0 {
		return r.report(policy, "rebase-in-progress",
			"itos push: a rebase is in progress or a conflict is left; nothing was pulled or pushed.",
			"Resolve it and run git rebase --continue, or run git rebase --abort, then itos push again."), true
	}
	if changed := v.Changes(ctx); len(changed) > 0 {
		lines := []string{"itos push: tracked files have uncommitted changes; commit or stash them first, then run itos push again:"}
		for _, l := range changed {
			lines = append(lines, "  "+l)
		}
		return r.report(policy, "uncommitted", lines...), true
	}
	return Answer{}, false
}

// fetch fetches the upstream's branch and gives the commit fetched; exists
// is false when the remote has no such branch yet, which a push creates.
// git's own words are printed only when the fetch fails, and failure is then
// their kind (remoteKind). A remote out of reach, or no repository, is not
// asked again whether it has the branch: the answer would be the same
// failure, after a second wait for a connection that times out.
func (r pushRun) fetch(ctx context.Context, remote, ref string) (onto string, exists bool, failure kind.Kind, failed bool) {
	v := r.ports
	sha, err := v.Fetch(ctx, remote, ref, r.talk(nil))
	if err == nil {
		return sha, true, kind.Unknown, false
	}
	failure = remoteKind(err)
	// A remote without the branch lists none of it.
	if failure == kind.Unknown {
		if refs, err := v.RemoteRefs(ctx, remote, []string{ref}, gitrun.Talking(r.origin, nil, io.Discard, nil)); err == nil && len(refs) == 0 {
			return "", false, kind.Unknown, false
		}
	}
	io.WriteString(r.o.Stderr, words(err))
	return "", false, failure, true
}

// words is what git said on stderr of a command that failed, as it said
// it; none when it did not run.
func words(err error) string {
	var failed *vcsport.Failed
	if errors.As(err, &failed) {
		return failed.Words
	}
	return ""
}

// remoteKind is the kind of a fetch's or a push's failure, read from git's
// words: kind.Temporary for a remote out of reach, kind.Missing for one that
// is no repository or cannot be found, and kind.Unknown for any other
// failure, a refusal among them.
func remoteKind(err error) kind.Kind {
	var failed *vcsport.Failed
	if !errors.As(err, &failed) {
		return kind.Unknown
	}
	switch failed.Failure {
	case vcsport.Unreachable:
		return kind.Temporary
	case vcsport.NoRepository:
		return kind.Missing
	}
	return kind.Unknown
}

// hasRemote is whether the repository has the remote.
func hasRemote(ctx context.Context, v vcsport.VCS, remote string) bool {
	_, err := v.RemoteURL(ctx, remote)
	return err == nil
}

// remoteExit is the exit code of a fetch's or a push's failure of the kind,
// as internal/cli's ExitCode gives it: 75 for a remote out of reach, 3 for
// one that is no repository, 70 for a failure of no kind.
func remoteExit(failure kind.Kind) int {
	switch failure {
	case kind.Temporary:
		return temporary
	case kind.Missing:
		return missing
	}
	return software
}

// remoteLines say what failed, by its kind, and what to do next; nothing was
// pushed, whatever the kind.
func (r pushRun) remoteLines(what string, failure kind.Kind) []string {
	switch failure {
	case kind.Temporary:
		return []string{
			fmt.Sprintf("itos push: %s: %s cannot be reached (git's message above); nothing was pushed.", what, r.remote),
			"Check the network, then run itos push again.",
		}
	case kind.Missing:
		return []string{
			fmt.Sprintf("itos push: %s: %s is no repository git can find (git's message above); nothing was pushed.", what, r.remote),
			fmt.Sprintf("Check the remote's URL (git remote get-url %s), then run itos push again.", r.remote),
		}
	}
	return []string{fmt.Sprintf("itos push: %s (git's message above); nothing was pushed", what)}
}

// rebase rebases the branch onto the commit fetched, with git's own
// settings for it overruled; done when the rebase stopped or did not start,
// or left a conflict, with the outcome reported: nothing is pushed then.
func (r pushRun) rebase(ctx context.Context, onto string) (Answer, bool) {
	v := r.ports
	if v.Stealth(ctx) {
		if err := gitrun.RewriteNotes(ctx, v); err != nil {
			fmt.Fprintf(r.o.Stderr, "itos: cannot set notes.rewriteRef: %s\n", err)
			return Answer{Code: missing}, true
		}
	}
	code, err := gitrun.ExitOf(v.Rebase(ctx, onto, r.o.Quiet, r.talk(r.o.Stderr)))
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return Answer{Code: missing}, true
	}
	if v.Rebasing(ctx) || len(v.Conflicted(ctx)) > 0 {
		lines := []string{
			fmt.Sprintf("itos push: the rebase onto %s stopped; nothing was pushed.", r.base()),
			"Resolve the conflicts, git add the files and run git rebase --continue, then itos push again;",
			"or run git rebase --abort to go back to where the branch was.",
		}
		return r.report(policy, "rebase-stopped", append(lines, r.registryConflict(ctx)...)...), true
	}
	if code != 0 {
		return r.report(policy, "rebase-failed",
			fmt.Sprintf("itos push: the rebase onto %s did not run (git's message above); nothing was pushed", r.base())), true
	}
	return Answer{}, false
}

// registryConflict is what the push says, in a person's words, when its
// rebase stopped on a conflict in the work registry (slice 66), nothing when
// it did not or there is no config to name the registry: two people took the
// same item, or changed it, and the upstream's commit landed first, so the
// conflict is the lock and the remote keeps the first take. It names the
// file and each item both sides changed, the upstream's owner of one it
// took, and how to give the item up (git rebase --skip drops the commit
// being replayed, a registry command's being the registry alone). When no
// item clashes, both sides added or changed different items (two ideas
// appended at the registry's end, bug 118), so it says to keep both and
// continue: --skip would drop the commit's own items.
func (r pushRun) registryConflict(ctx context.Context) []string {
	v := r.ports
	if !v.ConfigFound(ctx) {
		return nil
	}
	cfg, err := v.LoadConfig(ctx)
	if err != nil {
		return nil
	}
	registry := path.Clean(filepath.ToSlash(cfg.Work.Registry))
	hit := false
	for _, f := range v.Conflicted(ctx) {
		hit = hit || path.Clean(f) == registry
	}
	if !hit {
		return nil
	}
	show := func(rev string) string {
		text, _, _ := v.File(ctx, rev, registry)
		return text
	}
	clashes := work.Clashes(show("REBASE_HEAD^"), show("HEAD"), show("REBASE_HEAD"))
	if len(clashes) == 0 {
		return []string{"", fmt.Sprintf("The conflict is in the work registry, %s: a commit on %s and yours added or", registry, r.base()),
			"changed different items, so nothing was taken twice. Keep both sides' items in the file, git add it",
			"and run git rebase --continue, then itos push again; or run git rebase --abort to leave it."}
	}
	lines := []string{"", fmt.Sprintf("The conflict is in the work registry, %s: a commit on %s changed", registry, r.base()),
		"the same item as yours, so it was taken, or changed, there first, and the remote keeps that."}
	for _, c := range clashes {
		switch {
		case c.Taken && c.Owner != "":
			lines = append(lines, fmt.Sprintf("  %s is taken: a commit on %s made %s its owner first.", c.ID, r.base(), c.Owner))
		default:
			lines = append(lines, fmt.Sprintf("  %s was changed on %s too.", c.ID, r.base()))
		}
	}
	return append(lines,
		"To leave it to them, run git rebase --skip, which drops your commit of the registry, then itos",
		"push again and itos work for another item; or agree with them who keeps it, and resolve the",
		"conflict as above.")
}

// push pushes the commit resolved before it, r.sha, to the upstream's
// branch: the pre-push hook runs, and a refusal, the remote's or the hook's,
// exits 1; any other failure exits by its kind, read from git's words as they
// stream to stderr. onto is the upstream's commit the branch was rebased onto, empty
// when the push makes the branch. HEAD is not read again: a commit made
// while the hook ran is not the one pushed.
func (r pushRun) push(ctx context.Context, remote, ref, onto string) Answer {
	if a, failed := r.send(ctx, []string{r.sha + ":" + ref}, false); failed {
		return a
	}
	pushed := r.pushedLine(ctx, "")
	if r.noWait {
		return r.report(0, "pushed", pushed)
	}
	return r.wait(ctx, remote, onto, pushed)
}

// send pushes the refspecs, in one atomic push when atomic is set: failed,
// with the outcome reported, when git or the pre-push hook refuses it.
func (r pushRun) send(ctx context.Context, specs []string, atomic bool) (Answer, bool) {
	outcome := r.ports.Push(ctx, r.remote, specs, vcsport.Pushing{Atomic: atomic, Quiet: r.o.Quiet}, r.talk(r.o.Stderr))
	code, err := gitrun.ExitOf(outcome)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return Answer{Code: missing}, true
	}
	if code != 0 {
		if failure := remoteKind(outcome); failure != kind.Unknown {
			return r.report(remoteExit(failure), "push-failed", r.remoteLines("the push to "+r.upstream()+" failed", failure)...), true
		}
		return r.report(refusalExit(outcome), "push-failed",
			fmt.Sprintf("itos push: the push to %s failed (git's message above); nothing was forced.", r.upstream()),
			"If the remote moved, run itos push again to rebase onto it; if a hook refused it, fix what it reported first."), true
	}
	return Answer{}, false
}

// pushedLine says what was pushed: the commit and where, then, when there is
// one, the pull request's address.
func (r pushRun) pushedLine(ctx context.Context, address string) string {
	shown, _ := r.ports.Show(ctx, r.sha)
	line := fmt.Sprintf("Pushed %s to %s.", shown.Short, r.upstream())
	if address != "" {
		line += " Pull request: " + address
	}
	return line
}

// wait waits for the CI run of the commit pushed when ci.watch has a
// provider, and exits with the run's result; with no config, or none, it
// reports the push as before. The commits are pushed whatever the run says.
// A config that cannot be read leaves the push as it was, saying why
// nothing was watched. A push whose commits touch only the work registry
// is not waited for either (slice 56): itos wrote and checked those
// commits, an item's take and its close, and a run for each made one item
// cost three waits; it says how to wait for that run all the same.
func (r *pushRun) wait(ctx context.Context, remote, onto, pushed string) Answer {
	if !r.ports.ConfigFound(ctx) {
		return r.report(0, "pushed", pushed)
	}
	cfg, err := r.ports.LoadConfig(ctx)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: no CI run watched, the config cannot be read: %s\n", err)
		return r.report(0, "pushed", pushed)
	}
	sha := r.sha
	if cfg.CI.Watch.Provider != "none" && r.registryOnly(ctx, onto, sha, cfg.Work.Registry) {
		return r.report(0, "pushed", pushed,
			fmt.Sprintf("Its commits touch only %s, so its CI run is not waited for; itos ci watch %s waits for it.", cfg.Work.Registry, sha))
	}
	wr, ok, err := r.ports.Watcher(ctx, cfg, remote)
	if !ok && err == nil {
		return r.report(0, "pushed", pushed)
	}
	if !r.o.JSON && !r.o.Quiet {
		fmt.Fprintln(r.o.Stdout, pushed)
		for _, l := range r.noUpstreamNote() {
			fmt.Fprintln(r.o.Stdout, l)
		}
	}
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: %s; the commits are pushed, and itos ci watch %s waits for their run\n", err, sha)
		r.watched = &ciwatch.Watched{Code: missing, Outcome: "error"}
	} else {
		w := ciwatch.Watch(ctx, cfg, wr, r.ports, sha, remote, "", ciwatch.Out(r.o))
		r.watched = &w
	}
	if r.o.JSON {
		return r.report(r.watched.Code, "pushed")
	}
	return Answer{Code: r.watched.Code}
}

// registryOnly is whether the commits the push added to the upstream's
// branch, onto..pushed, touch the work registry and no other path
// (touchesOnlyRegistry). A push that makes the branch has no onto, and is
// never registry-only: what it adds is not known without the remote's other
// branches.
func (r pushRun) registryOnly(ctx context.Context, onto, pushed, registry string) bool {
	if onto == "" {
		return false
	}
	files, err := r.ports.Touched(ctx, vcsport.Range{From: onto, To: pushed}, true)
	return touchesOnlyRegistry(registry, files, err)
}

// touchesOnlyRegistry is whether the paths some commits touch, each against
// its first parent, are the work registry and no other path: the one rule by
// which itos push does not wait for a run (slice 56) and work done passes
// over a commit (slice 93), since such a commit changes nothing a run
// judges. A merge counts what it brought in against its first parent, and a
// commit that touches no path adds nothing to the answer, so commits that
// touch nothing at all are not registry-only; nor are commits whose paths
// cannot be read.
func touchesOnlyRegistry(registry string, files []string, err error) bool {
	if err != nil {
		return false
	}
	registry = path.Clean(filepath.ToSlash(registry))
	touched := false
	for _, f := range files {
		if path.Clean(f) != registry {
			return false
		}
		touched = true
	}
	return touched
}

// talk is how the push runs git: as gitrun.Talking has it, from the folder
// the person stood in, its output the run's and its words to stderr.
func (r pushRun) talk(stderr io.Writer) vcsport.Talk {
	return gitrun.Talking(r.origin, nil, r.git, stderr)
}
