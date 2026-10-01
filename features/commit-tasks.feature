@phase-1
Feature: The commit-msg hook runs the static checks of the tasks a commit names
  A Task: footer says which task a commit serves. itos hook commit-msg runs
  that task's static checks, in written order up to its first late one (the
  cost rule CI uses), so a commit that breaks a finished task is stopped
  before it is made. A task whose work item is done must still pass: a
  failure is a regression and rejects the commit. A task still in progress is
  committed in steps, so its failure is reported with the task's status and
  the commit goes through; CI judges what is pushed. A task with no work item
  counts as in progress. hooks.commit_msg.task_checks turns this off, and
  hooks.commit_msg.check_timeout caps each check, in seconds.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  @ID-CTASK-01 @slice-7 @wip
  Scenario: A done task whose static check fails rejects the commit
    Given the task "T-001" has the static check "exit 3"
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "failing T-001"

  @ID-CTASK-02 @slice-7 @wip
  Scenario: A task in progress whose static check fails is reported, and the commit goes through
    Given the task "T-001" has the static check "exit 3"
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "doing"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And its output says "failing T-001"

  # The recording check writes a file, so whether it ran is read from the tree.
  @ID-CTASK-03 @slice-7 @wip
  Scenario: With hooks.commit_msg.task_checks false, the hook runs no task check
    Given the task "T-001" has a static check that records it ran
    And hooks.commit_msg.task_checks is false
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the recording check did not run

  # A static check written below a late one is late too (the written-order
  # rule), so the hook stops at the late one and runs neither.
  @ID-CTASK-04 @slice-7 @wip
  Scenario: The hook runs a task's checks only up to its first late one
    Given the task "T-001" has the late check "exit 0" then a static check that records it ran
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the recording check did not run

  @ID-CTASK-05 @slice-7 @wip
  Scenario: A static check that outlasts hooks.commit_msg.check_timeout fails
    Given the task "T-001" has the static check "sleep 5"
    And hooks.commit_msg.check_timeout is 1
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "failing T-001"
