---
status: accepted
date: 2026-10-04
---

# itos task runs each distinct check once, and task list runs nothing

## Context and Problem Statement

Tasks share checks that take minutes (the self-tests, `config check`), so `--pending` and `--group` ran them once per task.

## Considered Options

- `task list` running only the static checks
- `task list` running nothing, printing the registry's status

## Decision Outcome

One `itos task` invocation runs each distinct check once: the same command (whitespace collapsed) with the same timeout. The first task that lists it runs it, and each later one reads that exit status by its own `run:` or `fails:`; an `after: push` check still waits for the push, and nothing is kept between invocations. `task list` prints each task's status from the work registry, or `no item`, and runs nothing.

### Consequences

Accepted limit: a check whose answer a check above it in its own task would change reads the earlier run; no ledger pair needs that today. Listing is instant and says what the people working have recorded, while what the checks find stays `itos task`'s. A push's CI plan and the commit-msg hook keep their own runs; the nightly's tasks step shares them.
