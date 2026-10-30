// Package next is the next command (slice 203, features/next.feature), the
// first slice of the deterministic coordinator (docs/plans/v9-coordinator.md):
// an item's state and who acts next, read from the facts the repository
// holds, so a driver asks itos for the step, runs it, and asks again, and no
// session has to remember where an item stands. Its CLI and Run are the
// command line, its Handler the application over the Ports it owns, which
// internal/cli binds: the config every command finds, and the tracker store
// the questions of itos decision are in.
//
// # The facts
//
// Facts (facts.go) are read where their existing readers read them, so next
// cannot disagree with them: the item's registry entry as written and its
// dependencies' statuses from the registry in the working tree (work.Load,
// work.Show), the open questions naming the item from the questions file in
// the working tree (ask.Load through the tracker), a slice's or a bug's
// tagged scenarios at HEAD (tests.Tagged, as work done reads them, @wip ones
// included) and a task's ledger checks from the ledger in the working tree
// (ledger.Tasks). An item's kind is its kind key; with none, its id gives
// it by next's own rule, since work show reads no bug: slice-<n> a slice,
// bug-<n> a bug, an id ledger.id matches a task (and with no ledger.id, none
// is), else none. Only a slice's or a bug's scenarios and a task's checks are
// read: the other kinds have no spec itos reads. A registry that is not
// there, or an id no item has, is refused as work show refuses it, exit 1;
// what cannot be read (a registry, a ledger or a questions file that is not
// sound) is an error, never a state. It reads through tree and object reads
// alone, never git status, which rewrites the index.
//
// # The judgement
//
// Judge (judge.go) is pure over the Facts: the first state that holds wins,
// in the order done (status done), parked (dropped, or a deferred key),
// blocked (an open question names the item, blocked_reason decision and
// waits_on the questions' ids; else status blocked, blocked_reason status
// and waits_on the ids blocked_by lists when it is a list, none when it is
// prose, as the registry's header describes it, or absent), waiting (a
// dependency not done, waits_on those), idea (kind idea, or no kind next
// reads, which has no spec either), needs-spec (a slice or a bug with no
// tagged scenario, a task with no done_when check or no ledger entry) and
// in-flight (a spec present), this slice's stand-in for the states only the
// forge knows, the next slice's. Each state gives an actor and an action,
// and an argv only when the step is a deterministic itos command:
// needs-spec's is itos brief <id> --role drafter, which next names and never
// runs.
//
// The revision (revision.go) is sha256: and the hex digest of exactly the
// facts read, one line each in a fixed order, so a commit that touches none
// of them leaves it as it was and a change to any gives a new one; a driver
// hands it back to a mutating command as the state it acted on
// (docs/plans/v9-coordinator.md, rerun safety).
package next
