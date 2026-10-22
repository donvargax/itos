package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/engine/gitrun"
	"github.com/donvargax/itos/v8/internal/kind"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// namedRemote is the remote --tag and --ref push to: the current branch's
// upstream's, else origin.
func (r pushRun) namedRemote(ctx context.Context) string {
	v := r.ports
	if branch := v.Branch(ctx); branch != "" {
		remote, _, _ := v.Upstream(ctx, branch)
		return remote
	}
	return "origin"
}

// pushNamed pushes the tags and the branches, each to the remote's ref of
// the same name, in one atomic push.
func (r pushRun) pushNamed(ctx context.Context, tags, refs []string) Answer {
	r.remote, r.tags, r.refs = r.namedRemote(ctx), tags, refs
	v := r.ports
	local := map[string]string{}
	var named []string
	for _, t := range tags {
		named = append(named, "refs/tags/"+t)
	}
	for _, b := range refs {
		named = append(named, "refs/heads/"+b)
	}
	for _, ref := range named {
		sha, err := v.Ref(ctx, ref)
		if err != nil {
			return r.report(policy, "no-such-ref",
				fmt.Sprintf("itos push: this clone has no %s; nothing was pushed", strings.TrimPrefix(ref, "refs/")))
		}
		local[ref] = sha
	}
	theirs, failure, failed := r.remoteTags(ctx, tags)
	if failed {
		return r.report(remoteExit(failure), "fetch-failed", r.remoteLines("reading "+r.remote+"'s tags failed", failure)...)
	}
	var specs []string
	for _, ref := range named {
		switch at := theirs[ref]; {
		case at == "":
			specs = append(specs, ref+":"+ref)
		case at != local[ref]:
			return r.report(policy, "tag-exists",
				fmt.Sprintf("itos push: %s already has the tag %s, at %s, not this clone's %s; a tag is never moved, "+
					"so nothing was pushed.", r.remote, strings.TrimPrefix(ref, "refs/tags/"), short(at), short(local[ref])),
				"Release under a new tag, or delete this clone's and fetch the remote's.")
		}
	}
	if len(specs) == 0 {
		return r.report(0, "nothing-to-push", fmt.Sprintf("Nothing to push: %s has every tag named.", r.remote))
	}
	outcome := v.Push(ctx, r.remote, specs, vcsport.Pushing{Atomic: true, Quiet: r.o.Quiet}, r.talk(r.o.Stderr))
	code, err := gitrun.ExitOf(outcome)
	if err != nil {
		fmt.Fprintf(r.o.Stderr, "itos: cannot run git: %s\n", err)
		return Answer{Code: missing}
	}
	what := strings.Join(namesOf(specs), ", ")
	if code != 0 {
		if failure := remoteKind(outcome); failure != kind.Unknown {
			return r.report(remoteExit(failure), "push-failed", r.remoteLines("the push of "+what+" failed", failure)...)
		}
		return r.report(refusalExit(outcome), "push-failed",
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
func (r pushRun) remoteTags(ctx context.Context, tags []string) (map[string]string, kind.Kind, bool) {
	if len(tags) == 0 {
		return map[string]string{}, kind.Unknown, false
	}
	var refs []string
	for _, t := range tags {
		refs = append(refs, "refs/tags/"+t)
	}
	theirs, err := r.ports.RemoteRefs(ctx, r.remote, refs, gitrun.Talking(r.origin, nil, io.Discard, nil))
	if err != nil {
		io.WriteString(r.o.Stderr, words(err))
		return nil, remoteKind(err), true
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
func refusalExit(err error) int {
	var failed *vcsport.Failed
	if errors.As(err, &failed) && failed.Failure == vcsport.Refused {
		return policy
	}
	return software
}
