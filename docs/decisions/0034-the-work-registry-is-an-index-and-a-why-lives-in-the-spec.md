---
status: accepted
date: 2026-10-04
---

# The work registry is an index, and a why lives in the spec

## Context and Problem Statement

p1-registry-archive: dropping done items from the registry (your call of 2026-10-04) collides with what is released: work list promises every item, done ones too; the nightly's done-task step and slice 7 read done-ness from the registry, so a task whose item is gone would count as in progress; and reading dropped items back from close commits means parsing git history. Move each closed item to an archive file beside the registry (tasks/work-items-done.yaml) that work done writes, and that work list, work show, ids and the nightly read too: the live registry shrinks to open work, nothing released breaks, no history is parsed (the coordinator's recommendation)? Or drop them as decided and cut v4.0.0 for it?

Asked as q-11, about p1-registry-archive.

## Considered Options

The options are those the question names.

## Decision Outcome

Neither (the user's calls, 2026-10-04): the registry is an index, its whys an idea's alone, a slice's or bug's living in its feature file, a task's in its ledger, a change's in its commit; work done drops the closed item's why, so the registry grows with open work only. Slices 76 to 79, the breaking part in v4.0.0 with work list open by default.

### Consequences

None recorded.
