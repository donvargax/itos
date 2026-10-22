// Package port is the version control system's port (ADR-0053, step 6 of
// docs/plans/slices-and-ports.md): VCS, what itos asks of the repository it
// runs in, so that a backend beside git (Fossil, p2-vcs-and-tracker-seams)
// is one more adapter. Its in-memory fake and contract suite are in
// porttest, its git adapter in internal/vcs/adapter, laid out as the
// stores' ports (internal/store/<store>/port).
//
// It holds locating the repository (Locate, and Find, which the git shim
// asks on every command and so reads files alone), resolving a revision to
// its commit, ancestry (IsAncestor, Parent, Parents, FirstParents) and
// ranges (Range: Commits, Readable, UnpushedBase). The rest of what itos
// asks of git joins it operation group by operation group.
package port
