---
status: accepted
date: 2026-10-04
---

# A global itos is a launcher that runs the version a repository pins

## Context and Problem Statement

A global install would otherwise run whatever version was installed last, whatever each repository was proven with.

## Considered Options

The options are those the question names.

## Decision Outcome

The installed binary is a launcher, never rewritten, that runs the version a repository pins (`pin.version`, and `pin.checksums`, the SHA-256 of that release's `checksums.txt`, one hash for every platform), fetched into a cache and checked (slices 27 and 28). A config with no pin runs the binary that was called, and with no `itos.yaml`, or a stealth config that pins nothing, it runs the newest release, asked for at most once a day. `itos pin [<version>]` (slice 47) moves a pin to a release, the newest by default, writing both keys in place and committing nothing.

### Consequences

A bump is the project's own build commit.
