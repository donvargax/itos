// Package idcounter reserves numbered ids before a command writes the item.
// A project with a remote reserves from refs/itos/ids on the same remote itos
// pushes to: each reservation is a counter-tree commit pushed by fast-forward
// only. A failed non-fast-forward push is retried from the newly fetched
// counter, so two clones never hand out the same number. The ref and refspec
// are constants in this package; its only hook bypass is the push of that
// counter ref, which changes no branch or tag.
//
// In stealth mode, or when there is no remote, counters live in a file under
// the git common directory's itos folder. The file lock and atomic replacement
// make that fallback coherent across linked worktrees. Reservations are never
// returned, so a later validation or write failure may leave a gap.
package idcounter
