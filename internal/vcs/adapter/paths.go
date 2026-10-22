package adapter

import (
	"context"
	"strings"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// CommitPaths is git diff-tree --root, which lists nothing of a merge.
func (Git) CommitPaths(_ context.Context, rev string) ([]string, error) { return git.CommitPaths(rev) }

func (Git) OwnPaths(_ context.Context, rev string) ([]string, error) { return git.OwnPaths(rev) }

func (Git) Diff(_ context.Context, from, to string) ([]string, error) {
	return git.Paths("diff", "--name-only", "--no-renames", from, to, "--")
}

// Touched is git log --name-only, which lists a merge's paths against its
// first parent only when asked to.
func (Git) Touched(_ context.Context, r port.Range, merges bool) ([]string, error) {
	args := []string{"log", "--format=", "--name-only", "--no-renames"}
	if merges {
		args = append(args, "--diff-merges=first-parent")
	}
	lines, err := git.Paths(append(append(args, revs(r)...), "--")...)
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

func (Git) Staged(_ context.Context, base string) ([]string, error) {
	args := []string{"diff", "--cached", "--name-only", "--no-renames"}
	if base != "" {
		args = append(args, base)
	}
	return git.Paths(append(args, "--")...)
}

func (Git) StagedOwnPaths(_ context.Context, parents []string) ([]string, error) {
	return git.StagedOwnPaths(parents)
}

func (Git) Changes(context.Context) []string { return git.Changed() }

// PathStatus is git status --porcelain, every untracked file listed; its
// error is git's, worded as git.Read words it.
func (Git) PathStatus(_ context.Context, path string) ([]string, error) {
	out, err := git.Read("status", "--porcelain", "--untracked-files=all", "--", path)
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

func (Git) Tracked(_ context.Context, path string) bool {
	return git.Succeeds("ls-files", "--error-unmatch", "--", path)
}

func (Git) Conflicted(context.Context) []string { return git.Conflicted() }

func (Git) Rebasing(context.Context) bool { return git.Rebasing() }

func (Git) MergeHeads(context.Context) []string { return git.MergeHeads() }

func (Git) Branch(context.Context) string { return git.Branch() }
