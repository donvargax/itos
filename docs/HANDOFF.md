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

Last updated 2026-10-03, after slice 47 landed (v2.6.0) and slice 48 was
specified.

## Where things stand

All work is @donvargax's. `main` is green at `a1dd4ca` (CI run 37153083956,
every job: `ci`, the three `platform` jobs, `release`, which cut v2.6.0). itos is Go only; `tools/bin/itos` builds and runs this
tree's itos for every gate (`hooks.bin`, internal and unsupported for
consumers); Node stays as dev tooling.

Every push runs the plan `itos.yaml`'s `ci` states, the unit tests and every
feature on Linux, macOS and Windows (T-072), and, for a range with a feat or a
fix, the config schema held to the last release's (T-070) and the last
release's scenarios and corpus, help cases left out, against the new binary
(T-071; a fix may change an old one it names in `Changes:`, a feat never). A
change to the Claude Code plugin must raise its own version (T-074).
**Releases are cut by CI** (T-069): a green push to `main` carrying a feat, a
fix or a breaking change releases itself (version from the commits,
GoReleaser, attested, notes generated); nothing by hand. The nightly runs
every feature, the gates' self-tests, every done task's static checks, and
the checks that read CI's own result (the newest release attested; the
platform jobs green). The last nightly, 37131118750, predates all of that:
read the next one, and the consumer inbox (`node tools/bin/inbox.ts`), before
new work; a red nightly comes first.

Released: v2.6.0 (`itos pin [<version>]`, slice 47), v2.5.0 (the pre-push hook verifies the commits it pushes, slice 46;
silent when they pass), v2.4.0 (`itos config get`), v2.3.2 (Windows paths, bug 9), v2.3.1
(issue #3, bug 8), all cut by CI; v2.3.0 by hand (the guard, `itos work
list`, the itos plugin for Claude Code, now at plugin version 2.4.0). Don't
change any other repository.

## Next

One slice at a time, the user's order:

1. **`slice-48`** (was `p3-itos-init`; @ID-INIT-01 to 09 in
   `features/init.feature`, all `@wip`): init writes a starter config, pins
   the newest release and installs the hooks; run again it reports what is
   missing. Then slice 49 (offer the Claude Code plugin) and slice 50 (offer
   the git shim; report a pin behind the newest and a missing people file),
   to specify as 48 lands (the user's split, 2026-10-03), then
   `p3-init-agent-rules` and `p3-init-orchestration` (agent files; PLAN.md
   §10, "Adoption"), and `p3-pr-rebase-merge`. Each a feat, a minor release.
2. **`p3-human-waiver`** (`itos waive`, a plain config list; the guard keeps
   agents off it), after those.
3. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
   open for the user).

Small ideas, any time: `p3-guard-windows-paths` (the guard reads a Git Bash
`/c/…` folder as relative), `p3-plugin-guard-tested`,
`p3-plugin-registry-at-top`, `p3-plugin-release-tags`,
`p3-release-notes-bodies`, `p3-version-prerelease`, `p3-changes-at-commit`,
`p3-nightly-govulncheck`, `p3-ci-step-paths`, `p3-shim-push-args`, and
`p3-help-tests-own-path` (the help tests read the caller's PATH, so an
installed extension fails them locally; slice 47's agent committed with that
folder off the PATH), `p3-help-of-launcher-commands`, `p3-notice-names-pin`, `p3-config-21-own-value` (fold into the next feat or fix touching
config.feature). Issue #4 is open as `p1-ledger-pattern-static-after-late`.
Deferred until itos-cc is published: `p3-role-protocol`, `p3-debt-role`,
`p3-debt-claims`. Deferred, the user's to lift: `p1-backport-code-design`
(the user's code-design rules, from the project template). v3.0.0 will carry
`v3-hooks-bin-default` and `v3-stealth-pre-push` (a stealth config's hooks
gain the pre-push entry, changing a released corpus case).

Open for the user: when the other repositories move (by hand from a
release's notes now, or with `itos pin`; `itos init` only reports what an
existing repository lacks). Whether this repository's hand-written rules in
`AGENTS.md` become the generated block, once `p3-init-agent-rules` lands (the
coordinator recommends yes, a small `docs` task).

Agents commit with `tools/bin/itos commit` and push with `tools/bin/itos
push`, and hand back while their CI watch runs: watch the run yourself, and
`TaskStop` them once landed. A brief for a feat or fix names
`go run ./tools/bin/previous-release` (ORCHESTRATING's brief). Check a spec
can fail before the work: three specs this session could not (ID-GUARD-12,
ID-CONFIG-21, T-072's self-reading check).

## User review

- The unit coverage thresholds left with the template's demo app; coverage
  is still collected over `tools/itos/*.ts`, with no threshold. Whether to
  set one is the user's call; it does not block implementation.
- Slice 47's agent added a rule beyond the spec: `itos pin` to the version
  already pinned, but whose checksums.txt now hashes differently, refuses
  (exit 1) rather than re-pinning, since the release changed after it was
  pinned. Keep, or overwrite?
- Slice 46's calls: silent when the pushed commits pass (anything printed
  would change released corpus cases, a major release); no opt-out.
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
