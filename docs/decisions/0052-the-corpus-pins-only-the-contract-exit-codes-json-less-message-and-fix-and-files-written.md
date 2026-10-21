---
status: accepted
date: 2026-10-21
---

# The corpus pins only the contract: exit codes, json less message and fix, and files written

## Context and Problem Statement

ADR-0035 makes plain text a non-contract, yet this tree's corpus pins every word (329 exact stdout, 440 exact stderr comparisons, help.yaml's 77 exact help cases). Should the corpus pin only the contract (exit codes, json: less message and fix, files_after), converting text-only behaviour cases to json: and deleting wording-only cases, with plain text at most a loose stdout_has fact? Recommended: yes, one test task per corpus file, help.yaml first; scenarios keep loose substring checks that a refusal names the next step.

Asked as q-86, about p1-json-cases-pin-json.

## Considered Options

- The corpus pins only the contract; text-only behaviour cases convert to json:, wording-only cases go
- The corpus keeps pinning every word of plain output (ADR-0035 as written)
- A transcript comparison of all printed text proves the slices refactor

## Decision Outcome

The user, 2026-10-21: yes. Printed text is exempt from breaking changes, so the corpus pins only the contract ADR-0035 names: exit codes, json: (less message and fix) and files_after. A case that checks behaviour through text converts to json:, exit and files_after (adding --json where the command has it); a case that checks only wording is deleted, help.yaml shrinking to help exiting 0 on stdout and naming its subcommands; a behaviour case on a command with no --json keeps at most a loose stdout_has on its key fact, the missing --json a gap of its own. One test task per corpus file, sized by the count, help.yaml first. Scenarios keep loose substring checks that a refusal names the next step; they never pin a whole line. No transcript comparison of printed text is added for the slices refactor: the contract-only corpus and previous-release are its proof.

### Consequences

Amends ADR-0035, whose sentence 'this tree's own corpus still pins every word' no longer holds; the rest of 0035 stands. p1-json-cases-pin-json becomes one test task per corpus file, help.yaml first, before the per-command declaration of the slices refactor (p1-slices-and-ports) reorganises help. A command a behaviour case needs and that has no --json is a gap recorded as its own idea. The refactor gets no printed-text proof.
