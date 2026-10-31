package porttest

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// entry is a commit as Show gives it: its short name is its name less its
// last letter, its subject its message's first paragraph on one line.
func entry(c Commit) port.Entry {
	paragraph, _, _ := strings.Cut(strings.TrimLeft(c.Message, "\n"), "\n\n")
	return port.Entry{SHA: c.Name, Short: c.Name[:len(c.Name)-1],
		Subject: strings.ReplaceAll(strings.TrimRight(paragraph, "\n"), "\n", " "),
		Message: c.Message, Note: c.Note, Author: c.Author, Email: c.Email, Date: c.Date, Merge: len(c.Parents) > 1}
}

func (v VCS) Show(_ context.Context, rev string) (port.Entry, error) {
	c, ok := v.commit(rev)
	if !ok {
		return port.Entry{}, errors.New(rev + " names no commit")
	}
	return entry(c), nil
}

func (v VCS) Log(_ context.Context, r port.Range) ([]port.Entry, error) {
	in, ok := v.within(r)
	if !ok {
		return nil, errors.New("the range cannot be read")
	}
	var found []port.Entry
	for _, c := range v.History {
		if in[c.Name] {
			found = append(found, entry(c))
		}
	}
	return found, nil
}

func (v VCS) CommentChar(context.Context) string {
	if v.Comment == "" {
		return "#"
	}
	return v.Comment
}

// tree is the tree of a revision, none for EmptyTree; not ok when it names
// no commit.
func (v VCS) tree(rev string) (map[string]string, bool) {
	if rev == port.EmptyTree {
		return map[string]string{}, true
	}
	c, ok := v.commit(rev)
	return c.Tree, ok
}

// differ are the paths whose content, or presence, differs between two
// trees, in byte order.
func differ(a, b map[string]string) []string {
	var paths []string
	for _, p := range union(a, b) {
		x, inA := a[p]
		y, inB := b[p]
		if inA != inB || x != y {
			paths = append(paths, p)
		}
	}
	return paths
}

// union are the paths of the trees, each once, in byte order.
func union(trees ...map[string]string) []string {
	seen := map[string]bool{}
	for _, t := range trees {
		for p := range maps.Keys(t) {
			seen[p] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// own are the paths whose content, or presence, in tree is no parent's.
func own(tree map[string]string, parents []map[string]string) []string {
	var paths []string
	for _, p := range union(append([]map[string]string{tree}, parents...)...) {
		mine := true
		for _, parent := range parents {
			x, in := tree[p]
			y, inParent := parent[p]
			mine = mine && (in != inParent || x != y)
		}
		if mine {
			paths = append(paths, p)
		}
	}
	return paths
}

func (v VCS) CommitPaths(_ context.Context, rev string) ([]string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return nil, errors.New(rev + " names no commit")
	}
	switch len(c.Parents) {
	case 0:
		return differ(nil, c.Tree), nil
	case 1:
		parent, _ := v.tree(c.Parents[0])
		return differ(parent, c.Tree), nil
	}
	return nil, nil
}

func (v VCS) OwnPaths(_ context.Context, rev string) ([]string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return nil, errors.New(rev + " names no commit")
	}
	if len(c.Parents) < 2 {
		return nil, nil
	}
	return v.ownOn(c.Tree, c.Parents)
}

// ownOn are a tree's own paths on the parents: on two with a common commit,
// those where it differs from Merge's tree of them, every conflict among
// them; else those whose content is no parent's.
func (v VCS) ownOn(tree map[string]string, parents []string) ([]string, error) {
	var commits []Commit
	var trees []map[string]string
	for _, p := range parents {
		c, ok := v.commit(p)
		if !ok {
			return nil, errors.New(p + " names no commit")
		}
		commits, trees = append(commits, c), append(trees, c.Tree)
	}
	if len(commits) == 2 {
		if merged, conflicts, based := v.merged(commits[0], commits[1]); based {
			for _, p := range conflicts {
				merged[p] = "<<<<<<< conflict markers no tree holds"
			}
			return differ(merged, tree), nil
		}
	}
	return own(tree, trees), nil
}

func (v VCS) StagedOwnPaths(_ context.Context, parents []string) ([]string, error) {
	return v.ownOn(v.index(), parents)
}

func (v VCS) Diff(_ context.Context, from, to string) ([]string, error) {
	a, okA := v.tree(from)
	b, okB := v.tree(to)
	if !okA || !okB {
		return nil, errors.New("a commit is missing")
	}
	return differ(a, b), nil
}

func (v VCS) Touched(ctx context.Context, r port.Range, merges bool) ([]string, error) {
	in, ok := v.within(r)
	if !ok {
		return nil, errors.New("the range cannot be read")
	}
	seen := map[string]bool{}
	var paths []string
	for _, c := range slices.Backward(v.History) {
		if !in[c.Name] || len(c.Parents) > 1 && !merges {
			continue
		}
		var touched []string
		if len(c.Parents) > 1 {
			touched, _ = v.Diff(ctx, c.Parents[0], c.Name)
		} else {
			touched, _ = v.CommitPaths(ctx, c.Name)
		}
		for _, p := range touched {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths, nil
}

// head is HEAD's tree, none before the first commit.
func (v VCS) head() map[string]string {
	t, _ := v.tree("HEAD")
	return t
}

// index is the index's files, HEAD's when Index is nil.
func (v VCS) index() map[string]string {
	if v.Index == nil {
		return v.head()
	}
	return v.Index
}

// files are the work tree's files, the index's when Files is nil.
func (v VCS) files() map[string]string {
	if v.Work == nil {
		return v.index()
	}
	return v.Work
}

func (v VCS) Staged(_ context.Context, base string) ([]string, error) {
	if base == "" {
		base = "HEAD"
	}
	t, ok := v.tree(base)
	if !ok {
		return nil, errors.New(base + " names no commit")
	}
	return differ(t, v.index()), nil
}

// status is a change's letter between two trees: A added, D deleted, M
// modified, a space for none.
func status(before, after map[string]string, p string) byte {
	x, was := before[p]
	y, is := after[p]
	switch {
	case !was && is:
		return 'A'
	case was && !is:
		return 'D'
	case x != y:
		return 'M'
	}
	return ' '
}

func (v VCS) Changes(context.Context) []string {
	head, index, files := v.head(), v.index(), v.files()
	var lines []string
	for _, p := range union(head, index) {
		staged, unstaged := status(head, index, p), byte(' ')
		if _, tracked := index[p]; tracked {
			unstaged = status(index, files, p)
		}
		if staged != ' ' || unstaged != ' ' {
			lines = append(lines, string([]byte{staged, unstaged, ' '})+p)
		}
	}
	return lines
}

func (v VCS) PathStatus(ctx context.Context, path string) ([]string, error) {
	var lines []string
	for _, l := range v.Changes(ctx) {
		if l[3:] == path {
			lines = append(lines, l)
		}
	}
	_, tracked := v.index()[path]
	if _, there := v.files()[path]; there && !tracked {
		lines = append(lines, "?? "+path)
	}
	return lines, nil
}

func (v VCS) Tracked(_ context.Context, path string) bool {
	_, ok := v.index()[path]
	return ok
}

func (v VCS) Conflicted(context.Context) []string { return slices.Clone(v.Unmerged) }

func (v VCS) Rebasing(context.Context) bool { return v.InRebase }

func (v VCS) MergeHeads(context.Context) []string { return slices.Clone(v.MergeHead) }

func (v VCS) Branch(context.Context) string { return v.OnBranch }
