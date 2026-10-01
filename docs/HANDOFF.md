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

Last rewritten 2026-10-01, after v0.3.0 was released and slices 10 and 11
landed.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `0b29300` (CI run 36886103010). The last nightly is green:
run 36882519408 (commit `d547482`, dispatched by hand). Read the newest
nightly before starting the next implementation. A red nightly takes priority
over new work.

Released: [v0.3.0](https://github.com/donvargax/itos/releases/tag/v0.3.0),
which holds slices 4, 7, 8 and 9. Its Upgrading section is what a consumer's
session updates from. The user moves the consumers' pins (the project
template, the character editor) from their own repositories: don't change
any other repository.

Since v0.3.0, on `main` and unreleased, these change what a consumer sees:

- **Slice 10.** With a header-lint delegate configured, itos runs the footer
  rules itself, beside the delegate, in the hook, `commit check-message` and
  `verify`. A consumer's commitlint config that still carries the footer
  plugin reports each footer problem twice until it drops the plugin. A
  commit that a footer-less delegate used to pass can now be rejected,
  `verify` included.
- **Slice 11.** `ci run` logs a merged check as `(in the <kind> run above)`,
  no longer `(in the E2E run above)`.

These are internal only: the changelog renders again (T-010), the release
tasks' notes checks judge their own release (T-029), the E2E names are out
of the code (T-031), and this repository's own commitlint plugin and cost
pattern changed (T-028, T-030).

## Next

1. **Cheap fixes a consumer can hit.** These are ideas, so specify them
   first:
   - `p1-footer-source-missing`: since slice 10, any config with a delegate
     and no `tasks/` folder fails with a raw ENOENT;
   - `p1-empty-selection-pattern`: an empty smoke set may run every scenario.
     This was read from the code and not yet confirmed: confirm it before
     writing the spec.
2. **v0.4.0**, with slices 10 and 11 and whatever in step 1 lands. Its
   Upgrading section tells consumers to drop the commitlint footer plugin.
   Brief the release agent from T-027's brief (stop before the tag).
   `p1-release-manifest` (every consumer's install prompts for this
   repository's `prepare` script) is cheap to take with it.
3. **The rest of the config contract, before the port:**
   `p1-config-names-from-config`, `p1-one-defaults-table`,
   `p1-itos-in-every-pattern`, `p1-bin-in-defaults`, `p1-group-label-read`,
   `p1-verify-with-last-release`. Then **the Go port** (`p2-go-port`).
4. Continue with what `tools/bin/itos work` proposes.

When briefing agents in this repository:

- They must pull with
  `rtk proxy git pull --rebase --no-autostash origin main`. The rtk hook's
  rewrite of a plain `git pull` can fail.
- They hand back while their CI watch still runs, so watch the run yourself
  before relaying.
- Before committing a spec, check that its range and setup can actually
  reach the line it checks (ORCHESTRATING.md, "Check a spec's words"). Two
  specs this session needed a fix after an agent read them.

## User review

- The unit coverage thresholds left with the template's demo app; coverage
  is still collected over `tools/itos/*.ts`, with no threshold. Whether to
  set one is the user's call; it does not block implementation.
- Coordinator calls to confirm or overturn:
  - in slice 4, the keys of features not built yet are dropped rather than
    read;
  - in slice 7, a task with no work item counts as in progress (reported,
    not rejected);
  - in slice 8, each cost pattern is tried on the command as written as well
    as on the hooks.bin → itos form, so the change can only move checks from
    late to static;
  - in slice 10, a failing delegate's own exit code stays the hook's;
  - in slice 9, a check whose result depends on an earlier check in its own
    task reads a reused run (accepted limit).
