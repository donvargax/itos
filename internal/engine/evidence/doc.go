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
// The commit-msg hook (internal/slices/hook) uses the engine too, with Hook
// set, on the tree it captured (the Git it is given answers IndexTree with it,
// so a commit of paths keys the temporary index git hands the hook). Hit asks,
// without running or printing, whether a record would answer, so the hook
// makes a checkout only when some check must run; Spare hands it that
// checkout for the first miss when no check shares it. A request carries the
// seconds the hook's cap allows beside the check's own, which alone the key
// binds, so a run the cap stops is not recorded and a record made by hand
// answers it. A quiet run's output goes to a temporary file, not a pipe, as
// check.RunCaptured's does, so a process a check leaves running cannot hold
// the run, or the commit, past its timeout. The hook's engine prints only the line of a hit, naming the
// task, and a check it cannot key or run in a checkout of the tree is refused
// (Outcome.Err), never run in the working folder.
//
// Elsewhere nothing that goes wrong changes a verdict or an exit code. A check that
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
// is a miss, which lookup answers without a lock; the writer that follows takes
// the record's lock and, before its rename and again when the rename fails,
// judges what is at the place (an OS may let a rename replace a file). A whole record, which another writer made first, is kept as it is: the
// check has run and answers with its own result, recording nothing, its
// evidence saying already-recorded and carrying the kept record's time. Anything
// else is renamed aside to <key>.<reason>-<UTC timestamp>, the reason markerless
// for a folder without its marker and corrupt for the rest, and the rename is
// tried once more. A rename or a quarantine that cannot be done is a store that
// cannot be written: the check ran, and nothing is recorded. A temporary folder
// is never read, whatever it holds, and nothing here removes an old one.
//
// The package is an engine (internal/engine): several commands share it, itos
// task and ci run through internal/check's Runner and ci's driver, and no slice
// owns it. The engines rule of .golangci.yml therefore refuses it a command,
// a slice, a store's or the VCS's adapter, the launcher and the shim, and a
// port's fake in its code; a slice and internal/ci import it directly. It
// reaches the repository through Git and the store's place and lock through
// the private store's port, which internal/cli binds (newEvidence). It does use
// the operating system for the working folder's inputs and the store's files.
package evidence
