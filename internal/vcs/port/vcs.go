package port

import "context"

// Location is where a repository is from a folder. Top is its work tree's
// top folder, absolute, "" where there is none (inside the git folder).
// GitDir is the folder's git folder and Common the common dir every linked
// worktree of the repository shares, each as Locate names them, relative
// to the folder asked from or absolute, and as Find names them, absolute;
// each cleaned, in the system's form (backslashes on Windows).
type Location struct {
	Top, GitDir, Common string
}

// Unpushed is the start of the range that holds the commits of its end no
// remote branch has (the stealth mode's range, slice 34).
const Unpushed = "--remotes"

// EmptyTree is what a root commit's changes are read against: git's empty
// tree.
const EmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Range is a range of commits: those of To that From does not have, From a
// revision, "" for every commit up to To, or Unpushed for the commits of To
// no remote branch has; Since, when set, leaves that commit and its
// ancestors out.
type Range struct {
	From, To, Since string
}

// NotesRef is where the stealth mode keeps a commit's footers: its itos
// note (slice 32).
const NotesRef = "refs/notes/itos"

// Entry is one commit as a log reads it: its full and short names, its
// subject (its message's first paragraph on one line), its message as
// stored, its itos note ("" when it has none), its author's name, email
// and date (seconds since the epoch and the zone, as "1767225601 +0000"),
// and whether it is a merge.
type Entry struct {
	SHA, Short, Subject, Message, Note string
	Author, Email, Date                string
	Merge                              bool
}

// VCS is the repository itos runs in. A folder is relative to the folder
// itos runs in, "" for that one. Every method takes ctx first, which carries
// cancellation and nothing else; callers pass context.Background() until v9
// threads cancellation (ADR-0053). porttest.VCSContract holds what every
// one, the fake's and the adapter's, must do.
type VCS interface {
	// Locate is the repository holding the folder dir; an error outside
	// one. Asked once per folder and run.
	Locate(ctx context.Context, dir string) (Location, error)
	// Find is the work tree holding the folder dir, told from the files
	// alone, without running the VCS, as the git shim must on every
	// command it sees: its top and its common dir, absolute, links in dir
	// resolved. Not ok outside a work tree, inside the git folder among
	// them.
	Find(ctx context.Context, dir string) (Location, bool)
	// Resolve is the full name of the commit a revision names; an error
	// when it names none.
	Resolve(ctx context.Context, rev string) (string, error)
	// Shallow is whether the repository is a shallow clone: its history
	// stops short of commits the repository it was cloned from has.
	Shallow(ctx context.Context) bool
	// IsAncestor is whether the commit ancestor is rev or one of its
	// ancestors; not when either names no commit.
	IsAncestor(ctx context.Context, ancestor, rev string) bool
	// Parent is a commit's first parent, EmptyTree for a root commit or a
	// revision that names none.
	Parent(ctx context.Context, rev string) string
	// Parents are a commit's parents, the first first; none for a root
	// commit, and an error for a revision that names none.
	Parents(ctx context.Context, rev string) ([]string, error)
	// FirstParents are rev and its first parents, nearest first, skip of
	// them passed over and at most most of them listed; an error when rev
	// names no commit.
	FirstParents(ctx context.Context, rev string, skip, most int) ([]string, error)
	// Commits are a range's commits, oldest first; an error when the range
	// cannot be read.
	Commits(ctx context.Context, r Range) ([]string, error)
	// Readable is whether the range can be read: its ends name commits.
	Readable(ctx context.Context, r Range) bool
	// UnpushedBase is the one commit the unpushed commits of to grow from:
	// to itself, as given, when none is unpushed, the remote branches'
	// commit their history starts from when there is one, else "": with no
	// remote, or with the unpushed commits growing from several remote
	// branches' commits, none an ancestor of another.
	UnpushedBase(ctx context.Context, to string) string

	// Show is the commit a revision names; an error when it names none.
	Show(ctx context.Context, rev string) (Entry, error)
	// Log are a range's commits, oldest first, each as Show gives it; an
	// error when the range cannot be read.
	Log(ctx context.Context, r Range) ([]Entry, error)
	// CommentChar is what a comment line of a message being written starts
	// with, # when the repository sets nothing.
	CommentChar(ctx context.Context) string

	// The paths a list of changed paths names are the repository's, from
	// its top, each as it is named, in byte order unless said otherwise; a
	// rename is its old path's deletion and its new path's addition.

	// CommitPaths are the paths a commit changes against its parent, every
	// path it holds for a root commit, none for a merge; an error when rev
	// names no commit.
	CommitPaths(ctx context.Context, rev string) ([]string, error)
	// OwnPaths are the paths a merge changes of its own: those it holds
	// with lines no parent has there; none for a commit with one parent or
	// none, and an error when rev names no commit.
	OwnPaths(ctx context.Context, rev string) ([]string, error)
	// Diff are the paths that differ between two commits, from EmptyTree
	// for every path to holds; an error when either names none.
	Diff(ctx context.Context, from, to string) ([]string, error)
	// Touched are the paths a range's commits change, each against its
	// first parent, each path once in the order first named, newest
	// commit first; a merge's only when merges is set. An error when the
	// range cannot be read.
	Touched(ctx context.Context, r Range, merges bool) ([]string, error)
	// Staged are the paths the index changes against the commit base, HEAD
	// when base is ""; an error when base names none.
	Staged(ctx context.Context, base string) ([]string, error)
	// StagedOwnPaths are the paths a merge of parents holding the index
	// would change of its own, as OwnPaths gives a merge's; an error when a
	// parent names no commit.
	StagedOwnPaths(ctx context.Context, parents []string) ([]string, error)

	// Changes are the tracked files with uncommitted changes, staged or
	// not, a line each as git status --porcelain prints it ("XY path",
	// a rename "R  old -> new"), in byte order of their paths.
	Changes(ctx context.Context) []string
	// PathStatus is a path's line as Changes gives it, "?? path" when it is
	// untracked and not ignored, none when it has no change; an error
	// outside a repository.
	PathStatus(ctx context.Context, path string) ([]string, error)
	// Tracked is whether the index holds the path.
	Tracked(ctx context.Context, path string) bool
	// Conflicted are the paths a merge or a rebase left unmerged.
	Conflicted(ctx context.Context) []string
	// Rebasing is whether a rebase is in progress.
	Rebasing(ctx context.Context) bool
	// MergeHeads are the commits a merge in progress brings into HEAD; none
	// when no merge is in progress.
	MergeHeads(ctx context.Context) []string
	// Branch is the branch HEAD is on, its short name; "" when HEAD is
	// detached.
	Branch(ctx context.Context) string
}
