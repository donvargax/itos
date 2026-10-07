// Package lock is the one lock itos holds while it changes a file that more
// than one itos may write at once: itos followup's threads, itos draft's
// list and, under a stealth config, the registry (which every registry and
// ledger writer reads and writes) and the questions of itos decision beside
// it, each in the git common dir, which every linked worktree of the clone
// shares. A writer reads the whole file, changes it and saves it whole, so
// two at once lose one's change unless the lock is held across all three:
// the command line takes it from before the read to after the write
// (heldThreads, heldDrafts, heldAsks, soundRegistry in internal/cli). itos
// work done gives it back while it runs the task's checks and asks CI, and
// takes it again to make its change on the registry as it then is. A
// project's registry takes none: its writes are commits, and git orders
// them.
//
// The lock is a file beside the data, the data's name with ".lock" after
// it, made with O_EXCL by Hold: only one process can make it, on Linux,
// macOS and Windows alike (flock is not on Windows), and git's own
// index.lock is made the same way. A writer that finds it there retries
// with a growing pause up to Wait (10 seconds); one that waits too long is
// refused with an error naming the lock file and saying to remove it when no
// itos is running, for an itos killed while it held the lock leaves it
// behind. Release removes it.
package lock
