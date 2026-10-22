package draft

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// treeEntries are the commit's entries at the paths, named from the top: a
// path it lacks is not among them.
func (h Handler) treeEntries(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error) {
	entries, err := h.ports.TreeEntries(ctx, commit, paths)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s: %w", commit, err)
	}
	return entries, nil
}

// editPatch writes an edit draft's change as a patch file beside the list
// at file, which the caller removes: its copies against its base, through
// a scratch index. A copy removed takes its file out; an empty copy of a
// file the base lacks adds nothing. It gives why there is no patch when
// the base is gone or no copy differs from it.
func (h Handler) editPatch(ctx context.Context, file string, d domain.Draft) (patch, why string, err error) {
	scratch, err := os.MkdirTemp(filepath.Dir(file), ".index-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(scratch)
	index := h.ports.Scratch(ctx, filepath.Join(scratch, "index"))
	if err := index.ReadTree(ctx, d.Base); err != nil {
		return "", "the commit its copies were taken from, " + d.Base + ", is gone (" + oneLine(err.Error()) + ")", nil
	}
	had, err := h.treeEntries(ctx, d.Base, d.Paths)
	if err != nil {
		return "", "", err
	}
	for _, p := range d.Paths {
		e, inBase := had[p]
		info, err := os.Stat(domain.Copy(file, d.ID, p))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if inBase {
				if err := index.Remove(ctx, p); err != nil {
					return "", "", fmt.Errorf("git update-index --force-remove %s: %w", p, err)
				}
			}
			continue
		case err != nil:
			return "", "", err
		case !inBase && info.Size() == 0:
			continue
		}
		object, err := index.Store(ctx, domain.Copy(file, d.ID, p))
		if err != nil {
			return "", "", fmt.Errorf("git hash-object %s's copy of %s: %w", d.ID, p, err)
		}
		mode := "100644"
		if inBase {
			mode = e.Mode
		}
		if err := index.Set(ctx, mode, object, p); err != nil {
			return "", "", fmt.Errorf("git update-index --cacheinfo %s: %w", p, err)
		}
	}
	text, err := index.Diff(ctx, d.Base, d.Paths)
	if err != nil {
		return "", "", fmt.Errorf("git diff --cached --binary: %w", err)
	}
	if text == "" {
		return "", "no copy differs from what it was taken from (" + domain.Copies(file, d.ID) + ")", nil
	}
	made, err := os.CreateTemp(filepath.Dir(file), "."+d.ID+".*"+domain.PatchSuffix)
	if err != nil {
		return "", "", err
	}
	_, werr := made.WriteString(text)
	if err := errors.Join(werr, made.Close()); err != nil {
		os.Remove(made.Name())
		return "", "", err
	}
	return made.Name(), "", nil
}

// topPaths are the typed paths as the repository's top names them, with
// itos at the top; a path outside the work tree is a usage error.
func (h Handler) topPaths(ctx context.Context, command string, typedPaths []string, origin string) ([]string, error) {
	l, err := h.ports.Locate(ctx, "")
	if err == nil && l.Top == "" {
		err = errors.New("no work tree")
	}
	if err != nil {
		return nil, fmt.Errorf("the work tree's top cannot be read: %w", err)
	}
	top := l.Top
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	here, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(here); err == nil {
		here = real
	}
	paths := make([]string, len(typedPaths))
	for i, p := range typedPaths {
		full := typed(origin, p)
		if !filepath.IsAbs(full) {
			full = filepath.Join(here, full)
		}
		rel, err := filepath.Rel(top, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, out.Usage("%s: %s is outside the work tree", command, p)
		}
		paths[i] = filepath.ToSlash(rel)
	}
	if err := os.Chdir(top); err != nil {
		return nil, err
	}
	return paths, nil
}

