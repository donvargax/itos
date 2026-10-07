---
status: accepted
date: 2026-10-06
---

# A written file is contract by its data, not its comment lines

## Context and Problem Statement

Decision 35 makes the files itos writes part of its contract. Are their comment lines part of it too?

Asked as q-20.

## Considered Options

The options are those the question names.

## Decision Outcome

No. A written file is contract by its data; its comment lines are for people, like plain output, and may change in any release. This refines decision 35: docs/CLI.md's contract section says the data of the files itos writes, and the conformance runner's --additive mode, which previous-release uses, stops comparing comment lines in written YAML and Markdown. The user's call, 2026-10-07, recorded from that session's transcript.

### Consequences

None recorded.
