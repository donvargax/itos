---
status: accepted
date: 2026-10-21
---

# Each command is a vertical slice over shared engines, reaching the outside world through ports, as itos-cc's layout

## Context and Problem Statement

Does itos take itos-cc's architecture (its ADR-0051, ADR-0047, ADR-0053, docs/architecture.md): each command a vertical slice under internal/slices/<command> over shared engines, the outside world reached through ports with in-memory fakes and contract suites, the OS a value, held by depguard and architecture-check? Recommended: yes, as behaviour-neutral v8.x refactors, kong and itos-cc's exit codes in v9.0, no in-process driver yet.

Asked as q-87, about p1-slices-and-ports.

## Considered Options

- itos-cc's layout: slices, engines, ports with fakes and contract suites, depguard and architecture-check
- Keep internal/cli as it is and add ports only for the Fossil seams
- Adopt go-template-itos first and take the layout from it

## Decision Outcome

The user, 2026-10-21: yes, as fast as possible; issues found later are fixed forward, the scenarios, the contract-only corpus (ADR-0052) and previous-release covering the moves. (1) The layout and the ports land in v8.x as refactors, one command or one port operation group per task, no behaviour change; kong (p1-cli-kong) and itos-cc's exit codes (80 to 82, 100, 130, a per-problem exit) are designed now and land together as v9.0; an in-process second driver is left out unless it proves itself. (2) The gates come straight from itos-cc (pinned golangci-lint with depguard, tools/bin/architecture-check with its fixtures, no mocking library, no fake linked into the binary); p2-adopt-go-template-itos stays later. (3) Features freeze until the layout is done: only repairs and moves. (4) The first ports are the VCS (git) and the tracker store (the registry, ledgers, questions, records), the seams p2-vcs-and-tracker-seams needs for Fossil. (5) Every port method takes a context.Context first (itos-cc's decision 30); callers pass context.Background() at the edges until v9 threads cancellation. (6) Two implementing lanes in worktrees now, the moves and the corpus conversion; more once the ports and engines are out.

### Consequences

The move order (docs/plans/slices-and-ports.md): this record; the gates; global state out (the current source, the init re-entry of cli.Main); pilot moves guard, version, followup; a per-command declaration in place of the command tables, after help.yaml's contract-only conversion; the ports, the tree reader, then git by operation group, then the tracker store; the shared engines out of internal/cli (record, ids, ci watch, the commit judge, the status summary); the OS as a value; the remaining slices least coupled first, status and work last. Help stays hand-written until v9.0's kong. Mutation exceptions and .metrics/mutate snapshots move with their files. docs/ARCHITECTURE.md and the package doc comments describe the layout as each task lands.
