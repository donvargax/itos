// Package adapter is the tracker store's adapter
// (internal/store/tracker/port) over the disk and the VCS: Files, the
// documents as files of the working tree, each named by its path. Load and
// List read the disk directly, never through the tree reader, which
// rewords a read's error.
//
// Transact is the order itos has always written and committed its own
// files in (bug 13, bug 17, slice 91): the hooks ready (HooksReady), then
// each file clean, as HEAD has it, a deletion not committed counting as a
// change; each file written in its mode, 0644 for a new one, a write that
// fails putting back the files before it; under a stealth config, which
// commits nothing, done there. Else each created file staged (git add, so
// the commit's --only knows it), the commit of those files alone through
// the hooks, the index's entries of them set to the commit's (a hook that
// rewrites a file, a formatter, leaves its version in the commit and the
// working tree, which the index then matches), and HEAD resolved. A step
// that fails after the first write puts every file back as it was, file
// and index, a created one removed and unstaged; one git refused is a
// *port.Refused naming the step, any other failure an error saying what it
// could not do. Transact gives no error an exit kind: the hooks' error
// keeps HooksReady's, the lock's internal/lock's.
//
// Reserve is internal/idcounter's Claim, which is this adapter's internals
// (depguard's idcounter-internals refuses it to any other package's code):
// on the remote itos pushes to, else, and always under a stealth config,
// in the git common dir. The one kind the adapter gives an error is
// kind.Unknown, exit 70, for a git failing: Reserve's when it cannot locate
// the repository, which is not the missing repository (exit 3) the locate's
// own kind says (bug-119), and Ready's when git status fails, which is not
// a file with changes (bug-130), the error holding git's words. The
// counter's errors keep idcounter's kinds, a remote it cannot reach exit 3.
//
// HooksReady is called by Ready alone, and Reserve locates when it
// reserves, never when Files is built: a git that fails would refuse
// earlier, and with another exit than the step that asks (bug-119). Hold is
// internal/lock's lock of Lock under a stealth config, where every linked
// worktree writes the same files, else at once: a project's commits are
// ordered by git's index lock: the one lock (Lock, the registry's) covers
// every document, so a writer of the questions holds it as the registry's
// writers do. internal/cli binds Files (tracker.go) and reads through it the
// documents it writes, the ledger handed its Load, and reserves through it
// the IDs it mints; its tests run porttest.TrackerContract, as the fake's
// do.
package adapter
