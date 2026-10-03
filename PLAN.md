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
`docs/PHASES.md`, and the open work is `tasks/work-items.yaml`.

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

| Topic                    | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Implementation           | Go (`cmd/itos`, `internal/`). v0 was TypeScript in `tools/itos/`, run by Node directly; the Go port landed beside it, and the TypeScript left once Go passed everything and this repository ran the Go binary (T-062, the user's call: Go only as soon as possible).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Layout                   | The Go packages are `commits.path_sets.implementation`, and the scope rules read `$implementation` where a template project reads `src/**`. The corpus and the fixtures stay in `tools/itos/`, where the TypeScript was, since the ledger and the corpus name that path.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Named tests              | Gherkin feature files at the root, `features/`, run by godog through `go test`. The steps treat itos as a black box: they run the binary `ITOS_BIN` names in scratch repositories and assert exit codes and output, so the same files judged the TypeScript and the Go port, unchanged, and judge any build.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Selecting scenarios      | godog's tag filter takes exact tags joined by commas; itos's run templates join IDs into one regular expression with `\|`. The harness takes `-scenarios=<regexp>` over the tags and turns it into godog's filter, so the templates stay as they are.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Regression corpus        | The conformance YAML stays as a corpus itos must pass, as both implementations did, run by a CI step of its own, not as named tests.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Where checks start       | `commits.since` names the commit where verification starts: `verify` and every range check skip it and its ancestors; the commit-msg hook is unaffected. A repository made from a template begins with one squashed commit no rule passes, and a project adopting itos has a history written before its rules; both start clean with it. Chosen over skipping the root commit, which covers only the first case.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| Header lint              | `commits.header_lint.use`: `builtin`, itos's own lint of config-conventional's rules (slice 25), or `command`, the delegate `hook` and `stdin` name (commitlint here), which is also what no `use` means. The built-in lint replaces commitlint in v2.0.0 (the user's call, 2026-10-02: Go only, no Node for a consumer); this repository switches once it is held to commitlint's verdicts on its history (T-063), and running both at once (`alongside`) is not a config mode. It rejects as commitlint does, each problem under commitlint's rule id and words, so that a project can swap one for the other. The footer rules are itos's, run beside the header lint whatever it is, never left to it: a delegate carrying them as a plugin of its config skipped them without a word when it lacked the plugin (slice 10).                                                                                                                                                                                                                                                                                                                                                                                                             |
| Coverage                 | None collected: itos is proven through its command line, which a unit test's coverage cannot see, so it was held to no threshold, and went with Vitest when the TypeScript left (T-062). The audit scores this repository's own tooling by complexity alone.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Licence                  | AGPL-3.0 for the whole repository.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| Work registry            | Beside the ledger by default: `work-items.yaml` in the folder `ledger.files` names, `tasks/work-items.yaml` for the default ledger and with no ledger. It is itos's data, as the ledger is, and read every day, so `docs/` is left for prose, and a project that keeps its ledger in another folder finds the registry there too; `work.registry` puts it anywhere else. The default is the one table's, laid over with the config's ledger, so every command that reads the registry and `config check --print-defaults` agree (slice 22). With none where itos looks, the commands that read it say so, naming the path, so a project with its registry at the old default (`docs/work-items.yaml`) learns where itos reads it now.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Data at commit           | `itos hook commit-msg` validates itos's own data (`itos.yaml`, the ledger, the registry, the smoke set) when a commit stages any of it, with `config check`'s problems read from the staged tree: a project's pre-commit hook passes these files as prose, a rule a command can decide belongs in itos, and the working tree can hold what the commit does not. Chosen over a check in each project's pre-commit hook, which read the working tree (T-022).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Task checks at commit    | `itos hook commit-msg` runs, last, the checks of each task a `Task:` footer names, as staged, in written order up to the first late one (CI's cost rule, not a second one), each capped by `hooks.commit_msg.check_timeout`. A failure rejects the commit when the task's work item is `done`, and is printed with the task's status otherwise: a finished task that fails is a regression, while one in progress is committed in steps and CI judges the push. Last, because it is the slowest rule and reads a footer the header lint has judged. The checks run in the working tree, never in a scratch checkout: work here is one session at a time and parallel work has worktrees of its own, so what a check sees beside the commit is the committer's own.                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| itos in command patterns | Every pattern the config matches a check's or step's command with (`ci.cost.static`, a `ci.covers` rule's `matches`, a `ci.nightly_only` entry, a `tests.<kind>.recognize` template) reads a command whose first word is `hooks.bin` (`tools/bin/itos` by default) as starting with `itos`, so a pattern names itos once, however the project calls it; a command under any other path is read as written. Each pattern is tried on both readings, so a pattern that names the path keeps matching and the reading only ever adds a match: a command made static, a check covered, left to the nightly or read as a selection, never the reverse. It is for matching alone, in one place (`readings` in `config.ts`), so CI's plan, the commit-msg hook and `config check`'s written-order rule agree.                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| One run per check        | One `itos task` invocation runs each distinct check once: the same command (whitespace collapsed, as the config's patterns read it) with the same timeout. The first task that lists it runs it, and each later one reads that exit status by its own `run:` or `fails:`; an `after: push` check still waits for the push, and nothing is kept between invocations. Tasks share checks that take minutes (the self-tests, `config check`), so `--pending` and `--group` ran them once per task. Accepted limit: a check whose answer a check above it in its own task would change reads the earlier run; no ledger pair needs that today. `task list` prints each task's status from the work registry, or `no item`, and runs nothing: chosen over running only the static checks, so listing is instant and says what the people working have recorded, while what the checks find stays `itos task`'s. A push's CI plan and the commit-msg hook keep their own runs; the nightly's tasks step shares them.                                                                                                                                                                                                                              |
| Done tasks nightly       | `ci.nightly.steps` takes `{ tasks: done }`, with an optional `cost: static`: the checks of every task whose work item is `done` run every night, where the step is written, in cost order, each shared check once as in `itos task`; a red one fails the nightly, naming the task's ID and title. A task's checks otherwise run in CI only when a push names it, so a change elsewhere broke done tasks unseen (T-024 here, a consumer's self-test). "Done" is the registry's status, not every status `ci.wait_on_status` lets through: a task in progress may be red until it lands, and `done` is what the commit-msg hook calls a red check a regression by. Refused in `ci.steps`, whose tasks are the ones the commits name.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| The port's proof         | Each command group of the port is a task in `tasks/phase-2.yaml` whose checks run its part of the conformance corpus, and its scenarios with `ITOS_BIN` pointed at the Go build, against that build; its code lands as `refactor` commits with a `Task:` footer, since the behaviour is the TypeScript's, already specified, and a `Scenarios:` footer would run the scenarios against the TypeScript alone. Once a group lands, its part joins the ported set, which `tools/selftest/go-port.ts` runs against the Go build in every push's CI, so a later change to a ported group lands in both implementations in one push. Since T-053, with every group ported, that set is the whole corpus and every feature, with no list to forget a later file. The user's calls (2026-10-02), chosen over a nightly-only guard, which reports the drift a day late, and over `feat` with a `Task:` footer, which changes the rule for every feat. With the TypeScript gone (T-062) there is nothing to drift from: CI's corpus step and its one features run judge `tools/bin/itos`, the tree built on demand, so a push runs the smoke set and what it names again and the nightly every scenario, and `go-port.ts` is the port's tasks' check. |
| The Go version           | package.json's, as it was the TypeScript's: `tools/bin/build-go.ts` and `tools/bin/itos` stamp it into the binary at build, so a release still changes package.json alone and the corpus's `{{version}}` holds; a binary built without the stamp says the module version Go records, which `go install …@v<x>` sets.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| The first Go release     | v1.0.0 publishes the Go archives, checksums.txt and the config's JSON Schema (generated from the Go config's table, never kept by hand) beside the TypeScript tarball, which stays until phase 3 switches the consumers; the release job proves the very archives it uploads. The user's calls (2026-10-02).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| Pattern dialect          | The config's regular expressions (`ledger.id`, `ledger.group.pattern`, `tests.<kind>.id`, `ci.cost.static`, `ci.covers[].matches`) are RE2, Go's: the Go binary is what remains after the switch, and RE2 matches in linear time, so no pattern in a project's config can hang a commit hook. Until the TypeScript goes, it refuses what RE2 cannot compile (lookarounds, backreferences, the escapes and classes RE2 lacks, repeat counts above 1000), so a config that passes one implementation passes the other, and config check's fix names RE2. The user's call (2026-10-02), over JavaScript's dialect, which Go could read only through a second, backtracking dependency (slice 24).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Releases                 | Automated, the user's call (2026-10-02): no release waits for the user; the coordinator tags when CI and the release checks are green, after reading the Upgrading section, or has a reviewer agent read it against the commits where judgment is needed. Toward no review at all: an `Upgrading:` footer gathered into the notes (p1-upgrading-footer), and the release cut by CI itself (p1-itos-release, a follow-up).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Distribution             | Release archives plus checksums, installed pinned; `go install` for Go developers. v0 releases are the TypeScript, packed to JavaScript.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

## 4. The model

Each concept is defined once; the bold names are used in the schema and the
command line.

| Concept                  | Definition                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Ledger**               | The YAML files listing the **tasks** (`tasks/phase-*.yaml`).                                                                                                                                                                                                                                                                                                         |
| **Group**                | A set of tasks, from the ledger file's name (`phase-3.yaml` → `3`); the config names the label (`phase`), by which the tools call a group and `itos task` takes one (`--phase`, beside `--group`).                                                                                                                                                                   |
| **Task**                 | `id`, `type` (a commit type), `title`, `why`, `done_when` (a list of **checks**). **Status**: `done` (every check passes), `failing` (one fails), `pending` (none fails, one waits for the push), `review` (no checks).                                                                                                                                              |
| **Check**                | One command in `done_when`. **Mode** `run` (exit 0) or `fails` (non-zero). Optional `after: push`, `timeout` (seconds, default 600), `prose: true` (prose alone can change its result), `cost`. It does real work, never a grep over config.                                                                                                                         |
| **Cost class**           | `static` (needs nothing built, takes seconds) or `late`. A check's or step's own `cost:` wins; else a config pattern over the command (one starting with `hooks.bin` read as starting with `itos`); else `late`. A task's checks keep their written order: a check below a late one is late too. A CI run goes static steps, static checks, late steps, late checks. |
| **Footer**               | A `Key: id id, …` commit line naming IDs of one **source**: the ledger (`Task:`) or one kind of named test (`Scenarios:`).                                                                                                                                                                                                                                           |
| **Kind** (of named test) | Tests with stable IDs a footer can name and CI can select. Has an **adapter**, **run templates**, an optional **smoke set** and optional **range checks**.                                                                                                                                                                                                           |
| **Adapter**              | Lists a kind's tests at a tree (the working tree, the index, a commit): `id`, `file`, `live`. Built in (`gherkin`) or a command speaking JSON.                                                                                                                                                                                                                       |
| **Selection**            | What one run of a kind selects: `whole`, `ids`, or a `pattern` from a task check. A run's selections merge into one command.                                                                                                                                                                                                                                         |
| **Smoke set**            | Per-kind IDs every push runs, each with a `why`. Every file with a live test lists one; each listed ID is live in its file; more than one needs `more`.                                                                                                                                                                                                              |
| **Range check**          | A kind's rule on how tests may change between two trees: commands (staged in the hook, over the range in `verify`), or the built-in moves rule of a Gherkin kind (`builtin: moves`, with `except_types` and `allowed_renames`), judged on the index in the hook and on each commit against its parent in `verify`.                                                   |
| **Commit rules**         | **Header lint** (Conventional Commits, delegated or built in), **footer rules** (required per type, IDs exist and are live at the commit), **scope rules** (`only`, `never`, `must_touch` per type).                                                                                                                                                                 |
| **Range**                | The commits a CI run judges, `from..to`, less `commits.since` and its ancestors. An empty or unreadable `from` runs everything; a zero SHA is a new branch.                                                                                                                                                                                                          |
| **Range provider**       | Finds `from`: the last green CI run's head on the main branch if it is an ancestor of `to`, else empty. A pull request's base wins.                                                                                                                                                                                                                                  |
| **CI step**              | A command every full run executes, in order; one step may be a kind's merged run. A nightly step may instead be `{ tasks: done }`: the checks of every task whose work item is `done`, each shared check once, only the static ones with `cost: static`.                                                                                                             |
| **Coverage**             | A named task's check is not rerun when a step did it (equal to a step, or a `covers` rule); a run of a kind is merged into the kind's run, or runs as itself when the plan has none; a nightly-only check waits for the nightly.                                                                                                                                     |
| **Prose**                | Paths that can only break their own formatting. A prose-only range runs the prose steps and the named tasks' static and `prose: true` checks, and lists the rest as left out.                                                                                                                                                                                        |
| **Plan**                 | What a CI run executes and in what order, computed without side effects from the range, the config, the ledger and the registry. `ci plan --json` prints it.                                                                                                                                                                                                         |
| **Work registry**        | `work-items.yaml` beside the ledger, `tasks/work-items.yaml` by default (`work.registry`'s default): group owners and **work items** (owner, status, dependencies, kind). A named task whose item is `todo` waits in CI.                                                                                                                                             |
| **People source**        | Who may own work: the All Contributors table in `CONTRIBUTORS.md`, `.all-contributorsrc`, or a YAML list.                                                                                                                                                                                                                                                            |
| **Identity provider**    | Who a session works for: `--as`, else the provider (`gh api user`, a command, none).                                                                                                                                                                                                                                                                                 |

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
  whitespace collapsed (RE2 in Go: none uses what RE2 lacks). Each of them
  (`ci.cost.static`, `ci.covers`' `matches`), each `ci.nightly_only` entry and
  each `recognize` template is tried on the command as written and with its
  first word read as `itos` when that word is `hooks.bin`: `^itos work check`
  matches `tools/bin/itos work check`.
- **Templates** use `{name}`; a value going into a shell is single-quoted.
- **`shell`** is the argv prefix for every command, `[sh, -c]` by default.
- **`commits.since`**: the full SHA of the commit where verification starts.
- **The defaults are one table**, which the loader lays the file over and
  `config check --print-defaults` prints, so a default cannot be applied
  without being printed or printed without being applied; `tests.<kind>`'s
  apply to each kind. A key with no default stays absent. The Go port's
  loader keeps the table and the flag as one.
- **Every key it accepts is one a tool reads**: a key whose feature is not
  built yet is not accepted until it is (`features/config.feature`).

The sections: `ledger` (its files, the group in their names, the ID pattern,
the check timeout), `commits` (types, the header lint, footers, path sets,
scopes, `since`), `tests` (one entry per kind: adapter, root, ID pattern,
run and recognize templates, smoke set and whether every file needs a smoke
test, range checks), `ci` (steps, prose, cost patterns, covers, nightly, the
range provider), `work` (registry, its statuses, the key its owners per
group are under, people, identity) and `hooks` (the manager, over the one
detected, the binary the shims call, the
commit-msg hook's task checks and their timeout, the
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
`commits.types` (config-conventional's own list without it), lower case,
never empty; a subject, not sentence-, start-, pascal- or upper-case,
without a full stop; the header at most 100 characters and trimmed; body and
footer lines at most 100; a blank line before the body and the footers
(warnings, printed and never rejecting). `commits.header_lint.use: builtin`
lints it in itos; `use: command`, or no `use`, delegates it (commitlint,
`cog verify`): `commits.header_lint.hook` lints the message file and
`stdin` a message on stdin. **The footer rules** are itos's, whatever the
header lint: they run after it, or alone without one, and both report
before the exit, so a header problem does not hide a footer one; under
`--json` their problems are one list. `alongside: builtin`, the built-in
lint beside the delegate, warning only, stays unaccepted: the two are held
to each other over a history (T-063), not in a project's commits.

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
patterns are deduplicated in order and combined by `join`. An `ids` selection
with no IDs adds no pattern, since an empty alternation matches every test; with
nothing selected, no command runs. In `recognize`, a
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

| Command                                                                 | What it does                                                                                                                                                                                |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `task <id>…`, `--phase <g>`, `--pending`, `task list`                   | Runs the tasks' checks in written order, each distinct check once, or lists the tasks with their work items' status, running nothing; a status table.                                       |
| `work [--as <h>]`, `work check [<file>]`                                | Who the session works for and what they can start; validates the registry.                                                                                                                  |
| `commit check-message <file\|->`, `commit check-paths --type <t> <p>…`  | The header lint and the footer rules on one message; the scope rules alone, to plan a split.                                                                                                |
| `verify <from> <to>`                                                    | Re-checks every non-merge commit of the range after `commits.since` (message with footers at that commit, paths, the built-in moves rule against its parent), then each range command once. |
| `tests list <kind> [--at <tree>]`, `tests smoke check\|ids\|run <kind>` | The adapter's listing; the smoke rule, the smoke IDs, the smoke run.                                                                                                                        |
| `tests moves <kind>`                                                    | The staged feature files against HEAD's by the built-in moves rule, by hand.                                                                                                                |
| `ci plan <from> <to>`, `ci run [<from> <to>]`, `--nightly`              | Prints the plan; runs it, stopping at the first failure unless `ci.stop_at_first_failure` is false.                                                                                         |
| `ci scope <from> <to>`, `ci range --head <sha> [--base <sha>]`          | Whether a range is prose only; where a push's range starts.                                                                                                                                 |
| `hook commit-msg <file>`, `hook pre-push <remote> <url>`                | The hooks' entry points.                                                                                                                                                                    |
| `hooks install [--manager <m>] [--print] [--force]`                     | Writes the one-line shims for the hook manager it detects, or prints its snippet.                                                                                                           |
| `config check [--print-defaults]`                                       | Validates the config, the ledger, the registry and the smoke sets.                                                                                                                          |
| `version [--check]`                                                     | `--check` exits 1 if the binary does not satisfy `requires`.                                                                                                                                |

**Exit codes:** 0 success; 1 a policy failure (a check failed, a commit
rejected, an unknown task); 2 a usage or config error, a file or folder the
config names that is missing or unreadable among them (the ledger's folder a
footer reads, a smoke set); 3 a missing environment. In `ci run` a failing step exits with its own code. A missing
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
  tools/itos/conformance/   the regression corpus and its runner
  tools/itos/fixtures/      configs and ledgers the ledger's checks hand itos
  tools/bin/itos   the entry point the hooks and the scripts call: the Go binary
  features/        the named tests: Gherkin, godog steps, the smoke set
  cmd/itos/        the Go binary (v0 was TypeScript, in tools/itos/, until T-062)
  internal/        its packages: config, glob, git, shell, ledger, check,
                   message, scope, tests, plan, ci, providers, work, hook, out,
                   and value and source (YAML as JavaScript reads it; where
                   itos reads its data)
  tasks/           the ledger
  itos.yaml        this repository's own policy
```

The Go port shells out to git as the TypeScript did, has one dependency
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
`tasks/work-items.yaml`: the v0.1.0 release, packed to JavaScript (Node strips
types only outside `node_modules`) and published as a tarball on a tag; CI
verifying with the last released itos as well as the working tree, so a
commit that breaks the gate cannot approve itself; and the config keys that
are validated but not read.

### Phase 2: the Go port

One command group at a time, each shipped when it passes its features and its
part of the corpus, against both implementations:

1. The scaffold: the module's `cmd/` and `internal/`, answering the
   command line's own corpus (`cli.yaml`) and held to the ported set on every
   push (T-039); a snapshot release (T-040); the `--help` texts (T-041).
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

The user's calls (2026-10-02): Go only, as soon as possible, with no shadow
period and no two-week wait, since every push already runs the whole corpus
and every feature against the Go build (T-053) and itos has few users, who
report problems as issues that are then prioritised.

1. The hooks run the Go unit tests a change reaches (T-059).
2. Dogfood: this repository's hooks and CI run the Go binary, so it judges
   every commit here before any consumer's (T-060).
3. The TypeScript leaves the repository, with no release (T-062): every
   feature ran against both implementations on every push, so anything
   specified while it stayed was built twice. Done: the corpus and the
   features run against the Go binary, Vitest went with the TypeScript's
   unit tests, and the release stopped packing the tarball.
4. The built-in header lint replaces commitlint, so a consumer of the Go
   binary needs no Node (slice 25), built once, in Go; it is held to
   commitlint's verdicts on this repository's history before this
   repository switches to it (T-063). Built: `use: builtin` lints the header
   in the commit-msg hook, `commit check-message` and `verify`, and this
   repository still delegates to commitlint until T-063.
5. v2.0.0 (T-061): Go only, the tarball gone; consumers switch to the binary
   on v2's Upgrading section, which asks for every compatibility change at
   once.

Each step is reverted, never forced, if it goes wrong. After v2, features are
built once, in Go: extensions (`itos-<cmd>` on `PATH`), the stealth mode and
the GitHub modes.

## 10. Distribution

A release on a `v*` tag: for the Go binary, archives for linux and darwin on
amd64 and arm64 and windows/amd64, each with the binary, `LICENSE` and
`README.md` at its top level, named `itos-<version>-<os>-<arch>.tar.gz`
(`.zip` for windows), plus the config's JSON Schema, `itos.schema.json`,
which an editor checks an `itos.yaml` against, and `checksums.txt`, one file
holding every asset's SHA-256 (`sha256sum --ignore-missing -c checksums.txt`
checks whichever were downloaded). Until v2.0.0 the releases carried the
TypeScript tarball, `itos-<version>.tgz`, beside them, listed in the same
`checksums.txt`; it left with the TypeScript (T-062). The release job proves the very folder it
uploads before publishing anything. The repository is public, so no download
needs a token.

A consumer commits an install script that pins the version and each
platform's SHA-256 (a replaced release cannot pass), installs into an ignored
`.tools/bin/` from its package manager's install step, and points its hook
shims there; CI installs into the runner's path; Go developers can
`go install github.com/donvargax/itos/cmd/itos@<version>`. A version bump is
one `build` commit. Later channels: the aqua or mise registry, a Homebrew tap,
npm (as `palitos`) and PyPI wrappers, signatures.

v0 releases are the TypeScript packed to JavaScript, since Node strips types
only outside `node_modules`, published as a tarball a consumer pins.

Each release's notes are a committed file, `docs/releases/v<version>.md`, which the release
workflow publishes and without which it refuses the tag. They end with an "Upgrading" section a
consumer's session updates from alone, so it is complete: each config key added, removed, renamed
or with a changed default; each workaround a consumer can now drop; each behaviour that can reject
a commit that passed before; and the pin line to change.

## 11. Working rules

The rules for agents are `AGENTS.md` (implementing) and
`docs/ORCHESTRATING.md` (coordinating); the rules a command can check are
`itos.yaml`. Two of this repository's own: a change to what itos does lands
with its scenario in `features/`, and the conformance corpus keeps passing
(a case changes only with the behaviour it records, in the same commit); and
the features' steps never read itos's code, or they stop judging the port.

## 12. Risks and open points

| Risk                                                 | What shows it early, and the answer                                                                                                                                             |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The two implementations drift during the port.       | Every push held the Go build to the whole corpus and every feature (T-053) until the TypeScript left (T-062).                                                                   |
| Scope creep into a runner or a hook manager.         | The non-goals are the test; every new command is a task whose `why` names the policy it enforces.                                                                               |
| Agents depend on today's wording.                    | Wording kept verbatim and compared by the corpus; `--json` is the stable interface.                                                                                             |
| The built-in header lint disagrees with commitlint.  | It is held to commitlint's verdicts on this repository's whole history before this repository switches (T-063); a project keeps its delegate (`use: command`) until it chooses. |
| Glob semantics differ in Go.                         | The glob function is ported as written, and the corpus's glob table pins it.                                                                                                    |
| Windows: checks are `sh`.                            | `shell:` in the config; Git for Windows' `sh` preferred.                                                                                                                        |
| A released binary runs in every hook.                | Committed per-platform SHA-256s, a pinned version, `version --check`; signatures later.                                                                                         |
| An adapter is slow or flaky.                         | The hook lists only the kinds a message names; an adapter's failure is an error, never a pass.                                                                                  |
| The features' header-lint scenario needs commitlint. | It runs this checkout's commitlint until commitlint leaves (T-063); then its step uses the built-in lint, which has landed (slice 25), the scenario's text unchanged.           |
