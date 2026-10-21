package adapter

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// Trees is the tree reader over the disk and git.
type Trees struct{}

var _ port.Trees = Trees{}

// Worktree is the working tree's files.
var Worktree port.Source = worktree{}

type worktree struct{}

func (worktree) Tree(context.Context) string { return "worktree" }

// Has is whether the working tree has the path (existsSync).
func (worktree) Has(_ context.Context, path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (worktree) Read(_ context.Context, path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", nodeError(err, "open", path)
	}
	return string(text), nil
}

// List is the names of the entries directly in a folder, sorted.
func (worktree) List(_ context.Context, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
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
// git holds, "index" for the staged tree or a commit. An error outside a
// repository.
func (Trees) At(_ context.Context, tree string) (port.Source, error) {
	if tree == "worktree" {
		return Worktree, nil
	}
	out, err := git.Output("rev-parse", "--show-toplevel", "--path-format=absolute", "--git-common-dir")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || len(lines) != 2 {
		return nil, fmt.Errorf("%s cannot be read outside a git repository", tree)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	// git gives the top folder with its links resolved, as Node's cwd is.
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	return gitTree{tree: tree, top: lines[0], common: filepath.Clean(lines[1]), cwd: cwd}, nil
}

// gitTree is a tree git holds: the index or a commit. common is the git
// common dir, absolute.
type gitTree struct{ tree, top, common, cwd string }

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

// spec is a path as git show and cat-file take it: `:<path>` in the index,
// `<commit>:<path>` in a commit.
func (t gitTree) spec(p string) string {
	if t.tree == "index" {
		return ":" + t.inRepo(p)
	}
	return t.tree + ":" + t.inRepo(p)
}

func (t gitTree) Has(ctx context.Context, p string) bool {
	if t.aside(p) {
		return Worktree.Has(ctx, p)
	}
	return git.Succeeds("cat-file", "-e", t.spec(p))
}

func (t gitTree) Read(ctx context.Context, p string) (string, error) {
	if t.aside(p) {
		return Worktree.Read(ctx, p)
	}
	text, err := git.Output("show", t.spec(p))
	if err != nil {
		return "", fmt.Errorf("%s holds no %s", t.name(), p)
	}
	return text, nil
}

func (t gitTree) List(ctx context.Context, dir string) ([]string, error) {
	if t.aside(dir) {
		return Worktree.List(ctx, dir)
	}
	at := t.inRepo(dir)
	listed, err := listTree(t.tree, at)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range listed {
		if path.Dir(f) == path.Clean(at) {
			files = append(files, path.Base(f))
		}
	}
	if len(listed) == 0 {
		return nil, fmt.Errorf("%s holds no folder %s", t.name(), dir)
	}
	return files, nil
}

// listTree is the files git holds under a folder at a tree, as git names
// them: read NUL-separated, so that a path git would quote is listed as it
// is named (bug 31).
func listTree(tree, dir string) ([]string, error) {
	if tree == "index" {
		return git.Paths("ls-files", "--cached", "--", dir)
	}
	return git.Paths("ls-tree", "-r", "--name-only", tree, "--", dir)
}

// Texts are the files of a tree git holds under a folder, "index" for the
// staged tree or a commit, that keep accepts, as path to text, paths as git
// gives them. A tree that cannot be read (no HEAD yet, no such commit) has
// none. git cat-file reads them in one run, where the TypeScript runs git show
// per file: the same texts, in a run per tree rather than per file.
func (Trees) Texts(ctx context.Context, tree, dir string, keep func(string) bool) map[string]string {
	listed, err := listTree(tree, dir)
	if err != nil {
		return map[string]string{}
	}
	spec := tree + ":"
	if tree == "index" {
		spec = ":"
	}
	texts := map[string]string{}
	var paths []string
	var specs bytes.Buffer
	for _, f := range listed {
		if !keep(f) {
			continue
		}
		// git cat-file --batch takes one name a line, a carriage return at
		// its end dropped, so a path with a line break in it is read alone.
		if strings.ContainsAny(f, "\r\n") {
			text, err := git.Output("show", spec+f)
			if err != nil {
				return map[string]string{}
			}
			texts[f] = text
			continue
		}
		paths = append(paths, f)
		specs.WriteString(spec + f + "\n")
	}
	if len(paths) == 0 {
		return texts
	}
	cmd := exec.CommandContext(ctx, git.Bin(), "cat-file", "--batch")
	cmd.Stdin = &specs
	out, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for _, p := range paths {
		text, ok := batchEntry(r)
		if !ok {
			return map[string]string{}
		}
		texts[p] = text
	}
	return texts
}

// batchEntry is the next object of git cat-file --batch's output: its header
// (`<sha> <type> <size>`), its contents and a line break; not ok when git
// found no object, as git show would fail on it.
func batchEntry(r *bufio.Reader) (string, bool) {
	header, err := r.ReadString('\n')
	if err != nil {
		return "", false
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return "", false
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", false
	}
	content := make([]byte, size+1)
	if _, err := io.ReadFull(r, content); err != nil {
		return "", false
	}
	return string(content[:size]), true
}

// nodeError is a failed file operation as Node reports it: the code, what it
// means, the call and the path.
func nodeError(err error, call, path string) error {
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
