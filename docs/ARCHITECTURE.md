# Architecture as built

How the project is put together, as it actually is. Written once and amended
when a slice changes the shape of something; what landed, and when, is the
history (`vp run changelog`), and the decisions behind it are records in
`docs/decisions/`.

## The layout

- itos is Go: `cmd/itos` is the binary and `internal/` its packages ("The
  code" below). `tools/bin/itos`, the entry point the hooks, CI and the
  scripts call, builds it from the tree on demand and runs it. The scope rules
  name the Go files as `commits.path_sets.implementation`. v0 was TypeScript
  in `tools/itos/`, run by Node; it left the repository once the Go port
  passed everything and this repository ran the Go binary (T-062).
- `features/` holds itos's named tests: Gherkin feature files, their steps in
  Go (`*_test.go`, package `features`), and the smoke set (`smoke.yaml`).
- `tools/itos/conformance/` is the regression corpus and `tools/itos/fixtures/`
  the configs and ledgers the ledger's negative proofs hand itos, both where
  the TypeScript was; `tools/selftest/` holds the self-tests of the gates and
  of the release, `tools/changelog.ts` the changelog's filter. These, the
  builds (`tools/bin/build-go.ts`) and the consumer inbox are this
  repository's tooling, run by Node; no consumer of itos needs Node.
- **A release** is cut by CI (T-069): `ci.yml`'s `release` job, after its
  `ci` job and its three `platform` jobs (T-072) pass on a push to `main`, calls `.github/workflows/release.yml`,
  a reusable workflow and the only job given `contents: write`,
  `id-token: write` and `attestations: write`, one at a time in the
  `release` concurrency group, never cancelled. It adds no gate: the push's CI
  is what a release rests on. `go run ./tools/bin/release-version` computes
  the version from the commits since the newest `vX.Y.Z` tag HEAD reaches
  (any breaking change a major, read as the schema contract reads one; else a
  `feat` a minor; else a `fix` a patch; else nothing, and the job ends green)
  and prints `last=`, `next=`, `bump=` and `range=` lines for
  `$GITHUB_OUTPUT`. With a version the job tags the commit locally, runs
  GoReleaser (`tools/bin/pinned goreleaser release --clean`,
  `.goreleaser.yaml`), which builds the five archives, `itos.schema.json` and
  `checksums.txt` into `dist/goreleaser` and uploads them to a draft release
  whose `target_commitish` is the commit, attests every file `checksums.txt`
  lists (`actions/attest-build-provenance`, Sigstore), writes the notes
  (`tools/bin/release-notes`, below) and `upgrading.json` beside them, uploads
  `upgrading.json` to the draft and publishes the draft with the notes
  (`gh release edit --draft=false --latest`, the GitHub CLI pinned as the
  nightly pins it). GitHub creates the tag when the draft is published, so a
  tag never exists without its assets, and a run that fails leaves at most a
  draft, which `replace_existing_draft` replaces on the next. The releaser
  commits nothing. Until v2.3.0 a release was a hand-made `build` commit to
  `package.json`, a committed `docs/releases/v<version>.md` and a pushed tag,
  which `release.yml` then built and published; each of those tags keeps its
  notes, `docs/releases/v<version>.md`. The v1 releases also carried the TypeScript packed to one
  JavaScript file, `itos-<version>.tgz`, which left with it (T-062).
- **A release's notes** (`tools/bin/release-notes`, T-069; standard library
  only) are generated, never committed: a title and the range's counts;
  "What changed", every commit of the range by type, from git-cliff
  (`tools/bin/release-notes/cliff.toml`, run by `tools/bin/pinned`); and
  "Upgrading", last
  (`docs/decisions/0026-a-release-s-notes-are-generated-from-its-commits-and-end-with-a-complete-upgrading-section.md`): each breaking change's
  `BREAKING-CHANGE:` footer (or its `!` header), every `Upgrading:` footer
  quoted (`itos commit footers Upgrading` lists them, and reads a wrapped one's
  first line only, so the generator takes the lines below it from the
  message up to the next capitalised footer), every `Changes:` entry
  (`itos commit footers Changes`), the config's changes since the last
  release (`go run ./tools/bin/schema-contract -json -release <tag>`), then the
  install script, the pin (`pin.checksums` is `checksums.txt`'s own SHA-256),
  `go install` and the schema line, filled from `checksums.txt`.
- **A release's `upgrading.json`** (`tools/bin/release-notes -json`, T-091) is
  that Upgrading section as data, the asset `itos upgrade` (slice-75) reads of
  every release between a project's pin and the new one: `schema`, `version`,
  `previous` (the last release, `X.Y.Z`, the range's start, `""` with none),
  then `breaking`, `upgrading` and `changes`, each `{commit, header, text}`,
  and `config`, each `{key, change}`. They are the very lists the section
  quotes, in its order (`Upgrading: none` left out, a wrapped footer's lines
  kept), gathered by the same code, which with -json reads neither
  git-cliff nor `checksums.txt` for it. `previous` is how `itos upgrade` walks
  back from the new release to the pin without listing releases through an
  API. The workflow uploads it while the release is a draft, so it appears
  with the rest; it is not in `checksums.txt`, so not attested, since itos
  only prints it and a pin is still checked against `checksums.txt`.
  `tools/selftest/upgrading-json.ts` proves the asset for a range against
  `itos commit footers`, `schema-contract -json` and the range's breaking
  commits. Releases cut before T-091 have none.
- **The release tools** run at pinned versions through `tools/bin/pinned`, a
  POSIX sh script: GoReleaser and git-cliff, each fetched once into
  `.tools/<tool>-<version>/` from its GitHub release and checked against the
  SHA-256 written in the script before it is unpacked, each version at least
  7 days old (T-067's rule), for linux and darwin on amd64 and arm64.
- **Consumer reports** come in as issues: `.github/ISSUE_TEMPLATE/consumer-report.yml`
  is an issue form (the command, its output, `itos version`, the config, what
  was expected) that applies `consumer-report` and nothing else.
  `tools/bin/inbox.ts` is the coordinator's view of them (`docs/ORCHESTRATING.md`,
  the loop): it asks `gh api` for the repository's open issues, leaving pull
  requests out, and for the timeline of each issue that no allowed login
  opened. It prints two lists, actionable and the rest, with number, title,
  author and labels and no issue's text. An issue is actionable when a login
  on `ALLOWED`, the allow-list kept in the script alone, opened it, or when its
  timeline's last `itos-accepted` event is such a login applying it. The labels
  an issue carries count for nothing: a form or a workflow can apply them for
  anyone. `--self-test` runs fixture issues through the same judging; CI's
  token has `issues: read` for the task check that runs the live inbox.
- **The Claude Code plugin** (T-066): the repository is a Claude Code plugin
  marketplace. `.claude-plugin/marketplace.json` names one plugin, `itos`, at
  `integrations/claude-code/`; a person adds the marketplace by the GitHub
  repository and installs `itos@itos`, and Claude Code reads the plugin from
  the default branch's tree, so a commit there is what the next update gets.
  The version lives in `integrations/claude-code/.claude-plugin/plugin.json`
  alone (the marketplace entry gives none, so the two cannot disagree) and is
  the plugin's own: it followed itos's until T-069, which made the tag itos's
  version, and nothing compares the two now. Claude Code offers an installed
  plugin an update only when that version changes, so a change to anything
  under `integrations/claude-code/` carries a raise of it, semver for the
  plugin itself (a fix a patch, a new behaviour a minor, a breaking change a
  major), in a `chore` commit of the same push: CI's plugin version rule
  (T-074, below) refuses a range that changes the folder and leaves the
  version where it was. `hooks/hooks.json` holds both kinds of hook Claude Code reads
  there: `modules`, the function-hooks module `hooks/register.ts`, and
  `hooks`, the command hooks. The module draws the titles: a `ui.render` hook
  on `AssistantMessage` rewrites the props it hands on (the stream, the
  transcript and what the model reads back stay as written), with the titles
  `hooks/titles.ts` writes beside each known ID in prose, never in fenced code
  or a longer code span. The titles are asked for at `session.start` (a hot
  reload fires it again) and `turn.start`, never while drawing:
  `$.process.run` of `itos work list --all --json` and `itos task list --json`
  in the session's root, the items first and then the tasks with no item. An
  itos older than slice 77 refuses `--all`, so a `--all` run that gives no
  answer is followed by plain `itos work list --json`, every item before
  v4.0.0 (slice 79 made plain `work list` the open items alone); a run that
  cannot start, exits non-zero or prints no `items` (an itos older than
  v2.3.0 reads `work list` as `work`) falls back to reading
  `tasks/work-items.yaml` with `$.fs`. Both hooks run only the `itos` on the
  `PATH` (T-097), never a program the repository ships: not its `hooks.bin`
  and not a `tools/bin/itos` at its top. The guard runs before every Bash
  command and the titles on every reply, in every repository the plugin is
  enabled for (user scope covers them all), so resolving the repository's own
  itos, as T-073 did, ran a cloned repository's program just by opening Claude
  Code there. The `itos` on the `PATH` is the launcher of a global install,
  which runs the release the repository's pin names, fetched from itos's
  releases and checked against their checksums; a developer of itos who wants
  a dev build to answer puts it first on the `PATH` (this repository's own git
  hooks still run its `hooks.bin`, installed deliberately). Its tests
  (`*.test.ts`, run by `claude plugin test`) drive the module through the
  engine's own `$`, the test's hooks beneath it answering `session.root`,
  `process.run` and `fs.read`; a repository there ships an executable
  `tools/bin/itos` and a `hooks.bin` script, each answering titles of its own,
  and the tests prove only `itos` starts. The command hook is `PreToolUse` on
  Bash, `sh hooks/guard.sh`, which runs `itos hook pre-tool-use` behind two
  shell guards: no `itos` on the `PATH` answers nothing, and an exit code 2
  (an itos older than the guard, whose usage error Claude Code would take as
  a block) becomes 1, which Claude Code reports and lets the command run; the
  guard itself never exits 2.
  The plugin has no skill (T-092): how to work with itos is itos's own guides,
  `itos go` and `itos guide work` (`internal/guide/`), versioned with the
  binary, so no flow needs the plugin. The module and tests are typed by the
  declarations Claude Code writes beside a plugin it loads
  (`.claude-plugin/types/`, git-ignored, which the plugin's `tsconfig.json`
  extends), so the repository's type-aware lint leaves the plugin out;
  `claude plugin validate --strict` and `claude plugin test` judge it, in
  T-066's checks. The formatter and the audit read it, the audit told that
  `hooks.json` loads the module and that `claude-code` is the engine's.
- `go.mod` is the module `github.com/donvargax/itos/v5` (from v2.0.0 a
  module path ends in its major version, or Go refuses its tag, so the release
  cut, `tools/bin/release-version`, refuses a version whose major the path does
  not match: the path moves first, T-089), Go pinned by its
  `toolchain` line; its dependencies are godog's and `go.yaml.in/yaml/v3`,
  itos's one. itos is `cmd/itos` and `internal/` ("The code" below).

## The features

What itos does is Gherkin in `features/`, run by
[godog](https://github.com/cucumber/godog) through `go test ./features`:
`TestFeatures` (`features_test.go`) runs every feature file, each scenario a
subtest named after it. `features/README.md` holds the rules: the black-box
boundary, the ID scheme, the tags, the smoke set, the moving rule.

- **The steps** (`steps_test.go`) treat itos as a black box. Each scenario
  builds a scratch git repository in a temporary folder, writes its
  `itos.yaml` and ledger, runs the binary `ITOS_BIN` names
  (`tools/bin/itos` by default, the Go binary built from the tree, relative
  to the module's root) in it, and
  asserts the exit code, the output and the files it leaves. A command runs
  in a clean environment: the caller's, less `GIT_*`, `ITOS_*`, `GITHUB_*`
  and `CI`, with no global or system git config and a fixed identity, so a
  run inside a git hook or on a runner sees what it sees locally. Its
  `PATH` is the caller's with no `claude` on it (`callerPath`, slice 49):
  each folder holding one is replaced, once a run, by a folder of links to
  the rest of it, as the corpus's `hide` does, so no scenario reaches the
  Claude Code of the machine it runs on; a scenario's own `claude` goes
  first. A step
  never reads itos's code, so the same steps judged the TypeScript and the
  Go port alike.
- **The release server** (`release_test.go`), for the launcher's scenarios
  (`pin.feature`): an `httptest` server inside the test, logging every path
  asked of it, offering fake releases as real ones are laid out
  (`/download/v<version>/<asset>`, and the last version it is given under
  `/latest/download/<asset>`, as GitHub serves its latest release): the
  archive for the running platform,
  whose `itos` is a shell script that appends its version, the
  `ITOS_VERSION` it ran with and its arguments to a file of the scenario's
  and exits with the code the scenario chose, and a real `checksums.txt`.
  Steps replace a release's `checksums.txt` or archive after the config pins
  it, or make the server unreachable (itos is then given the address nothing
  answers on, without `ITOS_NO_UPDATE`). The launcher's daily state in the
  cache is never read by a step: a scenario tells "once a day" by what the
  server was asked and what ran since the last run. Every command's environment sets `ITOS_CACHE` to a folder of the
  scenario's and `ITOS_RELEASES` to that server, or, with none, to an address
  nothing answers on beside `ITOS_NO_UPDATE=1`, so no scenario reaches the
  network or a real cache. The conformance runner does the same for every
  case (`cleanEnv` in `run.ts`); a corpus config pins nothing, or the
  binary's own version.
- **Programs that are shell scripts** (`program_test.go`): a scenario's fake
  programs (an extension, a release's `itos`, the config's recording shell,
  the probe that asks the itos under test for its binary) are shell scripts,
  which Linux and macOS run by their `#!` line. Windows finds a program by its
  extension and runs no script, so there `writeProgram` writes `<name>.exe`
  instead: `script-exe` (`features/testdata/script-exe`, built once a run, out
  of every `./...`) with the script after a marker line, which reads its own
  file, writes the script to a temporary file and runs it with Git for
  Windows' `sh`, handing on its arguments, streams, environment and exit
  code. The shim's link is `git.exe` there, as `itos git-shim install` names
  it (T-072).
- **The extensions** (`extensions_test.go`, `extensions.feature`): each is a
  shell script `itos-<name>` in a folder of the scenario's that `env` puts
  first on the `PATH` of every command, so it reaches the itos a scenario
  runs and nothing else. It records its arguments one a line, its folder
  (`pwd -P`) and the `ITOS_*` variables it saw in a folder of its own,
  replaced on each run, then exits with the code the scenario chose or runs
  the command it was given (a call back through `$ITOS_BIN`) and records its
  output. Since the steps strip the caller's `ITOS_*`, every one an
  extension sees is itos's or the launcher's that the steps set.
- **The stealth mode's scenarios** (`stealth_test.go`, `stealth.feature`):
  the world's `dataDir` is where `writeConfig` and `startingFiles` put the
  config, the ledger, the registry and the people, the root or `.git/itos`,
  and the README stays in the root. A linked worktree is added after the
  scratch repository moves into a folder of its own under the scenario's
  support folder, so a path beside it, `../wt`, is the scenario's alone and
  goes with it; `itosIn` runs itos there. A hook declared in the git config
  is read back as git reads it (`git config --get-regexp` for the entry
  whose event it is, then `git hook list` naming it, so git runs it). The
  project's hooks are a committed folder `core.hooksPath` names, whose
  commit-msg hook records in the support folder that it ran; the step that
  installs itos's hooks sets `hooks.bin` to `itos` (as a stealth config
  has it whatever it says, slice 35) and puts a script of that
  name on the scenario's `PATH` that runs the itos under test (a link would
  not do: `tools/bin/itos` finds its checkout from its own path), and runs
  it once, so a hook that cannot start does not pass for one that refuses.
  That `PATH` folder is `binOnPath`'s, the support folder's `bin` put first
  once for every stand-in a scenario writes there; "no identity can be
  looked up" writes a signed-out `gh` in it, the scratch config naming no
  `work.identity`, so a `work` that asked anyone would find nobody
  (slice 38).
  A remote is a bare repository in the support folder that the scratch
  repository pushes its branch to with `origin` set, so `--remotes` sees it
  and the itos notes stay behind; a task's check in a stealth ledger is
  written in the git folder and not staged.
- **itos init's scenarios** (`init_test.go`, `init.feature`): the scratch
  repository starts with nothing of itos (a README and one commit), or as a
  folder with its `.git` removed; the config init wrote is read back as YAML
  wherever it is, `itos.yaml` in the root or the stealth one in `.git/itos`.
  A scenario with the step "no file changed since the last run" records
  every file of the scratch folder, the git folder's included, as its mode
  and text before each run of itos (`markRun`), and the step compares the
  folder after it, so a rerun that wrote anything, the index included,
  fails. Claude Code is a fake `claude` in `binOnPath`'s folder (slice 49),
  a script that appends each run's arguments to a file of the support
  folder, a tab after each, answers `plugin list --json` with no plugin or
  the one the scenario names, and for "writing .claude/settings.local.json
  as claude does" writes that file on a local install; "claude was given"
  matches a run whose arguments begin with the words given. The git shim's
  link is checked by shim.feature's `"…" runs itos`, the file at the path
  (`.exe` on windows) the same file as the itos binary under test.
- **itos commit's scenarios** (`commit_test.go`, `commit-command.feature`):
  the commit-msg hook is a shim in git's own hooks folder
  (`git rev-parse --git-path hooks`) that runs the itos under test, so a
  commit made through itos meets the hook any commit meets. The arguments
  are split as `sh` would split them (whitespace, single quotes), and a
  footer of HEAD is read back as git reads its trailers
  (`%(trailers:only,unfold)`), so a footer that landed in the body does not
  pass. Under a stealth config the footers are read from HEAD's note
  (`git notes --ref=refs/notes/itos show`), and `git commits` and
  `git amends HEAD` run git itself, with the hook, through the world's
  `run`, as `itos` runs the binary; an amend has to make a new HEAD, so a
  refused one fails its step rather than leaving the old commit's note to
  pass. The scratch config has a `Scenarios:` footer whenever it has a kind
  of named tests, required for no type, so the scenarios that name none are
  judged as before. A feature file a scenario writes is staged, so the feat
  committed next adds it, as a feat adds the scenario it names. A footer
  counted once is read from HEAD as it is, not from a new HEAD: an amend
  that writes nothing new in its commit's second is that commit again.
- **itos push's scenarios** (`push_test.go`, `push.feature`): the remote is
  a bare clone of the scratch repository in the support folder
  (`origin.git`), and "a clone of it, where itos runs" clones that, so its
  main tracks the remote's, lays in the files the scratch repository has not
  committed (`layUncommitted`, which the one-commit-deep clone shares), and
  moves the world's `dir` there. The remote gains a commit from another
  clone in the support folder, pushed with `--no-verify`; a commit of the
  clone or the remote writes its file as one line naming the commit, so two
  touching the same file conflict. git config settings go in the clone's own
  config, the steps' environment reading no global one. The remote's branch
  is read in the bare repository itself (`git log --format=%s main`,
  `git rev-list --merges main`), so what was pushed is what the remote has,
  not what the clone thinks it pushed; an uncommitted change is a known line
  appended to the file, read back with `git diff HEAD` still seeing it. Two
  takes of one item (slice 66) edit the item's one line of the registry as
  the work steps write it, its owner and status, so they conflict: the
  remote's first pushes the clone's registry to the remote, so both start
  from it, and the remote's registry is read with `git show main:<registry>`
  in the bare repository.
- **The fake GitHub** (`range_test.go`, for `ci.feature`'s range, `status.feature`
  and `watch.feature`, bugs 23, 24 and 29, slice 85): an `httptest` server
  itos is pointed at through `GITHUB_API_URL`, given `GITHUB_REPOSITORY` and
  `GITHUB_TOKEN` as Actions gives them, started by the first step that asks
  for it and closed after the scenario. It holds runs of several workflows,
  each on a branch, and answers a workflow's runs (`head_sha` and `branch`
  narrowing them, newest first, or a stale list a step sets) and a run's
  jobs; a step can make it refuse the token, every request a 401.
- **The CI watch's scenarios** (`watch_test.go`, `watch.feature`): the
  clone of push's scenarios, its config's `ci.watch` the github provider of
  ci.yml with no interval, committed and pushed to the remote's main with the
  hooks left out whenever a step changes it, so the clone holds no
  uncommitted change for itos push to refuse. The fake GitHub answers a
  commit of the watched workflow that has no run of its own with the watched
  run, one look a request, the nth request the nth look and the last look
  for every request after, recording the commit each look was given, so
  "one poll later" is the second look and "never asked" is no request at
  all. "No gh on the PATH" is `pathHiding`, the caller's PATH with gh hidden
  as claude always is, and the environment drops every `GH_*` variable
  beside `GITHUB_*`; "no GitHub token" takes the fake's `GITHUB_TOKEN` out,
  so no token reaches the scenario. Until v5.0.0 (slice 85) the watch was a
  command provider running a script of the scenario's.

- **The git shim's scenarios** (`shim_test.go`, `shim.feature`): the link
  is to the binary the itos under test runs, never `tools/bin/itos`, a
  script that finds its checkout from its own path; a throwaway extension in
  a temporary folder prints the `ITOS_BIN` itos tells it, asked once per
  run. The link lives in the support folder's `shim/`, which `env` puts
  first on every command's `PATH` (`pathFirst`, before the extensions'
  folder), and "git runs" runs the link itself, so the shim finds the real
  git after it. The repository with no itos config is `plain/` in the
  support folder, outside the scratch one. The `PATH` is only ever a
  command's own environment and the link goes with the support folder, so
  no step leaves a shim on the `PATH` of anything else; `env` strips the
  caller's `ITOS_*`, `ITOS_GIT` among them, so a run under itos (a hook's
  `go test`) does not have the shim pass every git straight through.
