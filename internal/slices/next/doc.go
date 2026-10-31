// Package next is the next command (slice 203, features/next.feature), the
// first slice of the deterministic coordinator (docs/plans/v9-coordinator.md):
// an item's state and who acts next, read from the facts the repository
// holds, so a driver asks itos for the step, runs it, and asks again, and no
// session has to remember where an item stands. Its CLI and Run are the
// command line, its Handler the application over the Ports it owns, which
// internal/cli binds: the config every command finds, the tracker store
// the questions of itos decision are in, and, under push.mode pr, the forge
// (gh, the repository push names) and the attempt journal.
//
// # The facts
//
// Facts (facts.go) are read where their existing readers read them, so next
// cannot disagree with them: the item's registry entry as written and its
// dependencies' statuses from the registry in the working tree (work.Load,
// work.Show), the open questions naming the item from the questions file in
// the working tree (ask.Load through the tracker), a slice's or a bug's
// tagged scenarios at HEAD (tests.Tagged, as work done reads them, @wip ones
// included; under push.mode pr at origin/<id> when the clone has that ref
// and it holds some, else at HEAD, nothing fetched, slice-209) and a task's
// ledger checks from the ledger in the working tree (ledger.Tasks). An item's kind is its kind key; with none, its id gives
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
// in-flight (a spec present), the stand-in for the states only the forge
// knows. Each state gives an actor and an action, and an argv only when the
// step is a deterministic itos command: needs-spec's is itos brief <id>
// --role drafter, which next names and never runs.
//
// # The forge
//
// Under push.mode pr an item in flight asks the forge (forge.go, slice-209,
// features/next-forge.feature) through the Forge port: gh pr list --state
// all for the branch named for it, whose open pull request is the item's,
// else a merged one, a closed one counting for nothing; for an open one the
// one question --ready and review post put (providers.AskPullState), which
// gives its head, its comments, its reviews with their commits, its threads,
// its head's statusCheckRollup and whether auto-merge is enabled for it
// (slice-216), from which the reviewers push.pr.reviewers
// names that were not requested for the head are read (Unrequested, by
// internal/engine/reviewers, as itos review request reads them);
// and the attempt journal, whose records next reads the role, attempt,
// outcome, head and verdict of. The verdict that stands is the latest spec
// marker of the item at the head by a login that counts (work.ReadVerdict,
// work.Registry.Counts), as review post counts them. A gh that is missing or
// fails, or an answer without the shape asked for, is next-forge-failed,
// exit 3, and no repository is no-github-repository, exit 3; a journal that
// cannot be read, or a line of it that is no record, is the error. Judge's
// forge then gives, the first that holds: merged, repair (ready, a check
// failed or a thread open), review-unrequested (slice-215: ready, a reviewer
// push.pr.reviewers names not requested for the head as the reviewers engine
// reads it), merging (ready, auto-merge on), handed-back (ready, auto-merge
// off, whatever the status and the journal: slice-216, since --ready marks
// the pull request ready before it turns auto-merge on, and a driver told
// merging would wait on nothing), blocked "attempt" (the latest
// implementer record asks the person, needs_decision or blocked, and the
// k-th question naming the item, k its place among such records, is not
// answered), spec-changes (the latest implementer record a spec_defect with
// no drafter record after it), review-unrequested where handed-back would
// be, handed-back (doing, the latest implementer record accepted done at the
// head), implementing (doing), spec-accepted
// and spec-changes (todo, by the verdict), spec-review (todo, a pull
// request), spec-proposed (todo, none); --json's observed says what was
// read.
//
// The revision (revision.go) is sha256: and the hex digest of exactly the
// facts read, the kind as read among them, under push.mode pr the ref the
// scenarios were read at and what the forge and the journal said, with the
// questions naming the item, one line each in a fixed order,
// so a commit that touches none of them leaves it as it was and a change to
// any, a ledger.id that no longer makes an id a task's included, gives a
// new one; a driver
// hands it back to a mutating command as the state it acted on
// (docs/plans/v9-coordinator.md, rerun safety): RevisionOf is the revision
// over the facts read as Handle reads them, which the assembly binds as
// work take's and work done's Revision port (slice 208, --expect), so the
// two cannot drift: it asks the forge as next does, and a forge that cannot
// be asked is its error, of kind Missing.
package next
