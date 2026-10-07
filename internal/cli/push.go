package cli

// itos push (slice 39, features/push.feature): the routine every session
// finished with by hand, as one command. Commit first, then pull the
// branch's upstream with a rebase, check that no rebase stopped and no
// conflict is left, then push in a separate step, never forced.
//
// It never asks git's settings how to pull: it fetches the upstream's
// branch and rebases onto the commit fetched with --no-autostash, so
// pull.rebase cannot turn the pull into a merge and rebase.autostash cannot
// pocket a change. It refuses to start when tracked files have uncommitted
// changes instead, untracked files being no reason to. A rebase that stops
// is left for the person to finish, and nothing is pushed. The push is
// the commit HEAD is at once the rebase is done, by its full SHA, to the
// upstream's branch, an explicit refspec, so the hooks run as for any push,
// nothing else goes with it (refs/notes/itos stays local in the stealth
// mode), and nothing forces it. That commit is resolved once (bug 21):
// git runs the pre-push hook after it resolves the refspec, and a commit
// made during the hook's unit tests moves HEAD, so nothing after the push
// reads HEAD again; the commit named, waited for and counted is the one
// pushed. Nothing forces the push: a force flag or a + refspec is
// a usage error, and a push the remote refuses is reported, exit 1, never
// retried. A fetch or a push that fails exits by the kind of the failure,
// read from git's words (slice 90), never with git's own code: a remote out
// of reach 75, one that is no repository 3, any other failure 70. A rebase that stops on a conflict in the work
// registry says so in a person's words beside git's advice (slice 66): two
// takes of one item meet there, and the remote keeps the first.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/kind"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/work"
)

// readPushArgs refuses every argument but --no-wait, which it gives: itos
// push pushes HEAD to the branch's upstream and takes nothing else, and an
// argument that would force the push says why it is refused.
func readPushArgs(args []string) (noWait bool, err error) {
	var rest []string
	for _, arg := range args {
		if arg == "--no-wait" {
			noWait = true
			continue
		}
		rest = append(rest, arg)
	}
	return noWait, refusePushArgs(rest)
}

// refusePushArgs refuses each argument, a forcing one saying why.
func refusePushArgs(args []string) error {
	for _, arg := range args {
		switch {
		case arg == "-f" || strings.HasPrefix(arg, "--force") ||
			(strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "f")):
			return usage("push never forces: %s would overwrite the remote's commits; itos push rebases onto them instead", arg)
		case strings.HasPrefix(arg, "+"):
			return usage("push never forces: %s is a forcing refspec; itos push rebases onto the remote's commits instead", arg)
		}
	}
	if len(args) > 0 {
		return usage("push takes no arguments but --no-wait: it pushes HEAD to the branch's upstream (%s)", args[0])
	}
	return nil
}

// pushRun is one itos push: where its report goes and what it says under
// --json.
type pushRun struct {
	o              Out
	git            io.Writer // where git's stdout goes: stderr under --json
	remote, branch string
	sha            string   // the commit pushed, resolved once before the push
	noWait         bool     // --no-wait: push and return, whatever ci.watch says
	watched        *watched // the CI run waited for after the push, if one was
}

