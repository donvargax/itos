# itos

**Every task is proven by commands, every commit names its work.** itos is a
task tool for repositories where people and coding agents work side by side:
a ledger of tasks whose "done" is a list of commands that must pass, commit
rules that tie each commit to the task or the scenarios it serves, and a CI
plan that runs what a push's commits name, in cost order. Its policy is one
YAML file, `itos.yaml`, so a project changes its rules without changing
code.

The name comes from _palitos_, the little sticks of a tally, the strokes
work is counted with; `-itos` is the Spanish diminutive, and said aloud it is
also _hitos_, milestones. It is not [withakay/ito](https://github.com/withakay/ito),
one letter away and also a task tool for agents, where an agent _says_ a task
is done; here a task is done when its commands pass.

## Two halves

A project works by its rules, and a rule is one of two kinds:

- **What a command can decide** is itos's: the commit's shape and footers,
  the paths each commit type may touch, which checks prove a task, what a CI
  run selects and in what order, who may take which work. A git hook or CI
  enforces it on every commit, whoever or whatever made it.
- **What no command can check** is the agent instructions': which session
  you are, how to split work into commits, when to stop and ask, how to brief
  and check an agent, what to do when a slice fails. They live in `AGENTS.md`
  (the implementing session's) and `docs/ORCHESTRATING.md` (the
  coordinator's).

A rule moves from the instructions into itos as soon as a command can decide
it, and the instructions then point to the check instead of restating it.

## What it does

- **Starting a session**: start every fresh agent session in a repository
  with `! itos go` (`!` runs it in Claude Code's prompt, so its output lands
  in the session). It prints the coordinator's guide, the repository's own
  notes, this clone's own notes and where the work stands, so the session you
  talk to coordinates: it specifies the work and hands each item to an agent.
  An agent it starts is told by its brief to run `itos guide work`, the
  implementer's guide, instead; no file asks an agent to judge which it is.
- **A ledger of tasks with executable checks** (`tasks/`): `itos task <id>`
  runs a task's `done_when` commands and says done, pending or failing, and
  `itos task add <id>` writes a new one into its group's file and its item
  into the work registry, committing the two alone.
- **Commit rules**: Conventional Commits; each `feat` or `fix` names the
  scenarios it turns green (`Scenarios: @ID-…`), every other type the task it
  belongs to (`Task: T-…`), and each type may touch only certain paths.
  `itos hook commit-msg` applies them before a commit, validates itos's
  own files as the commit stages them, and runs the static checks of the
  tasks it names, rejecting it when a finished task's check fails; `itos hook
pre-push` verifies the commits a push adds before they leave, refusing the
  push with how to fix them when one fails (and printing nothing when they
  pass), and `itos verify` re-checks a pushed range in CI, from
  `commits.since` on. `itos commit --task <id>` (or
  `--scenarios <ids>`, a flag for each footer of free text such as
  `--upgrading <text>`, and `--breaking <text>`) is `git commit` with the
  footers written for you, as git trailers, whether the message comes from
  `-m`, `-F` or the editor; a commit missing a footer its type requires is
  refused before git runs, naming the flag, and the hook judges the rest as
  typed footers.
- **Pushing**: `itos push` ends a piece of work after its commits. It pulls
  the branch's upstream with a rebase, whatever `pull.rebase` and
  `rebase.autostash` say, then pushes HEAD to it in a separate step, the
  pre-push hook running as for any push. It refuses to start while tracked
  files have uncommitted changes, leaves a rebase that stops for you to finish
  (`git rebase --continue`, then `itos push` again, or `git rebase --abort`)
  with nothing pushed, and never forces: a push the remote refuses is
  reported, not retried.
- **The git shim**: linked as `git` before the real git on your `PATH`, itos
  makes a hand-typed, scripted or editor's `git commit` and `git push` in a
  repository it manages run as `itos commit` and `itos push`, and passes
  everything else to the real git (below).
- **Named tests behind an adapter**: Gherkin is built in; any runner that can
  list its tests as JSON can be another kind. CI merges every selection of a
  kind into one run.
- **A CI plan**: `itos ci run` runs a push's steps and the checks of the
  tasks its commits name, in cost order, stopping at the first failure, with a
  shortcut for prose-only pushes; `itos ci plan` prints it without running
  anything.
- **Work routing**: `itos work` says what the person a session works for can
  start next, from `tasks/work-items.yaml` and `CONTRIBUTORS.md`; `itos work
list` lists every item with its title, done ones too, and `itos work show
<id>` one item with its scenarios and the commits that belong to it, by
  their footers, `--patch` adding their diffs. `itos work take <id>`
  sets an item in progress for the person, `itos work promote <idea>` makes
  an idea a slice or a task, `itos work done <id>` marks an item done once
  it has landed (its scenarios live, its commits pushed, its CI run green),
  and `itos work add <id>` and `itos work edit <id>` make an item and change
  one, each editing the registry in place and committing it alone.
- **Decisions waiting on the person**: `itos decision add <text> [--item <id>]`
  asks the person the work is for a question only they can answer, `q-1`,
  `q-2` and so on, `itos decision answer <id> <text>` keeps the answer beside
  it, `itos decision` lists the open ones and `itos decision show <id>`
  prints one; anything to take up with someone else, a question included, is
  `itos followup`. They are public: `asks.yaml` beside the work registry, each change
  committed alone, and `itos work show <id>` lists the questions naming the
  item.
- **Follow-ups with people**:
  `itos followup add <id> --with <who> --title … --note …` opens a thread,
  `itos followup note <id> <text>` appends a dated note,
  `itos followup close <id>` closes it, `itos followup` lists the open ones,
  `itos followup show <id>` prints one whole and
  `itos followup doc <id> <path>` writes it out as Markdown. The threads are
  yours alone, in the git folder (`follow-ups.yaml` under
  `git rev-parse --git-common-dir`/itos), never committed and shared by
  every worktree; no config is needed.

`tools/bin/itos --help` lists the commands, and `itos <command> --help` each
one. A command itos does not have runs `itos-<command>` from the `PATH`, so
itos takes extensions as git does: `docs/extensions.md` says how to write one.

## Status

**v2, Go only.** itos is the Go binary (`cmd/itos`, `internal/`), one file
with no runtime. The TypeScript v0 it began as has left the repository, and
from v2.0.0 no release carries its tarball. This repository governs itself
with itos, the Go binary: its own `itos.yaml`, ledger, hooks and CI.

itos's named tests are the Gherkin feature files in `features/`, whose steps
(Go, run by [godog](https://github.com/cucumber/godog)) treat itos as a black
box: they run whatever binary `ITOS_BIN` names in scratch repositories and
read its exit codes and output. The conformance corpus
(`tools/itos/conformance/`) is the regression record it must pass. `PLAN.md`
says what itos is, and `docs/decisions/` holds the decisions behind it.

## Install

**The newest release:** https://github.com/donvargax/itos/releases/latest.
Its notes give everything below filled in for it: the install script with
each platform's hash, the pin's two lines and the schema line. CI cuts a
release whenever a `feat` or a `fix` lands on `main` with its checks green,
its version computed from the commits since the last one. Each release holds:

- `itos-<version>-<os>-<arch>.tar.gz`, the Go binary, for linux and darwin on
  amd64 and arm64, and `itos-<version>-windows-amd64.zip`; each holds the
  binary (`itos`, or `itos.exe`), `LICENSE` and `README.md` at its top level;
- `itos.schema.json`, the config's JSON Schema;
- `checksums.txt`, the SHA-256 of every file above.
  `sha256sum --ignore-missing -c checksums.txt` checks the ones downloaded
  beside it, and `gh attestation verify <archive> -R donvargax/itos` checks an
  archive was built by this repository's release workflow.

Pin a release, never a branch, and pin each asset by its line in
`checksums.txt`, so a replaced release fails every later install.

**The binary**: download your platform's archive, check it against its line,
and unpack the binary into an ignored `.tools/bin/`. A project commits this as
a script that pins the version and each platform's hash: the newest
release's notes (Upgrading) give the whole script with both filled in, and
[v2.0.0's release notes](https://github.com/donvargax/itos/releases/tag/v2.0.0), Upgrading, step 1, the shim that runs it from your
hooks and CI.

Go developers can instead
`go install github.com/donvargax/itos/v5/cmd/itos@v<version>` for a v5
release (the module path ends in the major version from v2, as Go requires:
`/v4` for the v4 releases, `/v3` for v3.4.0 to v3.7.1, while v3.0.0 to v3.3.0
were cut from a path still ending in `/v2` and Go refuses them).

A version bump is the script's version and hashes, from the newer release's
notes.

**A global install** (from v2.1.0): install itos once
per machine, by either way above into a folder on your `PATH`, and pin the
release each repository runs in its `itos.yaml`:

```yaml
pin:
  version: <x.y.z> # the release, without its v
  checksums: <the SHA-256 of that release's checksums.txt> # sha256sum checksums.txt
```

(the release's notes give both lines filled in), or let `itos pin` write
them: `itos pin` moves the pin to the newest release, `itos pin <x.y.z>` to that
one, changing those two values and nothing else in the file, and prints the
release's notes to read before you commit the change.

The installed binary is then a launcher, never rewritten: in a repository that
pins another version it fetches that release into its cache, checks
`checksums.txt` against `pin.checksums` and the archive against its line, and
runs it with the same arguments and exit code. A release that cannot be fetched
or does not match exits 3 and nothing runs in its place. One hash covers every
platform, so the pin replaces the install script, and a version bump is the
two lines. A config without `pin` runs the binary that was called, so a
repository that installs itos its own way keeps it.

Where there is no `itos.yaml` at all (outside a project, or in a repository
that does not use itos) the launcher runs the newest release: it asks for it
at most once a day (`<base>/latest/download/checksums.txt`), fetches it into
the cache, checked against that list, and runs the newest of itself and the
releases the cache holds. In a repository that pins an older release than the
newest it runs the pin and says so on stderr, once a day per repository:

```
itos 2.3.0 is out (this repository pins 2.2.0): https://github.com/donvargax/itos/releases/tag/v2.3.0
```

It asks nothing where the answer goes unused (a config without `pin`), never
in CI (the `CI` variable set) and not with `ITOS_NO_UPDATE=1`, and never says
in CI or with `ITOS_NO_UPDATE_NOTICE=1`. A release server it cannot reach is
not an error: it waits a few seconds at most, once a day, and carries on with
what the cache has.

`ITOS_VERSION=<x.y.z>` runs another release for one call (trusting its
`checksums.txt` as fetched, unless it is the one pinned); `ITOS_RELEASES`
names where releases come from (`<base>/download/v<version>/<asset>`,
`https://github.com/donvargax/itos/releases` by default) and `ITOS_CACHE` the
cache (`itos/` in your user cache folder by default).

From v4.0.0 the hooks `itos hook install` writes call `itos`, this launcher,
unless the config sets `hooks.bin` (internal and unsupported, for a repository
that must run its own build, as this one sets `tools/bin/itos`), so every
machine that commits, and CI, needs it on the `PATH`.

**The hooks** (from v6.0.0): `itos hook install` declares itos's commit-msg
and pre-push hooks in the clone's own git config, `hook.itos-commit-msg` and
`hook.itos-pre-push` in `.git/config`, which git never commits and every
worktree of the clone shares. Git runs them beside the project's own hooks,
whatever `core.hooksPath` says, so a hook manager's files and settings are the
project's business and itos writes none of them. Run it once in each clone.
`itos commit`, `itos push`, the git shim's `git commit` and `git push`, and
the commands that commit the work registry refuse, exit 3, where git would
run none of itos's checks: no hook of itos's declared (run `itos hook
install`), or a git older than 2.54.0, the first that runs the hooks its
config declares (upgrade git). A project that called `itos hook commit-msg`
or `itos hook pre-push` from its hook manager's files takes those lines out,
or the checks run twice, and drops `hooks.manager` from its config, which
v6.0.0 refuses.

**In CI** (from the first release after v3.7.1): a CI runner gets the global
itos in one step, and the repository's pin chooses what runs, as on your
machine; a repository with no pin runs the version installed. On GitHub
Actions:

```yaml
- uses: donvargax/itos@v<x.y.z> # that release's launcher, on the PATH
```

(`with: { version: <x.y.z> }` installs another release; at a ref that is not a
version it installs the newest). Other CI runs the script the action runs,
fetched from a release tag, into a folder on its `PATH`:

```sh
curl -fsSL -o install-launcher https://raw.githubusercontent.com/donvargax/itos/v<x.y.z>/tools/bin/install-launcher
sh install-launcher "$HOME/.local/bin" <x.y.z>
```

It checks the archive against the release's `checksums.txt` and fails on any
mismatch or missing asset; with no version it installs the newest, and
`ITOS_RELEASES` moves where it fetches from.

**Ready a repository** (from v2.7.0): `itos init` in a
repository, or in a folder that is not one yet (it runs `git init` first),
writes a starter `itos.yaml` at its top, small and commented: the Conventional
Commits types under itos's own header lint, a `Task:` footer that every type
but `feat`, `fix` and `docs` needs, `docs` held to Markdown, `docs/` and
`tasks/` so that it cannot skip the footer for code, `commits.since` at HEAD so no commit written before
it is judged, and the hooks calling the global `itos`; a ledger,
`tasks/phase-1.yaml`, holding `T-1`, the task the commit that adds all this
names, and an empty work registry, `tasks/work-items.yaml`; and when
`features/` holds feature files, a `Scenarios:` footer that `feat` and `fix`
need and a smoke set, `features/smoke.yaml`, naming one live scenario of each
file. A file already there is kept. It pins the newest release, as `itos pin`
does (where it cannot reach the release server it pins nothing and says so),
then declares the hooks in the git config, as `itos hook install` does. Commit what it wrote
with `itos commit --task T-1 -m 'chore: adopt itos'`, and grow the config from
there: more path scopes, a CI plan, the people. Run again where a config is, it
writes nothing and lists what is missing (what `itos config check` finds, a
hook of itos's the git config does not declare, with what puts it right), exiting 1
when anything is, so it doubles as a check. `itos init --stealth` does the
same for one person in a repository whose team does not use itos (below).
It also offers the Claude Code plugin (below), through the `claude` on your
`PATH`, and never unasked: `--plugin` installs it for the project (in the
committed `.claude/settings.json`), `--plugin user` for every repository of
yours, `--plugin local` for you alone in this one, the default under
`--stealth`, which keeps `.claude/settings.local.json` out of `git status`;
on a terminal it asks, anywhere else it says how. Run again, it reports a
plugin not installed without counting it as missing. It offers the rules for
agents the same way: `--agent-rules` generates what the config decides (the
commit types, the footer each type needs, the paths each type may touch and
what the hooks and CI run) into `AGENTS.md`, between `<!-- itos:begin -->` and
`<!-- itos:end -->` at its end, the rest of the file kept, and adds an
`@AGENTS.md` line to `CLAUDE.md` so Claude Code reads it; `--no-agent-rules`
declines. The block is one line a paragraph, so a Markdown formatter leaves it
alone. Run again, init reports a block the config no longer matches, and
`itos init --agent-rules` rewrites only what is between the markers. Under
`--stealth` nothing tracked changes, so `--agent-rules` writes the block to
`.git/itos/AGENTS.md`, which every worktree shares, with a `CLAUDE.local.md`
importing it (and `@AGENTS.md`, when the project has one) for Claude Code and
an `AGENTS.override.md` for Codex holding a marked copy of the project's
`AGENTS.md`, then the block; both are listed in `.git/info/exclude`, and a file
of yours already there only gains the block. Run again, init also reports a
copy that no longer matches `AGENTS.md`. How to work with itos is not in the block: start a session with `! itos go`, and brief
an implementer to run `itos guide work` first.

**Where itos reads its config.** itos reads `itos.yaml` in the folder it runs
in, or the file `--config` or `ITOS_CONFIG` names; `--root <dir>` runs it as if
started in `<dir>`. With none of those and no `itos.yaml` in the folder, a run
inside a git repository reads the `itos.yaml` at the repository's top (or the
config in the git folder, below) and runs as if started there, so every path
the config names means what it means at the top, and the launcher runs the
version pinned there. A path you type (`itos commit check-paths`'s, a message
file, `itos commit`'s pathspecs) is still read from the folder you are in, as
git reads one.

**The git shim** (from v2.2.0): to have every `git commit` and `git push` in
a repository itos manages go through itos, whoever types them (you, a script,
an editor, an agent), link itos as `git` in a folder that comes before the real
git on your `PATH`:

```sh
itos git-shim install                 # a link named git beside the itos binary
itos git-shim install --dir ~/.local/bin/itos-shim   # or in a folder of your choice
```

It says whether that folder comes before the real git on the `PATH`; if not,
put it first (`export PATH="$HOME/.local/bin/itos-shim:$PATH"` in your shell's
profile), and run `hash -r` in a shell that has already looked git up. In a
repository with an `itos.yaml` (at its top, or in the folder you are in) or a
stealth config, from any folder of it, `git commit …` is then
`itos commit …` and `git push …` is `itos push …`, with the same arguments:
a commit missing a footer is refused before git runs, `git commit --task
T-001 -m …` writes the footer, and `git push --force` is refused. git's
`-C <dir>` and `-c <key>=<value>` before the command are honoured. Every other
command, and every command in any other repository, runs the real git (the
first `git` on the `PATH` that is not itos) with its arguments, input,
terminal and exit code untouched, at the cost of starting itos and a few file
checks. itos's own git, and any git a hook or check started by itos runs, is
always the real one. In a repository that pins a version, `git commit` is that
version's `itos commit` (v2.2.0 or later); under a pin older than v2.2.0,
which has no shim to hand it to, `git commit` and `git push` run the real git,
after a line on stderr saying so. The hooks and CI's `itos verify`
stay the gates: the shim is per machine and opt-in. To turn it off, remove the
link:

```sh
itos git-shim uninstall               # or: itos git-shim uninstall --dir <the folder>
```

Neither command touches a `git` that is not a link to itos.

**The Claude Code plugin**: this repository is also a Claude Code plugin
marketplace, and its one plugin, `itos`, is published from it, with a version
of its own, raised only when the plugin changes, not with each itos release. `itos init --plugin` installs it, or from Claude Code:

```
/plugin marketplace add donvargax/itos
/plugin install itos@itos
```

It runs the `itos` on your `PATH` (v2.3.0 or later) in every repository you
open, and does something only where itos manages the repository. It never
runs a program the repository ships, neither its `hooks.bin` nor a
`tools/bin/itos` at its top, since the guard runs before every Bash command
and the titles on every reply, and a repository you cloned would otherwise run
its own program just by being opened. A global install, above, is the
launcher, which lets each repository's pin pick the version from itos's
releases; to have a build of your own answer, put it first on your `PATH`.

- **Titles.** An itos ID in Claude's replies is drawn with its title beside
  it, `T-066` as `` `T-066: The itos plugin for Claude Code` ``, and "slice 43"
  as the registry's `slice-43`. Only the drawing changes: the transcript and
  what Claude reads back stay as written. The titles come from
  `itos work list --all --json` (plain `itos work list --json` on an older itos,
  which refuses `--all`) and `itos task list --json`, asked at the start of
  the session and of each turn; with no itos that answers, from
  `tasks/work-items.yaml`.
- **A guard.** A `PreToolUse` hook on Bash runs `itos guard claude-code`, which
  denies an agent's `git commit` or `git push`, its reason naming
  `itos commit --task <id>` (or `--scenarios <ids>`) or `itos push`, so the
  agent commits with the footers and pushes without forcing. Everything else
  gets no answer, so your own permission rules still decide. It reads the
  command as bash does, so `git -C . commit` and `make && git push` are
  caught; `sh -c '…'`, `eval` and scripts are not looked into, and the hooks
  stay the gates. Under a pin older than v6.0.0 the launcher answers for the
  guard with nothing; with no itos on the `PATH` the plugin answers nothing.

How to work with itos is not the plugin's: it is itos's own guides, versioned
with the binary, `itos go` for the session you talk to and `itos guide work` for
an agent it starts (above), so no flow needs the plugin. Try it in a repository
itos manages: start a session with `! itos go` (the IDs in Claude's answers
carry their titles), or ask Claude to commit with `git commit`, which the guard
turns into `itos commit`. To wire
only the guard, without the plugin, put the hook in `.claude/settings.json`:

```json
{
	"hooks": {
		"PreToolUse": [
			{ "matcher": "Bash", "hooks": [{ "type": "command", "command": "itos guard claude-code" }] }
		]
	}
}
```

**The schema, for editors.** An editor with a YAML language server checks an
`itos.yaml` as it is written, completes its keys and shows each one's
description and default, once the file's first line names the schema of the
release the project pins:

```yaml
# yaml-language-server: $schema=https://github.com/donvargax/itos/releases/download/v<x.y.z>/itos.schema.json
```

(the release's notes give the line for it).

`itos config check` stays the judge: the schema says less than it, never
something different.

## Use itos in a repository that doesn't

You can hold your own commits to itos in a repository whose team does not use
it, with nothing of it in the tree: keep the config in the git folder, as
`itos/itos.yaml` in the folder `git rev-parse --git-common-dir` names
(`.git/itos/itos.yaml` in a plain clone). Where there is no `--config`, no
`ITOS_CONFIG` and no `itos.yaml` in the root, itos reads that one, with nothing
to set, from any folder of the repository, running as if started at its top,
and every linked worktree of the repository shares it. The files it
names for itos's own data, the ledger, the work registry and the smoke sets,
are read beside it, in that folder, which git never commits; a `Task:` footer
is checked against that ledger at every commit, even with `read_at: commit`,
since no commit carries it. Its defaults fit one person, whatever it says of
them: its hooks call `itos`, the global install (`hooks.bin`), and no people
file is read, since you are the only one, so no owner is checked, and
`itos work` asks nobody who you are (neither gh nor a `work.identity` command)
and proposes every item as yours, whatever owner it names; `itos work --as
<handle>` still shows what that handle owns. Should the project adopt itos, its own `itos.yaml` in
the root wins.

`itos init --stealth` sets it up: a starter config there, its ledger and
registry beside it, the newest release pinned, and the hooks declared in the
git config (below), leaving `git status` as it was. By hand, the least of it:

```sh
dir="$(git rev-parse --git-common-dir)/itos"
mkdir -p "$dir/tasks"
printf 'phases: {}\nitems: []\n' > "$dir/tasks/work-items.yaml"
cat > "$dir/itos.yaml" <<'YAML'
version: 1
ledger:
  files: "tasks/phase-{group}.yaml" # beside this file, in the git folder
commits:
  types: [feat, fix, refactor, perf, test, build, ci, chore, docs, style, revert]
  footers:
    Task:
      source: ledger
      required_for: [refactor, perf, test, build, ci, chore, revert]
      validate_for: all
      read_at: commit
YAML
itos config check
```

A global install runs the newest release for such a config unless it pins one
(`pin`, above). `itos hook install` then declares itos's hooks in the
repository's own `.git/config`, which git never commits:

```ini
[hook "itos-commit-msg"]
	event = commit-msg
	command = itos hook commit-msg
```

and `hook.itos-pre-push` beside it, since the pre-push hook verifies the
commits a push adds and you have no CI of the project's to judge them. Git
(2.54.0 and later) runs a hook declared in its config as well as the
project's own in `core.hooksPath` or `.git/hooks`, so the project's hooks and
settings stay as they are, both run on every commit, and a hook manager that
resets `core.hooksPath` cannot remove itos's. Running it again changes
nothing. An older git, which does not run them, makes `hook install` say so
and exit 3. If an earlier `hook install` wrote shims into `.git/hooks`,
delete them, or the hooks run twice; if your config named another
`hooks.bin`, run `itos hook install` again so its hooks call `itos`.

A footer in a commit message is what everyone reads, so here the links, the
footers naming tasks and scenarios, live in git notes instead: commit with
`itos commit --task <id>` (and `--scenarios <ids>`), which writes them as a
note on the new commit, in `refs/notes/itos`, never in its message. A footer
of free text (`--upgrading <text>`) and `--breaking <text>` are content, and
stay in the message. `git push` does not send that ref
unless you ask (`git push origin refs/notes/itos`), and `git log
--notes=itos` shows it. The commit-msg hook judges the footers `itos commit`
hands it, and refuses a commit that needs one made with a bare `git commit`,
and a link typed into the message. `itos commit` adds `refs/notes/itos` to
`notes.rewriteRef` in the repository's config, so `git commit --amend` and
`git rebase` carry a commit's note to the commit they make, and an amend
passes the hook on the note it will carry: `itos commit --amend` tells the
hook it amends, while for a plain `git commit --amend` the hook guesses it
from the author's date, which `--date` or `--reset-author` defeats. `itos verify`
and `ci plan` read each commit's links from its note.

Others' commits here follow none of your rules, so `itos verify` and `itos ci
plan` with no range judge only yours: the commits of HEAD that no remote
branch has (`git rev-list HEAD --not --remotes`), every commit when there is
no remote. Run `itos verify` before you push, and it checks exactly what the
push will send. In a project's own repository both still need `<from> <to>`.
`itos push` pushes HEAD alone to its branch, so the notes stay local here
too, and a rebase it runs carries them.

## Working on it

```sh
vp install                               # dependencies and the git hooks
go test ./features -count=1              # every feature, against tools/bin/itos
go test ./cmd/... ./internal/...         # the unit tests
tools/bin/itos task --phase 1            # the phase's tasks and their state
vp run work                              # what you can take next
```

Read `AGENTS.md` before changing anything: the hooks run the checks on every
commit, and it says how work is split into commits.

## Where things are

| File                    | What it holds                                                             |
| ----------------------- | ------------------------------------------------------------------------- |
| `PLAN.md`               | What itos is for, its model, its schema and its risks.                    |
| `docs/decisions/`       | The decisions, one record each; its `README.md` lists those that stand.   |
| `docs/ARCHITECTURE.md`  | How the code is put together as built, the gates included.                |
| `AGENTS.md`             | The working rules for a session that implements.                          |
| `docs/ORCHESTRATING.md` | This repository's own notes for the coordinator, after `itos go`'s guide. |
| `docs/PHASES.md`        | Who owns which phase, and how work is routed.                             |
| `tasks/work-items.yaml` | The one list of open work: owners, statuses, dependencies, ideas.         |
| `tasks/`                | The ledger: every non-feature task and the checks that prove it.          |
| `features/`             | The scenarios: what itos does, through its command line.                  |
| `itos.yaml`             | This repository's policy, which every gate reads.                         |

The repository was made from the project template
[donvargax/project-template](https://github.com/donvargax/project-template),
whose setup is phase 0 here.

## Licence

[AGPL-3.0](LICENSE).
