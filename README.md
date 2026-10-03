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
  a pushed range in CI, from `commits.since` on.
- **Named tests behind an adapter**: Gherkin is built in; any runner that can
  list its tests as JSON can be another kind. CI merges every selection of a
  kind into one run.
- **A CI plan**: `itos ci run` runs a push's steps and the checks of the
  tasks its commits name, in cost order, stopping at the first failure, with a
  shortcut for prose-only pushes; `itos ci plan` prints it without running
  anything.
- **Work routing**: `itos work` says what the person a session works for can
  start next, from `tasks/work-items.yaml` and `CONTRIBUTORS.md`.

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

Each release on a `v*` tag holds:

- `itos-<version>-<os>-<arch>.tar.gz`, the Go binary, for linux and darwin on
  amd64 and arm64, and `itos-<version>-windows-amd64.zip`; each holds the
  binary (`itos`, or `itos.exe`), `LICENSE` and `README.md` at its top level;
- `itos.schema.json`, the config's JSON Schema;
- `checksums.txt`, the SHA-256 of every file above, which the release's
  description prints. `sha256sum --ignore-missing -c checksums.txt` checks the
  ones downloaded beside it.

Pin a release, never a branch, and pin each asset by its line in
`checksums.txt`, so a replaced release fails every later install.

**The binary**: download your platform's archive, check it against its line,
and unpack the binary into an ignored `.tools/bin/`. A project commits this as
a script that pins the version and each platform's hash
(`docs/releases/v2.0.0.md`, Upgrading, step 1, has one for every platform, and
the shim that runs it from your hooks and CI):

```sh
version=2.0.0 platform=linux-amd64   # or linux-arm64, darwin-amd64, darwin-arm64
archive="itos-$version-$platform.tar.gz"
curl -fsSLO "https://github.com/donvargax/itos/releases/download/v$version/$archive"
echo "<the hash in the release's checksums.txt>  $archive" | sha256sum -c -
mkdir -p .tools/bin && tar -xzf "$archive" -C .tools/bin itos
```

Go developers can instead
`go install github.com/donvargax/itos/v2/cmd/itos@v<version>` (the module
path ends in `/v2` from v2.0.0, as Go requires).

A version bump is the same again with the new version and hashes.

**A global install** (from the first release after v2.0.0): install itos once
per machine, by either way above into a folder on your `PATH`, and pin the
release each repository runs in its `itos.yaml`:

```yaml
pin:
  version: 2.1.0 # the release, without its v
  checksums: <the SHA-256 of that release's checksums.txt> # sha256sum checksums.txt
```

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
itos 2.1.0 is out (this repository pins 2.0.0): https://github.com/donvargax/itos/releases/tag/v2.1.0
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

**The schema, for editors.** An editor with a YAML language server checks an
`itos.yaml` as it is written, completes its keys and shows each one's
description and default, once the file's first line names the schema of the
release the project pins:

```yaml
# yaml-language-server: $schema=https://github.com/donvargax/itos/releases/download/v2.0.0/itos.schema.json
```

`itos config check` stays the judge: the schema says less than it, never
something different.

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
