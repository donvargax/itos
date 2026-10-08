# Architecture

A map of the repository as it is built: what lives where, how the pieces
meet, where to look for each concern, and the rules that cross packages. It
stays a map:

- **What one Go package does, how and why** is that package's doc comment, in
  a `doc.go` beside its code (`go doc ./internal/<pkg>`), changed in the same
  commit as the code. Read it before touching the package.
- **What landed, and when** is the history: `vp run changelog`, or
  `vp run changelog -- --task <id>` and `-- --scenario <id>` for one task's or
  one scenario's commits.
- **Why it is so** is a decision record in `docs/decisions/` when it was a
  decision, a task's `why` in `tasks/*.yaml`, a scenario's comment in its
  feature file.

## The layout

| Path                        | What it is                                                                                                 |
| --------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `cmd/itos`                  | The binary: the order a run passes through the packages.                                                   |
| `internal/`                 | itos's packages (below). With `cmd/`, `commits.path_sets.implementation`.                                  |
| `features/`                 | The named tests: Gherkin feature files, their Go steps (`*_test.go`), the smoke set `smoke.yaml`.          |
| `tools/itos/conformance/`   | The regression corpus, one YAML file per area, and its runner `run.ts`.                                    |
| `tools/itos/fixtures/`      | The configs and ledgers the ledger's negative proofs (`tasks/phase-0.yaml`) hand itos.                     |
| `tools/bin/itos`            | The entry point the hooks, CI and the ledger's checks call: builds `./cmd/itos` from the tree and runs it. |
| `tools/bin/*/` (Go)         | The gates and the release's programs, each `go run ./tools/bin/<name>`, each documented in its `main.go`.  |
| `tools/bin/*` (sh, `.ts`)   | `dev-version`, `go-unit-tests`, `pinned`, `install-launcher`, `build-go.ts`, `inbox.ts`.                   |
| `tools/selftest/`           | The self-tests of the gates and of the release (Node), run with the git `tools/bin/real-git` names.        |
| `tools/changelog.ts`        | The changelog's filter over git-cliff.                                                                     |
| `itos.yaml`                 | This repository's policy: every rule a command can decide.                                                 |
| `tasks/`                    | The ledger (`phase-<n>.yaml`), the work registry (`work-items.yaml`), the questions (`asks.yaml`).         |
| `docs/`                     | This map, `PHASES.md`, `ORCHESTRATING.md` (the repository's notes for `itos go`), `CLI.md`, decisions.     |
| `integrations/claude-code/` | The Claude Code plugin; `.claude-plugin/marketplace.json` makes the repository its marketplace.            |
| `.github/workflows/`        | `ci.yml` (every push), `release.yml` (called by `ci.yml`), `nightly.yml`.                                  |
| `.goreleaser.yaml`          | The release build.                                                                                         |
| `.vite-hooks/pre-commit`    | The project's own pre-commit hook; itos's hooks live in the git config.                                    |
| `deps-check.json`           | The dependency check's exceptions, each with a reason.                                                     |
| `.devcontainer/`            | A sandbox for Claude Code on this repository: shim first, egress allowlisted. Its README says how.         |

itos is Go. `go.mod` is the module `github.com/donvargax/itos/v7`: from v2 a
module path ends in its major version or Go refuses its tag, so the path
moves before a major release, and `tools/bin/release-version` refuses a
version whose major the path does not match. Go is pinned by its `toolchain`
line; the dependencies are few and held to the dependency check. Node runs
only this repository's tooling (the corpus runner, the self-tests, the
builds, the inbox); no consumer of itos needs it.

## How a run goes

```
git (the shim's link) ─┐
itos ──────────────────┴─> cmd/itos ─> shim? ─> launch ─> cli ─> command
                                                  │
                                       a pinned release, fetched,
                                       checked and run in its place
```

