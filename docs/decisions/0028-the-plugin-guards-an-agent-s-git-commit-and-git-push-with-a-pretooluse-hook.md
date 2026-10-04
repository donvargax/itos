---
status: accepted
date: 2026-10-03
---

# The plugin guards an agent's git commit and git push with a PreToolUse hook

## Context and Problem Statement

An agent that runs `git commit` or `git push` skips the footers `itos commit` writes and the routine `itos push` follows.

## Considered Options

- Permission rules, which match a command's prefix, so `git -C . commit` slips past them
- A `PreToolUse` hook that parses the command

## Decision Outcome

The plugin's `PreToolUse` hook (`itos hook pre-tool-use`, slice 42), in a repository with an itos config, answers a `git commit` or `git push` with the itos command to use instead: a guardrail for agents, a hook rather than permission rules.

### Consequences

The commit-msg hook stays the gate; the guard is a guardrail, not a fortress.
