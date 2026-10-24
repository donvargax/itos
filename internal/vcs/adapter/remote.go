package adapter

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Upstream is branch.<name>.remote and .merge, else origin and
// refs/heads/<name>.
func (g Git) Upstream(_ context.Context, branch string) (string, string, bool) {
	return g.git().Upstream(branch)
}

// Remotes is git remote, sorted.
func (g Git) Remotes(context.Context) ([]string, error) {
	out, err := g.git().Output("remote")
	if err != nil {
		return nil, err
	}
	remotes := strings.Fields(out)
	slices.Sort(remotes)
	return remotes, nil
}

// RemoteURL is git remote get-url.
func (g Git) RemoteURL(_ context.Context, remote string) (string, error) {
	out, err := g.git().Output("remote", "get-url", remote)
	if err != nil {
		return "", errors.New("no remote " + remote)
	}
	return strings.TrimSpace(out), nil
}

// RemoteRefs is git ls-remote, the refs given as its patterns.
func (g Git) RemoteRefs(ctx context.Context, remote string, refs []string, talk port.Talk) (map[string]string, error) {
	out, err := g.output(ctx, talk, true, append([]string{"ls-remote", remote}, refs...)...)
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			found[fields[1]] = fields[0]
		}
	}
	return found, nil
}

// RemoteHead is the branch the remote's HEAD names, asked of the remote
// itself with git ls-remote --symref, since a clone's refs/remotes/<remote>/HEAD
// is missing after git remote add; "" when the remote answers without a HEAD
// naming a branch. Not in the VCS port: only itos push in pr mode asks it.
func (g Git) RemoteHead(ctx context.Context, remote string, talk port.Talk) (string, error) {
	out, err := g.output(ctx, talk, true, "ls-remote", "--symref", remote, "HEAD")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if target, ok := strings.CutPrefix(line, "ref: "); ok {
			if ref, name, _ := strings.Cut(target, "\t"); name == "HEAD" {
				return strings.TrimPrefix(ref, "refs/heads/"), nil
			}
		}
	}
	return "", nil
}

// Fetch is git fetch --quiet --no-tags of the refspec, then FETCH_HEAD's
// commit; git fetch -q of the remote's own refspecs when refspec is "".
func (g Git) Fetch(ctx context.Context, remote, refspec string, talk port.Talk) (string, error) {
	if refspec == "" {
		return "", g.run(ctx, talk, nil, true, "fetch", "-q", remote)
	}
	if err := g.run(ctx, talk, nil, true, "fetch", "--quiet", "--no-tags", remote, refspec); err != nil {
		return "", err
	}
	sha, err := g.output(ctx, port.Talk{Dir: talk.Dir}, false, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	return strings.TrimSpace(sha), err
}

// Push is git push --no-follow-tags.
func (g Git) Push(ctx context.Context, remote string, specs []string, p port.Pushing, talk port.Talk) error {
	args := []string{"push"}
	if p.Atomic {
		args = append(args, "--atomic")
	}
	args = append(args, "--no-follow-tags")
	if p.Quiet {
		args = append(args, "--quiet")
	}
	if p.NoVerify {
		args = append(args, "--no-verify")
	}
	return g.run(ctx, talk, nil, true, append(append(args, remote), specs...)...)
}

// Rebase is git rebase --no-autostash, with git's hint to skip a commit on
// a stop left out (bug 118: --skip drops the commit being replayed).
func (g Git) Rebase(ctx context.Context, onto string, quiet bool, talk port.Talk) error {
	args := []string{"-c", "advice.mergeConflict=false", "rebase", "--no-autostash"}
	if quiet {
		args = append(args, "--quiet")
	}
	return g.run(ctx, talk, nil, false, append(args, onto)...)
}
