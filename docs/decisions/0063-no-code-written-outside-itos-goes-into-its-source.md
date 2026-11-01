---
status: accepted
date: 2026-11-01
---

# No code written outside itos goes into its source

## Context and Problem Statement

May code written outside itos go into its source?

Asked as q-106.

## Considered Options

- Allow code under a compatible license (itos is AGPL-3.0), with its notices kept
- Allow no outside code; reuse ideas and designs only

## Decision Outcome

No. No code written outside itos goes into its source, whatever its license: not copied, vendored, or ported line by line. Another project's ideas and designs are fine, reimplemented here from what they do. Libraries taken as dependencies through go.mod stay allowed, as deps-check governs them. Code from Open Orchestra, which the person co-owns, is the one exception.

### Consequences

itos's copyright stays with its owners, so they can license it as they choose, a hosted edition included. A port from another project starts from its documented behaviour, never from its source text. An agent that wants code from elsewhere reimplements it or proposes a dependency.
