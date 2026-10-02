# Handoff

This file holds only what is ahead, for the next session to act on. It is
the coordinator's to update after every landing (docs/ORCHESTRATING.md, the
loop's step 3), so it is right whenever a session stops, and kept short on
purpose: what was built, and why, is the history (`git log`, or `vp run
changelog`), and open work beyond the next few steps is
`tasks/work-items.yaml`.

Read [AGENTS.md](../AGENTS.md), [PLAN.md](../PLAN.md),
[docs/ARCHITECTURE.md](ARCHITECTURE.md) and
[tasks/work-items.yaml](../tasks/work-items.yaml) before taking work; a
coordinator reads [docs/ORCHESTRATING.md](ORCHESTRATING.md) too.

Last updated 2026-10-02, after v1.0.0 was released and slice 24 specified.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `9b9eb37` (CI run 37064539257), T-056's close, whose
download check passed against the published v1.0.0; this
repository's `itos.yaml` requires itos 0.6.0, whose built-in moves rule it
uses. CI now builds the Go binary and runs the ported set against it
(`tools/selftest/go-port.ts`): since T-053 the whole corpus and every
feature, about 15 seconds, so a behaviour change lands in both implementations
or CI is red. The
nightly builds and proves the Go release archives
(`tools/selftest/go-release.ts`, new with T-040, not yet run by a nightly),
then ends with `{ tasks: done, cost: static }`, every done task's static
checks. The last nightly, 37061038901, dispatched by hand on `10a14ed` before v1.0.0,
is green: the first to run the Go release, schema and whole-suite steps.
Read the consumer inbox beside it (`node tools/bin/inbox.ts`,
docs/ORCHESTRATING.md's loop).
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v1.0.0](https://github.com/donvargax/itos/releases/tag/v1.0.0)
(phase 2: the Go binary, T-039 to T-057, beside the unchanged TypeScript
tarball; release run 37064071661), after
[v0.6.0](https://github.com/donvargax/itos/releases/tag/v0.6.0). Its assets
are the five Go archives, `itos.schema.json`, `itos-1.0.0.tgz` and one
`checksums.txt`; every one was downloaded and verified after the release.
Each release's Upgrading section is what a consumer's session updates from.
The user moves the consumers' pins (the project template, the character
editor) from their own repositories: don't change any other repository.
Nothing is unreleased. Write the next release's notes as each slice lands,
from its report, in `docs/releases/v<next>.md`.

## Next

1. **Slice 24: `slice-24: The config's patterns are RE2 regular
expressions, in both implementations`** (`features/patterns.feature`,
   `@ID-PATTERN-01` and `02`, `@wip`; PLAN.md, "Pattern dialect", the user's
   call). The TypeScript refuses lookarounds and backreferences and both name
   RE2, in one push; `tests.<kind>.id` is checked as a pattern too. It
   refuses configs v1.0.0's TypeScript accepted, so release it as v1.1.0 with
   an Upgrading step (the user's note: v1.0.0 went out before this was
   settled; docs/ORCHESTRATING.md's new lesson).
2. **Phase 3, the switch** (PLAN.md, phase 3): the Go binary shadows the
   TypeScript's `ci plan --json` and `verify` in CI, any difference a
   warning, until none shows over many pushes and a nightly; then consumers
   switch, and the TypeScript goes after two weeks of green nightlies.
   Specify it first. Also open: `p1-verify-with-last-release` (deferral
   lifted; since T-053 the two implementations judge each other on every
   push, so what is left is whether CI also runs the released binary), and
   `p2-go-port`'s remainder, step 6's HTTP providers, which wait for
   `p1-conformance-http`.

   Follow-ups wait, none blocking: `p1-group-label-flag-word`,
   `p1-wip-tag-command-kind`, `p1-ledger-id-default`,
   `p1-own-recognize-for-itos`, `p1-release-defaults-per-config`,
   `p1-registry-old-default-hint`, `p1-moves-except-types-checked`,
   `p1-moves-merge-by-git`, `p2-go-unit-tests-in-hooks` (Go unit tests run
   only in CI), `p2-no-workflow-labels-check`.

3. **After the port: `p2-stealth-mode: A local mode, itos as one person's
discipline in a repository that does not use it`** (the user's idea and
   calls, 2026-10-02: config, ledger and registry under `.git/itos/`, found
   with no environment variable; the task link in git notes, never a footer;
   hooks that chain to the project's own). Specify it as scenarios once the
   Go binary is the one implementation. Beside it, the user's GitHub modes:
   `p2-github-hybrid` (issues as a second source of work items) and
   `p2-github-pure` (the registry as GitHub itself; labels or Projects is
   open for the user).
4. **The hand work that could be itos's** (the block above phase 2 in the
   registry) comes after the port, the user agreed; the coordinator had
   proposed `p1-itos-push` and `p1-ci-watch` before it, as they save work on
   every landing.

5. Continue with what `tools/bin/itos work` proposes.

Deferred, the user's to lift: `p1-backport-code-design`. The user is writing
code-design rules (vertical slices, no mocks, unit tests for the core and
integration tests for the rest, property-based testing) in the project
template first; they come back here once that is done. When they do,
`p1-several-test-kinds` moves up, for an integration-test kind.

Agents in this repository: pull with
`rtk proxy git pull --rebase --no-autostash origin main` (the rtk hook's
rewrite of a plain `git pull` can fail), and they are made to hand back
while their CI watch still runs: watch the run yourself before relaying.
Check before committing a spec that its range and setup can reach the line
it checks: two specs this session needed a fix after an agent read them.

## User review

- The unit coverage thresholds left with the template's demo app; coverage
  is still collected over `tools/itos/*.ts`, with no threshold. Whether to
  set one is the user's call; it does not block implementation.
- Coordinator calls to confirm or overturn:
  - slice 4 drops the keys of features not built yet rather than reading
    them;
  - in slice 7, a task with no work item counts as in progress;
  - slice 8 tries each cost pattern on the command as written and as
    hooks.bin → itos;
  - in slice 9, a check's reused run is read even when an earlier check of
    its own task changed the tree;
  - in slice 10, a failing delegate keeps its own exit code in the hook;
  - slice 13 makes a missing ledger folder exit 2, a config error (PLAN §7).
