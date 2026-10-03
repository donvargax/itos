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
                    stealth mode's <git common dir>/itos/itos.yaml)
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

const exitCodes = `Exit codes: 0 success; 1 policy failure (a check failed, a commit rejected,
a registry or smoke problem, an unknown task); 2 usage or config error;
3 missing environment (--as not among the people, a release that cannot be
fetched or checked, an extension that cannot start). In ` + "`ci run`" + ` a failing
step exits with its own code.`

const mainCommands = `itos: tasks, their checks, commit rules and CI plans

Usage: itos <command> [args] [global flags]

Commands:
  task <id>…                       run the tasks' checks; the status table
  task list [--group <g>]          the tasks and their work items' status; runs nothing
  work [--as <handle>]             what the person can start, and what waits
  work check [<file>]              validate the work registry
  commit [--task <id>] [--scenarios <ids>] [<git commit args>…]
                                   git commit, with the footers itos writes
  commit check-message <file|->    header lint and footer rules on one message
  commit check-paths --type <t> <path>…
                                   the commit type's path rules, for planning a split
  commit footers <name> <from> <to>
                                   a range's free-text footers, for the release notes
  verify <from> <to>               re-check every commit of a range
  tests list <kind> [--at <tree>]  the kind's named tests
  tests smoke check|ids|run <kind> the smoke rule, the smoke IDs, the smoke run
  tests moves <kind>               the staged feature files by the moves rule
  ci plan <from> <to> | --nightly | --whole
                                   print the CI plan; runs nothing
  ci run [<from> <to>] | --nightly run the CI plan
  ci scope <from> <to>             docs_only=true|false, for GITHUB_OUTPUT
  ci range --head <sha> [--base <sha>]
                                   FROM=<sha>: where a push's range starts
  hook commit-msg <file>           the commit-msg hook: data, paths, header lint, task checks
  hook pre-push <remote> <url>     the pre-push hook: the tests the pushed commits reach
  hooks install [--manager <m>] [--print] [--force]
                                   write the hooks' one-line shims for the hook manager
  config check [--print-defaults]  validate the config, ledger, registry and smoke sets
  version [--check]                the version; --check against the config's requires`

const mainHelpTail = globalFlags + `

` + versions + `

` + otherCommands + `

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

	"work": `Usage: itos work [--as <handle>]
       itos work check [<file>]

