// Package port is the version control system's port (ADR-0053, step 6 of
// docs/plans/slices-and-ports.md): VCS, what itos asks of the repository it
// runs in, so that a backend beside git (Fossil, p2-vcs-and-tracker-seams)
// is one more adapter. Its in-memory fake and contract suite are in
// porttest, its git adapter in internal/vcs/adapter, laid out as the
// stores' ports (internal/store/<store>/port).
//
// It holds locating the repository (Locate, and Find, which the git shim
// asks on every command and so reads files alone) and making one (Init),
// resolving a revision to its commit, ancestry (IsAncestor, Parent,
// Parents, FirstParents) and ranges (Range: Commits, Readable,
// UnpushedBase, UnpushedBases), commits as a log reads them (Entry: Show, Log, with the
// itos note of NotesRef) and the comment character, changed paths (a
// commit's, a merge's own, between two commits, a range's, the index's)
// and the work tree's state (Changes, PathStatus, Tracked, Conflicted,
// Rebasing, MergeHeads, Branch, NotIgnored); the files at a tree (File,
// Files, Texts, Index the index's), a commit's patch (Patch, Shown) and the
// refs (Ref, Refs, Symref); the remote (Upstream, Remotes, RemoteURL, RemoteRefs,
// Fetch, Push, Rebase), whose failures are a Failed with its Failure; and
// committing (Commit, Add, Unstage, Untrack, Restore, Note, and CommitFile,
// a commit outside every branch, as the ID counter makes one; CherryPick
// makes one the same way from another's change, a *Conflicts error when it
// does not apply, and Reset moves HEAD to a commit). A command
// that may talk to the person runs as its Talk says. What is git's alone
// (its config and the hooks it declares, a draft's patch plumbing, finding
// the real git) is the adapter's, beside the port.
package port
