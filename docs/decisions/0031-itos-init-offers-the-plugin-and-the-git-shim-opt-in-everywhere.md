---
status: accepted
date: 2026-10-03
---

# itos init offers the plugin and the git shim, opt-in everywhere

## Context and Problem Statement

The plugin and the git shim help agents work through itos, but installing either changes the person's tools and settings.

## Considered Options

The options are those the question names.

## Decision Outcome

`itos init` offers the plugin (slice 49) through Claude Code's own CLI, `--plugin <scope>` answering (`project`, `user`, `local` or `no`; a bare `--plugin` taking `project`, or `local` under `--stealth`, which refuses `project` since the project's settings are committed); a terminal is asked, and anywhere else nothing is installed and the report says how. It offers the git shim (slice 50) the same way.

### Consequences

A `claude` that fails at the install exits 1, the rest of init done.
