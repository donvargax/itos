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

`main` is green at `3210cc3` (CI run 36946719116) after slices 14, 15 and
16 and T-035; every task in the ledger is done. The nightly is green on it
(run 36946722134, dispatched by hand) and now ends with
`{ tasks: done, cost: static }`, every done task's static checks. Its first
run was red: the nightly's checkout was shallow and lacked actionlint, which
those checks need; both are fixed in `nightly.yml`. Read the newest nightly
before starting the next implementation. A red nightly takes priority over
new work.

Released: [v0.4.0](https://github.com/donvargax/itos/releases/tag/v0.4.0)
(slices 10 to 13, T-032), after
[v0.3.0](https://github.com/donvargax/itos/releases/tag/v0.3.0) (slices 4, 7,
8, 9). Their Upgrading sections are what a consumer's session updates from.
The user moves the consumers' pins (the project template, the character
editor) from their own repositories: don't change any other repository.
Unreleased: slices 14 to 16. `docs/releases/v0.5.0.md` already says what
each changes and how to upgrade, written from each slice's report as it
landed; the release task finishes that file (intro, known issues, the pin
and check steps) rather than starting one.

## Next

1. **The rest of the config contract, before the port** (ideas, specify
   first): `p1-config-names-from-config`, `p1-one-defaults-table`,
   `p1-itos-in-every-pattern`, `p1-bin-in-defaults`, `p1-group-label-read`,
   `p1-verify-with-last-release`. Then **the Go port** (`p2-go-port`).
2. **The hand work that could be itos's** (the block above phase 2 in the
   registry) comes after the port, the user agreed; the coordinator had
   proposed `p1-itos-push` and `p1-ci-watch` before it, as they save work on
   every landing.

3. Continue with what `tools/bin/itos work` proposes.

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
