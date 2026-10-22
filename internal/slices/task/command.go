package task

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "task",
	Rank: 40,
	Lines: `  task <id>…                       run the tasks' checks; the status table
  task list [--group <g>]          the tasks and their work items' status; runs nothing
  task add --group <g> --type <type> --title <title> --why <why> --check <command> […]
                                   add a task to the ledger and its item, and commit both
  task next-id                     the ledger's next free task ID; reads only
`,
	Help: map[string]string{
		"": `Usage: itos task <id>… [--skip <ids>]
       itos task --group <g> [--skip <ids>]
       itos task --pending
       itos task list [--group <g>]
        itos task add --group <g> --type <type> --title <title> --why <why> --check <command> […]
       itos task next-id

--phase <g> and --<label> <g> are --group <g>, the label being the config's
ledger.group.label (phase by default).

Runs each task's done_when checks in written order: verbose (each command and
its output) for one task, then one status line per task: done, pending (a check
waits on the push), failing or review. A task with no checks is done when its
work item in the work registry is done (work done takes a closed task's checks
out of the ledger), review otherwise. --pending shows the tasks not done. A
check that more than one task lists (the same command and timeout) runs once,
and each task reads its exit status by its own run: or fails:.
Exit 1 when a check fails or an ID is not in the ledger, 2 when nothing matches.

--json: {"schema":1,"tasks":[{"id","type","title","group","status","checks":[{"command","fails"?,"result"}]}]}`,

		"list": `Usage: itos task list [--group <g>]

Lists the ledger's tasks (status, id, group, type, title) without running any
check. The status is the task's work item's in the work registry (the item
whose id is the task's), or "no item".

--json: {"schema":1,"tasks":[{"id","type","title","group","checks","status"}]}`,

		"next-id": `Usage: itos task next-id

Prints the ledger's next free task ID: one past the highest number of the
tasks' IDs and of the work registry's item ids that ledger.id matches (work
promote names a task there before the ledger holds it), with the ledger's
padding and as wide as ledger.id asks (T-003 after T-001 and T-002). Help for
writing a task: it reads the working tree, writes nothing and judges nothing,
and a number only a removed task held is not seen.

--json: {"schema":1,"id"}`,

		"add": `Usage: itos task add --group <g> --type <type> --title <title> --why <why>
         --check <command> [--timeout <seconds>] [--check <command> [--timeout <seconds>]]…

Mints a task ID from the shared counter, then adds it at the end of its group's ledger file (ledger.files with {group}
filled in, made when the group has none yet): its id, type, title, why (a
folded text) and done_when, one run: check per --check in order, a --timeout
after a --check giving that check its own seconds. Adds its item to the work
registry, a task, todo and nobody's, as work add does but with no why (the
ledger entry holds it), then commits the ledger
file and the registry alone, "docs: add <id>", as work take commits the
registry (itos help work take); a commit a hook refuses puts both back.
--phase <g> and --<label> <g> are --group <g>. Refused, nothing written (exit
2): an explicit id. Refused, nothing written (exit 1): a <type>
commits.types does not list, a group ledger.group.pattern does not match, a
ledger with problems (itos config check), a registry that is not sound, what
work add refuses (a group the registry does not list),
either file with changes no commit holds. Under a stealth config both are
written and nothing committed.

--json: {"schema":1,"ok":true,"task":{…},"file":"<ledger file>","item":{…},"commit":"<sha>"|null},
or {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Value("--group"), declare.Value("--phase"), declare.Value("--skip"),
		declare.Switch("--pending")}, More: declare.GroupLabel, Words: true, Subs: map[string]declare.Spec{
		"list": {Flags: []declare.Flag{declare.Value("--group"), declare.Value("--phase")}, More: declare.GroupLabel},
		"add": {Flags: []declare.Flag{declare.Value("--group"), declare.Value("--phase"), declare.Value("--type"),
			declare.Value("--title"), declare.Value("--why"), declare.Values("--check"), declare.Values("--timeout")},
			More: declare.GroupLabel},
		"next-id": {},
	}},
}
