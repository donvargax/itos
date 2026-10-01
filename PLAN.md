# itos — plan

itos is a task tool for a repository where people and coding agents work
side by side. It holds the ledger of tasks, each proven by commands; the
commit rules that tie every commit to the task or the scenarios it serves;
the CI plan that runs what a push's commits name, in cost order; and the
routing that says who may take which work. Its policy is one YAML file, so a
project changes its rules without forking code. It is deliberately not a
command runner, a hook manager, a changelog writer or a test runner: it calls
those, and decides only what they are asked to do.

This file holds the decisions and the order of the work. How the code is
actually put together is `docs/ARCHITECTURE.md`; what has been built, and
why, is the history (`vp run changelog`); who works which phase is
`docs/PHASES.md`, and the open work is `docs/work-items.yaml`.

---

## 1. Goals, non-goals and the name

### Goals

- **Our own tool, sharing the ledger and the policy.** Not Taskfile, just or a
  workspace task runner: what is worth sharing between projects is the ledger
  and the rules, not command running. Copying the tooling from project to
  project made each copy drift; one tool with one config ends that.
- **Usable in any project: one Go binary, one YAML config.** The binary has
  no runtime to install, and the config is data any language can read.
- **In two steps.** First the TypeScript the tool began as, generalised in
  place: every hardcoded table became config, named tests went behind an
  adapter, every command gained `--json`, the host-bound parts became
  providers. That is v0, in this repository. Then the stable core is ported
  to Go here, one command group at a time, judged by the same tests.
- **Public, AGPL-3.0, consumed as a pinned release.** A consumer pins a
  version and checks its checksum; it never tracks a branch.
- **Version 1 covers**: the ledger and its checks (`run`, `fails`,
  `after: push`, `timeout`, `prose`, `cost`; statuses done, pending, failing,
  review); CI selection and cost order driven by footers; commit rules
  (header lint, footers, path scopes); work routing; hook entry points and
  one-line shims; YAML for the config and the ledger.
- **No host lock-in.** Where a CI range starts and who a session works for are
  providers: `github` now, `gitlab` and `forgejo` with the Go port, `command`
  and `none` always.

### Non-goals

- **A command runner.** itos runs check commands, CI steps and one merged
  command per kind of named test, nothing else. Scripts and task runners run
  the rest.
- **Hook management.** Vite+, husky, lefthook, pre-commit or plain git install
  hooks; itos gives the entry points and writes the shims. A project's own
  pre-commit hook stays its own.
- **The changelog.** git-cliff reads the `Task:` and `Scenarios:` footers;
  itos keeps them and nothing more.
- **Test running.** It builds one command per kind from a template; the
  project's runner executes it.
- **Host glue.** Deploy keys, browser installs, the red-nightly issue and
  artifact uploads stay in the workflows.

### Rules for itos, rules for agents

itos and a project's agent instructions work together, and the line between
them is whether a rule can be decided by a command:

- **itos holds every rule a command can decide**: commit shape and footers,
  path scopes, which checks prove a task, what a CI run selects and in what
  order, who may take which work. A hook or CI enforces it on every commit,
  whoever or whatever made it.
- **The agent instructions hold what no command can check**: which session
  you are, how to split work, when to stop and ask, how to brief and check an
  agent, what to do when a slice fails, how the user wants to be asked.

A rule moves from the instructions into itos as soon as it can be checked,
and the instructions then drop it or point to the check: a rule written twice
drifts.

### Name

**`itos`, from _palitos_**: the little sticks of a tally, the strokes work is
counted with, and `-itos`, the Spanish diminutive; said aloud it is also
_hitos_, milestones, which is what a ledger of done-when tasks tracks. The
project, the repository, the binary and the config (`itos.yaml`,
`ITOS_CONFIG`) all use it. npm's `itos` is taken by an empty library, so an
npm wrapper, if one is ever wanted, publishes as `palitos` and installs
`itos`.