1. **The git shim** (`internal/shim`): started as `git` through the link
   `itos git-shim install` makes, itos passes every git command to the real
   git, except `git commit` and `git push` in a repository itos manages, which
   become `itos git-shim run`.
2. **The real git** (`internal/git`): `git.Export` sets `ITOS_GIT` so nothing
   the run starts reaches the shim again.
3. **The launcher** (`internal/launch`): picks the version to run
   (`ITOS_VERSION`, else the config's `pin`, else the newest release where
   there is no config), fetches it into a cache, checks it against the pin's
   checksums and runs it in place of itself, or runs itself. A global itos is
   this launcher
   (`docs/decisions/0024-a-global-itos-is-a-launcher-that-runs-the-version-a-repository-pins.md`).
4. **The command line** (`internal/cli`): global flags, the spec of each
   command's flags, the extensions (`itos-<name>` on the `PATH`), the move to
   the repository's top from a subfolder, then the command, which reads the
   config and the data, judges, writes and reports.

The pieces every command meets:

- **The config** (`internal/config`) is `itos.yaml`, found by `--config`,
  `ITOS_CONFIG`, `itos.yaml` at the top, or the stealth config in
  `<git common dir>/itos/`. It is laid over one table of defaults, the table
  `config check --print-defaults` prints, and every reader goes through
  `config.Load`.
- **The data** (`internal/source`): the ledger, the registry, the people, the
  smoke sets, the questions and the decision records are read through one
  source, the working tree by default, or the index or a commit while a
  function runs. So the commit-msg hook's check of staged data is
  `config check` run over the index.
- **The providers** (`internal/providers`): the world outside the
  repository. Where a push's range starts (`ci.range`), a commit's CI run and
  the nightly (`ci.watch`), who a session works for (`work.identity`), who
  works on the project (`work.people`). GitHub's API is reached through
  `GITHUB_API_URL` when set, so tests point it at a local server.
- **The shell** (`internal/shell`): every command the config or the ledger
  gives starts through the config's `shell` (`[sh, -c]`); itos's own git calls
  start directly, always to `git.Bin()`.
- **The output** (`internal/out`, `internal/kind`): `--json` is one object
  with `"schema": 1`, logs on stderr; a problem has a sentence, a rule id and a
  fix. The exit code comes from the error's kind alone: 0, 1 a policy
  failure, 2 usage or config, 3 a missing environment, 75 may pass when run
  again, 70 unclassified.

How the gates use them: the git hooks call `itos hook commit-msg` and
`itos hook pre-push`; CI calls `itos ci range`, `itos verify` and
`itos ci run`; a release calls `itos commit footers` and the programs under
`tools/bin/`. Every rule they apply is a command anyone can run locally.

## Where to look

| Concern                                                                  | Package                                                |
| ------------------------------------------------------------------------ | ------------------------------------------------------ |
| A command's flags, help text, exit code, output                          | `internal/cli` (`spec.go`, `help.go`, the command)     |
| Choosing, fetching and running a pinned release; the daily update notice | `internal/launch`, `internal/release`                  |
| Release addresses, `checksums.txt`, newest tag, what is releasable       | `internal/release`                                     |
| Finding and loading the config; defaults; stealth mode; `commits.since`  | `internal/config`                                      |
| Reading data at the working tree, the index or a commit                  | `internal/source`                                      |
| The ledger's files, tasks and checks                                     | `internal/ledger`                                      |
| Running checks, their cost classes                                       | `internal/check`, `internal/shell`                     |
| The commit message: footers, the built-in header lint, notes, wrapping   | `internal/message`                                     |
| The paths each commit type may touch                                     | `internal/scope`, `internal/glob`                      |
| Named tests, adapters, run templates, smoke set, the moves rule          | `internal/tests`                                       |
| CI's plan, and its driver                                                | `internal/plan`, `internal/ci`                         |
| The work registry: reading, proposing, every writer                      | `internal/work` (judging), `internal/cli` (writing)    |
| The code proof of a CI run: a provider's check, read by its contract     | `internal/proof`, read by `internal/ci`                |
| Questions and decision records                                           | `internal/ask`, `internal/adr`                         |
| Follow-up threads, the coordinator's drafts                              | `internal/follow`, `internal/draft`                    |
| The guides `itos go` and `itos guide` print                              | `internal/guide`                                       |
| The git shim, the Claude Code guard                                      | `internal/shim`, `internal/guard`                      |
| git itself: the real binary, paths, merges, ranges, rebase state         | `internal/git`                                         |
| YAML read as itos reads it, and edited in place                          | `internal/value`                                       |
| The lock around shared files; next IDs; the version                      | `internal/lock`, `internal/nextid`, `internal/version` |

