// Package nextid is the next free ID of a series: itos tests next-id's tag
// and itos task next-id's task ID, each one past the highest number the
// series already holds, as internal/ask's NextID gives a question its id, so
// that a writer of specs never greps for one.
//
// Next is one past the highest <prefix><digits>, as wide as the widest the
// series writes; a fresh ID- stem of an ID- pattern starts two digits wide,
// and a ledger ID is widened until ledger.id matches it. The callers gather
// the series: tests.NextTag reads every word of a tag line of the kind's
// feature files that starts with the tag prefix (a command adapter's listed
// IDs instead) and the work registry's item ids, since slice-<n> and bug-<n>
// items are named before their scenarios; ledger.NextID reads the ledger's
// IDs and the registry's that ledger.id matches, since work promote names a
// task before the ledger holds it. Both read the working tree only: itos
// tests next-id of a scenario area then claims the tag through the id
// counter (internal/idcounter), past the counter as well as the files.
package nextid
