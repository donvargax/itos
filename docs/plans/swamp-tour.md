# The Swamp tour

Part 1 of `p1-swamp-factory-spike` (`docs/plans/swamp-spike.md` is part 2):
Swamp as its users meet it, on the host, before any itos is plugged in. Three
runs, 2026-11-03, Swamp `20261103.194535.0`, software-factory `2026.10.23.1`.

## The runs

**1. A first task with Codex** (`swamp start`, its own workspace). "Summarise
yesterday's failed GitHub Actions runs." Codex pulled a community GitHub
extension, added a read-only method of its own with typed schemas and 14
tests, scanned 134 repositories and found 6 failed runs, then made the window
relative and scheduled it daily through `swamp serve` as a systemd user
service. After that the report runs with no agent.

**2. software-factory, driven by an agent** (Claude Code, `sdlc-classic`). A
`wc` CLI and one work item: plan, adversarial review, implement, `go test` as
a workflow, a local tag, UAT. Reached `done` in 23 minutes with 4 reviewer
subagents and 2 approvals. The rework and the failing test were staged.

**3. software-factory, driven by a workflow** (Claude Code built it, then
launched it). A `wc-tick` workflow reads the factory's status, runs the
stage's work (`@swamp/agent-runner` with Claude, `auth=cli`; a `wc-test`
workflow for `go test`), records the result and advances: one transition per
run. 16 ticks reached `done` in 22 minutes; one rework happened unplanned.
About $0.84 of agent calls by the CLI's estimate.

## What Swamp is

A model ("behavior") with methods is the unit; every method's output is typed,
versioned data with provenance and a lifetime; workflows only sequence method
calls (DAG, `forEach`, conditions on dependencies, manual approvals, signal
waits, checkpoint and resume; no loops); `swamp serve` adds schedules,
webhooks, workers, a dashboard and approvals decided under a user's grant.
Extensions are TypeScript run by Deno, shared through a registry. The design
is clean; it is the part worth having.

software-factory is a generic state machine over that: stages, transitions and
gates as data, a driver loop (status, record dispatch, work, record, advance),
the dispatch guard against runaway loops. It is small next to itos.

## What the runs showed

- **Only Swamp's own run records are verified.** In the factory, the one gate
  Swamp checks is `workflow-succeeded`. Findings, change summaries, release
  evidence and approvals are whatever the driver recorded.
- **Approvals don't bind.** An approval's actor is free text. In run 2 the
  driving agent ran `approve` with the person's name after a chat reply. In
  run 3 agents were kept from running `swamp`, but anything running as the
  user could approve. Binding needs workflow `manual_approval` steps on runs
  `serve` drives, which software-factory's `approve` is not (run 4, below).
- **The item's history is a second authority.** `state`, `journal`, artifacts
  and approvals live in `.swamp/`, gitignored; git holds only the code and the
  tag. Nothing in git says the item was reviewed.
- **The loop needs a driver outside the workflow.** No loop construct, so a
  lifecycle with cycles is a tick workflow re-run by something: a schedule, a
  webhook, or an agent (run 3's agent launched all 16 ticks). This is the shape
  of `itos next` and the bug-157 driver, one step per tick.
- **Judgement still routes.** In run 3 a reviewer's severity rating decided an
  unplanned rework; schemas and cycle limits contain agent variance, they do
  not remove it.
- **Third-party code loads before anyone reads it.** Swamp's getting-started
  flow pulls an extension and imports it (`describe`, `create`, `validate`)
  before reading its source; its adversarial-review guide covers code the
  agent writes, not code it pulls. An extension runs with whatever credential
  its CLI tool holds (`gh`: the whole account).
- **`@swamp/agent-runner` doesn't hold its limits on Claude:** `readOnly`
  restricts no tools, and with no permission-mode option a `defaultMode` in
  the user's settings overrides `allowedTools`. It downloads and pins its own
  Claude binary (323 MB per repo). Agents inherit the user's `HOME`.
- **Tests outlive their purpose.** Codex kept a one-off repair method and its
  test (log backfill for the first run) in the suite; nothing in Swamp tells a
  lasting contract from a one-time check.
- **The account gates everything.** Every command but help, version, auth and
  `start` needs a swamp-club.com credential, local datastores included; a
  login holds 14 days offline. Telemetry is on from the first run.
- **Scheduling needs `serve` running**; missed runs are not caught up.

Frictions met, all third-party: no `now()` in expressions (run 1 moved the time
window into extension code); `behavior validate` refuses the
`data.latest(self.name, …)` binding the factory's own example uses; `workflow
validate` refuses a whole-object `inputs` expression; a `*_test.ts` behavior
file is silently not registered; `findings-clear` and `cel` gates disagree on a
missing `resolved`; the factory example names skills no registry extension
ships; the skill's `swamp model method run` is `swamp behavior run`.

## Next

Run 4: the tick workflow driven by `swamp serve`, human gates as
`manual_approval` steps decided in the dashboard, to see whether approvals bind
to a user and whether software-factory's gates appear there. Then part 2:
itos as the judge, with Swamp's store as a cache of what itos derives from git.
