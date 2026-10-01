@phase-1
Feature: A missing ledger folder is one problem in every command that reads the ledger
  The ledger is the folder of task files ledger.files names, and a project
  can configure one before it makes the folder. The footer rules report a
  missing folder as one problem naming it, exit 2, as for any file the config
  names that cannot be read. Every other command that reads the ledger
  stopped instead on a raw "ENOENT … scandir 'tasks'", and config check
  --json then printed no JSON. Each reports the folder the same way: one
  problem naming it, exit 2, and under --json the config error's report.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the ledger folder is missing

  @ID-LEDGER-01 @slice-14 @wip
  Scenario: config check names the missing ledger folder
    When itos checks the config
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"

  @ID-LEDGER-02 @slice-14 @wip
  Scenario: config check --json reports the missing ledger folder as JSON
    When itos checks the config as JSON
    Then itos exits with code 2
    And its output is a JSON report that is not valid
    And its output does not say "ENOENT"

  @ID-LEDGER-03 @slice-14 @wip
  Scenario: Running a task names the missing ledger folder
    When itos runs the task "T-001"
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"

  @ID-LEDGER-04 @slice-14 @wip
  Scenario: task list names the missing ledger folder
    When itos lists the tasks
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"

  # The commit names a task, so the plan has a task to read from the ledger.
  @ID-LEDGER-05 @slice-14 @wip
  Scenario: ci plan names the missing ledger folder
    Given the CI steps are "exit 0"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos plans CI over the commits after the first
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"

  # No step runs: the plan stops before the run.
  @ID-LEDGER-06 @slice-14 @wip
  Scenario: ci run names the missing ledger folder, and runs no step
    Given the CI steps are "exit 0" then a step that records it ran
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"
    And the recording step did not run
