---
status: accepted
date: 2026-10-04
---

# The work registry lives beside the ledger by default

## Context and Problem Statement

The registry is itos's data, as the ledger is, and read every day; its old default, `docs/work-items.yaml`, put it among the prose.

## Considered Options

The options are those the question names.

## Decision Outcome

`work-items.yaml` in the folder `ledger.files` names: `tasks/work-items.yaml` for the default ledger and with no ledger; `work.registry` puts it anywhere else. The default is the one table's, laid over with the config's ledger, so every command that reads the registry and `config check --print-defaults` agree (slice 22).

### Consequences

`docs/` is left for prose, and a project that keeps its ledger in another folder finds the registry there too. With none where itos looks, the commands that read it say so, naming the path, so a project with its registry at the old default learns where itos reads it now.
