---
status: accepted
date: 2026-10-22
---

# On a git older than 2.54, hook files are a temporary compatibility path; the git config stays the default

## Context and Problem Statement

On a git older than 2.54, which runs no hook its config declares, how does itos install its hooks, and does the git config stay the default?

Asked as q-96, about slice-165.

## Considered Options

- Hook files only where git runs no config hooks, the git config the default elsewhere, removed in a few months (chosen)
- Hook files everywhere: drops ADR-0037's reasons (hook managers, issue #16) for every git
- No support below 2.54: itos unusable on the gits most distributions ship

## Decision Outcome

The user, 2026-10-22: the git config stays the default and the only place itos writes on a git that runs its config's hooks (ADR-0037 holds there). On an older git, down to 2.51, hook install writes hook files into the folder git runs hooks from (git rev-parse --git-path hooks), an untracked hook already there kept and run first; a hook file git tracks is never written, the refusal naming the file and the line to add; a project's own hook file that calls itos's hook satisfies the commands that commit or push. Scenarios fake the old git with a wrapper; a real git 2.51 job in CI is a later task. This is a compatibility path, kept a few months until most people's git runs config hooks, then removed.

### Consequences

ADR-0037 holds on git 2.54 and later. hook install, and the Ready check of every command that commits or pushes, learn the hooks folder for an older git. A later item removes the compatibility path once most people's git runs config hooks (p1-drop-hook-files-compat).
