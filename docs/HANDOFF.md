# Handoff

This file holds only what is ahead, for the next session to act on. It is
the coordinator's to rewrite at the end of each session, and kept short on
purpose: what was built, and why, is the history (`vp run changelog`), and
open work beyond the next few steps is `docs/work-items.yaml`.

Read [AGENTS.md](../AGENTS.md), [PLAN.md](../PLAN.md),
[docs/ARCHITECTURE.md](ARCHITECTURE.md) and
[docs/work-items.yaml](work-items.yaml) before taking work; a coordinator
reads [docs/ORCHESTRATING.md](ORCHESTRATING.md) too.

Last rewritten 2026-10-01, after phase 1's first work landed: itos's own
layout, `commits.since`, the features run by godog, AGPL-3.0.

## Where things stand

Phase 0 (the template's scaffold) is done. Phase 1, itos v0 in TypeScript,
is in progress; phase 2 is the Go port. All three are @donvargax's.

`main` is green at `c55d8cf` (CI run 36811208702), and the nightly is green:
run 36811349341 (2026-10-01, commit `c55d8cf`, dispatched by hand). Read the
newest nightly before beginning the next implementation; a red nightly takes
priority over new work.

itos has two consumers waiting on its first release: donvargax/project-template,
which still carries a copy of v0 in `tools/itos/`, and the character editor,
which carries the original. Each switches to the release from its own
repository, coordinated there; this repository only has to release.

## Next

1. **v0.1.0** (`p1-release-v0.1.0`): packed to JavaScript, because Node
   strips types only outside `node_modules`, and released as a tarball on a
   `v0.1.0` tag, so a consumer pins it as a dependency. Consumers wait on it.
2. **CI verifies with the last release too** (`p1-verify-with-last-release`),
   so a commit that breaks the gate cannot approve itself.
3. **A clean config contract before the port**: `itos config check` in a
   gate (`p1-config-check-gate`), every accepted key read
   (`p1-config-keys-honoured`), one defaults table (`p1-one-defaults-table`).
   The Go port copies the contract, so its gaps are cheaper to close here.
4. **The Go port** (`p2-go-port`), one command group at a time, judged by
   the same features and the conformance corpus.
5. Continue with what `vp run work` proposes. Ideas, deferred work and
   everything further out live only in `docs/work-items.yaml`.

## User review

The unit coverage thresholds left with the template's demo app; coverage is
still collected over `tools/itos/*.ts`, with no threshold. Whether to set
one is the user's call; it does not block implementation.
