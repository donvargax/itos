---
status: accepted
date: 2026-10-07
---

# A task's proof is computed from its diff and its type, and its own checks are scaffolding

## Context and Problem Statement

done_when and dumb checks (p1-nightly-late-done-checks' note holds the proposal): (a) do generic standing rules, itos-enforced, match what itos was started to prevent, dumb checks never existing or never breaking? (b) ban raw string checks (grep, test) in done_when, or let them expire at done? (c) are coverage, tools exercised by a standing step and docs references resolving the right set of standing rules, or is a category missing (workflow behaviour is covered only by tools/selftest/gates.ts)?

Asked as q-13, about p1-nightly-late-done-checks.

## Considered Options

The options are those the question names.

## Decision Outcome

Settled 2026-10-07; docs/plans/v7-proof-of-done.md holds it. (a) Yes: guarantees come from rules itos enforces, never from a task's own checks. A task's proof is computed from its diff and its type: Go code by itos-cc's mutation check over the functions the item changed (--since its base, --fail-uncovered), behaviour by its scenarios, gates by their own tests run, prose by the person's review; nothing closes an item past a failing proof. (b) String checks are scaffolding: a task's checks run while it is open and gate work done, then leave the ledger; nothing reruns a done task's checks. (c) The standing rules are: no test reads a policy file as text; every gate program's tests run in CI; every command, path, scenario and decision the docs name resolves. No coverage threshold (q-6).

### Consequences

None recorded.
