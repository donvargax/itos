# Handoff

This file holds only what is ahead, for the next session to act on. It is
the coordinator's to rewrite at the end of each session, and kept short on
purpose: what was built, and why, is the history (`git log`; `vp run
changelog` is broken until `p1-changelog-unconventional`), and open work
beyond the next few steps is `tasks/work-items.yaml`.

Read [AGENTS.md](../AGENTS.md), [PLAN.md](../PLAN.md),
[docs/ARCHITECTURE.md](ARCHITECTURE.md) and
[tasks/work-items.yaml](../tasks/work-items.yaml) before taking work; a
coordinator reads [docs/ORCHESTRATING.md](ORCHESTRATING.md) too.

Last rewritten 2026-10-01, after v0.1.0 and v0.2.0 were released and slice 7
landed.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `ac2e037` (CI run 36823765235). The last nightly is
green: run 36824086819 (commit `ac2e037`, dispatched by hand). Read the newest nightly before beginning the next
implementation; a red nightly takes priority over new work.

Released: [v0.2.0](https://github.com/donvargax/itos/releases/tag/v0.2.0)
(the registry at `tasks/work-items.yaml`, itos's data checked at commit, every
command through `shell`). Its notes' Upgrading section is what a consumer's
session updates from. The user moves the consumers' pins (the project
template, the character editor) from their own repositories: don't change
any other repository.

Since v0.2.0, on `main` and unreleased: T-026 (the docs call
`tools/bin/itos task`; the `task` script is gone) and slice 7 (the
commit-msg hook runs the static checks of the tasks a commit names; a
failure rejects the commit only when the task's item is `done`).

## Next

1. **Slice 4** (`features/config.feature`, @ID-CONFIG-07 to 14, `@wip`):
   `work.statuses`, `work.groups_key`, `smoke.every_file` and `hooks.manager`
   read; `commits.header_lint.use` and `alongside` and `ledger.check.pushed`
   leave the schema (this repository's `itos.yaml` sets `use` and `pushed`:
   they go with it). None of its steps exist yet. It drops keys consumers
   may set, so it ships in v0.3.0 with notes, never alone in a patch.
2. **v0.3.0** (T-027): slices 7 and 4. Take `p1-release-notes-schema-keys`
   first or with it, or the notes check misses slice 4's dropped keys; and
   `p1-release-checks-its-notes` is cheap beside it. The tag waits for the
   user's review of the Upgrading section (ORCHESTRATING.md, Lessons).
3. **What the consumers found**, already ideas: `p1-footers-beside-header-lint`
   (footer rules run only through the commitlint plugin when a delegate is
   configured) and `p1-e2e-names` (the template's E2E names in the code and
   its output). Then `p1-release-manifest` (every consumer's install prompts
   for this repository's `prepare` script) and `p1-changelog-unconventional`
   (AGENTS.md sends agents to a command that fails).
4. **The rest of the config contract before the port**:
   `p1-config-names-from-config`, `p1-one-defaults-table`,
   `p1-verify-with-last-release`, then **the Go port** (`p2-go-port`).
5. Continue with what `tools/bin/itos work` proposes.

Agents in this repository: pull with
`rtk proxy git pull --rebase --no-autostash origin main` (the rtk hook's
rewrite of a plain `git pull` can fail), and they are made to hand back
while their CI watch still runs: watch the run yourself before relaying.

## User review

- The unit coverage thresholds left with the template's demo app; coverage
  is still collected over `tools/itos/*.ts`, with no threshold. Whether to
  set one is the user's call; it does not block implementation.
- Coordinator calls to confirm or overturn: slice 4 drops the keys of
  features not built yet rather than reading them; in slice 7 a task with no
  work item counts as in progress (reported, not rejected).
