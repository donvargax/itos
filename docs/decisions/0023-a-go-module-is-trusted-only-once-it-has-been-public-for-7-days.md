---
status: accepted
date: 2026-10-03
---

# A Go module is trusted only once it has been public for 7 days

## Context and Problem Statement

Supply-chain attacks are common and easy. pnpm's release age holds npm packages back, but Go has no such setting; `go get` runs no dependency code, so a check between it and the commit is safe.

## Considered Options

The options are those the question names.

## Decision Outcome

`tools/bin/deps-check` (T-067) refuses a Go module younger than 7 days unless `deps-check.json` excepts that version with a reason (an urgent security fix is exactly a young version), then runs govulncheck, itself a pinned tool dependency held to the same age; at pre-commit when `go.mod` or `go.sum` is staged, and in CI on a range that changes them, never otherwise, as it needs the network.

### Consequences

A script in this repository, in Go and on the standard library alone, so it can move unchanged into an `itos-security` extension once the user's repository template is ready (`p3-extension-pins`).
