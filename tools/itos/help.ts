// What `itos --help` and each command's `--help` print. Agents read these, so
// conformance/help.yaml holds every one byte for byte: a change here changes
// it in the same commit.
const GLOBAL = `Global flags:
  --config <file>   the config (default itos.yaml, or ITOS_CONFIG)
  --root <dir>      run as if started in <dir>
  --json            one object on stdout ("schema": 1, keys only ever added); logs on stderr
  --no-color        no colour (itos prints none; passed on as NO_COLOR)
  -q, --quiet       leave out a check's success line
  -h, --help        this text, or a command's`;

const EXIT = `Exit codes: 0 success; 1 policy failure (a check failed, a commit rejected,
a registry or smoke problem, an unknown task); 2 usage or config error;
3 missing environment (--as not among the people). In \`ci run\` a failing step
exits with its own code.`;

export const HELP: Record<string, string> = {
	"": `itos: tasks, their checks, commit rules and CI plans

Usage: itos <command> [args] [global flags]

Commands:
  task <id>…                       run the tasks' checks; the status table
  task list [--group <g>]          list the tasks, running nothing
  work [--as <handle>]             what the person can start, and what waits
  work check [<file>]              validate the work registry
  commit check-message <file|->    header lint and footer rules on one message
  commit check-paths --type <t> <path>…
                                   the commit type's path rules, for planning a split
  verify <from> <to>               re-check every commit of a range
  tests list <kind> [--at <tree>]  the kind's named tests
  tests smoke check|ids|run <kind> the smoke rule, the smoke IDs, the smoke run
  ci plan <from> <to> | --nightly | --whole
                                   print the CI plan; runs nothing
  ci run [<from> <to>] | --nightly run the CI plan
  ci scope <from> <to>             docs_only=true|false, for GITHUB_OUTPUT
  ci range --head <sha> [--base <sha>]
                                   FROM=<sha>: where a push's range starts
  hook commit-msg <file>           the commit-msg hook: path rules, range checks, header lint
  hook pre-push <remote> <url>     the pre-push hook: the tests the pushed commits reach
  hooks install [--manager <m>] [--print] [--force]
                                   write the hooks' one-line shims for the hook manager
  config check [--print-defaults]  validate the config, ledger, registry and smoke sets
  version [--check]                the version; --check against the config's requires

${GLOBAL}

${EXIT}`,

	task: `Usage: itos task <id>… [--skip <ids>]
       itos task --group <g> [--skip <ids>]   (--phase is the same)
       itos task --pending
       itos task list [--group <g>]

Runs each task's done_when checks in written order: verbose (each command and
its output) for one task, then one status line per task: done, pending (a check
waits on the push), failing or review (no checks). --pending shows the tasks
not done. Exit 1 when a check fails or an ID is not in the ledger, 2 when
nothing matches.

--json: {"schema":1,"tasks":[{"id","type","title","group","status","checks":[{"command","fails"?,"result"}]}]}`,

	"task list": `Usage: itos task list [--group <g>]

Lists the ledger's tasks (id, group, type, title) without running anything.

--json: {"schema":1,"tasks":[{"id","type","title","group","checks"}]}`,

	work: `Usage: itos work [--as <handle>]
       itos work check [<file>]

Who the session works for (--as, else the config's work.identity provider) and
their items: in progress, can start now, unowned, waiting, ideas, deferred.
Exit 1 when the registry is not sound, 3 when --as is not among the people.

--json: the proposal {"schema":1,"person","doing","next","unowned","waiting","ideas","deferred"}`,

	"work check": `Usage: itos work check [<file>]

Validates the work registry (the config's work.registry, or <file>): duplicate
IDs, unknown phases, statuses, kinds, owners and dependencies, cycles. Exit 1
on a problem.

--json: {"schema":1,"file","sound","problems":[{"rule","message","fix"?}]}`,

	commit: `Usage: itos commit check-message <file|-> [--at <sha>]
       itos commit check-paths --type <type> <path>…`,

	"commit check-message": `Usage: itos commit check-message <file|-> [--at <sha>]

Runs the header lint (commits.header_lint) and the footer rules
(commits.footers) on one message, read from <file> or stdin (-). --at reads the
footers' IDs at that commit. Exit 1 when the message is rejected.

--json: {"schema":1,"ok","problems":[{"rule","message","fix"?,"level"}]}`,

	"commit check-paths": `Usage: itos commit check-paths --type <type> <path>…

Applies the type's path rules (commits.scopes) to the paths and prints what
they reject, without committing. Exit 1 when a path is rejected.

--json: {"schema":1,"type","files","ok","problems":[{"rule","message","fix"}]}`,

	verify: `Usage: itos verify <from> <to>

Re-checks every non-merge commit of the range (from may be empty or all zeros:
every commit up to <to>): its message, with the footers read at that commit,
and its paths; then each named-test kind's range check. Prints
"<n>/<m> commits pass the commit rules". Exit 1 on any failure.

--json: {"schema":1,"range","commits":[{"sha","header","ok"}],"passed","total","range_checks"}`,

	tests: `Usage: itos tests list <kind> [--at <tree>]
       itos tests smoke check|ids|run <kind>`,

	"tests list": `Usage: itos tests list <kind> [--at <tree>]

The kind's adapter's listing: each named test's ID, file and whether it is
live, at the working tree (default), the index or a commit.

--json: {"schema":1,"protocol":1,"tests":[{"id","file","live"}],"files"}`,

	"tests smoke": `Usage: itos tests smoke check <kind> [--features <dir>]
       itos tests smoke ids <kind>
       itos tests smoke run <kind> [-- <runner args>…]

check: every file with a live test has a smoke test, and every smoke ID is live
(exit 1 when not). ids: the smoke set's IDs, one a line. run: the kind's run of
exactly the smoke set; its exit code is the runner's.

--json (check): {"schema":1,"kind","ok","ids","problems":[{"rule","message","fix"}]}
--json (ids):   {"schema":1,"kind","ids"}`,

	ci: `Usage: itos ci plan <from> <to> | --nightly | --whole [--data-at <sha>]
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
order, stopping at the first failure. With no range, every step and every
test. A failing step exits with its own code.

--json: the run's log on stderr; {"schema":1,"ok","failed_at"?} on stdout`,

	"ci scope": `Usage: itos ci scope <from> <to>

Prints docs_only=true when the range touches only prose (ci.prose.paths).

--json: {"schema":1,"docs_only"}`,

	"ci range": `Usage: itos ci range --head <sha> [--base <sha>]

Prints FROM=<sha>, where a push's range starts: the pull request's base, else
what the ci.range provider says if it is an ancestor of the head, else empty
(run everything). A provider that fails is not an error.

--json: {"schema":1,"from"}`,

	hook: `Usage: itos hook commit-msg <file>
       itos hook pre-push <remote> <url>`,

	"hook commit-msg": `Usage: itos hook commit-msg <file>

The commit-msg hook. The staged files against the message's type (the path
rules, then each kind's staged range check: the scenario moves here), then the
header lint (commits.header_lint.hook, commitlint here, whose config runs the
footer rules; with no delegate, the footer rules alone). The first to fail
prints its report and decides: exit 1, or the header lint's own code.`,

	"hook pre-push": `Usage: itos hook pre-push <remote> <url>

The pre-push hook. Reads git's ref lines on stdin (or PRE_COMMIT_FROM_REF and
PRE_COMMIT_TO_REF under pre-commit or prek) and runs hooks.pre_push.per_base
once per remote commit the push builds on, else hooks.pre_push.whole (a new
branch, or a base this clone lacks). A deleted branch runs nothing. Exit 1
when a command fails.`,

	hooks: `Usage: itos hooks install [--manager vp|git|husky|lefthook|pre-commit|prek] [--print] [--force]`,

	"hooks install": `Usage: itos hooks install [--manager vp|git|husky|lefthook|pre-commit|prek] [--print] [--force]

Writes the commit-msg and pre-push hooks as one-line shims calling
\`<hooks.bin> hook …\` (tools/bin/itos by default). Without --manager it
detects the hook manager from its markers and says which it found: Vite+ (a
.vite-hooks/ folder, or core.hooksPath .vite-hooks/_), husky (.husky/),
lefthook (lefthook.yml), pre-commit or prek (.pre-commit-config.yaml), else
plain git (the repository's hooks folder). lefthook and pre-commit keep hooks
in their config, so their snippet is printed to add there. --print prints the
shims and writes nothing. A hook that is not a shim is left alone, and the
exit is 1, unless --force replaces it.

--json: {"schema":1,"manager","marker","files":[{"path","content","action"}]}
        or, for lefthook, pre-commit and prek, {"schema":1,"manager","marker","file","snippet","installed"}`,

	config: `Usage: itos config check [--ledger <file>] [--print-defaults]`,

	"config check": `Usage: itos config check [--ledger <file>] [--print-defaults]

Validates the config, then the ledger (or one ledger file), the work registry
and each kind's smoke set. Exit 2 when the config is invalid, 1 for any other
problem. --print-defaults prints the values the tools take when a key is left
out, and checks nothing.

--json: {"schema":1,"config","valid","problems":[{"rule","message","fix"?,"area"}]}`,

	version: `Usage: itos version [--check]

Prints itos's version. --check exits 1 when it does not satisfy the config's
requires.

--json: {"schema":1,"version","requires"?,"satisfied"?}`,
};

// The help for a command path: the longest one HELP knows.
export function helpFor(path: string[]): string {
	for (let n = path.length; n > 0; n--) {
		const key = path.slice(0, n).join(" ");
		if (key in HELP) return HELP[key]!;
	}
	return HELP[""]!;
}
