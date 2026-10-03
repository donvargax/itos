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

Last updated 2026-10-03, after T-076 landed.

## Where things stand

All work is @donvargax's. `main` is green at `04412f9` (CI run 37159171253,
every job; v2.9.0 is the newest release). The coordinator commits docs from
its own worktree, `.claude/worktrees/coord-docs` (ORCHESTRATING.md). itos is Go only; `tools/bin/itos` builds and runs this
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

Released: v2.9.0 (`itos init --git-shim`, untagged features need no Scenarios
footer, slice 50's first half), v2.8.1 (`ci plan` and `ci run` without a `work:` section, bug 10),
v2.8.0 (`itos init --plugin`, slice 49), v2.7.0 (`itos init`, slice 48), v2.6.0 (`itos pin [<version>]`, slice 47), v2.5.0 (the pre-push hook verifies the commits it pushes, slice 46;
silent when they pass), v2.4.0 (`itos config get`), v2.3.2 (Windows paths, bug 9), v2.3.1
(issue #3, bug 8), all cut by CI; v2.3.0 by hand (the guard, `itos work
list`, the itos plugin for Claude Code, now at plugin version 2.4.0). Don't
change any other repository.

## Next

One slice at a time, the user's order:

1. The rest of **`slice-50`**: @ID-INIT-20 and 21, the rerun's people-file
   and pin-behind notes. Built and held as `slice50-held.patch` in the
   coordinator's scratchpad; T-076 landed, so v2.9.0's corpus judged
   additively passes against it. Needs help, corpus and ARCHITECTURE
   updates. A feat.
2. **`p3-help-tests-own-path`**, so no brief has to mention it again.
3. The orchestration commands, the user's order (2026-10-03), ahead of the
   rest of init: **`p1-ci-watch`** (`itos push` waits for CI),
   **`p1-work-take-done-promote`** with `p1-slice-done-check` (`itos work
done` verifies before it writes), **`p1-handoff-status`** after
   `p1-work-queue-order` and `p1-follow-ups` (`itos status` replaces this
   file), **`p1-tests-next-id-and-steps`**, **`p1-work-brief`** (now `itos
work show`), and `p1-push-needs-hooks`. Each to specify first.
4. Then `p3-init-agent-rules` and `p3-init-orchestration` (agent files;
   PLAN.md §10, "Adoption"), `p3-pr-rebase-merge` and `p3-init-starter-ci`.
5. **`p3-human-waiver`** (`itos waive`, a plain config list; the guard keeps
   agents off it), after those.
6. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
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
