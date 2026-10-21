package cli

// itos push --tag <tag> and --ref <branch> (slice 131, features/push.feature):
// a release's tags, or a branch for another platform's CI, pushed through
// itos rather than a git that skips the shim's checks. They push what they
// name and nothing else, in one git push --atomic, so a release of several
// tags, some on commits HEAD does not reach, lands whole or not at all. No
// rebase runs and nothing of HEAD goes: the working tree is not read, so
// uncommitted changes are no reason to refuse; and no CI run is waited for,
// --no-wait or not, since ci.watch watches the branch's push.
//
// The remote is the current branch's upstream's, origin with none or on a
// detached HEAD. Each tag goes to the remote's tag of its name, each branch
// to the remote's branch of its name, never forced. A tag the remote already
// has as another object is refused before anything is pushed, so the
// pre-push hook's commands do not run for a push git would refuse: a tag is
// never moved. One it has as the same object is left out, and a push with
// nothing left is nothing to push. A branch the remote moved is git's to
// refuse, or the pre-push hook's (rewrites). Failures exit by their kind, as
// the plain push's (slice 90).

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/kind"
)

// namedRemote is the remote --tag and --ref push to: the current branch's
// upstream's, else origin.
func namedRemote() string {
	if branch := git.Branch(); branch != "" {
		remote, _, _ := git.Upstream(branch)
		return remote
	}
	return "origin"
}

// pushNamed pushes the tags and the branches, each to the remote's ref of
// the same name, in one atomic push.
func (r pushRun) pushNamed(tags, refs []string) (int, error) {
	r.remote, r.tags, r.refs = namedRemote(), tags, refs
	local := map[string]string{}
	var named []string
	for _, t := range tags {
		named = append(named, "refs/tags/"+t)
	}
	for _, b := range refs {
		named = append(named, "refs/heads/"+b)
	}
	for _, ref := range named {
		sha, err := git.Output("rev-parse", "--verify", "--quiet", ref)
		if err != nil {
			return r.report(ExitPolicy, "no-such-ref",
				fmt.Sprintf("itos push: this clone has no %s; nothing was pushed", strings.TrimPrefix(ref, "refs/")))
		}
		local[ref] = strings.TrimSpace(sha)
	}
	theirs, failure, failed := r.remoteTags(tags)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("reading "+r.remote+"'s tags failed", failure)...)
	}
	var specs []string
	for _, ref := range named {
		switch at := theirs[ref]; {
		case at == "":
			specs = append(specs, ref+":"+ref)
		case at != local[ref]:
			return r.report(ExitPolicy, "tag-exists",
				fmt.Sprintf("itos push: %s already has the tag %s, at %s, not this clone's %s; a tag is never moved, "+
					"so nothing was pushed.", r.remote, strings.TrimPrefix(ref, "refs/tags/"), short(at), short(local[ref])),
				"Release under a new tag, or delete this clone's and fetch the remote's.")
		}
	}
	if len(specs) == 0 {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s has every tag named.", r.remote))
	}
	argv := []string{"push", "--atomic", "--no-follow-tags"}
	if r.o.Quiet {
		argv = append(argv, "--quiet")
	}
	var stderr bytes.Buffer
	code, err := r.run(io.MultiWriter(r.o.Stderr, &stderr), append(append(argv, r.remote), specs...)...)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return ExitMissing, nil
	}
	what := strings.Join(namesOf(specs), ", ")
	if code != 0 {
		words := stderr.String()
		if failure := git.RemoteFailure(words); failure != kind.Unknown {
			return r.report(remoteExit(failure), "push-failed", r.remoteLines("the push of "+what+" failed", failure)...)
		}
		return r.report(refusalExit(words), "push-failed",
			fmt.Sprintf("itos push: the push of %s to %s failed (git's message above); nothing was forced, and none "+
				"of them landed.", what, r.remote),
			"If the remote has commits a branch lacks, rebase the branch onto them first; if a hook refused the push, "+
				"fix what it reported.")
	}
	return r.report(0, "pushed", fmt.Sprintf("Pushed %s to %s.", what, r.remote))
}

// remoteTags are the remote's refs of the tags, each ref to the object it
// names there; a tag the remote does not have is not among them. failed when
// the remote cannot be read, failure then its kind.
func (r pushRun) remoteTags(tags []string) (map[string]string, kind.Kind, bool) {
	theirs := map[string]string{}
	if len(tags) == 0 {
		return theirs, kind.Unknown, false
	}
	argv := []string{"ls-remote", "--tags", r.remote}
	for _, t := range tags {
		argv = append(argv, "refs/tags/"+t)
	}
	var stdout, stderr bytes.Buffer
	code, err := runGit(argv, os.Environ(), &stdout, Out{Stderr: &stderr, Origin: r.o.Origin})
	if err != nil || code != 0 {
		r.o.Stderr.Write(stderr.Bytes())
		return nil, git.RemoteFailure(stderr.String()), true
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			theirs[fields[1]] = fields[0]
		}
	}
	return theirs, kind.Unknown, false
}

// namesOf are the refspecs' refs as a person names them: tag v1.0.0, branch
// ci/windows.
func namesOf(specs []string) []string {
	var names []string
	for _, spec := range specs {
		ref, _, _ := strings.Cut(spec, ":")
		if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
			names = append(names, "tag "+tag)
		} else {
			names = append(names, "branch "+strings.TrimPrefix(ref, "refs/heads/"))
		}
	}
	return names
}

// refusalExit is the exit code of a push git failed that reached no remote
// failure: 1 when the remote or a hook refused it, else 70.
func refusalExit(words string) int {
	if git.Refused(words) {
		return ExitPolicy
	}
	return ExitSoftware
}
