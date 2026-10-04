---
status: accepted
date: 2026-10-04
---

# The commit-msg hook runs, last, the checks of the tasks a commit names

## Context and Problem Statement

A task's checks otherwise ran only in CI, after the push, while a finished task that fails is a regression the commit itself could have caught.

## Considered Options

The options are those the question names.

## Decision Outcome

`itos hook commit-msg` runs, last, the checks of each task a `Task:` footer names, as staged, in written order up to the first late one (CI's cost rule, not a second one), each capped by `hooks.commit_msg.check_timeout`. A failure rejects the commit when the task's work item is `done`, and is printed with the task's status otherwise: one in progress is committed in steps and CI judges the push. Last, because it is the slowest rule and reads a footer the header lint has judged. The checks run in the working tree, never in a scratch checkout.

### Consequences

Work here is one session at a time and parallel work has worktrees of its own, so what a check sees beside the commit is the committer's own.
