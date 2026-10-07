---
status: accepted
date: 2026-10-06
---

# The commands the Claude Code plugin calls are part of the contract, never renamed or removed

## Context and Problem Statement

The Claude Code plugin calls itos commands (guard claude-code, and work list --all and task list with --json for the titles): may a release rename or remove them, and does the launcher map them to an older pin's names?

Asked as q-21, about slice-94.

## Considered Options

The options are those the question names.

## Decision Outcome

They are part of the contract with the plugin, never renamed or removed, even in a major release. The v6.0.0 rename stands, with no compatibility kept for pins older than v6: no launcher mapping. docs/CLI.md rule 43 gains the exception, and the entry-points rules name these commands as the plugin's contract; a ci check that every itos command the plugin's scripts call is answered by the built itos makes the promise a gate, so slice-94, which specified the launcher mapping, is dropped. The user's calls, 2026-10-07, recorded from that session's transcript.

### Consequences

None recorded.
