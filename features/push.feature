@phase-3
Feature: itos push, the pull-rebase-push routine as one command
  Every session finished the same way by hand: commit, git pull --rebase
  --no-autostash, check that no rebase stopped and no conflict is left, then
  push in a separate command, never piped and never forced. itos push is
  that routine, built in since it is core (the user's call, 2026-10-03:
  extensions are for what is not). It rebases onto the branch's upstream with
  its own flags, so git's pull.rebase and rebase.autostash settings change
  nothing; it refuses to start with uncommitted changes to tracked files,
  which autostash would otherwise pocket; a rebase that stops leaves the
  rebase for the person to finish and pushes nothing; and it never forces:
  a push the remote refuses is reported, not retried with force. The hooks
  run as for any push. It is itos commit's other half.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # pull.rebase false would make a plain git pull merge; itos push's own
  # flags are what keep the history a line.
  @ID-PUSH-01 @slice-39
  Scenario: The local commits are rebased onto the remote's and pushed, the history a line
    Given the clone's git config says "pull.rebase" is "false"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the docs" then "chore: tidy the readme", with no merge commit

  @ID-PUSH-02 @slice-39
  Scenario: A rebase that stops on a conflict pushes nothing and says how to go on
    Given the remote has gained the commit "chore: reword the readme" touching "README.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git rebase --continue"
    And the remote's branch does not have "chore: tidy the readme"

  # rebase.autostash true would stash the change and could leave it stashed;
  # itos push refuses before pulling instead.
  @ID-PUSH-03 @slice-39
  Scenario: Uncommitted changes to tracked files are refused before anything is pulled
    Given the clone's git config says "rebase.autostash" is "true"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    And the clone's "README.md" has an uncommitted change
    When itos runs "push"
    Then itos exits with code 1
    And its output says "commit"
    And the remote's branch does not have "chore: tidy the readme"
    And the clone's "README.md" still has its uncommitted change

  @ID-PUSH-04 @slice-39
  Scenario: itos push never forces
    When itos runs "push --force"
    Then itos exits with code 2
    And its output says "force"

  # Slice 66: two people taking the same item edit the same lines of the
  # registry, so the second push's rebase stops on the conflict and nothing
  # lands twice; the conflict is the lock. itos push names the registry and
  # says what happened in a person's words, rather than git's alone.
  @ID-PUSH-05 @slice-66
  Scenario: A take that meets another person's take of the same item stops, and says the item was taken
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the remote has gained the commit "docs: take slice-9" making "ana" the owner of "slice-9"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "tasks/work-items.yaml"
    And its output says "taken"
    And the remote's registry gives "slice-9" the owner "ana"

  # git resolves HEAD before the pre-push hook runs, but itos push read HEAD
  # again after git push returned, so a commit made while the hook ran (its
  # unit tests take minutes) was named as pushed, and push waited on a CI run
  # that would never come (found by bug 20's agent, 2026-10-04,
  # p1-push-names-head-after-hook). itos push resolves the commit once,
  # before pushing, and pushes, names and waits on that one.
  @ID-PUSH-06 @bug-21
  Scenario: itos push names the commit git pushed, though a commit lands while its pre-push hook runs
    Given the pre-push hook is installed
    And hooks.pre_push's command commits "docs: late" touching "late.md" in the clone
    And the clone has the commit "docs: mine" touching "mine.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the remote's branch has "docs: mine"
    And the remote's branch does not have "docs: late"
    And its output names the remote branch's head as the commit pushed
