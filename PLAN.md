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

| Topic                    | Decision                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Implementation           | Go (`cmd/itos`, `internal/`). v0 was TypeScript in `tools/itos/`, run by Node directly; the Go port landed beside it, and the TypeScript left once Go passed everything and this repository ran the Go binary (T-062, the user's call: Go only as soon as possible).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Layout                   | The Go packages are `commits.path_sets.implementation`, and the scope rules read `$implementation` where a template project reads `src/**`. The corpus and the fixtures stay in `tools/itos/`, where the TypeScript was, since the ledger and the corpus name that path.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Named tests              | Gherkin feature files at the root, `features/`, run by godog through `go test`. The steps treat itos as a black box: they run the binary `ITOS_BIN` names in scratch repositories and assert exit codes and output, so the same files judged the TypeScript and the Go port, unchanged, and judge any build.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Selecting scenarios      | godog's tag filter takes exact tags joined by commas; itos's run templates join IDs into one regular expression with `\|`. The harness takes `-scenarios=<regexp>` over the tags and turns it into godog's filter, so the templates stay as they are.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Regression corpus        | The conformance YAML stays as a corpus itos must pass, as both implementations did, run by a CI step of its own, not as named tests.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Where checks start       | `commits.since` names the commit where verification starts: `verify` and every range check skip it and its ancestors; the commit-msg hook is unaffected. A repository made from a template begins with one squashed commit no rule passes, and a project adopting itos has a history written before its rules; both start clean with it. Chosen over skipping the root commit, which covers only the first case. A footer's own `since` does the same for one rule: verify leaves that commit and its ancestors out of the footer's `required_for`, so a footer required later does not fail the history before it (T-062 met that trap); the commit-msg hook always requires it (slice 26).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Header lint              | `commits.header_lint.use`: `builtin`, itos's own lint of config-conventional's rules (slice 25), or `command`, the delegate `hook` and `stdin` name (commitlint, say), which is also what no `use` means. The built-in lint replaces commitlint in v2.0.0 (the user's call, 2026-10-02: Go only, no Node for a consumer); this repository switched to it once it was held to commitlint's verdicts on its history (T-063), and running both at once (`alongside`) is not a config mode. It rejects as commitlint does, each problem under commitlint's rule id and words, so that a project can swap one for the other. The footer rules are itos's, run beside the header lint whatever it is, never left to it: a delegate carrying them as a plugin of its config skipped them without a word when it lacked the plugin (slice 10).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Coverage                 | None collected: itos is proven through its command line, which a unit test's coverage cannot see, so it was held to no threshold, and went with Vitest when the TypeScript left (T-062). The audit scores this repository's own tooling by complexity alone.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Licence                  | AGPL-3.0 for the whole repository.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Work registry            | Beside the ledger by default: `work-items.yaml` in the folder `ledger.files` names, `tasks/work-items.yaml` for the default ledger and with no ledger. It is itos's data, as the ledger is, and read every day, so `docs/` is left for prose, and a project that keeps its ledger in another folder finds the registry there too; `work.registry` puts it anywhere else. The default is the one table's, laid over with the config's ledger, so every command that reads the registry and `config check --print-defaults` agree (slice 22). With none where itos looks, the commands that read it say so, naming the path, so a project with its registry at the old default (`docs/work-items.yaml`) learns where itos reads it now.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Data at commit           | `itos hook commit-msg` validates itos's own data (`itos.yaml`, the ledger, the registry, the smoke set) when a commit stages any of it, with `config check`'s problems read from the staged tree: a project's pre-commit hook passes these files as prose, a rule a command can decide belongs in itos, and the working tree can hold what the commit does not. Chosen over a check in each project's pre-commit hook, which read the working tree (T-022).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Task checks at commit    | `itos hook commit-msg` runs, last, the checks of each task a `Task:` footer names, as staged, in written order up to the first late one (CI's cost rule, not a second one), each capped by `hooks.commit_msg.check_timeout`. A failure rejects the commit when the task's work item is `done`, and is printed with the task's status otherwise: a finished task that fails is a regression, while one in progress is committed in steps and CI judges the push. Last, because it is the slowest rule and reads a footer the header lint has judged. The checks run in the working tree, never in a scratch checkout: work here is one session at a time and parallel work has worktrees of its own, so what a check sees beside the commit is the committer's own.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| itos in command patterns | Every pattern the config matches a check's or step's command with (`ci.cost.static`, a `ci.covers` rule's `matches`, a `ci.nightly_only` entry, a `tests.<kind>.recognize` template) reads a command whose first word is `hooks.bin` (`tools/bin/itos` by default) as starting with `itos`, so a pattern names itos once, however the project calls it; a command under any other path is read as written. Each pattern is tried on both readings, so a pattern that names the path keeps matching and the reading only ever adds a match: a command made static, a check covered, left to the nightly or read as a selection, never the reverse. It is for matching alone, in one place (`readings` in `config.ts`), so CI's plan, the commit-msg hook and `config check`'s written-order rule agree.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| One run per check        | One `itos task` invocation runs each distinct check once: the same command (whitespace collapsed, as the config's patterns read it) with the same timeout. The first task that lists it runs it, and each later one reads that exit status by its own `run:` or `fails:`; an `after: push` check still waits for the push, and nothing is kept between invocations. Tasks share checks that take minutes (the self-tests, `config check`), so `--pending` and `--group` ran them once per task. Accepted limit: a check whose answer a check above it in its own task would change reads the earlier run; no ledger pair needs that today. `task list` prints each task's status from the work registry, or `no item`, and runs nothing: chosen over running only the static checks, so listing is instant and says what the people working have recorded, while what the checks find stays `itos task`'s. A push's CI plan and the commit-msg hook keep their own runs; the nightly's tasks step shares them.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| Done tasks nightly       | `ci.nightly.steps` takes `{ tasks: done }`, with an optional `cost: static`: the checks of every task whose work item is `done` run every night, where the step is written, in cost order, each shared check once as in `itos task`; a red one fails the nightly, naming the task's ID and title. A task's checks otherwise run in CI only when a push names it, so a change elsewhere broke done tasks unseen (T-024 here, a consumer's self-test). "Done" is the registry's status, not every status `ci.wait_on_status` lets through: a task in progress may be red until it lands, and `done` is what the commit-msg hook calls a red check a regression by. Refused in `ci.steps`, whose tasks are the ones the commits name.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| The port's proof         | Each command group of the port is a task in `tasks/phase-2.yaml` whose checks run its part of the conformance corpus, and its scenarios with `ITOS_BIN` pointed at the Go build, against that build; its code lands as `refactor` commits with a `Task:` footer, since the behaviour is the TypeScript's, already specified, and a `Scenarios:` footer would run the scenarios against the TypeScript alone. Once a group lands, its part joins the ported set, which `tools/selftest/go-port.ts` runs against the Go build in every push's CI, so a later change to a ported group lands in both implementations in one push. Since T-053, with every group ported, that set is the whole corpus and every feature, with no list to forget a later file. The user's calls (2026-10-02), chosen over a nightly-only guard, which reports the drift a day late, and over `feat` with a `Task:` footer, which changes the rule for every feat. With the TypeScript gone (T-062) there is nothing to drift from: CI's corpus step and its one features run judge `tools/bin/itos`, the tree built on demand, so a push runs the smoke set and what it names again and the nightly every scenario, and `go-port.ts` is the port's tasks' check.                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| The Go version           | The release's tag (T-069): GoReleaser stamps it into a release's binary, and `tools/bin/itos` and `tools/bin/build-go.ts` stamp a build of a checkout with what `tools/bin/dev-version` reads from `git describe` (the release's version at its tag, `X.Y.(Z+1)-dev.N.g<sha>` after it, a pre-release with no `+`, so it sorts between the releases and a pin reads it), so nothing in the tree holds a version and the corpus's `{{version}}` is what the binary says; a binary built without the stamp says the module version Go records, which `go install …@v<x>` sets. Until v2.3.0 it was package.json's, bumped by hand.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| The first Go release     | v1.0.0 publishes the Go archives, checksums.txt and the config's JSON Schema (generated from the Go config's table, never kept by hand) beside the TypeScript tarball, which stays until phase 3 switches the consumers; the release job proves the very archives it uploads. The user's calls (2026-10-02).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Pattern dialect          | The config's regular expressions (`ledger.id`, `ledger.group.pattern`, `tests.<kind>.id`, `ci.cost.static`, `ci.covers[].matches`) are RE2, Go's: the Go binary is what remains after the switch, and RE2 matches in linear time, so no pattern in a project's config can hang a commit hook. Until the TypeScript goes, it refuses what RE2 cannot compile (lookarounds, backreferences, the escapes and classes RE2 lacks, repeat counts above 1000), so a config that passes one implementation passes the other, and config check's fix names RE2. The user's call (2026-10-02), over JavaScript's dialect, which Go could read only through a second, backtracking dependency (slice 24).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| Releases                 | Cut by CI, with no review (T-069, the user's call, 2026-10-03): a push to `main` whose CI is green and whose commits since the last tag carry a `feat`, a `fix` or a breaking change cuts the release, its version computed from those commits (`tools/bin/release-version`: a breaking change a major, a feat a minor, a fix a patch), tagged, published with GoReleaser and attested, its notes generated from the commits (`tools/bin/release-notes`, every `Upgrading:` footer, slice 26 and T-061), nothing committed by the releaser. Before it (2026-10-02) the coordinator tagged by hand after reading the Upgrading section. What a release rests on is the push's CI, with the schema contract (T-070, `tools/bin/schema-contract`): a config the last release accepts is still accepted, with the same defaults, unless a commit since that release carries `BREAKING-CHANGE` or a `!`, so a breaking config change cannot ship as a minor; and the last release's scenarios and corpus run against the new binary (T-071, `tools/bin/previous-release`): an old one that fails is accepted only with a breaking change, or when a fix names it in a `Changes:` footer, so a fix whose old scenario held the bug stays a patch and a feat may never break one. The old corpus's help cases (an argv asking for `--help`, `-h` or `help <command>`, or a bare itos) are left out of that run, the user's call (2026-10-03): help text is documentation, not compatibility, `--json` and exit codes being the stable interface, and this tree's own corpus still pins its help exactly; judged, every feat that adds a flag or a command would pass only as a breaking change. |
| Distribution             | Release archives plus checksums, installed pinned; `go install` for Go developers. v0 releases are the TypeScript, packed to JavaScript.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Go dependencies          | A Go module is trusted only once it has been public for 7 days, as pnpm's release age holds npm packages (the user's call, 2026-10-03: supply-chain attacks are common and easy). Go has no such setting, but `go get` runs no dependency code, so a check between it and the commit is safe: `tools/bin/deps-check` (T-067) refuses a younger module unless `deps-check.json` excepts that version with a reason (an urgent security fix is exactly a young version), then runs govulncheck, itself a pinned tool dependency held to the same age; at pre-commit when `go.mod` or `go.sum` is staged, and in CI on a range that changes them, never otherwise, as it needs the network. A script in this repository, in Go and on the standard library alone, so it can move unchanged into an `itos-security` extension once the user's repository template is ready (`p3-extension-pins`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |

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
| **Footer**               | A `Key: id id, …` commit line naming IDs of one **source**: the ledger (`Task:`) or one kind of named test (`Scenarios:`); or free text (`source: text`), what a consumer must do or `none`. Its own `since` leaves older commits out of its `required_for` in verify.                                                                                               |
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
| **Identity provider**    | Who a session works for: `--as`, else the provider (`gh api user`, a command, none); under a stealth config, with no `--as`, nobody is asked and the session owns every item.                                                                                                                                                                                        |

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
error that names the key it misspells.

**From a subfolder** (slice 40): with no `--config`, no `ITOS_CONFIG` and no
`--root`, and no `itos.yaml` in the folder it is run in, a run inside a git
repository looks for `itos.yaml` at the repository's top level
(`git rev-parse --show-toplevel`) and, finding it there or the stealth config
(below), runs as if started at the top, as `--root <top>` would, so every
path the config names means what it means there; the launcher finds the pin
the same way. A folder's own `itos.yaml` still wins, and outside a repository
nothing changes. A path the person types (`commit check-paths`' paths, a
message or registry file, `config check --ledger`, `tests smoke check
--features`, `itos commit`'s pathspecs and `-F` file, which git is run in
that folder for) is read from the folder they stood in, as git reads one,
and named from the top; under `--root` it is the root's, as before. An
extension runs at the top, `ITOS_ROOT` naming it, as under `--root`.

**The stealth mode** (slice 30, the user's call): one person's itos in a
repository whose team does not use it, with nothing of it in the tree. With no
`--config`, no `ITOS_CONFIG` and no `itos.yaml` in the root, itos reads
`<git common dir>/itos/itos.yaml` (`git rev-parse --git-common-dir`), with no
variable to set, so every linked worktree shares it; a project's own
`itos.yaml` in the root always wins, as the project's mode. The files that
config names for itos's own data, the ledger, the work registry and the smoke
sets, resolve beside it, in that folder, which git never commits; the
project's paths (a kind's tests, the globs, the commands) stay the root's. Its
defaults fit one person, whatever it says (slice 35, the user's calls of
2026-10-03): `hooks.bin` is `itos`, the global launcher, so its hooks call
that, and no people file is read, the person being the only one, so no owner
is checked and any session is listed. For the same reason `itos work` with no
`--as` looks nobody up, neither gh nor a `work.identity` command, and treats
every item as the session's, whatever owner it names (slice 38, the user's call
of 2026-10-03): owners stay in the registry as written and stop deciding what
is proposed, and `--as` still proposes a handle's items by owner, as in a
project. A project's `hooks.bin` default stays
`tools/bin/itos` through v2 and flips to `itos` in v3 (`v3-hooks-bin-default`); the key itself
stays, internal and unsupported, for a repository that must run its own build of itos, as this
one does (the user's call, 2026-10-03). A config is the stealth one by where it is, so an extension's
call back reads the same files; one anywhere else that `--config` or
`ITOS_CONFIG` names resolves its paths from the root, as it always has. Since
no commit carries that ledger, a footer naming a task is checked against its
file at every commit, `read_at: commit` falling back to the working file, and
a git tree read (the index, a commit) reads any path in the git folder from
the file. Where it pins nothing, a global itos runs the newest release for it,
as where there is no config. A footer in the message is what would show to
everyone, so under a stealth config a commit's footers live in a git note on
it, in `refs/notes/itos`, which `git push` does not send unless asked (slice
32, the user's calls of 2026-10-03): `itos commit` writes the note from its
flags and hands the same lines to the commit-msg hook in `ITOS_FOOTERS`; the
hook judges them as typed ones, refuses a commit that needs a footer made
without `itos commit` and a link typed into the message, each naming the
`itos commit` flag, and takes an amend on HEAD's note, knowing it from
`itos commit`, which says in `ITOS_AMEND` whether `--amend` is among the
arguments it hands git (slice 37), and guessing it only for a commit made
without it, from an author HEAD's to the second; itos adds `refs/notes/itos` to `notes.rewriteRef` in the local
config, so an amend or a rebase carries the note; verify and ci plan's named
tasks read each commit's note. Only the links live there: a footer of free
text is content, and stays in the message (slice 36, §7). A project's config keeps
its footers in the message. Slice 33 declares the hooks in the git config,
and slice 34 has verify and ci plan given no range judge the person's
unpushed commits (§7, `features/stealth.feature`). This repository's own `itos.yaml` is
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
- **`commits.since`**: the full SHA of the commit where verification starts;
  `commits.footers.<name>.since`, the one after which a footer is required.
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

Global flags: `--config`, `--root`, `--json`, `-q`, written anywhere for a
built-in command and before the name for an extension. With neither
`--config` nor `--root`, a run from a subfolder of a repository finds its
config at the top and runs from there (§5).

| Command                                                                                                 | What it does                                                                                                                                                                                                       |
| ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `task <id>…`, `--phase <g>`, `--pending`, `task list`                                                   | Runs the tasks' checks in written order, each distinct check once, or lists the tasks with their work items' status, running nothing; a status table.                                                              |
| `work [--as <h>]`, `work list`, `work check [<file>]`                                                   | Who the session works for and what they can start; every item of the registry with its title, done ones too, judging nothing; validates the registry.                                                              |
| `commit [--task <id>] [--scenarios <ids>] [--<footer> <text>] [--breaking <text>] [<git commit args>…]` | `git commit` with the footers itos writes from its flags, as git's `--trailer`, a commit missing a required one refused before git runs; the hooks judge them as typed ones, and git's exit code is itos's.        |
| `commit check-message <file\|->`, `commit check-paths --type <t> <p>…`                                  | The header lint and the footer rules on one message; the scope rules alone, to plan a split.                                                                                                                       |
| `commit footers <name> <from> <to>`                                                                     | A range's free-text footers with their commits, leaving out `none`, for a release's notes.                                                                                                                         |
| `push`                                                                                                  | Pulls the upstream with a rebase whatever git's settings say, then pushes HEAD to it in a separate step; refuses uncommitted changes, stops with a stopped rebase, never forces.                                   |
| `git-shim install\|uninstall [--dir <folder>]`, `git-shim run <commit\|push> [<git args>…]`             | Links itos as `git` (or removes the link) and says where the folder stands on the `PATH`; what the link runs in a repository itos manages (below).                                                                 |
| `pin [<version>]`                                                                                       | Moves the config's pin (`pin.version`, `pin.checksums`) to the release named, or the newest, editing those values alone; the launcher's own command, run whatever the pin says; commits nothing.                   |
| `init [--stealth]`                                                                                      | Readies a repository (`git init` first where needed): a starter config, its ledger, registry and smoke set, the newest release pinned, the hooks; with a config there, writes nothing and reports what is missing. |
| `verify <from> <to>`, `verify` (stealth)                                                                | Re-checks every non-merge commit of the range after `commits.since` (message with footers at that commit, paths, the built-in moves rule against its parent), then each range command once.                        |
| `tests list <kind> [--at <tree>]`, `tests smoke check\|ids\|run <kind>`                                 | The adapter's listing; the smoke rule, the smoke IDs, the smoke run.                                                                                                                                               |
| `tests moves <kind>`                                                                                    | The staged feature files against HEAD's by the built-in moves rule, by hand.                                                                                                                                       |
| `ci plan [<from> <to>]`, `ci run [<from> <to>]`, `--nightly`                                            | Prints the plan (with no range, only in stealth mode); runs it, stopping at the first failure unless `ci.stop_at_first_failure` is false.                                                                          |
| `ci scope <from> <to>`, `ci range --head <sha> [--base <sha>]`                                          | Whether a range is prose only; where a push's range starts.                                                                                                                                                        |
| `hook commit-msg <file>`, `hook pre-push <remote> <url>`, `hook pre-tool-use`                           | The hooks' entry points: git's two (the pre-push one verifies the pushed commits first), and Claude Code's PreToolUse guard against an agent's `git commit` and `git push` (below).                                |
| `hooks install [--manager <m>] [--print] [--force]`                                                     | Writes the one-line shims for the hook manager it detects, or prints its snippet; under a stealth config, declares the hooks in the git config.                                                                    |
| `config check [--print-defaults]`, `config get <key>`                                                   | Validates the config, the ledger, the registry and the smoke sets; prints a key's value as the tools read it, defaults applied (slice 45).                                                                         |
| `version [--check]`                                                                                     | Needs no config; `--check` exits 1 if the binary does not satisfy `requires`.                                                                                                                                      |
| `help <command>`                                                                                        | The command's help, or `itos-<command> --help` for an extension; `itos --help` lists the extensions on the `PATH`.                                                                                                 |
| any other `<command> [args]`                                                                            | An extension: runs `itos-<command>` found on the `PATH` with the arguments after its name, unread, and hands back its exit code (below).                                                                           |

**Extensions** (slice 29, the user's idea): a command itos does not have runs
the program `itos-<command>` found on the `PATH`, and only there, as git runs
`git-<command>`; a built-in always wins, and a command that is neither is the
usage error `unknown command: <command>`. Everything after the command's name
is the extension's, given to it unread, so it can take flags of any name,
`--json` and `--config` among them; only the global flags written before the
name are itos's. They apply first (`--root` is the folder the extension runs
in, as is the repository's top when a run from a subfolder moves there, §5)
and reach it in its environment: `ITOS_CONFIG` and `ITOS_ROOT`, absolute;
`ITOS_JSON=1` with `--json`, else unset; `ITOS_BIN`, the itos binary running,
to call back; `ITOS_VERSION`, its version, which the launcher reads (§10), so
a call back runs the same version. The trivial first one, `itos-hello`, is an
example for authors in `docs/extensions.md`, not a release asset; the hand work
(a push that waits for CI, the inbox) can follow as extensions.

**itos commit** (slice 31, the user's calls of 2026-10-03): `itos commit`
runs `git commit` with every argument but its own flags, `--task <ids>` and
`--scenarios <ids>`, and writes from them the footer whose source is the
ledger and the one whose source is a kind of named tests (the first of
`commits.footers` each, found by the source, never the key), each passed as
git's own `--trailer`, so they land whether the message comes from `-m`, `-F`
or the editor. IDs go on lines of at most 100 characters, the key repeated,
since a wrapped footer is read only from its first line. The hooks run as for
any commit and judge a footer itos writes as they judge a typed one; a commit
they refuse is not made, and itos hands back git's exit code. git tells a
hook nothing of an amend, so itos commit does (slice 37): `ITOS_AMEND` is 1
when `--amend` is among the options it hands git, 0 otherwise, set every
time, and the hook guesses from the author's date only when it is absent.

**Every footer a commit needs** (slice 36, the user's calls of 2026-10-03):
links (the footers of the ledger and of named tests, IDs itos resolves) and
content (a footer of free text, `BREAKING-CHANGE`, text for people and
tools) share git's trailer block, and the config's source tells them apart,
so the config does not change. Each footer of free text it declares has an
`itos commit` flag of its name in lower case (`--upgrading` for
`Upgrading`), unless a built-in flag has that name, and `--breaking <text>`
writes `BREAKING-CHANGE: <text>`, the form git reads as a trailer
(`BREAKING CHANGE:` with its space makes `git interpret-trailers` see no
trailers in the block at all); their value is the next argument, as git
takes an option's, an empty one a usage error. The lines go links first,
then free text in the config's order, then `BREAKING-CHANGE`. Before git
runs, a commit whose type requires a footer that neither its flags nor its
message give is refused (exit 1, nothing committed), naming the flag; the
type is read from `-m`, `-F <file>`, or HEAD's message for
`--amend --no-edit`, and a message from the editor or `-F -` is left to the
hook, which still judges every commit made any other way. git runs with
`trailer.ifExists=addIfDifferent`, so `itos commit --amend --task T-001` on
a commit carrying `Task: T-001` leaves one footer. Under a stealth config
only the links go to the note; content stays in the message, where the
hook, verify and `commit footers` read it. A first argument
naming a subcommand (`check-message`, `check-paths`, `footers`) is that
subcommand, anything else a commit. It is the habit an agent is pointed to
(the plugin answers a bare `git commit` with it, §10), the other half of
pushing through itos (`itos push`, below), and the one the stealth mode keeps,
where the same lines become a note on the new commit instead (slice 32, §5).

**itos push** (slice 39, the user's call of 2026-10-03: built in, since
extensions are for what is not core) is the routine every session ended
with by hand (AGENTS.md's "Finishing"): commit, pull with a rebase, check
that no rebase stopped and no conflict is left, then push in a separate
step. It takes no arguments. It fetches the branch's upstream (its
`branch.<name>.remote` and `.merge`, else `origin` and the branch's own
name) and rebases onto the commit fetched with its own flags,
`git rebase --no-autostash` after a `git fetch` rather than `git pull`, so
`pull.rebase` cannot make it a merge and `rebase.autostash` cannot pocket a
change. It refuses to start (exit 1) when tracked files have uncommitted
changes, saying to commit or stash them first (untracked files are no
reason), when a rebase is in progress or a conflict is left, and on a
detached HEAD. A rebase that stops is left in progress for the person,
nothing is pushed, and it exits 1 saying how to go on (`git rebase
--continue`, then `itos push` again; or `git rebase --abort`). Then, a
rebase in progress and a conflict checked for again, it pushes HEAD to the
upstream's branch as an explicit refspec: the pre-push hook runs as for any
push, and nothing else goes, so the stealth mode's `refs/notes/itos` stays
local whatever push refspec the config has (under a stealth config
`notes.rewriteRef` is set first, as `itos commit` sets it, so the rebase
carries the notes). It never forces: `--force`, `-f`, `--force-with-lease`
or a `+` refspec is a usage error (exit 2) naming why, and a push the remote
or the hook refuses is reported with git's exit code, never retried.
Nothing to push is success (exit 0, saying so); a remote without the branch
is a new branch, pushed with no rebase. Outside a repository, or with no
upstream and no `origin`, it exits 3. Printing the CI run the push started
is `p1-ci-watch`'s.

**The pre-push hook verifies** (slice 46, the user's question of
2026-10-03): a commit that skipped the commit-msg hook (`--no-verify`, a
clone without the hooks) was first judged by CI, once on the remote and no
longer the person's to amend. `itos hook pre-push` now runs verify first,
over what each pushed ref adds: `<remote sha>..<local sha>` when the clone
has the remote's commit, else (a new branch, or a remote commit the clone
lacks) the local commit's commits on no remote-tracking branch, as `verify`
with no range takes them under a stealth config; a deleted ref adds
nothing. `commits.since` and the range checks apply as in verify. It prints
nothing when every commit passes, so a push of passing commits reads as it
did, and the released corpus's cases of the hook keep their output, which
let it ship as a feat. A failure prints verify's report and how to fix the
commits (`git commit --amend` for the last one, `git rebase -i` from the
parent of the first failing one for an earlier one), exits 1 and runs none
of `hooks.pre_push`'s commands; without `hooks.pre_push` the hook is the
verify alone, and a config with no `commits` section has nothing to verify.
No key turns it off. CI still verifies every commit since its last green
run: it is the gate nobody can skip on their own machine. Under a stealth
config `hooks install` still declares no pre-push entry without
`hooks.pre_push`, as a released corpus case promises; declaring it always is
`v3-stealth-pre-push`'s.

**The git shim** (slice 41, the user's calls of 2026-10-03): an alias reaches
no script, editor or agent, so itos can stand in for git on the `PATH`. One
binary, busybox's trick: started under the name `git` (argv[0]'s base name,
`git` or `git.exe`), through a link named git in a folder before the real git,
itos is the shim, so the shim is the itos version in effect. The real git is
the first `git` on the `PATH` that is not this binary, compared as files, so
a link to itos is skipped; with none, exit 3, saying so. In a repository itos
manages (an `itos.yaml` in the folder git runs in or at its top, or a stealth
config), found from any folder of it by plain file checks walking up to the
`.git`, which agree with §5's lookup and run no git, `git commit …` runs as
`itos commit …` and `git push …` as `itos push …`, with the same arguments,
none read as an itos global flag (the shim runs `itos git-shim run -- <command>
<args>…`). Of git's options before the command, `-C <path>` is honoured (the
shim moves there before it looks) and so is `-c <name>=<value>`, handed to
every git itos runs in `GIT_CONFIG_COUNT`, since editors commit with
`git -c <key>=<value> commit`; `--no-pager` and `-P` change nothing. Any other
option there (`--git-dir`, `--work-tree`, …), or `GIT_DIR` or `GIT_WORK_TREE`
set, names a repository the file checks do not follow, and runs the real git.
Every other command, and every command elsewhere, replaces the process by the
real git on unix (a child whose exit code is handed back elsewhere), with its
arguments, stdin, terminal and exit code untouched, for the cost of a start and
a few file checks. itos itself never runs the git on the `PATH`: every git it
starts is the real one, and it sets `ITOS_GIT` to it for everything it starts,
so a git that a hook, a check or an older pinned itos runs under an itos run
passes straight through the shim, and `itos commit` cannot recurse into itself.
The launcher and the pin apply to `git-shim run` as to any run, so in a pinned
repository `git commit` is the pinned itos's. One that predates the shim
(older than 2.2.0, by the pin or `ITOS_VERSION`) has no `git-shim` to run, so
there `git commit` and `git push` run the real git, after one line on stderr
naming the version and the shim's least (bug 7); `git-shim install` and
`uninstall` always run the
binary called, the one they link. `install` links it as git in `--dir`
(default: the folder holding it), replaces only a link to an itos, refuses any
other git (exit 1), and says whether the folder comes before the real git on
the `PATH`; `uninstall` removes the link, and only a link to itos. Per machine
and opt-in: the hooks and CI's `verify` stay the gates, and with the shim on a
hand-typed `git commit` goes through `itos commit`, so the hook guesses an
amend only for a git the shim does not wrap, and the plugin's guard (§10) is a
backstop.

**The guard** (slice 42, the user's calls of 2026-10-03): `itos hook
pre-tool-use` is the `PreToolUse` hook the Claude Code plugin (§10, T-066)
wires on Bash, so a repository's pin picks the version that answers. It reads
Claude Code's JSON on stdin (`tool_name`, `tool_input.command`, `cwd`; every
other key ignored) and judges the repository from `cwd`, else the folder itos
was started in, as the shim does (an `itos.yaml` in the folder or at the top,
or a stealth config, by file checks). There a Bash command that runs
`git commit` or `git push` is denied: exit 0 and Claude Code's
`hookSpecificOutput` with `permissionDecision: deny` on stdout, its reason
naming `itos commit --task <id>` or `--scenarios <ids>`, or `itos push`, which
Claude Code shows the agent. Everything else gets no answer, exit 0 and
nothing on stdout, never an `allow`, which would skip the person's own
permission rules. An input it cannot read exits 1, the reason on stderr and
naming PreToolUse, since Claude Code blocks a tool on exit 2 alone. The command
is parsed as bash parses it (`mvdan.cc/sh/v3/syntax`): every simple command of
a list, pipeline, subshell, compound command or substitution, its variable
assignments skipped, read past `command`, `exec`, `nohup` and `env` (and env's
`NAME=VALUE` words), then `git` or a path ending in `/git`, its global options
(`-C` moving the folder judged, as the shim honours it; the others, with their
values, passed over), then the subcommand; text an argument carries
(`grep 'git commit'`) is a word, not a command. A guardrail for agents that
follow it, not a fortress: `sh -c`, `eval`, scripts, git aliases, words built
from variables, a `cd` before the command and a command bash cannot parse are
not looked into, and the commit-msg hook and CI's `verify` stay the gates.
`itos waive` joins what it denies with p3-human-waiver. Where the launcher
would hand `hook pre-tool-use` to an itos older than the guard (2.3.0, by the
pin, `ITOS_VERSION` or the newest release), which has no such hook and whose
usage error's exit 2 Claude Code takes as a block, it runs nothing and answers
nothing itself, exit 0, after one line on stderr naming the version (slice
44, as bug 7 did for the shim), so the plugin calls `itos hook pre-tool-use`
plainly whatever a repository pins.

**Hooks in the git config** (slice 33, the user's calls of 2026-10-03): git
2.5x runs a hook declared in its config, `hook.<name>.event` and
`hook.<name>.command`, as well as the one in `core.hooksPath` or the hooks
folder. `hooks install --manager git-config` declares itos's there, in the
repository's own `.git/config`, never committed: `hook.itos-commit-msg`, and
`hook.itos-pre-push` when `hooks.pre_push` gives it commands (git refuses a
hook named after its event), each running `<hooks.bin> hook <event>`. So
itos's hooks run beside a project's own without touching its hook files or
settings, and a hook manager resetting `core.hooksPath` cannot remove them.
It is what `hooks install` picks under a stealth config (§5) when neither
`--manager` nor `hooks.manager` names a manager; running it twice changes
nothing, and a git that does not run config hooks (asked by `git hook list`
of a hook declared with `-c`, the feature rather than a version) exits 3.
With `itos commit`'s note (slice 32) and a `hooks.bin` of `itos` (slice 35),
the stealth mode then touches nothing tracked and nothing shared.

**The person's own commits** (slice 34): in a repository that does not use
itos, others' commits follow no rules of the person's, so under a stealth
config `verify` and `ci plan` given no range judge the commits of HEAD on no
remote branch, `HEAD --not --remotes`: the person's, not yet pushed, and
every commit of HEAD when there is no remote. Their footers come from each
commit's note (slice 32). In a project both still require their range, the
usage error unchanged, and a range given in stealth mode is read as before.
The range starts at `--remotes` (`git.Unpushed`), which `ci plan --json`
gives as its `from`; the files it touched, for the prose shortcut, are the
paths its commits touch, and a range command's `{from}` is the pushed commit
they grow from, empty as a new branch's when there is none or several.
`ci run` with no range still runs every step and every test.

**Exit codes:** 0 success; 1 a policy failure (a check failed, a commit
rejected, an unknown task); 2 a usage or config error, a file or folder the
config names that is missing or unreadable among them (the ledger's folder a
footer reads, a smoke set); 3 a missing environment, a pinned release the
launcher cannot fetch or check among them (§10), an extension that cannot
start, and a git shim with no other git on the `PATH`. In `ci run` a failing step exits with its own code, an extension's
exit code is the run's, `itos commit`'s is git's, and so is `itos push`'s
when the fetch or the push fails. A missing
identity and a failing range provider are not errors, nor is a project's
people file missing or unreadable: every command goes on without the people,
checking no owner, and `config check` alone warns of it, never failing (slice
35; `itos init` is where it is reported properly).

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
                   value and source (YAML as JavaScript reads it; where
                   itos reads its data), launch (the launcher) and shim (itos
                   started as git)
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
   repository switches to it (T-063). Done: `use: builtin` lints the header
   in the commit-msg hook, `commit check-message` and `verify`, and this
   repository runs it, commitlint gone, held to a fixture of commitlint's
   verdicts recorded before it went (`tools/selftest/header-agreement.ts`).
5. v2.0.0 (T-061): Go only, the tarball gone; consumers switch to the binary
   on v2's Upgrading section, which asks for every compatibility change at
   once.

Each step is reverted, never forced, if it goes wrong. After v2, features are
built once, in Go: extensions (`itos-<cmd>` on `PATH`; done, slice 29, §7), the stealth mode and
the GitHub modes. The stealth mode's config in the git folder is done (slice
30, §5), and so is `itos commit`, which writes a commit's footers from its
flags (slice 31, §7), the stealth mode's footers in git notes (slice 32,
§5), its hooks in the git config (slice 33, §7) and its range, the person's
unpushed commits (slice 34, §7), and its defaults, one person's: the global
`itos` in its hooks and no people file (slice 35, §5). `itos commit` tells
the hook an amend, which it guessed before (slice 37, §7), and writes every
footer a commit needs, refusing one that lacks a required footer before git
runs (slice 36, §7). A stealth session owns every item, `itos work` looking
no identity up (slice 38, §5). `itos push` pulls with a rebase and pushes,
never forcing (slice 39, §7). From a subfolder, itos and the launcher find the
config at the repository's top and run from there (slice 40, §5). Linked as
git before the real one, itos runs git commit and git push as its own in a
repository it manages, and the real git for everything else (slice 41, §7).
`itos hook pre-tool-use` denies an agent's git commit and git push in Claude
Code, naming the itos command to use (slice 42, §7). The pre-push hook
verifies the commits it pushes before its commands run (slice 46, §7).

## 10. Distribution

A release, cut by CI when a push to `main` that carries a `feat`, a `fix` or a breaking change is
green (T-069, §3, `.github/workflows/release.yml`): for the Go binary, archives for linux and darwin on
amd64 and arm64 and windows/amd64, each with the binary, `LICENSE` and
`README.md` at its top level, named `itos-<version>-<os>-<arch>.tar.gz`
(`.zip` for windows), plus the config's JSON Schema, `itos.schema.json`,
which an editor checks an `itos.yaml` against, and `checksums.txt`, one file
holding every asset's SHA-256 (`sha256sum --ignore-missing -c checksums.txt`
checks whichever were downloaded). Until v2.0.0 the releases carried the
TypeScript tarball, `itos-<version>.tgz`, beside them, listed in the same
`checksums.txt`; it left with the TypeScript (T-062). GoReleaser builds them (`.goreleaser.yaml`),
uploads them to a draft, the archives are attested (`gh attestation verify <file> -R
donvargax/itos`), and the draft is published with its notes, which is when its tag is created;
`tools/selftest/release-cut.ts` holds a snapshot to what the last release published. The
repository is public, so no download needs a token.

A consumer commits an install script that pins the version and each
platform's SHA-256 (a replaced release cannot pass), installs into an ignored
`.tools/bin/`, and the script `hooks.bin` names runs it before the binary, so
it installs on first use, in a clone and on a CI runner alike, and does
nothing once that version is there (v2.0.0's Upgrading); Go developers can
`go install github.com/donvargax/itos/v2/cmd/itos@<version>` (the module path
ends in its major version from v2, as Go requires). This repository commits nothing for a release:
the tag is the version (T-069). A global install (slices 27 and 28): the installed binary is a launcher,
never rewritten, that runs the version a repository pins (`pin.version`, and `pin.checksums`, the
SHA-256 of that release's `checksums.txt`, one hash for every platform), fetched into a cache and
checked; a config with no pin runs the binary that was called, and with no `itos.yaml`, or a stealth
config (§5) that pins nothing, it runs the newest release, asked for at most once a day.
`itos pin [<version>]` (slice 47) moves a pin to a release, the newest by default, writing both
keys in place and committing nothing, so a bump is the project's own build commit. Asked for the
version already pinned, it refuses (exit 1) when that release's `checksums.txt` no longer hashes to
the pin, rather than re-pinning: the release changed after it was pinned, which the pin exists to
catch (the user's call, 2026-10-03). Later channels: the aqua or mise registry, a Homebrew tap,
npm (as `palitos`) and PyPI wrappers, signatures.

**Adoption** (the user's calls, 2026-10-03). This repository is also a Claude Code plugin
marketplace: `.claude-plugin/marketplace.json` at the root names the itos plugin in
`integrations/claude-code/` (T-066), with a version of its own in its `plugin.json`: it followed
itos's until T-069 made the tag itos's version, and may move to a repository of its own. The plugin carries the titles (an
itos ID drawn with its title in Claude's replies, from the project's own itos), a short skill on
working with itos (find work with `itos work`, commit with `itos commit --task`, push with
`itos push`, read what a gate says, never run the gates by hand), and a `PreToolUse` hook
(`itos hook pre-tool-use`, slice 42, §7) that,
in a repository with an itos config, answers a `git commit` or `git push` with the itos command to
use instead: a guardrail for agents, a hook rather than permission rules, since a rule matches a
command's prefix and `git -C . commit` slips past it; the commit-msg hook stays the gate. Its
hooks run the itos the repository's git hooks run (T-073, the user's call): its effective
`hooks.bin`, asked of the `itos` on the `PATH` with `itos config get hooks.bin`, a path resolved
against the repository's top; with no answer (no `itos`, or one older than v2.4.0), the v2 default
`tools/bin/itos` when it is executable; else the `itos` on the `PATH`. So a repository's own
build or install script answers, and through the launcher its pin picks the version: a pin older than the guard gets
no answer from the launcher rather than a block (slice 44, §7). `itos init` (slice 48) makes a repository ready,
new (`git init` first) or existing: where there is no config, a starter `itos.yaml`, small and
commented (the Conventional Commits types under the built-in header lint, a `Task` footer every
type but feat and fix needs, `commits.since` at HEAD so history written before itos is never
judged, `hooks.bin: itos`, no path scopes, which are each project's own), a ledger holding `T-1`,
the task the adoption commit names, and an empty registry under `tasks/`, a `Scenarios` footer and
a smoke set naming each file's first live scenario when feature files exist, a pin on the newest
release (none, said so, where the release server cannot be reached), then the hooks, as `hooks
install` detects their manager; with `--stealth`, all of it under the git folder and the hooks in
`.git/config`, nothing tracked touched. It is the launcher's own command, as `pin` is: where there
is no config there is no pin to hand the run to. It offers the plugin (slice 49), opt-in
everywhere (the user's call, 2026-10-03), through Claude Code's own CLI (`claude plugin list
--json`, then `claude plugin marketplace add donvargax/itos` and `claude plugin install itos@itos`
with `--scope`): `--plugin <scope>` answers, `project`, `user`, `local` or `no`, a bare `--plugin`
taking `project`, or `local` under `--stealth`, which refuses `project` since the project's
settings are committed and lists `.claude/settings.local.json` in `.git/info/exclude` when git
would show it; a terminal is asked, that scope its default, and anywhere else nothing is installed
and the report says how; a `claude` that fails at the install exits 1, the rest of init done. It
offers the git shim (§7) the same way, which `itos git-shim install` makes. Run again where a config is,
it changes nothing and reports what is missing (config check's problems and a hook that does not
call itos, slice 48), exit 1 when anything is, so it doubles as a doctor; it also reports, never
counting them as missing, the plugin not installed (slice 49; `--plugin` installs it there too),
and the git shim, a pin behind the newest and the people file (slice 50). Agents too (the user's calls, 2026-10-03): the
config's rules are generated into a marked block of `AGENTS.md`, itos touching only what is inside
its markers, how to work with itos staying in the plugin's skill; `--agents` writes the
orchestration files (the coordinator's guide, the handoff, the brief template) once, as the
project's own, a candidate to move into an extension later; under `--stealth`, Claude Code gets a
`CLAUDE.local.md` importing the rules and Codex an `AGENTS.override.md` holding a copy of the
project's `AGENTS.md` beside them, both listed in `.git/info/exclude`.

v0 releases are the TypeScript packed to JavaScript, since Node strips types
only outside `node_modules`, published as a tarball a consumer pins.

Each release's notes are generated, never committed (T-069): `tools/bin/release-notes` writes them
from the range, published as the release's description, and `tools/selftest/release-notes.ts`
proves what a command can of them. `docs/releases/` keeps the committed notes of the releases up to
v2.3.0, which were written by hand. They list every commit by type (git-cliff), and end with an
"Upgrading" section a consumer's session updates from alone, so it is complete: what each breaking
change's `BREAKING-CHANGE:` footer asks; what every `Upgrading:` footer since the last release asks
(from T-061), quoted, which is where a commit says what a consumer must change, drop or expect to
be rejected; every `Changes:` entry (T-071); each config key added, removed or with a changed
default, from the schema contract (T-070); and the pin to change: the install script with each
platform's hash from the release's `checksums.txt`, the pin's two lines with that file's own hash,
`go install` and the schema line.

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
| Agents depend on today's wording.                    | Wording kept verbatim and compared by the corpus; `--json` is the stable interface. Help text is held to this tree's corpus only, never to the last release's (T-071).          |
| The built-in header lint disagrees with commitlint.  | It is held to commitlint's verdicts on this repository's whole history before this repository switches (T-063); a project keeps its delegate (`use: command`) until it chooses. |
| Glob semantics differ in Go.                         | The glob function is ported as written, and the corpus's glob table pins it.                                                                                                    |
| Windows: checks are `sh`.                            | `shell:` in the config; Git for Windows' `sh` preferred.                                                                                                                        |
| A released binary runs in every hook.                | Committed per-platform SHA-256s, a pinned version, `version --check`; signatures later.                                                                                         |
| An adapter is slow or flaky.                         | The hook lists only the kinds a message names; an adapter's failure is an error, never a pass.                                                                                  |
| The features' header-lint scenario needs commitlint. | It ran this checkout's commitlint until commitlint left (T-063); its step now writes the built-in lint, the scenarios' text unchanged.                                          |
