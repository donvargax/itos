// Package ids is the ids engine (ADR-0053): the next free number of an
// item's kind or of a scenario area's stem, reserved through the tracker's
// Reserve so two machines never take the same one, which work add, work
// promote, task add, a command draft and tests next-id share.
//
// Mint is an item's id: a task's past the ledger's and the registry's task
// ids (ledger.NextID, ledger.NextIDAfter, ledger.id the one authority for
// its prefix and padding), a slice's or a bug's past the registry's ids and
// the feature files' tags of its kind, as wide as the widest of them. Claim
// is a run of an area's tags, next's number its floor, each as wide as next.
// Items and Registry are the registry's ids, which name an item before its
// scenarios or its ledger entry exist; Registry none when the config's
// registry is not there. The counter behind Reserve is the tracker's
// adapter's (internal/store/tracker/adapter), tested there; this package's
// tests run over the tracker's fake.
package ids
