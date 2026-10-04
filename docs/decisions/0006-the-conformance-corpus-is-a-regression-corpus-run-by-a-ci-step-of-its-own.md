---
status: accepted
date: 2026-10-04
---

# The conformance corpus is a regression corpus run by a CI step of its own

## Context and Problem Statement

The conformance YAML in `tools/itos/conformance/` records what v0 does, as cases run through its command line, and both implementations were held to it during the port.

## Considered Options

The options are those the question names.

## Decision Outcome

The conformance YAML stays as a corpus itos must pass, as both implementations did, run by a CI step of its own, not as named tests.

### Consequences

Any implementation passes it. A case changes only with the behaviour it records, in the same commit.