`internal/cli`'s doc comment has a heading per command group: the commands the
launcher leaves to the binary (`pin`, `upgrade`, `init`, `git-shim`),
`itos commit`, `itos push`, the hooks, `verify`, CI, tasks and named tests,
the registry's commands, `followup`, `draft`, `decision`, `go`, `status` and
the guard.

## Rules that cross packages

What a reader needs before touching any package:

- **Policy is config, never code.** A rule a project might want otherwise is a
  key of `itos.yaml`. Every key accepted is one a tool reads; an unknown key is
  an error naming the one it misspells, and a key a major release removed is
  refused as removed, saying what to write instead.
- **One table of defaults.** No reader writes a fallback of its own; a default
  is applied exactly when `config check --print-defaults` prints it.
- **One reader per thing.** The footers have one reader (`internal/message`),
  feature files one parser (the Gherkin adapter), the config one loader, a
  commit's type one reading (`message.Type`), a command's run of named tests
  one builder (`tests.CommandFor`), shared by the hook, verify, the plan and
  the commands, so they cannot disagree. Nothing reads a footer by its key:
  a footer's `source` decides what it names.
- **Every git is the real git**, through `git.Bin()`. Paths from git are read
  NUL-separated (`-z`), changed paths with `--no-renames` and no filter that
  drops a change type, so the hook, verify and the plan agree on what a commit
  touches. A merge is judged by its own changes.
- **A gate never passes on what it could not read.** A shallow clone, a range
  start that is not a commit, a release that cannot be fetched: exit 2 or 3,
  never 0.
- **Files people edit are edited in place.** The config's pin, the registry
  and the ledger are changed through `internal/value`'s editors, which keep
  every comment and quote and refuse any edit that does not read back as
  intended. Files that are itos's own (follow-ups, drafts, questions) are
  written whole.
- **The registry is written by commands, never by hand**, each committing its
  own change alone as a `docs` commit with no footer; under a stealth config
  nothing is committed and the lock (`internal/lock`) orders writers instead.
  `work done` closing a task with checks commits the task's ledger file with
  the registry, its `done_when` taken out (`ledger.WithoutChecks`): a task's
  checks gate its close, then leave the ledger, git's history keeping them.
- **The stealth mode** (one person's itos in a repository that does not use
  it) is known to few places: the config's location and defaults, the source
  (a path in the git folder is read from the file), the footer rules (links
  live in git notes, `refs/notes/itos`), `itos commit` and `itos push` (the
  notes), and the unpushed range for `verify` and `ci plan`.
- **itos's words follow its first implementation.** itos began as TypeScript;
  YAML is read by the 1.2 core schema into JavaScript's values, globs and
  whitespace follow JavaScript, and messages quote values as JavaScript
  rendered them, since the corpus pins those words.
- **Only machine output is contract**: exit codes, `--json` less `message` and
  `fix`, and the files itos writes
  (`docs/decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md`).
  Plain text, help included, is for people and may change; the commands the
  plugin calls are never renamed or removed (decision 0041).
- **A behaviour change is a `feat` or a `fix`** with its scenario in
  `features/` and, where a corpus case records the old behaviour, the case.
  The help texts live in one table (`internal/cli/help.go`); a change to one
  lands with its case in the corpus's `help.yaml`.

