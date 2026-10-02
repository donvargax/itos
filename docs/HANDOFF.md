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

Last updated 2026-10-02, after T-042 (the Go config loader) landed.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `b57a365` (CI run 37039088349), after T-042; this
repository's `itos.yaml` requires itos 0.6.0, whose built-in moves rule it
uses. CI now builds the Go binary and runs the ported set against it
(`tools/selftest/go-port.ts`, its one list `PORTED`: `cli.yaml`,
`help.yaml`, `config.yaml` and config check's scenarios so far, so a help text changes in help.ts, help.go and help.yaml
in one push). The
nightly builds and proves the Go release archives
(`tools/selftest/go-release.ts`, new with T-040, not yet run by a nightly),
then ends with `{ tasks: done, cost: static }`, every done task's static
checks. The last nightly, 37003001717 (scheduled, on `e2b4f29`), is green.
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v0.6.0](https://github.com/donvargax/itos/releases/tag/v0.6.0)
(slices 18 to 23, T-037: the config contract the Go port copies), after
[v0.5.0](https://github.com/donvargax/itos/releases/tag/v0.5.0). Each
release's Upgrading section is what a consumer's session updates from. The
user moves the consumers' pins (the project template, the character editor)
from their own repositories: don't change any other repository. Unreleased: the Go port so far (T-039 to T-042), which changes nothing a
consumer of the TypeScript sees. Write the next release's notes as each slice lands, from its
report, in `docs/releases/v<next>.md`.

## Next

1. **The Go port** (`p2-go-port: The Go port, one command group at a
time`; PLAN.md, phase 2, and its decisions "The port's proof" and "The Go
   version"). The scaffold is in (`T-039`: commands not ported yet exit 3,
   `itos: <command> is not in this build yet`). The help texts (`T-041`) and the release archives (`T-040`,
   `node tools/bin/build-go.ts --release <dir>`) are in, and so is `T-042: The Go config loader and config check`
   (the whole loader and its one table; `config check` in Go agrees with the
   TypeScript's on this repository, `--print-defaults` byte for byte). Next,
   one agent at a time, the rest of the config group: `T-043: The Go task
runner, task and task list`, `T-044: The Go globs and path rules, commit
check-paths`. Each group adds its corpus files and `-scenarios=`
   selections to `PORTED`, and may widen `ci.covers` (one rule per ported
   corpus file) only for files already in it, or CI would skip a task's
   real check. Then specify step 3 on. Small follow-ups wait, none blocking:
   `p1-group-label-flag-word`, `p1-wip-tag-command-kind`,
   `p1-ledger-id-default`, `p1-own-recognize-for-itos`,
   `p1-release-defaults-per-config`, `p1-registry-old-default-hint`,
   `p1-moves-except-types-checked`, `p1-moves-merge-by-git`,
   `p2-go-unit-tests-in-hooks`, `p2-go-release-proves-its-archives`, `p2-go-source-trees` (the Go
   source reads the working tree only; step 3's footers at a commit need it),
   `p2-config-regexp-dialect` (Go validates the config's patterns as RE2,
   the TypeScript as JavaScript's; none of today's patterns differ). `p1-verify-with-last-release` comes with
   the port (the user's call).
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