- **The guard's scenarios** (`guard_test.go`, `guard.feature`): the steps
  write Claude Code's PreToolUse input themselves, as its documentation
  shows it (`session_id`, `cwd`, `hook_event_name`, `tool_name`,
  `tool_input`, `tool_use_id`), and run `itos hook pre-tool-use` in the
  folder the input names, with the input on stdin (`runWith`, which `run`
  calls with none). A deny is read as Claude Code reads it, stdout's JSON;
  "writes nothing" is an empty stdout. The repository with no itos config is
  the shim's `plain/`.
- **The header lint**, where a scenario needs one, is itos's built-in one,
  `use: builtin` in the scratch config, which needs nothing installed and
  holds no footer rule, so a scenario's footer rules are itos's own. The
  step that names commitlint's conventional config writes it too: it ran
  this checkout's commitlint until commitlint left (T-063).
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
the commands, `itos <command> --help` each one). The code is Go, `cmd/itos`
and `internal/` ("The code" below). The mechanisms below were first built in
the TypeScript v0, and where one names a `.ts` module or a function in
camelCase it is that module's; "The code" names the Go package that ports
each, and holds it since the TypeScript left (T-062).

- **One policy file.** `itos.yaml` at the root holds every table the tool
  reads: the ledger's layout (`ledger`), the commit types, footers, path sets
  and scopes (`commits`), the named tests and their adapters (`tests`), CI's
  steps, costs, prose shortcut, nightly and range (`ci`), the work registry
  and the people (`work`), and the hooks (`hooks`), and where verification
  starts (`commits.since`). `tools/itos/config.ts`
  loads it and rejects a key it does not know, naming the one it misspells;
  `ITOS_CONFIG` or `--config` names another file. A project changes its
  policy here, not in the code, and every key accepted is one a tool reads:
  a key waiting on a feature not built yet (`commits.header_lint.alongside`,
  a second way to tell a commit is pushed) is rejected until that feature
  reads it; `commits.header_lint.use` was, until the built-in lint gave it a
  second value. A key or a value a major release removed is refused as
  removed, not as unknown (`removedKeys` and `removedValues` in
  `internal/config/schema.go`, rule `config-removed`), its message saying
  what to write instead, since config check prints the message alone: v5.0.0
  removed the command providers of `ci.range`, `ci.watch` and
  `work.identity`, which ran a repository's own commands unasked, and
  `ci.range.github.branch` (slice 85); `config get` names such a key as
  removed too. **The defaults are one table**,
  `DEFAULTS` in `config.ts`: the loader lays the file over it
  (`withDefaults`, `tests.<kind>` under each kind the file has), and
  `config check --print-defaults` prints it, so no tool writes a fallback of
  its own and a default is applied exactly when it is printed. Some defaults
  depend on the config: `work.registry` is beside the config's ledger, and a
  stealth config's `hooks.bin` is `itos` with no `work.people` (slice 35), so
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
  (`tests.scenario.smoke`), the decision records in `work.decisions` (below,
  under ask). `itos config check` validates all of them, the
  people as a warning that never fails it: a project whose people file is
  missing or unreadable goes on without one everywhere else (slice 35).
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
  The five pattern keys (`ledger.id`, `ledger.group.pattern`,
  `tests.<kind>.id`, `ci.cost.static`, `ci.covers[].matches`) are RE2, the
  Go binary's dialect, and `config check` refuses one RE2 cannot compile
  (`config-regexp`). While the TypeScript lasted it ran them with
  JavaScript's `RegExp` and refused beside that what RE2 cannot compile, so
  a config that passed one implementation passed the other.
