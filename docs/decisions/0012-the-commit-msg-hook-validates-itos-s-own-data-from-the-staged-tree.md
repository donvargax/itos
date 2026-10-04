---
status: accepted
date: 2026-10-04
---

# The commit-msg hook validates itos's own data from the staged tree

## Context and Problem Statement

A project's pre-commit hook passes `itos.yaml`, the ledger, the registry and the smoke set as prose; a rule a command can decide belongs in itos, and the working tree can hold what the commit does not.

## Considered Options

- A check in each project's pre-commit hook, which read the working tree
- itos's commit-msg hook, reading the staged tree

## Decision Outcome

`itos hook commit-msg` validates itos's own data (`itos.yaml`, the ledger, the registry, the smoke set) when a commit stages any of it, with `config check`'s problems read from the staged tree (T-022).

### Consequences

None recorded.
