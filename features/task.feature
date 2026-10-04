@phase-1
Feature: The task runner runs each check once, and lists task status without running any
  itos task runs the checks of the tasks it is given: by ID, by --group
  (or --phase), or every task that is not done (--pending). Tasks often list
  the same check, such as a self-test or config check, and one check can take
  minutes. So one invocation runs each check only once. A check is the same
  check when it has the same command and the same timeout. It runs the first
  time a task lists it, and each later task that lists it reuses that run's
  exit code, read the way its own run: or fails: says. itos task list runs
  no check at all. It prints each task's status from the work registry (todo,
  doing, done or blocked), or "no item" for a task the registry does not
  have. itos task add writes a new task into the ledger and its item into
  the registry.

  Background:
    Given a repository whose ledger has the tasks "T-001" and "T-002"

  # The counting check appends a line to a file each time it runs, then
  # exits 3, so its run count is read from that file.
  @ID-TASK-01 @slice-9
  Scenario: task --pending runs a check two tasks list only once, and both report its result
    Given the tasks "T-001" and "T-002" each have the counting check
    When itos runs the pending tasks
    Then itos exits with code 1
    And its output lists "T-001" as "failing"
    And its output lists "T-002" as "failing"
    And the counting check ran once

  @ID-TASK-02 @slice-9
  Scenario: task --group runs a check two tasks list only once
    Given the tasks "T-001" and "T-002" each have the counting check
    When itos runs the tasks of the group "1"
    Then itos exits with code 1
    And the counting check ran once

  @ID-TASK-03 @slice-9
  Scenario: A command one task lists as run and another as fails runs once, and each task reads the result its own way
    Given the task "T-001" has the counting check as run
    And the task "T-002" has the counting check as fails
    When itos runs the tasks "T-001" and "T-002"
    Then itos exits with code 1
    And its output lists "T-001" as "failing"
    And its output lists "T-002" as "done"
    And the counting check ran once

  @ID-TASK-04 @slice-9
  Scenario: task list prints each task's status from the work registry and runs no check
    Given the task "T-001" has a static check that records it ran
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "doing"
    When itos lists the tasks
    Then itos exits with code 0
    And its output lists "T-001" as "doing"
    And its output lists "T-002" as "no item"
    And the recording check did not run

  # Slice 55 (the user's calls, 2026-10-03): a task is added by a command, as
  # registry items are (slice 54). task add writes the task into the ledger
  # file of its group (ledger.files with {group} filled in), with its type,
  # title, why and checks (--check, one per check), and the registry item of
  # kind task beside it, todo; it refuses an id ledger.id does not match, an
  # id the ledger has, and a type commits.types does not list; and it commits
  # the ledger file and the registry alone.
  @ID-TASK-05 @slice-55
  Scenario: task add writes the task into its group's ledger file and its item into the registry, and commits both
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    When itos runs the command line "task add T-003 --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 0
    And the ledger file "tasks/phase-1.yaml" has the task "T-003" with the check "true"
    And the registry's item "T-003" is a task titled "Tidy the readme" with the status "todo"
    And the last commit's header is "docs: add T-003"
    And the last commit touches only "tasks/phase-1.yaml" and "tasks/work-items.yaml"

  @ID-TASK-06 @slice-55
  Scenario: task add refuses an id the ledger already has
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    When itos runs the command line "task add T-002 --group 1 --type chore --title 'Again' --why 'A second one.' --check 'true'"
    Then itos exits with code 1
    And its output says "T-002"

  @ID-TASK-07 @slice-55
  Scenario: task add refuses an id that ledger.id does not match
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    When itos runs the command line "task add task-3 --group 1 --type chore --title 'Tidy' --why 'Because.' --check 'true'"
    Then itos exits with code 1
    And its output says "ledger.id"

  @ID-TASK-08 @slice-55
  Scenario: task add refuses a type the config's commits.types does not list
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    When itos runs the command line "task add T-003 --group 1 --type tidy --title 'Tidy' --why 'Because.' --check 'true'"
    Then itos exits with code 1
    And its output says "tidy"
