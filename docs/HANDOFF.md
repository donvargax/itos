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

Last updated 2026-10-03, after slice 45 landed and v2.4.0 was cut by CI.

## Where things stand

Phase 1, itos v0 in TypeScript, is in progress; phase 2 is the Go port. All
are @donvargax's.

`main` is green at `ec14add` (CI run 37146297010, every job: `ci`, the
three `platform` jobs, `release`). Since T-062 itos is Go only,
`tools/bin/itos` the Go binary every gate calls (`hooks.bin`, internal and
unsupported for consumers); Node stays as this repository's dev tooling. Every
push runs, beside the plan `itos.yaml`'s `ci` states: the unit tests and every
feature on Linux, macOS and Windows (T-072), and for a range with a feat or a
fix the config schema held to the last release's (T-070) and the last
release's scenarios and corpus, help cases left out, against the new binary
(T-071; a fix may change an old one it names in `Changes:`). **Releases are
cut by CI** (T-069): a green push to `main` carrying a feat, a fix or a
breaking change releases itself, the version from the commits, GoReleaser,
attested, notes generated; no commit, task or tag by hand. The nightly runs
every feature, the gates' self-tests, every done task's static checks, and
the checks that read CI's own result (the newest release attested, T-069;
the platform jobs green, T-072). The last nightly, 37131118750 (on
`1435336`), is green; the next is the first with the attestation and
platform checks. Read it, and the consumer inbox (`node tools/bin/inbox.ts`),
before new work; a red nightly comes first.

Released: [v2.4.0](https://github.com/donvargax/itos/releases/tag/v2.4.0),
cut by CI from slice 45's feat (`itos config get`); [v2.3.2](https://github.com/donvargax/itos/releases/tag/v2.3.2),
cut by CI from bug 9's fix (Windows: the registry and ledger paths as git's
slash paths); [v2.3.1](https://github.com/donvargax/itos/releases/tag/v2.3.1),
the first cut by CI, from bug 8's (issue #3, closed); v2.3.0 by hand (the
guard, `itos work list`, the itos plugin for Claude Code). Before them v2.2.0,
v2.1.0, v2.0.0. Don't change any other repository.

## Next

The user's order (2026-10-02, adoption added 2026-10-03), one slice at a time:

1. **v2.1.0 and v2.2.0 are out.** Open: `p3-shim-push-args` (`git push`
   takes no arguments through the shim), small, when it bites.
2. **Next**: slice 45 (`itos config get`, v2.4.0) and T-073 (the plugin
   runs what `hooks.bin` names) landed; **T-074** (a plugin change must
   raise the plugin's own version, checked in CI; 2.4.0 first, shipping
   T-073), then the adoption slices below. Ideas left:
   `p3-guard-windows-paths` (the guard reads a Git Bash `/c/…` folder as
   relative), `p3-release-notes-bodies`,
   `p3-version-prerelease`, `p3-changes-at-commit`, `p3-config-21-own-value`
   (fold into the next feat or fix touching config.feature). Deferred until
   itos-cc is published: `p3-role-protocol`, `p3-debt-role`,
   `p3-debt-claims`.
3. **Then**: **`p3-pre-push-verify`**, **`p3-pin-bump`** and
   **`p3-itos-init`**.
4. **After those**: **`p3-human-waiver`** (`itos waive`, a plain config
   list; the guard keeps agents off it), the user's call (2026-10-03).
5. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
   open for the user).

T-067, the dependency check, landed (`tools/bin/deps-check`: Go modules
younger than 7 days refused unless excepted in `deps-check.json`, then
govulncheck, at a commit staging go.mod or go.sum and in CI); it left
`p3-nightly-govulncheck` and `p3-ci-step-paths`. It may move to an
itos-security extension once the user's repository template is ready
(`p3-extension-pins`).

Releases are cut by CI: a feat makes a minor, a fix a patch, a breaking
change a major (v3.0.0 will carry `v3-hooks-bin-default`: `hooks.bin`
defaults to `itos`, the key kept as internal and unsupported). Open for the
user: when the other repositories move, by hand from a release's notes now,
or once `p3-pin-bump` lands (`itos init` as specified only reports what an
existing repository lacks; it migrates nothing).

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
