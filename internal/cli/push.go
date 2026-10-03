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
// HEAD to the upstream's branch, an explicit refspec, so the hooks run as
// for any push, nothing else goes with it (refs/notes/itos stays local in
// the stealth mode), and nothing forces it: a force flag or a + refspec is
// a usage error, and a push the remote refuses is reported with git's exit
// code, never retried.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/out"
)

// readPushArgs refuses every argument: itos push pushes HEAD to the
// branch's upstream and takes nothing else, and an argument that would
// force the push says why it is refused.
func readPushArgs(args []string) error {
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
		return usage("push takes no arguments: it pushes HEAD to the branch's upstream (%s)", args[0])
	}
	return nil
}

// pushRun is one itos push: where its report goes and what it says under
// --json.
type pushRun struct {
	o              Out
	git            io.Writer // where git's stdout goes: stderr under --json
	remote, branch string
}

// report ends the run: under --json the outcome on stdout, else the lines
// on stdout for a success, on stderr for anything else.
func (r pushRun) report(code int, outcome string, lines ...string) (int, error) {
	if r.o.JSON {
		fields := []out.Field{{Key: "ok", Value: code == 0}, {Key: "outcome", Value: outcome}}
		if r.remote != "" {
			fields = append(fields, out.Field{Key: "remote", Value: r.remote}, out.Field{Key: "branch", Value: r.branch})
		}
		if outcome == "pushed" {
			if sha, err := git.Output("rev-parse", "HEAD"); err == nil {
				fields = append(fields, out.Field{Key: "commit", Value: strings.TrimSpace(sha)})
			}
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
	if err := readPushArgs(args); err != nil {
		return 0, err
	}
	r := pushRun{o: o, git: o.Stdout}
	if o.JSON {
		r.git = o.Stderr
	}
	if !git.Succeeds("rev-parse", "--git-dir") {
		fmt.Fprintln(o.Stderr, "itos: push runs in a git repository, and this is none")
		return ExitMissing, nil
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
	onto, exists, code := r.fetch(remote, ref)
	if code != 0 {
		return r.report(code, "fetch-failed", fmt.Sprintf("itos push: fetching %s failed (git's message above); nothing was pushed", r.upstream()))
	}
	if exists {
		if !git.Succeeds("merge-base", "--is-ancestor", onto, "HEAD") {
			if code, done, err := r.rebase(onto); done {
				return code, err
			}
		}
		ahead, _ := git.Output("rev-list", "--count", onto+"..HEAD")
		if strings.TrimSpace(ahead) == "0" {
			return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s is at %s.", branch, r.upstream()))
		}
	}
	return r.push(remote, ref)
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
// git's own words are printed only when the fetch fails, and code is then
// git's exit code.
func (r pushRun) fetch(remote, ref string) (onto string, exists bool, code int) {
	var stderr bytes.Buffer
	code, err := r.run(&stderr, "fetch", "--quiet", "--no-tags", remote, ref)
	if err == nil && code == 0 {
		sha, err := git.Output("rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
		if err == nil {
			return strings.TrimSpace(sha), true, 0
		}
	}
	// A remote without the branch says so with ls-remote's exit 2.
	if lsCode, _ := r.run(io.Discard, "ls-remote", "--exit-code", remote, ref); lsCode == 2 {
		return "", false, 0
	}
	r.o.Stderr.Write(stderr.Bytes())
	if code == 0 {
		code = ExitPolicy
	}
	return "", false, code
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
		code, err := r.report(ExitPolicy, "rebase-stopped",
			fmt.Sprintf("itos push: the rebase onto %s stopped; nothing was pushed.", r.upstream()),
			"Resolve the conflicts, git add the files and run git rebase --continue, then itos push again;",
			"or run git rebase --abort to go back to where the branch was.")
		return code, true, err
	}
	if code != 0 {
		code, err := r.report(ExitPolicy, "rebase-failed",
			fmt.Sprintf("itos push: the rebase onto %s did not run (git's message above); nothing was pushed", r.upstream()))
		return code, true, err
	}
	return 0, false, nil
}

// push pushes HEAD to the upstream's branch: the pre-push hook runs, and a
// refusal is reported with git's exit code.
func (r pushRun) push(remote, ref string) (int, error) {
	argv := []string{"push"}
	if r.o.Quiet {
		argv = append(argv, "--quiet")
	}
	code, err := r.run(r.o.Stderr, append(argv, remote, "HEAD:"+ref)...)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
	}
	if code != 0 {
		return r.report(code, "push-failed",
			fmt.Sprintf("itos push: the push to %s failed (git's message above); nothing was forced.", r.upstream()),
			"If the remote moved, run itos push again to rebase onto it; if a hook refused it, fix what it reported first.")
	}
	short, _ := git.Output("rev-parse", "--short", "HEAD")
	return r.report(0, "pushed", fmt.Sprintf("Pushed %s to %s.", strings.TrimSpace(short), r.upstream()))
}

// run runs git, its stdout the run's, its stderr to stderr, and gives its
// exit code.
func (r pushRun) run(stderr io.Writer, argv ...string) (int, error) {
	return runGit(argv, os.Environ(), r.git, Out{Stderr: stderr})
}
