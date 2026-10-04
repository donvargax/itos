---
status: accepted
date: 2026-10-04
---

# itos is distributed as release archives with checksums, installed pinned

## Context and Problem Statement

A consumer must get the same itos on every clone and CI runner, and a replaced release must not pass unseen.

## Considered Options

The options are those the question names.

## Decision Outcome

A release publishes archives of the Go binary for linux and darwin on amd64 and arm64 and windows/amd64, each with the binary, `LICENSE` and `README.md` at its top level, named `itos-<version>-<os>-<arch>.tar.gz` (`.zip` for windows), plus the config's JSON Schema, `itos.schema.json`, and `checksums.txt`, every asset's SHA-256. GoReleaser builds them and the archives are attested. A consumer commits an install script that pins the version and each platform's SHA-256 and installs into an ignored `.tools/bin/`, and the script `hooks.bin` names runs it before the binary. Go developers can `go install` the module, whose path ends in its major version from v2.

### Consequences

It installs on first use, in a clone and on a CI runner alike, and a replaced release cannot pass. The repository is public, so no download needs a token. The v0 and v1 releases also carried the TypeScript packed to JavaScript, a tarball that left with it (T-062). Later channels: the aqua or mise registry, a Homebrew tap, npm (as palitos) and PyPI wrappers, signatures.
