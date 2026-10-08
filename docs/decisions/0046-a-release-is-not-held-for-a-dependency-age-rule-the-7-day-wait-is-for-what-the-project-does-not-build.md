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

Extends ADR-0023 rather than replacing it: deps-check still refuses a young version of a module the project depends on, and a release this project builds is not a dependency and is not held. **itos-cc is one of ours** (the person, 2026-10-08), so `tools/bin/pinned` takes its newest release the day it is published and waiting seven days to try a new itos-cc feature is not a trade worth making; T-120 can proceed as soon as the required release exists. Every other dependency keeps the wait, and the rest of T-067's rule is untouched: the pinned tool is fetched with its checksums, govulncheck still runs, and nothing is skipped but the age. T-120 does not gate v7.0.0 either way
