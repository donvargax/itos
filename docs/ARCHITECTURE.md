# Architecture

A map of the repository as built: what lives where, how a run goes, where to
look, and the rules that cross packages. The detail is beside what it
describes:

- **What a Go package does, how and why** is its doc comment (`doc.go`,
  `go doc ./internal/<pkg>`), changed with its code: read it before touching
  the package. A program, a workflow or `itos.yaml` says its own in comments.
- **Why it is so** is a decision record ([`decisions/`](decisions/README.md)
  lists them), a task's `why` in `tasks/*.yaml`, a scenario's comment.

## The layout

- `cmd/itos`, the binary, and `internal/`, its packages: together the
  implementation path set ([ADR-0003]).
- `features/`: the named tests, Gherkin with Go steps, and the smoke set.
- `tools/itos/conformance/`: the regression corpus; `tools/itos/fixtures/`,
  what the ledger's negative proofs hand itos.
- `tools/bin/itos`: what the hooks, CI and the ledger's checks call; it builds
  `./cmd/itos` from the tree and runs it.
- `tools/bin/<name>/`: the gates, the release's programs and the inbox, each
  `go run ./tools/bin/<name>`; beside them the shell scripts (`pre-commit`,
  `go-unit-tests`, `pinned`, `dev-version`, …). `tools/selftest/`: the gates'
  self-tests that test no one program.
- `itos.yaml`: this repository's policy. `tasks/`: the ledger
  (`phase-<n>.yaml`), the work registry (`work-items.yaml`), the questions
  (`asks.yaml`). `docs/`: this map, `PHASES.md`, `ORCHESTRATING.md`, `CLI.md`,
  the decisions.
- `integrations/claude-code/`: the Claude Code plugin.
- `.github/workflows/`: `ci.yml`, `release.yml` (which `ci.yml` calls),
  `nightly.yml`. `.goreleaser.yaml`: the release build.
- `deps-check.json`: the dependency check's exceptions. `.devcontainer/`: a
  sandbox for Claude Code.

## How a run goes

```
git (the shim's link) ─┐
itos ──────────────────┴─> cmd/itos ─> shim? ─> launch ─> cli ─> command
                                                  │
                                       a pinned release, fetched,
                                       checked and run in its place
```

1. **The git shim** (`internal/shim`): run as `git` through the link
   `itos git-shim install` makes, it hands every git command to the real git,
   except, in a repository itos manages, `git commit` and `git push`, which
   become `itos git-shim run`, and a `git add` staging more than the files it
   names, which it refuses.
2. **The real git** (`internal/vcs`): `Export` sets `ITOS_GIT`, so
   nothing the run starts reaches the shim again.
3. **The launcher** (`internal/launch`): runs the release a repository pins,
   fetched into a cache and checked against the pin's checksums, in its own
   place, or runs itself ([ADR-0024]). A global itos is this launcher.
4. **The command line** (`internal/cli`): the flags, the extensions
   (`itos-<name>` on the `PATH`), the move to the repository's top, then the
   command: a slice, `internal/slices/<name>`, its ports bound in
   `internal/cli`, whose doc heads each command group.

What every command meets:

- **the config** (`internal/config`): `itos.yaml`, or the stealth config,
  laid over one table of defaults and read through `config.Load` alone;
- **the data**: the ledger, registry, people, smoke sets, questions and
  decision records, read through the tree reader's port
  (`internal/store/tree`) from the source `config.Loaded` carries, the
  working tree, the index (the commit-msg hook) or a commit (`--data-at`);
- **the providers** (`internal/providers`): the world outside the
  repository, a push's range start, a commit's CI run, who a session works for;
- **the shell** (`internal/shell`): every command the config or the ledger
  gives runs through the config's `shell`; itos's own git calls do not;
- **the output** (`internal/out`, `internal/kind`): `--json` is one object
  with `"schema": 1`, a problem a sentence, a rule id and a fix, and the exit
  code comes from the error's kind alone ([ADR-0036]).

## Where to look

- A command's help and flags: its slice's `Command` (`internal/declare`).
- What several commands share: `internal/engine/<name>`, its doc saying
  what.