## The features

What itos does is Gherkin in `features/`, run by godog through
`go test ./features`, each scenario a subtest of `TestFeatures`.
`features/README.md` holds the rules: the black-box boundary, the ID scheme,
the tags, the smoke set, the moving rule. The steps never read itos's code:
each scenario builds a scratch git repository, runs the binary `ITOS_BIN`
names (`tools/bin/itos` by default) in a clean environment (no caller's
`GIT_*`, `ITOS_*`, `GITHUB_*`, global git config, network, real cache or
Claude Code), and asserts the exit code, the output and the files left.
Fake release servers, a fake GitHub, fake `gh` and `claude`, extensions and
the shim's link are local to each scenario; how each part of the harness is
built is `features/features_test.go`'s comment.

`-scenarios=<regexp>` runs the live scenarios with a tag the expression
matches, the form itos's run templates produce (`tests.scenario.run`); an
expression matching no tag fails the run. `@wip` scenarios never run.

**The conformance corpus** (`tools/itos/conformance/`) is what itos does as
cases run through its command line in scratch repositories, one file per
area, every `--help` text among them, since agents read them.
`run.ts --bin <command>` runs them through any binary; a tampered case file
fails, since the runner compares what it claims to. A case's `github` key
starts a fake GitHub API in the runner for that case, so the GitHub
providers have cases without a network. CI runs the corpus as a step of its
own; its `--additive` mode is how the last release's corpus judges this
tree (below).

## The release

A release is cut by CI, never by hand
(`docs/decisions/0021-ci-cuts-a-release-from-a-green-push-judged-against-the-last-release.md`):
the version is the tag, held nowhere in the tree.

- **The cut.** `ci.yml`'s `release` job, after its `ci` job and its three
  `platform` jobs pass on a push to `main`, calls `release.yml`, the only job
  given `contents: write`, `id-token: write` and `attestations: write`, one
  at a time in the `release` concurrency group, never cancelled. It adds no
  gate: the push's CI is what a release rests on.
  `go run ./tools/bin/release-version` computes the version from the
  commits since the newest `vX.Y.Z` tag HEAD reaches (a breaking change a
  major, else a `feat` a minor, else a `fix` a patch, else nothing and the job
  ends green); a `go.mod` path ahead of that release's major promises it, so
  any `feat`, `fix` or breaking change cuts that major (T-123). While `tools/bin/release-version/prerelease` says `rc` (T-118, T-119), a
  major is cut as `<major>.0.0-rc.<n>`, the next candidate after each `feat`,
  `fix` or breaking change, published as a pre-release and never latest;
  removing the file cuts `<major>.0.0`. With a version the job tags the commit locally, runs
  GoReleaser (`tools/bin/pinned goreleaser release --clean`), which builds
  the five archives, `itos.schema.json` and `checksums.txt` into
  `dist/goreleaser` and uploads them to a draft release on the commit,
  attests every file `checksums.txt` lists
  (`actions/attest-build-provenance`), writes the notes and `upgrading.json`,
  uploads `upgrading.json` and publishes the draft with the notes. GitHub
  creates the tag when the draft is published, so a tag never exists without
  its assets, and a failed run leaves at most a draft, which the next
  replaces. The releaser commits nothing.
- **The build** (`.goreleaser.yaml`): one binary per platform
  `docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md`
  lists, packed with `LICENSE` and `README.md` at the top of
  `itos-<version>-<os>-<arch>.tar.gz` (`.zip` for windows), entries owned by
  root by number and dated at the commit; `itos.schema.json` (written by a
  `before` hook) and `checksums.txt` in `sha256sum`'s format beside them.
  `tools/bin/build-go.ts <out dir>` builds a checkout's binary with
  `CGO_ENABLED=0` and `-trimpath`, stamped with `tools/bin/dev-version`'s
  version (`internal/version`); `--release` builds a GoReleaser snapshot.
