---
status: accepted
date: 2026-10-11
---

# The template's interrupt rule is taken with bug-38, after v8

## Context and Problem Statement

Does itos take go-template-itos's decision 0008: an interrupted command exits 130, says so once, and a second interrupt ends it at once? Today itos lets git act on the interrupt (internal/cli/gitcommit.go) and has no rule of its own.

Asked as q-59.

## Considered Options

- Take go-template-itos's decision 0008 as a rule of its own
- Take it as part of bug-38's interruption-safety work
- Leave it

## Decision Outcome

Yes, after v8 and as part of bug-38's work rather than a rule of its own (the user, 2026-10-11). The exit code alone adds little, since a shell already reports 130 for a process Ctrl-C kills; the worth is the first interrupt letting itos finish or undo what it was doing (the drafts lock, a half-written registry or ledger, git's own operation), saying so once, and a second interrupt ending it at once. That is the interruption-safety bug-38 (atomic writes, q-22) is about, so they are built together.

### Consequences

Nothing changes in v8. bug-38's work builds the first interrupt's cleanup and the exit code together.
