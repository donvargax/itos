---
status: accepted
date: 2026-10-22
---

# Task checks run in process over fakes in the hooks, and with real adapters in CI

## Context and Problem Statement

Who runs a task's done_when checks, and how do agents trust it instead of running them by hand?

Asked as q-94, about p1-agent-test-runs-once.

## Considered Options

- Two tiers, as itos-cc: the hooks run done_when in process over the ports' fakes; CI runs them with real git and GitHub (chosen)
- CI only: one run per push, but every failure turns main red before anyone hears of it
- Agents run itos task before each push (today): a by-hand repeat of what CI runs

## Decision Outcome

The user, 2026-10-22: both tiers run them, as itos-cc does (its decisions 44 and 52). Locally, the hooks run each check in process over the ports' fakes, fast and without the network. CI runs the same checks with the real adapters, git and GitHub. Agents never run done_when by hand. When a check fails, itos says so with enough to fix it: the task, the check's command, its exit code and its output, in text and --json, from the hook and from itos push or ci watch. done_when holds only what no gate already runs. This brings back the in-process second driver, which the refactor plan had left out unless it proved itself (option C).

### Consequences

Agents stop running done_when, lint and unit tests by hand; briefs and AGENTS.md drop those lines. itos push, ci watch and the hook print a failing check's task, command, exit code and output. done_when lists only what no gate runs. The features gain an in-process driver over the fakes (the refactor plan's option C), designed before it is built (p1-agent-test-runs-once).