- **The notes** (`tools/bin/release-notes`) are generated, never committed: a
  title and the range's counts; "What changed", every commit by type, from
  git-cliff; and "Upgrading", last
  (`docs/decisions/0026-a-release-s-notes-are-generated-from-its-commits-and-end-with-a-complete-upgrading-section.md`):
  each breaking change, every `Upgrading:` footer and `Changes:` entry quoted,
  the config's changes since the last release (from the schema contract),
  then the install script, the pin and `go install` lines.
- **`upgrading.json`** (`release-notes -json`) is that Upgrading section as
  data, the asset `itos upgrade` reads of every release between a project's
  pin and the new one: `schema`, `version`, `previous` (the last release, so
  `itos upgrade` walks back without listing releases through an API), then
  `breaking`, `upgrading`, `changes` and `config`. It is not in
  `checksums.txt`, so not attested: itos only prints it.
- **The config's JSON Schema** (`itos.schema.json`, for an editor's
  `yaml-language-server` line) is generated by `tools/bin/config-schema` from
  `config.Schema()` with the defaults laid on it. What the schema cannot say it
  leaves to `config check`, so it says less, never something different.
- **The release tools** run at pinned versions through `tools/bin/pinned`:
  GoReleaser and git-cliff, each fetched once into `.tools/<tool>-<version>/`
  and checked against the SHA-256 written in the script, each version at least
  7 days old.

## The Claude Code plugin

The repository is a Claude Code plugin marketplace:
`.claude-plugin/marketplace.json` names one plugin, `itos`, at
`integrations/claude-code/`, read from the default branch's tree. Its
version lives in `integrations/claude-code/.claude-plugin/plugin.json` alone
and is the plugin's own; Claude Code offers an update only when it changes,
so a change under the folder raises it in the same push, which CI's plugin
version rule enforces. `hooks/hooks.json` holds:

- **The titles**, a function-hooks module (`hooks/register.ts`): a
  `ui.render` hook on `AssistantMessage` draws each known ID's title beside it
  in prose (never in code), without changing what the model reads back. The
  titles are asked for at `session.start` and `turn.start`, never while
  drawing: `itos work list --all --json` and `itos task list --json`, falling
  back for an older itos and then to reading `tasks/work-items.yaml`.
- **The guard**, a `PreToolUse` command hook on Bash, `sh hooks/guard.sh`,
  running `itos guard claude-code`; no `itos` answers nothing, and an exit 2
  (an itos older than the guard) becomes 1, so it never blocks by accident.

Both run only the `itos` on the `PATH`, the global launcher, never a program
the repository ships: the plugin runs in every repository it is enabled for,
and running a cloned repository's own program would run it just by opening
Claude Code there. The plugin has no skill: how to work with itos is itos's own
guides (`internal/guide`), so no flow needs the plugin. Its tests
(`*.test.ts`, `claude plugin test`) drive the module through the engine's own
`$`; the module is typed by the declarations Claude Code writes beside a
plugin it loads (git-ignored), so the repository's type-aware lint leaves it
out, and `claude plugin validate --strict` judges it.

**Consumer reports** come in as issues through
`.github/ISSUE_TEMPLATE/consumer-report.yml`. `tools/bin/inbox.ts` is the
coordinator's view (`docs/ORCHESTRATING.md`): open issues, actionable when a
login on its allow-list opened one or last applied `itos-accepted` to it,
printed without any issue's text. Labels count for nothing, since a form or a
workflow can apply them for anyone.

## The gates and CI

Every rule a command can decide is itos's, run by the hooks and again by CI
on every pushed commit, so a hook skipped turns CI red. `AGENTS.md`'s
generated block lists, from `itos.yaml`, what each gate runs.

### The hooks

