---
status: accepted
date: 2026-10-03
---

# This repository is a Claude Code plugin marketplace for the itos plugin

## Context and Problem Statement

Agents work with itos in Claude Code, and the plugin that helps them must be installable from somewhere.

## Considered Options

The options are those the question names.

## Decision Outcome

`.claude-plugin/marketplace.json` at the root names the itos plugin in `integrations/claude-code/` (T-066), with a version of its own in its `plugin.json`: it followed itos's until T-069 made the tag itos's version, and may move to a repository of its own. The plugin carries the titles (an itos ID drawn with its title in Claude's replies, from the project's own itos), a short skill on working with itos, and a `PreToolUse` hook.

### Consequences

A change to the plugin raises its version, since Claude Code offers an installed plugin an update only when it changes.
