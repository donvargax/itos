package adapter

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Trees is the tree reader over the disk and the VCS, which says the folder
// itos runs in (its Folder): the working tree's paths are read from it.
type Trees struct{ VCS vcs.VCS }

var _ port.Trees = Trees{}

// WorktreeIn is the working tree's files, a relative path read from the
// folder dir ("" for the process's), and named in an error as given.
func WorktreeIn(dir string) port.Source { return worktree{dir} }

// Worktree is the working tree's files from the process's folder, for a
// program that reads them where it stands.
var Worktree = WorktreeIn("")

type worktree struct{ dir string }

func (worktree) Tree(context.Context) string { return "worktree" }

// path is p read from the folder.
func (w worktree) path(p string) string { return vcs.In(w.dir, p) }

// Has is whether the working tree has the path (existsSync).
func (w worktree) Has(_ context.Context, path string) bool {
	_, err := os.Stat(w.path(path))
	return err == nil
}

func (w worktree) Read(_ context.Context, path string) (string, error) {
	text, err := os.ReadFile(w.path(path))
	if err != nil {
		return "", nodeError(err, "open", path)
	}
	return string(text), nil
}

// List is the names of the entries directly in a folder, sorted.
func (w worktree) List(_ context.Context, dir string) ([]string, error) {
	entries, err := os.ReadDir(w.path(dir))
	if err != nil {
		return nil, nodeError(err, "scandir", dir)
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// At is the source a tree names: the working tree for "worktree", else a tree
// the VCS holds, "index" for the staged tree or a commit. An error outside a
// repository.
func (t Trees) At(ctx context.Context, tree string) (port.Source, error) {
	dir := t.VCS.Folder(ctx)
	if tree == "worktree" {
		return WorktreeIn(dir), nil
	}
	l, err := t.VCS.Locate(ctx, "")
	if err != nil || l.Top == "" {
		return nil, fmt.Errorf("%s cannot be read outside a git repository", tree)
	}
	// git gives the top folder with its links resolved, as Node's cwd is.
	cwd := dir
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	common := l.Common
	if !filepath.IsAbs(common) {
		common = filepath.Join(cwd, common)
	}
	return gitTree{v: t.VCS, tree: tree, top: l.Top, common: filepath.Clean(common), cwd: cwd, files: worktree{dir}}, nil
}

// gitTree is a tree the VCS v holds: the index or a commit. common is the
// git common dir, absolute, cwd the folder itos runs in, its links
// resolved, and files the working tree's, which a path in the git common
// dir is read from.
type gitTree struct {
	v                      vcs.VCS
	tree, top, common, cwd string
	files                  worktree
}

// aside is whether a path is in the git folder, which no tree git holds can
// have: it is read from the file where it is.
func (t gitTree) aside(p string) bool {
	if !filepath.IsAbs(p) {
		p = filepath.Join(t.cwd, p)
	}
	p = filepath.Clean(p)
	return p == t.common || strings.HasPrefix(p, t.common+string(filepath.Separator))
}

func (t gitTree) Tree(context.Context) string { return t.tree }

// name is how a message names the tree.
func (t gitTree) name() string {
	if t.tree == "index" {
		return "the index"
	}
	return t.tree
}

// inRepo is a path relative to the repository's top, with forward slashes.
func (t gitTree) inRepo(p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(t.cwd, p)
	}
	rel, err := filepath.Rel(t.top, p)
	if err != nil || rel == "" {
		return "."
	}
	return filepath.ToSlash(rel)
}

// at is the tree as the VCS names it: vcs.Index for "index".
func at(tree string) string {
	if tree == "index" {
		return vcs.Index
	}
	return tree
}

func (t gitTree) Has(ctx context.Context, p string) bool {
	if t.aside(p) {
		return t.files.Has(ctx, p)
	}
	_, ok, err := t.v.File(ctx, at(t.tree), t.inRepo(p))
	return ok && err == nil
}

func (t gitTree) Read(ctx context.Context, p string) (string, error) {
	if t.aside(p) {
		return t.files.Read(ctx, p)
	}
	text, ok, err := t.v.File(ctx, at(t.tree), t.inRepo(p))
	if err != nil || !ok {
		return "", fmt.Errorf("%s holds no %s", t.name(), p)
	}
	return text, nil
}

func (t gitTree) List(ctx context.Context, dir string) ([]string, error) {
	if t.aside(dir) {
		return t.files.List(ctx, dir)
	}
	in := t.inRepo(dir)
	listed, err := t.v.Files(ctx, at(t.tree), in)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range listed {
		if path.Dir(f) == path.Clean(in) {
			files = append(files, path.Base(f))
		}
	}
	if len(listed) == 0 {
		return nil, fmt.Errorf("%s holds no folder %s", t.name(), dir)
	}
	return files, nil
}

// Texts are the files of a tree the VCS holds under a folder, "index" for
// the staged tree or a commit, that keep accepts, as path to text, paths as
// the VCS gives them. A tree that cannot be read (no HEAD yet, no such
// commit) has none.
func (t Trees) Texts(ctx context.Context, tree, dir string, keep func(string) bool) map[string]string {
	listed, err := t.VCS.Files(ctx, at(tree), dir)
	if err != nil {
		return map[string]string{}
	}
	// git lists the files from the folder it runs in, and reads one by its
	// path from the top unless that starts with ./: the two differ in a
	// project in a subfolder.
	var kept []string
	for _, f := range listed {
		if keep(f) {
			kept = append(kept, "./"+f)
		}
	}
	texts, err := t.VCS.Texts(ctx, at(tree), kept)
	if err != nil {
		return map[string]string{}
	}
	byName := make(map[string]string, len(texts))
	for p, text := range texts {
		byName[strings.TrimPrefix(p, "./")] = text
	}
	return byName
}

// nodeError is a failed file operation as Node reports it: the code, what it
// means, the call and the path, as it was given.
func nodeError(err error, call, path string) error {
	var failed *fs.PathError
	if errors.As(err, &failed) {
		failed.Path = path
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	switch errno {
	case syscall.ENOENT:
		return errors.New("ENOENT: no such file or directory, " + call + " '" + path + "'")
	case syscall.ENOTDIR:
		return errors.New("ENOTDIR: not a directory, " + call + " '" + path + "'")
	case syscall.EACCES:
		return errors.New("EACCES: permission denied, " + call + " '" + path + "'")
	case syscall.EISDIR:
		return errors.New("EISDIR: illegal operation on a directory, read")
	}
	return err
}
