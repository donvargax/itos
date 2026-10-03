@phase-1
Feature: itos tests smoke run, the smoke set run through the kind's runner
  itos tests smoke run <kind> runs exactly the smoke set: the kind's run.select
  with the smoke IDs as its pattern, and its exit code handed back. Arguments
  for the runner follow the kind's name and are added to the command as
  given, so a person or a ledger check can pass the runner a flag of its own
  (--workers=1 for Playwright); a -- before them is still taken, to pass
  something that looks like itos's own flag, and is not passed on.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a feature file "a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And "record-run" is a script that records its arguments
    And the kind "scenario" runs a selection as "./record-run {pattern}"

  # Bug 8, issue #3 (character-editor): the arguments after the kind were read
  # only from after a --, so --workers=1 was dropped without a word and the run
  # looked as if it had taken the flag. The reporter's first choice: pass them.
  @ID-SMOKE-01 @bug-8
  Scenario: Runner arguments after the kind reach the runner without a --
    When itos runs "tests smoke run scenario --workers=1"
    Then itos exits with code 0
    And "record-run" was given "--workers=1"

  @ID-SMOKE-02 @bug-8
  Scenario: A -- before the runner arguments is still taken, and not passed on
    When itos runs "tests smoke run scenario -- --workers=1"
    Then itos exits with code 0
    And "record-run" was given "--workers=1"
    And "record-run" was not given "--"
