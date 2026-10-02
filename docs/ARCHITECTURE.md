# Architecture as built

How the project is put together, as it actually is. Written once and amended
when a slice changes the shape of something; what landed, and when, is the
history (`vp run changelog`), and the decisions behind it are in `PLAN.md`.

## The layout

- `tools/itos/` is itos v0: TypeScript, one module per concern, run by Node
  directly (`tools/bin/itos` is the entry point). The scope rules name it as
  `commits.path_sets.implementation`; "Task tooling" below says what is in it.
- `features/` holds itos's named tests: Gherkin feature files, their steps in
  Go (`*_test.go`, package `features`), and the smoke set (`smoke.yaml`).
- `tools/itos/conformance/` is the regression corpus, `tools/selftest/` the
  self-tests of the gates and of the release, `tools/changelog.ts` the
  changelog's filter.
- **The package** is for consumers, not for this repository: `vp pack` (the
  `pack` block of `vite.config.ts`, tsdown) bundles `tools/itos/main.ts` and
  everything it imports, `yaml` and the commands' lazy imports included, into
  one file, `dist/itos.mjs`, which `package.json`'s `bin` names; its `files`
  are that, `LICENSE` and `README.md`. `tools/bin/pack.ts` is the one way to
  pack it, for the release workflow and `release.ts` alike: it runs `vp pack`,
  copies those files into a scratch folder beside a manifest of `package.json`'s
  `name`, `version`, `type`, `bin` and `files` plus `engines` (Node 24), and
  `npm pack`s that folder into `itos-<version>.tgz`, with no runtime
  dependencies and no install script. This repository's `package.json`, with
  its `prepare` script and devDependencies, is never packed. Here `tools/bin/itos`
  runs the working tree's TypeScript, since the hooks and CI judge the working
  tree. Nothing in the source reads a file beside itself, which is what lets
  one file carry it: the version is `package.json`'s, which `version.ts`
  imports as JSON and the bundle inlines, so a release is one `build` commit
  to `package.json` and a tag.
- **A release** (`.github/workflows/release.yml`, on a `v*` tag) refuses a tag
  that is not `v<package.json's version>`, a packed itos that says another,
  and a tag with no `docs/releases/v<version>.md`; it proves the tarball with
  `release.ts` (the packed manifest's keys, an offline install reporting no
  install script, the corpus and every feature against the installed itos), then publishes it with `checksums.txt`, the release's body
  being a header, the checksum and that notes file with `{sha256}` replaced by
  the tarball's hash. The notes end with an "Upgrading" section a consumer
  updates from alone (`PLAN.md`, §10).
- `go.mod` is the module `github.com/donvargax/itos`, Go pinned by its
  `toolchain` line; its dependencies are godog's and `go.yaml.in/yaml/v3`,
  the port's one. The Go port is `cmd/itos` and `internal/` ("The Go port"
  below).

## The features

