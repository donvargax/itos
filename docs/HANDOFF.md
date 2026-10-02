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

Last updated 2026-10-02, after slice 23 landed.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `d8d08ec` (CI run 36959557625), after slices 18 to 23
and T-037: the moving rule is itos's built-in now, and `scenario-moves.ts`
is gone. The nightly ends with `{ tasks: done, cost: static }`, every done
task's static checks. The last green nightly is 36957821730 (on `4740dbf`,
after slice 22); one was dispatched on `d8d08ec` (run 36959769930) for slice 23.
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v0.5.0](https://github.com/donvargax/itos/releases/tag/v0.5.0)
(slices 14 to 17, T-034, T-035), after
[v0.4.0](https://github.com/donvargax/itos/releases/tag/v0.4.0). Each
release's Upgrading section is what a consumer's session updates from;
v0.5.0's adds the nightly's `{ tasks: done, cost: static }` step the
character editor asked for, and has consumers drop ledger checks of itos's
own behaviour. The user moves the consumers' pins (the project template, the
character editor) from their own repositories: don't change any other
repository. Unreleased: slices 18 to 23 and T-037, each already in
`docs/releases/v0.6.0.md` (its notes check passes; the release adds the
intro, known issues, the pin's hash and the check step). Write each slice
into those notes as it lands, from its report.

## Next

1. **The config contract is done** (slices 18 to 23, T-037), but for
   small follow-ups the slices left, none blocking: `p1-group-label-flag-word`,
   `p1-wip-tag-command-kind`, `p1-ledger-id-default`,
   `p1-own-recognize-for-itos`, `p1-release-defaults-per-config`,
   `p1-registry-old-default-hint`, `p1-moves-except-types-checked`,
   `p1-moves-merge-by-git`. `p1-verify-with-last-release` waits for the port
   (the user's call). Next: **release v0.6.0** (its notes are written, slice
   by slice, in `docs/releases/v0.6.0.md`), then **the Go port**
   (`p2-go-port`), unless the user wants some follow-ups first.
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
