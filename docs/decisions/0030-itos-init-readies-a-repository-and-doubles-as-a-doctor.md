---
status: accepted
date: 2026-10-03
---

# itos init readies a repository and doubles as a doctor

## Context and Problem Statement

Adopting itos meant writing a config, a ledger, a registry, a smoke set, a pin and the hooks by hand.

## Considered Options

The options are those the question names.

## Decision Outcome

`itos init` (slice 48) makes a repository ready, new (`git init` first) or existing: where there is no config, a starter `itos.yaml`, small and commented, with `commits.since` at HEAD so history written before itos is never judged; a ledger holding `T-1`, the task the adoption commit names (kept; under `--stealth` the ledger starts empty, since nothing is committed); an empty registry; a `Scenarios` footer and a smoke set when feature files exist; a pin on the newest release; then the hooks. It is the launcher's own command, as `pin` is: where there is no config there is no pin to hand the run to.

### Consequences

Run again where a config is, it changes nothing and reports what is missing, exit 1 when anything is, so it doubles as a doctor; it also reports, never counting them as missing, the plugin not installed, the git shim, a pin behind the newest and the people file.
