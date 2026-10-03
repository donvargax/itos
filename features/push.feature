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
