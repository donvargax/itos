@phase-1
Feature: The nightly runs the checks of every done task
  A task's checks run in CI only when a pushed commit names the task, so a
  change elsewhere can break a done task's check unseen, until someone runs
  every done task's checks by hand. A nightly step, { tasks: done } in
  ci.nightly.steps, runs the checks of every task whose work item's status is
  done, the tasks whose red check the commit-msg hook already calls a
  regression. A task in progress, or with no work item, is left out: its
  checks may be red until it lands. They run as itos task runs them, each
  check two tasks share once. With cost: static the step runs only the
  checks the cost rule calls static, the ones that take seconds. A red check
  fails the nightly, naming its task's ID and title.

  Background:
    Given a repository whose ledger has the tasks "T-001" and "T-002"

  # The recording check passes and leaves a file; the counting check fails
  # (exit 3) and counts its runs, so each is read from the tree.
  @ID-NIGHTLY-01 @slice-16 @wip
  Scenario: The nightly runs a done task's checks and leaves out a task in progress
    Given the task "T-001" has a static check that records it ran
    And the task "T-002" has the counting check as run
    And the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "doing"
    And the nightly steps run the checks of the done tasks
    When itos runs the nightly
    Then itos exits with code 0
    And the recording check ran
    And the counting check did not run

  @ID-NIGHTLY-02 @slice-16 @wip
  Scenario: With cost static, the nightly runs a done task's static checks and leaves out its late ones
    Given the task "T-001" has a static check that records it ran
    And the task "T-002" has the late check "exit 3"
    And the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "done"
    And the nightly steps run the static checks of the done tasks
    When itos runs the nightly
    Then itos exits with code 0
    And the recording check ran

  # ci.stop_at_first_failure false, so the second task is reached after the
  # first one's check fails.
  @ID-NIGHTLY-03 @slice-16 @wip
  Scenario: A check two done tasks share runs once in the nightly
    Given the tasks "T-001" and "T-002" each have the counting check
    And the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "done"
    And the nightly steps run the checks of the done tasks
    And ci.stop_at_first_failure is false
    When itos runs the nightly
    Then itos exits with code 1
    And the counting check ran once

  # The scratch ledger titles T-001 "Tidy".
  @ID-NIGHTLY-04 @slice-16 @wip
  Scenario: A done task's red check fails the nightly, naming the task's ID and title
    Given the task "T-001" has the check "exit 3"
    And the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "doing"
    And the nightly steps run the checks of the done tasks
    When itos runs the nightly
    Then itos exits with code 1
    And its output says "T-001"
    And its output says "Tidy"
