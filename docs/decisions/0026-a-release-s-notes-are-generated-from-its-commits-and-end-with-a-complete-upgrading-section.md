---
status: accepted
date: 2026-10-03
---

# A release's notes are generated from its commits and end with a complete Upgrading section

## Context and Problem Statement

A consumer's session moves its pin from the release notes alone, so the notes must say everything a consumer must change. Until v2.3.0 they were written by hand in `docs/releases/`.

## Considered Options

The options are those the question names.

## Decision Outcome

Each release's notes are generated, never committed (T-069): `tools/bin/release-notes` writes them from the range, published as the release's description, and `tools/selftest/release-notes.ts` proves what a command can of them. They list every commit by type (git-cliff) and end with an Upgrading section: what each `BREAKING-CHANGE:` footer asks; every `Upgrading:` footer since the last release, quoted (from T-061); every `Changes:` entry (T-071); each config key added, removed or with a changed default, from the schema contract (T-070); and the pin to change: the install script with each platform's hash, the pin's two lines, `go install` and the schema line.

### Consequences

A commit says what a consumer must change in its `Upgrading:` footer, which is where the notes find it. The hand-written notes stay at their tags.
