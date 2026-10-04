---
status: accepted
date: 2026-10-04
---

# The nightly runs the checks of every done task

## Context and Problem Statement

A task's checks run in CI only when a push names it, so a change elsewhere broke done tasks unseen (T-024 here, a consumer's self-test).

## Considered Options

The options are those the question names.

## Decision Outcome

`ci.nightly.steps` takes `{ tasks: done }`, with an optional `cost: static`: the checks of every task whose work item is `done` run every night, where the step is written, in cost order, each shared check once as in `itos task`; a red one fails the nightly, naming the task's ID and title. Refused in `ci.steps`, whose tasks are the ones the commits name.

### Consequences

Done is the registry's status, not every status `ci.wait_on_status` lets through: a task in progress may be red until it lands, and `done` is what the commit-msg hook calls a red check a regression by.
