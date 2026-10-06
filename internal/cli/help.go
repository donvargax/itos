package cli

import (
	"fmt"
	"strings"
)

// The help texts, help.ts ported: what `itos --help` and each command's
// --help print, one place for all of them rather than one per command.
// Agents read these, so tools/itos/conformance/help.yaml holds every one
// byte for byte, and a change here is the same change to help.ts.

const globalFlags = `Global flags:
  --config <file>   the config (default ITOS_CONFIG, else itos.yaml, else the
                    stealth mode's <git common dir>/itos/itos.yaml; in a
                    subfolder with no itos.yaml, the repository top's, and
                    itos runs as if started there)
  --root <dir>      run as if started in <dir>
  --json            one object on stdout ("schema": 1, keys only ever added); logs on stderr
  --no-color        no colour (itos prints none; passed on as NO_COLOR)
  -q, --quiet       leave out a check's success line
  -h, --help        this text, or a command's`

const versions = `Versions: a config's pin (pin.version, pin.checksums) runs that itos release,
fetched from ITOS_RELEASES into ITOS_CACHE and checked first; ITOS_VERSION
runs another. A project's config with no pin runs this binary; no config, or a
stealth one with no pin, runs the newest release, asked for once a day, never
in CI or with ITOS_NO_UPDATE=1; ITOS_NO_UPDATE_NOTICE=1 hides the notice that a
pin has fallen behind it.`

const otherCommands = `Other commands: one itos does not have runs itos-<command> from the PATH, its
exit code the run's, with the arguments after its name unread; global flags
before the name apply first and reach it as ITOS_CONFIG, ITOS_ROOT, ITOS_JSON,
ITOS_BIN and ITOS_VERSION. ` + "`itos help <command>`" + ` runs ` + "`itos-<command> --help`" + `.`

const asGit = `As git: linked as git before the real git on the PATH (itos git-shim install),
itos runs git commit and git push as itos commit and itos push in a repository
with an itos config, from any folder of it, and the real git for anything else.`

const exitCodes = `Exit codes: 0 success; 1 policy failure (a check failed, a commit rejected,
a registry or smoke problem, an unknown task, a failing step of ci run);
2 usage or config error; 3 missing environment (--as not among the people, a
release the server does not have or that fails its check, an extension that
cannot start, a git shim with no git to run); 70 an error itos cannot
classify: report it; 75 a failure that may pass when run again unchanged (a
release server or GitHub's API that cannot be reached or fails, a CI run not
finished within ci.watch.timeout).`

const mainCommands = `itos: tasks, their checks, commit rules and CI plans

Usage: itos <command> [args] [global flags]

Commands:
  go                               the coordinator's guide, the repository's own notes, the status
  guide coordinate|work            a guide shipped in itos: the coordinator's, the implementer's
  status [--as <handle>]           where things stand: head, CI, release, work, questions
  task <id>…                       run the tasks' checks; the status table
  task list [--group <g>]          the tasks and their work items' status; runs nothing
  task add <id> --group <g> --type <type> --title <title> --why <why> --check <command> […]
                                   add a task to the ledger and its item, and commit both
  task next-id                     the ledger's next free task ID; reads only
  work [--as <handle>]             what the person can start, and what waits
  work list [--all]                the open items of the work registry; --all every one
  work show <id> [--patch]         an item, its scenarios and its commits; reads only
  work take <id> [--as <handle>]   set an item in progress for the person, and commit it
  work promote <idea> --id <id> --kind slice|task [--title <title>]
                                   make an idea a slice or a task, and commit it
  work done <id>                   mark an item done once it has landed, and commit it
  work add <id> --title <title> --why <why> […]
                                   add an item to the work registry, and commit it
  work edit <id> [--title <title>] [--depends-on <ids>] [--refs <refs>] [--note <text>]
                                   change an item, or add a note to its why, and commit it
  work queue <id> --top|--before <id>|--after <id>|--remove
                                   order the work: put an item in the queue, or take it out
  work drop <id> --why <reason>    take an item out of the open work, and commit the reason
  work check [<file>]              validate the work registry
  question [--all]                 the open questions to the person the work is for
  question add <text> [--item <id>]
                                   ask a question, and commit it
  question answer <id> <text>      answer a question, and commit it
  question record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>] [--none]
                                   write an answer as a decision record, and commit it
  question show <id>               a question and its answer
  followup [--all]                 your open threads with people; private, never committed
  followup add <id> --with <who> --title <title> --note <text>
                                   open a thread with someone
  followup note <id> <text>        add a dated note to a thread
  followup close <id> [--note <text>]
                                   close a thread
  followup show <id>               a thread and all its notes
  followup doc <id> <path> [--force]
                                   write a thread out as Markdown
  commit [--task <id>] [--item <id>] [--scenarios <ids>] [--<footer> <text>] [<git commit args>…]
                                   git commit, with the footers itos writes
  commit check-message <file|->    header lint and footer rules on one message
  commit check-paths --type <t> <path>…
                                   the commit type's path rules, for planning a split
  commit footers <name> <from> <to>
                                   a range's free-text footers, for the release notes
  push [--no-wait]                 pull with a rebase, push HEAD; with ci.watch, wait for CI
  git-shim install|uninstall [--dir <folder>]
                                   link itos as git, for itos's commit and push
  pin [<version>|latest]           move the config's pin to a release, the newest by default
  upgrade [<version>|latest]       move to a newer itos, listing what each release since asks
  init [--stealth] [--plugin [<scope>]] [--git-shim [--git-shim-dir <folder>] | --no-git-shim]
       [--agent-rules | --no-agent-rules]
                                   ready a repository for itos; run again, what is missing
  verify <from> <to>               re-check every commit of a range
  tests list <kind> [--at <tree>]  the kind's named tests
  tests smoke check|ids|run <kind> the smoke rule, the smoke IDs, the smoke run
  tests moves <kind>               the staged feature files by the moves rule
  tests next-id <kind> <stem>      the kind's next free tag of the stem; reads only
  ci plan <from> <to> | --nightly | --whole
                                   print the CI plan; runs nothing
  ci run [<from> <to>] | --nightly run the CI plan
  ci scope <from> <to>             docs_only=true|false, for GITHUB_OUTPUT
  ci range --head <sha> [--base <sha>]
                                   FROM=<sha>: where a push's range starts
  ci watch [<sha>]                 wait for a commit's CI run, HEAD's by default
  hook commit-msg <file>           the commit-msg hook: data, paths, header lint, task checks
  hook pre-push <remote> <url>     the pre-push hook: verify, then the tests the push reaches
  hook install [--manager <m>] [--print] [--force]
                                   write the hooks' one-line shims for the hook manager
  guard claude-code                Claude Code's PreToolUse hook: denies git commit and git push
  config check [--print-defaults]  validate the config, ledger, registry and smoke sets
  config get <key>                 a config key's value as itos reads it, defaults applied
  version [--check]                the version; --check against the config's requires
  --version                        itos version, as the first argument`

const mainHelpTail = globalFlags + `

` + versions + `

` + otherCommands + `

` + asGit + `

` + exitCodes

// mainHelp is itos's own help where the PATH has no extension; with one, the
// extensions are listed after the commands (extensionsHelp).
const mainHelp = mainCommands + "\n\n" + mainHelpTail

