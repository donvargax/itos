@phase-1
Feature: ci run's log names a merged check by the kind of named tests it ran in
  A push's CI merges every task check that runs named tests into one run of
  their kind (the `tests:` step), and its log lists each merged check under
  its task. That line said "(in the E2E run above)", the template's word for
  its Playwright suite, whatever the kind was called. itos knows the kind by
  the name tests.<kind> gives it, so the line names that kind: the log tells
  an agent which run to read for the check's result.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-CI-01 @slice-11
  Scenario: A task's check that runs named tests is logged as part of its kind's run
    Given the CI steps run the named tests of the kind "scenario"
    And the task "T-001" has a check that runs the scenario "@ID-A-01"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "(in the scenario run above)"
    And its output does not say "E2E"

  # An ids selection renders through ids_pattern, so an empty one rendered
  # "@(?:)\b", which matches every tag: the run meant for one scenario ran
  # them all.
  @ID-CI-02 @slice-12
  Scenario: An empty smoke set adds no pattern to the run of named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And the task "T-001" has a check that runs the scenario "@ID-A-01"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "run @ID-A-01"
    And its output does not say "@(?:)"

  @ID-CI-03 @slice-12
  Scenario: A range that names no test, with an empty smoke set, runs no named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output does not say "printf"
    And its output does not say "run every scenario"
