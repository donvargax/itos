// Package port is the tracker store's port (ADR-0053, step 6 of
// docs/plans/slices-and-ports.md): Tracker, the documents itos tracks work
// in (the registry, the ledger files, the questions, the decision records),
// read, listed, and changed together in one transaction that writes them
// and, in a project, commits them alone. It is storage level, never a
// method per record kind, so typed items or another tracker (Fossil's
// tickets, p2-vcs-and-tracker-seams) fit behind the same methods. Its
// in-memory fake and contract suite are in porttest, its adapter over the
// disk and the VCS in internal/store/tracker/adapter, laid out as the
// private store's (internal/store/private/port).
package port
