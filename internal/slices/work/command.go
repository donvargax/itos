package work

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "work",
	Rank: 50,
	Lines: `  work [--as <handle>]             what the person can start, and what waits
  work list [--all] [--tag <tag>]  the open items of the work registry; --all every one
  work show <id> [--patch]         an item, its scenarios and its commits; reads only
  work take <id> [--as <handle>]   set an item in progress for the person, and commit it
  work promote <idea> --kind slice|task [--title <title>] […]
                                   make an idea a slice or a task (and its ledger entry), and commit it
  work done <id>                   mark an item done once it has landed, and commit it
  work add [<idea-id>] --title <title> --why <why> [--kind idea|slice|task|bug] […]
                                   add an item to the work registry, and commit it
  work edit <id>… [--title <title>] [--depends-on <ids>] [--refs <refs>] [--tags <tags>] [--note <text>]
                                   change an item, or add a note to its why, and commit it
  work queue <id>… --top|--before <id>|--after <id>|--remove
                                   order the work: put an item in the queue, or take it out
  work drop <id>… --why <reason>   take an item out of the open work, and commit the reason
  work defer <id>… --why <reason>  put an item off, and commit the reason
  work resume <id>…                lift an item's deferral, and commit it
  work give-back <id>… --why <reason> [--as <handle>]
                                   put an item in progress back to todo, and commit the reason
  work check [<file>]              validate the work registry
`,
	Help: map[string]string{
		"": `Usage: itos work [--as <handle>]
       itos work list [--all] [--tag <tag>]
       itos work show <id> [--patch]
       itos work take <id> [--as <handle>]
        itos work promote <idea> --kind slice|task [--title <title>] […]
       itos work done <id>
        itos work add [<idea-id>] --title <title> --why <why> [--kind idea|slice|task|bug] […]
       itos work edit <id>… [--title <title>] [--depends-on <ids>] [--refs <refs>] [--tags <tags>] [--note <text>]
       itos work queue <id>… --top|--before <id>|--after <id>|--remove
       itos work drop <id>… --why <reason>
       itos work defer <id>… --why <reason>
       itos work resume <id>…
       itos work give-back <id>… --why <reason> [--as <handle>]
       itos work check [<file>]

Who the session works for (--as, else the config's work.identity provider) and
their items: in progress, can start now, unowned, waiting, ideas, deferred,
each its id, its tags in brackets when it carries any, and its title.
Each list is in the order of the registry's queue (itos help work queue), the
items it does not name after, in the registry's order; what another person
owns is in none, so each person sees their part of the queue.
Exit 1 when the registry is not sound, 3 when --as is not among the people.
With no people to read (a stealth config, or a people file missing or
unreadable) any handle is taken, and nothing is said of them. Under a stealth
config with no --as, no identity is looked up: every item is the session's,
whatever owner it names (every_item: true in --json).

--json: the proposal {"schema":1,"person","every_item"?,"doing","next","unowned","waiting","ideas","deferred"},
each list in the queue's order, each item with its tags ([] for none)

work list prints the registry's open items instead, and --all every item
(itos help work list), work show one item with its scenarios and commits (itos
help work show); work take, work promote, work done, work add, work edit, work
queue, work drop, work defer, work resume and work give-back write it, each
committing it (itos help work take).`,

		"show": `Usage: itos work show <id> [--patch]

Prints the item (its title, kind, status, owner, phase, depends_on, the items
depending on it, refs and why, a task's why read from its ledger entry first,
its registry why after it unless the two read the same),
its scenarios at HEAD (those tagged @<id>, @slice-<n> for the item slice-<n>,
live or @wip) and its commits, oldest first,
each with its short SHA and header, and between them the questions of itos
decision naming the item (itos help decision), each with its id, open or
answered, and its text. A commit is the item's when its footers of IDs name the item (Task:) or
one of its scenarios (Scenarios:), a later fix naming one included, or when it
is one of itos's registry commits naming it in its header (docs: take <id>,
docs: promote <idea> to <id>, docs: close <id>, docs: add <id>, docs: edit
<id>, docs: queue <id>, docs: drop <id>, docs: defer <id>, docs: resume <id>,
docs: give back <id>);
under a stealth config the footers are read from each commit's note in
refs/notes/itos. --patch adds each commit as git show prints it, message and
diff: the whole of a review's input. It reads, never writes, and judges
nothing, as work list. Exit 1 when there is no registry where itos looks, or no
item with the id; 2 when itos decision's file is there and cannot be read.

--json: {"schema":1,"ok":true,"item":{…},"ledger_why"?:"<why>"|null,"depended_on_by":[…],"scenarios":[{"id","file","live"}],
"questions":[{"id","item","question","status","answer"?}],
"commits":[{"sha","short","header","message"?,"patch"?}]}, message and patch with --patch;
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"take": `Usage: itos work take <id> [--as <handle>]

Sets the item in progress for the person, its status doing and its owner the
person (--as, else the config's work.identity provider, as for work), and
commits the registry alone, "docs: take <id>", through the hooks, leaving
whatever else is staged staged. The registry is edited in place, its comments
and quoting kept. Refused, nothing written (exit 1): a registry that is not
sound or has changes no commit holds, an id no item has, an idea (work promote
it first), an item done, dropped or deferred, one whose owner, or its group's,
is someone else, one that waits on an item not done, a commit a hook refuses
(the registry put back). An item already in progress for the person changes
nothing. Exit 3 when the person is nobody or not among the people. Under a
stealth config the registry is written and nothing committed; with no --as
every item is the session's, and its owner is left as it is.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"promote": `Usage: itos work promote <idea> --kind slice [--title <title>]
       itos work promote <idea> --kind task --type <type> --check <command> [--timeout <seconds>]… [--title <title>] [--why <why>]

Makes an idea a slice or a task: mints an ID from the shared counter, renames
it, sets its kind and renames it in every depends_on and every question
(work.asks) that names it. A slice's why is led by "Was <idea>."; a task's
moves to its ledger entry, which the promote writes at the end of the file of
the idea's group: the --type, the checks in order (each --check, its
--timeout after it, as task add reads them), and the idea's why unless --why
gives another. No check is run. It then commits the registry, with the ledger
file and the questions when they changed, "docs: promote <idea> to <id>", as
work take does (itos help work take). --title gives it a new title in the same
commit, whose body says so; without it the idea's title is kept.
Refused before an ID is minted, nothing written (exit 2): a caller-supplied
ID, a task with no --type or no --check, a --timeout that is not a number
above 0 after its --check, and --why, --type, --check or --timeout for a
slice. Refused before an ID is minted, nothing written (exit 1): a registry
that is not sound or has changes no commit holds, questions that cannot be
read, or that name the idea and have changes no commit holds, an id no item
has, an item that is not an idea, an idea dropped; for a task, a type
commits.types does not list, a ledger with problems, a group ledger.files
cannot name, an idea with no why and no --why, a ledger file with changes no
commit holds. Refused after, the files put back (exit 1): a commit a hook
refuses. Under a stealth config the files are written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"was","rewritten":[…],"task"?:{…},"file"?,"questions"?:[…],"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"done": `Usage: itos work done <id>

Marks the item done once its work has landed, its status done, its owner left
as it is and its why dropped (the registry is an index: a slice's why lives in
its feature file, a task's in its ledger entry, the text in git's history),
and commits the registry alone, "docs: close <id>", as work take does (itos
help work take). Closing a task with checks, the same commit takes its
done_when out of its ledger entry, its why kept: the checks were its progress
while it was open and gate the close, and the commit before it still holds
them. A check meant to keep running belongs in ci.steps or ci.nightly.steps.
Landed is: none of the item's scenarios at HEAD
(those tagged @<id>, @slice-<n> for the item slice-<n>, @bug-<n> for bug-<n>)
still @wip; each of the item's own commits on a remote (itos push them); a
task's static checks passing; and with ci.watch, a CI run that has the newest
of its own commits passed. Its own
commits are those whose footers name it (Item:, Task:, or Scenarios: naming one
of its scenarios), as itos work show lists them, its registry commits aside.
Commits no remote has that are not the item's are passed over, so several
closes land in one itos push after the last. The run judged is the newest
commit's own when it passed, else any run on its branch whose head has it and
passed: a later run's range leaves the item's commits out, so a later red is
not the item's. With none, done waits for the commit's run as itos ci watch
waits, or for the newest run whose head has it, and names it. An item with no
commit of its own is judged by HEAD: no commit of HEAD that no remote has, and
HEAD's CI run passed, the commits at HEAD that touch only the work registry,
pushed or not, passed over and the newest commit that touches more judged;
pushed with them, that commit may have no run of its own, and the newest run
on its branch whose head has it judges it. Without ci.watch CI is not
checked, and done says so.
A task's static checks are every done_when check whose cost is static, in the
order written, a static check written below a late one included (with
ci.cost.keep_written_order a check below a late one is late itself), each under
its own timeout (its timeout:, else ledger.check.timeout; hooks.commit_msg.check_timeout
caps the commit-msg hook alone), stopping at the first that fails, whose output
done prints under the task and its check as itos task does. The late checks
are CI's, which runs every check of the tasks a push names: done never runs
them, and says each is left to CI. An after: push check stays pending, neither
run nor looked up. A check that opts in to evidence is looked up by HEAD's
tree, never the index's, and answers from a record a hand run or the commit-msg
hook left there ("cached evidence"); a miss runs in a checkout of HEAD's tree
under the git common dir's itos/snapshots and is recorded, and one whose
checkout cannot be made runs uncached in the working folder, its exit its own.
A staged change or an unstaged edit reaches neither the run nor the key. A
check without evidence runs in the working folder, in the commit-msg hook's
environment, and so does one that says snapshot: false (its evidence ignored,
nothing looked up or recorded). The checkout has the repository's git context:
HEAD, history, refs, remotes and the branch's upstream. These checks are local feedback: a record answers done and proves
nothing to CI.
The code proof of the config's proof.code is not run here: CI's run, which
done requires green, judges it over the whole range (itos ci run --help).
Refused, nothing written (exit 1): any of those not so, a registry, or the
ledger file of a task with checks, that is
not sound or has changes no commit holds, an id no item has, an idea (work
promote it first), an item dropped or deferred, a commit a hook refuses. Exit
75 when the run does not end within ci.watch.timeout, is cancelled with no
newer run to follow, or GitHub's API keeps failing, 3 when the run cannot be
looked at. An item already done changes nothing.
An item the registry's queue holds is then taken out of it, in a commit of its
own, "docs: queue <id>". Under a stealth config the registry and the ledger
are written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"ci":"success"|"unwatched","run"?,"evidence"?:[{"command","hit","recorded",…}],"left_to_ci"?:[{"task","command"}],"queue_commit"?:"<sha>"|null,"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}],"ci"?,"run"?,"evidence"?};
a failing check's problem (rule work-done-task-check) adds "task", "command",
"exit" (null when it did not exit by itself), "timed_out" and "output", the
tail of what it printed. "evidence" is there once an opted-in check was looked
up, in run order; "left_to_ci" names each late check of a task closed. The
late checks are named on stderr under --json.`,

		"add": `Usage: itos work add [<idea-id>] --title <title> --why <why> [--kind idea|slice|task|bug]
         [--phase <phase>] [--owner <handle>] [--depends-on <id>,…] [--refs <ref>,…]
         [--tags <tag>,…]

Adds an item at the end of the work registry, todo: an idea unless --kind says
otherwise. An idea keeps its required <idea-id>; a slice, task or bug takes a
minted ID from the shared counter, a bug's bug-<n> past the registry's bugs and
the feature files' @bug-<n> tags. It is in the phase --phase names (else the
one a p<n>- idea id names, else the registry's only one), owned by --owner or
nobody, carrying the tags --tags gives, its why a folded text, then commits the
registry alone, "docs: add <id>", as work take does (itos help work take).
Refused, nothing written (exit 2): an ID given for a slice, task or bug, or none
for an idea.
Refused, nothing written (exit 1): a registry that is not sound or has changes
no commit holds, an <id> an item already has, no phase to put it in, what work
check would find with the item there (a phase not listed, an
owner not among the people, a dependency on no item, a tag work.tags does not
declare, any tag when it declares none), a commit a hook refuses. Under a
stealth config the registry is written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

		"edit": `Usage: itos work edit <id>… [--title <title>] [--depends-on <id>,…] [--refs <ref>,…]
         [--tags <tag>,…] [--note <paragraph>]

Changes an item: its title, its depends_on, its refs or its tags, each
replaced (an empty --depends-on, --refs or --tags empties the list), or adds
--note's paragraph to its why, after a blank line in a folded why; then
commits the registry alone, "docs: edit <id>", as work take does (itos help
work take). An owner changes by work take, a kind by work promote, a status by
work take and work done. Refused, nothing written (exit 1): a registry that is
not sound or has changes no commit holds, an id no item has, --note on a slice
or a task (the registry keeps an idea's why alone: a slice's belongs in its
feature file, a task's in its ledger entry), what work check would find with
the item so (a dependency on no item, a cycle, an item done waiting on one not
done, a tag work.tags does not declare), a commit a hook refuses. An item
already as asked changes nothing. Under a stealth config the registry is
written and nothing committed.

Several ids each take the same change (--depends-on, --refs, --tags,
--note; --title is one item's own, exit 2 with several), every one judged
before anything is written: when any one would be refused, nothing is
written and each id refused is named (exit 1). Then one commit holds them
all, "docs: edit <id>, <id>…", or "docs: edit <n> items" when the ids do not
fit the header, its body naming every item changed; an item already as
asked is left out of it. An id named twice exits 2.

--json: {"schema":1,"ok":true,"item":{…},"changed":["title"|"depends_on"|"refs"|"tags"|"why",…],"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"drop": `Usage: itos work drop <id>… --why <reason>

Takes an item out of the open work: its status dropped, a status itos knows
whatever work.statuses lists, its why dropped as work done drops a closed
item's, and the item taken out of the registry's queue; then commits the
registry alone, "docs: drop <id>", as work take does (itos help work take),
the reason the commit's body: the commit is the record of why. The item stays
in the registry, so its id is never given again; work never proposes it, work
take refuses it, and work check reports an item still to do that depends on
it. Refused, nothing written (exit 1): a registry that is not sound or has
changes no commit holds, an id no item has, an item done, an item that an item
neither done nor dropped depends on (naming those: edit their depends_on, or
drop them, first), a commit a hook refuses. An item already dropped changes
nothing. Exit 2 without --why. Under a stealth config the registry is written
and nothing committed.

Several ids are each dropped for the one reason, in the order given, each
judged with the ids before it dropped (so name an item after those that
depend on it), all before anything is written: when any one would be
refused, nothing is written and each id refused is named (exit 1). Then one
commit holds them all, "docs: drop <id>, <id>…", or "docs: drop <n> items"
when the ids do not fit the header, its body naming every item dropped, then
the reason. An id named twice exits 2.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"defer": `Usage: itos work defer <id>… --why <reason>

Puts an item off: its deferred key the reason, a folded text, then commits the
registry alone, "docs: defer <id>", as work take does (itos help work take),
the reason the commit's body. The item keeps its status, owner, tags and place
in the queue; work and status list it apart, among the deferred, and work take
and work done refuse it until work resume lifts the deferral. Refused, nothing
written (exit 1): a registry that is not sound or has changes no commit holds,
an id no item has, an item already deferred (naming its reason: resume it
first), an item done or dropped, one of any other status but todo, a commit a
hook refuses. Exit 2 without --why. Under a stealth config the registry is
written and nothing committed.

Several ids are each deferred for the one reason, every one judged before
anything is written: when any one would be refused, nothing is written and
each id refused is named (exit 1). Then one commit holds them all, "docs:
defer <id>, <id>…", or "docs: defer <n> items" when the ids do not fit the
header, its body naming every item, then the reason. An id named twice
exits 2.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"resume": `Usage: itos work resume <id>…

Lifts an item's deferral: its deferred key removed, nothing else of it
changed; then commits the registry alone, "docs: resume <id>", as work take
does (itos help work take), the reason it was deferred named in the commit's
body. Refused, nothing written (exit 1): a registry that is not sound or has
changes no commit holds, an id no item has, an item that is not deferred, a
commit a hook refuses. Under a stealth config the registry is written and
nothing committed.

Several ids are each resumed, every one judged before anything is written:
when any one would be refused, nothing is written and each id refused is
named (exit 1). Then one commit holds them all, "docs: resume <id>, <id>…",
or "docs: resume <n> items" when the ids do not fit the header, its body
naming every item and the reason it was deferred. An id named twice exits 2.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"give-back": `Usage: itos work give-back <id>… --why <reason> [--as <handle>]

Gives back an item in progress, undoing work take: its status todo again,
its owner kept and nothing else of it changed; then commits the registry
alone, "docs: give back <id>", as work take does (itos help work take), the
reason the commit's body. The item can then be taken again, or deferred (itos
help work defer). The person is work take's (--as, else the config's
work.identity provider). Refused, nothing written (exit 1): a registry that
is not sound or has changes no commit holds, an id no item has, an item not
in progress (naming its status), one whose owner, or its group's, is someone
else (naming them), a commit a hook refuses. Exit 2 without --why, 3 when the
person is nobody or not among the people. Under a stealth config the registry
is written and nothing committed; with no --as every item is the session's.

Several ids are each given back for the one reason, every one judged before
anything is written: when any one would be refused, nothing is written and
each id refused is named (exit 1). Then one commit holds them all, "docs:
give back <id>, <id>…", or "docs: give back <n> items" when the ids do not
fit the header, its body naming every item, then the reason. An id named
twice exits 2.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"queue": `Usage: itos work queue <id>… --top|--before <id>|--after <id>|--remove

Orders the work: the registry's queue, a top-level queue: list of item ids,
one for the whole repository, ideas included, says what comes first, and itos
work proposes in its order. --top puts the item first, --before and --after
just before or after an item the queue holds, --remove takes it out; an item the
queue holds already is moved. The queue is written whole as a block list
before items:, then the registry is committed alone, "docs: queue <id>" for
insertions or "docs: dequeue <id>" for --remove. work done takes a closed item out.
Refused, nothing written (exit 1): a registry that is not sound or has changes
no commit holds, an id no item has, an item done or dropped, an item placed
before or after itself or an item the queue does not hold, a commit a hook
refuses. An item already where it is put, or not in the queue for --remove,
changes nothing. Under a stealth config the registry is written and nothing
committed.

Several ids go there together, in the order given: --top puts them first,
--before and --after at that place, beside an item they do not name, and
--remove takes each out. Every id is judged before anything is written: when
any one would be refused, nothing is written and each id refused is named
(exit 1). Then one commit, "docs: queue <id>, <id>…" or "docs: dequeue <id>, <id>…",
or the matching "docs: queue <n> items" / "docs: dequeue <n> items" when
the ids do not fit the header, its body naming every item. An id
named twice exits 2.

--json: {"schema":1,"ok":true,"item":{…},"queue":[…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}; for several
ids {"schema":1,"ok":true,"items":[{…},…],"queue":[…],"commit":"<sha>"|null},
or {"schema":1,"ok":false,"refused":[<id>,…],"problems":[…]}`,

		"list": `Usage: itos work list [--all] [--tag <tag>]

Prints the open items of the work registry (the config's work.registry, by
default work-items.yaml in the ledger's folder), every one whose status is
neither done nor dropped (blocked and any status work.statuses adds among
them), in its order, whatever their kind or owner: each one's id, kind, status
and title, "-" for a kind or status it does not give, and (deferred) after the
title of one deferred. --all prints every item, done and dropped ones too
(until v4.0.0, plain work list printed every item). --tag <tag> prints only
the items carrying the tag. It judges nothing, so a registry that is not sound
still lists; exit 1 when there is no registry where itos looks.

--json: {"schema":1,"file","items":[{"id","title",…}]}, the items it would
print, each with its fields as work --json writes it`,

		"check": `Usage: itos work check [<file>]

Validates the work registry (the config's work.registry, by default
work-items.yaml in the ledger's folder, or <file>): that it is there,
duplicate IDs, unknown groups (the owners per group under work.groups_key,
phases by default), statuses (those work.statuses lists), kinds, owners, tags
(a list of those work.tags declares, each once; none when it declares none),
dependencies, cycles, and the queue (a list of item ids, none unknown, none
twice). Exit 1 on a problem. The messages call a group by ledger.group.label
(phase by default).

--json: {"schema":1,"file","sound","problems":[{"rule","message","fix"?}]}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Value("--as")}, Subs: map[string]declare.Spec{
		"list": {Flags: []declare.Flag{declare.Switch("--all"), declare.Value("--tag")}},
		"show": {Flags: []declare.Flag{declare.Switch("--patch")}},
		"take": {Flags: []declare.Flag{declare.Value("--as")}},
		"promote": {Flags: []declare.Flag{declare.Value("--id"), declare.Value("--kind"), declare.Value("--title"), declare.Value("--why"), declare.Value("--type"),
			declare.Values("--check"), declare.Values("--timeout")}},
		"done": {},
		"add": {Flags: []declare.Flag{declare.Value("--title"), declare.Value("--why"), declare.Value("--kind"), declare.Value("--phase"), declare.Value("--owner"),
			declare.Value("--depends-on"), declare.Value("--refs"), declare.Value("--tags")}},
		"edit":      {Flags: []declare.Flag{declare.Value("--title"), declare.Value("--depends-on"), declare.Value("--refs"), declare.Value("--tags"), declare.Value("--note")}},
		"queue":     {Flags: []declare.Flag{declare.Switch("--top"), declare.Switch("--remove"), declare.Value("--before"), declare.Value("--after")}},
		"drop":      {Flags: []declare.Flag{declare.Value("--why")}},
		"defer":     {Flags: []declare.Flag{declare.Value("--why")}},
		"resume":    {},
		"give-back": {Flags: []declare.Flag{declare.Value("--why"), declare.Value("--as")}},
		"check":     {},
	}},
}
