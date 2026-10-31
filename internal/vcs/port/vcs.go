package port

import (
	"context"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// Location is where a repository is from a folder. Top is its work tree's
// top folder, absolute, "" where there is none (inside the git folder).
// GitDir is the folder's git folder and Common the common dir every linked
// worktree of the repository shares, each as Locate names them, relative
// to the folder asked from or absolute, and as Find names them, absolute;
// each cleaned, in the system's form (backslashes on Windows).
type Location struct {
	Top, GitDir, Common string
}

// In is the path p read from the folder dir: joined to it, unless p is
// absolute or dir is "", the process's own folder.
func In(dir, p string) string {
	if dir == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// Within is the path p read from the folder itos runs in, v's Folder.
func Within(ctx context.Context, v VCS, p string) string { return In(v.Folder(ctx), p) }

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

// Index names the index where a tree is asked for: the files it stages.
const Index = ":"

// Failure is why a command that reached for a remote failed, read from the
// VCS's own words: Unreachable, a remote out of reach, worth trying again;
// NoRepository, one that is no repository the VCS can find; Refused, a push
// the remote or a hook refused; Other, any other failure.
type Failure int

const (
	Other Failure = iota
	Unreachable
	NoRepository
	Refused
)

// Failed is a VCS command that ran and failed: the command as a person
// would type it, its exit code, its own words on stderr and, for one that
// reached for a remote, why (Failure). Its error says all three: "git push
// origin x: exit status 1: <words>".
type Failed struct {
	Command string
	Code    int
	Words   string
	Failure Failure
}

func (f *Failed) Error() string {
	return f.Command + ": exit status " + strconv.Itoa(f.Code) + ": " + strings.TrimSpace(f.Words)
}

// Conflicts is why a change does not apply (CherryPick): the paths it
// conflicts in, in byte order.
type Conflicts struct {
	Paths []string
}

func (c *Conflicts) Error() string {
	return "the change conflicts in " + strings.Join(c.Paths, ", ")
}

// Talk is how a command that may talk to the person runs: in Dir, relative
// to the folder itos runs in unless absolute, "" for that one; its input Stdin, none when nil; its output and
// its own words to Stdout and Stderr, dropped when nil (Failed keeps the
// words either way); in the environment Env, itos's own when nil. NoPrompt
// never asks for credentials; Owns leaves an interrupt to the command and
// the editor it opens while it runs, so itos does not end first.
type Talk struct {
	Dir            string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Env            []string
	NoPrompt, Owns bool
}

// Pushing is how a push goes: Atomic, every ref or none; Quiet, without
// progress; NoVerify, without the pre-push hook; Lease, when set, the one
// ref the push may replace with a commit that does not build on the remote's.
type Pushing struct {
	Atomic, Quiet, NoVerify bool
	Lease                   *Lease
}

// Lease lets a push replace the remote's ref (a full name, refs/heads/…) when
// the remote has it at Expect, and refuses the ref otherwise, as
// git push --force-with-lease=<ref>:<expect> does. Every other ref of the
// push still has to be a fast-forward.
type Lease struct {
	Ref, Expect string
}

// Commit is a commit to make, of the index unless Only names paths, whose
// files are committed as the work tree has them and nothing else. Its
// message is Paragraphs, each a paragraph, else Message, as written, unless
// Args give it: Args are the person's own arguments to the commit, given
// through as they are (itos commit's). Quiet asks for no summary. A trailer
// the message already has is not added twice.
type Commit struct {
	Paragraphs []string
	Message    string
	Only       []string
	Args       []string
	Quiet      bool
}

// VCS is the repository itos runs in. A folder is relative to the folder
// itos runs in, "" for that one. Every method takes ctx first, which carries
// cancellation and nothing else; callers pass context.Background() until v9
// threads cancellation (ADR-0053). porttest.VCSContract holds what every
// one, the fake's and the adapter's, must do.
type VCS interface {
	// Folder is the folder itos runs in, absolute, which every folder and
	// path the VCS is given or gives relative is relative to; "" for the
	// process's own, which only a test that stands in it gives.
	Folder(ctx context.Context) string
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
	// UnpushedBases are the commits the unpushed commits of to grow from,
	// each a commit some remote branch has and none an ancestor of another;
	// adds is whether any commit of to is unpushed. A to that names no
	// commit adds with no base.
	UnpushedBases(ctx context.Context, to string) (bases []string, adds bool)

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

	// A tree is a revision, or Index; its files are named from the top.

	// File is the text of the file path at tree; not ok when the tree has
	// no such file, and an error when the tree cannot be read.
	File(ctx context.Context, tree, path string) (text string, ok bool, err error)
	// Files are the files the tree holds under the folder dir, "" for every
	// one, in byte order; none for a folder it lacks, and an error when the
	// tree cannot be read.
	Files(ctx context.Context, tree, dir string) ([]string, error)
	// Texts are the texts of the files at tree, by path; an error when the
	// tree lacks one or cannot be read.
	Texts(ctx context.Context, tree string, paths []string) (map[string]string, error)
	// Patch is the change a commit makes, as a patch; an error when rev
	// names no commit.
	Patch(ctx context.Context, rev string) (string, error)
	// Shown are the commits as the VCS shows them to a person, each its
	// message and its patch, with its itos note when notes is set; an
	// error when one names no commit.
	Shown(ctx context.Context, revs []string, notes bool) (string, error)
	// Ref is the object a ref names, an annotated tag's own and not its
	// commit; an error when it names none.
	Ref(ctx context.Context, ref string) (string, error)
	// Refs are the refs under prefix, each the object it names, and for
	// an annotated tag its ref with ^{} too, its commit; none when it holds
	// none.
	Refs(ctx context.Context, prefix string) (map[string]string, error)
	// Symref is the full name of the ref a symbolic ref names (a clone's
	// refs/remotes/origin/HEAD names refs/remotes/origin/main), asked of
	// the repository alone; not ok when the ref is no symbolic ref or does
	// not exist.
	Symref(ctx context.Context, ref string) (target string, ok bool)

	// The remote.

	// Upstream is where the branch pulls from and pushes to: its remote
	// and the remote's ref it merges, as its config sets them, else origin
	// and refs/heads/<branch>; set is whether the config sets them.
	Upstream(ctx context.Context, branch string) (remote, ref string, set bool)
	// Remotes are the remotes' names, in byte order; an error outside a
	// repository.
	Remotes(ctx context.Context) ([]string, error)
	// RemoteURL is the remote's address; an error when there is no such
	// remote.
	RemoteURL(ctx context.Context, remote string) (string, error)
	// RemoteRefs are the remote's refs, each the object it names and an
	// annotated tag's ^{} its commit, those refs alone when refs are given;
	// a Failed error when the remote cannot be read.
	RemoteRefs(ctx context.Context, remote string, refs []string, talk Talk) (map[string]string, error)
	// Fetch fetches refspec of the remote and gives the commit fetched; its
	// default refspecs, and nothing given, when refspec is "". A Failed
	// error when the fetch fails.
	Fetch(ctx context.Context, remote, refspec string, talk Talk) (string, error)
	// Push pushes the refspecs (<commit or ref>:<ref>) to the remote, forcing
	// none but the ref a Lease names, and that only while the remote has it at
	// the commit the lease expects; a Failed error when it fails.
	Push(ctx context.Context, remote string, specs []string, p Pushing, talk Talk) error
	// Rebase replays HEAD's commits onto onto; a Failed error when it
	// stops on a conflict, left in progress (Rebasing, Conflicted), or does
	// not run.
	Rebase(ctx context.Context, onto string, quiet bool, talk Talk) error
	// CherryPick makes a commit on onto that carries the change commit makes
	// against its first parent, as the VCS merges it: with commit's author,
	// author date and message and no itos note, committed by the person
	// running. No ref, index or work tree changes, and nothing is made when the
	// change does not apply: a *Conflicts error then, a Failed one when it
	// cannot run.
	CherryPick(ctx context.Context, commit, onto string) (string, error)
	// Merge merges rev into HEAD as a merge commit with the message, past the
	// commit hooks and whatever the VCS's settings say of a merge; nothing
	// when HEAD has rev. A HEAD that rev has all of is fast-forwarded to it
	// with ff, and gets a merge commit too without. A Failed error when it
	// makes none, a file the work tree does not track and the merge would
	// overwrite among the reasons: a conflict is then left in progress
	// (MergeHeads, Conflicted).
	Merge(ctx context.Context, rev, message string, ff, quiet bool, talk Talk) error
	// Reset sets HEAD, the branch it is on, the index and the work tree to the
	// commit, discarding what they held; an error when rev names no commit.
	Reset(ctx context.Context, rev string) error

	// Committing.

	// Commit makes the commit, through the hooks; a Failed error when it
	// fails, nothing then committed.
	Commit(ctx context.Context, c Commit, talk Talk) error
	// Add stages the paths as the work tree has them; a Failed error when
	// it fails.
	Add(ctx context.Context, paths []string, talk Talk) error
	// Unstage sets the index's entries of the paths to HEAD's.
	Unstage(ctx context.Context, paths []string) error
	// Untrack takes the paths out of the index, leaving the work tree's
	// files; a path the index lacks is passed over.
	Untrack(ctx context.Context, paths []string) error
	// Restore sets the paths, in the index and the work tree, to HEAD's.
	Restore(ctx context.Context, paths []string) error
	// Note sets a commit's itos note (NotesRef) to text, replacing the one
	// it has.
	Note(ctx context.Context, rev, text string) error
	// CommitFile makes a commit outside every branch: parent's files ("" for
	// none, and no parent) with path set to text, its message message, by
	// the author name <email>. It gives the commit.
	CommitFile(ctx context.Context, parent, path, text, message, name, email string) (string, error)

	// Init makes the folder itos runs in a repository.
	Init(ctx context.Context) error
	// NotIgnored is whether the path is in the work tree and not ignored.
	NotIgnored(ctx context.Context, path string) bool
}
