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

Last updated 2026-10-03, after v2.3.0 was released.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `c99276b` (CI run 37130620622); this
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
checks. The last nightly, 37125737029 (scheduled, on `d4b0865`), is green.
Read the consumer inbox beside it (`node tools/bin/inbox.ts`,
docs/ORCHESTRATING.md's loop).
Read the newest nightly before starting the next implementation. A red
nightly takes priority over new work.

Released: [v2.3.0](https://github.com/donvargax/itos/releases/tag/v2.3.0)
(T-068, release run 37131270855): `itos hook pre-tool-use`, the guard;
`itos work list`; the guard under an old pin; the itos plugin for Claude Code,
this repository a plugin marketplace. Additive. The nightly on its commit,
37131118750, is green. Before it, v2.2.0 (`itos push`, the git shim), v2.1.0,
v2.0.0, v1.1.0, v1.0.0. Don't change any other repository. Unreleased: T-067,
the dependency check (this repository's own).

## Next

The user's order (2026-10-02, adoption added 2026-10-03), one slice at a time:

1. **v2.1.0 and v2.2.0 are out.** Open: `p3-shim-push-args` (`git push`
   takes no arguments through the shim), small, when it bites.
2. **v2.3.0 is out** (release run 37131270855; its assets downloaded and
   checksummed, the linux binary says `itos 2.3.0`).
   Next **T-070** (the config schema held to the last release's) and
   **T-071** (the last release's scenarios run against the new binary; a
   fix may change one it names in `Changes:`, a feat never), then
   **T-069**, the user's call (2026-10-03): a push to `main` carrying a
   `feat` or a `fix` with CI green cuts the release itself (version from the
   commits, GoReleaser, attestations, generated notes, no commit by the
   releaser), before v2.4.0's slices, so v2.4.0 is the first cut by CI.
3. **v2.4.0**: **`p3-pre-push-verify`**, **`p3-pin-bump`** and
   **`p3-itos-init`**.
4. **After v2.4.0**: **`p3-human-waiver`** (`itos waive`, a plain config
   list; the guard keeps agents off it), the user's call (2026-10-03).
5. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
   open for the user).

T-067, the dependency check, landed (`tools/bin/deps-check`: Go modules
younger than 7 days refused unless excepted in `deps-check.json`, then
govulncheck, at a commit staging go.mod or go.sum and in CI); it left
`p3-nightly-govulncheck` and `p3-ci-step-paths`. It may move to an
itos-security extension once the user's repository template is ready
(`p3-extension-pins`).

Releases, the user's plan (2026-10-03): v2.1.0 and v2.2.0 released; v2.3.0
the plugin and the guard; v2.4.0 pre-push verify, the pin bump and
`itos init`; the waiver after it; v3.0.0 the breaking cleanup
(`v3-drop-hooks-bin`, the notes gathering once git-cliff writes them).
Open for the user: whether the other repositories move at v2.3.0, by hand
from the Upgrading sections, or at v2.4.0 with `p3-pin-bump` (`itos init`
as specified only reports what an existing repository lacks; it migrates
nothing).

Each is specified as scenarios when its turn comes. Left for later:
`p3-release-signatures`, the launcher checking T-069's attestations itself. Waiting, none
blocking: issues #3 and #4 (`p1-smoke-run-arguments`,
`p1-ledger-pattern-static-after-late`), `p1-verify-with-last-release`, the
HTTP providers (after `p1-conformance-http`), `p2-no-workflow-labels-check`,
`p3-architecture-go-names`, `p3-header-lint-unread-keys`,
`p3-header-case-decomposition`, `p3-text-footer-lines`, the small `p1-*`
config-check refinements, and the hand work as extensions (`p1-ci-watch`).

Deferred, the user's to lift: `p1-backport-code-design`. The user is writing
code-design rules (vertical slices, no mocks, unit tests for the core and
integration tests for the rest, property-based testing) in the project
template first; they come back here once that is done. When they do,
`p1-several-test-kinds` moves up, for an integration-test kind.

Agents in this repository commit with `tools/bin/itos commit` and push with
`tools/bin/itos push` (AGENTS.md), and they are made to hand back while their
CI watch still runs: watch the run yourself before relaying.
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
