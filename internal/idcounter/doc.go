// Package idcounter reserves numbered ids before a command writes the item,
// or before tests next-id prints a scenario area's: Mint one, Claim a run of
// them in a row, raising the counter past the last in one reservation.
// A project with a remote reserves from refs/itos/ids on the same remote itos
// pushes to, through the VCS port (Store.VCS: RemoteRefs, Fetch, File,
// CommitFile, Push): each reservation is a counter-tree commit pushed by
// fast-forward only. A push the remote refuses after its counter moved past
// the commit the push was built on is retried from the newly fetched counter,
// so two clones never hand out the same number; one that leaves the counter as
// it was is refused. The ref and refspec are constants in this package; its
// only hook bypass is the push of that counter ref, which changes no branch or
// tag.
//
// In stealth mode, or when there is no remote, counters live in a file under
// the git common directory's itos folder. The file lock and atomic replacement
// make that fallback coherent across linked worktrees. Reservations are never
// returned, so a later validation or write failure may leave a gap. The file
// is read as every YAML file itos reads is (value.Parse), and a hand-edited one
// that Parse refuses or whose counter is no whole number from 0 to 2^53 - 1 is
// refused, never read lower than written.
//
// It is the tracker store's adapter's internals: the tracker port's Reserve
// (internal/store/tracker/adapter) claims through it, and depguard's
// idcounter-internals refuses it to any other package's code.
package idcounter
