---
status: accepted
date: 2026-10-04
---

# The features' harness selects scenarios by a regular expression over their tags

## Context and Problem Statement

godog's tag filter takes exact tags joined by commas, while itos's run templates join IDs into one regular expression with `|`.

## Considered Options

The options are those the question names.

## Decision Outcome

The harness takes `-scenarios=<regexp>` over the tags and turns it into godog's filter, so the templates stay as they are.

### Consequences

None recorded.