Who the session works for (--as, else the config's work.identity provider) and
their items: in progress, can start now, unowned, waiting, ideas, deferred.
Exit 1 when the registry is not sound, 3 when --as is not among the people.

--json: the proposal {"schema":1,"person","doing","next","unowned","waiting","ideas","deferred"}`,

	"work check": `Usage: itos work check [<file>]

Validates the work registry (the config's work.registry, by default
work-items.yaml in the ledger's folder, or <file>): that it is there,
duplicate IDs, unknown groups (the owners per group under work.groups_key,
phases by default), statuses (those work.statuses lists), kinds, owners and
dependencies, cycles. Exit 1 on a problem. The messages call a group by
ledger.group.label (phase by default).

--json: {"schema":1,"file","sound","problems":[{"rule","message","fix"?}]}`,

	"commit": `Usage: itos commit [--task <id>] [--scenarios <ids>] [<git commit args>…]
       itos commit check-message <file|-> [--at <sha>]
       itos commit check-paths --type <type> <path>…
       itos commit footers <name> <from> <to>

Runs git commit with every argument but its own flags, and writes the footers
from them as git's --trailer, so they land whether the message comes from -m,
-F or the editor: --task the footer whose source is the ledger (Task: here),
--scenarios the one whose source is a kind of named tests, the first of
commits.footers each. A flag may repeat, its value one ID or several
separated by commas or spaces, written on lines of 100 characters at most,
the key on each. The hooks judge the commit as any other, so a commit they
refuse is not made. -q is git's --quiet. A first argument that names a
subcommand runs it. Exit: git's own code; 2 when a flag has no value or the
config no footer for it.

--json: git's output on stderr; {"schema":1,"ok","commit"?}`,

	"commit check-message": `Usage: itos commit check-message <file|-> [--at <sha>]

Runs the header lint (commits.header_lint: itos's own with use: builtin, else
the stdin delegate), if any, and itos's footer rules (commits.footers),
always, on one message, read from <file> or stdin (-), and reports both. --at
reads the footers' IDs at that commit. Exit 1 when either rejects the
message; the built-in lint's warnings alone do not.

--json: {"schema":1,"ok","problems":[{"rule","message","fix"?,"level"}]}`,

	"commit check-paths": `Usage: itos commit check-paths --type <type> <path>…

Applies the type's path rules (commits.scopes) to the paths and prints what
they reject, without committing. Exit 1 when a path is rejected.

--json: {"schema":1,"type","files","ok","problems":[{"rule","message","fix"}]}`,

	"commit footers": `Usage: itos commit footers <name> <from> <to>

Lists the footers <name> that the range's non-merge commits carry (from may be
empty or all zeros: every commit up to <to>), where commits.footers.<name> is
free text (source: text): what each commit asks of a consumer, gathered for a
release's notes. One line per footer, oldest commit first, its commit's short
SHA and its text; a commit with several gives a line for each. A footer that
says none (the word alone, in lower case) asks nothing and is left out, as is
an empty one. Exit 2 when <name> is not a footer of free text.

--json: {"schema":1,"footer","range","footers":[{"sha","subject","text"}]}`,

	"verify": `Usage: itos verify <from> <to>

Re-checks every non-merge commit of the range (from may be empty or all zeros:
every commit up to <to>): its message, with the footers read at that commit,
its paths, and, by a kind's built-in moves rule, its feature files against its
parent's, unless the rule's except_types names its type; then each kind's
range command. The commit commits.since names and its ancestors are left out,
and the range commands start there; a footer's own since leaves that commit
and its ancestors out of the footer's required_for. Prints "<n>/<m> commits
pass the commit rules". Exit 1 on any failure, 2 when commits.since or a
footer's since is not a commit of the repository.

--json: {"schema":1,"range","since"?,"commits":[{"sha","header","ok"}],"passed","total","range_checks"}`,

	"tests": `Usage: itos tests list <kind> [--at <tree>]
       itos tests smoke check|ids|run <kind>
       itos tests moves <kind>`,

	"tests list": `Usage: itos tests list <kind> [--at <tree>]

The kind's adapter's listing: each named test's ID, file and whether it is
live, at the working tree (default), the index or a commit.

--json: {"schema":1,"protocol":1,"tests":[{"id","file","live"}],"files"}`,

	"tests smoke": `Usage: itos tests smoke check <kind> [--features <dir>]
       itos tests smoke ids <kind>
       itos tests smoke run <kind> [-- <runner args>…]

check: every file with a live test has a smoke test (unless the kind's
smoke.every_file is false), and every smoke ID is live (exit 1 when not). ids:
the smoke set's IDs, one a line. run: the kind's run of exactly the smoke set;
its exit code is the runner's.

--json (check): {"schema":1,"kind","ok","ids","problems":[{"rule","message","fix"}]}
--json (ids):   {"schema":1,"kind","ids"}`,

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
       itos ci range --head <sha> [--base <sha>]`,

	"ci plan": `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]

Prints the plan a CI run would carry out, in cost order, and runs nothing.
--data-at reads the ledger, the registry and the smoke set at that commit.

--json: the plan contract, keys only ever added
        {"schema":1,"range","prose","steps","order","tasks","left_out","unknown","not_started"}`,

	"ci run": `Usage: itos ci run [<from> <to>] | --nightly

Runs the plan: unknown task IDs fail first, tasks not started and checks a
prose-only range leaves out are listed, then each step and task check in cost
order, stopping at the first failure; with ci.stop_at_first_failure false,
every one runs and the first failure is the run's. With no range, every step
and every test. A failing step exits with its own code. --nightly runs
ci.nightly.steps; a { tasks: done } step there runs the checks of every task
whose work item is done, each shared check once, and a failure names the task.

--json: the run's log on stderr; {"schema":1,"ok","failed_at"?} on stdout`,

	"ci scope": `Usage: itos ci scope <from> <to>

Prints docs_only=true when the range touches only prose (ci.prose.paths).

--json: {"schema":1,"docs_only"}`,

	"ci range": `Usage: itos ci range --head <sha> [--base <sha>]

Prints FROM=<sha>, where a push's range starts: the pull request's base, else
what the ci.range provider says if it is an ancestor of the head, else empty
(run everything). A provider that fails is not an error.

--json: {"schema":1,"from"}`,

	"hook": `Usage: itos hook commit-msg <file>
       itos hook pre-push <remote> <url>`,

	"hook commit-msg": `Usage: itos hook commit-msg <file>

The commit-msg hook. When the commit stages itos.yaml, a ledger file, the work
registry or a smoke set, itos config check's problems, read from the staged
tree; then the staged files against the message's type (the path rules, then
each kind's staged range checks: the built-in moves rule here, for a type its
except_types does not name), then the header lint (commits.header_lint: the
built-in one, or the hook delegate if any) and itos's footer
rules (commits.footers), always,
both reported, then the checks of each task a ledger footer names (Task:
here), as staged, in written order up to its first late one (CI's cost rule;
an after: push check waits), each capped at hooks.commit_msg.check_timeout
seconds (60). A failure rejects the commit when
the task's work item is done, and is printed with the task's status otherwise.
hooks.commit_msg.task_checks: false runs none. The first to fail prints its
report and decides: exit 1, or the header lint's own code.`,

	"hook pre-push": `Usage: itos hook pre-push <remote> <url>

The pre-push hook. Reads git's ref lines on stdin (or PRE_COMMIT_FROM_REF and
PRE_COMMIT_TO_REF under pre-commit or prek) and runs hooks.pre_push.per_base
once per remote commit the push builds on, else hooks.pre_push.whole (a new
branch, or a base this clone lacks). A deleted branch runs nothing. Exit 1
when a command fails.`,

	"hooks": `Usage: itos hooks install [--manager vp|git|husky|lefthook|pre-commit|prek] [--print] [--force]`,

	"hooks install": `Usage: itos hooks install [--manager vp|git|husky|lefthook|pre-commit|prek] [--print] [--force]

Writes the commit-msg and pre-push hooks as one-line shims calling
` + "`" + `<hooks.bin> hook …` + "`" + ` (tools/bin/itos by default), for the hook manager
--manager names, else the one hooks.manager names. Without either it detects
the hook manager from its markers and says which it found: Vite+ (a
.vite-hooks/ folder, or core.hooksPath .vite-hooks/_), husky (.husky/),
lefthook (lefthook.yml), pre-commit or prek (.pre-commit-config.yaml), else
plain git (the repository's hooks folder). lefthook and pre-commit keep hooks
in their config, so their snippet is printed to add there. --print prints the
shims and writes nothing. A hook that is not a shim is left alone, and the
exit is 1, unless --force replaces it.

--json: {"schema":1,"manager","marker","files":[{"path","content","action"}]}
        or, for lefthook, pre-commit and prek, {"schema":1,"manager","marker","file","snippet","installed"}`,

	"config": `Usage: itos config check [--ledger <file>] [--print-defaults]`,

	"config check": `Usage: itos config check [--ledger <file>] [--print-defaults]

Validates the config, then the ledger (or one ledger file), the work registry
and each kind's smoke set. Exit 2 when the config is invalid, 1 for any other
problem. --print-defaults prints the values the tools take when a key is left
out (work.registry's in this config's ledger folder), and checks nothing.

--json: {"schema":1,"config","valid","problems":[{"rule","message","fix"?,"area"}]}`,

	"version": `Usage: itos version [--check]

Prints itos's version, and needs no config. --check exits 1 when it does not
satisfy the config's requires, and 2 when the config cannot be read.

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
// and gives the exit code: 2 for a bare `itos`, which names no command,
// else 0.
func help(g Globals, o Out) int {
	name, rest := "", []string(nil)
	if len(g.Rest) > 0 {
		name, rest = g.Rest[0], g.Rest[1:]
	}
	path := helpPath(g.Rest)
	if name == "help" {
		path = rest
		// An extension prints its own help.
		if len(path) > 0 {
			if ext := extensionPath(path[0]); ext != "" {
				return runExtension(g, ext, []string{"--help"}, o)
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
