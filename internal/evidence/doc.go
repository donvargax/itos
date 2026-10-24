// Package evidence is the local evidence records of ADR-0060: an opted-in
// check reruns from a record of its last run when nothing it ran on has
// changed, the way Go's test cache answers, and runs in a checkout of the tree
// it is keyed by, so that what it answers is what the next commit holds.
//
// # Opting in
//
// A done_when check, or a run: step of ci.steps, opts in with an evidence
// mapping (config.Evidence): inputs, paths from the repository's top (a folder
// covers every file under it) that git does not track or that the check wants
// named; env, names of variables hashed by value; tools, programs resolved on
// the PATH and hashed by content. {} opts in with nothing declared. A check
// with none is never looked up or recorded; it runs as it always did.
//
// # The key
//
// Manifest is everything a key binds, and Key its SHA-256: the index tree the
// check runs on, its command (whitespace collapsed) and timeout, the
// declaration, a hash of the config's settings after defaults (never its path
// or any runtime field, so every worktree shares the records), itos's version,
// the OS and architecture (Binding), the declared environment and, when ci run
// applies ci.env, ci.env's names and values, the declared inputs git does not
// track, and the declared tools. A declared path git tracks is the index's, and
// the tree covers it; an untracked or ignored one is hashed by its content
// (a link by its target, an executable bit with its file), a missing one as
// absent, which is not an empty file, and a path that is none of file, link or
// folder cannot be keyed. The mode, run: or fails:, is not bound: a record
// holds the raw exit status and each check reads its verdict from it, as
// check.Key already shares one run between them.
//
// # Running
//
// An Engine is one invocation: it captures the index once (Git.IndexTree),
// counts the tracked paths whose working copy differs from it, which it says
// and which --json carries as unstaged, and looks each opted-in check up.
// A hit answers with the recorded exit status and output tail and makes no
// checkout. A miss takes the record's lock (the private store's Hold of
// evidence/<key>, internal/lock's), looks again, since a writer that waited may
// find the record its holder made, makes a checkout of the index tree of its own
// (bug-112's task snapshot; never shared between checks) with the declared
// untracked inputs copied in, runs the check there with the checkout's
// environment, and writes the record.
//
// Nothing that goes wrong changes a verdict or an exit code. A check that
// cannot be keyed, whose checkout cannot be made (a gitlink in the index, the
// snapshots folder unusable, an input that cannot be copied) or that the
// index cannot be read for runs in the working folder, uncached, with the
// reason on its Info; a lock held past its wait or a store that cannot be
// written runs it in its checkout, unrecorded; a timeout, or a command that
// did not exit by itself, is run and not recorded, since it is not a result
// to reuse. Info.Recorded is true only on the run that wrote the record.
//
// # The store
//
// Records live in itos/evidence in the git common dir, never in the tree. A
// record is the folder <key> holding manifest.json, result.json and the marker
// complete, written in a folder named .tmp-<key>-<unique> with every file
// synced and the marker last, then renamed into place, never overwriting one.
// A record is trusted only whole: its marker there, its manifest of this schema
// hashing to the key its folder is named by, its result readable. Anything else
// is a miss, and the rename that follows a run finds the folder taken and
// records nothing. What the store does under contention and damage beyond
// that, quarantining a record found broken, is the next slice's
// (features/evidence-store.feature).
//
// The Engine reaches the repository through Git and the store's place and lock
// through the private store's port, which internal/cli binds (newEvidence), so
// that this package imports no adapter.
package evidence
