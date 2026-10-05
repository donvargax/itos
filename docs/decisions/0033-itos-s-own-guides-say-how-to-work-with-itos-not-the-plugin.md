---
status: accepted
date: 2026-10-04
---

# itos's own guides say how to work with itos, not the plugin

## Context and Problem Statement

With the plugin's itos skill dropped (sessions start with ! itos go, and every implementer runs itos guide work), where does how to work with itos live, now that ADR-0032 says it stays in the plugin's skill?

Asked as q-12, about p3-plugin-guide-wrappers.

## Considered Options

The options are those the question names.

## Decision Outcome

The config's rules are generated into a marked block of AGENTS.md, itos touching only what is inside its markers. How to work with itos lives in itos's own guides, versioned with the binary: itos go for the session the person talks to, itos guide work for an implementer, told to run it by its brief. No flow needs the Claude Code plugin, which keeps its titles and its guard and drops its itos skill. --agents writes the orchestration files once, as the project's own. Under --stealth, Claude Code gets a CLAUDE.local.md importing the rules and Codex an AGENTS.override.md holding a copy of the project's AGENTS.md beside them, both listed in .git/info/exclude. (The user's calls, 2026-10-04.)

### Consequences

None recorded.

## More Information

Supersedes ADR-0032.
