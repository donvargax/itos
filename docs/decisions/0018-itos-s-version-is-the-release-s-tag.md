---
status: accepted
date: 2026-10-04
---

# itos's version is the release's tag

## Context and Problem Statement

Until v2.3.0 the version was `package.json`'s, bumped by hand in a commit for each release.

## Considered Options

The options are those the question names.

## Decision Outcome

The version is the release's tag (T-069): GoReleaser stamps it into a release's binary, and `tools/bin/itos` and `tools/bin/build-go.ts` stamp a build of a checkout with what `tools/bin/dev-version` reads from `git describe` (the release's version at its tag, `X.Y.(Z+1)-dev.N.g<sha>` after it, a pre-release with no `+`, so it sorts between the releases and a pin reads it). This repository commits nothing for a release.

### Consequences

Nothing in the tree holds a version, and the corpus's `{{version}}` is what the binary says. A binary built without the stamp says the module version Go records, which `go install …@v<x>` sets.
