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

Last updated 2026-10-04, at the end of the session that built slices 46 to 55
(v2.4.0 to v2.15.0) and the itos commands that replace the coordinator's hand
work. Start the next session by reading this file, `itos work`, the last nightly
and the inbox.

## Where things stand

All work is @donvargax's. `main` is green at `a1232bf`, slice 66's close (the slice's CI run 37189815542, every job,
`release` cutting v2.24.0); the coordinator's registry batch after it is pushed
with this file. The coordinator commits only between
agents, from the main checkout; its docs worktree was dropped (ORCHESTRATING.md). itos is Go only; `tools/bin/itos` builds and runs this
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

Released: v2.24.0 (the queue in the registry, `itos work queue`, slice 66), v2.23.0 (`work promote --title`, slice 65), v2.22.0 (`itos go` and `itos guide work`, the generic guides in the binary, slice 64), v2.21.1 (the starter drops docs from the Task footer and scopes docs, bug 12's second half;
its Upgrading says how an existing config matches it), v2.21.0 (the `Item:` footer, `itos commit --item`, taken in place of `Task:` for test, docs
and chore here, slice 63), v2.20.0 (`itos ask`, the user's questions beside the registry, slice 62), v2.19.2 (one lock keeps every write of two writers at once to follow's threads and a stealth
config's registry and ledger, bug 16), v2.19.1 (the wrap never starts a line with a footer token, a note or a comment, bug 15), v2.19.0 (`itos follow`, private threads with people, slice 61), v2.18.0 (`itos commit` wraps a long body line under the built-in lint, slice 58), v2.17.0 (`itos work show`, slice 57; plugin 2.7.0), v2.16.0 (`itos push` returns at once for a registry-only range, slice 56), v2.15.3 (work edit writes a list whole, whatever its old shape, bug 14), v2.15.2 (a ledger file deleted but not committed is refused, and the writers'
rollback is one tested restore, bug 13), v2.15.1 (the first registry item and ledger task written into init's empty lists, bug 12's
@ID-INIT-25; T-081 let its Changes: footer name the init case by a prefix), v2.15.0 (`itos task add`, slice 55), v2.14.0 (`itos work add` and `edit`, slice 54; `itos work` refusing
an unknown subcommand, bug 11), v2.13.0 (`itos work done`, slice 53, which closed itself), v2.12.0 (`itos work take` and `itos work promote`, each committing
the registry alone, slice 52), v2.11.0 (`itos push` waits for CI with `ci.watch`, opt-in, and
`itos ci watch`, slice 51), v2.10.0 (init's rerun notes a pin behind the newest and a missing
people file, slice 50's rest), v2.9.0 (`itos init --git-shim`, untagged features need no Scenarios
footer, slice 50's first half), v2.8.1 (`ci plan` and `ci run` without a `work:` section, bug 10),
v2.8.0 (`itos init --plugin`, slice 49), v2.7.0 (`itos init`, slice 48), v2.6.0 (`itos pin [<version>]`, slice 47), v2.5.0 (the pre-push hook verifies the commits it pushes, slice 46;
silent when they pass), v2.4.0 (`itos config get`), v2.3.2 (Windows paths, bug 9), v2.3.1
(issue #3, bug 8), all cut by CI; v2.3.0 by hand (the guard, `itos work
list`, the itos plugin for Claude Code, now at plugin version 2.4.0). Don't
change any other repository.

## Next

One slice at a time, the user's order. Everything in 1 comes before the user
puts itos in their other repositories (the user's call, 2026-10-03). Every
item's registry steps are commands now: `itos work add`, `edit`, `promote`,
`take`, `done`; nothing in `tasks/work-items.yaml` is edited by hand.

1. **The order is the registry's queue now** (slice 66): `tools/bin/itos work`
   proposes in it, and `itos work queue <id> --top|--before|--after|--drop`
   changes it. It holds this file's former list, slice 67 (`itos status`) first;
   once it lands, this file is deleted in the commit that cuts
   `docs/ORCHESTRATING.md` to this repository's own (`p1-orchestrating-own-notes`,
   second). `p1-push-needs-hooks` waits on q-2.
2. Then `p3-init-orchestration` (PLAN.md §10, "Adoption"), `p3-pr-rebase-merge` and `p3-init-starter-ci`.
3. **`p3-human-waiver`** (`itos waive`, a plain config list; the guard keeps
   agents off it), after those.
4. **`p2-github-hybrid`**, then **`p2-github-pure`** (labels or Projects is
   open for the user).
5. For the user's stealth work in a monorepo: `p3-stealth-scope` (phase 1)
   and `p3-stealth-branch-push`.

itos-cc (on the PATH as an extension) was tried on itos: issues
donvargax/itos-cc#1 to #8 are filed; use it by hand, as advice, not a gate,
until #1, #2 and #4 are fixed; mutate only in a scratch clone (it annotates
source files). `vp staged`'s backup uses git's stash, shared by every
worktree: a commit hook failing with "lint-staged failed due to a git
error" is transient; run the command again.

Small ideas, any time: `p3-guard-windows-paths` (the guard reads a Git Bash
`/c/…` folder as relative), `p3-plugin-guard-tested`,
`p3-plugin-registry-at-top`, `p3-plugin-release-tags`,
`p3-release-notes-bodies`, `p3-version-prerelease`, `p3-changes-at-commit`,
`p3-nightly-govulncheck`, `p3-ci-step-paths`, `p3-shim-push-args`, and
`p3-help-of-launcher-commands`, `p3-notice-names-pin`, `p3-config-21-own-value` (fold into the next feat or fix touching
config.feature). Issue #4 is open as `p1-ledger-pattern-static-after-late`.
Deferred until itos-cc is published: `p3-role-protocol`, `p3-debt-role`,
`p3-debt-claims`. Deferred, the user's to lift: `p1-backport-code-design`
(the user's code-design rules, from the project template). v3.0.0 will carry
`v3-hooks-bin-default` and `v3-stealth-pre-push` (a stealth config's hooks
gain the pre-push entry, changing a released corpus case).

**Questions for the user are `itos ask`** (slice 62): `tools/bin/itos ask`
lists the open ones, each naming the item it holds up; `itos ask answer
<id> '…'` records an answer. Read them at the start of a session, beside the
nightly and the inbox.

Agents commit with `tools/bin/itos commit` and push with `tools/bin/itos
push`, and hand back while their CI watch runs: watch the run yourself, and
`TaskStop` them once landed. A brief for a feat or fix names
`go run ./tools/bin/previous-release` (ORCHESTRATING's brief). Check a spec
can fail before the work: three specs this session could not (ID-GUARD-12,
ID-CONFIG-21, T-072's self-reading check).