- **One command line** (`internal/cli`): exit 0 on success, 1 for a
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
- **Named tests behind an adapter** (`internal/tests`). A kind of named
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
  `commit-scope.ts`'s rejection beside the path rules), or for an amend
  HEAD's parent against it (bug 6, below), `verify` each commit
  of its range against its parent, the empty tree for a root commit
  (`commitMoveIssues`, in that commit's rejection, so it counts against the
  commit), and `itos tests moves <kind>` HEAD against the index by hand. Both
  hook and verify skip a type `except_types` names, and one that is not in
  `commits.types` when the config lists them (a merge's "Merge …"); the
  command ranges wait for the type to have a path rule, the built-in does
  not. A tree's feature set is read once per run. On a kind whose adapter
  is a command (slice 82) the rule reads the adapter's listing at both ends
  instead, so it needs `supports_at: true`: no live test added or lost, none
  switched between live and wip, and a live test's title, when the adapter
  gives one at both ends, kept unless `allowed_renames` lists the new one;
  bodies are not compared, since the adapter lists none. `config check`
  refuses `builtin` beside a command, on a command kind without
  `supports_at: true` or a kind whose named adapter is not `gherkin`, and
  `allowed_renames` without it.
- **The footers** have one reader (`footers.ts`), which the footer rules
  and CI share: which types need each footer, which IDs must exist, and
  `read_at: commit`, which reads the IDs that exist (the ledger's tasks, the
  live scenarios) at the commit being checked. The footer rules are itos's
  whatever the header lint: `commit.ts` runs them after the delegate, if
  any (the Go binary, after the built-in lint too), in the commit-msg hook, `commit check-message` and `verify`, and
  reports both, one list of problems under `--json`. Nothing reads a footer
  by its key: CI's tasks (`tasksIn` in `ci-scope.ts`) and the commit-msg
  hook's are those of every footer whose `source` is `ledger`, a kind's named
  tests those of every footer whose source is that kind, so `Task:` and
  `Scenarios:` are only this repository's names for them. A footer whose
  source is `registry` (slice 63) names work items, read from the registry
  file at the tree its `read_at` says (`work.IDsAt`; under a stealth config
  the file in the git folder, as the ledger is); its `in_place_of` maps
  another footer of IDs to the types it stands in for it (`stoodIn`, which
  both the rule and `itos commit`'s refusal before git runs ask), and
  `work show` reads its IDs as links to the item, as it reads the ledger's. What itos prints
  names the ledger's folder by `ledger.files` (`ledgerLayout`) and the prose
  steps by `ci.prose.steps`, never `tasks/` or `vp check`. A footer whose
  source is `text` (slice 26) has no IDs and so no source to read: each line
  `<Key>: <text>` is one footer, whatever a consumer must do or `none`, and
  its rule is only that one says something (`required_for`) and none is empty
  (`validate_for`); CI's plan reads no text footer, and `itos commit footers`
  gathers a range's for a release's notes, leaving out `none`. Any footer may
  name its own `since`, which verify applies to its `required_for` as it
  applies `commits.since` to the whole range (below).
- **One shell** (`shell.ts`): every command itos takes from the config or the
  ledger (a task check, a CI step, a header-lint delegate, a range check, a
  provider's or a command adapter's command, the pre-push commands, the smoke
  run) starts through `inShell`, which appends it to `shell` (`[sh, -c]` by
  default) as one argument. itos's own `git` calls start directly.
- **The world outside the repository** is three providers
  (`providers.ts`): where a push's range starts (`ci.range`: the last green
  run on GitHub, a command, or none), who a session works for
  (`work.identity`: `gh api user`, a command, or only `--as`; never asked
  under a stealth config, whose session owns every item), and who works
  on the project (`work.people`).
- **Where verification starts** (`commits.since`, `repo.ts`): `verify`, and
  with it the built-in moves rule, lists a range's commits less that commit
  and its ancestors, and a range command's `{from}` is that commit when the range's own
  start is empty or older. `config check` and `verify` fail with exit 2 when
  the repository does not have it; in a shallow clone (`git rev-parse
--is-shallow-repository`, actions/checkout's default one commit deep) the
  same problem says the commit may lie beyond the clone's history, with
  `git fetch --unshallow` or `fetch-depth: 0` as its fix. The commit-msg hook
  never reads it. A footer's own `since` (`commits.footers.<name>.since`) is
  the same idea for one rule: verify leaves that commit and its ancestors out
  of the footer's `required_for` (`config.Before`), so a footer a project
  requires later does not fail the history written before it, while the
  commit-msg hook and `commit check-message`, which judge the commit being
  made, always require it. config check and verify hold it to the same checks
  as `commits.since` (`SinceIssues`, one problem per key).
- **Conformance** (`tools/itos/conformance/`): what itos does, as cases run
  through its command line in scratch repositories, one file per area, every
  `--help` text among them, since agents read them. `run.ts --bin <command>`
  runs them through any binary; a tampered case file must fail, so the
  runner compares what it claims to. It is a regression corpus, run by a CI
  step of its own: a change to what itos does lands with its scenario in
  `features/` and, where a case records the old behaviour, the case. A
  case's `github` key starts a fake GitHub API in the runner for it alone
  (slice 85), whose runs answer for a commit or for any commit (`*`), with a
  sequence of looks for a run going on, so the github providers of
  `ci.range`, `ci.watch` and `status` have cases without a network; a fake
  `gh` on the case's PATH gives `work.identity` its person.
  `tools/itos/fixtures/` holds the configs and ledgers the
  negative proofs in `tasks/phase-0.yaml` run against.

## The code

The Go build of itos landed beside the TypeScript one command group at a time
(`docs/decisions/0017-the-go-port-s-proof-is-its-tasks-checks-landed-as-refactor-commits.md`),
until it passed everything and the TypeScript went
(T-062). Each package below says which TypeScript module it ports, so the
mechanisms above, written against those modules, read across.

- **The packages.** `cmd/itos` is the binary, a call to `internal/cli`.
  `internal/cli` is `main.ts`: the global flags wherever they stand, the
  command table with every command's argument errors, and how a failure is
  reported and which exit code it takes; `configcheck.go` in it is
  `config-check.ts`. `internal/version` is `version.ts`, and `internal/out`
  prints a `--json` object with `"schema": 1` first and its keys in the order
  written (Go's maps would sort them). The rest are the packages under `internal/`,
  each holding what the groups ported so far need: `internal/ledger`
  (the layout, the files, every task and check problem, and the tasks and
  checks typed, and the task IDs at a tree), `internal/tests` (the Gherkin
  adapter and the command adapter at a tree, the smoke set and its rule,
  the run templates, the moves rule; below),
  `internal/message` (below), `internal/plan` (CI's plan; below),
  `internal/providers` (the range, watch and identity providers and the people;
  below), `internal/work` (the registry and its problems, the items'
  statuses, the proposal, the edits that take, promote, close, add,
  change and drop an item, and the item as work show prints it;
  below), `internal/follow` (itos follow's threads; below), `internal/ask`
  (itos ask's questions; below), `internal/adr` (the decision records itos
  ask record writes; below), `internal/nextid` (the next free ID of a
  series; below), `internal/guide` (the guides itos go and
  itos guide print; below), `internal/lock`
  (the lock file a writer of shared data holds; below),
  `internal/shell`, `internal/check`, `internal/glob`, `internal/scope`
  (below) and `internal/git` (the repository's state and ranges, read
  through the real git; below), beside four
  the TypeScript has no module for:
  `internal/value`, `internal/source`, `internal/shim` and
  `internal/release` (below). The port
  shells out to git where it needs it, as the TypeScript did, but always to
  `git.Bin`, never to a `git` looked up on the `PATH`, which may be itos.
  Every list of paths it reads from git (`diff --name-only`, `diff-tree`,
  `ls-files`, `ls-tree`, `log --name-only`, `status --porcelain`) is read
  NUL-separated, with `-z` (`git.Paths`, bug 31), so a path git would
  C-quote is matched as it is named; a merge's own paths, read from the
  headers of its combined diff, are unquoted. Every list of changed paths
  (the hook's staged paths, verify's commit paths, the CI plan's range and
  the unpushed commits' paths) is read with `--no-renames` and no diff
  filter that drops a change type (bug 32), so a rename is its old path's
  deletion and its new path's addition, a type change (T) is listed, and the
  hook, verify and the plan agree on what a commit touches; only the
  conflicted paths select, with `--diff-filter=U`.
- **The launcher** (`internal/launch`, slice 27) runs before the command
  line: `cmd/itos` calls `launch.Main`, and only when it hands the run back
  `cli.Main`. It picks the version to run, `ITOS_VERSION` when set, else the
  config's `pin.version`, and a binary whose own version that is runs itself,
  so the version the launcher runs (to which it passes `ITOS_VERSION`) never
  launches again. It reads the config where `cli` does (`--config`, else
  `config.Locate` under `--root`, or with neither under the top `config.Top`
  finds from a subfolder: `ITOS_CONFIG`, `itos.yaml` or the stealth
  config), from the working tree, and only its `pin`,
  so a config written for a newer itos still reaches the version it pins;
  `readConfig` tells three states apart: `pinned`, `absent` (no file there at
  all, or the stealth config with no `pin` key, one person's itos in a
  repository that does not use it, which keeps to the newest release as
  where there is none) and `unpinned` (a project's config with no pin, one it cannot read, or one whose
  pin fails `config.PinVersion` and `config.PinChecksums`, the patterns config
  check holds the keys to), which runs the binary that was called. Otherwise `ensure`
  finds `<cache>/<version>/itos` (`ITOS_CACHE`, else `itos/` under
  `os.UserCacheDir`), cached when present and, under a pin, when the
  `checksums.txt` beside it hashes to `pin.checksums`; else `fetch` gets
  `<base>/download/v<version>/checksums.txt` (`ITOS_RELEASES`, else
  GitHub's releases of itos; the addresses, the fetch and the hashing are
  `internal/release`'s, which the command line uses too), holds it to the pin (an `ITOS_VERSION` the
  config does not pin trusts it as fetched), takes the platform archive's
  line (`itos-<version>-<os>-<arch>.tar.gz`, `.zip` on windows), fetches and
  checks the archive, extracts the binary at its top and moves it with that
  `checksums.txt` into the cache as one folder, written beside it first. Any
  failure is one line on stderr and exit 3 (`cli.ExitMissing`), and nothing
  runs. `run` is `syscall.Exec` on unix, so the version run owns the
  process, its signals and its exit code, and a child whose exit code is
  passed back elsewhere. Before `ensure`, `unguarded` catches `hook
pre-tool-use` (`cli.Parse`'s rest) chosen for a version older than
  `cli.GuardSince` (`pretooluse.go`, the first itos with the guard), by any
  of `choose`'s branches: `answerNothing` reads stdin to its end (not a
  terminal's), says why in one stderr line and the run exits 0 with nothing
  on stdout, fetching nothing (slice 44); an `ITOS_VERSION` that is no
  version is handed on, to fail as for any command.
- **Keeping to the newest release** (`internal/launch/update.go`, slice 28)
  hangs off two branches of `choose`. `absent` calls `newest`, which runs the
  newest of the binary's own version and the stable releases the cache holds
  (`cachedVersions`: the `<version>` folders whose binary is there,
  pre-releases left out, as GitHub's latest release is never one), after
  `install`ing the release the server announces when the cache lacks it,
  checked against the announced `checksums.txt` (`fetch` is the pinned path:
  its `checksums.txt` held to the pin, then the same `install`). `pinned`
  calls `notice` before the pin runs, even when the pin is this binary.
  Both read `announced`: the newest version the server named, kept with the
  time it was had in `<cache>/state/latest` ("<unix seconds> <version>"),
  asked for again (`<base>/latest/download/checksums.txt`, its version read
  from the archive names, within `askTimeout`, three seconds) only when that
  is a day old and the run may ask (`CI` and `ITOS_NO_UPDATE` both unset). A
  question with no answer is written as asked, so an offline machine pays
  the timeout once a day. `notice` compares the announced version with the
  pin (`version.Compare`), and says it once a day per repository, keyed by
  the config's absolute path in `<cache>/state/notice-<hash>`, written
  before the line is said so a cache it cannot write to stays silent rather
  than saying it every run. Nothing here is ever an error: every failure
  falls through to what the cache has. A binary built without a version
  (`version.Unstamped`) runs itself where there is no config.
- **itos pin** (`internal/cli/pin.go`, `internal/value/edit.go`, slice 47)
  is the launcher's own command: `binaryCommand` leaves `pin` to the binary
  called, as it leaves `git-shim install`, since the version pinned may
  predate it, and so no notice is said either. It reads the config
  `config.Path` finds after `applyGlobals` (the stealth one included), asks
  `internal/release` for the version's `checksums.txt`, or for the newest's
  at `<base>/latest/download/checksums.txt` with the version read from its
  archive names (asked whatever `CI`, `ITOS_NO_UPDATE` or the daily state
  say, and not written to that state), within 30 seconds, and writes
  `pin.version` and that file's SHA-256 with `value.SetScalars`, which edits
  the text rather than re-encoding it: yaml/v3's nodes give each value's line
  and column, the token there is replaced in the style it is written in
  (plain kept plain unless the core schema would read the new string as
  something else), a missing key goes after the section's last one (a line
  in a block mapping, `, key: value` in a flow one), a missing section after
  the top-level `version` line, and an empty one (`pin:`, `~`, `null`, `{}`)
  below its key; a value over several lines, a tag, an anchor or an alias is
  refused, and so is any edit that does not parse back to the old document
  with only those values changed, so the file is never written wrong.
  Nothing is written when the fetch fails (exit 3), when the pin is already
  that version with those checksums (exit 0), or when it is that version
  with other valid checksums (exit 1: the release changed after it was
  pinned, and re-pinning it quietly would defeat the pin).
- **itos upgrade** (`internal/cli/upgrade.go`, slice 75) is the launcher's
  own command too (`binaryCommand`), and moves the pin with pin's code: the
  config and its pin read by `readConfigPin`, the release by `pinned`, the
  edit by `configPin.moved` and the refusal of a release changed after it
  was pinned by `configPin.replaced`. The version it moves from is the pin's,
  else the `version=` line of `tools/bin/install-itos`, the install script
  the notes write; with neither, or a version older than that, it refuses
  (exit 1) and names `itos pin`. It walks back from the new release through
  each `upgrading.json`'s `previous` (T-091) until the old version, asking
  `internal/release` for each; `release.Get` reports an answer other than
  200 OK as a `StatusError`, so a 404 (`release.NotFound`), a release cut
  before T-091, ends the walk naming its notes, while any other failure
  exits 3. A file of a schema it does not read is named by its notes too.
  Every edit is made in memory first, the pin, the config's first line when
  it is a `yaml-language-server` schema line naming a release's
  `itos.schema.json` (only its version changes), and the script's
  `version=` value and each `platform=<os-arch> sum=<hash>` hash, looked up
  in the new `checksums.txt` by `release.Listed` (the launcher's reader of
  that file); a layout it cannot edit so exits 2. Only then are the files
  written, so a failed fetch or edit leaves every file as it was. It prints
  each release's lists oldest first, or under `--json` returns them as
  `upgrading.json` holds them, and commits nothing.
- **itos init** (`internal/cli/init.go`, `starter.go`, slice 48) is the
  launcher's own command too (`binaryCommand`): where there is no config
  there is no pin to hand it to, and the newest release must not run in its
  place. It moves to the repository's top (`git rev-parse --show-toplevel`;
  `applyGlobals` moves there only when a config is at the top), after a
  `git init` where `git rev-parse --git-dir` finds no repository, before
  anything asks `config.Path`, whose stealth lookup caches the git common
  dir per folder. With a config there (`config.Path` names a file that
  exists, the project's or the stealth one) it writes nothing: `initReport`
  lists `configFindings`' problems and `hookProblems`', for the manager
  `chosenManager` gives (a shim file missing, not calling itos or, for
  plain git, not executable; a lefthook or pre-commit config without
  itos's lines; a git config entry missing, the pre-push one only when
  `declaresPrePush` says `declareHooks` writes it), exit 1 when there is
  any; then its notes, never counted as missing (slice 50), under their
  own heading and in the `checked` object's `notes`: `configFindings`'
  warnings (the people file's) and `pinBehind`, a pin older than the
  newest release by `version.Compare`, asked of the release server as
  `pinned("")` asks it, and left out when it does not answer. Else `initWrite` asks
  `pinned("")`, pin's own question for the newest release, renders the
  starter (`starter.config`, a template of commented YAML, not
  `value.YAML`, so it reads as a person's file), writes it, the ledger and
  the registry, never over a file that is there, and, when `features/`
  holds a `.feature` file, loads the config just written and lists the
  scenario kind's tests through the adapter to name each file's first live
  one in the smoke set, so the smoke rule and init cannot disagree; where
  the list has no test at all (no scenario carries an ID tag, slice 50), it
  writes the config again with `starter.untagged`, whose `Scenarios` footer
  is required of no type, and says how to tag one. Then
  `hooksInstall`, as `hooks install` runs, its exit code init's; under
  `--json` its object is captured and nested as `hooks`, its keys in the
  order it wrote them. `--stealth` puts the config at
  `<git common dir>/itos/itos.yaml` and its files beside it, and
  `hooksInstall` picks the git config for a stealth config by itself.
  ITOS_CONFIG naming a file that does not exist is a usage error: init
  writes only the two configs itos finds by itself. Last, both ways, it
  makes the plugin's offer (`initplugin.go`, slice 49), `pluginOffer.run`:
  `--plugin no` ends it, a `claude` the `PATH` lacks is said only for
  `--plugin`, `claude plugin list --json` read as a list whose `itos@itos`
  entry, enabled where init runs, means installed; then the scope is the
  flag's (a bare one `pluginDefault`'s), or the terminal's answer
  (`question`, only on the first run, without `--json`, where stdin and
  stdout are both a character device other than the null device), or none,
  when it says how. It runs `marketplace add`, whose failure alone is not
  one (the marketplace may be known), then `install`, whose failure is
  printed with claude's output and exits 1; a local install under
  `--stealth` lists `.claude/settings.local.json` in
  `git rev-parse --git-path info/exclude` when `git status` shows it
  untracked. `--stealth --plugin project` is a usage error before
  anything runs, and so is `--plugin project` where the config found is
  the stealth one. After the plugin's, the git shim's offer
  (`initshim.go`, slice 50), `shimOffer.run`: `--no-git-shim` ends it;
  `shimPlace` gives the link `git-shim install` would make (its `--dir`
  `--git-shim-dir`, made absolute where it was typed), unless, with no
  folder named, the first git on the `PATH` already links this itos, which
  is then the link; a link to this itos is kept, a git that is no link to
  itos refused for `--git-shim` (exit 1, as `git-shim install` refuses it)
  and otherwise only said, a link to another itos replaced; then, with no
  flag, the terminal's yes or no (`question`, sharing one buffered reader of
  stdin with the plugin's, so neither takes the other's answer), or none,
  when it says how; then `makeLink`, and `standingLine`. Run again where a
  config is, neither offer asks: each says how, or installs for its flag,
  and the `checked` object always holds `notes` and `git_shim`. These
  additions to the report passed the last release's corpus because T-076
  judges an old case's output additively, a line or a key added passing.
  Last, the offer of the rules for agents (`initrules.go`, slice 59),
  `rulesOffer.run`, after the shim's so the lines the old corpus pins stay
  adjacent: `rulesBlock` renders the loaded config (`commits.types`, the
  header lint, each footer's source and `required_for`, with `in_place_of`
  as an alternative, `commits.scopes` with its `$sets` already expanded, the
  hooks' rules and `hooks.pre_push`, `ci.steps` in `check.CostOf`'s order,
  the prose steps and the nightly's) as Markdown a formatter leaves alone:
  one line a paragraph or list item, a blank line between blocks, every
  config value through `code`, a one-line code span fenced past any backtick
  it holds. `splitBlock` cuts `AGENTS.md` around the lines
  `<!-- itos:begin -->` and `<!-- itos:end -->`, refusing a lone or
  misordered marker or a second block; `writeRules` replaces that block, or
  appends one after a blank line, and gives `CLAUDE.md` an `@AGENTS.md` line
  unless it has one or is `AGENTS.md` itself; both files are read with CRLF
  made LF and written back with the line endings they had. The first run
  without a flag asks a terminal (sharing the one reader of stdin) or says
  how; a rerun with no flag reads no config unless a block is there, and
  then compares it with `rulesBlock`'s, saying only that it is stale.
  Under a stealth config `runStealth` (`initrulesstealth.go`, slice 80)
  makes the same offer and writes three files, never `AGENTS.md` or
  `CLAUDE.md`: the block to `<git common dir>/itos/AGENTS.md`, a
  `CLAUDE.local.md` whose block imports `@AGENTS.md` when the project has
  one and then that file, by the path `git rev-parse --git-common-dir`
  gives (relative in the main worktree, absolute in a linked one), and an
  `AGENTS.override.md` holding a copy of `AGENTS.md` between
  `<!-- itos:agents-md:begin -->` and `<!-- itos:agents-md:end -->`, then the
  block. `blockSpan`, which `splitBlock` now calls, finds a block's marker
  lines while skipping another's, so the rules markers are looked for
  outside the copy. An `AGENTS.override.md` is itos's, and gets the copy,
  when it is missing, holds the copy's markers or holds nothing outside the
  rules block; any other is the person's and only gains the block, as a
  `CLAUDE.local.md` always does. `plan` computes each file's next text, so a
  rerun compares texts and says whether the rules or the copy are stale; a
  rewrite refuses a file at the top that the project tracks, and lists both
  files with `excludeIfShown`, the plugin's, which adds a line only while git
  shows the file. The shared block names the config and its ledger and
  registry by their place beside it (`stealthSource`, `stealthData`), since
  their paths differ between worktrees and the block must not.
- **Extensions** (`internal/cli/extension.go`, slice 29): a command itos
  does not have runs `itos-<command>` from the `PATH`, as git runs
  `git-<command>`. `cli.Parse` reads the arguments for both `cli.Main` and
  the launcher's `readConfig`: it finds the command's name (`commandAt`, the
  first argument that is neither a global flag nor a valued one's value),
  and when `extensionPath` finds a program for it, only the global flags
  before the name are read and `Globals.Extension` holds the program; else
  it is `ParseGlobals` as before, global flags anywhere, so a command that
  is neither built in nor found is still `unknown command` and
  `itos bogus --help` still the main help. `extensionPath` refuses a
  built-in (`builtin`: the command table and `help`, which always win), a
  name that is empty, starts with `-` or holds a path separator, so nothing
  but the `PATH` is searched, and takes `exec.LookPath`'s answer, which
  skips relative `PATH` folders (`exec.ErrDot`). After `applyGlobals` (the
  `chdir` of `--root`, or to the top from a subfolder), `runExtension` gives the program the rest of the
  arguments unread, or `--help` alone when a `--help` came before the name,
  in `extensionEnv`: itos's environment with `ITOS_CONFIG` (`config.Path()`
  made absolute), `ITOS_ROOT` (the working folder), `ITOS_BIN`
  (`os.Executable`), `ITOS_VERSION` (`version.Version()`, which the
  launcher reads, so `$ITOS_BIN` called back runs itself and never launches
  another version) and, with `--json`, `ITOS_JSON=1`, set in place of any of
  them the environment had. `runProgram` is `syscall.Exec` on unix, as the
  launcher's `run`, and a child whose exit code is passed back elsewhere; a
  program that cannot start exits 3. `help` runs `<program> --help` for
  `itos help <extension>`, and the main help lists `extensions()`: every
  `itos-<name>` in the `PATH`'s folders that `extensionPath` would run for
  `<name>` (so the first folder's, and no built-in), sorted, between the
  commands and the global flags, and nothing when there is none, so the
  corpus's main help case is the help with an empty `PATH` of extensions.
- **itos commit** (`internal/cli/gitcommit.go`, slices 31, 36, 37 and 58):
  `commit` in the command table hands its arguments to a subcommand when the
  first names one (`commitSubcommands`) and to `gitCommit` otherwise, which
  first loads the config (`commitConfig`: none, with no error, where there
  is no config file, so itos commit still runs git where itos is not set
  up). `readCommitFlags` takes itos's flags out of the arguments before any
  `--`, leaving the rest to git in order: `--task` and `--scenarios`
  (`--flag <ids>` or `--flag=<ids>`, repeatable, split by
  `message.SplitIDs` as the footer reader splits them), and the flags of
  the content (slice 36), each footer of free text's (`message.TextFlags`:
  `--` and its key in lower case, unless one of `message.BuiltinFlags` has
  that name) and `--breaking`, whose value is the next argument whatever
  it is, as git takes an option's. `footerLines` gives the links, each
  flag's footer found by its source (`message.LedgerFooter`,
  `message.TestsFooter`: the first of `commits.footers`) and its IDs packed
  by `message.FooterLines` onto lines within the header lint's 100
  characters, the key on each; and the content, `<Key>: <text>` in the
  config's order, then `BREAKING-CHANGE: <text>`, the form git reads as a
  trailer. `readGitArgs` reads git's own arguments as git reads them (a
  valued option's value skipped, `gitValued` and `gitShortValued`, so
  `-m --amend` is a message): the `-m` messages, the `-F` file, each with
  where its value sits (`valueAt`), `--amend` and `--no-edit`. `lacking` refuses up front what the hook would: from the
  message itos can read (`-m`, `-F <file>`, or HEAD's for
  `--amend --no-edit`; nothing for the editor's or `-F -`), its type's
  required footers that neither it nor the flags give
  (`message.Missing`), each named by its flag (`message.Flag`); under a
  stealth config the links are only the flags', or for an amend that gives
  none HEAD's note, as the hook will read them. `refuseCommit` reports them
  as a rejection, exit 1, before git runs. `wrapBody` (slice 58) then
  wraps the message to `message.BodyLimit`, the built-in lint's 100 and 0
  for no lint or a delegate: `message.Wrap` reads the `-m` paragraphs as
  one message, git's blank line between them, keeps the header, every line
  from the first the lint reads as a footer, comment and indented lines and
  lines within the limit, and breaks the rest greedily at spaces, a list
  item's continuation indented by its marker's width. A break whose next
  line would start with a footer token or a breaking-change note (the
  parser's `footerToken` and `noteLine`), a configured footer key and its
  colon, or the comment char moves back a word, and with none left the line
  runs over the limit (bug 15); each `-m` value is
  rewritten in its argument, and an `-F` text (stdin's for `-F -`) goes to
  a temporary file named in its place, removed once git exits. `trailers`
  makes each line a `--trailer`, which git applies before the editor and
  the commit-msg hook, under `-c trailer.ifExists=addIfDifferent`, so a
  footer the message
  already has, an amend's, is not written again. That is the one step the
  stealth mode changes (slice 32): under a stealth config
  (`config.IsStealth`) the links go to the hook in `ITOS_FOOTERS`
  (`message.FootersEnv`, any inherited one dropped first) and the content
  stays trailers, and once git exits 0 with a new HEAD, `writeNote` writes
  the links as its note in `refs/notes/itos` (`git notes add -f`, replacing
  what an amend carried over), while `rewriteNotes` first adds that ref to
  `notes.rewriteRef` in the local config unless a value (a glob too)
  already names it. git tells a hook nothing of an amend, so `gitCommit`
  sets `ITOS_AMEND` (`AmendEnv`) to 1 or 0 every time (slice 37), from
  `readGitArgs`. The hook's `amending` takes that word, and only when the
  variable is absent, a commit made without itos commit, guesses from the
  author git exports (`GIT_AUTHOR_NAME`, `_EMAIL`, `_DATE`), which an amend
  keeps from HEAD to the second; `withoutFooters` drops an inherited one
  and `checkEnv` keeps it from the task checks. `runGit` runs `git commit`
  as a child, not by `exec`, so itos can act after it, with the terminal's
  stdin for the editor and an interrupt left to git; its exit code is
  itos's, and one git cannot start exits 3. A global `-q` is passed on as
  git's `--quiet`; under `--json` git's stdout goes to stderr and stdout
  has `{"schema":1,"ok","commit"?}`, or the refusal's problems.
- **itos push** (`internal/cli/push.go`, slice 39): `push` in the command
  table first refuses every argument (`readPushArgs`): a force flag
  (`-f`, anything starting `--force`, a short cluster holding `f`) or a
  `+` refspec as `push never forces: …`, anything else as
  `push takes no arguments`, both usage errors. Then, outside a repository,
  exit 3. `ready` refuses (exit 1) a rebase in progress or a conflict left
  (`git.Rebasing`: a `rebase-merge` or `rebase-apply` folder at
  `git rev-parse --git-path`; `git.Conflicted`:
  `git diff --name-only --diff-filter=U`, docs/ORCHESTRATING.md's guard
  one-liner) and tracked changes (`git.Changed`: `git status --porcelain
--untracked-files=no`, listed); a detached HEAD (`git.Branch`) is refused
  next. `git.Upstream` gives `branch.<name>.remote` and `.merge`, else
  `origin` and `refs/heads/<name>`, and with no upstream a missing `origin`
  exits 3. `fetch` runs `git fetch --quiet --no-tags <remote> <ref>` with
  its stderr held back and takes `FETCH_HEAD^{commit}`; when it fails,
  `git ls-remote --exit-code` exiting 2 says the remote has no such branch
  yet (a new branch: no rebase, the push creates it), and anything else
  prints git's words and exits with git's code. Unless the fetched commit is
  already an ancestor of HEAD, `rebase` runs `git rebase --no-autostash
<sha>` (never `git pull`, so neither `pull.rebase`, `pull.ff` nor a fork
  point enters it), after `rewriteNotes` under a stealth config so the
  rebase carries the itos notes; afterwards `git.Rebasing` or
  `git.Conflicted` is the stop (exit 1, how to go on; when the work registry
  is among the conflicted files, `registryConflict` adds what happened in a
  person's words, slice 66: the file, each item both sides changed by
  `work.Clashes` over the registry at `REBASE_HEAD^`, `HEAD` and
  `REBASE_HEAD`, the upstream's owner of one it took, and `git rebase --skip`
  to give the item up, since two takes of one item meet there and the remote
  keeps the first), and a non-zero exit
  without either a rebase that did not start (exit 1). HEAD is then
  resolved to a full SHA once (`rev-parse HEAD^{commit}`, kept as
  `pushRun.sha`), and nothing after reads HEAD again (bug 21): git runs the
  pre-push hook after it resolves the refspec, so a commit made during the
  hook's unit tests moves HEAD without being pushed. That commit at or
  behind the fetched one (`rev-list --count <onto>..<sha>` of 0) is nothing
  to push, exit 0. `push` then runs `git push <remote> <sha>:<ref>`, an
  explicit refspec, so a configured push refspec (the notes') sends nothing
  and the pre-push hook runs as for any push; a refusal is reported with
  git's exit code. The success line's short SHA, `--json`'s `commit`, the
  registry-only range and the run waited for are all that SHA's. Each git runs through `runGit` with the terminal's stdin (a
  credential prompt), its stdout the terminal's, or stderr under `--json`,
  where stdout is `{"schema":1,"ok","outcome","remote"?,"branch"?,
"commit"?}`; a global `-q` passes `--quiet` to the rebase and the push and
  leaves out the success line. itos push reads no config until it has
  pushed, so it runs where itos is not set up, as itos commit does. After a
  push that sent something, unless `--no-wait` (`readPushArgs` takes it and
  refuses the rest), `wait` loads the config when there is one: with none, a
  config that cannot be read (said on stderr, the push's exit unchanged) or
  `ci.watch.provider: none` it reports as before; else it prints the push's
  line and hands the pushed SHA to `watchRun` (below), whose code is push's,
  and under `--json` adds its `ci` and `run` keys to push's object. Before
  that, `registryOnly` (slice 56) reads the paths of `onto..<sha>`, the
  commits the push added after the rebase (`git log --name-only
--no-renames --diff-merges=first-parent`): when there are some and every
  one is `work.registry`, push reports the push and a line naming `itos ci
watch <sha>`, exit 0, with no provider made; a push that made the branch
  has no `onto` and is never registry-only.
- **The git shim** (`internal/shim`, `internal/git/bin.go`,
  `internal/config/managed.go`, `internal/cli/gitshim.go`, slice 41):
  `cmd/itos` asks `shim.Named(os.Args[0])` first (base name `git`, or on
  windows `git`/`git.exe` in any case) and, when it is, `shim.Main` before
  anything else. The real git is `git.Inherited` (`ITOS_GIT`, unless it is
  an itos) or `git.Real`: the first `git` (on windows each `PATHEXT`
  extension) in the `PATH`'s absolute folders that is an executable regular
  file and not an itos (`git.IsItos`, bug 45): not `os.SameFile` with
  `os.Executable`, not a symbolic link whose target, at any link of the chain,
  is named `itos` (`itos.exe`), and not a Go binary whose build information
  (`debug/buildinfo`) names `cmd/itos` of the itos module at any major
  version, so this binary's shim and another itos's (the global one's, seen
  from a pinned release the launcher runs from its cache or a repository's
  own build), linked symbolically, hard or copied, are all skipped, by reading
  files and never by running one; none is exit 3. With `ITOS_GIT` inherited, a git
  started under an itos run, the shim passes through at once. Otherwise
  `parse` reads git's options before the command (`-C` joined as git joins
  them, `-c` kept, `--no-pager`/`-P` passed over, anything else not the
  shim's), and for `commit` or `push`, with no `GIT_DIR` or `GIT_WORK_TREE`,
  `config.Managed` decides by file checks alone: `ITOS_CONFIG` naming a file,
  else `itos.yaml` in the folder, else `WorkTree`'s top (the first folder up
  from the physical one with a `.git`, a `.git` file's `gitdir:` and that
  gitdir's `commondir` giving the common dir, and none inside a git folder,
  told by `HEAD`, `objects` and `refs`) holding `itos.yaml` or its common dir
  the stealth config; `managed_test.go` holds it to `Top` and `Locate` on
  each layout. Then `enter` `chdir`s to the `-C` folder, keeping the one to
  come back to, and `tooOld` asks `launch.Handed` the version the launcher
  would hand `git-shim run` to by name (`ITOS_VERSION` when it is a version,
  else the pin, read by `readConfig` as `choose` reads it, `config.Top`'s
  answer cached for the launcher's own read; never the newest release, and
  nothing when it is this binary's version): older than `cli.GitShimSince`
  (`gitshim.go`, the first itos with `git-shim`), the shim says so in one
  line, `chdir`s back and runs the real git with the arguments as they came
  (bug 7); a folder it cannot come back to hands the command on. Otherwise
  `configure` appends each `-c` to `GIT_CONFIG_COUNT`/`_KEY_<n>`/`_VALUE_<n>`
  and sets `ITOS_GIT`, and the
  arguments become `git-shim run -- <command> <args>…`, which go through
  `launch.Main` and `cli.Main` as any run's: the `--` keeps every git
  argument from `ParseGlobals`, and `git-shim run` calls `gitCommit` or
  `push` directly, so `git commit check-paths` stays a commit. Anything else
  is `run`: `syscall.Exec` of the real git with the arguments and the
  environment untouched on unix, a child with the terminal's streams, an
  interrupt left to it and its exit code handed back elsewhere. Every git
  itos itself starts (`git.Output`, `git.Succeeds`, `runGit`, the notes, the
  git config hooks, `source`'s `cat-file`) is `git.Bin()`: `Inherited`, else
  `Real`, else `git`, found again when `ITOS_GIT` or the `PATH` changes (a
  unit test swaps the `PATH`'s git); `cmd/itos` calls `git.Export` before the
  launcher, setting `ITOS_GIT` for all a run starts, the version the launcher
  runs included. `git-shim install` and `uninstall` (`gitshim.go`) link
  `os.Executable` as `git` (`git.exe`) in `--dir`, typed from where the
  person stood, or its own folder, a symbolic link or on windows a hard link
  where that fails; a git there that is this binary is kept, a symbolic link
  to another binary named `itos` replaced, anything else refused (exit 1);
  `pathStanding` places the folder and `git.Real`'s folder among the `PATH`'s
  by `os.SameFile`. The launcher's `binaryCommand` leaves both to the binary
  called, so a pin never links a cached version (and `pin`, slice 47, above).
- **The guard** (`internal/guard`, `internal/cli/pretooluse.go`, slice 42):
  `hook pre-tool-use` is dispatched with git's two hooks, and
  `hookPreToolUse` never returns an error, since `failure` would make it a
  usage error's exit 2, which Claude Code takes as a block: an input
  `guard.Read` refuses (not one JSON object, no string `tool_name`, a Bash
  input with no string `command`) is one stderr line and exit 1. The folder
  judged is the input's `cwd`, read from `typed(".")` when relative, else
  `typed(".")` itself, where itos was started before `applyGlobals` moved to
  the top. `guard.GitCalls` parses the command with `mvdan.cc/sh/v3/syntax`
  (bash) and walks every `CallExpr`, so a call inside a subshell, a compound
  command, a function's body or a command substitution is met in its turn;
  `literal` gives a word's text when it is fixed (unquoted with bash's
  backslashes taken out, single-quoted, double-quoted with no expansion),
  and the first word that is not fixed ends what the call can say.
  `gitCall` skips `command`, `exec`, `nohup` and `env` with env's
  `NAME=VALUE` words (the call's own assignments are the parser's, apart
  from its words), takes `git` or a path ending in `/git`, then git's global
  options (`-C` joined as git joins them, the options of `valued` passing
  over their next word, any other `-` word passed over) and the subcommand.
  `guard.Reason` asks `config.Managed`, the shim's file checks, of each
  call's folder and names each guarded subcommand once, in `Guarded`'s
  order; `guard.Deny` encodes Claude Code's `hookSpecificOutput`, with no
  HTML escaping so the reason reads as written. A command bash cannot parse
  gets no answer. The corpus (`hooks.yaml`) pins the deny's bytes. The
  launcher answers for the guard where it would hand it to an itos without
  one (`launch.unguarded`, above).
- **The config** (`internal/config`) is `config.ts`'s loader, and every Go
  reader of the config goes through it. It finds the file (`--config`,
  `ITOS_CONFIG`, else `itos.yaml`, else the stealth config, after `--root`'s
  `chdir` or the move to the repository's top, below: `Path`, which is
  `Locate("")`), holds it to the schema
  (`schema.go`, `SCHEMA`'s specs as data, with its problems' wording), then to
  the cross-checks (`cross.go`), and lays it over **the one table of
  defaults**, `defaults()` in `defaults.go`: one ordered tree, as `DEFAULTS`
  is one object, merged under the file by `layered` (`tests.<kind>` under each
  kind) and decoded into the typed `Config` the tools read. `DefaultsFor` is
  `defaultsFor`, the registry beside the config's ledger, and takes whether
  the config is the stealth one, whose `hooks.bin` is `GlobalBin` (`itos`)
  whatever the file says and which has no `work.people` (`stealthOnly`, laid
  over the file too, so the file cannot say otherwise). `GlobalBin` is every
  config's default `hooks.bin` since v4.0.0 (slice 79; `tools/bin/itos`
  before), the launcher on the `PATH` running the version the repository
  pins; the key stays, internal and unsupported, for a repository that must
  run its own build, as this one sets it. `config check
--print-defaults` prints exactly that tree, so a default cannot be applied
  without being printed. A key with no default is a nil pointer, a nil list or
  an empty `Ordered` (a mapping whose order matters, as written); the file as
  written stays beside the loaded config for `HasSection` and `Section`, and
  the tree it was decoded from for `Get`, which `itos config get <key>`
  (slice 45) reads a dotted key out of, after `KnownKey` has held the key to
  the schema (an object's keys, any key of a map, nothing under a list or a
  scalar), so it prints what the tools use. A stealth config's own data
  (`ledger.files`, `work.registry`, each smoke file) is resolved beside it in
  that tree before the typed config is decoded (`beside`), so the two agree.
  `Readings` and `MatchesStatic` are `readings` and `matchesStatic`.
- **From a subfolder** (`internal/config/top.go`, slice 40): with no
  `--config`, no `ITOS_CONFIG` and no `--root`, and no `itos.yaml` in the
  folder, `Top` asks `git rev-parse --show-toplevel --git-common-dir` once
  per folder and run (so the launcher and `cli` ask once between them) and
  gives the top level when it is not the folder itself and holds an
  `itos.yaml` or the stealth config; inside the git folder, where git has no
  top level, and outside a repository it gives nothing. `cli.applyGlobals`
  then `chdir`s there before any command, as `--root <top>` would, and keeps
  where the person stood, relative to the top, in `origin`: `typed` reads a
  path they typed from there (check-paths' paths, check-message's,
  work check's and hook commit-msg's file, `config check --ledger`,
  `tests smoke check --features`), as git reads a path typed in a subfolder,
  and `runGit` runs `git commit` (and `itos push`'s git) in it, so the
  pathspecs and the `-F` file given to git mean what they meant. Under
  `--root` nothing is translated: a typed path is the root's, as before.
  The launcher's `readConfig` takes `Top("")` as its `--root`.
- **The stealth mode** (`internal/config/stealth.go`, slice 30) is one
  person's itos in a repository whose team does not use it. Where there is
  no `--config`, no `ITOS_CONFIG` and no `itos.yaml` in the root (in the
  current source or the working tree), `Locate` gives
  `<git common dir>/itos/itos.yaml` when it exists: `stealthFile` asks
  `git rev-parse --git-common-dir` once per folder and run and keeps git's
  answer, relative to the folder (`.git`) or absolute (in a linked
  worktree), so messages name `.git/itos/…` and every linked worktree finds
  the main one's. `IsStealth` tells the stealth config by where it is
  (`os.SameFile` with that path, after a check of the file's and its
  folder's names that spares a project's `itos.yaml` any git), not by how it
  was found, so the absolute `ITOS_CONFIG` an extension is given reads the
  same files when it calls back. `Load` marks it `Stealth`, lays its own
  defaults over the file (`stealthOnly`, slice 35: `hooks.bin` is `itos`, the
  global launcher, so the hooks `hooks install` declares call it, and there
  is no `work.people`, the person being the only one; `work` with no `--as`
  asks no identity provider and gives the session every item, slice 38,
  below), and `beside`
  rewrites the paths of itos's own data, `ledger.files`, `work.registry` and
  each kind's `smoke.file`, into its folder; a kind's
  `root`, the globs and the commands stay the root's, being the project's.
  Nothing else knows the mode but two readers: `internal/source` reads a path
  in the git folder from the file whatever the tree (below), and the footer
  rules' `knownFor` reads a ledger footer's IDs in the working tree for a
  stealth config whatever its `read_at`, since no commit carries that ledger,
  and without the warning a commit predating its ledger gives.
- **The stealth mode's footers** (`internal/message/notes.go`, slice 32):
  its links, the footers of IDs, live in a git note on each commit, in
  `refs/notes/itos`, never in its message; a footer of free text is
  content and stays in the message (slice 36). `message.Reading.Note`
  carries a commit's links, and `FooterProblems` reads them there for a
  stealth config, its type and its footers of free text still the
  message's; a link written in the message is that footer's problem
  (`typedFooter`), and a missing one's sentence ends with
  the `itos commit` flag that writes it (`stealthNeed`, `stealthFix` for the
  fix). Who fills `Note`: the commit-msg hook's `handedFooters`, from
  `ITOS_FOOTERS` or, for what `amending` takes for an amend, HEAD's note, which the rewrite carries to the new commit; it
  hands the same lines to the task checks rule, and `checkEnv` keeps them
  from the checks; `check-message`, from `ITOS_FOOTERS`; verify, from
  `message.Note` of each commit. The range reader of links, `IDsIn` (ci
  plan's named tasks), logs `%N` with `--no-notes --notes=refs/notes/itos`
  in place of `%B`; `Gathered` (`commit footers`) reads free text, so the
  message, in either mode.
- **The stealth mode's range** (`internal/git/unpushed.go`, slice 34) is
  the person's unpushed commits, `HEAD --not --remotes` (every commit of
  HEAD with no remote), which `verify` and `ci plan` take when given no
  range under a stealth config; `internal/cli/commands.go` asks
  `config.IsStealth(config.Path())` before the usage error, so a project's
  is unchanged and needs no config load. A range's start is a string
  everywhere a range goes, so these commits are the range from
  `git.Unpushed` (`"--remotes"`), and `git.Revs(from, to)` is the arguments
  `git rev-list` and `git log` take for any range: `from..to`, or
  `to --not --remotes`. Its readers: `cfg.RangeArgs` (verify's commits,
  `^<commits.since>` put first, since `--not` turns round what follows),
  `message.IDsIn` (ci plan's tasks and scenarios) and `plan.Readable`;
  `plan.Changed` takes `git.UnpushedPaths`, the paths its commits touch,
  there being no one commit to diff from in general; `tests.RangeCommands`
  gives `{from}` `git.UnpushedBase`, the pushed commit they grow from (one
  boundary of `rev-list --boundary` left by `merge-base --independent`),
  the end itself when nothing is unpushed, else empty, as a new branch's;
  and `plan.RangeOf` gives the JSON's `from` as `--remotes`.
- **YAML as JavaScript reads it** (`internal/value`). go.yaml.in/yaml/v3
  parses, but every value is JavaScript's: each plain scalar is resolved by
  the YAML 1.2 core schema the `yaml` package uses (yaml/v3 would read `017`
  as octal and `1_000` or `0b1` as numbers), mappings keep JavaScript's key
  order (array indices first), absent is `Undefined` apart from null, and the
  messages render a value as a template literal (`String`), `typeof`
  (`TypeOf`) and `JSON.stringify` (`JSON`) would, so a problem quoting an odd
  value reads the same in both. `YAML` writes the `yaml` package's block
  style, for `--print-defaults`; `SetScalars` edits a person's YAML in place,
  for `itos pin` (above), and `Doc` (`doc.go`, slice 52) does the same by
  paths of keys and indices into the document, for the registry's writers
  (above): `Set` replaces a one-line scalar in its style (quoted in a flow
  collection when a plain one would hold its indicators) or adds a key after
  the mapping's last one-line value, `Lead` puts a sentence before a why (a
  new first line of a folded or literal block), and, for `work add` and `work
edit` (slice 54, `docedit.go`), `Append` adds a block mapping after a block
  list's last item (the document's top one, a ledger file, for `task add`,
  slice 55) (a blank line before it when one parts the last two, a
  why folded and wrapped at 100), `SetList` writes a list whole as one flow
  list on its key's line (bug 14: a flow list on that line replaced where it
  stands; one the formatter wrapped below its key, or a block list, cut from
  past the key's colon to its last line; or the key added), and `Note` adds a paragraph
  to a text after a blank line (a one-line why in a block mapping becoming a
  folded block), and `SetBlockList` (slice 66) writes a list whole as a block
  list below its key, `[]` when empty, added before a given key above the
  comments leading into it (the registry's queue, before `items:`), a shape
  no formatter rewraps however long it grows, each edit made in `Want` too,
  and `Text` refuses the edits unless the text reads back as `Want`, keys in
  any order. Of two edits at one offset the earlier writes first. The patterns are compiled as RE2, the
  config's dialect, which the TypeScript held them to by refusing what RE2
  cannot compile (the config loader, above); where itos builds a pattern
  around `\s` or trims, it uses `value.Space` and `value.Trim`, JavaScript's
  whitespace, not RE2's ASCII one.
- **Where itos reads its data** (`internal/source`) is `source.ts`: every
  reader of the config, the ledger, the registry, the people and the smoke
  sets reads through `Has`, `Read` and `List`, with Node's wording for a
  failed read of the working tree (`ENOENT: no such file or directory, open
'people.yaml'`). They read the current `Source`: `Worktree` by default, and
  `ReadingFrom(s, fn)` swaps in `At("index")` or `At(<commit>)` while `fn`
  runs, a tree git holds read by path from the repository's top (`git
cat-file -e`, `git show`, `ls-files` or `ls-tree`), so the commit-msg hook's
  check of the staged data is `config check` again under `ReadingFrom`. The
  config is read from the source when it holds it and where it is otherwise
  (an `ITOS_CONFIG` outside the repository), and a ledger folder a git tree
  lacks is the folder-missing error, as `source.ts` has them. A path in the
  git common dir, which no tree git holds can have (the stealth mode's
  config and data), is read from the file by `Worktree` whatever the source:
  `At` asks git for the top and the common dir, absolute, in one
  `rev-parse`, and `gitTree.aside` compares the path, made absolute against
  the resolved working folder, with that. So the commit-msg hook's task
  checks, which read the staged ledger, find the stealth one. Beside the
  source, `Texts(tree, dir, keep)` is `treeTexts`: a git tree's files under a
  folder, read in one `git cat-file --batch` run (the TypeScript ran `git
show` per file), none when the tree cannot be read. `ledger.IDs` and the
  Gherkin adapter read a footer's or `--at`'s tree through it, whatever the
  source is.
- **The footers and the message** (`internal/message`) are `footers.ts` and
  `commit.ts`. `FooterProblems(cfg, message, Reading)` runs each footer's
  rule in the config's order, a `read_at: commit` footer at `Reading.At` (the
  staged tree when empty; `commit check-message` sets it from `--at`, else
  `ITOS_AT`) and the others at the working tree, with a commit that predates
  the source read against the working tree and its warning on
  `Reading.Warn`; `PrintFooters` prints them as commitlint does. `Check` is
  `commit check-message`: the delegate (`commits.header_lint.stdin`) through
  `internal/shell` with the message on stdin and `ITOS_AT` in its
  environment, its report printed as it comes or read by `ParseReport` into
  leveled problems under `--json`, then the footer rules always. Verify reads
  each commit's footers through `FooterProblems` with `At` set to it and
  `Made` set, so a footer's `since` lifts its `required_for` there and only
  there, and the commit-msg hook through the same call with the hook delegate
  beside it. A free-text footer is read by `Texts` (a line each, trimmed) and
  judged by `checkText`; `None` is the one reading of `none` (the word alone,
  as written), and `Gathered` is a range's texts with their commits
  (`git log --no-merges --reverse` over `from..to`), which `commit footers`
  (`internal/cli/commit.go`) prints.
  Under `commits.header_lint.use: builtin` all three run, in the delegate's
  place, the built-in header lint (`HeaderProblems`, slice 25), which has
  no TypeScript original: it is commitlint 21 with config-conventional as
  this checkout's `node_modules` hold it, read from those packages.
  `header.go` is conventional-commits-parser with the conventionalcommits
  preset's patterns (the breaking-change header tried first, the body ending
  at the first footer-token line, a breaking-change note running to the next
  one) and config-conventional's rules in its order, each a judgement that
  gives commitlint's words or none; `ignore.go` is is-ignored's wildcards
  (merges, reverts, reapplies, fixups, versions by semver's strict pattern);
  `cases.go` is subject-case and type-case as `@commitlint/ensure` and
  es-toolkit find a case, its word pattern written out with the backtracking
  each alternative needs, JavaScript's case mappings where Go's differ, and
  lengths in UTF-16 code units. Patterns use JavaScript's dot and `\s`
  (`value.Space`) and spell `/i` out on ASCII, since RE2's differ. The hook
  passes the message file with a line break added and git's comment
  character (`CommentChar`, `core.commentChar` or `#`), whose lines and
  scissors it leaves out, as commitlint `--edit` does; check-message and
  verify pass the message as given, as commitlint reads stdin. A blank
  message has no problems in the hook's reading, where commitlint `--edit`
  lints nothing and git aborts the empty commit itself, and on stdin names
  type-empty and subject-empty, where commitlint refuses it as no input.
  `builtin`
  prints the problems as commitlint prints one (`PrintLeveled`), errors
  before warnings, then the footer problems, or one leveled list under
  `--json`; a warning alone passes. Its one known difference is wording:
  es-toolkit's deburr decomposes every precomposed letter (NFD), and
  `cases.go` only those of Latin-1 and Latin Extended-A, so subject-case
  may name start-case or pascal-case for a subject with another one, where
  the verdict is the same. `tools/selftest/header-agreement.ts` holds it to
  commitlint without commitlint: `header-agreement-record.ts`, run while
  commitlint was installed, wrote commitlint's verdict and report lines for
  every message of this history, of the corpus and of a set aimed at each
  rule, in both readings, to `header-agreement.json`, and the check runs the
  binary's hook and check-message over them in a scratch repository whose
  config is the header lint alone. A
  command adapter (`internal/tests`) is `<command> list --at <tree>` through
  the config's shell, its output read by `value.ParseJSON` (`JSON.parse`'s
  reading, keys in JavaScript's order) and held to the protocol, its failures
  carrying its stderr and its exit written as `spawnSync`'s `status ??
signal` (`shell.Result.Status`); `supports_at: false` warns once a run on
  `tests.Warnings`, which `cli.Main` points at its stderr.
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
  it, as the TypeScript failed only there. `work.ItemStatuses` is
  `itemStatuses`, the registry read alone, without the people, so `task list`
  works over a registry with problems. A group flag's value is read by
  JavaScript's `Number()` (`value.ToNumber`, so `--group 01` is group 1) and
  the status table pads by UTF-16 units (`value.PadEnd`). The runner's ID
  pattern is `^(?:<ledger.id>)$`, `T-\d+` without one, compiled as RE2
  like every config pattern.
- **The globs and the path rules.** `internal/glob` is `globToRegExp` and
  `matchesAny` ported as written: `Source` makes the TypeScript's five
  replacements in the same order, so a glob means what it means there, its
  quirks too (a `?` stays the regular expression's `?`; a `*` right after a
  `.` is left alone, so `a.*` is `a` and any dots). RE2 forced two changes of
  construction, not of meaning: the last replacement's lookbehind (a `*` not
  after a `.`) is a scan, and every `.` the replacements write is spelled as
  JavaScript's dot, `[^\n\r\x{2028}\x{2029}]`, since RE2's stops at `\n`
  alone. A glob V8 refuses is an error worded as V8 words it
  (`Invalid regular expression: /^?a$/: Nothing to repeat`), the leading `?`
  RE2 would take included, and only when a path reaches it: `MatchesAny`
  stops at the first match, as `some` does, so a bad glob after it is never
  compiled. `internal/scope` is `commit-scope.ts`'s path rules: `scope.Of`
  takes the loaded config (whose loader has expanded each `$set`) and is a
  config error without a `commits` section, as `section("commits")` is; its
  `Rules` give `Ruled` (whether a type has rules, even empty ones: the
  commit-msg hook runs neither the paths nor the staged range checks for one
  without), `Issues` (a type and its paths to the problems, with the rule
  ids and the fixes naming the types that would take a path, from
  `TypesFor`), and `Reject` (the rejection as the hook prints it). `commit
check-paths` (`internal/cli/commit.go`) is a call to them, and the
  commit-msg hook and `verify` call them on a commit's paths without going
  through the command line.
- **The named tests** are `internal/tests` and `internal/cli/tests.go`
  (`tests.ts`, `smoke.ts`, `smoke-rule.ts` and `tests-command.ts`).
  `ListTests` is the adapter's listing at a tree, and `ListTestsUnder` the
  same with `tests smoke check --features <dir>` standing in for a Gherkin
  kind's root; a command adapter's `List` keeps its object as printed in
  `Raw`, which `tests list --json` prints key for key, extra keys included,
  as the TypeScript printed the parsed object (`out.Emit` gives a key written
  twice its first place and its last value, as `{ schema: 1, ...value }`
  does). `CommandFor(cfg, kind, selections)` is `commandFor`, the one
  function that turns `Selection`s (`Whole()`, `IDs(…)`, `Pattern(p)`) into
  one command through the kind's run templates: `whole` if any selection is
  whole, else a pattern per IDs selection by `ids_pattern` (its `{ids}` the
  bare IDs, deduplicated, joined by `|`), the patterns deduplicated in order
  and, when more than one, each in `join.each`'s `{p}` joined by `join.sep`,
  the result one shell word in `select`'s `{pattern}`; each placeholder is
  replaced once, as `String.replace` does. It says false when nothing is
  selected (an IDs selection with no IDs adds no pattern), and a template
  the selections need and the kind lacks is a config error. `tests smoke
run` is it over the smoke IDs, run through the config's shell with
  itos's streams, its exit the runner's or 1 (`status ?? 1`); CI's plan
  calls the same function for its one merged run. itos compiles none of
  the templates' patterns: the runner reads them in its own dialect.
- **next-id** (slice 60, `internal/cli/nextid.go`) is help for a spec
  writer, reading only the working tree. `tests next-id <kind> <stem>` is
  `tests.NextTag`: every word of a tag line of the kind's feature files that
  starts with the tag prefix (a command adapter's listed IDs instead), and the
  work registry's item ids, since `slice-<n>` and `bug-<n>` items are named
  before their scenarios; `task next-id` is `ledger.NextID`, the ledger's IDs
  and the registry's that `ledger.id` matches (`work promote` names a task
  before the ledger holds it), its prefix `ledger.id`'s literal one. Both end
  in `nextid.Next`: one past the highest `<prefix><digits>`, as wide as the
  widest the series writes, a fresh `ID-` stem of an `ID-` pattern two digits
  wide, and the ledger's widened until `ledger.id` matches it.
- **The moves rule and verify.** The moves rule is `tests.Moves`
  (`internal/tests/moves.go`, `moves.ts`): one comparison, `MoveProblems`
  over two `FeatureSet`s that `ReadFeatures` builds through `ParseFeature`
  with comment lines dropped, its IDs and files kept in the order
  JavaScript's Maps keep them (files by path, as git lists them; an ID
  written twice keeps its first place and its last block), since the
  problems are printed in that order. A command kind's comparison is
  `ListedProblems` over two adapter listings (`moves_command.go`), each read
  once per tree through `ListTests` at the index or at a commit, a tree that
  names no commit (the empty tree, an unborn HEAD) listing nothing; a merge's
  listing before its own changes is the merge's with the tests of its own
  paths (the kind's root joined to each test's file) as its first parent
  lists them. `Between` and `Merge` go through one `judge`, which picks the
  comparison by the kind's adapter. `NewMoves(cfg)` reads each kind's
  feature files once per tree and gives the three callers: `Commit(sha,
type)` for verify (the commit against `git.Parent`, the empty tree for a
  root commit, nothing read when no check judges the type), `Between` for
  the commit-msg hook (HEAD, or for an amend HEAD's parent, against the
  index), `Merge` for a merge commit in both (bug 30, below) and
  `Index(kind)` for `tests moves`. The hook's `judgedBase` is
  that parent (`git.Parent("HEAD")`) when `amending` takes the commit for an
  amend, else HEAD, and the staged-data rule and the path rules read
  `stagedFiles(base)` against it too: an amend is judged as the commit it
  makes, so one that only rewords a feat still has the feat's paths (bug 6). `verify` is `internal/cli/verify.go`: the range's commits
  from `git rev-list --reverse` over `cfg.RangeArgs(from, to)`
  (commits.since and its ancestors left out), each one's message through
  `message.Check` at that commit, the delegate's report on stdout (stderr
  under `--json`), then, only when the message holds, its paths
  (`git.CommitPaths`) through `scope` and its moves as one rejection; then
  `tests.RangeCommands(cfg, from, to)`, each kind's `range` commands with
  `{from}` at `cfg.RangeStart(from)`, run until one fails. That walk is
  `verifier.over`, which returns each commit's result (`verifyRun`), so
  `verifyRange` adds only the config's checks, `--json` and the exit code,
  and the pre-push hook calls it on each pushed range. The range
  helpers sit beside `SinceIssue` in `internal/config/since.go`.
- **The headers git writes, and merges** (bug 30). The header lint leaves
  git's own headers alone, as commitlint does (`ignore.go`), but every
  other rule judges them by what they stand for, through `message.Type`,
  the one reading of a commit's type the paths, the footers, the moves rule
  and `itos commit`'s footer flags share: `Revert "…"` and `Reapply "…"`
  are `revert`, and an `amend!`, `fixup!` or `squash!` commit is the type of
  the header it names once the prefixes are off (`message.Named`), a named
  header with no type of the commit types refused under `named-type`
  (`namedProblems`, run first among the footer rules, so every lint path
  has it). A merge commit is judged by its own changes, `git.OwnPaths`:
  the paths of its dense combined diff (`git diff-tree --cc`, what `git
show` shows of a merge), those with a hunk whose lines in the merge are
  no parent's there. A clean merge has none, even of a file both sides
  changed, since each hunk is one side's, and so does a conflict resolved by
  taking a side; the paths whose whole content matches no parent (`-c`)
  would count every file both sides touched, and a re-merge's diff
  (`--remerge-diff`) any merge made with another strategy or option. A
  merge with none passes whatever its message (verify counts it, the hook
  returns before its rules); one with some is judged as its first line's
  type, its paths its own and its moves its own paths against its first
  parent (`Moves.Merge`), and refused under `merge-type` when that type is
  none of the commit types (`untypedMerge`). The hook reads the merge being
  made the same way: its parents are HEAD and `MERGE_HEAD`'s commits (or,
  for an amend, HEAD's), and `git.StagedOwnPaths` commits the index's tree
  on them, an object nothing keeps, to read its combined diff. Other headers
  the lint leaves alone that name no type (a "Merge …" header on a commit
  with one parent, a bare version) keep the word they start with, which no
  rule judges.
- **CI's plan** is `internal/plan` (`ci-plan.ts`, `ci-plan-json.ts` and the
  plan's half of `ci-scope.ts`), with `ci plan` and `ci scope` in
  `internal/cli/ci.go`. A `Plan` is a value the driver walks, not text:
  `Order` is every `Item` in the order the run takes it, a `Step` (its
  command, its cost, and `Tests`, the kind, when it is the one run of named
  tests) or a `Check` (a `check.CostedCheck`, so its task with the title a
  failure names, its place in `done_when` and its cost, plus its `Action`:
  `run`, `merged` into the kind's run, `covered` by a step, or `nightly`);
  `Steps`, `Tasks`, `LeftOut`, `Unknown` and `NotStarted` sit beside it for
  the preamble. `Make` is `ciPlan` over an `Input`; `For(cfg, from, to,
Data)` is `planWith`, `DataAt` reading the ledger, the registry and the
  smoke set from the working tree or, for `--data-at`, from a commit under
  `source.ReadingFrom` (the config stays the working tree's); `ForNightly`
  is `planNightly`, its `{ tasks: done }` step the checks of `DoneTasks`
  (`work.ItemStatuses` says `done`). `Fields` and `Print` are the JSON and
  text `ci plan` prints, the contract phase 3's shadow compares, so their
  keys, order and values are the TypeScript's. It builds on
  `check.CostedChecks` for the cost classes and written order and on
  `tests.CommandFor` for the merged run. A task check becomes a selection
  through `tests.Recognize` (`internal/tests/recognize.go`), each template a
  pattern whose `{pattern}` is one shell word, the bare word bounded by
  JavaScript's whitespace rather than RE2's. A range's commits name tasks and
  tests through `message.IDsIn`, `footers.ts`'s range log (`git log
--format=%B from..to`, newest first). That range is `from..to` as the
  TypeScript's plan read it: `commits.since` does not narrow it, as it
  narrows verify's, and an empty start is unread, so every test runs, yet
  reads the footers of every commit up to the end (bug 33), so the tasks and
  tests they name run too.
  A stealth config's `ci plan` with no range plans the unpushed commits
  (`git.Unpushed`, above). The plan carries its range's `Ends` (`ends.go`,
  slice 83): the start a range check's `{from}` gets (`tests.RangeFrom`: the
  unpushed commits' base, `commits.since`) and the end, as full SHAs, the
  start empty for a range that runs everything; the nightly's are those of
  every commit up to HEAD. Once the plan is made, `{from}` and `{to}` in each
  step's command, and in the step a covered check names, are filled in by
  the range checks' filler (`tests.FillRange`, each one shell word), so `ci
plan` prints the command `ci run` runs, while the cost class and
  `ci.covers` read the step as `ci.steps` writes it.
- **CI's driver** is `internal/ci` (`ci.ts`'s `ciRun`), with `ci run` in
  `internal/cli/ci.go` making the plan as `ci plan` does and handing it over:
  the driver never plans, so a run carries out what `ci plan` prints. It sets
  `ci.env` in its own environment, then `ITOS_FROM` and `ITOS_TO`, the plan's
  `Ends`, which every step and check it runs inherits, says the preamble (`Unknown`, named against
  the ledger's folder, ends the run whatever `ci.stop_at_first_failure` says;
  `NotStarted`; `Prose` with `LeftOut`), then walks `Order`: a `Step` runs
  through `shell.Run` on itos's own streams (the log is stdout, stderr under
  `--json`), so the command's output keeps its place beside itos's lines, and
  a failing step's exit code is the run's (1 when it has none, `status ??
1`); a `Check` whose `Action` is `run` goes through a verbose
  `check.Runner`, a failure exiting 1 and naming the task and its title, and
  the other actions are only logged. The nightly keeps the Runner's runs, so
  a check its done tasks share runs once; a push runs each named task's
  checks as its own, as the TypeScript did (its `runs` is the nightly's
  only). `--json`'s `failed_at` is a `Failure`, its keys in `ci.ts`'s order.
- **Where a range starts** is `ci range` (`internal/cli/ci.go`) over
  `internal/providers/range.go` (`providers.ts`'s range half and
  `ci-scope.ts`'s `rangeStart`). It reads the config first, so a broken
  `ci.range` is a config error even with `--base`, then `RangeStart` keeps a
  pull request's base without asking the provider, else holds the provider's
  start to the head with `git merge-base --is-ancestor`. A provider is a
  `Range`, a function of the head giving the start or `""`, never an error,
  made by `RangeProvider(cfg, env)` from `ci.range` and the
  environment: `none`, and `github`, a `GitHub` value (repository, token, workflow, API
  address) whose `NearestGreen` walks the head's first parents from its
  parent (`git rev-list --first-parent`, `FirstParentsAsked` of them, 100)
  and asks the API through `net/http` for each commit's runs (`GreenRunOf`,
  `head_sha=` alone), starting at the first with a successful run of
  that commit, on whatever branch it ran (bug 29: filtering by
  `ci.range.github.branch` made a push to another branch start at main's
  green ancestor; v5.0.0 removed the key, slice 85).
  It reads no list of the branch's runs: GitHub served that list
  stale on 2026-10-05 and the range reached back past commits already proved
  (bug 23). A provider over another forge's API (GitLab, Forgejo, waiting
  for `p1-conformance-http`) is another such value and another case in
  `RangeProvider`. The API's address is `GITHUB_API_URL` (`APIEnv`) when the
  environment sets it, as Actions does and GitHub Enterprise needs, for
  every look at GitHub, ci.range's, ci.watch's and itos status's (bug 24:
  `watchGitHub` gives the address with the repository and token), else the
  variable `GitHubAPI`, so a
  unit test points either at an `httptest` server and the scenarios point
  the binary at one through the environment, as no conformance case can
  reach the network. Node's `fetch` waits however long the API takes; the
  port gives up after `Timeout` (a minute), which reads as no green run and
  runs everything, as every failure of the walk does.
- **Waiting for a CI run** (slice 51) is `ci watch` and the end of `itos
push` (`internal/cli/watch.go`) over `internal/providers/watch.go`, beside
  the range provider. `WatchProvider(cfg, WatchSetup)` makes a `Watch` from
  `ci.watch`, one look at a commit's run: `none` makes none (push then
  reports as before); `github` is the `GitHub` value again, whose `RunOf`
  lists the workflow's runs for `head_sha`, takes the newest by
  `created_at`, and reads its jobs, a `Run` (url, status, conclusion, jobs), with a
  token from `ci.range.github.token_env` else `GhToken` (`gh auth token`),
  and the repository from `ci.range.github.repository_env` else
  `GitHubRepository`, the remote's URL read as GitHub's (https, ssh or
  scp-like). With no token it fails before any request, naming both ways to
  give one. `RunOf` answers `found` false while GitHub has no run, and a
  `Transient` error for no network, a 5xx or a 429; any other refusal is an
  error. `watchRun` looks, then sleeps `min(interval, time left)`
  (`sleep`, a variable a unit test makes instant), until the run is `Done`
  (completed with a conclusion: GitHub can say completed a moment before it
  records one) or the deadline passes: the run's address is printed when
  first seen and each job's result once, keyed by its name, as it becomes
  `Done`, to stdout (stderr under `--json`, nowhere under `-q`); the end goes
  to stdout for a success, else to stderr with the jobs that did not succeed,
  skipped and neutral ones aside. A `Transient` error is remembered and
  looked past, and named if the timeout comes first; any other error, or the
  timeout, exits 3 naming `itos ci watch <sha>`. The outcome is a `watched`
  (code, `success`/`failure`/`timeout`/`error`, the run as last seen), whose
  `fields` are the `ci` and `run` keys both commands' `--json` add.
- **The work routing** is `work` and `work check` (`internal/cli/work.go`,
  `work.ts`'s two commands), and `work list` (slice 43, Go only), over the registry reading the config group
  ported (`work.Load`, `Issues`), never a second one. `work check` is
  `ProblemsAt`, the config's `work.registry` or a file named after `check`
  (the argument whatever it is, as `main.ts` takes it), whose missing-file
  fix differs (`Missing`); a named file that is not there is found before
  the config is read, as the TypeScript read the config only to load the
  registry. `work.Load` reads the people beside the registry, and none under
  a stealth config; a people file that is missing or cannot be read leaves
  `Registry.People` false, silently, and then no owner is checked and any
  handle is listed (slice 35): only `config check` says why, as a warning
  (`PeopleProblem`, `WARN` on stderr and a `warnings` list in `--json` only
  when there is one), which never sets its exit code and is no finding of
  the commit-msg hook's check of staged data. `work` runs that check
  quietly, its problems on stderr even
  under `--json`, then, under a stealth config with no `--as`, no lookup at
  all: `ProposeEvery` is the proposal of a session that owns every item,
  whatever owner it or its group names, nothing unowned, the text saying so
  and `--json` adding `every_item: true` after a null `person`, a key no
  other proposal has (slice 38, the person being the only one). Otherwise
  `Whoami`: `--as`, which must be among the people when
  there are any (exit 3), else who the people make it (bug 46): one login
  is the session, and with no one listed (`Registry.Nobody`: no people file,
  an unreadable one, or one listing nobody) the session is nobody and no one
  is asked, `work take` then leaving the owner as a stealth config does.
  Only among several people is the `work.identity` provider asked, an
  `Identity` function made
  by `IdentityProvider` (`internal/providers/identity.go`) that answers a
  handle or why it has none, never an error: `none` with its hint, and
  `github` running `gh api user --jq .login` as the
  TypeScript did, gh's stderr dropped and gh missing (`exec.ErrNotFound`)
  told apart from gh failing. gh gets `GhTimeout`, 5 seconds, a constant
  and no setting, its stdin the null device; past it the context kills gh
  and, on unix, its process group (`killTree`, `gh_unix.go`), `WaitDelay`
  stops waiting on its output, and the answer is a problem naming `--as`. A session with no handle is nobody, and one
  the people do not list owns nothing yet; both still see what nobody owns.
  `Propose` keeps each item the mapping as written (`value.Map`, its keys in
  JavaScript's order, `depends_on` added last when absent), so the `--json`
  proposal is the TypeScript's byte for byte. The identity providers over
  the GitLab and Forgejo APIs wait for `p1-conformance-http`, as the range
  ones do: neither implementation's schema accepts them yet, so a config
  naming one is a config error, not a command that runs half-built.
  `work list` is `work.Load` and `work.Open` after it: the open items
  (every one neither done nor dropped, slice 79 and bug 25: the two
  statuses itos knows to be closed, so blocked and any status a project
  adds to `work.statuses` are open) in the registry's order, and with `--all`
  (slice 77) every item, which plain `work list` printed until v4.0.0;
  judged by nothing, as `task list` is not, so a reader of the titles
  (T-066's plugin, which asks `--all`) keeps them through a registry
  problem. Its `--json` items are the same `value.Map`s the proposal
  writes, so the two commands describe an item alike, and its text is
  `work.PrintList`, which marks a deferred item `(deferred)` after its
  title.
- **The registry's writers** are `work take` and `work promote` (slice 52,
  `internal/cli/workwrite.go` over `internal/work/write.go`), the registry
  being written by commands, never by hand, and each committing its own
  change. Both start from a sound registry (`work.Problems`, reported as
  `work check` would, exit 1) and its text. `work.Take` and `work.Promote`
  judge it, an item found by its id with `find`, and give a `Change`: the text
  after, the item as it reads after, the items whose `depends_on` were
  renamed, the commit's header and body, or `Unchanged` (an item already in
  progress for the person); or an `out.Problem` that refuses, nothing
  written. `work.Promote` also replaces the title when `--title` gives one
  (slice 65), its body naming the new title. `work take`'s person is `Whoami`'s, as for `work`, nobody or one
  the people do not list exiting 3; under a stealth config, or with no one
  in the people (`Registry.Nobody`, bug 46), with no `--as` nobody is asked,
  as `ProposeEvery` asks nobody, and the owner is left as it is. Ownership is the item's owner, else its group's (`ownerOf`, as
  `Propose` reads it). The text is edited with `value.Doc` (below), every
  comment and quote kept. `writeRegistry` is the part any registry command
  shares, `work done` (slice 53) included: in a project it refuses a registry
  git does not hold as HEAD has it (untracked, or changed, staged or not),
  whose changes the commit would sweep in, then writes the text and runs
  `git commit --only -m <header> -m <body> -- <registry>` from the top, with
  the hooks, git's words on stderr, so the commit holds the registry alone
  and what else is staged stays staged (git commits the path through a
  temporary index). A commit that fails puts the file back and the index
  entry with `git reset`, which is exact since the registry was clean. The
  commit is a `docs` one with no footer, its body wrapped at 100 by the wrap
  `itos commit` uses (`selfBody`, `message.Wrap`, bug 18), so no line starts
  with what the header lint reads as a footer or with git's comment char: a
  `Task:` footer on a take would make CI run the task's checks before its
  work exists. Under a stealth config the registry is in the git folder, tracked
  by nothing: written, nothing committed, and `--json`'s `commit` null.
- **work done** (slice 53, `internal/cli/workdone.go`) is the landing's
  check, then a registry write like the two above: `work.Done` judges the
  registry alone (an idea, a deferred item or a status neither todo nor doing
  refused, an item already done `Unchanged`), then the command checks, in
  cost order, stopping at the first that refuses (exit 1): the item's
  scenarios at HEAD (`tests.Tagged`, the Gherkin kinds' scenarios whose tag
  line, or their file's, holds `@<id>`, so `@slice-<n>` for the item
  `slice-<n>`; a command adapter's list carries no tags and is left out),
  none still `@wip`; no commit of HEAD that no remote has (`rev-list HEAD
--not --remotes`, said and skipped with no remote at all); a task's static
  checks, by the commit-msg hook's `firstFailure`, when the id is a task of
  the ledger; and with `ci.watch`, HEAD's run, by `itos ci watch`'s `watcher`
  and `watchRun`, so a run still going is waited for and one that does not
  end exits 3. Without `ci.watch` CI is not checked, said on stderr, and
  `--json`'s `ci` is `unwatched`. Since the checks can take minutes, the
  registry is then read again (`soundRegistry`) and `work.Done` judged and
  made afresh on it, so a registry commit made during the wait is kept, not
  written over with the text read before it, and an item no longer one
  `work done` may close is refused, nothing written (bug 34). The close
  commit is `docs: close <id>`, its body naming the run that passed.
- **work add and work edit** (slice 54, `internal/cli/workedit.go`) write
  the rest of the registry, through `writeRegistry` as the three above.
  `work.Add` puts a new item, todo, at the end of the items (`value.Doc`'s
  `Append`, keys in the registry's own order, its why folded); its group is
  `--phase`, else the one a `p<n>-` id names, else the registry's only one.
  `work.Edit` replaces a title (`Set`), a `depends_on` or `refs` (`SetList`)
  and adds a paragraph to the why (`Note`); `Change.Changed` names the keys,
  and a list the item lacks compares as an empty one, so emptying it is no
  change. A note on a slice or a task (`SpecKind`, an item of no kind read
  by its id) is refused before anything is written, rule `work-note-spec`,
  naming where its why lives (`WhereWhy`): the registry keeps an idea's why
  alone (decision 0034, slice 79; slice 76 only warned).
  Neither restates `work check`: each runs `work.Issues` on the registry with
  the item as it would be (`sound`), and, the registry having been sound,
  refuses with whatever that finds (an owner not among the people, a
  dependency on no item, a cycle). Only the id taken, a task id `ledger.id`
  does not match and a group not given where none is plain are judged before
  it. `edit` changes no owner, kind or status: `take`, `promote` and `done`
  own those, each with its own rules.
- **The queue** (slice 66, `internal/work/queue.go`) is a top-level `queue:`
  list of item ids, one for the repository, ideas included: the order the
  work comes in, kept in the registry rather than in a handoff's prose.
  `work.Load` keeps it as written (`Registry.Queue`), `work.Issues` refuses a
  queue that is not a list, an id no item has and one named twice
  (`queueIssues`), and `Queued` gives a sound one's ids. `work.Queue` puts an
  item first, just before or after one the queue holds, or out of it (`work
queue <id> --top|--before|--after|--drop`, `internal/cli/workwrite.go`),
  the queue written whole with `value.Doc`'s `SetBlockList` before `items:`
  and committed by `writeRegistry` as `docs: queue <id>`; an item done is
  refused, and one already where it is put is `Unchanged`. `Propose` walks
  the items in the queue's order, the unqueued after in the registry's
  (`inQueueOrder`), so every list of the proposal, `--json`'s too, is in it,
  and what another person owns stays out of each, every person seeing their
  part. `work done` takes the closed item out (`work.Unqueue`) in a commit of
  its own after the close, under a stealth config while it still holds the
  registry's lock; `work.Promote` renames an idea in the queue as in every
  `depends_on`.
- **work drop** (slice 78, `internal/work/drop.go`,
  `internal/cli/workdrop.go`) takes an item out of the open work without
  deleting it, so its id is never given again: `work.Drop` sets its status
  `dropped` (`work.Dropped`), drops its why as `Done` does, takes it out of
  the queue and puts the `--why` reason first in the body of its
  `docs: drop <id>` commit. `dropped` is known whatever `work.statuses`
  lists, since only itos sets it: `itemIssues` accepts it, exempts a dropped
  idea or deferred item from the started-item problems, and reports an item
  still to do (`live`: neither done nor dropped) that depends on a dropped
  one (`work-dropped-dependency`). `Drop` refuses an item done and one that a
  live item depends on, naming those (`dependants`); `Take`, `Done`,
  `Promote` and `Queue` refuse a dropped item, and `Propose` never lists one,
  its status not being `todo`.
- **task add** (slice 55, `internal/cli/taskadd.go`) writes two files in one
  commit: the task at the end of its group's ledger file (`ledger.Add`,
  `internal/ledger/add.go`) and its item, a task, todo and nobody's, through
  `work.Add` as `work add` makes one. `ledger.Add` judges the id against
  `ledger.id`, the type against `commits.types`, then a ledger without
  problems (`Issues`, as `config check`) that lacks the id; the group's file
  is the one `Files` gives for it (numeric groups compared as numbers), else
  `ledger.files` with `{group}` filled in, made new when `ledger.group.pattern`
  matches. The task's keys are in the order the ledger's own are written (id,
  type, title, why, done_when; a check's run, then timeout), appended by
  `value.Doc`'s `Append` with no path, the document's top list, its checks a
  block list below `done_when` (`BlockItem` writes a file's first task).
  `writeRegistry` is `writeCommitted` of the registry alone; `task add` gives
  it both files, each refused before anything is written when git does not
  hold it as HEAD has it (`uncommitted`, naming the file): a file there must
  be tracked with no change, and one not there, which the command makes,
  unknown to git, so a file deleted and the deletion not committed is
  refused rather than written afresh over it (bug 13). A new file is `git
add`ed so `--only` can name it, git add's words and the commit's on
  stderr. Whatever fails after the first write, a later write (`put` says
  whether it touched the file), git add, a git that cannot start or a commit
  a hook refuses, goes through one `restore`: every file written gets its old
  text back where it differs, and its index entry reset to HEAD's, a new one
  unstaged and removed; what cannot be put back is an error naming the file.
  `internal/cli/workwrite_test.go` makes each of those fail in a scratch
  repository.
- **work show** (slice 57, `internal/cli/workshow.go`) reads, never writes:
  the item and the ids of the items whose `depends_on` name it
  (`work.Show`, `internal/work/show.go`), its scenarios at HEAD by
  `tests.Tagged` as `work done` reads them, and the commits of HEAD's
  history, non-merge, oldest first, that belong to it. Which do is itos's
  knowledge, not git's: `message.History` (`internal/message/history.go`)
  logs every commit's SHA, short SHA and message with its links, the
  footers of IDs (`Links`: each footer of `commits.footers` whose source is
  the ledger or a kind of named tests, its `strip_prefix` taken off), read
  from the message, or under a stealth config from the itos note (`%N` with
  `--no-notes --notes=refs/notes/itos`), as every reader of a made commit's
  links reads them. A commit belongs when a ledger link is the item's id, a
  tests link is one of its scenarios (with or without the kind's
  `tag_prefix`), or its header is one of the registry writers' naming the
  item (`work.RegistryHeader`, `docs: queue <id>` among them, kept beside the writers, whose headers its
  unit test reads). The registry is loaded and not judged, as `work list`
  loads it. `--patch` hands the commits to `git show`, in order, its notes
  the itos ones under a stealth config; `--json` gives each commit's message
  and diff (`git show --format=`) instead.
- **follow** (slice 61, `internal/cli/follow.go` over `internal/follow`) is
  the person's threads with people, apart from everything else itos keeps:
  it reads no config, so it runs in any git repository, and its one file,
  `follow-ups.yaml`, is in `config.StealthFolder` of the absolute git common
  dir (`git rev-parse --path-format=absolute --git-common-dir`), where the
  stealth mode keeps its data, never committed and the same from every
  linked worktree. The file is itos's own, so it is read and written whole
  with `yaml/v3` and typed structs rather than edited in place through
  `value.Doc`: `Load` refuses a key it does not know, a thread with no id or
  a repeated one, a status neither open nor closed, and (bug 16) a second
  YAML document or none at all, since saving what it read would drop the
  rest and itos never writes an empty file; `Save` writes a file beside it,
  mode 0600, syncs it and moves it over, so a reader in another worktree
  never sees half of it and a crash leaves the old file or the new, and
  makes the folder (0700) where there is none. Every subcommand that writes
  loads through `heldThreads`, which holds the file's lock
  (`internal/lock`, below) until the command returns, so two writers at
  once take turns. A note's time is written RFC 3339 to the second in local
  time and printed to the minute in the reader's zone (`follow.Show`,
  `time.Local`), so notes written from two zones read in order; the corpus
  pins `TZ` to the stored stamps' offset. The command line reads the clock
  in `followNow` alone and hands each change its time, and no case reads it
  (a case of add, note or close pins what it prints, never the file).
  `follow doc` writes `follow.Markdown` where the person typed (`typed`),
  0600 in folders made 0700, refuses a file already there without
  `--force`, and records the absolute path in the thread's `docs`; when
  `git check-ignore` exits 1 for the typed path (in the work tree, not
  ignored) it warns on stderr, and `-` prints the Markdown and writes
  nothing (`followDocOut`). A refusal is `refuseWork`'s problem, exit 1; no
  git repository is exit 3.
- **ask** (slice 62, `internal/cli/ask.go` over `internal/ask`) is the
  questions waiting on the person the work is for, public where follow's
  threads are private: one file, `work.asks`, whose default `DefaultsFor`
  puts beside the registry the config names, or the default one (so in the
  folder of `ledger.files`), and which `beside` puts in the git folder under
  a stealth config as it does the registry. The file is itos's own, so it is
  read with typed structs (`ask.Parse`: unknown keys, an id not `q-<n>` or
  given twice, a second document and an empty file refused, exit 2, as
  follow's) and written whole (`ask.Text`), every text a double-quoted
  scalar written by `encoding/json`, whose escapes YAML reads the same, so
  what it holds reads back exactly and `vp check` leaves it as it is. An id
  is one past the highest the file holds (`NextID`), so a question answered
  or removed by hand never gives its id again. `add` and `answer` write
  through `writeCommitted`, the file alone, `docs: ask q-<n>` and
  `docs: answer q-<n>`, refused while the file has changes no commit holds
  (`uncommitted`, rule `asks-file-uncommitted`); `add --item` reads the
  registry, unjudged, for the id. Under a stealth config they hold the
  registry's lock (`heldAsks`) from before the read to after the write, the
  one every stealth writer holds, and commit nothing. `work show` lists the
  questions naming its item (`File.About`), and `--json` gives them always.
  `record` (slice 69, over `internal/adr`) writes an answered question as an
  architecture decision record in MADR 4's format (slice 71, which replaced
  slice 69's adr-tools records): YAML frontmatter holding the status and the
  date, then MADR's bare-minimal sections (`adr.Text`), Considered Options
  from `--option`, which `repeated` takes out of the arguments before
  `subArgs` reads the rest. The folder is `work.decisions`, `docs/decisions`
  by default; `stealthOnly` makes it `decisions`, which `beside` puts in the
  git folder beside the stealth config. The number is one past the highest
  any name there starts with (`adr.Next`), the slug `adr.Slug`'s. A record
  is read by its structure, never its bytes: `Record.Status` parses the
  frontmatter as YAML, `Record.Title` takes the first `# ` heading after it,
  outside a code block. A supersede sets the old record's status to
  `superseded by ADR-NNNN` (`adr.SetStatus`, which replaces the frontmatter's
  status line, adds one, or adds frontmatter to a record with none), and the
  new record ends with More Information naming the old one.
  The index is the folder's `README.md`, its part between the markers
  rewritten whole (`adr.Index`) from the records whose status is accepted,
  by number and title, the rest kept; everything written is in the form a
  Markdown formatter leaves alone (bug 17).
  The question gains `decision: <n>` (or `none` for `--none`), and the
  questions, the new record, the one it supersedes and the index go through
  one `writeCommitted`, each `written` naming its own rule for changes no
  commit holds (`decision-file-uncommitted`, `decision-index-uncommitted`);
  the folders made for the record are removed again when the write fails
  (`madeDirs`, `unmake`). A question answered with no decision is
  `Unrecorded`, and `ask` ends with one line naming each as
  `itos ask record <id>`.
  `config check` holds the folder to two rules (slice 74, `adr.Problems`,
  area `decisions`): no two records share a number
  (`decisions-number-twice`, naming both files), and a status
  `superseded by ADR-NNNN`, read by `Record.SupersededByNumber` in any case,
  names a number a record there has (`decisions-superseded-by-missing`).
  `adr.List` reads the folder through `internal/source`, so the commit-msg
  hook, whose `stagesData` counts a staged record or index in the folder as
  itos's data, judges the records as staged; a folder that is not there
  holds none. The index is not held to the folder: `ask record` writes it
  whole at every record, and nothing regenerates it on demand.
- **go and guide** (slice 64, `internal/cli/guide.go` over `internal/guide`)
  print the guides a session starts from, Markdown files beside the package
  (`coordinate.md`, `work.md`) embedded with `go:embed` and printed as
  written, so one text, versioned with itos, serves every repository and the
  binary needs no file of its own at run time. They are generic by rule:
  what a repository learns that holds for no other is that repository's own
  notes, which `itos go` (and `itos guide coordinate`, the same) appends
  after a line of `---`: `guide.orchestrating`, a path read where the
  config's other paths are, whose default `docs/ORCHESTRATING.md` is in the
  one table and which `beside` puts in the git folder under a stealth config.
  With no config the default is read from the repository's top, reached by
  `git rev-parse --show-cdup` so the path stays relative; outside a
  repository the guide prints alone. A config that cannot be read, or notes
  that are there but cannot be read, is a warning on stderr, never a
  failure: the guide is what a session starts from. They write nothing.
  `itos go` alone then prints this clone's own notes (slice 68,
  `localNotes`): `notes.md` in itos's folder of the git common dir
  (`git rev-parse --git-common-dir`, as itos follow's threads are found),
  which git never commits and every linked worktree shares, for what holds
  on this machine only. They follow the repository's notes after another
  `---`, under `# This clone's own notes` and a line saying they are
  uncommitted, and are named by `local_notes` under `--json`; a file that is
  missing or holds only blanks prints nothing at all, and one that is the
  file `guide.orchestrating` resolved to (a stealth config naming it) prints
  once, as the repository's notes (`os.SameFile`). `itos guide coordinate`
  leaves them out. The coordinator's guide names the file, the one nudge to
  keep such notes there.
- **status** (slice 67, `internal/cli/status.go`) is where the work stands,
  built from what already records it and never written: the remote branch's
  head is asked of the remote (`git ls-remote` with `GIT_TERMINAL_PROMPT=0`
  and a 20 s timeout), the branch's upstream or origin's `HEAD` from a
  detached one, and read from `refs/remotes/` as last fetched when the remote
  does not answer; its header is the local commit's when it is fetched. Its
  CI run is one call of the `providers.Watch` that `watcher` builds for
  `itos ci watch`, so one look, never the loop; a run not done is `going`.
  The last nightly's run (slice 72, `readNightly`) is one call of the
  `providers.Nightly` that `NightlyProvider` builds from the same
  `ci.watch.provider`: `github` is the `GitHub` value with
  `ci.watch.github.nightly_workflow` and the head's branch, whose
  `NewestRun` lists the workflow's runs on the branch and reads the newest
  as `RunOf` reads a commit's (`newestOf`), with the token, repository and
  API address the watch takes (`watchGitHub`). A provider naming no nightly gives no
  look and no line; the head's line and the nightly's share `runLine`, so
  both word a result alike. The nightly is read even where the head is not
  (no remote, no branch yet), where `GITHUB_REPOSITORY` names the
  repository.
  While the head's run is going, did not pass or has not started (any
  result but `success`; not when it could not be read), the commit main
  last proved follows it (slice 73, `readLastGreen`): one call of the
  `providers.LastGreenLook` that `LastGreenProvider` builds from
  `ci.range`, the provider a push's range starts from. `none` gives no look
  and no line; `github` is the `GitHub`
  value with `ci.range.github`'s workflow, its token and
  repository found by `watchGitHub` as the watch's are (the environment,
  else gh and the remote's URL), so it reads outside CI, where
  `RangeProvider` finds neither, and the API's address; the look is given
  the branch's head as fetched (`refs/remotes/<remote>/<branch>`), and its
  `LastGreenFrom` walks from it as `NearestGreen` walks from a push's head
  (bug 24: the one walk, `greenFirstParent`, the head itself included here
  and its parent first there), saying what went wrong where the range reads
  it as no start. It reads no list of the branch's runs, for bug 23's
  reason. The commit's header
  is the local commit's when it is fetched, else the SHA stands alone.
  The newest release (slice 70, `readRelease`) is asked of the same remote
  with `git ls-remote --tags` under the same timeout, an annotated tag's
  peeled `^{}` commit standing for it, and read from `refs/tags/` as last
  fetched when the head was (the remote is not waited for twice) or the tags
  do not come; `release.Newest` picks the highest `vX.Y.Z`, three numbers
  and nothing else ordered numerically, the rule `tools/bin/release-version`
  picks the last release with from the tags HEAD reaches (bug 20): itos never
  cuts a prerelease, so a prerelease or build-metadata tag is never the newest
  release. The unreleased commits are `git log <release>..<head>` in this
  clone, kept where `release.Releasable` holds: a feat, a fix, or a `!` or
  `BREAKING-CHANGE` footer of any type, the rules `tools/bin/release-version`
  reads a commit with too (T-088). A head not fetched here lists to the remote branch
  as last fetched, with a line saying the list may be behind. status never
  fetches.
  The person is `work.Whoami`'s, as `itos work`'s, and the items are
  `work.Propose`'s: in progress, then `work.Startable`, the proposal's own
  and unowned items merged in `inQueueOrder`, cut to five; the questions are
  `loadAsks`'s open ones. Each part that cannot be reached appends a line to
  `standing.Unread` and to the lines printed, and the rest goes on. `itos go`
  (`goStatus`) computes it after the guide and prints it after a second
  `---`, or under `--json` as the `status` key; a config that is not there
  or not read, or a registry with problems, is one line on stderr instead.
- **The lock** (`internal/lock`, bug 16) is the one way itos keeps two
  writers of a file from losing a change: `lock.Hold(path)` makes
  `path.lock` with `O_EXCL`, which works the same on Linux, macOS and
  Windows (`flock` is not on Windows), retrying with a growing pause up to
  `lock.Wait` (10 s) and then failing with an error that names the lock file
  and says to remove it when no itos runs; `Release` removes it. It is held
  from before the read to after the write, around itos follow's threads
  (`heldThreads`) and, under a stealth config, around the registry, which
  every registry and ledger writer reads in `soundRegistry` and writes, and
  around itos ask's questions beside it (`heldAsks`):
  those files are in the git common dir, shared by every worktree, and the
  lock is `<registry>.lock` beside them. `work done` gives it back while it
  runs the task's checks and asks CI, and takes it again to make its change
  on the registry as it then is. A project's registry takes none: its writes
  are commits.
- **The hooks** are `hook commit-msg`, `hook pre-push`
  (`internal/cli/hook.go`) and `hooks install` (`internal/cli/install.go`),
  `hooks.ts` with `commit-data.ts`, `commit-scope.ts`, `commit-tasks.ts` and
  `pre-push.ts`. `hook commit-msg` is four rules in the TypeScript's order,
  the first to fail deciding, each a call to the judgement its own command
  makes: itos's data at commit is `configFindings` under
  `source.ReadingFrom(source.At("index"), …)` when a staged path (`git diff
--cached --name-only --no-renames`) is the config, a ledger file, the
  registry or a smoke set as the staged config names them, data that cannot
  be read one `data-unreadable` problem, the rejection's first line the
  staged `commits.reject_message`; the staged rule is `scope.Of(cfg).Issues`
  on the staged paths and each kind's `staged` range command (both only for
  a type `Ruled`), then `tests.NewMoves(cfg).Staged(type)`; the header lint
  is `message.LintFile`, the built-in lint under `use: builtin`, else the
  `commits.header_lint.hook` delegate with
  `{file}` as one `tests.ShellWord`, inheriting the hook's streams, its exit
  code the hook's, beside `FooterProblems` at the staged tree (or
  `ITOS_AT`); last, the named tasks' checks up to each task's first late one
  (`check.ChecksBeforeLate`), the ledger and the registry read from the
  index by the staged config, the checks' costs and timeouts by the working
  tree's, as the TypeScript read each. Each check runs through
  `check.RunCaptured`, `runCheckCaptured` ported: quiet, stdout and stderr in
  one temporary file rather than a pipe, so a command it leaves running
  cannot hold the commit past its timeout, the timeout capped by
  `hooks.commit_msg.check_timeout` (`Capped` says the cap set it), a
  timeout a failure whatever the code, in the hook's environment less
  `GIT_INDEX_FILE`. A failure rejects the commit when `work.ItemStatuses`
  says the task is `done`, and is only printed otherwise. `hook pre-push`
  reads git's ref lines, or under pre-commit or prek the line their
  environment gives, as `pushedRefs`: each ref's local commit and the
  remote's commit it replaces when this clone has it, a deleted ref left
  out. It verifies first (`verifyPush`, slice 46): verify's
  `verifier.over` on each distinct `pushedRange`, `<remote sha>..<local
sha>`, or `git.Unpushed` up to the local commit when there is no remote
  commit to compare with, each run's report written to a buffer, so a push
  whose commits pass prints nothing and the commands' output is all the
  hook says, as before. A failing range's report is printed on stderr with
  `fixAdvice` (`git commit --amend`, or `git rebase -i` from the parent of
  the first failing commit, `--root` for a root commit) and the hook exits 1
  with no command run. A config with no `commits` section has no rules to
  verify. Then it runs `hooks.pre_push`'s `per_base` once per remote base
  this clone has (`pushBases`), else `whole`, nothing for a deleted branch,
  and nothing at all without `hooks.pre_push`. `hooks
install` picks the manager (`--manager`, `hooks.manager`, then the markers)
  and writes or prints the one-line shims (`#!/bin/sh` and executable for
  plain git, written as a new file is), or prints a config-file manager's
  snippet. A shim's folder is `hookDir`'s: `.vite-hooks` or `.husky`, else
  the one `git rev-parse --git-path hooks` names, which `hookFile` takes as
  it is when absolute (an absolute `core.hooksPath`, or a linked worktree's,
  the main repository's) and under the working tree's root when relative,
  the name printed being the folder written (bug 35); `init`'s doctor reads
  the same files. A hook that is not a shim (`isShim`: one line calling `itos hook`,
  besides comments and a shebang) is replaced only with `--force`. Under a
  stealth config, unless `--manager` or `hooks.manager` names one, the
  manager is the git config (`internal/cli/gitconfig.go`, slice 33):
  `declareHooks` writes `hook.itos-commit-msg` and, when `hooks.pre_push` is
  set or the config is the stealth one (`declaresPrePush`, slice 79: the
  pre-push hook verifies the pushed commits whatever `hooks.pre_push` says,
  and a stealth user has no CI of the project's to judge them),
  `hook.itos-pre-push` into the repository's own config
  (`git config --local --replace-all`, its `.command` `<hooks.bin> hook
<event>`, `itos hook <event>` under a stealth config, git appending the hook's arguments, and its one `.event`), and
  removes an itos pre-push entry when it is not. Git runs those beside the
  hook in `core.hooksPath` or the hooks folder, so nothing of the project's
  changes. An entry already as written is `unchanged`; one under itos's name
  whose command does not call itos (`callsItos`) is replaced only with
  `--force`. Before writing, `configHooksRun` asks
  `git -c hook.itos-probe.event=commit-msg -c hook.itos-probe.command=true
hook list commit-msg` for the probe's name, testing the feature rather
  than a version, and a git that does not list it exits 3. The hooks
  this repository's git calls run the Go binary since T-060.
- **Every command is ported**, each taking its arguments as the TypeScript
  did, so a usage error read the same in both. While the groups landed, a
  command whose group had not exited 3 saying so; once every push held the
  Go build to the whole suite (T-053) that error went.
- **The help texts** are `internal/cli/help.go`, `help.ts` ported: every
  text in one table keyed by command path, as `HELP` is, not one beside each
  command, so the two read side by side while both lasted. `itos`,
  `itos help …` and any `--help` print the longest command path the table
  knows, before any config is read. A change to a text lands with its case in
  `help.yaml`, which CI's corpus step runs on every push.
- **The version** is the release's tag, held nowhere in the tree (T-069). A
  release's binary is stamped by GoReleaser with its tag's version; a build of
  a checkout with what `tools/bin/dev-version` reads from `git describe`: the
  release's version at its tag, `X.Y.(Z+1)-dev.N.g<sha>` N commits after the
  newest `vX.Y.Z` tag (`0.0.1-dev.N.g<sha>` with none, `0.0.0-dev` outside
  git), a pre-release of the next patch, so it sorts above the last release
  and below the next, and all pre-release with no `+` build metadata, since
  `pin.version` and the launcher's own version read a pre-release and refuse
  a `+`. A config's `requires` reads `X.Y.Z` alone, so such a build cannot be
  named by one; the corpus's `{{release}}` is that version's `X.Y.Z` for the
  cases that try. `tools/bin/build-go.ts <out dir>` builds `./cmd/itos` into
  `<out dir>/itos` with `CGO_ENABLED=0` and `-trimpath`, stamping that
  version into `internal/version` (`-ldflags -X`). A binary built without the
  stamp says the module version Go records (`go install …@v<x>`), or
  `(devel)` when there is none. Every self-test builds the binary through it.
- **The release build** is GoReleaser's (`.goreleaser.yaml`): the same build
  once per platform
  `docs/decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md`
  lists, each binary packed with `LICENSE`
  and `README.md` at the top level of `itos-<version>-<os>-<arch>.tar.gz`
  (`.zip` for windows), every entry root's by number and dated at the commit,
  and `itos.schema.json` (below, written by a `before` hook into
  `.tools/release/`) and `checksums.txt` in `sha256sum`'s format beside them,
  naming the archives and the schema: what a release uploads, named and laid
  out as every release since v1.0.0 (with the TypeScript tarball's line added
  until it left, T-062). `tools/bin/build-go.ts --release <out dir>` runs it
  as a snapshot (`goreleaser release --snapshot`, into `dist/goreleaser`)
  stamped with the checkout's version, or `ITOS_SNAPSHOT_VERSION`, and copies
  those seven files into `<out dir>`. `tools/selftest/go-release.ts` builds
  them into a scratch folder and reads them back with the system's `tar`,
  `unzip` and `sha256sum`, the version read from the archives' names; the
  nightly runs it (`ci.nightly.steps`), not every push. With `--dir <dir>`
  (and `--version`) it builds nothing and proves a folder already built.
  `tools/selftest/release-cut.ts`, T-069's check, proves the version over
  scratch histories, builds a snapshot as the version this tree would
  release, proves it with `go-release.ts --dir`, reads each archive's name as
  the launcher's `archiveLine` does, holds the names, the linux and windows
  archives' entries (modes and owners) and `checksums.txt`'s line format to
  what the last release published, and writes and proves its notes.
- **The config's JSON Schema** (`itos.schema.json`, draft 2020-12, for an
  editor: `# yaml-language-server: $schema=<its release URL>` atop an
  `itos.yaml`) is generated, never kept by hand: `tools/bin/config-schema`
  (`go run`, from `build-go.ts`'s `buildSchema`) converts `config.Schema()`,
  the schema table as exported data with each key's words (`about` in
  `schema.go`), and lays `DefaultsFor(nil)` on it as `default`s, refusing a
  default the schema has no key for. An object refuses an unknown key, as
  config check does; what the schema cannot say (version 1, the cross-checks)
  it leaves to config check, so it says less, never something different.
  `tools/selftest/go-schema.ts` generates it and validates with ajv: this
  repository's `itos.yaml` and the configs `config.yaml` calls sound pass,
  every corpus config it refuses is one itos refuses, and a misspelt key, a
  wrong type and a value not allowed are refused. The nightly runs it.
- **This repository runs it** (T-060): `tools/bin/itos`, which the hooks,
  CI's steps, the ledger's checks and the nightly call (`hooks.bin`), is a
  POSIX sh script that builds `./cmd/itos` into `.tools/bin/itos` (ignored)
  as `build-go.ts` does, stamped with `tools/bin/dev-version`'s version (sh
  and git, so it needs Go and git but never Node), which it records in
  `.tools/bin/itos.version`, and `exec`s it. It builds when the binary is
  missing, when anything under `cmd/` or `internal/`, `go.mod` or `go.sum`
  is newer than it (`find -newer`; a folder counts, so a removed source does
  too), and when HEAD's version is not the recorded one (a commit, a tag): a
  restamp is a link, from Go's build cache. A fresh binary costs one
  `git describe` and that one `find`, about 10 ms. The build goes to a name of its own and is moved over the
  binary, dated from before it started, so concurrent calls never see half a
  binary and a file changed during it still reads as newer. A build that
  fails prints `itos: the Go binary does not build (go build ./cmd/itos):`
  and the first error, the rest indented below, and exits 3, never running
  the old binary. CI and the nightly build it once, as the step `Build
itos`, before any other step calls it. `tools/selftest/go-dogfood.ts`
  proves it with a PATH of links that holds go, git and sh but no node.
  CI's conformance step and the features (their default `ITOS_BIN`) run it.
- **The ported set is the whole suite.** `tools/selftest/go-port.ts` builds
  the binary into a scratch folder and runs the whole corpus through
  `run.ts --bin` and every feature as `go test ./features -count=1` with
  `ITOS_BIN` pointed at the build, and fails, naming which, when the build,
  the corpus or the features fail. While the groups landed it held a list
  each group added its part to (T-039 to T-052); once the list named
  everything it went, and the script became a late step of every push's CI,
  so a behaviour change landed in both implementations in one push (T-053).
  With the TypeScript gone (T-062) it left CI's steps, since the corpus step
  and the features step run `tools/bin/itos`, the same tree, and it is the
  port's tasks' check. Those tasks' checks that build the binary and run part
  of the corpus by `--only` are skipped after the corpus step (`ci.covers`).
- **Its commits.** The port's code was `refactor` with a `Task:` footer: the
  behaviour was the TypeScript's, already specified; a change to what itos
  does is now a `feat` or a `fix`. The Go files under `cmd/` and `internal/`
  are `commits.path_sets.implementation`, and their `*_test.go` are the
  `test` type's; CI's `gofmt` step covers them and a late
  step runs their unit tests (`go test ./cmd/... ./internal/...`), of which
  the hooks run those a change reaches (T-059).

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
    the registry) or a feature file, the dependency check when `go.mod` or
    `go.sum` is staged (below), then the unit tests the change reaches
    (`tools/bin/go-unit-tests --cached`), then `fallow audit` on what is new
    against HEAD, which reads this repository's own TypeScript tooling (the
    self-tests, the corpus runner, the builds) and scores it by complexity,
    with no coverage. The unit tests are itos's Go packages' (the TypeScript's
    Vitest suite left with it, T-062), chosen without building anything: a changed file belongs to the
    deepest package folder holding it (testdata and embedded files count),
    and one `go list -e -test -deps` over `./cmd/... ./internal/...` gives
    every package whose Deps, its tests' included, reach one of those, which
    `go test` then runs, Go's test cache answering for what did not change;
    `go.mod` or `go.sum` runs them all, and a change under neither `cmd/` nor
    `internal/` starts no `go` at all. `features/` is not among them: it is
    the named tests, CI's and the nightly's. It does not check itos's own data: the commit-msg
    hook does, from the staged tree (T-023 dropped T-022's check here, which
    read the working tree).
  - **The dependency check** (`tools/bin/deps-check`, T-067; by hand,
    `go run ./tools/bin/deps-check`) runs where a Go module can come in: at
    pre-commit when `go.mod` or `go.sum` is staged, and in CI as the first late
    step with `-changed-since "$FROM"`, the range's start the workflow puts in
    the job's environment, so a range that leaves both alone checks nothing. It
    lists every module of the build, the tools' included (`go list -m -json
all`; a module replaced by a version is checked as that version, one
    replaced by a folder not at all), asks GOPROXY for each one's
    `@v/<version>.info` (never a module GONOPROXY names, which the go command
    never asks about either), and refuses one published less than 7 days ago,
    or that no proxy knows, unless `deps-check.json` at the root excepts that
    module and version with a reason; an exception that excuses nothing (its
    version is not in the build, or is old enough) fails too. Only then does it
    run `go tool govulncheck -test ./...`, a tool dependency in `go.mod` and so
    held to the same age, as running it runs its code; a finding (exit 3)
    fails the check. It needs the network, as the `go get` that changed
    `go.mod` did, and costs a few seconds with a warm build cache (the first
    run builds govulncheck). It imports only the standard library, so no
    module it is about to refuse runs inside it, which is why its file is JSON.
    `tools/selftest/deps-check.ts` proves it against a proxy of local files
    with a stub govulncheck.
  - **The schema contract** (`tools/bin/schema-contract`, T-070; by hand,
    `go run ./tools/bin/schema-contract`) holds the config to the last
    release's, as T-069's releases take their version from the commits alone:
    a config the last release accepts must still be accepted, and mean the
    same, unless a commit says otherwise. It runs in CI as a late step with
    `-range-from "$FROM"`, checking nothing unless the range has a `feat` or a
    `fix`, the commits a release is cut for. The last release is the newest
    `vX.Y.Z` tag reachable from HEAD (`git tag --merged HEAD`, by version), its
    schema the `itos.schema.json` of its GitHub release, downloaded
    (`GITHUB_REPOSITORY`, else the remote origin, names the repository), and
    this tree's is `go run ./tools/bin/config-schema`'s. The comparison walks
    the two over the keywords that generator writes, an `anyOf`'s entries
    matched by type: a key removed, a type narrowed, an enum value removed, a
    key newly required, an object closed or a default changed (added, removed
    or another value: a config's default is behaviour) is breaking, a key
    added, a type widened, an enum value added, a key no longer required or an
    object opened compatible, each printed with its config path
    (`ci.steps[]`, a map's entries as `tests.<key>`). A breaking change fails
    (exit 1), naming the key, the change and the remedy, unless a commit in
    `<tag>..HEAD` carries a `BREAKING-CHANGE:` (or `BREAKING CHANGE:`) footer
    in its last paragraph or a `!` before its header's colon. It never passes
    on what it could not read: a failed download or a release without the
    asset, a shallow clone (which may lack the tag) and a keyword the
    comparison does not read each stop it with exit 2; no release tag at all is
    the one pass without a comparison, and says so. Standard library only, as
    the dependency check is. `-release <tag>` names the release to compare
    with, and `-json` prints `{"release","findings":[{"path","change","breaking"}]}`
    instead of lines, both for the release notes. `-old` and `-new` take the
    two schemas from files, which `tools/selftest/schema-contract.ts` does, in a scratch repository
    with a tag and a local server standing for GitHub.
  - **The plugin's version rule** (`tools/bin/plugin-version`, T-074; by
    hand, `go run ./tools/bin/plugin-version -range-from <rev>`) holds a
    change to the Claude Code plugin to a raise of its version, since Claude
    Code offers an installed plugin an update only when that version changes.
    It runs in CI as a static step with `-range-from "$FROM"`, and as a prose
    step too, as a range that touches only Markdown in the plugin's folder
    is prose-only (its skill's, until T-092 dropped it). It compares the trees at the range's start and at HEAD (`git
diff --name-only --no-renames`, so a change undone within the range is
    none) under the plugin's folder (`-plugin`, `integrations/claude-code` by
    default, `.` for a plugin with a repository of its own); when they differ,
    `.claude-plugin/plugin.json`'s `version` at HEAD must be higher by semver
    2.0.0's precedence than at the start (build metadata does not count, a
    pre-release sorts below its release), or it fails (exit 1) naming the
    files changed, both versions and the patch, minor and major to raise it
    to. A plugin.json new in the range passes with any version; one gone, or
    whose version is not semver, fails. A range that leaves the folder alone
    passes, saying so. It never passes on what it cannot read: an empty range
    start (no green run to start from, or a range provider that failed), a
    start that is not a commit of the clone, and a shallow clone each stop it
    with exit 2. Standard library only, git alone, no network.
    `tools/selftest/plugin-version.ts` proves each case in a scratch
    repository laid out as this one, and that both step lists run it.
  - **The last release's suite** (`tools/bin/previous-release`, T-071; by
    hand, `go run ./tools/bin/previous-release [-bin <itos>]`) holds this
    tree's itos to what the last release's scenarios and corpus promised, as
    a scenario edited alongside a change no longer checks what it said. The
    release is the newest `vX.Y.Z` tag reachable from HEAD, as for the schema
    contract, and the check runs in CI as a late step after this tree's corpus,
    with `-range-from "$FROM"`. The binary judged is this tree's `./cmd/itos`,
    built into the scratch folder stamped with the version
    `tools/bin/release-version` computes (the next patch when nothing is
    releasable), as a release's is, unless `-bin` names another:
    `tools/bin/itos` says a pre-release between releases, which the old
    corpus's cases that put the binary's own version into `requires` cannot
    read. It checks the tag out with `git worktree add --detach` into a
    scratch folder, links the checkout's `node_modules` in for the YAML
    parser its fixture rewrite reads with, leaves in the release's fixtures
    only what itos's contract holds a case to, then runs, at once, the
    release's `go test ./features -count=1 -json` with `ITOS_BIN` naming the
    binary and this tree's corpus runner over the release's fixtures,
    `node <top>/tools/itos/conformance/run.ts --bin <bin> --additive --only <its *.yaml>`
    (T-076): the runner asks the binary its version for the corpus's
    `{{version}}`, so a build stamped with a version the tag never had is not
    failed for saying it. The release's scenarios are judged as they stand,
    by what their steps assert. Its corpus is judged by machine output alone
    (T-100, decision 35): only exit codes, `--json` less every key named
    `message` or `fix`, and the files itos writes are its contract, every
    plain-text output, help, a refusal or a success alike, being for people.
    A Node script in the command, with the linked `yaml`, rewrites each
    fixture from what it parses: every case's `stdout`, `stderr`,
    `stdout_has` and `stderr_has` go, and every `message` and `fix` key at
    any depth of its `json`; its exit code, `files_after`, every other `json`
    key and what it sets up stay. The rewritten file must parse back equal to
    what the script readied, or the check stops, and the check prints how
    many cases it read, how many lost their plain output and how many
    message and fix keys it left out (v5.1.0: 872 cases, 704 and 245).
    `--additive` judges what is left by what the new output must still hold
    (the user's call, 2026-10-03: additive properties are compatible,
    removals and changes breaks): its `json` as a subset of the new, every
    key it expects there with the value it expects at every depth, an array
    element by element at the same length, a key added anywhere passing, a
    failure naming the path (`.plugin.scope: expected "user", got
"project"`). So a rule id renamed, a problem dropped, a key removed, an
    exit code changed or a file written otherwise is a break, and a reworded
    refusal, help text or message is not. A help case and a usage error are
    judged as any case is: the help-case exemption (2026-10-03) and the
    usage-error one (T-095) were narrower forms of this rule and went with
    T-100. A usage error the command comes to accept changes its exit code,
    and fails as any changed exit code does. No release's runner has the
    mode, so the release's fixtures are read by this tree's runner, not its
    own, and this tree's corpus, run without the flag or the rewrite, pins
    every output exactly. A failing subtest
    (`TestFeatures/<name>`, spaces as underscores) is named by the `@ID-` tag
    above its `Scenario:` line in the release's feature files, a failing case
    by the runner's `FAIL <file>: <name>` line as `<file base name>: <name>`.
    A failure is accepted when a commit in `<tag>..HEAD` is breaking (read
    as the schema contract reads it) or is a `fix` whose `Changes:` footer
    names it: an entry a line, scenario IDs (`Changes: @ID-CMSG-03`) or one
    case (`Changes: hooks.yaml: <case name>`), read from the message's last
    paragraph. A case entry names the case of that file whose name is the
    entry's, or else the one case whose name starts with it (T-081), so a name
    longer than the header lint's 100-character footer line is given by a
    prefix; the names are the release's fixtures' as they parse, which the
    script that readies its corpus prints, and a prefix more than one case
    starts with names none and is warned of as ambiguous. A `feat` naming it
    does not count, and the refusal says so.
    `Changes:` is a footer of free text in `commits.footers`, so
    `itos commit --changes` writes it and an ID the fix removed is not refused
    at the commit; the check warns about an entry that names nothing of the
    release instead of failing, since a pushed commit cannot be rewritten.
    Exit 1 lists each refused failure, what its suite printed and the remedy;
    exit 2 for a shallow clone, a tag that cannot be checked out, or a suite
    that cannot run or whose failure it cannot name; no release tag passes,
    saying so. `tools/selftest/previous-release.ts` proves it in a scratch
    repository whose tag holds two scenarios (a stdlib Go test standing for
    godog's), this repository's corpus runner, four cases (one named longer
    than a footer line, and two starting with the same word) and two help
    cases, against a script that breaks one scenario and one case by its exit
    code, and one whose help and usage errors' words alone differ, which
    passes; a report's lines and a JSON object with a message and a fix,
    against a script that adds keys, rewords the message and fix and
    rewrites the report, which passes, and three that each take a key away,
    change a value or change an exit code, each refused; and two usage errors
    and a config error, against a script that rewords all three and gives a
    usage error exit 1, refused for that usage error alone. Its unit tests
    run the script on a scratch fixture, plain output and message and fix
    keys leaving and the rest staying, then run the readied fixture against
    the test binary standing in for an itos that rewords everything: the
    reworded stdout, stderr, message and fix pass, a changed exit code, rule
    id, removed key and files_after fail.
  - **commit-msg** first checks itos's own data when the commit stages any
    of it (`commit-data.ts`): the config, a ledger file, the registry or a
    smoke set, as the staged config names them, runs `config check`'s
    problems over the index, the registry's own check among them, and rejects
    the commit with them, since a project's pre-commit hook passes them as
    prose and the working tree may hold what the commit does not. It comes
    first because the other rules read the config. Then it applies the
    type's path rules (`commit-scope.ts`), then outside `feat` and `fix` the
    scenario moving rule (the built-in, `moves.ts`), then the header lint:
    the built-in one (`use: builtin`, `config-conventional`'s rules; this
    repository delegated to commitlint until T-063), and after it, always, itos's footer rules (`commit.ts`, one `<key>-footer`
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
  - **pre-push** verifies the commits each pushed ref adds, as `itos verify`
    does (slice 46), printing nothing when they pass and refusing the push
    when one fails; then it runs `hooks.pre_push`: `tools/bin/go-unit-tests <remote
sha>` (the base against the working tree) for each pushed ref, or the
    whole unit suite (`--all`) when there is no remote commit to compare
    with. The package choice is the script's alone, shared with pre-commit. Nothing else: the
    scenarios and the task checks are CI's.
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
  `go.mod`, and builds the Go itos once (`Build itos`) before any step calls
  it. Claude Code, which T-066's checks run, is installed (the latest release,
  by its own native installer, into `~/.local/bin`) only when the plan for the
  range (`itos ci plan --json`) will run a check that starts with `claude`;
  neither of those checks needs a login, the step is handed no token, and
  the nightly never installs it, since those checks are late.
  **The plan** (`ci-plan.ts`, its cost rule in `cost.ts`; `itos ci plan <from> <to>` prints it, running
  nothing) is one sequence in cost order: the static steps of `ci.steps`
  (`vp check`, `gofmt`, `go vet`, the smoke rule, `itos config check`, the plugin's version rule) and every named task check
  that is static (its own `cost: static`, else a pattern of
  `ci.cost.static`: `matchesStatic` in `config.ts`, which `config
check`'s written-order rule reads too); then the late steps (the dependency check, for a range that changes
  `go.mod` or `go.sum`; the whole unit suite, the Go packages'; the audit, the conformance corpus against `tools/bin/itos`, T-007); then **one run of the features** over the smoke
  set (of the kind the `tests:` step names; a CI without one reads none), the scenarios the `Scenarios:` footers name and the subsets of the
  tasks the ledger footers (`Task:`) name; then the named tasks' late checks. A task's checks
  keep their written order. A check a step has just done is skipped
  (`ci.covers`), one in `ci.nightly_only` is left out of the push, one marked
  `after: push` is listed as pending and neither run nor counted (bug 28: it
  means something only once the push has landed, so `itos task` and `itos
work done` run it), and a task whose work item is still `todo` waits
  (`ci.wait_on_status`). Every one of
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
  Markdown; the registry and the ledger, under `tasks/`, are not prose; and
  the plugin's version rule, since a change to the plugin can be Markdown alone) and the named tasks' static and `prose: true` checks,
  and no features.
- **The platform jobs** (`ci.yml`'s `platform`, T-072) run beside it on every
  push, a matrix of `ubuntu-latest`, `macos-latest` and `windows-latest`,
  each named `platform (<runner>)`: itos built natively, stamped as
  `tools/bin/itos` stamps it, into `ITOS_BIN` (`itos.exe` on windows, where
  Go runs no shell script and `tools/bin/itos` is one), then the Go unit
  tests and every feature against that build, in Git for Windows' `bash` on
  windows, the features whatever the unit tests said. They block, and the
  `release` job needs them, so a platform break stops a release; the releases
  are still built on Linux alone. Their actions are pinned by commit, as
  `release.yml`'s are.
- **The nightly** (`.github/workflows/nightly.yml`, at 11:44 UTC on `main` or
  by hand) runs `itos ci run --nightly`: `ci.nightly.steps` in written order,
  here every feature, then the gates self-tests (`gates.ts`, `go-hooks.ts`),
  the Go release build (`go-release.ts`) and the config's schema
  (`go-schema.ts`), then the static checks of every done task, then T-069's
  check that the newest release's linux-amd64 archive passes
  `gh attestation verify`, which `ci.nightly_only` keeps out of pushes: it
  passes only once the release job has cut a release, which needs a green
  push, and a push's range starts at the last green run, so in pushes it
  would hold every one red; then T-072's, that the newest completed CI run on
  `main` passed its three platform jobs, also kept out of pushes, since a
  push's own run is still going when its task checks run. The done tasks' checks are the step `{ tasks: done, cost: static }`
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
  `gates.ts` runs the real hooks in a scratch worktree and shows that CI runs
  the whole unit suite and the features they leave to it, and that CI's steps
  catch what they let through (a refactor that changes what itos prints, in a
  Go package whose unit tests do not read it, passes both hooks and fails the
  push's features step); `go-hooks.ts` that the hooks run exactly the Go unit
  tests a change reaches, which the nightly runs beside it;
  `ci-scope.ts` and `features-scope.ts` prove the scope and that the plan's
  features command runs exactly the scenarios it claims, by go test's own
  record of what it ran (`-json`). They ask `tools/bin/itos` for the plan and
  the tests (`ci plan --json`, `ci scope`, `tests list --json`), so they hold
  the binary to this repository's config; the github provider's choice of
  run, which no command reaches without a network, is the walk's
  `TestNearestGreen*` and `TestLastGreenFromWalksFromTheHeadAndSaysWhatWentWrong`
  in `internal/providers` and bugs 23's and 24's scenarios. T-008's check
  still runs `TestFirstGreen`, the newest success of a list of runs, which
  nothing reads since bug 24 (`p1-t008-check-the-walk`).
  `config-gate.ts` proves itos's data is checked where it is guarded: in a
  scratch worktree, running both hooks as git does, the commit-msg hook
  rejects a commit staging a ledger with a misspelt key and passes a sound
  one, a commit staging only `docs/**` and `tasks/**` runs no unit tests, and
  CI's plan runs `itos config check` for a range touching
  the registry or the ledger and for a prose-only range touching
  `CONTRIBUTORS.md`. It and `gates.ts` build their worktree with
  `scratch.ts`; `gates.ts` takes the worktree's `node_modules` away for the
  commit-msg hook's header case, so the built-in lint is shown judging a
  header with no commitlint to run. `header-agreement.ts` holds the built-in header
  lint to a fixture of commitlint's verdicts (above, `internal/message`).
  `release-notes.ts` proves what a command
  can of a release's notes (generated ones by `--notes <file>`, which
  `release-cut.ts` passes; a tagged release's from its committed file, or its
  GitHub description when it has none): the notes are there; their last `##` section is
  "Upgrading" with the pin to change, from v2.0.0 the binary's install lines
  (`version=<version>`, the download from the release, `checksums.txt`) and no
  tarball, before it the pin line naming that version's tarball; every
  `Upgrading:` footer `itos commit footers` lists for the commits since the
  last release appears in that section, whitespace and case aside, and so
  does every `Changes:` entry; and every
  config key whose default differs between the last release (its Go archive
  for this machine, or before v1.0.0 its tarball, downloaded from GitHub,
  verified and run) and this tree, by `config check --print-defaults --json`,
  is named in that section.
- **The changelog** (`tools/changelog.ts`, `cliff.toml`): git-cliff groups
  the Conventional Commits by type, each with its footers and body, into
  `docs/changelog/`, which git, the formatter, the linter and the audit
  ignore; `-- --task <id>` and `-- --scenario <id>` print one footer's
  commits.
- **Agents' worktrees** live under `.claude/worktrees/`, ignored by git, and
  left out of the linter and the formatter: their files are
  theirs, often half-written, and never this checkout's.
