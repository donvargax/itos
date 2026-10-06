---
status: accepted
date: 2026-10-06
---

# itos installs its hooks only in the git config, and knows no hook manager

## Context and Problem Statement

How does itos install its hooks from v6.0.0, now that it refuses to commit or push where they cannot run?

Asked as q-17.

## Considered Options

- Write shims into each hook manager's layout, detected by marker files (v5)
- A hooks.install command the project names, quoted when hooks cannot run
- Declare the hooks in the git config alone, everywhere

## Decision Outcome

Only in the git config, everywhere (the user's call, 2026-10-06): itos hook install declares its commit-msg and pre-push hooks as hook.<name>.event and hook.<name>.command in .git/config, which every worktree of the clone shares and git runs beside any hooks folder, whatever core.hooksPath says. itos knows no hook manager: the marker detection, the writing in each manager's layout, hook install's --manager and hooks.manager go, and a project's own hooks are its own business. Every itos command that commits or pushes refuses, exit 3, when its hooks are not declared or the git cannot run hooks its config declares, naming itos hook install or the git to upgrade. Each fresh clone installs once; a git too old for config hooks is the cost, recorded with the version that added them. Issue #16 would not have happened: the fresh worktree shared the config.

### Consequences

slice-91 removes the hook managers' code, --manager and hooks.manager, and refuses to commit or push where itos's config hooks will not run. This repository's .vite-hooks commit-msg and pre-push lines that call itos go, its vp staged pre-commit stays. Older gits cannot run config hooks, so they are refused with the version to upgrade to.
