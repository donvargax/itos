package porttest

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// The fake's files at a tree, patches and refs.

// at is the files of a tree: the index's for port.Index, else a commit's;
// not ok when it names none.
func (v VCS) at(tree string) (map[string]string, bool) {
	if tree == port.Index {
		return v.index(), true
	}
	c, ok := v.commit(tree)
	return c.Tree, ok
}

func (v VCS) File(_ context.Context, tree, path string) (string, bool, error) {
	files, ok := v.at(tree)
	if !ok {
		return "", false, errors.New(tree + " names no tree")
	}
	text, ok := files[path]
	return text, ok, nil
}

func (v VCS) Files(_ context.Context, tree, dir string) ([]string, error) {
	files, ok := v.at(tree)
	if !ok {
		return nil, errors.New(tree + " names no tree")
	}
	var found []string
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if dir == "" || dir == "." || p == dir || strings.HasPrefix(p, dir+"/") {
			found = append(found, p)
		}
	}
	return found, nil
}

func (v VCS) Texts(_ context.Context, tree string, paths []string) (map[string]string, error) {
	files, ok := v.at(tree)
	if !ok {
		return nil, errors.New(tree + " names no tree")
	}
	texts := map[string]string{}
	for _, p := range paths {
		text, ok := files[p]
		if !ok {
			return nil, errors.New(tree + " holds no " + p)
		}
		texts[p] = text
	}
	return texts, nil
}

// patch is the change from one tree to another, a path's old lines marked
// - and its new +, in byte order of the paths.
func patch(before, after map[string]string) string {
	var b strings.Builder
	for _, p := range differ(before, after) {
		b.WriteString("diff " + p + "\n")
		for _, l := range strings.SplitAfter(before[p], "\n") {
			if l != "" {
				b.WriteString("-" + l)
			}
		}
		for _, l := range strings.SplitAfter(after[p], "\n") {
			if l != "" {
				b.WriteString("+" + l)
			}
		}
	}
	return b.String()
}

// Patch is the change against the first parent, against nothing for a
// root commit.
func (v VCS) Patch(_ context.Context, rev string) (string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return "", errors.New(rev + " names no commit")
	}
	var parent map[string]string
	if len(c.Parents) > 0 {
		parent, _ = v.tree(c.Parents[0])
	}
	return patch(parent, c.Tree), nil
}

// Shown is each commit's name, message, note under "Notes:" and patch.
func (v VCS) Shown(ctx context.Context, revs []string, notes bool) (string, error) {
	var b strings.Builder
	for _, rev := range revs {
		c, ok := v.commit(rev)
		if !ok {
			return "", errors.New(rev + " names no commit")
		}
		b.WriteString("commit " + c.Name + "\n\n" + c.Message)
		if notes && c.Note != "" {
			b.WriteString("\nNotes:\n" + c.Note)
		}
		p, _ := v.Patch(ctx, rev)
		b.WriteString("\n" + p)
	}
	return b.String(), nil
}

func (v VCS) Ref(_ context.Context, ref string) (string, error) {
	if sha, ok := v.Named[ref]; ok {
		return sha, nil
	}
	return "", errors.New(ref + " names nothing")
}

// refsOf are refs as a listing gives them: each with the object it names,
// and an annotated tag's with ^{} its commit too.
func (v VCS) refsOf(refs map[string]string, keep func(string) bool) map[string]string {
	found := map[string]string{}
	for ref, object := range refs {
		if !keep(ref) {
			continue
		}
		found[ref] = object
		if commit, ok := v.Tags[object]; ok {
			found[ref+"^{}"] = commit
		}
	}
	return found
}

func (v VCS) Symref(_ context.Context, ref string) (string, bool) {
	target, ok := v.Symbolic[ref]
	return target, ok
}

func (v VCS) Refs(_ context.Context, prefix string) (map[string]string, error) {
	return v.refsOf(v.Named, func(ref string) bool { return strings.HasPrefix(ref, prefix) && ref != "HEAD" }), nil
}
