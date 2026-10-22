package porttest

import (
	"context"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// The broken fake's Show, Log, changed paths and work tree's state.

// entry is an entry as the show faults break it, Show's and Log's alike.
func (b broken) entry(e port.Entry) port.Entry {
	switch {
	case b.fault == "show subject as first line":
		e.Subject, _, _ = strings.Cut(e.Message, "\n")
	case b.fault == "show message trimmed":
		e.Message = strings.TrimRight(e.Message, "\n")
	case b.fault == "show note of another ref" && e.SHA == "c-b":
		e.Note = "elsewhere\n"
	case b.fault == "show no note":
		e.Note = ""
	case b.fault == "show no author":
		e.Author, e.Email, e.Date = "", "", ""
	case b.fault == "show merge never":
		e.Merge = false
	case b.fault == "show short whole":
		e.Short = e.SHA
	}
	return e
}

func (b broken) Show(ctx context.Context, rev string) (port.Entry, error) {
	e, err := b.VCS.Show(ctx, rev)
	if b.fault == "show of nothing" && err != nil {
		return port.Entry{}, nil
	}
	return b.entry(e), err
}

func (b broken) Log(ctx context.Context, r port.Range) ([]port.Entry, error) {
	switch b.fault {
	case "log since kept":
		r.Since = ""
	case "log of nothing":
		if !b.VCS.Readable(ctx, r) {
			return nil, nil
		}
	}
	found, err := b.VCS.Log(ctx, r)
	for i := range found {
		found[i] = b.entry(found[i])
		if b.fault == "log without notes" {
			found[i].Note = ""
		}
	}
	if b.fault == "log newest first" {
		slices.Reverse(found)
	}
	return found, err
}

func (b broken) CommentChar(ctx context.Context) string {
	switch {
	case b.fault == "comment char always #":
		return "#"
	case b.fault == "comment char unset empty" && b.Comment == "":
		return ""
	}
	return b.VCS.CommentChar(ctx)
}

func (b broken) CommitPaths(ctx context.Context, rev string) ([]string, error) {
	parents, err := b.VCS.Parents(ctx, rev)
	switch {
	case b.fault == "commit paths of a root none" && err == nil && len(parents) == 0:
		return nil, nil
	case b.fault == "commit paths of a merge against its first parent" && len(parents) > 1:
		return b.VCS.Diff(ctx, parents[0], rev)
	case b.fault == "commit paths of nothing" && err != nil:
		return nil, nil
	}
	return b.VCS.CommitPaths(ctx, rev)
}

func (b broken) OwnPaths(ctx context.Context, rev string) ([]string, error) {
	parents, err := b.VCS.Parents(ctx, rev)
	switch {
	case b.fault == "own paths against the first parent" && len(parents) > 1:
		return b.VCS.Diff(ctx, parents[0], rev)
	case b.fault == "own paths of one parent" && len(parents) == 1:
		return b.VCS.CommitPaths(ctx, rev)
	case b.fault == "own paths of nothing" && err != nil:
		return nil, nil
	}
	return b.VCS.OwnPaths(ctx, rev)
}

func (b broken) Diff(ctx context.Context, from, to string) ([]string, error) {
	paths, err := b.VCS.Diff(ctx, from, to)
	switch {
	case b.fault == "diff from the empty tree fails" && from == port.EmptyTree:
		return nil, errBroken
	case b.fault == "diff without deletions":
		tree, _ := b.tree(to)
		paths = slices.DeleteFunc(paths, func(p string) bool { _, ok := tree[p]; return !ok })
	case b.fault == "diff of nothing" && err != nil:
		return nil, nil
	}
	return paths, err
}

func (b broken) Touched(ctx context.Context, r port.Range, merges bool) ([]string, error) {
	switch b.fault {
	case "touched merges always":
		merges = true
	case "touched merges never":
		merges = false
	case "touched of nothing":
		if !b.VCS.Readable(ctx, r) {
			return nil, nil
		}
	}
	paths, err := b.VCS.Touched(ctx, r, merges)
	if b.fault == "touched oldest first" {
		slices.Reverse(paths)
	}
	return paths, err
}

func (b broken) Staged(ctx context.Context, base string) ([]string, error) {
	_, ok := b.tree(base)
	switch {
	case b.fault == "staged against HEAD always" && ok:
		base = ""
	case b.fault == "staged against the empty tree" && base == "":
		base = port.EmptyTree
	case b.fault == "staged of nothing" && !ok && base != "":
		return nil, nil
	}
	return b.VCS.Staged(ctx, base)
}

func (b broken) StagedOwnPaths(ctx context.Context, parents []string) ([]string, error) {
	paths, err := b.VCS.StagedOwnPaths(ctx, parents)
	switch {
	case b.fault == "staged own as staged" && err == nil:
		return b.VCS.Staged(ctx, "")
	case b.fault == "staged own of nothing" && err != nil:
		return paths, nil
	}
	return paths, err
}

func (b broken) Changes(ctx context.Context) []string {
	lines := b.VCS.Changes(ctx)
	switch {
	case b.fault == "changes unstaged only":
		lines = slices.DeleteFunc(lines, func(l string) bool { return l[0] != ' ' })
	case b.fault == "changes untracked listed" && b.Files != nil:
		lines = append(lines, "?? u.txt")
	case b.fault == "changes of a clean tree" && len(lines) == 0:
		lines = []string{"M  a.txt"}
	}
	return lines
}

func (b broken) PathStatus(ctx context.Context, path string) ([]string, error) {
	lines, err := b.VCS.PathStatus(ctx, path)
	switch b.fault {
	case "path status untracked hidden":
		lines = slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, "??") })
	case "path status of every path":
		lines = b.VCS.Changes(ctx)
	}
	return lines, err
}

func (b broken) Tracked(ctx context.Context, path string) bool {
	switch b.fault {
	case "tracked by the work tree":
		_, ok := b.files()[path]
		return ok
	case "tracked by HEAD":
		_, ok := b.head()[path]
		return ok
	}
	return b.VCS.Tracked(ctx, path)
}

func (b broken) Conflicted(ctx context.Context) []string {
	if b.fault == "conflicted none" {
		return nil
	}
	return b.VCS.Conflicted(ctx)
}

func (b broken) Rebasing(ctx context.Context) bool {
	switch b.fault {
	case "rebasing never":
		return false
	case "rebasing when conflicted":
		return len(b.Unmerged) > 0
	}
	return b.VCS.Rebasing(ctx)
}

func (b broken) MergeHeads(ctx context.Context) []string {
	switch {
	case b.fault == "merge heads none":
		return nil
	case b.fault == "merge heads in a rebase" && b.InRebase:
		return []string{"c-t"}
	}
	return b.VCS.MergeHeads(ctx)
}

func (b broken) Branch(ctx context.Context) string {
	switch b.fault {
	case "branch never":
		return ""
	case "branch always main":
		return "main"
	}
	return b.VCS.Branch(ctx)
}
