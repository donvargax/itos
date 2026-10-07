---
status: accepted
date: 2026-10-07
---

# No coverage threshold: mutants judge the tests, and --fail-uncovered refuses untested changed code

## Context and Problem Statement

Set a unit coverage threshold? Coverage is collected with none since the template's demo app left; it blocks nothing.

Asked as q-6.

## Considered Options

The options are those the question names.

## Decision Outcome

No (2026-10-07): no coverage threshold, global or as a floor that only rises. A percentage rewards tests that touch lines without checking them; itos-cc's --fail-uncovered refuses untested new code in the functions an item changes, and mutants judge whether a test notices a change. Coverage stays collected, for itos-cc to read, including the features' through Go's integration coverage (T-121, itos-cc#14).

### Consequences

None recorded.
