// Package source is where itos reads its own data (tools/itos/source.ts): the
// config, the ledger, the work registry, the people and the smoke sets. By
// default that is the working tree's files; ReadingFrom reads them from a tree
// git holds instead, the index (the staged tree, which the commit-msg hook
// judges, since it is what the commit will hold) or a commit, for as long as a
// function runs. Every reader of them goes through Has, Read and List, so
// config check and the hook's check of the staged data are one check.
//
// Beside the source, Texts reads the files of a tree under a folder at once
// (repo.ts's treeTexts): the ledger's IDs and the Gherkin adapter's feature
// files at the tree a footer or `--at` names, whatever the source is.
//
// A working tree's read that fails says so in Node's words (`ENOENT: no such
// file or directory, open 'people.yaml'`), since what itos prints quotes them.
package source

import (
	"bufio"
	"bytes"
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

	"github.com/donvargax/itos/v2/internal/git"
)

// Source is a tree itos reads its data from. Paths are the working tree's,
// relative to the current folder, as everywhere else.
type Source interface {
	// Tree is "worktree", "index" or a commit, as a kind's adapter takes a
	// tree.
	Tree() string
	Has(path string) bool
	// Read is the file's text; an error when the source does not hold it.
	Read(path string) (string, error)
	// List is the names of the files directly in a folder; an error when
	// there is none.
	List(dir string) ([]string, error)
}

// Worktree is the working tree's files.
var Worktree Source = worktree{}

type worktree struct{}

func (worktree) Tree() string { return "worktree" }

// Has is whether the working tree has the path (existsSync).
func (worktree) Has(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (worktree) Read(path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", nodeError(err, "open", path)
	}
	return string(text), nil
}

// List is the names of the entries directly in a folder, sorted.
func (worktree) List(dir string) ([]string, error) {
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
func At(tree string) (Source, error) {
	if tree == "worktree" {
		return Worktree, nil
	}
	top, err := git.Output("rev-parse", "--show-toplevel")
	if err != nil {
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
	return gitTree{tree: tree, top: strings.TrimSpace(top), cwd: cwd}, nil
}

// gitTree is a tree git holds: the index or a commit.
type gitTree struct{ tree, top, cwd string }

func (t gitTree) Tree() string { return t.tree }

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

func (t gitTree) Has(p string) bool { return git.Succeeds("cat-file", "-e", t.spec(p)) }

func (t gitTree) Read(p string) (string, error) {
	text, err := git.Output("show", t.spec(p))
	if err != nil {
		return "", fmt.Errorf("%s holds no %s", t.name(), p)
	}
	return text, nil
}

func (t gitTree) List(dir string) ([]string, error) {
	at := t.inRepo(dir)
	listed, err := listTree(t.tree, at)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(listed, "\n") {
		if f != "" && path.Dir(f) == path.Clean(at) {
			files = append(files, path.Base(f))
		}
	}
	if len(files) == 0 && strings.TrimSpace(listed) == "" {
		return nil, fmt.Errorf("%s holds no folder %s", t.name(), dir)
	}
	return files, nil
}

// listTree is the files git holds under a folder at a tree, one per line, as
// git names them.
func listTree(tree, dir string) (string, error) {
	if tree == "index" {
		return git.Output("ls-files", "--cached", "--", dir)
	}
	return git.Output("ls-tree", "-r", "--name-only", tree, "--", dir)
}

var current = Worktree

// Current is the source itos's data is read from now.
func Current() Source { return current }

// ReadingFrom runs fn with itos's data read from s while it runs.
func ReadingFrom(s Source, fn func() error) error {
	before := current
	current = s
	defer func() { current = before }()
	return fn()
}

// Has is whether the current source has the path.
func Has(path string) bool { return current.Has(path) }

// Read is the file's text in the current source.
func Read(path string) (string, error) { return current.Read(path) }

// List is the names of the entries directly in a folder of the current
// source.
func List(dir string) ([]string, error) { return current.List(dir) }

// Texts are the files of a tree git holds under a folder, "index" for the
// staged tree or a commit, that keep accepts, as path to text, paths as git
// gives them. A tree that cannot be read (no HEAD yet, no such commit) has
// none. git cat-file reads them in one run, where the TypeScript runs git show
// per file: the same texts, in a run per tree rather than per file.
func Texts(tree, dir string, keep func(string) bool) map[string]string {
	listed, err := listTree(tree, dir)
	if err != nil {
		return map[string]string{}
	}
	var paths []string
	var specs bytes.Buffer
	for _, f := range strings.Split(listed, "\n") {
		if f == "" || !keep(f) {
			continue
		}
		paths = append(paths, f)
		if tree == "index" {
			specs.WriteString(":" + f + "\n")
		} else {
			specs.WriteString(tree + ":" + f + "\n")
		}
	}
	texts := map[string]string{}
	if len(paths) == 0 {
		return texts
	}
	cmd := exec.Command("git", "cat-file", "--batch")
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
