package initcmd

import "github.com/donvargax/itos/v8/internal/declare"

var Command = declare.Command{
	Name: "init",
	Rank: 140,
	Lines: `  init [--stealth] [--policy <file>] [--plugin [<scope>]]
       [--git-shim [--git-shim-dir <folder>] | --no-git-shim] [--agent-rules | --no-agent-rules]
                                   ready a repository for itos, from a policy or the starter;
                                   run again, what is missing
`,
	Help: map[string]string{
		"": `Usage: itos init [--stealth] [--policy <file>] [--plugin [project|user|local|no]]
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
reached pins nothing and says so; then declares the hooks in the git config,
as itos hook install does, its exit code init's. --stealth writes it all in
the git folder, beside the stealth config, so nothing the project tracks
changes.

Where a config is there (itos.yaml, or the stealth one), it writes nothing and
reports what is missing: what itos config check finds, and each hook of itos's
the git config does not declare, or a git that runs no hook its config
declares, naming what puts it right; exit 1 when anything is
missing, 0 when nothing is. It runs the binary called, whatever a pin says.
Then it notes, never counting them as missing: a people file the config names
that the repository lacks or cannot read, as itos config check warns, and a
pin behind the newest release, naming itos upgrade, where the release server
answers.

--policy <file> makes a fresh project of a template's policy: the file, an
ordinary complete itos config read from where you stand, is checked as itos
config check checks one and copied as the project's own config, comments and
all, never followed afterwards. commits.since becomes HEAD (none in a
repository with no commit) and no footer keeps a since; the policy's pin is
kept, or with none the newest release pinned as above; with no ledger the
starter's tasks/phase-{group}.yaml is added. Then fresh data at the config's
paths: its ledger's group 1 with the adoption task T-1 (no task under
--stealth), a registry with no item and the group 1 owned by nobody under
work.groups_key. For each kind with a smoke file, a smoke set there already
is the project's own, kept as it is when itos tests smoke check passes over
it; where there is none, one is made of the project's own tests, a command
adapter's from its list, run once in the project. Nothing else comes from the
template: no task, owner, status, question, smoke selection or proof result,
and nothing else the policy names runs. Before anything is written, git init
included, it refuses (exit 2) a policy config check refuses, an ID pattern,
type list or ledger layout that cannot hold T-1 in group 1, and a file it
would write outside the project (or the stealth folder) once symbolic links
are followed; and (exit 1) a config already there, any file the ledger's
pattern names or the registry, an empty one too, a smoke set there that the
smoke check refuses, with its problems, and a command adapter's list that
fails or prints what the protocol refuses. It never writes over a file, and
--config or ITOS_CONFIG is a usage error with it. A failure once writing
began says what this run did, and removes nothing.

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
the PATH being itos, is kept; a git there that is not itos is never
replaced, exit 1, the rest of init's work done. Run again where a config is,
it never asks: a shim not linked is reported, never counted as missing, and
--git-shim links it.

It offers the rules for agents the same way: the staging boundary, the same in every
repository, written into AGENTS.md between the lines <!-- itos:begin --> and
<!-- itos:end --> under a heading: stage only your own files, by explicit path.
The block does not route sessions to a guide. The commit policy the config
sets is itos guide commit's. A line importing the block, @AGENTS.md, goes at
the end of CLAUDE.md, written when it is missing. The block has one line a
paragraph, so a Markdown formatter leaves it as written. An AGENTS.md without
the markers gains the block at its end, its text kept; nothing outside them is
ever changed, and a file's CRLF line endings are kept. --agent-rules writes
it, --no-agent-rules declines; on a terminal with neither it asks, yes its
default answer; anywhere else it writes nothing and says how to. Run again
where a config is, it never asks: a block other than the one this itos writes
(an older itos's) is reported, never counted as missing, and --agent-rules
rewrites it.

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

--json: {"schema":1,"config","action":"initialized","policy"?,"git_init","since","files":[{"path",
        "action"}],"pin","pin_problem"?,"hooks","plugin","git_shim","agent_rules"} (hooks as itos hook
        install --json prints it), or {"schema":1,"config","action":"checked","missing":[{"rule",
        "message","fix"?,"area"}],"plugin","notes":[{"rule","message","fix"?,"area"}],"git_shim",
        "agent_rules"}, or for --policy {"schema":1,"config","action":"refused","problems":[{"rule",
        "message","fix"}]};
        plugin {"action","scope","excluded","problem"?}, action one of installed, already,
        offered, declined, no_claude, unknown (claude plugin list failed) or failed;
        git_shim {"action","link","on_path","before_git","problem"?}, action one of linked,
        replaced (another itos), kept, offered, declined, refused (a git that is not itos
        is there) or failed;
        agent_rules {"action","files":[{"path","action"}],"problem"?}, action one of written,
        current (the block is the one this itos writes), stale (it is not, and was not
        rewritten), offered, declined or failed; under a stealth config, stale too when
        the copy of AGENTS.md no longer matches it; each file's action wrote, updated or
        kept`,
	},
	Spec: declare.Spec{Flags: []declare.Flag{declare.Switch("--stealth"), declare.Value("--policy"), declare.Optional("--plugin"), declare.Switch("--git-shim"), declare.Switch("--no-git-shim"),
		declare.Value("--git-shim-dir"), declare.Switch("--agent-rules"), declare.Switch("--no-agent-rules")}},
}
