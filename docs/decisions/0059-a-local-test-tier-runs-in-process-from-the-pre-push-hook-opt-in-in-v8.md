---
status: accepted
date: 2026-10-23
---

# A local test tier runs in process from the pre-push hook, opt-in in v8

## Context and Problem Statement

p1-agent-test-runs-once: where does the local tier run, how long may it take, how does a done_when shell check get its in-process run, is it breaking, and how does it start? (draft: .git/itos/design-agent-test-runs-once.md)

Asked as q-97, about p1-agent-test-runs-once.

## Considered Options

- pre-push runs the tier (chosen) or pre-commit
- budget under 2 minutes warm, cap 5 (chosen), under 1 minute, or no cap
- a done_when check declares its local command (chosen), itos recognizes known command shapes, or the local tier leaves task checks to CI
- spike first (chosen), ADR and plan first, or park it

## Decision Outcome

The user, 2026-10-23: the pre-push hook runs the local tier (it has the push range; commits stay fast). Budget under 2 minutes warm, hard cap 5; over the cap it fails naming the timing, never skips quietly. A done_when check declares its local in-process command beside its run: command (an optional key); a completeness check refuses a check without one, and an unsupported check blocks the local run, never passes vacuously. v8 ships everything additive and opt-in (the driver, the state isolation, the optional key, the structured failures, task checks at pre-push behind a config key this repository turns on); making pre-push the default instead of the commit-msg hook changes live scenarios (commit-tasks.feature) and waits for v9 with q-89. Start with a throwaway spike that proves one scenario in process before any ADR or plan; CI keeps owning task checks and the push proof (ADR-0057) meanwhile.

### Consequences

The spike (2026-10-23, branch spike-in-process 5c6e8afb) ran config.feature 54 of 54 and guard.feature 11 of 12 through internal/cli.Main in process, serially; the launcher scenario stays a binary run. With real git it saved no time (setup and git dominate), so the speed comes only with fakes, while the coverage gain (scenario-reached lines count for changed-line coverage and the mutant) comes at once. cli.Main must stop changing the process working folder and environment and reading os.Stdin before runs can go in parallel. CI keeps the binary driver with real adapters on Linux, macOS and Windows, and keeps owning task checks and the push proof (ADR-0057) until the tier is complete. The order is docs/plans/in-process-tier.md.
