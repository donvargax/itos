// Package guides is what itos guide and itos go share beside the guides
// themselves (internal/guide), an engine (ADR-0053, step 9 of
// docs/plans/slices-and-ports.md): the notes printed after the
// coordinator's guide and the commit policy printed after the commit
// procedure (slice 119). They read the config the command found (Config),
// locate the repository through the VCS port, read files and write
// nothing; what cannot be read is a warning on the stderr they are given,
// and the guide prints without it.
//
// Compose is a guide as printed: the coordinator's (itos go and itos guide
// coordinate) followed, after a line of --- (Separator), by the
// repository's own notes (Notes): the file guide.orchestrating names
// (docs/ORCHESTRATING.md by default; under the git folder for a stealth
// config, as its other data), read where the config's paths are read, or,
// with no config, from the repository's top (git rev-parse --show-cdup, so
// the path stays relative: upToTop), and outside a repository not at all.
// For itos go it then appends this clone's own notes (LocalNotes, slice 68):
// notes.md in itos's folder of the git common dir, which git never commits
// and every linked worktree shares, for what holds on this machine only,
// under LocalNotesHeading; a file missing or holding only blanks prints
// nothing, and one that is the file guide.orchestrating resolved to prints
// once, as the repository's notes (os.SameFile). CommitPolicy is the commit
// types, footers and paths the config sets (internal/engine/agentrules'
// CommitPolicy), under a heading of their own.
package guides