// helpTexts is each command path's help, keyed as help.ts's HELP is: the
// command and its subcommands joined by a space, "" for itos's own.
var helpTexts = map[string]string{
	"": mainHelp,

	"task": `Usage: itos task <id>… [--skip <ids>]
       itos task --group <g> [--skip <ids>]
       itos task --pending
       itos task list [--group <g>]
       itos task add <id> --group <g> --type <type> --title <title> --why <why> --check <command> […]
       itos task next-id

--phase <g> and --<label> <g> are --group <g>, the label being the config's
ledger.group.label (phase by default).

Runs each task's done_when checks in written order: verbose (each command and
its output) for one task, then one status line per task: done, pending (a check
waits on the push), failing or review (no checks). --pending shows the tasks
not done. A check that more than one task lists (the same command and timeout)
runs once, and each task reads its exit status by its own run: or fails:.
Exit 1 when a check fails or an ID is not in the ledger, 2 when nothing matches.

--json: {"schema":1,"tasks":[{"id","type","title","group","status","checks":[{"command","fails"?,"result"}]}]}`,

	"task list": `Usage: itos task list [--group <g>]

Lists the ledger's tasks (status, id, group, type, title) without running any
check. The status is the task's work item's in the work registry (the item
whose id is the task's), or "no item".

--json: {"schema":1,"tasks":[{"id","type","title","group","checks","status"}]}`,

	"task next-id": `Usage: itos task next-id

Prints the ledger's next free task ID: one past the highest number of the
tasks' IDs and of the work registry's item ids that ledger.id matches (work
promote names a task there before the ledger holds it), with the ledger's
padding and as wide as ledger.id asks (T-003 after T-001 and T-002). Help for
writing a task: it reads the working tree, writes nothing and judges nothing,
and a number only a removed task held is not seen.

--json: {"schema":1,"id"}`,

	"task add": `Usage: itos task add <id> --group <g> --type <type> --title <title> --why <why>
         --check <command> [--timeout <seconds>] [--check <command> [--timeout <seconds>]]…

Adds a task at the end of its group's ledger file (ledger.files with {group}
filled in, made when the group has none yet): its id, type, title, why (a
folded text) and done_when, one run: check per --check in order, a --timeout
after a --check giving that check its own seconds. Adds its item to the work
registry, a task, todo and nobody's, as work add does, then commits the ledger
file and the registry alone, "docs: add <id>", as work take commits the
registry (itos help work take); a commit a hook refuses puts both back.
--phase <g> and --<label> <g> are --group <g>. Refused, nothing written (exit
1): an <id> ledger.id does not match or the ledger already has, a <type>
commits.types does not list, a group ledger.group.pattern does not match, a
ledger with problems (itos config check), a registry that is not sound, what
work add refuses (an <id> an item has, a group the registry does not list),
either file with changes no commit holds. Under a stealth config both are
written and nothing committed.

--json: {"schema":1,"ok":true,"task":{…},"file":"<ledger file>","item":{…},"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work": `Usage: itos work [--as <handle>]
       itos work list [--all]
       itos work show <id> [--patch]
       itos work take <id> [--as <handle>]
       itos work promote <idea> --id <id> --kind slice|task [--title <title>]
       itos work done <id>
       itos work add <id> --title <title> --why <why> […]
       itos work edit <id> [--title <title>] [--depends-on <ids>] [--refs <refs>] [--note <text>]
       itos work queue <id> --top|--before <id>|--after <id>|--remove
       itos work drop <id> --why <reason>
       itos work check [<file>]

Who the session works for (--as, else the config's work.identity provider) and
their items: in progress, can start now, unowned, waiting, ideas, deferred.
Each list is in the order of the registry's queue (itos help work queue), the
items it does not name after, in the registry's order; what another person
owns is in none, so each person sees their part of the queue.
Exit 1 when the registry is not sound, 3 when --as is not among the people.
With no people to read (a stealth config, or a people file missing or
unreadable) any handle is taken, and nothing is said of them. Under a stealth
config with no --as, no identity is looked up: every item is the session's,
whatever owner it names (every_item: true in --json).

--json: the proposal {"schema":1,"person","every_item"?,"doing","next","unowned","waiting","ideas","deferred"},
each list in the queue's order

work list prints the registry's open items instead, and --all every item
(itos help work list), work show one item with its scenarios and commits (itos
help work show); work take, work promote, work done, work add, work edit, work
queue and work drop write it, each committing it (itos help work take).`,

	"work show": `Usage: itos work show <id> [--patch]

Prints the item (its title, kind, status, owner, phase, depends_on, the items
depending on it, refs and why, a task's why read from its ledger entry first),
its scenarios at HEAD (those tagged @<id>, @slice-<n> for the item slice-<n>,
live or @wip) and its commits, oldest first,
each with its short SHA and header, and between them the questions of itos
question naming the item (itos help question), each with its id, open or
answered, and its text. A commit is the item's when its footers of IDs name the item (Task:) or
one of its scenarios (Scenarios:), a later fix naming one included, or when it
is one of itos's registry commits naming it in its header (docs: take <id>,
docs: promote <idea> to <id>, docs: close <id>, docs: add <id>, docs: edit
<id>, docs: queue <id>, docs: drop <id>); under a stealth config the footers are read from each commit's note in
refs/notes/itos. --patch adds each commit as git show prints it, message and
diff: the whole of a review's input. It reads, never writes, and judges
nothing, as work list. Exit 1 when there is no registry where itos looks, or no
item with the id; 2 when itos question's file is there and cannot be read.

--json: {"schema":1,"ok":true,"item":{…},"ledger_why"?:"<why>"|null,"depended_on_by":[…],"scenarios":[{"id","file","live"}],
"questions":[{"id","item","question","status","answer"?}],
"commits":[{"sha","short","header","message"?,"patch"?}]}, message and patch with --patch;
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work take": `Usage: itos work take <id> [--as <handle>]

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

	"work promote": `Usage: itos work promote <idea> --id <id> --kind slice|task [--title <title>]

Makes an idea a slice or a task: renames it <id>, sets its kind, puts
"Was <idea>." before its why and renames it in every depends_on that names it,
then commits the registry alone, "docs: promote <idea> to <id>", as work take
does (itos help work take). --title gives it a new title in the same commit,
whose body says so; without it the idea's title is kept. Refused, nothing
written (exit 1): a registry that is not sound or has changes no commit holds,
an id no item has, an item that is not an idea, an idea dropped, an <id> an
item already has, a task's <id> that ledger.id does not match, a commit a hook
refuses. Under a stealth config the registry is written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"was","rewritten":[…],"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work done": `Usage: itos work done <id>

Marks the item done once its work has landed, its status done, its owner left
as it is and its why dropped (the registry is an index: a slice's why lives in
its feature file, a task's in its ledger entry, the text in git's history),
and commits the registry alone, "docs: close <id>", as work take does (itos
help work take). Landed is: none of the item's scenarios at HEAD
(those tagged @<id>, @slice-<n> for the item slice-<n>) still @wip; no commit
of HEAD that no remote has (itos push them); a task's static checks passing, as
the commit-msg hook runs them; and with ci.watch, HEAD's CI run passed, waited
for as itos ci watch waits when it is still going. Without ci.watch CI is not
checked, and done says so. Refused, nothing written (exit 1): any of those not
so, a registry that is not sound or has changes no commit holds, an id no item
has, an idea (work promote it first), an item dropped or deferred, a commit a
hook refuses. Exit 75 when the run does not end within ci.watch.timeout or
GitHub's API keeps failing, 3 when the run cannot be looked at. An item
already done changes nothing. An item the registry's queue holds is then
taken out of it, in a commit of its own, "docs: queue <id>". Under a stealth
config the registry is written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"ci":"success"|"unwatched","run"?,"queue_commit"?:"<sha>"|null,"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}],"ci"?,"run"?}`,

	"work add": `Usage: itos work add <id> --title <title> --why <why> [--kind idea|slice|task]
         [--phase <phase>] [--owner <handle>] [--depends-on <id>,…] [--refs <ref>,…]

Adds an item at the end of the work registry, todo: an idea unless --kind
says otherwise, in the phase --phase names (else the one a p<n>- id names,
else the registry's only one), owned by --owner or nobody, its why a folded
text, then commits the registry alone, "docs: add <id>", as work take does
(itos help work take). Refused, nothing written (exit 1): a registry that is
not sound or has changes no commit holds, an <id> an item already has, a
task's <id> that ledger.id does not match, no phase to put it in, what work
check would find with the item there (a phase not listed, an owner not among
the people, a dependency on no item), a commit a hook refuses. Under a stealth
config the registry is written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work edit": `Usage: itos work edit <id> [--title <title>] [--depends-on <id>,…] [--refs <ref>,…]
         [--note <paragraph>]

Changes an item: its title, its depends_on or its refs, each replaced (an
empty --depends-on or --refs empties the list), or adds --note's paragraph to
its why, after a blank line in a folded why; then commits the registry alone,
"docs: edit <id>", as work take does (itos help work take). An owner changes
by work take, a kind by work promote, a status by work take and work done.
Refused, nothing written (exit 1): a registry that is not sound or has changes
no commit holds, an id no item has, --note on a slice or a task (the registry
keeps an idea's why alone: a slice's belongs in its feature file, a task's in
its ledger entry), what work check would find with the item so (a dependency
on no item, a cycle, an item done waiting on one not done), a commit a hook
refuses. An item already as asked changes nothing. Under a
stealth config the registry is written and nothing committed.

--json: {"schema":1,"ok":true,"item":{…},"changed":["title"|"depends_on"|"refs"|"why",…],"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work drop": `Usage: itos work drop <id> --why <reason>

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

--json: {"schema":1,"ok":true,"item":{…},"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work queue": `Usage: itos work queue <id> --top|--before <id>|--after <id>|--remove

Orders the work: the registry's queue, a top-level queue: list of item ids,
one for the whole repository, ideas included, says what comes first, and itos
work proposes in its order. --top puts the item first, --before and --after
just before or after an item the queue holds, --remove takes it out; an item the
queue holds already is moved. The queue is written whole as a block list
before items:, then the registry is committed alone, "docs: queue <id>", as
work take does (itos help work take). work done takes a closed item out.
Refused, nothing written (exit 1): a registry that is not sound or has changes
no commit holds, an id no item has, an item done or dropped, an item placed
before or after itself or an item the queue does not hold, a commit a hook
refuses. An item already where it is put, or not in the queue for --remove,
changes nothing. Under a stealth config the registry is written and nothing
committed.

--json: {"schema":1,"ok":true,"item":{…},"queue":[…],"commit":"<sha>"|null}, or
{"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"work list": `Usage: itos work list [--all]

Prints the open items of the work registry (the config's work.registry, by
default work-items.yaml in the ledger's folder), every one whose status is
neither done nor dropped (blocked and any status work.statuses adds among
them), in its order, whatever their kind or owner: each one's id, kind, status
and title, "-" for a kind or status it does not give, and (deferred) after the
title of one deferred. --all prints every item, done and dropped ones too
(until v4.0.0, plain work list printed every item). It judges nothing, so
a registry that is not sound still lists; exit 1 when there is no registry
where itos looks.

--json: {"schema":1,"file","items":[{"id","title",…}]}, the items it would
print, each with its fields as work --json writes it`,

	"work check": `Usage: itos work check [<file>]

Validates the work registry (the config's work.registry, by default
work-items.yaml in the ledger's folder, or <file>): that it is there,
duplicate IDs, unknown groups (the owners per group under work.groups_key,
phases by default), statuses (those work.statuses lists), kinds, owners and
dependencies, cycles, and the queue (a list of item ids, none unknown, none
twice). Exit 1 on a problem. The messages call a group by
ledger.group.label (phase by default).

--json: {"schema":1,"file","sound","problems":[{"rule","message","fix"?}]}`,

	"commit": `Usage: itos commit [--task <id>] [--item <id>] [--scenarios <ids>]
                   [--<footer> <text|none>] [--breaking <text>] [<git commit args>…]
       itos commit check-message <file|-> [--at <sha>]
       itos commit check-paths --type <type> <path>…
       itos commit footers <name> <from> <to>

Runs git commit with every argument but its own flags, and writes the footers
from them as git's --trailer, so they land whether the message comes from -m,
-F or the editor. --task writes the footer whose source is the ledger (Task:
here), --item the one whose source is the work registry (Item: here; in
place of Task: for the types its in_place_of names) and --scenarios the one
whose source is a kind of named tests, the first of commits.footers each,
their IDs separated by commas or spaces and written on lines of 100
characters at most, the key on each. Each footer of free text (source: text)
has a flag of its name in lower case (--upgrading for Upgrading:), unless
--task, --item, --scenarios or --breaking has that name, and --breaking
writes BREAKING-CHANGE:, the form git reads as a trailer; their value is the
next argument, whatever it is. Every flag may repeat, and a footer the
message already has is not written again (trailer.ifExists addIfDifferent),
so an amend keeps one.

Before git runs, a commit whose type requires a footer that neither the flags
nor the message give is refused, naming the flag, and nothing is committed;
the type is read from -m, from -F <file>, or for --amend --no-edit from HEAD's
message. A message from the editor, or -F -, is left to the hooks, which judge
the commit as any other, so a commit they refuse is not made; ITOS_AMEND tells
them whether --amend is among git's arguments, so they know an amend rather
than guess one. Under a stealth config (<git common dir>/itos/itos.yaml) the
links, the footers --task, --item and --scenarios write, are never written
into the message: they are handed to the hook in ITOS_FOOTERS and written as
the new commit's note in refs/notes/itos, and notes.rewriteRef is set to that
ref, so an amend or a rebase carries the note; footers of free text and
BREAKING-CHANGE stay in the message.
-q is git's --quiet. A first argument that is not a flag is a subcommand or a
usage error, never a path: paths for git go after a flag or after --, as in
itos commit --task T-1 -m '…' -- README.md. Exit: git's own code; 1 when a
required footer is missing; 2 when the first word is no subcommand, a flag
has no value or the config no footer for it.

--json: git's output on stderr; {"schema":1,"ok","commit"?}, or when a footer
        is missing {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"commit check-message": `Usage: itos commit check-message <file|-> [--at <sha>]

Runs the header lint (commits.header_lint: itos's own with use: builtin, else
the stdin delegate), if any, and itos's footer rules (commits.footers),
always, on one message, read from <file> or stdin (-), and reports both. --at
reads the footers' IDs at that commit. Under a stealth config the footers of
IDs are the ones ITOS_FOOTERS gives, never the message's, and a footer of
free text is the message's. Exit 1 when either rejects the message; the
built-in lint's warnings alone do not.

--json: {"schema":1,"ok","problems":[{"rule","message","fix"?,"level"}]}`,

	"commit check-paths": `Usage: itos commit check-paths --type <type> <path>…

Applies the type's path rules (commits.scopes) to the paths and prints what
they reject, without committing. A path is read from the folder itos is run
in, as git reads one, and named from the repository's top. Exit 1 when a path
is rejected.

--json: {"schema":1,"type","files","ok","problems":[{"rule","message","fix"}]}`,

	"commit footers": `Usage: itos commit footers <name> <from> <to>

Lists the footers <name> that the range's non-merge commits carry (from may be
empty or all zeros: every commit up to <to>), where commits.footers.<name> is
free text (source: text): what each commit asks of a consumer, gathered for a
release's notes. One line per footer, oldest commit first, its commit's short
SHA and its text; a commit with several gives a line for each. A footer that
says none (the word alone, in lower case) asks nothing and is left out, as is
an empty one. The footers are each commit's message's, under a stealth config
too. Exit 2 when <name> is not a footer of free text.

--json: {"schema":1,"footer","range","footers":[{"sha","subject","text"}]}`,

	"push": `Usage: itos push [--no-wait]

The routine that ends a piece of work, after its commits: fetches the branch's
upstream (branch.<name>.remote and branch.<name>.merge, else origin and the
branch's own name), rebases the branch onto it with --no-autostash, so git's
pull.rebase and rebase.autostash settings change nothing, checks that no
rebase is in progress and no conflict is left, then pushes HEAD to the
upstream's branch in a separate step, the pre-push hook running as for any
push. It refuses to start when tracked files have uncommitted changes (commit
or stash them first; untracked files are no reason), when a rebase is in
progress or a conflict is left, and on a detached HEAD. A rebase that stops is
left in progress for the person and nothing is pushed: resolve the conflicts,
git rebase --continue, then itos push again, or git rebase --abort. A conflict
in the work registry is said in a person's words beside that: the file, each
item a commit on the upstream changed too, its owner there when it took it
first, which the remote keeps, and git rebase --skip to give it up. Only HEAD
goes, to that branch, whatever the config's push refspecs say, so the stealth
mode's refs/notes/itos stays local; under a stealth config notes.rewriteRef is
set first, as itos commit sets it, so the rebase carries the notes. It never
forces: --force, -f, --force-with-lease or a + refspec is a usage error, as is
any other argument but --no-wait, and a push the remote or the hook refuses is
reported, not retried. Nothing to push is success. -q is git's --quiet and
leaves out the success lines.

With ci.watch's provider (none by default), it then waits for the CI run of
the commit it pushed as itos ci watch does, printing each job's result as it
finishes and the run's address, and exits with the run's result: 0 when it
succeeded, 1 when it did not, naming the failed jobs; 75 when the run is
still going after ci.watch.timeout seconds or GitHub's API keeps failing, 3
when it cannot be looked at, naming itos ci watch <sha>. The commits are
pushed either way. --no-wait pushes and returns.

Exit: 0 pushed, or nothing to push; 1 refused before the pull, the rebase
stopped, the remote or the pre-push hook refused the push, or the CI run
waited for failed; 2 an argument; 3 no git repository, no upstream and no
remote origin, a remote that is no repository or that git cannot find, or a
CI run not to be looked at; 75 a remote that cannot be reached (a connection
refused or timed out, a host that does not resolve), a CI run not finished,
or GitHub's API failing; 70 a fetch or a push that failed for another reason.
The kind of a failed fetch or push is read from git's message; git's own exit
code is never itos's.

--json: git's output on stderr; {"schema":1,"ok","outcome","remote"?,"branch"?,"commit"?,"ci"?,"run"?},
        outcome one of pushed, nothing-to-push, uncommitted, rebase-in-progress,
        detached, no-remote, fetch-failed, rebase-stopped, rebase-failed, push-failed;
        ci and run as itos ci watch's, when it waited`,

	"verify": `Usage: itos verify <from> <to>
       itos verify

Re-checks every commit of the range (from may be empty or all zeros: every
commit up to <to>): its message, with the footers read at that commit, its
paths, and, by a kind's built-in moves rule, its feature files against its
parent's, unless the rule's except_types names its type; then each kind's
range command. A merge commit is judged by its own changes, the paths of its
dense combined diff (git diff-tree --cc): with none it passes whatever its
message, with some it is judged as its first line's type, and refused when
that is none of the commit types. The commit commits.since names and its ancestors are left out,
and the range commands start there; a footer's own since leaves that commit
and its ancestors out of the footer's required_for. Under a stealth config a
commit's footers of IDs are read from its note in refs/notes/itos, and with
no range the commits are HEAD's on no remote branch (every one with no
remote), the range's from "--remotes": the person's own, not yet pushed.
Prints "<n>/<m> commits pass the commit rules". Exit 1 on any failure, 2 when
commits.since or a footer's since is not a commit of the repository.

--json: {"schema":1,"range","since"?,"commits":[{"sha","header","ok"}],"passed","total","range_checks"}`,

	"tests": `Usage: itos tests list <kind> [--at <tree>]
       itos tests smoke check|ids|run <kind>
       itos tests moves <kind>
       itos tests next-id <kind> <stem>`,

	"tests list": `Usage: itos tests list <kind> [--at <tree>]

The kind's adapter's listing: each named test's ID, file and whether it is
live, at the working tree (default), the index or a commit. A command
adapter may give a test a title too.

--json: {"schema":1,"protocol":1,"tests":[{"id","file","live","title"?}],"files"}`,

	"tests smoke": `Usage: itos tests smoke check <kind> [--features <dir>]
       itos tests smoke ids <kind>
       itos tests smoke run <kind> [--] [<runner args>…]

check: every file with a live test has a smoke test (unless the kind's
smoke.every_file is false), and every smoke ID is live (exit 1 when not). ids:
the smoke set's IDs, one a line. run: the kind's run of exactly the smoke set,
everything after <kind> added to it as given (a -- right after <kind> is taken,
to pass what looks like an itos flag, and not passed on); its exit code is the
runner's.

--json (check): {"schema":1,"kind","ok","ids","problems":[{"rule","message","fix"}]}
--json (ids):   {"schema":1,"kind","ids"}`,

	"tests next-id": `Usage: itos tests next-id <kind> <stem>

Prints the next free tag of the kind with the stem, <tag_prefix><stem>-<n>
(@ID-A-04, @slice-10): n is one past the highest that any tag
<tag_prefix><stem>-<n> of the kind's files holds at the working tree, live or
@wip, and any work registry item id <stem>-<n> (an item such as slice-9 is
named before its scenarios are written). A kind behind a command adapter
counts its listing's IDs instead of tags. The number keeps the width the stem
already has; a stem nothing uses starts at 1, padded to two digits where the
kind's ID pattern is an ID- one and the tag is one of its IDs (@ID-NEW-01).
Help for writing a spec: it reads, writes nothing and judges nothing, and a
number only a removed scenario held is not seen. A <kind> the config does not
have is a usage error (exit 2).

--json: {"schema":1,"kind","stem","id"}`,

	"tests moves": `Usage: itos tests moves <kind>

Judges the staged feature files of a Gherkin kind against HEAD's by the moves
rule, as the commit-msg hook does for a type the rule does not exempt: live
scenarios keep their ID, name, tags and steps exactly, none is lost or added,
and a file with a live scenario keeps its header and Background; @wip ones may
come, go or change, and comment lines are never compared. A rename the kind's
range check { builtin: moves } lists in allowed_renames passes. Exit 1 with
each problem.

--json: {"schema":1,"kind","ok","problems":[{"rule","message"}]}`,

	"ci": `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]
       itos ci run [<from> <to>] | --nightly
       itos ci scope <from> <to>
       itos ci range --head <sha> [--base <sha>]
       itos ci watch [<sha>]`,

	"ci plan": `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]

Prints the plan a CI run would carry out, in cost order, and runs nothing.
Each task check says what the run does with it: run, merged, covered,
nightly (ci.nightly_only leaves it out of a push) or, for a named task's
after: push check in a push, pending (it runs after the push).
With ci.keep_step_order true, a push's plan is ci.steps as written, then the
named tasks' checks in ledger order.
An empty <from> is every commit up to <to>: every test runs, and the checks
of the tasks those commits name.
--data-at reads the ledger, the registry and the smoke set at that commit.
Under a stealth config, given no range, it plans the commits of HEAD on no
remote branch (every one with no remote), the range's from "--remotes".

--json: the plan contract, keys only ever added
        {"schema":1,"range","prose","steps","order","tasks","left_out","unknown","not_started"}`,

	"ci run": `Usage: itos ci run [<from> <to>] | --nightly

Runs the plan: unknown task IDs fail first, tasks not started and checks a
prose-only range leaves out are listed, then each step and task check in cost
order, stopping at the first failure; with ci.stop_at_first_failure false,
every one runs and the first failure is the run's. A named task's after: push
check is listed as pending, to run after the push (itos task, itos work done),
and is neither run nor counted; one ci.nightly_only lists is left out. With no
range, every step and every test. A failing step exits 1, naming its code.
--nightly runs ci.nightly.steps; a { tasks: done } step there runs the checks
of every task whose work item is done, each shared check once, and a failure
names the task. With ci.keep_step_order true, a push's run takes ci.steps as
written, then the named tasks' checks in ledger order.

--json: the run's log on stderr; {"schema":1,"ok","failed_at"?} on stdout`,

	"ci scope": `Usage: itos ci scope <from> <to>

Prints docs_only=true when the range touches only prose (ci.prose.paths).

--json: {"schema":1,"docs_only"}`,

	"ci range": `Usage: itos ci range --head <sha> [--base <sha>]

Prints FROM=<sha>, where a push's range starts: the pull request's base, else
what the ci.range provider says if it is an ancestor of the head, else empty
(run everything). A provider that fails is not an error. github says the
nearest of the head's first parents, from its parent, with a green run of
ci.range.github.workflow on any branch, asking GitHub's API at GITHUB_API_URL,
else https://api.github.com, about each commit's runs, 100 at most.

--json: {"schema":1,"from"}`,

	"ci watch": `Usage: itos ci watch [<sha>]

Waits for the CI run of the commit, HEAD's by default, as itos push waits for
the run of the commit it pushed: looks at it every ci.watch.interval seconds,
printing the run's address and each job's result once, as it finishes, until
the run completes or ci.watch.timeout seconds pass. ci.watch.provider says how
it looks: github asks GitHub's API (GITHUB_API_URL, else
https://api.github.com) for the run of ci.watch.github.workflow, with a token
from ci.range.github.token_env, else gh auth token, for the repository
ci.range.github.repository_env names, else the remote's URL. A run not there
yet is waited for. Exit 0 when the run succeeded; 1 when it did not, naming the
failed jobs; 2 ci.watch.provider none, or not a commit; 3 a refusal from
GitHub's API, or no token, naming both ways to give one; 75 the run still
going, or not there, after the timeout, or six looks in a row failing for a
server error, a rate limit or no network.

--json: the progress on stderr; {"schema":1,"ok","commit","ci","run"?},
        ci one of success, failure, timeout, error; run {"url","status","conclusion"?,"jobs"}`,

	"hook": `Usage: itos hook commit-msg <file>
       itos hook pre-push <remote> <url>
       itos hook install [--manager vp|git|husky|lefthook|pre-commit|prek|git-config] [--print] [--force]`,

	"hook commit-msg": `Usage: itos hook commit-msg <file>

The commit-msg hook. When the commit stages itos.yaml, a ledger file, the work
registry, a smoke set, or a decision record or the index in work.decisions,
itos config check's problems, read from the staged tree; then the staged files
against the message's type (the path rules, then
each kind's staged range checks: the built-in moves rule here, for a type its
except_types does not name), then the header lint (commits.header_lint: the
built-in one, or the hook delegate if any) and itos's footer
rules (commits.footers), always,
both reported, then the checks of each task a ledger footer names (Task:
here), as staged, in written order up to its first late one (CI's cost rule;
an after: push check waits), each capped at hooks.commit_msg.check_timeout
seconds (60). A failure rejects the commit when
the task's work item is done, and is printed with the task's status otherwise.
hooks.commit_msg.task_checks: false runs none. An amend, which itos commit
says in ITOS_AMEND (else an author HEAD's to the second), is judged as the
commit it makes: its staged files against HEAD's parent. The first to fail
prints its report and decides: exit 1, or the header lint's own code.`,

	"hook pre-push": `Usage: itos hook pre-push <remote> <url>

The pre-push hook. Reads git's ref lines on stdin (or PRE_COMMIT_FROM_REF and
PRE_COMMIT_TO_REF under pre-commit or prek) and first verifies the commits
each pushed ref adds, as itos verify does (messages, paths, range checks,
commits.since left out): the ones after the remote's commit it replaces, else
(a new branch, or a remote commit this clone lacks) the ones on no
remote-tracking branch. It prints nothing for commits that pass. A failure
prints verify's report and how to fix the commits (git commit --amend for the
last one, git rebase -i for an earlier one), runs nothing more and exits 1.
Then it runs hooks.pre_push.per_base once per remote commit the push builds
on, else hooks.pre_push.whole (a new branch, or a base this clone lacks);
without hooks.pre_push, nothing. A deleted branch runs nothing. Exit 1 when
a command fails.`,

	"hook install": `Usage: itos hook install [--manager vp|git|husky|lefthook|pre-commit|prek|git-config] [--print] [--force]

Writes the commit-msg and pre-push hooks as one-line shims calling
` + "`" + `<hooks.bin> hook …` + "`" + ` (itos, the global launcher, by default, and under a
stealth config whatever it says), for the hook manager
--manager names, else the one hooks.manager names. Without either, a stealth
config declares them in the git config; any other detects the hook manager
from its markers and says which it found: Vite+ (a .vite-hooks/ folder, or
core.hooksPath .vite-hooks/_), husky (.husky/), lefthook (lefthook.yml),
pre-commit or prek (.pre-commit-config.yaml), else plain git (the
repository's hooks folder). lefthook and pre-commit keep hooks in their
config, so their snippet is printed to add there. --print prints the shims
and writes nothing. A hook that is not a shim is left alone, and the exit is
1, unless --force replaces it.

git-config declares the hooks in the repository's own git config, never
committed: hook.itos-commit-msg, and hook.itos-pre-push when hooks.pre_push
is set or the config is a stealth one, each running
` + "`" + `<hooks.bin> hook <event>` + "`" + `. Git runs them beside the hook in core.hooksPath or
the hooks folder, so the project's hooks and settings stay as they are. It
needs a git that runs the hooks its config declares (git hook list shows
them), else it exits 3.

--json: {"schema":1,"manager","marker","files":[{"path","content","action"}]}
        or, for lefthook, pre-commit and prek, {"schema":1,"manager","marker","file","snippet","installed"}
        or, for git-config, {"schema":1,"manager","marker","hooks":[{"name","event","command","action"}]}`,

	"guard": `Usage: itos guard claude-code`,

	"guard claude-code": `Usage: itos guard claude-code

Claude Code's PreToolUse hook, which the itos plugin runs before each Bash
command. Reads Claude Code's JSON on stdin (tool_name, tool_input.command,
cwd). In a repository itos manages, found from cwd (else the folder itos runs
in) as everywhere else, a Bash command that runs git commit or git push is
denied: exit 0 and Claude Code's deny on stdout, its reason naming itos commit
--task <id> or --scenarios <ids>, or itos push. The command is read as bash
reads it: each command of a list, pipeline, subshell or substitution, its
variable assignments skipped, past command, exec, nohup and env, then git (or
a path ending in /git), its global options (-C moves the folder judged), and
the subcommand; text an argument carries is not a command. sh -c, eval,
scripts, git aliases and words built from variables are not looked into.
Everything else gets no answer (exit 0, nothing on stdout), never an allow,
so Claude Code's permission rules decide. An input it cannot read: exit 1,
the reason on stderr; Claude Code blocks a tool on exit 2 alone. Where the
launcher would hand it to an itos older than 6.0.0, which has no such command
(a pin, ITOS_VERSION or the newest release), it gets no answer either, after
a line on stderr saying why.`,

	"config": `Usage: itos config check [--ledger <file>] [--print-defaults]
       itos config get <key>`,

	"config check": `Usage: itos config check [--ledger <file>] [--print-defaults]

Validates the config, then the ledger (or one ledger file), the work registry,
each kind's smoke set and the decision records in the folder work.decisions
names: no two share a number, and a record whose status is "superseded by
ADR-NNNN" names a number a record there has. Exit 2 when the config is invalid,
1 for any other problem. A warning (WARN) never fails it: a check static by a
ci.cost.static pattern written below a late check of its task, which
ci.cost.keep_written_order runs late, and a project's people file missing or
unreadable (a stealth config reads none). --print-defaults prints the
values the tools take when a key is left out (work.registry's in this config's
ledger folder; a stealth config's hooks.bin, itos, and no work.people), and
checks nothing.

--json: {"schema":1,"config","valid","problems":[{"rule","message","fix"?,"area"}],"warnings"?}`,

	"config get": `Usage: itos config get <key>

Prints the value of a config key, a dotted path (hooks.bin,
hooks.commit_msg.check_timeout, tests.scenario.root), as itos reads it: the
config's own, else its default, a stealth config's hooks.bin itos whatever it
says, from the config every command finds. A string, a number or a boolean
prints bare on one line, a mapping or a list as JSON on one line, and a key
with no value and no default prints nothing. A path is relative to the
repository's top, the files a stealth config names for itos's own data
resolved beside it, where itos reads them. A key the config does not have is
a usage error, exit 2; a config that cannot be read exits 2, as for every
command.

--json: {"schema":1,"key","value"}, value null for a key with no value`,

	"git-shim": `Usage: itos git-shim install [--dir <folder>]
       itos git-shim uninstall [--dir <folder>]
       itos git-shim run <commit|push> [<git args>…]

The git shim is itos started under the name git, through a link named git
(git.exe on windows) to it in a folder before the real git on the PATH. In a
repository itos manages (an itos.yaml in the folder git runs in or at the
repository's top, or a stealth config), git commit runs as itos commit and git
push as itos push, with git's arguments, none of them read as itos's global
flags; git's -C <path> and -c <name>=<value> before the command are honoured.
Every other command, every command elsewhere, and every git an itos run starts
run the real git, the first git on the PATH that is not itos, with the same
arguments, streams and exit code; with no such git the shim exits 3. In a
pinned repository git commit and git push are the pinned itos's; a pin, or
ITOS_VERSION, older than 2.2.0, which has no git shim, runs the real git
instead, after a line on stderr saying so.

install links the itos running as git in <folder> (default: the folder holding
it) and says whether that folder comes before the real git on the PATH. It
never replaces a git that is not a link to itos (exit 1), and replaces a link
to another itos. uninstall removes the link, and leaves any other git alone
(exit 1). Both run the binary called, whatever a pin says. run is what the
link runs in a repository itos manages: git commit or git push, as itos's.

--json: {"schema":1,"link","action","target"?,"on_path"?,"before_git"?,"git"?},
        action one of linked, kept, replaced, refused (install) or removed,
        absent, refused (uninstall)`,

	"pin": `Usage: itos pin [<version>|latest]

Moves the config's pin to a release of itos: <version> (a leading v is taken),
or with none or latest the newest, asked for as the launcher asks for it
(<ITOS_RELEASES>/latest/download/checksums.txt) but now, whatever CI,
ITOS_NO_UPDATE or the once a day say. It sets pin.version and pin.checksums,
the SHA-256 of that release's checksums.txt, in the config itos finds (the
stealth one included), adding the pin when there is none, and changes no other
byte of it; a layout it cannot edit so (a value over several lines, a tag, an
anchor) exits 2, the config untouched. It prints the old version, the new one
and the release's notes, <ITOS_RELEASES>/tag/v<version>, whose Upgrading
section says what the project must change, and commits nothing. A pin already
on the version changes nothing, exit 0; one on it with other checksums is left
as it is, exit 1: the release changed after it was pinned. A release the
server does not have exits 3, and a release server that cannot be reached or
fails 75, the config untouched. It runs the binary called, whatever the pin
says.

--json: {"schema":1,"config","action","version","checksums","previous"?,"notes"?},
        action one of pinned, already, refused`,

	"upgrade": `Usage: itos upgrade [<version>|latest]

Moves the project to a newer itos: <version> (a leading v is taken), or with
none or latest the newest, asked for as itos pin asks for it. The version it moves from
is the config's pin, or with no pin the install script's,
tools/bin/install-itos; with neither it exits 1, naming itos pin, and so does
a version older than that one, which is itos pin's to move back to.

It prints, oldest first, what each release after the version it leaves asks:
its breaking changes, its Upgrading footers, the old scenarios and corpus
cases it changes on purpose and the config keys it adds, removes or gives
another default. It reads them from each release's upgrading.json
(<ITOS_RELEASES>/download/v<version>/upgrading.json), walking back through
each one's previous from the new release to the old version. A release with
no upgrading.json, cut before they were published, is named by its notes,
<ITOS_RELEASES>/tag/v<version>, and the walk stops there.

It edits only what a release fixes exactly: the pin (pin.version and
pin.checksums, as itos pin sets them; none is added where there is none), the
config's first line when it is a yaml-language-server schema line naming a
release's itos.schema.json, and the install script's version= line and each
platform's sum= hash, from the new release's checksums.txt. What the releases
ask is listed for the person to do, never edited. A config or an install
script it cannot edit so exits 2. It fetches everything before it writes
anything, so a release the server does not have exits 3, and a release
server that cannot be reached or fails 75, every file untouched.
A project already on the version changes nothing, exit 0; a pin on it with
other checksums is left as it is, exit 1: the release changed after it was
pinned. It commits nothing, and runs the binary called, whatever the pin says.

--json: {"schema":1,"action","from","to","releases","edited"},
        action one of upgraded, already, refused; each release
        {"version","breaking","upgrading","changes","config"}, or
        {"version","notes"} for one with no upgrading.json`,

	"init": `Usage: itos init [--stealth] [--plugin [project|user|local|no]]
                 [--git-shim [--git-shim-dir <folder>] | --no-git-shim]
                 [--agent-rules | --no-agent-rules]

Readies the repository it runs in for itos, at its top, after git init where
the folder is no repository yet. Where there is no config it writes a starter:
itos.yaml with the Conventional Commits types under the built-in header lint,
a Task footer from the ledger that every type but feat and fix needs,
commits.since at HEAD (none in a repository with no commit) and hooks.bin
itos; the ledger, tasks/phase-1.yaml, and the work registry,
tasks/work-items.yaml; and when features/ holds feature files, the scenario
kind, a Scenarios footer that feat and fix need, and a smoke set,
features/smoke.yaml, naming each file's first live scenario; where no scenario
carries an ID tag yet, the footer is required of no type, and init says how
to tag one so that it can be. A file already there is kept. It pins the
newest release, as itos pin does, or where the release server cannot be
reached pins nothing and says so; then installs the hooks, as itos hooks
install does, its exit code init's. --stealth writes it all in the git
folder, beside the stealth config, and declares the hooks in the git config,
so nothing the project tracks changes.

Where a config is there (itos.yaml, or the stealth one), it writes nothing and
reports what is missing: what itos config check finds, and each hook that does
not call itos, naming the command that puts it right; exit 1 when anything is
missing, 0 when nothing is. It runs the binary called, whatever a pin says.
Then it notes, never counting them as missing: a people file the config names
that the repository lacks or cannot read, as itos config check warns, and a
pin behind the newest release, naming itos pin, where the release server
answers.

It offers the itos plugin for Claude Code, through the claude on the PATH:
claude plugin list --json says whether itos@itos is installed, and claude
plugin marketplace add donvargax/itos then claude plugin install itos@itos
install it. --plugin <scope> answers the offer: project (.claude/settings.json,
committed), user (every repository of yours), local
(.claude/settings.local.json, you alone in this one) or no; a bare --plugin
takes project, or local with --stealth, which refuses project and, after a
local install, lists .claude/settings.local.json in .git/info/exclude when git
would show it. On a terminal with no --plugin it asks, that scope its default
answer; anywhere else it installs nothing and says how to. Run again where a
config is, it never asks: a plugin not installed is reported, never counted as
missing, and --plugin installs it. With no claude on the PATH it says so only
for --plugin. A claude that fails at the install is reported, exit 1, the rest
of init's work done.

It offers the git shim the same way, as itos git-shim install makes it: itos
linked as git, in the folder holding itos or the one --git-shim-dir names.
--git-shim links it, --no-git-shim declines; on a terminal with neither it
asks, yes its default answer; anywhere else it links nothing and says how to.
A link to this itos already there, or with no --git-shim-dir the first git on
the PATH being itos, is kept; a git there that is no link to itos is never
replaced, exit 1, the rest of init's work done. Run again where a config is,
it never asks: a shim not linked is reported, never counted as missing, and
--git-shim links it.

It offers the rules for agents the same way: what the config decides (the
commit types, the footer each type needs, the paths each type may touch and
what the hooks and CI run), generated from it into AGENTS.md between the lines
<!-- itos:begin --> and <!-- itos:end -->, and a line importing it, @AGENTS.md,
at the end of CLAUDE.md, written when it is missing. The block has one line a
paragraph, so a Markdown formatter leaves it as written. An AGENTS.md without
the markers gains the block at its end, its text kept; nothing outside them is
ever changed, and a file's CRLF line endings are kept. --agent-rules writes
it, --no-agent-rules declines; on a terminal with neither it asks, yes its
default answer; anywhere else it writes nothing and says how to. Run again
where a config is, it never asks: a block the config no longer matches is
reported, never counted as missing, and --agent-rules rewrites it.

Under a stealth config, which changes nothing tracked, AGENTS.md and CLAUDE.md
are never written. The block goes to <git common dir>/itos/AGENTS.md, which
every worktree shares, and each agent gets a file at the worktree's top, listed
in the git folder's info/exclude: CLAUDE.local.md, whose block imports
@AGENTS.md when the project has one, then that file, by the path git names the
common dir by; and AGENTS.override.md, which Codex reads in place of AGENTS.md,
holding a copy of the project's AGENTS.md between <!-- itos:agents-md:begin -->
and <!-- itos:agents-md:end -->, then the block. A CLAUDE.local.md or
AGENTS.override.md already there is yours: it gains only the block, at its end.
Run again, a copy that no longer matches AGENTS.md is reported as well, and
--agent-rules rewrites it.

--json: {"schema":1,"config","action":"initialized","git_init","since","files":[{"path","action"}],
        "pin","pin_problem"?,"hooks","plugin","git_shim","agent_rules"} (hooks as itos hook install
        --json prints it), or {"schema":1,"config","action":"checked","missing":[{"rule","message",
        "fix"?,"area"}],"plugin","notes":[{"rule","message","fix"?,"area"}],"git_shim","agent_rules"};
        plugin {"action","scope","excluded","problem"?}, action one of installed, already,
        offered, declined, no_claude, unknown (claude plugin list failed) or failed;
        git_shim {"action","link","on_path","before_git","problem"?}, action one of linked,
        replaced (a link to another itos), kept, offered, declined, refused (a git that is no
        link to itos is there) or failed;
        agent_rules {"action","files":[{"path","action"}],"problem"?}, action one of written,
        current (the block matches the config), stale (it does not, and was not rewritten),
        offered, declined or failed; under a stealth config, stale too when the copy of
        AGENTS.md no longer matches it; each file's action wrote, updated or kept`,

	"followup": `Usage: itos followup [--all]
       itos followup add <id> --with <who> --title <title> --note <text>
       itos followup note <id> <text>
       itos followup close <id> [--note <text>]
       itos followup show <id>
       itos followup doc <id> <path> [--force]

Your threads with people: following up with someone on something, a thread
of dated notes rather than a work item. A thread has an id, the person it is
with (any text: no people file is read), a title, a status, open or closed,
and its notes, each dated when written and never edited. They are yours
alone: kept in follow-ups.yaml of itos's folder in the git common dir (git
rev-parse --git-common-dir, where the stealth mode keeps its data), readable
by you only, never committed, and the same from every linked worktree of the
clone. No config is needed, only a git repository: exit 3 outside one.

A subcommand that changes the threads holds follow-ups.yaml.lock beside the
file from before it reads them to after it writes them, so two at once, in
two worktrees say, take turns and both changes are kept; one that finds the
lock held waits, and gives up after 10 seconds naming it. An itos stopped
while it held the lock leaves the file behind: remove it when no itos runs.
Times print to the minute in your time zone.

itos followup lists the open threads, each with its title, who it is with and
when it last had a note; --all adds the closed ones after them. Exit 1 for a
refusal (an id a thread has, no thread with the id), 2 for a usage error, a
lock still held, or a follow-ups.yaml itos cannot read or did not write
whole (a second YAML document, an empty file), which it leaves as it is.

--json: {"schema":1,"threads":[{"id","with","title","status","opened","last","notes"}]},
notes the count; each subcommand's in its help (itos help followup <subcommand>)`,

	"followup add": `Usage: itos followup add <id> --with <who> --title <title> --note <text>

Opens a thread with <who>, its first note <text> dated now. An id is letters,
digits, '.', '_' and '-', a letter or digit first. Refused, nothing written
(exit 1): an id a thread already has, open or closed.

--json: {"schema":1,"ok":true,"thread":{"id","with","title","status","closed"?,"notes":[{"at","text"}],"docs"?}},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"followup note": `Usage: itos followup note <id> <text>…

Appends a note to the thread, dated now: the words after the id joined by
spaces, so the text needs no quotes unless the shell would read it; after
"--" a word that starts with "-" is text too. A closed thread takes notes and
stays closed. Refused (exit 1): no thread with the id.

--json: as itos followup add's, the thread as it is after`,

	"followup close": `Usage: itos followup close <id> [--note <text>]

Closes the thread, dated now, with a last note when --note gives one. A closed
thread is left out of itos followup's list and still shows and takes notes.
Refused, nothing written (exit 1): no thread with the id, a thread already
closed.

--json: as itos followup add's, the thread as it is after`,

	"followup show": `Usage: itos followup show <id>

Prints the whole thread: its id and title, who it is with and its status,
then every note in the order written, each with its date and time, and the
files itos followup doc wrote it to. It reads, never writes. Exit 1 when no
thread has the id.

--json: as itos followup add's`,

	"followup doc": `Usage: itos followup doc <id> <path> [--force]
       itos followup doc <id> -

Writes the whole thread at <path> as Markdown, in the repository or not, the
folders it needs made: its title, who it is with and its status, then each
note under its date, in order, the raw material for a document someone
refines. The thread records the file, as itos followup show prints it. A file
already at <path> is refused, nothing written (exit 1), unless --force writes
over it; so is a folder, or no thread with the id.

The thread is private, so the file is readable by you alone (0600, the
folders made 0700), and a <path> in the work tree that git does not ignore
is written with a warning on stderr: a git add would commit it. With - for
<path> the Markdown is printed instead, and no file written or recorded.

--json: as itos followup add's, with "path", the file written, absolute; with
-, as itos followup add's with "markdown", the text`,

	"question": `Usage: itos question [--all]
       itos question add <text> [--item <id>]
       itos question answer <id> <text>
       itos question record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>]
       itos question record <id> --none
       itos question show <id>

The questions waiting on the person the work is for, kept as data beside the
work registry rather than by hand: in work.asks, by default asks.yaml in the
registry's folder (tasks/asks.yaml where the ledger is in tasks/). They are
public, unlike itos followup's threads: add and answer write the file and
commit it alone, as work take commits the registry (itos help work take). A
question has an id, q-1, q-2 and so on, one past the highest the file holds,
answered or not, so an id is never given twice; its text; the registry item it
holds up, when --item names one; and its answer, once given, kept beside it.
Under a stealth config the file is in the git folder beside the stealth
registry, written while the registry's lock is held, as every stealth writer
holds it, and nothing is committed.

An answered question is a decision: itos question record writes it as an
architecture decision record, notes the record's number on the question and
commits both (itos help question record), or with --none marks an answer that
concerned its item alone, recorded nowhere.

itos question lists the open questions, each with the item it names, then one
line naming the answered questions recorded nowhere yet, each as itos question
record <id>, when there are any; --all adds the answered ones after the open
ones, each with its answer and its decision. Exit 1 for a refusal (no question with
the id, an item the registry does not have), 2 for a usage error or an
asks.yaml itos cannot read or did not write whole (a key it does not know, an
id not q-<n> or given twice, a decision neither a number nor none, a second
YAML document, an empty file), which it leaves as it is.

--json: {"schema":1,"questions":[{"id","item"?,"question","status","answer"?,"decision"?}]},
status open or answered, decision the record's number or "none"; each
subcommand's in its help (itos help question <subcommand>)`,

	"question add": `Usage: itos question add <text>… [--item <id>]

Asks a question, open, with the next free id: the words before or after
--item joined by spaces, so the text needs no quotes unless the shell would
read it; after "--" a word that starts with "-" is text too. --item names the
registry item the question holds up. Then commits the questions' file alone,
"docs: ask q-<n>", through the hooks, leaving whatever else is staged staged.
Refused, nothing written (exit 1): an item the registry does not have, no
registry where itos looks, a file with changes no commit holds, a commit a
hook refuses (the file put back). Under a stealth config the file is written
and nothing committed.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?},"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"question answer": `Usage: itos question answer <id> <text>…

Answers the question: the words after the id joined by spaces are kept beside
it, and it is answered, left out of itos question's list. Then commits the
questions' file alone, "docs: answer <id>", as itos question add does. Refused,
nothing written (exit 1): no question with the id, a question already
answered, a file with changes no commit holds, a commit a hook refuses. Under
a stealth config the file is written and nothing committed.

--json: as itos question add's, the question as it is after`,

	"question record": `Usage: itos question record <id> --title <title> [--option <text>]… [--consequences <text>] [--supersedes <n>]
       itos question record <id> --none

Writes the answered question as the next architecture decision record, in
MADR 4's format (adr/madr, its bare-minimal sections): NNNN-<slug>.md, the
slug the title lowercased with each run of other characters one dash. Its
YAML frontmatter holds "status: accepted" and the date it is recorded; then
"# <title>"; "## Context and Problem Statement", the question and "Asked as
<id>" (with the item it names); "## Considered Options", a bullet for each
--option given, else "The options are those the question names."; "## Decision
Outcome", the answer; and "### Consequences", --consequences, else "None
recorded.". The records' folder is the one work.decisions names, by default
docs/decisions; the number is one past the highest any name there starts
with. The folder's README.md is the index of the records whose status is
accepted, by number and title, between <!-- itos:decisions:begin --> and
<!-- itos:decisions:end -->, made when missing, its text outside them kept. A
record is read by its frontmatter and headings, so one edited by hand reads
the same. --supersedes <n> sets record <n>'s status to "superseded by
ADR-<nnnn>", the new record's number in four digits, so it leaves the index,
and the new record ends with "## More Information", "Supersedes ADR-<nnnn>.".
The question gains "decision: <n>". Then commits the questions' file, the
record, the one it supersedes and the index together, "docs: record <id> as
decision <n>", as itos question add commits.

--none writes no record: the answer concerned its item alone, and the
question gains "decision: none", so itos question stops naming it; the
questions' file is committed alone, "docs: mark <id> as recorded nowhere".

Refused, nothing written (exit 1): no question with the id, one not answered,
one already recorded or marked, --supersedes naming no record in the folder,
a file with changes no commit holds, a commit a hook refuses (the files put
back). Under a stealth config the records are in the git folder beside the
stealth config, decisions/ unless its work.decisions names another, and
nothing is committed.

--json: {"schema":1,"ok":true,"question":{…,"decision"},"record":"<path>"|null,"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,

	"question show": `Usage: itos question show <id>

Prints the question: its id and text, whether it is open or answered, the
item it names and its decision, then its answer when it has one. It reads, never writes. Exit 1
when no question has the id.

--json: {"schema":1,"ok":true,"question":{"id","item"?,"question","status","answer"?,"decision"?}}`,

	"go": `Usage: itos go

Prints the coordinator's guide, the one shipped in itos, the same in every
repository: how the session that coordinates the work runs it, from reading
where things stand to handing one item at a time to an implementing agent and
landing it. A session that coordinates starts with it (! itos go). Then, after
a line of ---, the repository's own notes, when it keeps them: the file
guide.orchestrating names (docs/ORCHESTRATING.md by default), read from the
repository's top, or in the git folder's itos/ for a stealth config. Then,
after another line of ---, this clone's own notes, when it keeps them:
.git/itos/notes.md (itos/notes.md in the git common dir, which every linked
worktree shares and git never commits), for what is true on this machine
only, under the heading "# This clone's own notes"; printed once, as the
repository's notes, when guide.orchestrating names that same file. Last,
after another line of ---, where things stand, as itos status prints it for
the person the identity provider names; with no config, or no sound work
registry, a line on stderr says it is left out, and the guide still prints.
It writes nothing; outside a repository the guide prints alone. itos guide
coordinate prints the guide and the repository's notes alone.

--json: {"schema":1,"guide":"coordinate","text","notes"?,"local_notes"?,
"status"?}, notes the file appended, local_notes this clone's own notes' file,
status the object itos status --json prints, but its schema`,

	"status": `Usage: itos status [--as <handle>]

Prints where things stand, for the person the work is for (--as, else the
identity provider's, as itos work): the remote branch's head, its short SHA and
header, asked of the remote (the branch's upstream, else origin), or as last
fetched when the remote cannot be reached; its CI run, looked at once through
ci.watch's provider and never waited for, a run still going said to be going;
while it is going or has not passed, the commit main last proved, "Last green:"
and its short SHA and header (the SHA alone when it is not fetched here), as
ci.range's provider names it: for github the nearest of the branch's first
parents as fetched, its head included and 100 at most, with a successful run of
ci.range.github's workflow on any branch (its repository, token and API address
found as ci.watch's), read once, and left out for none; the last nightly's run,
looked at once the same way, "Nightly:" and its result and address, read from
ci.watch.github.nightly_workflow's newest run on the branch, and left out when
that key is not set; the newest release, the highest tag
v<semver> on the remote (a prerelease below its release), asked of it as the
head is, or as last fetched, or no release yet; the commits since it the next
release would carry, the feat, fix and breaking ones, by header, oldest first,
read from the commits as fetched here, with a line saying they may be behind
when the head is not fetched; the person's items in progress; the next items
they can start, theirs and the unowned, in the queue's order and the unqueued
after, five at most; and the open questions (itos question). It reads, never writes
or fetches. What cannot be reached (no remote, ci.watch.provider none, a
provider that fails, for the head or the nightly, a ci.range provider that
fails) is one line naming it, and the rest still prints, exit 0.
Exit 1 when the work registry is not there or not sound, 3 when --as is not
among the people. itos go prints it last.

--json: {"schema":1,"person","every_item"?,"head":{"remote","branch","commit",
"header","last_fetched"}|null,"ci":{"result","run"?}|null,"last_green":
{"commit","header"}|null,"nightly":{"url","status","conclusion"?,"jobs"}|null,
"release":{"tag","commit","last_fetched"}|null,"unreleased":["<header>"]|null,
"doing","next","more","questions":[{"id","item"?,"question","status"}],
"unread"}, result success, failure (or another conclusion), going, or none for
no run yet; last_green null when the head's run passed or was not read,
ci.range's provider is none, it names no commit or it cannot be read, header ""
when not fetched; nightly null when no nightly is named, it has no run yet or it
cannot be read; unreleased null with no release or when they cannot be listed;
unread the lines of what could not be reached`,

	"guide": `Usage: itos guide coordinate|work

Prints a guide shipped in itos, as written, the same in every repository:
coordinate, the coordinator's, as itos go prints it with the repository's own
notes after it; work, the implementer's, which a coordinator asks each agent
it starts to run first: how to take an item, commit, push and read what a
gate says. It needs no config and writes nothing. A name it has no guide by
is a usage error (exit 2) naming those it has.

--json: {"schema":1,"guide","text","notes"?}`,

	"version": `Usage: itos version [--check]
       itos --version [--check]

Prints itos's version, and needs no config; a first argument --version is itos
version. In a project that pins a version, the pinned itos answers, and the
itos that was called says on stderr that the repository pins it and which
version it is itself. --check exits 1 when it does not satisfy the config's
requires, and 2 when the config cannot be read.

--json: {"schema":1,"version","requires"?,"satisfied"?}`,
}

// helpFor is the help for a command path: the longest one helpTexts knows,
// else itos's own.
func helpFor(path []string) string {
	for n := len(path); n > 0; n-- {
		if text, ok := helpTexts[strings.Join(path[:n], " ")]; ok {
			return text
		}
	}
	return mainHelp
}

// helpPath is the command path a --help asks about: the command and its
// subcommands, up to three words that are not flags.
func helpPath(rest []string) []string {
	var path []string
	for _, a := range rest {
		if len(path) == 3 {
			break
		}
		if !strings.HasPrefix(a, "-") {
			path = append(path, a)
		}
	}
	return path
}

// help prints what `itos`, `itos help <command>…` and any --help ask for,
// and gives the exit code: 2 for a bare `itos`, which names no command, and
// for `itos help <topic>` of a command itos does not have, else 0.
func help(g Globals, o Out) int {
	name, rest := "", []string(nil)
	if len(g.Rest) > 0 {
		name, rest = g.Rest[0], g.Rest[1:]
	}
	path := helpPath(g.Rest)
	if name == "help" {
		path = rest
		// An extension prints its own help; a topic that is neither one nor
		// a command of itos's is an unknown command, exit 2 (slice 89).
		if len(path) > 0 {
			if ext := extensionPath(path[0]); ext != "" {
				return runExtension(g, ext, []string{"--help"}, o)
			}
			if _, ok := commands[path[0]]; !ok && path[0] != "help" {
				return failure(usage("unknown command: %s", path[0]), o)
			}
		}
	}
	text := helpFor(path)
	if text == mainHelp {
		text = mainCommands + extensionsHelp() + "\n\n" + mainHelpTail
	}
	fmt.Fprintln(o.Stdout, text)
	if name == "" && !g.Help {
		return ExitUsage
	}
	return 0
}