- Running a pinned release, the update notice: `internal/launch`; release
  addresses, checksums, the newest tag: `internal/release`.
- The config, its defaults, stealth mode, `commits.since`: `internal/config`.
- The ledger: `internal/ledger`; running checks: `internal/check`.
- The commit message (footers, header lint, notes): `internal/message`; the
  paths each type may touch: `internal/scope`, `internal/glob`.
- Named tests, adapters, run templates, the smoke set, the moving rule:
  `internal/tests`.
- CI's plan and its driver: `internal/plan`, `internal/ci`; a run's code
  proof: `internal/proof`.
- The registry: `internal/work` judges, `internal/store/tracker` reads,
  locks, writes and reserves IDs (`internal/idcounter`, [ADR-0045]).
- Questions and decision records: `internal/ask`, `internal/adr`; follow-ups
  and drafts: `internal/slices/followup`, `internal/draft`.
- The guides `itos go` and `itos guide` print: `internal/guide`.
- The Claude Code guard: `internal/slices/guard`.
- YAML read as itos reads it, and edited in place: `internal/value`.
- New IDs' numbers, the version: `internal/nextid`, `internal/version`.

## Rules that cross packages

- **Policy is config, never code** ([ADR-0001]), and every key accepted is
  one a tool reads.
- **One table of defaults.** No reader has a fallback of its own; a default
  applies exactly when `config check --print-defaults` prints it.
