package next

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "next",
	Rank: 35,
	Lines: `  next <id>                        an item's state and who acts next, from the repository; reads only
`,
	Help: map[string]string{
		"": `Usage: itos next <id>

Prints an item's state and the next step, read from the facts the repository
holds (the first slice of the deterministic coordinator,
docs/plans/v9-coordinator.md): the item's registry entry, read from the
working tree, its dependencies' statuses, the open questions of itos decision
naming it (work.asks, from the working tree), a slice's or a bug's scenarios
at HEAD (those tagged @<id>, live or @wip, as work done reads them) and a
task's checks in its ledger entry. It writes no file, makes no commit and asks
no forge and no remote; git is read through tree and object reads alone, never
git status. An item's kind is its kind key; with none, its id says: slice-<n>
a slice, bug-<n> a bug, an id ledger.id matches a task.

The states, the first that holds winning: done (status done); parked
(dropped, or deferred); blocked (an open question names the item, or its
status is blocked); waiting (an item in its depends_on is not done); idea
(kind idea, or an item of no kind, which has no spec itos reads either);
needs-spec (a slice or a bug no scenario is tagged for, or a task whose
ledger entry has no done_when check, or none); in-flight (a spec present).
A repository whose config declares no scenario kind has no scenario for any
item, so a slice or a bug there needs a spec. In-flight stands in for the
states only the forge knows (spec review, implementing, merging, merged),
which a later slice adds.

Each state gives who acts next and what: done and parked, none none;
blocked, person answer; waiting, none wait; idea, person promote; needs-spec,
drafter specify, run as itos brief <id> --role drafter; in-flight, forge
observe. A command is named only when the step is a deterministic itos
command, and never run.

One line: "<id>: <state>; next: <actor> <action>", then "; run: <command>"
when there is one. Exit 0 in every state; 1 when there is no registry where
itos looks, or no item with the id (rule work-unknown-item, as work show
refuses it); 2 when the registry, the ledger a task needs or the questions
file cannot be read.

The registry's blocked_by: a YAML list of ids says what a blocked item waits
on, and its ids are the item's waits_on; prose says why in words, as the
registry's header describes it, and names nothing.

--json: {"schema":1,"ok":true,"item","kind","state","owner","next":{"actor","action","argv"?},
"waits_on"?,"blocked_reason"?,"revision"}: kind slice, task, idea or bug, or
null for none; owner the item's own as written, null when it is null, never
its group's; argv only for a deterministic itos command; waits_on only what
the state waits on: the open questions' ids for an item a question blocks,
the ids blocked_by lists for one whose status is blocked (none when it is
prose or absent), the undone dependencies' ids for a waiting one;
blocked_reason on a blocked item alone, "decision" for an open question and
"status" for the registry's status blocked, the question winning when both
hold; revision "sha256:" and the hex digest of exactly the facts read, the
kind as read among them, so a commit that touches none of them leaves it as
it was and a config change that moves the step moves it. Refused:
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{},
}
