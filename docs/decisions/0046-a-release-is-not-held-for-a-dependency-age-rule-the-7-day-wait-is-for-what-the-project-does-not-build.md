---
status: accepted
date: 2026-10-08
---

# A release is not held for a dependency age rule; the 7-day wait is for what the project does not build

## Context and Problem Statement

Does cutting v7.0.0 wait on T-120, which cannot start before 2026-10-15?

Asked as q-29, about T-120.

## Considered Options

- Hold v7.0.0 until T-120 can start, so the proof is on in this repository before the major ships.
- Cut v7.0.0 when the bundle lands and let T-120 follow the same week.

## Decision Outcome

The user (2026-10-08): it does not hold the release. T-120 is this repository adopting the proof, not product behaviour: the product side shipped in slice-100 (the proof rule and work done, v7.0.0-rc.1) and slice-106 (the code proof in itos ci run, v7.0.0-rc.3), and the 7-day wait is a supply-chain rule for a dependency the project does not build itself, not for the artifacts we build. Cut v7.0.0 when the bundle lands; T-120 follows without holding it

### Consequences

Extends ADR-0023 rather than replacing it: deps-check still refuses a young version of a module the project depends on, and a release this project builds (itos itself, and a sibling repository it maintains) is not a dependency and is not held. T-120 therefore does not gate the release, and itos-cc is read as a dependency until the person says otherwise, so its pin keeps the 2026-10-15 date (T-067 refuses a release younger than 7 days). Nothing here loosens a gate: the exemption is about what the rule is for, not about skipping it
