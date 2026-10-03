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

Last updated 2026-10-03, after v2.0.0 was released: phase 3 is done.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `26bde05` (CI run 37086973283), T-061's close, whose
download check passed against the published v2.0.0; this
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
repositories: don't change any other repository. Nothing is unreleased.
From v2.1.0 on, `tools/selftest/release-notes.ts` holds every `Upgrading:`
footer of the range to the notes.

## Next

1. **Phase 3 is done.** What waits, none blocking: the release cut by CI
   (`p1-itos-release`), `p1-verify-with-last-release`, the HTTP providers
   (after `p1-conformance-http`), `p2-no-workflow-labels-check`,
   `p3-architecture-go-names`, `p3-header-lint-unread-keys` (a warning,
   never a refusal), `p3-header-case-decomposition`, `p3-text-footer-lines`,
   and the small `p1-*` config-check refinements. The user picks what comes
   next among them and item 2.
2. **After v2, built once in Go:** `p3-claude-code-plugin` (the local
   itos-titles mod, published from this repository), `p3-self-update` (a
   global itos as a version manager: automatic self-update, an exact pin, a
   plain note when a newer version is out), `p3-extensions` (`itos-<cmd>` on the
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
