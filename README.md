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

- **A ledger of tasks with executable checks** (`tasks/`): `itos task <id>`
  runs a task's `done_when` commands and says done, pending or failing.
- **Commit rules**: Conventional Commits; each `feat` or `fix` names the
  scenarios it turns green (`Scenarios: @ID-…`), every other type the task it
  belongs to (`Task: T-…`), and each type may touch only certain paths.
  `itos hook commit-msg` applies them before a commit, validates itos's
  own files as the commit stages them, and runs the static checks of the
  tasks it names, rejecting it when a finished task's check fails; `itos verify` re-checks
  a pushed range in CI, from `commits.since` on. `itos commit --task <id>` (or
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
list` lists every item with its title, done ones too.

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
has the order of the work.

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
`docs/releases/v2.0.0.md`, Upgrading, step 1, the shim that runs it from your
hooks and CI.

Go developers can instead
`go install github.com/donvargax/itos/v2/cmd/itos@v<version>` (the module
path ends in `/v2` from v2.0.0, as Go requires).

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

(the release's notes give both lines filled in).

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
of its own. Install it from Claude Code:

```
/plugin marketplace add donvargax/itos
/plugin install itos@itos
```

It calls the `itos` on your `PATH` (v2.3.0 or later; a global install,
above, lets each repository's pin pick the version), in every repository
you open, and does something only where itos manages the repository:

- **Titles.** An itos ID in Claude's replies is drawn with its title beside
  it, `T-066` as `` `T-066: The itos plugin for Claude Code` ``, and "slice 43"
  as the registry's `slice-43`. Only the drawing changes: the transcript and
  what Claude reads back stay as written. The titles come from
  `itos work list --json` and `itos task list --json`, asked at the start of
  the session and of each turn; with no itos that answers, from
  `tasks/work-items.yaml`.
- **A skill**, `itos`, on working in such a repository: find work with
  `itos work`, commit with `itos commit --task <id>` or `--scenarios <ids>`,
  push with `itos push`, and fix what a gate reports rather than run the
  gates by hand or skip them.
- **A guard.** A `PreToolUse` hook on Bash runs `itos hook pre-tool-use`, which
  denies an agent's `git commit` or `git push`, its reason naming
  `itos commit --task <id>` (or `--scenarios <ids>`) or `itos push`, so the
  agent commits with the footers and pushes without forcing. Everything else
  gets no answer, so your own permission rules still decide. It reads the
  command as bash does, so `git -C . commit` and `make && git push` are
  caught; `sh -c '…'`, `eval` and scripts are not looked into, and the hooks
  stay the gates. Under a pin older than v2.3.0 the launcher answers for the
  hook with nothing; with no `itos` on the `PATH` the plugin answers nothing.

Try it in a repository itos manages: ask Claude what to work on next (it runs
`itos work`, and the IDs in its answer carry their titles), or ask it to
commit with `git commit`, which the guard turns into `itos commit`. To wire
only the guard, without the plugin, put the hook in `.claude/settings.json`:

```json
{
	"hooks": {
		"PreToolUse": [
			{ "matcher": "Bash", "hooks": [{ "type": "command", "command": "itos hook pre-tool-use" }] }
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
(`pin`, above). `itos hooks install` then declares itos's hooks in the
repository's own `.git/config`, which git never commits:

```ini
[hook "itos-commit-msg"]
	event = commit-msg
	command = itos hook commit-msg
```

and `hook.itos-pre-push` too when the config sets `hooks.pre_push`. Git
(2.5x) runs a hook declared in its config as well as the project's own in
`core.hooksPath` or `.git/hooks`, so the project's hooks and settings stay as
they are, both run on every commit, and a hook manager that resets
`core.hooksPath` cannot remove itos's. Running it again changes nothing. An
older git, which does not run them, makes `hooks install` say so and exit 3;
`--manager git` still writes shims into `.git/hooks` where the project sets no
`core.hooksPath`. If an earlier `hooks install` wrote those shims, delete
them, or the hooks run twice; if your config named another `hooks.bin`, run
`itos hooks install` again so its hooks call `itos`.

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

| File                    | What it holds                                                                |
| ----------------------- | ---------------------------------------------------------------------------- |
| `PLAN.md`               | What itos is for, its model, schema and command line, and the order of work. |
| `docs/ARCHITECTURE.md`  | How the code is put together as built, the gates included.                   |
| `docs/HANDOFF.md`       | Only what the next session should do; the coordinator rewrites it.           |
| `AGENTS.md`             | The working rules for a session that implements.                             |
| `docs/ORCHESTRATING.md` | The working rules for the session that coordinates.                          |
| `docs/PHASES.md`        | Who owns which phase, and how work is routed.                                |
| `tasks/work-items.yaml` | The one list of open work: owners, statuses, dependencies, ideas.            |
| `tasks/`                | The ledger: every non-feature task and the checks that prove it.             |
| `features/`             | The scenarios: what itos does, through its command line.                     |
| `itos.yaml`             | This repository's policy, which every gate reads.                            |

The repository was made from the project template
[donvargax/project-template](https://github.com/donvargax/project-template),
whose setup is phase 0 here.

## Licence

[AGPL-3.0](LICENSE).