// report ends the run: under --json the outcome on stdout, else the lines
// on stdout for a success, on stderr for anything else.
func (r pushRun) report(code int, outcome string, lines ...string) (int, error) {
	if r.o.JSON {
		fields := []out.Field{{Key: "ok", Value: code == 0}, {Key: "outcome", Value: outcome}}
		if r.remote != "" {
			fields = append(fields, out.Field{Key: "remote", Value: r.remote}, out.Field{Key: "branch", Value: r.branch})
		}
		if outcome == "pushed" && r.sha != "" {
			fields = append(fields, out.Field{Key: "commit", Value: r.sha})
		}
		if r.watched != nil {
			fields = append(fields, r.watched.fields()...)
		}
		return code, out.Emit(r.o.Stdout, fields...)
	}
	w := r.o.Stderr
	if code == 0 {
		if r.o.Quiet {
			return 0, nil
		}
		w = r.o.Stdout
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	return code, nil
}

// upstream is the upstream's name as people write it: origin/main.
func (r pushRun) upstream() string { return r.remote + "/" + r.branch }

// push is `itos push`.
func push(args []string, o Out) (int, error) {
	noWait, err := readPushArgs(args)
	if err != nil {
		return 0, err
	}
	r := pushRun{o: o, git: o.Stdout, noWait: noWait}
	if o.JSON {
		r.git = o.Stderr
	}
	if !git.Succeeds("rev-parse", "--git-dir") {
		fmt.Fprintln(o.Stderr, "itos: push runs in a git repository, and this is none")
		return ExitMissing, nil
	}
	// Where itos is set up, a push git would make without itos's pre-push
	// hook is refused before anything is fetched (slice 91).
	if exists(config.Path()) {
		if err := hooksReady("pre-push"); err != nil {
			if o.JSON {
				fmt.Fprintf(o.Stderr, "itos: %s\n", err)
			}
			return r.report(ExitMissing, "hooks-not-run", "itos: "+err.Error())
		}
	}
	if code, done, err := r.ready(); done {
		return code, err
	}
	branch := git.Branch()
	if branch == "" {
		return r.report(ExitPolicy, "detached", "itos push: HEAD is not on a branch; check out the branch to push first")
	}
	remote, ref, set := git.Upstream(branch)
	r.remote, r.branch = remote, strings.TrimPrefix(ref, "refs/heads/")
	if !git.Succeeds("rev-parse", "--verify", "--quiet", "HEAD") {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s has no commit yet.", branch))
	}
	if !set && !git.Succeeds("remote", "get-url", remote) {
		return r.report(ExitMissing, "no-remote", fmt.Sprintf("itos push: %s has no upstream and there is no remote %s to push to", branch, remote))
	}
	onto, exists, failure, failed := r.fetch(remote, ref)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("fetching "+r.upstream()+" failed", failure)...)
	}
	if exists && !git.Succeeds("merge-base", "--is-ancestor", onto, "HEAD") {
		if code, done, err := r.rebase(onto); done {
			return code, err
		}
	}
	sha, err := git.Output("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: cannot resolve HEAD after the rebase: %s\n", err)
		return ExitMissing, nil
	}
	r.sha = strings.TrimSpace(sha)
	if exists {
		ahead, _ := git.Output("rev-list", "--count", onto+".."+r.sha)
		if strings.TrimSpace(ahead) == "0" {
			return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s is at %s.", branch, r.upstream()))
		}
	}
	return r.push(remote, ref, onto)
}

// ready is whether the working tree can be pulled into: no rebase in
// progress, no conflict left and no uncommitted change to a tracked file;
// done when it cannot, with the refusal reported.
func (r pushRun) ready() (int, bool, error) {
	if git.Rebasing() || len(git.Conflicted()) > 0 {
		code, err := r.report(ExitPolicy, "rebase-in-progress",
			"itos push: a rebase is in progress or a conflict is left; nothing was pulled or pushed.",
			"Resolve it and run git rebase --continue, or run git rebase --abort, then itos push again.")
		return code, true, err
	}
	if changed := git.Changed(); len(changed) > 0 {
		lines := []string{"itos push: tracked files have uncommitted changes; commit or stash them first, then run itos push again:"}
		for _, l := range changed {
			lines = append(lines, "  "+l)
		}
		code, err := r.report(ExitPolicy, "uncommitted", lines...)
		return code, true, err
	}
	return 0, false, nil
}

