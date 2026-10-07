---
status: accepted
date: 2026-10-06
---

# Pull requests join trunk-based work behind push.mode pr, merged by rebase-merge, claims staying on main

## Context and Problem Statement

Relax strict trunk-based work so pull requests join the way of working once work runs in parallel? On what terms?

Asked as q-19, about p1-pr-mode.

## Considered Options

The options are those the question names.

## Decision Outcome

Yes, behind a config switch, push.mode: trunk or pr, trunk the default, so it is additive. In pr mode an agent's work goes on a short-lived branch named for its item, merged by rebase-merge, never squash (each commit is a changelog entry with its footers, and the red-first test commit stays its own), the branch required to be up to date with main before it merges. Claims (work take), queue moves and the coordinator's registry commits stay direct to main, so two agents never take one item. itos push in pr mode pushes the branch, opens the PR with gh, turns on auto-merge (gh pr merge --auto --rebase; the repository allows auto-merge and a ruleset on main requires the checks) and watches its checks; since auto-merge never updates a branch main left behind, itos asks GitHub to rebase it (gh pr update-branch --rebase) rather than force-pushing; a merge queue would do it too but is offered only to organization-owned repositories. CI also runs on pull_request, its range merge-base..head. GitHub's rebase-merge gives the commits new SHAs, so work done means the PR merged and main's run green. Releases stay on main. itos init offers the repository settings. It also lets remote agents (cloud sessions pushing a branch and opening a PR) work as ordinary itos agents. Parallel agents come after, one clone or devcontainer each, never a shared worktree. The user's calls, 2026-10-06.

### Consequences

None recorded.
