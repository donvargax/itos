---
status: accepted
date: 2026-10-02
---

# The config's regular expressions are RE2

## Context and Problem Statement

The config's patterns (`ledger.id`, `ledger.group.pattern`, `tests.<kind>.id`, `ci.cost.static`, `ci.covers[].matches`) were JavaScript's while the TypeScript ran them, and Go's standard library compiles RE2.

## Considered Options

- JavaScript's dialect, which Go could read only through a second, backtracking dependency
- RE2

## Decision Outcome

The config's regular expressions are RE2, Go's: the Go binary is what remains after the switch, and RE2 matches in linear time. Until the TypeScript went, it refused what RE2 cannot compile (lookarounds, backreferences, the escapes and classes RE2 lacks, repeat counts above 1000), so a config that passed one implementation passed the other, and `config check`'s fix names RE2 (slice 24).

### Consequences

No pattern in a project's config can hang a commit hook.
