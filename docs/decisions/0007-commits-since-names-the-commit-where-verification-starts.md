---
status: accepted
date: 2026-10-04
---

# commits.since names the commit where verification starts

## Context and Problem Statement

A repository made from a template begins with one squashed commit no rule passes, and a project adopting itos has a history written before its rules; both must start clean with it.

## Considered Options

- Skip the root commit, which covers only the first case
- Name the commit in the config, `commits.since`

## Decision Outcome

`commits.since` names the commit where verification starts: `verify` and every range check skip it and its ancestors, and the commit-msg hook is unaffected. A footer's own `since` does the same for one rule: verify leaves that commit and its ancestors out of the footer's `required_for`, so a footer required later does not fail the history before it (T-062 met that trap); the commit-msg hook always requires it (slice 26).

### Consequences

None recorded.
