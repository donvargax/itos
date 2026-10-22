---
status: accepted
date: 2026-10-22
---

# CI owns the push proof and the task checks until the hooks run them in process

## Context and Problem Statement

Until the in-process tier lands (ADR-0056), do agents still run CI's push proof, or a task's checks, by hand before pushing?

Asked as q-95, about p1-agent-test-runs-once.

## Considered Options

- CI alone runs them, an agent fixing forward what it turns red (chosen)
- Agents run CI's push proof once before pushing Go (q-55): an exact repeat, which still drew a different site than CI's
- A pre-push changed-line coverage check over the unit tests alone: a stopgap the in-process tier makes unneeded

## Decision Outcome

The user, 2026-10-22: no. CI owns both: the push proof (one seeded mutant and the changed lines' coverage, every push) and every named task's checks. An agent's by-hand run repeated CI's exactly and still missed its draw (T-267 seeded from its take commit, CI from the last green run, main went red). A red CI is fixed forward by the agent that pushed. This retires q-55. When p1-agent-test-runs-once builds the in-process driver over the fakes, the pre-push hook runs the local tier automatically: task checks, changed-line coverage and a mutant of the changed functions over the unit tests and in-process scenarios, as itos-cc runs TestScenarios locally.

### Consequences

AGENTS.md and docs/ORCHESTRATING.md drop the by-hand push proof and itos task before a push; briefs drop them and the by-hand lint and staticcheck runs pre-push already makes. ADR-0056's in-process tier, when built, adds the local mutant and changed-line coverage to the pre-push hook. q-55 is retired.
