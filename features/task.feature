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

  # Bug 13, found by slice 55's post-landing review: writeCommitted, which
  # writes a command's files and commits them alone, judged a file missing
  # from the working tree as new, so a ledger file deleted but not committed
  # was written afresh and committed over the deletion, and its rollback was
  # never tested. task add now refuses a file with changes no commit holds,
  # a deletion among them, before writing anything; and a commit a hook
  # refuses puts every file back, the index too. The holes no scenario can
  # reach (a second write failing after the first, a git that cannot start,
  # git add's words swallowed) are the fix's unit tests.
  @ID-TASK-09 @bug-13
  Scenario: task add refuses a ledger file whose deletion no commit holds, and writes nothing
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    And the ledger file "tasks/phase-1.yaml" is deleted and the deletion not committed
    When itos runs the command line "task add T-003 --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 1
    And its output says "tasks/phase-1.yaml"
    And the file "tasks/phase-1.yaml" does not exist
    And the registry has no item "T-003"

  @ID-TASK-10 @bug-13
  Scenario: task add whose commit a hook refuses leaves the ledger and the registry as they were, and nothing staged
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    And a commit-msg hook that refuses every commit
    When itos runs the command line "task add T-003 --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 1
    And the registry has no item "T-003"
    And the ledger has no task "T-003"
    And git reports no change to the working tree or the index

  # Slice 101: work done takes a task's checks out of the ledger when it
  # closes the task (@ID-WORK-80), so a done task has none, and itos task
  # reads a task with no checks by its work item: done when the item is
  # done, review otherwise (no item, or one not done), as before. A done
  # task that still has checks, closed before slice 101, runs them as
  # before. --pending leaves out the done ones, as it leaves out every task
  # it calls done.
  @ID-TASK-11 @slice-101
  Scenario: itos task reads a task with no checks whose work item is done as done
    Given the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "doing"
    When itos runs the task "T-001"
    Then itos exits with code 0
    And its output lists "T-001" as "done"

  @ID-TASK-12 @slice-101
  Scenario: task --pending leaves out a done task with no checks, and lists one in progress with none as review
    Given the work registry has the item "T-001" with the status "done" and the item "T-002" with the status "doing"
    When itos runs the pending tasks
    Then itos exits with code 0
    And its output lists "T-002" as "review"
    And its output does not say "T-001"
