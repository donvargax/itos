---
status: accepted
date: 2026-10-06
---

# The decisions waiting on the person are itos decision, and a question for anyone else is itos followup

## Context and Problem Statement

What is the group for the questions waiting on the person the work is for called, after itos question still caught 'what should I ask X?'?

Asked as q-18.

## Considered Options

- Keep itos question
- itos pending
- itos decision

## Decision Outcome

itos decision (the user's call, 2026-10-06): a name should say who answers, not what is asked. The group holds the decisions waiting on the person the work is for, which end as decision records or as --none, so it is decision add, answer, record and show, and itos decision lists them; it lines up with docs/decisions, which it writes. Anything to take up with someone else, a question included, is itos followup. itos ask and itos question exit 2 naming both. asks.yaml, work.asks, the q-<n> ids and the docs: ask q-<n> headers stay. It changes decision 36's ask-to-question line before any release has it.

### Consequences

slice-92 renames itos question to itos decision in the v6.0.0 push, before any release has question; itos ask and itos question refuse with a pointer to both; the guides say a decision needed from the person is itos decision and anything to take up with someone else is itos followup.
