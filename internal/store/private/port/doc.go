// Package port is the clone's private store's port (ADR-0053): Store, the
// files itos keeps for one clone alone, by name, in itos's folder of the
// git common dir, so git never commits them and every linked worktree of
// the clone shares them, with the lock that orders their writers and
// whether a path the person names is one git would commit.
//
// It is the first of docs/plans/slices-and-ports.md's ports, placed where
// the tracker store of its step 6 grows from it: internal/store/<store>
// holds a store's port, its in-memory fake and contract suite in
// port/porttest, and its adapter over the disk and git in adapter. A slice
// imports the port and, in its tests alone, the fake; internal/cli binds the
// adapter. .golangci.yml's store rules and tools/bin/architecture-check hold
// that: the port and fake import nothing of ours and do no I/O, an
// adapter's code links no fake and no slice, and no binary links a fake.
package port
