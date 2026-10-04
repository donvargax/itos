---
status: accepted
date: 2026-10-04
---

# The Go packages are the implementation path set, and the corpus stays in tools/itos

## Context and Problem Statement

The scope rules of a project made from the template read `src/**` as the implementation, and the ledger and the conformance corpus name `tools/itos/`, where the TypeScript was.

## Considered Options

The options are those the question names.

## Decision Outcome

The Go packages are `commits.path_sets.implementation`, and the scope rules read `$implementation` where a template project reads `src/**`. The corpus and the fixtures stay in `tools/itos/`, where the TypeScript was, since the ledger and the corpus name that path.

### Consequences

None recorded.
