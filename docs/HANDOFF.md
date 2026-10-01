# Handoff

This file holds only what is ahead, for the next session to act on. It is
the coordinator's to rewrite at the end of each session, and kept short on
purpose: what was built, and why, is the history (`git log`, or `vp run
changelog`), and open work beyond the next few steps is
`tasks/work-items.yaml`.

Read [AGENTS.md](../AGENTS.md), [PLAN.md](../PLAN.md),
[docs/ARCHITECTURE.md](ARCHITECTURE.md) and
[tasks/work-items.yaml](../tasks/work-items.yaml) before taking work; a
coordinator reads [docs/ORCHESTRATING.md](ORCHESTRATING.md) too.

Last rewritten 2026-10-01, after v0.3.0 and v0.4.0 were released.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `9fe7488` (CI run 36901260677), and every task in the
ledger is done. The last nightly is green: run 36894008944 (commit `255b181`,
dispatched by hand). Since then only docs, the registry and T-032's packing
changed, and the release self-test covers the packing. Read the newest nightly
before starting the next implementation. A red nightly takes priority over
new work.

Released: [v0.4.0](https://github.com/donvargax/itos/releases/tag/v0.4.0)
(slices 10 to 13, T-032), after
[v0.3.0](https://github.com/donvargax/itos/releases/tag/v0.3.0) (slices 4, 7,
8, 9). Their Upgrading sections are what a consumer's session updates from;
v0.4.0's has consumers drop the commitlint footer plugin. The user moves the
consumers' pins (the project template, the character editor) from their own
repositories: don't change any other repository. Nothing a consumer runs is
unreleased; v0.5.0's notes are started, `docs/releases/v0.5.0.md`, with an
Upgrading step that has consumers drop ledger checks of itos's own behavior
(T-034). The next release's task builds on that file and keeps the step.

## Next

1. **Specified, in this order, one agent each:**
   - `slice-14: A missing ledger folder is one problem in every command that
reads the ledger` (`features/ledger.feature`);
   - `slice-15: A check merged into a run of named tests that does not happen
runs as itself` (`features/ci.feature`, @ID-CI-04);
   - `slice-16: The nightly runs every done task's checks`
     (`features/nightly.feature`), the character editor's request: a nightly
     step `{ tasks: done, cost: static }`; "done" is the work item's status
     (the user's call). Its agent also does `T-035: This repository's nightly
runs every done task's static checks`, a `ci` commit adding the step to
     `itos.yaml`.
2. **The rest of the config contract, before the port** (ideas, specify
   first): `p1-config-names-from-config`, `p1-one-defaults-table`,
   `p1-itos-in-every-pattern`, `p1-bin-in-defaults`, `p1-group-label-read`,
   `p1-verify-with-last-release`. Then **the Go port** (`p2-go-port`).
3. **The hand work that could be itos's** (the block above phase 2 in the
   registry) comes after the port, the user agreed; the coordinator had
   proposed `p1-itos-push` and `p1-ci-watch` before it, as they save work on
   every landing.

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
