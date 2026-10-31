package port

import "context"

// Source is a tree itos reads its data from. Paths are the working tree's,
// relative to the folder itos runs in, as everywhere else. Every method
// takes ctx first, which carries cancellation and nothing else; callers
// pass context.Background() until v9 threads cancellation (ADR-0053).
// porttest.TreesContract holds what every one, the fake's and the
// adapter's, must do.
type Source interface {
	// Tree is "worktree", "index" or a commit, as Trees.At was given it.
	Tree(ctx context.Context) string
	// Has is whether the source has the path: a file, or a folder in the
	// working tree or a commit (the index keeps no folder).
	Has(ctx context.Context, path string) bool
	// Read is the file's text; an error when the source does not hold it.
	Read(ctx context.Context, path string) (string, error)
	// List is the names of the entries directly in a folder, sorted: in the
	// working tree its files and folders, in a tree git holds its files;
	// an error when there is none.
	List(ctx context.Context, dir string) ([]string, error)
}

// Trees opens the trees itos reads: the working tree, the index and the
// commits. A path in the git common dir is in no tree git holds, so every
// source reads it from the working tree.
type Trees interface {
	// At is the source a tree names: the working tree for "worktree", else
	// a tree git holds, "index" for the staged tree or a commit. An error
	// for a tree git holds outside a repository.
	At(ctx context.Context, tree string) (Source, error)
	// Texts are the files of a tree git holds under a folder, and its
	// folders', that keep accepts, as path to text, paths from the folder
	// itos runs in, as git lists them. A tree that cannot be read (the
	// working tree, no HEAD yet, no such commit, no repository) has none.
	Texts(ctx context.Context, tree, dir string, keep func(string) bool) map[string]string
}
