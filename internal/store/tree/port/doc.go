// Package port is the tree reader's port (ADR-0053, step 6 of
// docs/plans/slices-and-ports.md): Trees opens a Source over the working
// tree, the index or a commit, where itos reads its own data (the config,
// the ledger, the registry, the people, the smoke sets, the decision
// records), and reads a git tree's files under a folder at once. Its
// in-memory fake and contract suite are in porttest, its adapter over the
// disk and git in internal/store/tree/adapter, laid out as the private
// store's (internal/store/private/port).
package port