The neighbour to set apart is [withakay/ito](https://github.com/withakay/ito),
one letter away and also an agent-facing task tool, where an agent _says_ a
task is done. The README's first line states the difference: every task is
proven by commands, every commit names its work. itos never uses its file
names (`.ito/`, `ito.json`).

## 2. References

- [Conventional Commits 1.0](https://www.conventionalcommits.org/) and
  `@commitlint/config-conventional`, which the header lint follows.
- [Gherkin](https://cucumber.io/docs/gherkin/) and
  [godog](https://github.com/cucumber/godog), for itos's own named tests.
- [All Contributors](https://allcontributors.org), the people source
  `CONTRIBUTORS.md` follows.
- The conformance corpus, `tools/itos/conformance/`: what v0 does, as cases
  run through its command line. Any implementation passes it.
- The project template this repository was made from,
  [donvargax/project-template](https://github.com/donvargax/project-template),
  and itos's first consumer, a game-art editor whose tooling itos was
  extracted from.

## 3. Decisions

| Topic               | Decision                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Implementation      | v0 is TypeScript in `tools/itos/`, run by Node directly. The Go port lands beside it (`cmd/itos`, `internal/`), and the TypeScript goes once Go passes everything.                                                                                                                                                                                                                                               |
| Layout              | The TypeScript stays in `tools/itos/`: it is temporary, and its corpus, fixtures and entry point all name that path. `commits.path_sets.implementation` names it, and the scope rules read `$implementation` where a template project reads `src/**`; the Go port adds its packages to that one set.                                                                                                             |
| Named tests         | Gherkin feature files at the root, `features/`, run by godog through `go test`. The steps treat itos as a black box: they run the binary `ITOS_BIN` names in scratch repositories and assert exit codes and output, so the same files judge the TypeScript and the Go port, unchanged.                                                                                                                           |
| Selecting scenarios | godog's tag filter takes exact tags joined by commas; itos's run templates join IDs into one regular expression with `\|`. The harness takes `-scenarios=<regexp>` over the tags and turns it into godog's filter, so the templates stay as they are.                                                                                                                                                            |
| Regression corpus   | The conformance YAML stays as a corpus both implementations must pass, run by a CI step of its own, not as named tests.                                                                                                                                                                                                                                                                                          |
| Where checks start  | `commits.since` names the commit where verification starts: `verify` and every range check skip it and its ancestors; the commit-msg hook is unaffected. A repository made from a template begins with one squashed commit no rule passes, and a project adopting itos has a history written before its rules; both start clean with it. Chosen over skipping the root commit, which covers only the first case. |
| Header lint         | Delegated to commitlint (`commits.header_lint.hook` and `stdin`) until a built-in lint matches it on a whole history; then the built-in one replaces it.                                                                                                                                                                                                                                                         |
| Coverage            | Collected for the audit's scores, held to no threshold: itos is proven through its command line, which a unit test's coverage cannot see.                                                                                                                                                                                                                                                                        |
| Licence             | AGPL-3.0 for the whole repository.                                                                                                                                                                                                                                                                                                                                                                               |
| Distribution        | Release archives plus checksums, installed pinned; `go install` for Go developers. v0 releases are the TypeScript, packed to JavaScript.                                                                                                                                                                                                                                                                         |

## 4. The model

Each concept is defined once; the bold names are used in the schema and the
command line.

| Concept                  | Definition                                                                                                                                                                                                                                                                                              |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Ledger**               | The YAML files listing the **tasks** (`tasks/phase-*.yaml`).                                                                                                                                                                                                                                            |
| **Group**                | A set of tasks, from the ledger file's name (`phase-3.yaml` → `3`); the config names the label (`phase`).                                                                                                                                                                                               |
| **Task**                 | `id`, `type` (a commit type), `title`, `why`, `done_when` (a list of **checks**). **Status**: `done` (every check passes), `failing` (one fails), `pending` (none fails, one waits for the push), `review` (no checks).                                                                                 |
| **Check**                | One command in `done_when`. **Mode** `run` (exit 0) or `fails` (non-zero). Optional `after: push`, `timeout` (seconds, default 600), `prose: true` (prose alone can change its result), `cost`. It does real work, never a grep over config.                                                            |
| **Cost class**           | `static` (needs nothing built, takes seconds) or `late`. A check's or step's own `cost:` wins; else a config pattern over the command; else `late`. A task's checks keep their written order: a check below a late one is late too. A CI run goes static steps, static checks, late steps, late checks. |
| **Footer**               | A `Key: id id, …` commit line naming IDs of one **source**: the ledger (`Task:`) or one kind of named test (`Scenarios:`).                                                                                                                                                                              |
| **Kind** (of named test) | Tests with stable IDs a footer can name and CI can select. Has an **adapter**, **run templates**, an optional **smoke set** and optional **range checks**.                                                                                                                                              |
| **Adapter**              | Lists a kind's tests at a tree (the working tree, the index, a commit): `id`, `file`, `live`. Built in (`gherkin`) or a command speaking JSON.                                                                                                                                                          |
| **Selection**            | What one run of a kind selects: `whole`, `ids`, or a `pattern` from a task check. A run's selections merge into one command.                                                                                                                                                                            |
| **Smoke set**            | Per-kind IDs every push runs, each with a `why`. Every file with a live test lists one; each listed ID is live in its file; more than one needs `more`.                                                                                                                                                 |
| **Range check**          | A kind's rule on how tests may change between two trees, run as a command: staged in the hook, over the range in CI (the scenario-move rule).                                                                                                                                                           |
| **Commit rules**         | **Header lint** (Conventional Commits, delegated or built in), **footer rules** (required per type, IDs exist and are live at the commit), **scope rules** (`only`, `never`, `must_touch` per type).                                                                                                    |
| **Range**                | The commits a CI run judges, `from..to`, less `commits.since` and its ancestors. An empty or unreadable `from` runs everything; a zero SHA is a new branch.                                                                                                                                             |
| **Range provider**       | Finds `from`: the last green CI run's head on the main branch if it is an ancestor of `to`, else empty. A pull request's base wins.                                                                                                                                                                     |
| **CI step**              | A command every full run executes, in order; one step may be a kind's merged run.                                                                                                                                                                                                                       |
| **Coverage**             | A named task's check is not rerun when a step did it (equal to a step, or a `covers` rule); a run of a kind is merged; a nightly-only check waits for the nightly.                                                                                                                                      |
| **Prose**                | Paths that can only break their own formatting. A prose-only range runs the prose steps and the named tasks' static and `prose: true` checks, and lists the rest as left out.                                                                                                                           |
| **Plan**                 | What a CI run executes and in what order, computed without side effects from the range, the config, the ledger and the registry. `ci plan --json` prints it.                                                                                                                                            |
| **Work registry**        | `docs/work-items.yaml`: group owners and **work items** (owner, status, dependencies, kind). A named task whose item is `todo` waits in CI.                                                                                                                                                             |
| **People source**        | Who may own work: the All Contributors table in `CONTRIBUTORS.md`, `.all-contributorsrc`, or a YAML list.                                                                                                                                                                                               |
| **Identity provider**    | Who a session works for: `--as`, else the provider (`gh api user`, a command, none).                                                                                                                                                                                                                    |

**Why written order, not `needs:`.** A cost class read from a command alone
once ran a check before the check above it that wrote the file it read.
Authors already write a task's checks in the order they depend on, and
`itos task` runs them in that order, so CI keeps it; `needs:` would need
check IDs and a graph to validate, for no case written order misses. A
`sh -c` is never static by pattern, since it wraps anything: a static one
says `cost: static`, and `config check` rejects a `cost: static` below a late
check, which could never take effect.

## 5. Config and ledger schema

`itos.yaml` at the root, or `--config` / `ITOS_CONFIG`. An unknown key is an
error that names the key it misspells. This repository's own `itos.yaml` is
the worked example, every table commented; `tasks/README.md` explains the
policy it sets.

- **Globs**: `*` does not cross `/`, `**` does, `**/` may match nothing,
  `{a,b}`, whole-path match, no `/` means the root only. The Go port
  reimplements exactly this, pinned by the corpus's glob table.
- **Command patterns** are regular expressions over the command with its
  whitespace collapsed (RE2 in Go: none uses what RE2 lacks).
- **Templates** use `{name}`; a value going into a shell is single-quoted.
- **`shell`** is the argv prefix for every command, `[sh, -c]` by default.
- **`commits.since`**: the full SHA of the commit where verification starts.

The sections: `ledger` (its files, the group in their names, the ID pattern,
the check timeout), `commits` (types, the header lint, footers, path sets,
scopes, `since`), `tests` (one entry per kind: adapter, root, ID pattern,
run and recognize templates, smoke set, range checks), `ci` (steps, prose,
cost patterns, covers, nightly, the range provider), `work` (registry,
people, identity) and `hooks` (manager, the binary the shims call, the
pre-push commands).

**The ledger schema**, validated strictly:

```yaml
- id: T-020 # required; matches ledger.id; unique across files
  type: ci # required; one of commits.types
  title: GitHub Actions CI # required
  why: > # optional
    Every push runs the same gates as local development.
  done_when: # optional; empty is review
    - run: vp run ci # exactly one of run / fails
    - fails: printf 'update things\n' | itos commit check-message -
    - run: sh -c 'test -z "$(git ls-files dist)"'
      cost: static # static | late; wins over ci.cost.static
    - run: gh run list … | grep -qx true
      after: push
      timeout: 900
    - run: itos work check
      prose: true
```

**The header lint** follows `@commitlint/config-conventional`: the type from
`commits.types`, lower case, never empty; a subject, not sentence-, start-,
pascal- or upper-case, without a full stop; the header at most 100
characters and trimmed; body and footer lines at most 100; a blank line
before the body and the footers (warnings). `use: command` delegates
(commitlint, `cog verify`), `use: none` turns it off, and `alongside:
builtin` runs the built-in lint beside the delegate, warning only, until the
two agree.

## 6. Named tests and their adapters

An adapter answers one question: which tests exist at a tree, and which are
live. Merging, recognition and the smoke rule are itos's, from the kind's
config.

**`<command> list --at <tree>`** (`worktree`, `index` or a SHA) prints one
JSON object and exits 0:

```json
{
	"protocol": 1,
	"tests": [
		{ "id": "ID-LIB-01", "file": "library.feature", "live": true },
		{ "id": "ID-SOCK-02", "file": "sockets.feature", "live": false }
	],
	"files": ["library.feature", "sockets.feature", "empty.feature"]
}
```

`id` has no tag prefix; `file` is the smoke rule's unit, relative to the
kind's root; `files` lists every file. A non-zero exit or invalid JSON fails
whatever needed the list, with the adapter's stderr: the tool never guesses.
With `supports_at: false` a historical commit's footers are read against the
working tree, with one warning per run.

Without an adapter's own `plan` and `recognize`, the templates decide: the
`whole` command if any selection is whole; otherwise each `ids` selection
becomes a pattern by `ids_pattern` (its `{ids}` the IDs joined with `|`), the
patterns are deduplicated in order and combined by `join`. In `recognize`, a
`{pattern}` matches one shell word; a check matching no template runs as it
is.

**The Gherkin adapter** reads every `*.feature` under the root at the tree. A
scenario is the block starting at a tag line holding `@<id>`, up to the next
such line; the header is everything before the first, the Background
included. A scenario is live when neither its tag line nor its file's header
holds the wip tag.

**This repository's kind** is `scenario`: `features/`, run by
`go test ./features -count=1`, a selection as
`-scenarios='^@(?:ID-A|ID-B)$'`. **A Go project** that names Go tests in its
footers points `adapter.command` at a script that turns `go test -list` into
the JSON above, with `select: "go test ./... -run {pattern}"` and
`ids_pattern: "^(?:{ids})$"`; pytest would use `select: "pytest -k
{pattern}"` and `join: { each: "{p}", sep: " or " }`.

## 7. The command line

Global flags: `--config`, `--root`, `--json`, `-q`.

| Command                                                                 | What it does                                                                                                                                  |
| ----------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `task <id>…`, `--phase <g>`, `--pending`, `task list`                   | Runs the tasks' checks in written order, or lists them; a status table.                                                                       |
| `work [--as <h>]`, `work check [<file>]`                                | Who the session works for and what they can start; validates the registry.                                                                    |
| `commit check-message <file\|->`, `commit check-paths --type <t> <p>…`  | The header lint and the footer rules on one message; the scope rules alone, to plan a split.                                                  |
| `verify <from> <to>`                                                    | Re-checks every non-merge commit of the range after `commits.since` (message with footers at that commit, paths), then each range check once. |
| `tests list <kind> [--at <tree>]`, `tests smoke check\|ids\|run <kind>` | The adapter's listing; the smoke rule, the smoke IDs, the smoke run.                                                                          |
| `ci plan <from> <to>`, `ci run [<from> <to>]`, `--nightly`              | Prints the plan; runs it, stopping at the first failure.                                                                                      |
| `ci scope <from> <to>`, `ci range --head <sha> [--base <sha>]`          | Whether a range is prose only; where a push's range starts.                                                                                   |
| `hook commit-msg <file>`, `hook pre-push <remote> <url>`                | The hooks' entry points.                                                                                                                      |
| `hooks install [--manager <m>] [--print] [--force]`                     | Writes the one-line shims for the hook manager it detects, or prints its snippet.                                                             |
| `config check [--print-defaults]`                                       | Validates the config, the ledger, the registry and the smoke sets.                                                                            |
| `version [--check]`                                                     | `--check` exits 1 if the binary does not satisfy `requires`.                                                                                  |

**Exit codes:** 0 success; 1 a policy failure (a check failed, a commit
rejected, an unknown task); 2 a usage or config error; 3 a missing
environment. In `ci run` a failing step exits with its own code. A missing
identity and a failing range provider are not errors.

**JSON.** Every command takes `--json`: one object on stdout with
`"schema": 1`, the logs on stderr, keys only ever added. Each problem carries
a sentence, a `rule` id and, where one exists, a `fix`.

**Wording.** The text output is kept line for line, because agents and
instructions quote it (`Commit rejected (see tasks/README.md):`,
`<n>/<m> commits pass the commit rules`, `CI failed at: <step>`), and every
`--help` text is a case of the corpus.

## 8. Architecture

```
itos/
  tools/itos/      v0, TypeScript: one module per concern, main.ts the CLI
  tools/itos/conformance/   the regression corpus and its runner
  tools/bin/itos   the entry point the hooks and the scripts call
  features/        the named tests: Gherkin, godog steps, the smoke set
  cmd/itos/        the Go binary (the port)
  internal/        its packages: config, glob, git, shell, ledger, check,
                   message, scope, tests, plan, ci, providers, work, hook, out
  tasks/           the ledger
  itos.yaml        this repository's own policy
```

The Go port shells out to git as the TypeScript does, has one dependency
beside godog's (`go.yaml.in/yaml/v3`), builds with `CGO_ENABLED=0`, and is
pinned by go.mod's `toolchain` line.

## 9. Phases

A phase is done when all its scenarios pass without `@wip` and
`itos task --phase <n>` reports every task done.

### Phase 0: the template's setup

Inherited from the project template: the gates, the hooks, itos v0, CI and
the nightly (`tasks/phase-0.yaml`).

### Phase 1: itos v0 in its own repository

The template's demo app goes; the named tests become the features at the
root, run by godog against the binary; verification starts after the
template's squashed commit (`commits.since`); the repository is AGPL-3.0 as a
whole (`tasks/phase-1.yaml`). What remains of v0 is in
`docs/work-items.yaml`: the v0.1.0 release, packed to JavaScript (Node strips
types only outside `node_modules`) and published as a tarball on a tag; CI
verifying with the last released itos as well as the working tree, so a
commit that breaks the gate cannot approve itself; and the config keys that
are validated but not read.

### Phase 2: the Go port

One command group at a time, each shipped when it passes its features and its
part of the corpus, against both implementations:

1. The scaffold: the module's `cmd/` and `internal/`, a snapshot release,
   its own ledger tasks.
2. The config, the globs, the ledger, the check runner, the cost classes and
   written order: `config check`, `task`, `task list`.
3. The messages and scopes: footers, the delegated and built-in header lint,
   `commit`, `verify`.
4. The named tests: the Gherkin and command adapters, merging, recognition,
   the smoke set, the range checks.
5. The plan and its driver: `ci plan`, `ci run`, `ci scope`, `ci range`.
6. The GitLab and Forgejo range providers, the identity and people providers.
7. Work routing: `work`, `work check`.
8. The hooks: `hook commit-msg`, `hook pre-push`, `hooks install`.
9. The v1.0.0 release.

### Phase 3: the switch

The Go binary shadows the TypeScript's `ci plan --json` and `verify` in CI,
any difference a warning, until no difference shows over many pushes and a
nightly; consumers then switch from the pinned TypeScript to the binary; the
TypeScript goes from this repository after two weeks of green nightlies; and
the built-in header lint replaces commitlint once it has agreed with it over a
whole history and a run of pushes. Each step is reverted, never forced, if it
goes wrong.

## 10. Distribution

A release on a `v*` tag: for the Go binary, archives for linux and darwin on
amd64 and arm64 and windows/amd64, each with the binary, `LICENSE` and
`README.md`, plus `checksums.txt` and the config's JSON Schema. The
repository is public, so no download needs a token.

A consumer commits an install script that pins the version and each
platform's SHA-256 (a replaced release cannot pass), installs into an ignored
`.tools/bin/` from its package manager's install step, and points its hook
shims there; CI installs into the runner's path; Go developers can
`go install github.com/donvargax/itos/cmd/itos@<version>`. A version bump is
one `build` commit. Later channels: the aqua or mise registry, a Homebrew tap,
npm (as `palitos`) and PyPI wrappers, signatures.

v0 releases are the TypeScript packed to JavaScript, since Node strips types
only outside `node_modules`, published as a tarball a consumer pins.

## 11. Working rules

The rules for agents are `AGENTS.md` (implementing) and
`docs/ORCHESTRATING.md` (coordinating); the rules a command can check are
`itos.yaml`. Two of this repository's own: a change to what itos does lands
with its scenario in `features/`, and the conformance corpus keeps passing
(a case changes only with the behaviour it records, in the same commit); and
the features' steps never read itos's code, or they stop judging the port.

## 12. Risks and open points

| Risk                                                 | What shows it early, and the answer                                                                                               |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| The two implementations drift during the port.       | A behaviour change lands in TypeScript with its scenario, then in Go; the shadow restarts its count on any difference.            |
| Scope creep into a runner or a hook manager.         | The non-goals are the test; every new command is a task whose `why` names the policy it enforces.                                 |
| Agents depend on today's wording.                    | Wording kept verbatim and compared by the corpus; `--json` is the stable interface.                                               |
| The built-in header lint disagrees with commitlint.  | It only warns until it has matched commitlint on a whole history and a run of pushes.                                             |
| Glob semantics differ in Go.                         | The glob function is ported as written, and the corpus's glob table pins it.                                                      |
| Windows: checks are `sh`.                            | `shell:` in the config; Git for Windows' `sh` preferred.                                                                          |
| A released binary runs in every hook.                | Committed per-platform SHA-256s, a pinned version, `version --check`; signatures later.                                           |
| An adapter is slow or flaky.                         | The hook lists only the kinds a message names; an adapter's failure is an error, never a pass.                                    |
| The features' header-lint scenario needs commitlint. | It runs this checkout's commitlint until the built-in lint lands; then its step uses that instead, the scenario's text unchanged. |
