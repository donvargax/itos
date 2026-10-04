---
status: accepted
date: 2026-10-02
---

# v1.0.0 publishes the Go archives, checksums and the config's JSON Schema

## Context and Problem Statement

The port's close needed a first Go release, while the consumers still ran the TypeScript tarball.

## Considered Options

The options are those the question names.

## Decision Outcome

v1.0.0 publishes the Go archives, `checksums.txt` and the config's JSON Schema (generated from the Go config's table, never kept by hand) beside the TypeScript tarball, which stays until phase 3 switches the consumers; the release job proves the very archives it uploads.

### Consequences

None recorded.
