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

Last updated 2026-10-03, after slice 38 (a stealth session owns every item) landed; v2.1.0 next.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `c17acf8` (CI run 37101277075), slice 38's close; this
repository requires an `Upgrading:` footer of every feat and fix (since
`2e522f2`), and its headers are judged by the built-in lint; since T-062 itos is Go
only, `tools/bin/itos` the Go binary every gate calls; Node stays as this
repository's dev tooling (vp, the corpus runner, the self-tests), never a
consumer's runtime; this
repository's `itos.yaml` requires itos 0.6.0, whose built-in moves rule it
uses. CI now builds the Go binary and runs the ported set against it
(`tools/selftest/go-port.ts`): since T-053 the whole corpus and every
feature, about 15 seconds, so a behaviour change lands in both implementations
or CI is red. The
nightly builds and proves the Go release archives
(`tools/selftest/go-release.ts`, new with T-040, not yet run by a nightly),
then ends with `{ tasks: done, cost: static }`, every done task's static
checks. The last nightly, 37083475482 (dispatched by hand on `719e0e8`, after T-063),
is green.
Read the consumer inbox beside it (`node tools/bin/inbox.ts`,
docs/ORCHESTRATING.md's loop).
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v2.0.0](https://github.com/donvargax/itos/releases/tag/v2.0.0),
Go only (phase 3: the TypeScript gone, the built-in header lint, the
`Upgrading:` footer; release run 37086752429), after v1.1.0 and v1.0.0. Its
assets (five archives, `itos.schema.json`, `checksums.txt`) were downloaded
and verified, and `go install github.com/donvargax/itos/v2/cmd/itos@v2.0.0`
works (the module path is `/v2` since T-061). Releases are automated
(PLAN.md, "Releases"). The user moves the consumers' pins from their own
repositories: don't change any other repository. Unreleased: slices 27 to 31
(the launcher, `pin`, the newest release and the notice, extensions, the
stealth config, `itos commit`, stealth footers in notes, hooks in the git
config, the unpushed range, a stealth config's defaults, the amend signal,
`itos commit`'s content flags, a stealth session owning every item) and the
`@bug-6` fix and
the `itos version` fix (issue #2).
From v2.1.0 on, `tools/selftest/release-notes.ts` holds every `Upgrading:`
footer of the range to the notes.

## Next

The user's order (2026-10-02, adoption added 2026-10-03), one slice at a time:

1. **Self-update** is done: slices 27 and 28. Their ideas wait in the
   registry (`p3-pin-bump`, `p3-launcher-subfolder`,
   `p3-launcher-cache-prune`, `p3-global-install-ci`).
2. **Extensions** is done: slice 29.
3. **Stealth mode** is done: slices 30 to 35, with `itos commit` (slice 31)
   and the `@bug-5` fix. The coordinator proposed cutting v2.1.0 now; the
   user has not answered. `hooks.bin` stays through v2 (`v3-drop-hooks-bin`).
4. **v2.1.0** (T-064): everything for it has landed; a nightly was dispatched
   by hand on `c17acf8`, since the last ran before slice 27. Then the notes,
   the version, the tag.
5. **`p1-itos-push`**, itos commit's other half (built in or an extension,
   the first call).
6. **`p3-claude-code-plugin`**: the marketplace, the titles, the skill, the
   git commit/push guard (PLAN.md §10, "Adoption").
7. **`p3-pre-push-verify`** and **`p3-human-waiver`** (`itos waive`, a
   plain config list; the guard keeps agents off it).
8. **`p3-itos-init`**, after `p3-pin-bump`: ties the above together.
9. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
   open for the user).

Releases, the user's plan (2026-10-03): v2.1.0 when item 4 is done (global
install, extensions, the stealth mode, `itos commit`); v2.2.0, git through
itos (`p1-itos-push`, `p3-launcher-subfolder`, `p3-git-shim`); v2.3.0,
adoption (the plugin, `p3-pre-push-verify`, `p3-human-waiver`,
`p3-itos-init`); v3.0.0 the breaking cleanup (`v3-drop-hooks-bin`, the
notes gathering once git-cliff writes them). The user moves the other
repositories at v2.3.0, so v2.1.0 and v2.2.0 ask nothing of them.

Each is specified as scenarios when its turn comes. Left for later, the
user's: the release cut by CI (`p1-itos-release`), which the user solves
elsewhere and ports back, and with it `p3-release-signatures`. Waiting, none
blocking: issues #3 and #4 (`p1-smoke-run-arguments`,
`p1-ledger-pattern-static-after-late`), `p1-verify-with-last-release`, the
HTTP providers (after `p1-conformance-http`), `p2-no-workflow-labels-check`,
`p3-architecture-go-names`, `p3-header-lint-unread-keys`,
`p3-header-case-decomposition`, `p3-text-footer-lines`, the small `p1-*`
config-check refinements, and the hand work as extensions (`p1-itos-push`,
`p1-ci-watch`).

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
