---
status: accepted
date: 2026-10-02
---

# The Go port's proof is its tasks' checks, landed as refactor commits

## Context and Problem Statement

The Go port reimplemented behaviour the TypeScript already had and the scenarios already specified, and the two implementations could drift while both lived.

## Considered Options

- A nightly-only guard, which reports the drift a day late
- `feat` with a `Task:` footer, which changes the rule for every feat
- `refactor` commits with a `Task:` footer, held to the ported set on every push

## Decision Outcome

Each command group of the port is a task in `tasks/phase-2.yaml` whose checks run its part of the conformance corpus and its scenarios, with `ITOS_BIN` pointed at the Go build; its code lands as `refactor` commits with a `Task:` footer, since a `Scenarios:` footer would run the scenarios against the TypeScript alone. Once a group lands, its part joins the ported set, which `tools/selftest/go-port.ts` runs against the Go build in every push's CI, so a later change to a ported group lands in both implementations in one push. Since T-053 that set is the whole corpus and every feature.

### Consequences

With the TypeScript gone (T-062) there is nothing to drift from: CI's corpus step and its one features run judge `tools/bin/itos`, and `go-port.ts` is the port's tasks' check.
