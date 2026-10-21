// Package source is where itos reads its own data: the config, the ledger,
// the work registry, the people, the smoke sets and the decision records.
// Every reader of them goes through Has, Read and List, which read the
// Current source: Worktree, the working tree's files, by default, or, while
// a function runs under ReadingFrom, a tree git holds, At("index") or
// At(<commit>). So the commit-msg hook's check of the staged data is config
// check itself, run under ReadingFrom, and the two cannot disagree.
//
// The config is read from the source when it holds it and where it is
// otherwise (an ITOS_CONFIG outside the repository), and a ledger folder a
// git tree lacks is the folder-missing error.
//
// The reading itself is the tree reader's port (internal/store/tree/port),
// bound here to its adapter, each call under context.Background(). The
// package-level source and these functions stay until T-211 threads a
// port.Trees value through the readers and removes them.
package source
