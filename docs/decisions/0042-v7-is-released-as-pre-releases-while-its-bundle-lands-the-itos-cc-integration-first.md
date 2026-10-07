---
status: accepted
date: 2026-10-07
---

# v7 is released as pre-releases while its bundle lands, the itos-cc integration first

## Context and Problem Statement

How is v7 released while its bundle lands one item at a time, given that every green push with a breaking commit cuts a major of its own today?

Asked as q-24, about p1-nightly-late-done-checks.

## Considered Options

The options are those the question names.

## Decision Outcome

As pre-releases: while itos.yaml carries a pre-release marker (T-118), the commits that would cut 7.0.0 cut 7.0.0-rc.1, rc.2 and so on, marked pre-release and never latest, so go install @latest, the update notice and a bare itos pin keep naming 6.x, and a repository opts in by pinning an rc. 6.x is finished first (T-117, T-113 and the two upgrade items), since moving the module path to /v7 for the first rc ends 6.x patches from main. The itos-cc integration lands first, as v7.0.0-rc.1 (the role protocol, the proof rule and work done, CI's mutation sample, expiring checks); the rest of the bundle follows as further rcs; removing the marker cuts v7.0.0. The user's calls, 2026-10-07; docs/plans/v7-proof-of-done.md holds the plan.

### Consequences

None recorded.