- **One reader per thing**, shared by the hook, verify, the plan and the
  commands so they cannot disagree: the footers (`internal/message`, by each
  footer's `source`, never its key), feature files (the Gherkin adapter), the
  config (`config.Load`), a commit's type (`message.Type`), a run of named
  tests (`tests.CommandFor`).
- **Every git is the real git**, run by `internal/vcs/adapter` alone. Paths
  are read NUL-separated, changes with `--no-renames` and no type filtered
  out, so the hook, verify and the plan agree on what a commit touches; a
  merge is judged by its own changes.
- **A gate never passes on what it could not read**: a shallow clone, a range
  start that is no commit, a release it cannot fetch exit 2 or 3, never 0.
- **Files people edit are edited in place** through `internal/value`'s
  editors, which keep comments and quotes and refuse an edit that does not
  read back as meant; itos's own files are written whole.
- **The registry is written by commands, never by hand**, each committing its
  change alone ([ADR-0034]); under a stealth config nothing is committed and
  the store's lock orders writers.
- **Stealth mode** is known to few places: the config, the source, the
  footers (git notes), `itos commit`, `itos push`, the unpushed range.
- **itos's words follow its first implementation**, TypeScript: YAML by the
  1.2 core schema, globs, whitespace and quoted values as JavaScript had them,
  since the corpus pins those words.
- **Only machine output is contract** ([ADR-0035], [ADR-0040]): exit codes,
  `--json` less `message` and `fix`, the files itos writes. The commands the
  plugin calls are never renamed or removed ([ADR-0041]).
- **A behaviour change is a `feat` or a `fix`** with its scenario and, where
  the corpus records the old behaviour, its case.
- **The gate programs use the standard library alone**, so no module they
  judge runs inside them.

## The features

What itos does is Gherkin in `features/`, run by godog against the binary
`ITOS_BIN` names, each scenario in a scratch repository of its own
([ADR-0004]); `features/README.md` holds the rules, `features_test.go`'s
comment the harness. The conformance corpus records what itos did, case by
case; `tools/bin/conformance` runs it and its `corpus` package documents a
case ([ADR-0006]).

## The release

CI cuts a release from a green push to `main`, never a person, and the tag is
the version ([ADR-0021]). `release.yml`'s comments give the steps, its
programs and `.goreleaser.yaml` theirs. What it publishes is [ADR-0022]'s,
its notes [ADR-0026]'s.

## The Claude Code plugin

`integrations/claude-code/`, the one plugin of this repository's marketplace
([ADR-0027]): the titles (`hooks/register.ts`), which run only
the `itos` on the `PATH`, never what a cloned repository ships, as its comment
says. It no longer guards Bash (T-320; [ADR-0028]). It has no skill:
itos's guides say how to work with itos ([ADR-0033]). `tools/bin/inbox` reads
consumers' issues, its `main.go` saying whose it trusts.

## The gates and CI

Every rule a command can decide is itos's, run by the hooks and again by CI
on every pushed commit. `itos guide commit` prints the commit rules, and
`itos ci plan <from> <to>` what CI runs (`internal/plan`).

### The hooks

`itos hook install` declares `itos hook commit-msg` and `itos hook pre-push`
in the clone's git config ([ADR-0037]), running `hooks.bin`, `tools/bin/itos`
here ([ADR-0014]). What they judge is `internal/cli`'s doc, "The hooks"
([ADR-0012], [ADR-0013]). pre-commit is the project's, `tools/bin/pre-commit`.

### CI

`ci.yml` wraps `itos ci run`, so it all runs locally too; its comments say how
the range and the platform jobs go. `nightly.yml` runs `itos ci run --nightly`
and every feature on three platforms. Each gate program is documented in its
`main.go`, the self-tests in `tools/selftest/scratch`. Mutants judge the tests,
not coverage ([ADR-0044], [ADR-0049]).

[ADR-0001]: decisions/0001-defaults-follow-common-conventions-and-configuration-is-the-escape-hatch.md
[ADR-0003]: decisions/0003-the-go-packages-are-the-implementation-path-set-and-the-corpus-stays-in-tools-itos.md
[ADR-0004]: decisions/0004-itos-s-named-tests-are-gherkin-features-run-by-godog-against-the-binary.md
[ADR-0006]: decisions/0006-the-conformance-corpus-is-a-regression-corpus-run-by-a-ci-step-of-its-own.md
[ADR-0012]: decisions/0012-the-commit-msg-hook-validates-itos-s-own-data-from-the-staged-tree.md
[ADR-0013]: decisions/0013-the-commit-msg-hook-runs-last-the-checks-of-the-tasks-a-commit-names.md
[ADR-0014]: decisions/0014-a-command-pattern-reads-hooks-bin-as-itos.md
[ADR-0021]: decisions/0021-ci-cuts-a-release-from-a-green-push-judged-against-the-last-release.md
[ADR-0022]: decisions/0022-itos-is-distributed-as-release-archives-with-checksums-installed-pinned.md
[ADR-0024]: decisions/0024-a-global-itos-is-a-launcher-that-runs-the-version-a-repository-pins.md
[ADR-0026]: decisions/0026-a-release-s-notes-are-generated-from-its-commits-and-end-with-a-complete-upgrading-section.md
[ADR-0027]: decisions/0027-this-repository-is-a-claude-code-plugin-marketplace-for-the-itos-plugin.md
[ADR-0028]: decisions/0028-the-plugin-guards-an-agent-s-git-commit-and-git-push-with-a-pretooluse-hook.md
[ADR-0033]: decisions/0033-itos-s-own-guides-say-how-to-work-with-itos-not-the-plugin.md
[ADR-0034]: decisions/0034-the-work-registry-is-an-index-and-a-why-lives-in-the-spec.md
[ADR-0035]: decisions/0035-only-machine-output-is-itos-s-contract-exit-codes-json-less-message-and-fix-and-the-files-it-writes.md
[ADR-0036]: decisions/0036-v6-0-0-renames-commands-by-the-cli-guidelines-parses-every-flag-from-a-spec-and-takes-exit-codes-from-the-error-s-kind.md
[ADR-0037]: decisions/0037-itos-installs-its-hooks-only-in-the-git-config-and-knows-no-hook-manager.md
[ADR-0040]: decisions/0040-a-written-file-is-contract-by-its-data-not-its-comment-lines.md
[ADR-0041]: decisions/0041-the-commands-the-claude-code-plugin-calls-are-part-of-the-contract-never-renamed-or-removed.md
[ADR-0044]: decisions/0044-no-coverage-threshold-mutants-judge-the-tests-and-fail-uncovered-refuses-untested-changed-code.md
[ADR-0045]: decisions/0045-every-id-itos-writes-is-minted-and-the-counter-is-claimed-on-the-remote.md
[ADR-0049]: decisions/0049-every-mutation-site-a-change-touches-is-judged-by-the-nightly-a-push-samples-one.md
