package adapter

import (
	"context"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// CommitPaths is git diff-tree --root, which lists nothing of a merge.
func (g Git) CommitPaths(_ context.Context, rev string) ([]string, error) {
	return g.git().CommitPaths(rev)
}

func (g Git) OwnPaths(_ context.Context, rev string) ([]string, error) { return g.git().OwnPaths(rev) }

func (g Git) Diff(_ context.Context, from, to string) ([]string, error) {
	return g.git().Paths("diff", "--name-only", "--no-renames", from, to, "--")
}

// Touched is git log --name-only, which lists a merge's paths against its
// first parent only when asked to.
func (g Git) Touched(_ context.Context, r port.Range, merges bool) ([]string, error) {
	args := []string{"log", "--format=", "--name-only", "--no-renames"}
	if merges {
		args = append(args, "--diff-merges=first-parent")
	}
	lines, err := g.git().Paths(append(append(args, revs(r)...), "--")...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range lines {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (g Git) Staged(_ context.Context, base string) ([]string, error) {
	args := []string{"diff", "--cached", "--name-only", "--no-renames"}
	if base != "" {
		args = append(args, base)
	}
	return g.git().Paths(append(args, "--")...)
}

func (g Git) StagedOwnPaths(_ context.Context, parents []string) ([]string, error) {
	return g.git().StagedOwnPaths(parents)
}

func (g Git) Changes(context.Context) []string { return g.git().Changed() }

// PathStatus is git status --porcelain, every untracked file listed; its
// error is a port.Failed holding git's words when git fails, git's own when
// it does not run.
func (g Git) PathStatus(ctx context.Context, path string) ([]string, error) {
	out, err := g.output(ctx, port.Talk{}, false, "status", "--porcelain", "--untracked-files=all", "--", path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func (g Git) Tracked(_ context.Context, path string) bool {
	return g.git().Succeeds("ls-files", "--error-unmatch", "--", path)
}

func (g Git) Conflicted(context.Context) []string { return g.git().Conflicted() }

func (g Git) Rebasing(context.Context) bool { return g.git().Rebasing() }

func (g Git) MergeHeads(context.Context) []string { return g.git().MergeHeads() }

func (g Git) Branch(context.Context) string { return g.git().Branch() }
