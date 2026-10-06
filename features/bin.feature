@phase-1
Feature: Every command pattern reads hooks.bin as itos
  The ledger and ci.steps write itos's own commands the way the project calls
  it, hooks.bin (tools/bin/itos by default), not as "itos". The cost rule
  already reads a command whose first word is hooks.bin as if that word were
  itos. The other places the config matches a command read it the same way:
  a ci.covers rule's matches pattern, a ci.nightly_only entry and a
  tests.<kind>.recognize template, each written for itos, match a check
  written with hooks.bin. A command that starts any other way is matched as
  it is written.

  Background:
    Given a repository whose ledger has the task "T-001"
    And "bin/itos" is a script that records it ran
    And hooks.bin is "bin/itos"

  # The step stands for one that has done what the check does; a covered
  # check is not run again, so the recording script stays silent.
  @ID-BIN-01 @slice-20
  Scenario: A ci.covers pattern written for itos covers a check that starts with hooks.bin
    Given the CI steps are "echo every check"
    And ci.covers says the step "echo every check" covers "^itos work check$"
    And the task "T-001" has the check "bin/itos work check"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check did not run
    And its output says "ran above as"

  # Bug 28 changed the line's words from "runs in the nightly": the nightly
  # runs no named task's checks, so it says only that push CI leaves it out.
  @ID-BIN-02 @slice-20
  Scenario: A ci.nightly_only entry written for itos leaves a check that starts with hooks.bin out of push CI
    Given the CI steps are "exit 0"
    And ci.nightly_only is "itos work check"
    And the task "T-001" has the check "bin/itos work check"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check did not run
    And its output says "left out of push CI"

  # The smoke set lists a scenario, so the run of named tests happens and the
  # recognized check is merged into it rather than run.
  @ID-BIN-03 @slice-20
  Scenario: A recognize template written for itos reads a check that starts with hooks.bin as a selection
    Given the CI steps run the named tests of the kind "scenario"
    And the kind "scenario" recognizes "itos tests smoke run scenario" as its smoke run
    And the task "T-001" has the check "bin/itos tests smoke run scenario"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check did not run
    And its output says "(in the scenario run above)"
