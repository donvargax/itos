---
status: accepted
date: 2026-10-21
---

# Script-facing plain outputs and the guard's hook JSON are contract; guide text is exempt like message and fix

## Context and Problem Statement

The corpus count for ADR-0052 found outputs ADR-0035 does not name: plain stdout meant for scripts (ci scope's docs_only=, ci range's FROM= and TO=, config get, tests next-id, tests smoke ids), guard claude-code's hook JSON read by Claude Code and the itos plugin, and guide --json's and go --json's text key holding guide prose. Which are contract? Recommended: the script outputs and the guard JSON are contract and stay pinned exactly; text is exempt like message and fix.

Asked as q-88, about p1-json-cases-pin-json.

## Considered Options

- Declare the script outputs and the hook JSON contract, exempt guide text
- Not contract: scripts use --json, the hook JSON changes with the plugin, guide text judged

## Decision Outcome

The user, 2026-10-21: each recommendation. Contract: the plain stdout meant for scripts (ci scope's docs_only=, ci range's FROM= and TO=, config get, tests next-id, tests smoke ids) and guard claude-code's hook JSON; their corpus cases keep their exact pins (the hook JSON with json:). Exempt like message and fix: the text key of guide --json and go --json. A usage case keeps its exit code and a loose stderr_has naming the offending flag or argument (a fact, not a wording), so it cannot pass for another reason (the coordinator's call).

### Consequences

Amends ADR-0035's contract list and ADR-0052's corpus rule. previous-release compares these plain outputs exactly and leaves out text as it leaves out message and fix (a ci change in the corpus runner task). Their help says they are for scripts. Changing one of them is a breaking change.
