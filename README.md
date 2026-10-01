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
  `itos hook commit-msg` applies them before a commit, and validates itos's
  own files as the commit stages them; `itos verify` re-checks
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
one.

## Status

**v0, in TypeScript** (`tools/itos/`), run by Node directly. This repository
governs itself with it: its own `itos.yaml`, ledger, hooks and CI.

**Next, the Go port**: one binary, one YAML config, built here beside the
TypeScript and judged by the same tests. itos's named tests are the Gherkin
feature files in `features/`, whose steps (Go, run by
[godog](https://github.com/cucumber/godog)) treat itos as a black box: they
run whatever binary `ITOS_BIN` names in scratch repositories and read its exit
codes and output. The conformance corpus (`tools/itos/conformance/`) is the
regression record both implementations must pass. `PLAN.md` has the order of
the work.

## Install

Each release on a `v*` tag holds `itos-<version>.tgz`, v0 packed to one
JavaScript file with no runtime dependencies (built for Node 24), and
`checksums.txt`, its SHA-256. Pin a release, never a branch: check the
tarball against the hash its `checksums.txt` lists, then add the tarball's
URL as a dependency, so `package.json` pins the URL and the lockfile the
tarball's integrity, and a replaced release fails every later install:

```sh
version=0.1.0
url="https://github.com/donvargax/itos/releases/download/v$version/itos-$version.tgz"
curl -fsSLO "$url"
echo "<the hash in the release's checksums.txt>  itos-$version.tgz" | sha256sum -c -
npm install --save-dev "$url"   # or pnpm add -D "$url"
```

The package's bin is `itos` (`npx itos --help`). A version bump is the same
again with the new version.

## Working on it

```sh
vp install                               # dependencies and the git hooks
go test ./features -count=1              # every feature, against tools/bin/itos
vp test                                  # the unit tests
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
