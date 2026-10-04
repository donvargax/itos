---
status: accepted
date: 2026-10-03
---

# CI cuts a release from a green push, judged against the last release

## Context and Problem Statement

Releases were tagged by hand by the coordinator after reading the Upgrading section (2026-10-02), and nothing stopped a breaking change shipping as a minor.

## Considered Options

- Tagged by hand after reading the Upgrading section
- Cut by CI, with no review

## Decision Outcome

A push to `main` whose CI is green and whose commits since the last tag carry a `feat`, a `fix` or a breaking change cuts the release, with no review (T-069): its version computed from those commits (`tools/bin/release-version`: a breaking change a major, a feat a minor, a fix a patch), tagged, published with GoReleaser and attested, its notes generated from the commits, nothing committed by the releaser. The push's CI holds the schema contract (T-070): a config the last release accepts is still accepted, with the same defaults, unless a commit since carries `BREAKING-CHANGE` or a `!`. It runs the last release's scenarios and corpus against the new binary (T-071): an old one that fails is accepted only with a breaking change, or when a fix names it in a `Changes:` footer.

### Consequences

The old corpus's help cases are left out (2026-10-03): help text is documentation, not compatibility, `--json` and exit codes being the stable interface. Its usage errors are judged by their exit code alone (T-075), a config error keeping its words judged. Its output is judged additively (T-076): a key or a line added passes, a removal or a change is a break. This tree's own corpus still pins every output exactly.
