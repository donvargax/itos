// Package gitrun is how itos runs git for a command the person typed
// (ADR-0053, step 9 of docs/plans/slices-and-ports.md), which commit and
// push share. Talking runs it with the terminal's stdin, so an editor can
// open, in the folder the person stood in (slice 40), owning an interrupt
// while it runs; ExitOf hands back git's exit code, 1 for a git a signal
// ended, and the error of one that could not start. NoVerify reads
// --no-verify as git does, its abbreviations down to --no-veri among them,
// for both to refuse (slice 128).
//
// Under a stealth config the links are a commit's note in refs/notes/itos
// (slice 32): WriteNote writes it on the commit git just made, replacing
// what an amend carried over, and RewriteNotes adds that ref to
// notes.rewriteRef in the repository's own config unless a value already
// names it, so an amend or a rebase carries the note to the commit it
// makes. The repository is the VCS port, and the config the Config the
// caller binds, which the port leaves to the adapter.
package gitrun