// takeChange writes the change of the paths against HEAD as the draft's
// patch, beside the list at file, and gives the paths as the top names them
// and the files the change touches. A path that names nothing in the work
// tree or HEAD, a path git ignores, no change at all and no HEAD are
// refusals, nothing written.
func (h Handler) takeChange(ctx context.Context, file, id string, typedPaths []string, origin string) (paths, names []string, refused *out.Problem, err error) {
	paths, err = h.topPaths(ctx, "draft add", typedPaths, origin)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := h.ports.Resolve(ctx, "HEAD"); err != nil {
		return nil, nil, &out.Problem{Rule: "draft-no-head", Message: "the repository has no commit yet, and a change is drafted against HEAD",
			Fix: "commit something first"}, nil
	}
	scratch, err := os.MkdirTemp(filepath.Dir(file), ".index-")
	if err != nil {
		return nil, nil, nil, err
	}
	defer os.RemoveAll(scratch)
	index := h.ports.Scratch(ctx, filepath.Join(scratch, "index"))
	if err := index.ReadTree(ctx, "HEAD"); err != nil {
		return nil, nil, nil, fmt.Errorf("git read-tree HEAD: %w", err)
	}
	if err := index.AddAll(ctx, paths); err != nil {
		return nil, nil, &out.Problem{Rule: "draft-paths", Message: fmt.Sprintf("%s cannot be drafted: %s", and(paths), err),
			Fix: "name files the work tree or HEAD has, and git does not ignore"}, nil
	}
	names, err = index.Changed(ctx, "HEAD", paths)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git diff --cached: %w", err)
	}
	if len(names) == 0 {
		return nil, nil, &out.Problem{Rule: "draft-no-change", Message: fmt.Sprintf("%s has no change against HEAD to draft", and(paths)),
			Fix: "change the files first, or draft a command line with -- <itos args>…"}, nil
	}
	patch, err := index.Diff(ctx, "HEAD", paths)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git diff --cached --binary: %w", err)
	}
	if err := domain.WriteFile(domain.Patch(file, id), []byte(patch)); err != nil {
		return nil, nil, nil, err
	}
	return paths, names, nil, nil
}

// splitNULs is a NUL-separated list, the empty entries dropped.
func splitNULs(text string) []string {
	var entries []string
	for _, e := range strings.Split(text, "\x00") {
		if e != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// wordsOf is what git said of a failure on stderr, when err is a command
// that failed, else err's own words.
func wordsOf(err error) string {
	var failed *vcsport.Failed
	if errors.As(err, &failed) {
		return strings.TrimSpace(failed.Words)
	}
	return err.Error()
}

// putBack puts the files, named from the top, back as HEAD has them, in
// the index and the working tree: one HEAD has is checked out of it, one it
// lacks is taken out of the index and removed, with the folders it leaves
// empty. Nothing else is touched.
func (h Handler) putBack(ctx context.Context, names []string) error {
	var kept, made []string
	for _, name := range names {
		if _, ok, err := h.ports.File(ctx, "HEAD", name); ok && err == nil {
			kept = append(kept, name)
		} else {
			made = append(made, name)
		}
	}
	var failed error
	if len(kept) > 0 {
		if err := h.ports.Restore(ctx, kept); err != nil {
			failed = fmt.Errorf("git checkout HEAD -- %s: %s", and(kept), wordsOf(err))
		}
	}
	if len(made) > 0 {
		if err := h.ports.Untrack(ctx, made); err != nil {
			failed = errors.Join(failed, fmt.Errorf("git rm --cached -- %s: %s", and(made), wordsOf(err)))
		}
	}
	for _, name := range made {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failed = errors.Join(failed, err)
			continue
		}
		for dir := filepath.Dir(name); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
			if os.Remove(dir) != nil {
				break
			}
		}
	}
	return failed
}

// patchNames are the files a patch touches, from git apply --numstat -z:
// each entry added, deleted and the path, renames being off in its diff.
func patchNames(numstat string) []string {
	var names []string
	for _, entry := range splitNULs(numstat) {
		fields := strings.SplitN(entry, "\t", 3)
		if len(fields) == 3 && fields[2] != "" {
			names = append(names, fields[2])
		}
	}
	return names
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c"
// (internal/cli's).
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// typed is a path the person typed, as itos reads it at the top it moved to
// from origin, the subfolder they stood in (internal/cli's): relative to
// it, as git reads a path typed there. An absolute path, "-" (stdin) and
// every path of a run that did not move stay as typed.
func typed(origin, p string) string {
	if origin == "" || p == "" || p == "-" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(origin, p)
}

// oneLine is a text on one line, its whitespace each a space, as a list and
// a commit's body give it (internal/cli's).
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }
