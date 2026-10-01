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

`main` is green at `0f1d884` (CI run 36896159158). The last nightly is green:
run 36894008944 (commit `255b181`, dispatched by hand; nothing after it changes
the implementation but T-032's packing). Read the newest nightly before
beginning the next implementation. A red nightly takes priority over new work.

Released: [v0.4.0](https://github.com/donvargax/itos/releases/tag/v0.4.0)
(slices 10 to 13, T-032), after
[v0.3.0](https://github.com/donvargax/itos/releases/tag/v0.3.0) (slices 4, 7,
8, 9). Their Upgrading sections are what a consumer's session updates from;
v0.4.0's has consumers drop the commitlint footer plugin. The user moves the
consumers' pins (the project template, the character editor) from their own
repositories: don't change any other repository. Nothing is unreleased.

## Next

1. **What the last slices found, cheap and consumer-facing** (ideas, specify
   first): `p1-ledger-folder-missing` (`config check`, `task`, `ci plan` and
   `ci run` still crash on a missing ledger folder), and
   `p1-merged-check-without-a-run`.
2. **The rest of the config contract, before the port:**
   `p1-config-names-from-config`, `p1-one-defaults-table`,
   `p1-itos-in-every-pattern`, `p1-bin-in-defaults`, `p1-group-label-read`,
   `p1-verify-with-last-release`. Then **the Go port** (`p2-go-port`).
3. **The hand work that could be itos's**, recorded at the user's request
   (the block above phase 2 in the registry). Each one is something this
   session did by hand, the same way every time:
   - `p1-work-take-done-promote`, `p1-itos-push`, `p1-ci-watch`;
   - `p1-upgrading-footer`, `p1-itos-release`;
   - `p1-done-tasks-nightly`, `p1-wip-red-first`, `p1-commit-message-file`;
   - and a second set, refined with the user: `p1-work-brief`,
     `p1-work-queue-order`, `p1-slice-done-check`, `p1-checkout-lease`,
     `p1-itos-upgrade`, `p1-tests-next-id-and-steps`, `p1-handoff-status`,
     `p1-work-spend`, `p1-smoke-suggest`.

   The user liked all of them; ask which come before the port.

4. Continue with what `tools/bin/itos work` proposes.

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
