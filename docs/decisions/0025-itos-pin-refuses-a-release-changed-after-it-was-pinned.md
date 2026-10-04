---
status: accepted
date: 2026-10-03
---

# itos pin refuses a release changed after it was pinned

## Context and Problem Statement

Asked for the version already pinned, `itos pin` could re-pin a release whose `checksums.txt` no longer hashes to the pin.

## Considered Options

The options are those the question names.

## Decision Outcome

`itos pin` refuses (exit 1) when the pinned release's `checksums.txt` no longer hashes to the pin, rather than re-pinning: the release changed after it was pinned, which the pin exists to catch.

### Consequences

None recorded.
