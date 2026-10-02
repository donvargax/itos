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

Last updated 2026-10-02, after T-059 (the Go unit tests in the hooks) landed.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `e2ad4fb` (CI run 37072752161), after T-059; this
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

Released: [v1.1.0](https://github.com/donvargax/itos/releases/tag/v1.1.0)
(slice 24 and its fix: the config's patterns as RE2 in both
implementations; release run 37071299183), after
[v1.0.0](https://github.com/donvargax/itos/releases/tag/v1.0.0), the Go
binary. Every asset was downloaded and verified after each release. Releases
are automated (PLAN.md, "Releases"): no tag waits for the user. The user moves
the consumers' pins from their own repositories: don't change any other
repository. Nothing is unreleased. Write the next release's notes as each
slice lands, from its report, in `docs/releases/v<next>.md`.

## Next

1. **Phase 3, Go only as soon as possible** (PLAN.md, phase 3, the user's
   calls: no shadow period, no two-week wait), one agent at a time:
   - `T-060: This repository's hooks and CI run the Go binary` (dogfood);
   - `T-062: The TypeScript implementation leaves the repository` (no
     release; reordered so slice 25 is built once, in Go);
   - `slice-25: The built-in header lint replaces commitlint`, to specify as
     scenarios (draft: `use: builtin`, config-conventional's errors with
     `commits.types` as type-enum, commitlint's rule ids and words);
   - `T-063: The built-in header lint agrees with commitlint, and this
repository switches to it`;
   - `T-061: v2.0.0, Go only`, to specify once T-063 lands.

   Proposed to the user, not yet decided: `p1-upgrading-footer` before v2,
   so v2's notes are gathered from commits. Follow-ups: the release cut by
   CI (`p1-itos-release`), `p1-verify-with-last-release`, the HTTP
   providers (after `p1-conformance-http`), `p2-no-workflow-labels-check`,
   and the small `p1-*` config-check refinements.

2. **After v2, built once in Go:** `p3-extensions` (`itos-<cmd>` on the
   PATH, a trivial `itos-hello` first); `p2-stealth-mode` (config, ledger and
   registry under `.git/itos/`, found with no environment variable; the task
   link in git notes, never a footer; hooks that chain to the project's own);
   `p2-github-hybrid` (issues as a second source of work items) and
   `p2-github-pure` (the registry as GitHub itself; labels or Projects is
   open for the user). Each is specified as scenarios when its turn comes.
3. **The hand work that could be itos's** (the block above phase 2 in the
   registry), as extensions where it fits: `p1-itos-push`, `p1-ci-watch`.
4. Continue with what `tools/bin/itos work` proposes.

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