// fetch fetches the upstream's branch and gives the commit fetched; exists
// is false when the remote has no such branch yet, which a push creates.
// git's own words are printed only when the fetch fails, and failure is then
// their kind (git.RemoteFailure). A remote out of reach, or no repository, is
// not asked again whether it has the branch: the answer would be the same
// failure, after a second wait for a connection that times out.
func (r pushRun) fetch(remote, ref string) (onto string, exists bool, failure kind.Kind, failed bool) {
	var stderr bytes.Buffer
	code, err := r.run(&stderr, "fetch", "--quiet", "--no-tags", remote, ref)
	if err == nil && code == 0 {
		sha, err := git.Output("rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
		if err == nil {
			return strings.TrimSpace(sha), true, kind.Unknown, false
		}
	}
	failure = git.RemoteFailure(stderr.String())
	// A remote without the branch says so with ls-remote's exit 2.
	if failure == kind.Unknown {
		if lsCode, _ := r.run(io.Discard, "ls-remote", "--exit-code", remote, ref); lsCode == 2 {
			return "", false, kind.Unknown, false
		}
	}
	r.o.Stderr.Write(stderr.Bytes())
	return "", false, failure, true
}

// remoteExit is the exit code of a fetch's or a push's failure of the kind:
// 75 for a remote out of reach, 3 for one that is no repository, 70 for a
// failure of no kind (ExitCode).
func remoteExit(failure kind.Kind) int {
	return ExitCode(kind.Wrap(failure, errors.New("git failed")))
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
func (r pushRun) rebase(onto string) (int, bool, error) {
	if config.IsStealth(config.Path()) {
		if err := rewriteNotes(); err != nil {
			fmt.Fprintf(r.o.Stderr, "itos: cannot set notes.rewriteRef: %s\n", err)
			return ExitMissing, true, nil
		}
	}
	argv := []string{"rebase", "--no-autostash"}
	if r.o.Quiet {
		argv = append(argv, "--quiet")
	}
	code, err := r.run(r.o.Stderr, append(argv, onto)...)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, true, nil
	}
	if git.Rebasing() || len(git.Conflicted()) > 0 {
		lines := []string{
			fmt.Sprintf("itos push: the rebase onto %s stopped; nothing was pushed.", r.upstream()),
			"Resolve the conflicts, git add the files and run git rebase --continue, then itos push again;",
			"or run git rebase --abort to go back to where the branch was.",
		}
		code, err := r.report(ExitPolicy, "rebase-stopped", append(lines, r.registryConflict()...)...)
		return code, true, err
	}
	if code != 0 {
		code, err := r.report(ExitPolicy, "rebase-failed",
			fmt.Sprintf("itos push: the rebase onto %s did not run (git's message above); nothing was pushed", r.upstream()))
		return code, true, err
	}
	return 0, false, nil
}

// registryConflict is what the push says, in a person's words, when its
// rebase stopped on a conflict in the work registry (slice 66), nothing when
// it did not or there is no config to name the registry: two people took the
// same item, or changed it, and the upstream's commit landed first, so the
// conflict is the lock and the remote keeps the first take. It names the
// file and each item both sides changed, the upstream's owner of one it
// took, and how to give the item up (git rebase --skip drops the commit
// being replayed, a registry command's being the registry alone).
func (r pushRun) registryConflict() []string {
	file := config.Path()
	if _, err := os.Stat(file); err != nil {
		return nil
	}
	cfg, err := config.Load(file)
	if err != nil {
		return nil
	}
	registry := path.Clean(filepath.ToSlash(cfg.Work.Registry))
	hit := false
	for _, f := range git.Conflicted() {
		hit = hit || path.Clean(f) == registry
	}
	if !hit {
		return nil
	}
	lines := []string{"", fmt.Sprintf("The conflict is in the work registry, %s: a commit on %s changed", registry, r.upstream()),
		"the same item as yours, so it was taken, or changed, there first, and the remote keeps that."}
	show := func(rev string) string {
		text, _ := git.Output("show", rev+":"+registry)
		return text
	}
	for _, c := range work.Clashes(show("REBASE_HEAD^"), show("HEAD"), show("REBASE_HEAD")) {
		switch {
		case c.Taken && c.Owner != "":
			lines = append(lines, fmt.Sprintf("  %s is taken: a commit on %s made %s its owner first.", c.ID, r.upstream(), c.Owner))
		default:
			lines = append(lines, fmt.Sprintf("  %s was changed on %s too.", c.ID, r.upstream()))
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
func (r pushRun) push(remote, ref, onto string) (int, error) {
	argv := []string{"push"}
	if r.o.Quiet {
		argv = append(argv, "--quiet")
	}
	var stderr bytes.Buffer
	code, err := r.run(io.MultiWriter(r.o.Stderr, &stderr), append(argv, remote, r.sha+":"+ref)...)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
	}
	if code != 0 {
		words := stderr.String()
		if failure := git.RemoteFailure(words); failure != kind.Unknown {
			return r.report(remoteExit(failure), "push-failed", r.remoteLines("the push to "+r.upstream()+" failed", failure)...)
		}
		exit := ExitSoftware
		if git.Refused(words) {
			exit = ExitPolicy
		}
		return r.report(exit, "push-failed",
			fmt.Sprintf("itos push: the push to %s failed (git's message above); nothing was forced.", r.upstream()),
			"If the remote moved, run itos push again to rebase onto it; if a hook refused it, fix what it reported first.")
	}
	short, _ := git.Output("rev-parse", "--short", r.sha)
	pushed := fmt.Sprintf("Pushed %s to %s.", strings.TrimSpace(short), r.upstream())
	if r.noWait {
		return r.report(0, "pushed", pushed)
	}
	return r.wait(remote, onto, pushed)
}

// wait waits for the CI run of the commit pushed when ci.watch has a
// provider, and exits with the run's result; with no config, or none, it
// reports the push as before. The commits are pushed whatever the run says.
// A config that cannot be read leaves the push as it was, saying why
// nothing was watched. A push whose commits touch only the work registry
// is not waited for either (slice 56): itos wrote and checked those
// commits, an item's take and its close, and a run for each made one item
// cost three waits; it says how to wait for that run all the same.
func (r *pushRun) wait(remote, onto, pushed string) (int, error) {
	file := config.Path()
	if _, err := os.Stat(file); err != nil {
		return r.report(0, "pushed", pushed)
	}
	cfg, err := config.Load(file)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: no CI run watched, the config cannot be read: %s\n", err)
		return r.report(0, "pushed", pushed)
	}
	sha := r.sha
	if cfg.CI.Watch.Provider != "none" && registryOnly(onto, sha, cfg.Work.Registry) {
		return r.report(0, "pushed", pushed,
			fmt.Sprintf("Its commits touch only %s, so its CI run is not waited for; itos ci watch %s waits for it.", cfg.Work.Registry, sha))
	}
	wr, ok, err := watcher(cfg, remote, r.o)
	if !ok && err == nil {
		return r.report(0, "pushed", pushed)
	}
	if !r.o.JSON && !r.o.Quiet {
		fmt.Fprintln(r.o.Stdout, pushed)
	}
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: %s; the commits are pushed, and itos ci watch %s waits for their run\n", err, sha)
		r.watched = &watched{code: ExitMissing, outcome: "error"}
	} else {
		w := watchRun(cfg, wr, sha, remote, r.o)
		r.watched = &w
	}
	if r.o.JSON {
		return r.report(r.watched.code, "pushed")
	}
	return r.watched.code, nil
}

// registryOnly is whether the commits the push added to the upstream's
// branch, onto..pushed, touch the work registry and no other path
// (touchesOnlyRegistry). A push that makes the branch has no onto, and is
// never registry-only: what it adds is not known without the remote's other
// branches.
func registryOnly(onto, pushed, registry string) bool {
	if onto == "" {
		return false
	}
	return touchesOnlyRegistry(registry, onto+".."+pushed)
}

// touchesOnlyRegistry is whether the commits git log gives for the revisions
// touch the work registry and no other path: the one rule by which itos push
// does not wait for a run (slice 56) and work done passes over a commit
// (slice 93), since such a commit changes nothing a run judges. A merge counts
// what it brought in against its first parent, and a commit that touches no
// path adds nothing to the answer, so commits that touch nothing at all are
// not registry-only.
func touchesOnlyRegistry(registry string, revs ...string) bool {
	args := append([]string{"log", "--format=", "--name-only", "--no-renames", "--diff-merges=first-parent"}, revs...)
	files, err := git.Paths(args...)
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

// run runs git, its stdout the run's, its stderr to stderr, and gives its
// exit code.
func (r pushRun) run(stderr io.Writer, argv ...string) (int, error) {
	return runGit(argv, os.Environ(), r.git, Out{Stderr: stderr})
}
