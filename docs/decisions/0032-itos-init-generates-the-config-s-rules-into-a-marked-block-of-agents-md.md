---
status: superseded by ADR-0033
date: 2026-10-03
---

# itos init generates the config's rules into a marked block of AGENTS.md

## Context and Problem Statement

Agents read their instructions, and a rule copied there by hand beside the config drifts from it.

## Considered Options

The options are those the question names.

## Decision Outcome

The config's rules are generated into a marked block of `AGENTS.md`, itos touching only what is inside its markers, how to work with itos staying in the plugin's skill. `--agents` writes the orchestration files (the coordinator's guide, the handoff, the brief template) once, as the project's own, a candidate to move into an extension later. Under `--stealth`, Claude Code gets a `CLAUDE.local.md` importing the rules and Codex an `AGENTS.override.md` holding a copy of the project's `AGENTS.md` beside them, both listed in `.git/info/exclude`.

### Consequences

None recorded.
