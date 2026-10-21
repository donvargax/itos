package source

import (
	"context"

	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// Source is a tree itos reads its data from, a port.Source read under
// context.Background(). Paths are the working tree's, relative to the
// current folder, as everywhere else.
type Source interface {
	// Tree is "worktree", "index" or a commit, as a kind's adapter takes a
	// tree.
	Tree() string
	Has(path string) bool
	// Read is the file's text; an error when the source does not hold it.
	Read(path string) (string, error)
	// List is the names of the entries directly in a folder; an error when
	// there is none.
	List(dir string) ([]string, error)
}

// bound is a port's source as a Source.
type bound struct{ source port.Source }

func (b bound) Tree() string { return b.source.Tree(context.Background()) }

func (b bound) Has(path string) bool { return b.source.Has(context.Background(), path) }

func (b bound) Read(path string) (string, error) {
	return b.source.Read(context.Background(), path)
}

func (b bound) List(dir string) ([]string, error) {
	return b.source.List(context.Background(), dir)
}

// Worktree is the working tree's files.
var Worktree Source = bound{adapter.Worktree}

// At is the source a tree names: the working tree for "worktree", else a tree
// git holds, "index" for the staged tree or a commit. An error outside a
// repository.
func At(tree string) (Source, error) {
	s, err := adapter.Trees{}.At(context.Background(), tree)
	if err != nil {
		return nil, err
	}
	return bound{s}, nil
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
// gives them; port.Trees.Texts says which.
func Texts(tree, dir string, keep func(string) bool) map[string]string {
	return adapter.Trees{}.Texts(context.Background(), tree, dir, keep)
}
