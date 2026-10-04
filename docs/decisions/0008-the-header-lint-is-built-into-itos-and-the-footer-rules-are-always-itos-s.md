---
status: accepted
date: 2026-10-02
---

# The header lint is built into itos, and the footer rules are always itos's

## Context and Problem Statement

The header lint was delegated to commitlint, which a consumer of the Go binary would need Node to run. A delegate carrying the footer rules as a plugin of its config skipped them without a word when it lacked the plugin (slice 10).

## Considered Options

- The built-in lint
- A delegate, such as commitlint
- Both at once (`alongside`), which is not a config mode

## Decision Outcome

`commits.header_lint.use` is `builtin`, itos's own lint of config-conventional's rules (slice 25), or `command`, the delegate `hook` and `stdin` name (commitlint, say), which is also what no `use` means. The built-in lint replaces commitlint in v2.0.0 (the user's call, 2026-10-02: Go only, no Node for a consumer); this repository switched to it once it was held to commitlint's verdicts on its history (T-063). It rejects as commitlint does, each problem under commitlint's rule id and words, so that a project can swap one for the other. The footer rules are itos's, run beside the header lint whatever it is, never left to it.

### Consequences

None recorded.
