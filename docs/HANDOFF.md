# Handoff

This file holds only what is ahead, for the next session to act on. It is
the coordinator's to rewrite at the end of each session, and kept short on
purpose: what was built, and why, is the history (`vp run changelog`), and
open work beyond the next few steps is `docs/work-items.yaml`.

Read [AGENTS.md](../AGENTS.md), [PLAN.md](../PLAN.md),
[docs/ARCHITECTURE.md](ARCHITECTURE.md) and
[docs/work-items.yaml](work-items.yaml) before taking work; a coordinator
reads [docs/ORCHESTRATING.md](ORCHESTRATING.md) too.

Last rewritten <date>, after <what the last session landed, in a line>.

## Where things stand

<Which phases are done and which is in progress, and who owns what is not
yours (`docs/PHASES.md`).>

`main` is green at `<short sha>` (CI run <id>), and the latest nightly is
<green | red on …>: run <id> (<date>, commit `<short sha>`). Read the newest
nightly before beginning the next implementation; a red nightly takes
priority over new work.

## Next

1. **<The next item>** (`<work item id>`): <what it is for, what is decided,
   what is waiting on whom>.
2. **<The one after>** (`<work item id>`): <…>.
3. Continue with what `vp run work` proposes. Ideas, deferred work and
   everything further out live only in `docs/work-items.yaml`.

## User review

<What waits on the user's review or decision, and whether it blocks
implementation.>
