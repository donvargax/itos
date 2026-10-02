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

Last updated 2026-10-02, after the Go port's first step was specified.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `d68efea` (CI run 36961474674), after v0.6.0 and its
close; this repository's `itos.yaml` requires itos 0.6.0, whose built-in
moves rule it uses. The nightly ends with `{ tasks: done, cost: static }`, every done
task's static checks. The last nightly, 37003001717 (scheduled, on
`e2b4f29`), is green.
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v0.6.0](https://github.com/donvargax/itos/releases/tag/v0.6.0)
(slices 18 to 23, T-037: the config contract the Go port copies), after
[v0.5.0](https://github.com/donvargax/itos/releases/tag/v0.5.0). Each
release's Upgrading section is what a consumer's session updates from. The
user moves the consumers' pins (the project template, the character editor)
from their own repositories: don't change any other repository. Nothing is
unreleased. Write the next release's notes as each slice lands, from its
report, in `docs/releases/v<next>.md`.

## Next

1. **The Go port** (`p2-go-port: The Go port, one command group at a
time`; PLAN.md, phase 2, and its two new decisions: the port's proof, the
   Go version). Step 1 is specified as three tasks in `tasks/phase-2.yaml`:
   `T-039: The Go scaffold, held to the command line's corpus on every push`
   first, then `T-040: A snapshot release of the Go binary, built as a
release will be` and `T-041: The Go binary's --help texts, the corpus's
help cases`. Then specify the config group (step 2: `config check`,
   `task`, `task list`). Small follow-ups the slices left wait, none
   blocking: `p1-group-label-flag-word`, `p1-wip-tag-command-kind`,
   `p1-ledger-id-default`, `p1-own-recognize-for-itos`,
   `p1-release-defaults-per-config`, `p1-registry-old-default-hint`,
   `p1-moves-except-types-checked`, `p1-moves-merge-by-git`.
   `p1-verify-with-last-release` comes with the port (the user's call).
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