What itos does is Gherkin in `features/`, run by
[godog](https://github.com/cucumber/godog) through `go test ./features`:
`TestFeatures` (`features_test.go`) runs every feature file, each scenario a
subtest named after it. `features/README.md` holds the rules: the black-box
boundary, the ID scheme, the tags, the smoke set, the moving rule.

- **The steps** (`steps_test.go`) treat itos as a black box. Each scenario
  builds a scratch git repository in a temporary folder, writes its
  `itos.yaml` and ledger, runs the binary `ITOS_BIN` names
  (`tools/bin/itos` by default, relative to the module's root) in it, and
  asserts the exit code, the output and the files it leaves. A command runs
  in a clean environment: the caller's, less `GIT_*`, `ITOS_*`, `GITHUB_*`
  and `CI`, with no global or system git config and a fixed identity, so a
  run inside a git hook or on a runner sees what it sees locally. A step
  never reads itos's code, so the same steps judge the Go port.
- **The header lint**, where a scenario needs one, is this checkout's
  commitlint with only `@commitlint/config-conventional` (its config written
  to the scenario's temporary folder, `--cwd` the checkout so the extends
  resolve), so a scenario's footer rules are itos's own rather than a
  commitlint plugin's.
- **Selection.** `-scenarios=<regexp>` runs the live scenarios with a tag the
  expression matches: the harness reads every tag in the feature files and
  hands godog the matching ones as its own filter (exact tags joined by
  commas, each `&&~@wip`). itos's run templates produce that expression
  (`tests.scenario.run` in `itos.yaml`: `ids_pattern` `^@(?:{ids})$`, joined
  with `|`); godog's filter could not take it directly. An expression that
  matches no tag fails the run.
- `@wip` scenarios never run.

## Task tooling

**itos** is the task tooling: the ledger and its checks, the commit rules,
named tests and the smoke set, the CI plan and its driver, the work registry
and the hooks, behind one command line, `tools/bin/itos` (`itos --help` lists
the commands, `itos <command> --help` each one). The code is
`tools/itos/`, TypeScript run by Node directly.

- **One policy file.** `itos.yaml` at the root holds every table the tool
  reads: the ledger's layout (`ledger`), the commit types, footers, path sets
  and scopes (`commits`), the named tests and their adapters (`tests`), CI's
  steps, costs, prose shortcut, nightly and range (`ci`), the work registry
  and the people (`work`), and the hooks (`hooks`), and where verification
  starts (`commits.since`). `tools/itos/config.ts`
  loads it and rejects a key it does not know, naming the one it misspells;
  `ITOS_CONFIG` or `--config` names another file. A project changes its
  policy here, not in the code, and every key accepted is one a tool reads:
  a key waiting on a feature not built yet (the built-in header lint's
  `use` and `alongside`, a second way to tell a commit is pushed) is
  rejected until that feature reads it. **The defaults are one table**,
  `DEFAULTS` in `config.ts`: the loader lays the file over it
  (`withDefaults`, `tests.<kind>` under each kind the file has), and
  `config check --print-defaults` prints it, so no tool writes a fallback of
  its own and a default is applied exactly when it is printed. One default
  depends on the config: `work.registry` is beside the config's ledger, so
  `defaultsFor` gives the table as it applies to a file, and both the loader
  and `--print-defaults` read it through that. `section()`
  still asks whether the file has a section (`hasSection`), since a section
  the file leaves out holds only its defaults. A key with no default
  (`ledger.id`, a kind's `root`) stays absent.
- **What it reads.** The ledger is `tasks/phase-<n>.yaml` (`ledger.files`),
  the registry `tasks/work-items.yaml` (`work.registry`'s default,
  `work-items.yaml` beside the ledger in `ledger.files`' folder, but not a
  ledger file, since it does not match `ledger.files`), the people
  `CONTRIBUTORS.md` (`work.people`), the smoke set `features/smoke.yaml`
  (`tests.scenario.smoke`). `itos config check` validates all of them.
  A ledger group is called by `ledger.group.label` (`phase` by default) in
  what the tools print (`work check`'s messages, `itos task`'s usage), and
  `itos task` takes `--<label>` beside `--group` and `--phase`; the rule IDs,
  the registry's item key `phase` and `work.groups_key` are data, and keep
  their names whatever the label.
  The ledger's folder missing is one config error, `ledger-folder-missing`,
  exit 2: the ledger's loader (`ledgerFiles` in `config.ts`) reports it for
  every command that reads the ledger, and `config check` as its one problem;
  the footer rules name it by their own rule, `footer-source-missing`.
  A kind's smoke set (`tests.<kind>.smoke.file`) has one loader
  (`smoke.ts`); its rule and its run, `itos tests smoke`, are
  `smoke-rule.ts`.
  Every reader of them goes through one source (`source.ts`): the working
  tree by default, or, while a function runs, a tree git holds, the index or
  a commit. So the commit-msg hook's check of the staged data is
  `config check` itself, run over the index; a config the tree does not hold
  (an `ITOS_CONFIG` outside the repository) is read where it is.
- **One command line** (`tools/itos/main.ts`): exit 0 on success, 1 for a
  policy failure (a check failed, a commit rejected, an unknown task), 2 for a
  usage or config error, 3 for a missing environment. `--json` prints one
  object with `"schema": 1`, logs on stderr; each problem in it has a
  sentence, a `rule` id and, where one exists, a `fix`. The task runner is
  called by its own name, `tools/bin/itos task <id>`; `vp run work` and
  `vp run ci` are `package.json` scripts over it. One invocation of the
  runner (`cli.ts`) runs each distinct check once: `runCheck` (`checks.ts`)
  keys a run by the command, whitespace collapsed (`normal`), and the
  timeout, and every later task that lists it reads the kept exit status by
  its own `run:` or `fails:`. The runs live for the invocation alone; CI's
  plan and the commit-msg hook run their checks their own way. `task list`
  runs nothing: it reads each task's item status from the registry
  (`itemStatuses` in `work.ts`, which the hook's `itemStatus` reads too).
- **Named tests behind an adapter** (`tools/itos/tests.ts`). A kind of named
  test (here one, `scenario`) says how its tests are listed and run. The
  built-in Gherkin adapter (`gherkin.ts`) is the only module that parses a
  feature file; a scenario is live when neither its tag line nor its file's
  header holds `@wip`. The kind's `run` and `recognize` templates are how CI
  reads a task check as a selection of tests and merges every selection into
  one command; nothing else knows Gherkin or the runner. Another kind can be
  any command that prints the same JSON. `ciPlan` (`ci-plan.ts`) marks a
  check merged only when that command is in the plan: with nothing selected
  (an empty smoke set, a range that names no test) the kind's run is left
  out, and a check read `as: smoke` runs as itself. `as: whole` and
  `as: pattern` always select something, so their run is always there.
- **Range checks** (`tests.<kind>.range_checks`): a kind's rule on how its
  tests may change between two trees. One is either commands (`staged`, run
  by the commit-msg hook on the index, `range`, run once by `verify` over the
  range with `{from}` and `{to}`) or the built-in moves rule
  (`builtin: moves`, `moves.ts`), which reads a Gherkin kind's feature files
  through `parseFeature` with the kind's root, ID pattern, tag prefix and wip
  tag. The hook judges HEAD against the index (`stagedMoveIssues`, in
  `commit-scope.ts`'s rejection beside the path rules), `verify` each commit
  of its range against its parent, the empty tree for a root commit
  (`commitMoveIssues`, in that commit's rejection, so it counts against the
  commit), and `itos tests moves <kind>` HEAD against the index by hand. Both
  hook and verify skip a type `except_types` names, and one that is not in
  `commits.types` when the config lists them (a merge's message); the
  command ranges wait for the type to have a path rule, the built-in does
  not. A tree's feature set is read once per run. `config check` refuses
  `builtin` beside a command, on a kind whose adapter is not `gherkin`, and
  `allowed_renames` without it.
- **The footers** have one reader (`footers.ts`), which the footer rules
  and CI share: which types need each footer, which IDs must exist, and
  `read_at: commit`, which reads the IDs that exist (the ledger's tasks, the
  live scenarios) at the commit being checked. The footer rules are itos's
  whatever the header lint: `commit.ts` runs them after the delegate, if
  any, in the commit-msg hook, `commit check-message` and `verify`, and
  reports both, one list of problems under `--json`. Nothing reads a footer
  by its key: CI's tasks (`tasksIn` in `ci-scope.ts`) and the commit-msg
  hook's are those of every footer whose `source` is `ledger`, a kind's named
  tests those of every footer whose source is that kind, so `Task:` and
  `Scenarios:` are only this repository's names for them. What itos prints
  names the ledger's folder by `ledger.files` (`ledgerLayout`) and the prose
  steps by `ci.prose.steps`, never `tasks/` or `vp check`.
- **One shell** (`shell.ts`): every command itos takes from the config or the
  ledger (a task check, a CI step, a header-lint delegate, a range check, a
  provider's or a command adapter's command, the pre-push commands, the smoke
  run) starts through `inShell`, which appends it to `shell` (`[sh, -c]` by
  default) as one argument. itos's own `git` calls start directly.
- **The world outside the repository** is three providers
  (`providers.ts`): where a push's range starts (`ci.range`: the last green
  run on GitHub, a command, or none), who a session works for
  (`work.identity`: `gh api user`, a command, or only `--as`), and who works
  on the project (`work.people`).
- **Where verification starts** (`commits.since`, `repo.ts`): `verify`, and
  with it the built-in moves rule, lists a range's commits less that commit
  and its ancestors, and a range command's `{from}` is that commit when the range's own
  start is empty or older. `config check` and `verify` fail with exit 2 when
  the repository does not have it; in a shallow clone (`git rev-parse
--is-shallow-repository`, actions/checkout's default one commit deep) the
  same problem says the commit may lie beyond the clone's history, with
  `git fetch --unshallow` or `fetch-depth: 0` as its fix. The commit-msg hook
  never reads it.
- **Conformance** (`tools/itos/conformance/`): what itos does, as cases run
  through its command line in scratch repositories, one file per area, every
  `--help` text among them, since agents read them. `run.ts --bin <command>`
  runs them through any binary; a tampered case file must fail, so the
  runner compares what it claims to. It is a regression corpus, run by a CI
  step of its own: a change to what itos does lands with its scenario in
  `features/` and, where a case records the old behaviour, the case. `tools/itos/fixtures/` holds the configs and ledgers the
  negative proofs in `tasks/phase-0.yaml` run against.

## The Go port

The Go build of itos lands beside the TypeScript one command group at a time
(`PLAN.md`, phase 2), until it passes everything and the TypeScript goes.

- **The packages.** `cmd/itos` is the binary, a call to `internal/cli`.
  `internal/cli` is `main.ts`: the global flags wherever they stand, the
  command table with every command's argument errors, and how a failure is
  reported and which exit code it takes; `configcheck.go` in it is
  `config-check.ts`. `internal/version` is `version.ts`, and `internal/out`
  prints a `--json` object with `"schema": 1` first and its keys in the order
  written (Go's maps would sort them). The rest are the packages `PLAN.md` §8
  lists, each holding what the groups ported so far need: `internal/ledger`
  (the layout, the files, every task and check problem, and the tasks and
  checks typed), `internal/tests` (the Gherkin adapter over the working tree,
  the smoke set and its rule), `internal/providers` (the people),
  `internal/work` (the registry and its problems, and the items' statuses),
  `internal/shell`, `internal/check` (below) and `internal/git`, beside two
  the TypeScript has no module for:
  `internal/value` and `internal/source` (below). The port shells out to git
  where it needs it, as the TypeScript does.
- **The config** (`internal/config`) is `config.ts`'s loader, and every Go
  reader of the config goes through it. It finds the file (`--config`,
  `ITOS_CONFIG`, after `--root`'s `chdir`), holds it to the schema
  (`schema.go`, `SCHEMA`'s specs as data, with its problems' wording), then to
  the cross-checks (`cross.go`), and lays it over **the one table of
  defaults**, `defaults()` in `defaults.go`: one ordered tree, as `DEFAULTS`
  is one object, merged under the file by `layered` (`tests.<kind>` under each
  kind) and decoded into the typed `Config` the tools read. `DefaultsFor` is
  `defaultsFor`, the registry beside the config's ledger, and `config check
--print-defaults` prints exactly that tree, so a default cannot be applied
  without being printed. A key with no default is a nil pointer, a nil list or
  an empty `Ordered` (a mapping whose order matters, as written); the file as
  written stays beside the loaded config for `HasSection` and `Section`.
  `Readings` and `MatchesStatic` are `readings` and `matchesStatic`.
- **YAML as JavaScript reads it** (`internal/value`). go.yaml.in/yaml/v3
  parses, but every value is JavaScript's: each plain scalar is resolved by
  the YAML 1.2 core schema the `yaml` package uses (yaml/v3 would read `017`
  as octal and `1_000` or `0b1` as numbers), mappings keep JavaScript's key
  order (array indices first), absent is `Undefined` apart from null, and the
  messages render a value as a template literal (`String`), `typeof`
  (`TypeOf`) and `JSON.stringify` (`JSON`) would, so a problem quoting an odd
  value reads the same in both. `YAML` writes the `yaml` package's block
  style, for `--print-defaults`. The patterns are compiled as RE2: the
  config's patterns were written for JavaScript, so one using lookaround or a
  backreference is refused by Go and accepted by the TypeScript, and the
  reverse for an inline flag such as `(?i)`; where itos builds a pattern
  around `\s` or trims, it uses `value.Space` and `value.Trim`, JavaScript's
  whitespace, not RE2's ASCII one.
- **Where itos reads its data** (`internal/source`) is `source.ts`: every
  reader of the config, the ledger, the registry, the people and the smoke
  sets reads through it, with Node's wording for a failed read
  (`ENOENT: no such file or directory, open 'people.yaml'`). It has the
  working tree alone so far; the hooks group adds the index, so the
  commit-msg hook's check of the staged data is `config check` again.
- **The task runner** is `internal/cli/task.go`, `cli.ts` ported, over three
  packages the later groups share. `internal/shell` is `shell.ts`: `Run`
  starts a command through the config's `shell`, with its streams (an
  `*os.File` handed over as it is, so a check's output keeps its place beside
  what itos prints; nil is `/dev/null`), its timeout and its environment, and
  says how it ended (`Result`: the code, whether it timed out, why it could
  not start). Past its timeout a command gets SIGTERM, as `spawnSync`'s
  default `killSignal`, then `shell.Grace` (10 seconds) before it is killed,
  where Node would wait however long it takes; a command that traps the
  signal and exits 0 has code 0, as `spawnSync`'s status is 0, so a caller
  that fails it whatever its code reads `TimedOut`. `internal/check` is
  `checks.ts` and `cost.ts`: a `Runner` is one invocation's runs, keyed by
  `Key` (the command through `config.Normal` and the timeout), its `Run`
  printing the verbose command line and reusing a run as `runCheck` does,
  and `CostOf`, `CostedChecks` and `ChecksBeforeLate` are the cost classes
  and written order, for CI's plan and the commit-msg hook (config check's
  written-order rule stays in `internal/ledger`, since it reads the ledger
  before it is typed). `ledger.Tasks` is `loadTasks`, through `ledger.Files`:
  a task whose checks cannot run (a `done_when` that is not a list, a check
  without a command) carries its error and fails only the command that runs
  it, as the TypeScript fails only there. `work.ItemStatuses` is
  `itemStatuses`, the registry read alone, without the people, so `task list`
  works over a registry with problems. A group flag's value is read by
  JavaScript's `Number()` (`value.ToNumber`, so `--group 01` is group 1) and
  the status table pads by UTF-16 units (`value.PadEnd`). The runner's ID
  pattern is `^(?:<ledger.id>)$`, `T-\d+` without one, compiled as RE2
  like every config pattern.
- **What is not ported yet** fails loudly: every command takes its arguments
  as the TypeScript does, so a usage error reads the same in both, and then a
  command whose group has not landed exits 3, the missing environment's code,
  with `itos: <command> is not in this build yet (the Go port has not reached
it)`.
- **The help texts** are `internal/cli/help.go`, `help.ts` ported: every
  text in one table keyed by command path, as `HELP` is, not one beside each
  command, so the two read side by side. `itos`, `itos help …` and any
  `--help` print the longest command path the table knows, before any config
  is read, whether or not the command is ported. A change to a text lands in
  both files and `help.yaml` in one push, since the corpus file is in the
  ported set.
- **The version** is `package.json`'s: `tools/bin/build-go.ts <out dir>`
  builds `./cmd/itos` into `<out dir>/itos` with `CGO_ENABLED=0` and
  `-trimpath`, stamping the version into `internal/version` (`-ldflags -X`).
  A binary built without the stamp says the module version Go records
  (`go install …@v<x>`), or `(devel)` when there is none. Every self-test
  builds the binary through it.
- **The release build** is `tools/bin/build-go.ts --release <out dir>`: the
  same build once per platform `PLAN.md` §10 lists, each binary packed with
  `LICENSE` and `README.md` at the top level of
  `itos-<version>-<os>-<arch>.tar.gz` (`.zip` for windows), and
  `checksums.txt` in `sha256sum`'s format beside them, what the release
  workflow will upload at v1.0.0. The archives are written with Node's
  `zlib`, not the machine's tar or zip, with HEAD's commit time on every
  entry, so a commit rebuilt by the same Go toolchain gives the same bytes.
  `tools/selftest/go-release.ts` builds them into a scratch folder and reads
  them back with the system's `tar`, `unzip` and `sha256sum`; the nightly
  runs it (`ci.nightly.steps`), not every push.
- **The ported set** is `PORTED` in `tools/selftest/go-port.ts`, the one list
  of it: the conformance corpus files and the scenario selections (`-scenarios=`
  expressions) of the groups that have landed. `go-port.ts` builds the binary
  into a scratch folder and runs the corpus files through `run.ts --bin` and
  each selection as `go test ./features` with `ITOS_BIN` pointed at the build,
  and fails when the set is empty or anything in it fails. It is a late step
  of every push's CI, so a later change to a ported group lands in both
  implementations in one push; `ci.covers` skips a task check that builds the
  binary and runs a ported corpus file again. A group adds its part to the
  list when it lands.
- **Its commits.** The port's code is `refactor` with a `Task:` footer: the
  behaviour is the TypeScript's, already specified. The Go files under `cmd/`
  and `internal/` are in `commits.path_sets.implementation`, and their
  `*_test.go` are the `test` type's; CI's `gofmt` step covers them and a late
  step runs their unit tests (`go test ./cmd/... ./internal/...`).

## The gates and CI

- **The hooks** (`.vite-hooks/`): `commit-msg` and `pre-push` are one-line
  shims `itos hooks install` writes for the manager `hooks.manager` names
  (over the one its markers show), calling `itos hook commit-msg` and
  `itos hook pre-push`; `pre-commit` is the project's own. `vp config`
  (`prepare`, on `vp install`) points git at the folder.
  - **pre-commit** runs `vp staged` (each path's command in `vite.config.ts`'s
    `staged`: `vp check --fix`, or for Go `gofmt -w` and then `go vet` over the
    module, since it reads packages rather than files), then, unless every
    staged file is Markdown, under `docs/**`, under `tasks/**` (the ledger and
    the registry) or a feature file, `vp test run --changed HEAD` with coverage collected but
    no thresholds, then `fallow audit` on what is new against HEAD. Vitest
    follows the imports from every changed file; `forceRerunTriggers` reruns
    everything when the config, the lockfile or `itos.yaml` changes, written
    as the files themselves, since vitest's own defaults never match a changed
    file. The audit scores changed functions by that coverage
    (`.fallowrc.json`), exact for the changed files since every test that
    runs one imports it. It does not check itos's own data: the commit-msg
    hook does, from the staged tree (T-023 dropped T-022's check here, which
    read the working tree).
  - **commit-msg** first checks itos's own data when the commit stages any
    of it (`commit-data.ts`): the config, a ledger file, the registry or a
    smoke set, as the staged config names them, runs `config check`'s
    problems over the index, the registry's own check among them, and rejects
    the commit with them, since a project's pre-commit hook passes them as
    prose and the working tree may hold what the commit does not. It comes
    first because the other rules read the config. Then it applies the
    type's path rules (`commit-scope.ts`), then outside `feat` and `fix` the
    scenario moving rule (the built-in, `moves.ts`), then the header lint:
    commitlint (`commitlint.config.ts`, `config-conventional` alone), and
    after it, always, itos's footer rules (`commit.ts`, one `<key>-footer`
    rule per footer of `commits.footers`), both reported before the exit,
    a failing delegate's code the hook's, then the static
    checks of the tasks the `Task:` footer names (`commit-tasks.ts`),
    stopping at the first that fails. The task checks come last because
    they are the slowest and read a footer the header lint has judged: each
    named task's checks, as the staged ledger holds them, run in written
    order up to its first late one by CI's cost rule (`cost.ts`), quietly,
    through the shell, without the commit's `GIT_INDEX_FILE`, each capped
    by `hooks.commit_msg.check_timeout`; an `after: push` check waits. A
    failure rejects the commit when the task's item in the staged registry
    is `done`, else it is printed with the task's status and the commit
    goes through. `hooks.commit_msg.task_checks: false` turns them off.
  - **pre-push** runs `hooks.pre_push`: `vp test run --changed <remote sha>`
    for each pushed ref, or the whole unit suite when there is no remote
    commit to compare with. Nothing else: the scenarios and the task checks
    are CI's.
- **CI** (`.github/workflows/ci.yml`) is one job, a thin wrapper around
  `itos ci run`, so everything it does runs locally too. A newer push
  replaces a run still waiting for the runner; a running one finishes, and
  the newest run checks every commit since the last green one. The range starts at the last
  green run on `main` (`itos ci range`, the `ci.range` provider), or at a pull
  request's base; empty means run everything. It is written to the job's
  environment once, so the scope, the commit re-check and the plan read the
  same range. `itos verify` re-checks every commit of the range after
  `commits.since` with the commit-msg rules, so a commit made with the hooks
  bypassed fails CI. The workflow sets Node and Vite+ up, and Go from
  `go.mod`.
  **The plan** (`ci-plan.ts`, its cost rule in `cost.ts`; `itos ci plan <from> <to>` prints it, running
  nothing) is one sequence in cost order: the static steps of `ci.steps`
  (`vp check`, `gofmt`, `go vet`, the smoke rule, `itos config check`) and every named task check
  that is static (its own `cost: static`, else a pattern of
  `ci.cost.static`: `matchesStatic` in `config.ts`, which `config
check`'s written-order rule reads too); then the late steps (the whole unit suite, the Go
  packages' unit tests, the audit, the conformance corpus, the Go build against the
  ported set, T-007); then **one run of the features** over the smoke
  set (of the kind the `tests:` step names; a CI without one reads none), the scenarios the `Scenarios:` footers name and the subsets of the
  tasks the ledger footers (`Task:`) name; then the named tasks' late checks. A task's checks
  keep their written order. A check a step has just done is skipped
  (`ci.covers`), one in `ci.nightly_only` waits for the nightly, and a task
  whose work item is still `todo` waits (`ci.wait_on_status`). Every one of
  these patterns (`ci.cost.static`, `ci.covers`, `ci.nightly_only` and the
  kind's `recognize` templates) reads a command whose first word is
  `hooks.bin` as starting with `itos`, through one function, `readings` in
  `config.ts`: each is tried on the command as written and as so read, so a
  pattern written for `itos` and one naming the path both match. It stops at
  the first failure; with `ci.stop_at_first_failure: false` it runs every step
  and check, says where each failure was, and exits with the first one's code.
  A range of only `ci.prose.paths` (Markdown, `docs/**`)
  runs `ci.prose.steps` (`vp check`, and `itos config check`, since
  `CONTRIBUTORS.md`, the people a registry's owners must be among, is
  Markdown; the registry and the ledger, under `tasks/`, are not prose) and the named tasks' static and `prose: true` checks,
  and no features.
- **The nightly** (`.github/workflows/nightly.yml`, at 11:44 UTC on `main` or
  by hand) runs `itos ci run --nightly`: `ci.nightly.steps` in written order,
  here every feature, then the gates self-test, then the static checks of every
  done task. That last is the step `{ tasks: done, cost: static }`
  (`nightlyPlan` in `ci-plan.ts`): the checks of each task whose work item is
  `done` in the registry (`itemStatuses`, the status the commit-msg hook calls
  a red check a regression by), in cost order where the step is written, only
  the static ones with `cost: static`. A task in progress, or with no item, is
  left out. A check a nightly step has done is not run again, one that is a run
  of the kind is in the nightly's whole run, and `ci run` hands the step's
  checks one `Runs` map (`checks.ts`), so a check two done tasks share runs
  once, as in `itos task`; a push's run keeps each named task's checks its
  own. `config check` refuses the step in `ci.steps`, where the tasks are the
  ones the commits name. A failing task check names the task's ID and title,
  in a push and in the nightly. A red run opens one issue labelled `nightly-red`, or comments on
  the open one with the failing scenarios (go test's `--- FAIL:
TestFeatures/…` lines); a green run closes it.
- **The self-tests** (`tools/selftest/`) prove the gates rather than the code:
  `gates.ts` runs the real hooks in a scratch worktree and shows that they run
  only what a change reaches and that CI's steps catch what they leave out
  (a refactor that changes what itos prints, in a module no unit test
  imports, passes both hooks and fails the push's features step);
  `ci-scope.ts`, `ci-range.ts` and `features-scope.ts` prove the scope, the
  range and that the plan's features command runs exactly the scenarios it
  claims, by go test's own record of what it ran (`-json`).
  `config-gate.ts` proves itos's data is checked where it is guarded: in a
  scratch worktree, running both hooks as git does, the commit-msg hook
  rejects a commit staging a ledger with a misspelt key and passes a sound
  one, a commit staging only `docs/**` and `tasks/**` runs no unit tests, and
  CI's plan runs `itos config check` for a range touching
  the registry or the ledger and for a prose-only range touching
  `CONTRIBUTORS.md`. It and `gates.ts` build their worktree with
  `scratch.ts`.
  `release.ts` proves the tarball rather than the source: it packs (or takes
  `--tarball`, the one a release publishes), installs it with
  `npm install --offline` and an empty cache into a scratch project, which
  must then hold itos alone, and runs the conformance corpus and every
  feature against the installed bin. `release-notes.ts` proves what a command
  can of a release's notes: the file is there, its last `##` section is
  "Upgrading" with the pin line naming this version's tarball, and every
  config key whose default differs between the last release (its tarball
  downloaded from GitHub, verified and installed offline) and this tree, by
  `config check --print-defaults --json`, is named in that section.
- **The changelog** (`tools/changelog.ts`, `cliff.toml`): git-cliff groups
  the Conventional Commits by type, each with its footers and body, into
  `docs/changelog/`, which git, the formatter, the linter and the audit
  ignore; `-- --task <id>` and `-- --scenario <id>` print one footer's
  commits.
- **Agents' worktrees** live under `.claude/worktrees/`, ignored by git, and
  left out of the unit tests, the linter and the formatter: their files are
  theirs, often half-written, and never this checkout's.
