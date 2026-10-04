---
status: accepted
date: 2026-10-02
---

# itos is implemented in Go

## Context and Problem Statement

itos began as TypeScript, v0, in `tools/itos/`, run by Node directly. It is meant to be usable in any project as one binary with no runtime to install, and while two implementations lived side by side every feature had to be built twice.

## Considered Options

- Keep the TypeScript beside the Go build for a shadow period
- Go only, as soon as the Go build passes everything

## Decision Outcome

itos is Go: the binary is `cmd/itos` and its packages are under `internal/`. The Go port landed beside the TypeScript, one command group at a time, and the TypeScript left once Go passed everything and this repository ran the Go binary (T-062, the user's call: Go only as soon as possible).

### Consequences

A consumer needs no Node. Every feature is built once, in Go.