`itos hook install`, run once in each clone, declares `itos hook commit-msg`
and `itos hook pre-push` in the clone's git config
(`docs/decisions/0037-itos-installs-its-hooks-only-in-the-git-config-and-knows-no-hook-manager.md`);
git runs them beside whatever `core.hooksPath` holds. `hooks.bin` is
`tools/bin/itos` here, so this repository's hooks run its own build.

- **pre-commit** is the project's own (`.vite-hooks/pre-commit`, where
  `vp config` points git): `vp staged` (each path's command in
  `vite.config.ts`'s `staged`: `vp check --fix`, or for Go `gofmt -w` and
  `go vet`), then, unless every staged file is Markdown, under `docs/**`,
  under `tasks/**` or a feature file: the dependency check when `go.mod` or
  `go.sum` is staged, the unit tests the change reaches
  (`tools/bin/go-unit-tests --cached`), and `fallow audit` on what is new
  against HEAD (this repository's TypeScript tooling, scored by complexity).
  `go-unit-tests` chooses without building: a changed file belongs to the
  deepest package folder holding it, and one `go list -e -test -deps` over
  `./cmd/... ./internal/...` gives every package whose tests reach one;
  `go.mod` or `go.sum` runs them all. `features/` is not among them: it is
  CI's and the nightly's. It does not check itos's data: the commit-msg hook
  does, from the staged tree.
- **commit-msg** (`internal/cli`'s hooks): itos's own data when the commit
  stages any of it, as `config check` judges it over the index; the type's
  path rules; outside `feat` and `fix` the scenario moving rule; the header
  lint (built in here) and always the footer rules; then the static checks of
  the tasks the `Task:` footer names, up to each task's first late one, a
  failure refusing the commit only once the task's item is done.
- **pre-push**: the commits each pushed ref adds, verified as `itos verify`
  does, then `hooks.pre_push`: `tools/bin/go-unit-tests <remote sha>` per
  pushed ref, or `--all` with no remote commit to compare with. The scenarios
  and the task checks are CI's.

### CI

`.github/workflows/ci.yml` is a thin wrapper around `itos ci run`, so
everything it does runs locally too. A newer push replaces a run still
waiting for a runner; a running one finishes. The range starts at the last
green run on `main` (`itos ci range`, the `ci.range` provider) or a pull
request's base, empty meaning run everything, and is written to the job's
environment once, so the commit re-check and the plan read the same range.
`itos verify` re-checks every commit of the range after `commits.since`. The
workflow sets up Node, Vite+ and Go from `go.mod`, and builds itos once
(`Build itos`) before any step calls it, and installs actionlint, which the
plan's own step runs. Claude Code is not installed: no step of a push's plan
runs it, the nightly's plugin steps do.

**The plan** (`itos ci plan <from> <to>` prints it, running nothing;
`internal/plan`) is one sequence in cost order: the static steps of
`ci.steps` and every named task check that is static (its `cost: static`,
else a pattern of `ci.cost.static`); then the late steps (the dependency
check, the schema contract, the unit suites, the audit, the corpus, the last
release's suite, the gates' own self-tests); then one run of the features
over the smoke set, the scenarios the `Scenarios:` footers name and the
subsets of the tasks the `Task:` footers name; then the named tasks' late
checks. A check a step has done is skipped (`ci.covers`), one in
`ci.nightly_only` left out, one marked `after: push` listed as pending
(`itos task` and `itos work done` run it), and a task whose item is still
`todo` waits (`ci.wait_on_status`). It stops at the first failure unless
`ci.stop_at_first_failure: false`. A range of only `ci.prose.paths` runs
`ci.prose.steps` and the named tasks' static and `prose: true` checks, and no
features.

**The platform jobs** (`platform (<runner>)` on `ubuntu-latest`,
`macos-latest` and `windows-latest`) build itos natively, stamped as
`tools/bin/itos` stamps it, and run the Go unit tests and every feature
against it, in Git for Windows' `bash` on windows. They block, and the release
job needs them; releases are still built on Linux alone. Actions are pinned
by commit.

**`tools/bin/itos`** is a POSIX sh script that builds `./cmd/itos` into
`.tools/bin/itos` (ignored), stamped with `tools/bin/dev-version`'s version
and recorded in `.tools/bin/itos.version`, then `exec`s it. It rebuilds when
the binary is missing, when anything under `cmd/` or `internal/`, `go.mod` or
`go.sum` is newer (a removed source counts), or when HEAD's version changed
(a restamp is a link from Go's build cache); a fresh binary costs about
10 ms. The build goes to a name of its own and is moved over the binary, so
concurrent calls never see half a binary. A build that fails prints the
first error and exits 3, never running the old binary.

**The gate programs** under `tools/bin/`, standard library only so that no
module they judge runs inside them, each documented in its `main.go`:

- `deps-check`: no Go module of the build younger than 7 days unless
  `deps-check.json` excepts it with a reason, then govulncheck; at
  pre-commit when `go.mod` or `go.sum` is staged, and in CI for a range that
  changes them.
- `schema-contract`: a config the last release accepts is still accepted and
  means the same (a default changed is breaking), unless a commit says it is
  breaking; checked for a range with a `feat` or a `fix`.
- `plugin-version`: a range that changes the plugin raises its version.
- `doc-budget`: the docs every session or most briefs read stay within the
  byte caps in `tools/bin/doc-budget/caps.json`; in the prose plan too, as
  docs-only pushes are where the docs grow.
- `plugin-calls`: every itos command the plugin's scripts call, read from
  the scripts, is answered by `tools/bin/itos` (decision 41).
- `previous-release`: the last release's scenarios and corpus run against
  this tree's binary, the corpus judged by machine output alone in the
  runner's `--additive` mode; a failure is accepted only when a `fix` or a
  breaking commit since names it in a `Changes:` footer (T-106).

**The nightly** (`.github/workflows/nightly.yml`, at 11:44 UTC on `main` or
by hand) runs `itos ci run --nightly`: `ci.nightly.steps` in written order,
here every feature, the gates' self-tests, the release build and the
config's schema, then the steps T-125 kept from the done tasks' checks (what
a push's features step selects, the build script's staleness rule, the
upgrade asset, five runs with something hostile first on the PATH, two
coverage runs and the plugin's Claude Code checks), then the newest release's
archive passing `gh attestation verify` and the newest completed CI run on
`main` having passed its three platform jobs. A red run opens one
`nightly-red` issue, or comments on the open one with the failing scenarios; a
green run closes it.

**The self-tests** (`tools/selftest/`) prove the gates rather than the code,
in scratch worktrees whose hooks run as git runs them (`scratch.ts`):
`gates.ts` that the hooks and CI catch what they should, and that CI catches
what the hooks let through; `go-hooks.ts` that the hooks run exactly the unit
tests a change reaches; `ci-scope.ts` and `features-scope.ts` that the plan
runs exactly the scenarios it claims; `config-gate.ts` that itos's data is
checked where it is guarded; `header-agreement.ts` that the built-in header
lint agrees with a recorded fixture of commitlint's verdicts; `go-release.ts`,
`release-cut.ts`, `release-notes.ts` and `upgrading-json.ts` the release and
its notes; `go-schema.ts` the config's schema; `deps-check.ts`,
`schema-contract.ts`, `plugin-version.ts` and `previous-release.ts` the gate
programs; `go-dogfood.ts` that `tools/bin/itos` needs no Node. The GitHub
providers' choice of run, which no command reaches without a network, is
their unit tests' and the fake GitHub's scenarios.

**The changelog** (`tools/changelog.ts`, `cliff.toml`): git-cliff groups the
commits by type, each with its footers and body, into `docs/changelog/`,
which git, the formatter, the linter and the audit ignore.

**Agents' worktrees** live under `.claude/worktrees/`, ignored by git and
left out of the linter and the formatter: their files are theirs.
