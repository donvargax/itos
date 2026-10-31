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
at HEAD (those tagged @<id>, live or @wip, as work done reads them; under
push.mode pr at origin/<id> when the clone has that ref and it holds some, else
at HEAD) and a task's checks in its ledger entry. It writes no file, makes no
commit and fetches nothing; git is read through tree and object reads alone,
never git status. An item's kind is its kind key; with none, its id says:
slice-<n> a slice, bug-<n> a bug, an id ledger.id matches a task.

The states, the first that holds winning: done (status done); parked
(dropped, or deferred); blocked (an open question names the item, or its
status is blocked); waiting (an item in its depends_on is not done); idea
(kind idea, or an item of no kind, which has no spec itos reads either);
needs-spec (a slice or a bug no scenario is tagged for, or a task whose
ledger entry has no done_when check, or none); in-flight (a spec present).
A repository whose config declares no scenario kind has no scenario for any
item, so a slice or a bug there needs a spec. Under push.mode trunk in-flight
stands for the states only the forge knows, and nothing is asked of it.

Under push.mode pr an item in flight asks the forge, through gh and the
repository push names: gh pr list --state all for the branch named for the
item (its open pull request, else a merged one; a closed one counts for
nothing), then for an open one the one gh api graphql question --ready asks
(its head, comments, threads, the head's statusCheckRollup and whether
auto-merge is enabled for it, its autoMergeRequest), and reads the
attempt journal, itos/attempts/<id>.jsonl in the git folder. The states, the
first that holds winning: merged (driver close, itos work done <id>); repair
(ready, a check failed or a thread unresolved: implementer implement, itos
brief <id> --role implementer); review-unrequested (where merging or
handed-back would be, a reviewer push.pr.reviewers names not requested for the
head, neither asked with itos review request's marker nor answered by its own
review of the head: driver request, itos review request <id>); merging (ready,
auto-merge on: forge observe); handed-back (ready, auto-merge off, whatever
the item's status or the journal says: driver land, itos push --ready);
blocked (the
latest implementer record is needs_decision or blocked and the k-th question
naming the item, k its place among such records, is not answered: person
answer, blocked_reason attempt; the person's step is itos decision add
<question> --item <id>, then itos decision answer); spec-changes (the latest
implementer record is spec_defect and no drafter record follows it: drafter
specify); handed-back (doing, the pull request a draft, the latest
implementer record accepted done at its head: driver land, itos push
--ready); implementing (doing: implementer implement); spec-accepted (todo,
the latest counted spec verdict marker at the head approves: driver take,
itos work take <id>); spec-changes (todo, it asks for changes: drafter
specify); spec-review (todo, a pull request and no verdict at its head:
reviewer review, itos brief <id> --role reviewer); spec-proposed (todo, no
pull request: drafter specify). A verdict marker counts as review post counts
it: of this item's spec stage, by a login among the people, any login with
none.

Each state gives who acts next and what: done and parked, none none;
blocked, person answer; waiting, none wait; idea, person promote; needs-spec,
drafter specify, run as itos brief <id> --role drafter; in-flight, forge
observe. A command is named only when the step is a deterministic itos
command, and never run.

One line: "<id>: <state>; next: <actor> <action>", then "; run: <command>"
when there is one. Exit 0 in every state; 1 when there is no registry where
itos looks, or no item with the id (rule work-unknown-item, as work show
refuses it); 2 when the registry, the ledger a task needs, the questions file
or the attempt journal cannot be read; 3 when gh is missing or fails (rule
next-forge-failed, naming the call and gh's words) or no GitHub repository is
named (no-github-repository).

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
blocked_reason on a blocked item alone, "decision" for an open question,
"status" for the registry's status blocked, the question winning when both
hold, and "attempt" for an implementer's result asking the person; observed
when the forge was asked: {"pr":{"number","draft","head","merged"} or null,
"verdict":"approve"|"changes"|null, "checks":"pending"|"failure"|"success"|
"none", "threads_open", "attempt":{"role","attempt","verdict","outcome",
"head"} or null, the journal's latest implementer record, "unrequested", the
push.pr.reviewers logins, as written, not requested for the open pull
request's head, [] for none, "auto_merge", whether auto-merge is enabled for
the open pull request, false when none was asked about}; revision "sha256:"
and the hex digest of exactly the facts read, the kind as read among them and
under pr the ref the scenarios were read at and what the forge and the
journal said, so a commit that touches none of them leaves it as it was and a
config change that moves the step moves it. Refused:
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{},
}
