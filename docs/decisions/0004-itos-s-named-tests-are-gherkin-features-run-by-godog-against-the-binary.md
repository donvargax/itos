---
status: accepted
date: 2026-10-04
---

# itos's named tests are Gherkin features run by godog against the binary

## Context and Problem Statement

itos needs named tests a footer can name and CI can select, and the same tests had to judge the TypeScript and the Go port alike.

## Considered Options

The options are those the question names.

## Decision Outcome

The named tests are Gherkin feature files at the root, `features/`, run by godog through `go test`. The steps treat itos as a black box: they run the binary `ITOS_BIN` names in scratch repositories and assert exit codes and output.

### Consequences

The same files judged the TypeScript and the Go port, unchanged, and judge any build. The steps never read itos's code, or they would stop judging the port.
