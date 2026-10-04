---
status: accepted
date: 2026-10-03
---

# The plugin's hooks run the itos the repository's git hooks run

## Context and Problem Statement

The plugin's hooks must answer with the itos a repository uses, whatever is on the `PATH`.

## Considered Options

The options are those the question names.

## Decision Outcome

The plugin's hooks run the repository's effective `hooks.bin` (T-073), asked of the `itos` on the `PATH` with `itos config get hooks.bin`, a path resolved against the repository's top; with no answer (no `itos`, or one older than v2.4.0), the v2 default `tools/bin/itos` when it is executable; else the `itos` on the `PATH`.

### Consequences

A repository's own build or install script answers, and through the launcher its pin picks the version: a pin older than the guard gets no answer from the launcher rather than a block (slice 44).
