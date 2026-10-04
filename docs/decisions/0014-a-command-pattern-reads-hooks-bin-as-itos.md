---
status: accepted
date: 2026-10-04
---

# A command pattern reads hooks.bin as itos

## Context and Problem Statement

A project calls itos by a path (`tools/bin/itos` by default), so a pattern over a check's or step's command would otherwise have to name every way a project calls it.

## Considered Options

The options are those the question names.

## Decision Outcome

Every pattern the config matches a check's or step's command with (`ci.cost.static`, a `ci.covers` rule's `matches`, a `ci.nightly_only` entry, a `tests.<kind>.recognize` template) reads a command whose first word is `hooks.bin` as starting with `itos`; a command under any other path is read as written. Each pattern is tried on both readings, so a pattern that names the path keeps matching and the reading only ever adds a match. It is for matching alone, in one place, so CI's plan, the commit-msg hook and `config check`'s written-order rule agree.

### Consequences

None recorded.
