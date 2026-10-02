@phase-1
Feature: Live scenarios only move, by a rule itos ships
  Outside the commit types that change behaviour (feat and fix here), a commit
  may move scenarios between feature files but not change, add or lose a live
  one: a scenario is the specification, and one changed to fit the code no
  longer checks anything. This repository enforced it with a script of its
  own, scenario-moves.ts, which hardcoded feat and fix and which no consumer
  had. itos ships the rule as a built-in range check of a Gherkin kind,
  { name, builtin: moves, except_types }, so the commit-msg hook judges the
  staged feature files and verify each commit of a range, skipping the types
  except_types names, and as a command, itos tests moves <kind>, which judges
  the staged files by hand. A rename the project allows is listed in the
  check's allowed_renames, by ID and new name. Comment lines are never
  compared, so a scenario's reason may be written beside it in any commit.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the committed feature file "a.feature" with the live scenario "@ID-A-01"
    And the kind's range check is the built-in moves rule, except for "feat, fix"

  @ID-MOVES-01 @slice-23
  Scenario: The commit-msg hook rejects a test commit that changes a live scenario's steps
    Given a change to the steps of the scenario "@ID-A-01" is staged
    When the commit-msg hook checks the message:
      """
      test: tidy the scenario

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "changes the live scenario ID-A-01"

  @ID-MOVES-02 @slice-23
  Scenario: The commit-msg hook leaves a type except_types names to its own rules
    Given a change to the steps of the scenario "@ID-A-01" is staged
    When the commit-msg hook checks the message "feat: change what the scenario checks"
    Then itos exits with code 0

  @ID-MOVES-03 @slice-23
  Scenario: A test commit may move a live scenario to another feature file unchanged
    Given the scenario "@ID-A-01" is staged moved to "b.feature"
    When the commit-msg hook checks the message:
      """
      test: move the scenario

      Task: T-001
      """
    Then itos exits with code 0

  @ID-MOVES-04 @slice-23
  Scenario: A rename allowed_renames lists passes outside feat and fix
    Given the moves rule allows renaming "ID-A-01" to "Opens again"
    And the scenario "@ID-A-01" is staged renamed to "Opens again"
    When the commit-msg hook checks the message:
      """
      test: rename the scenario

      Task: T-001
      """
    Then itos exits with code 0

  # verify judges each commit against its parent, skipping except_types: the
  # Background's feature file was committed as a feat, which adds its live
  # scenario, and the test commit names T-001, so only the moves rule can
  # reject the range.
  @ID-MOVES-05 @slice-23
  Scenario: verify rejects a commit of the range that changed a live scenario outside except_types
    Given the commit "test: tidy the scenario" naming the task "T-001" and changing the steps of the scenario "@ID-A-01" on top of it
    When itos verifies the commits after the first
    Then itos exits with code 1
    And its output says "changes the live scenario ID-A-01"

  @ID-MOVES-06 @slice-23
  Scenario: itos tests moves judges the staged feature files by hand
    Given a change to the steps of the scenario "@ID-A-01" is staged
    When itos checks the staged moves of the kind "scenario"
    Then itos exits with code 1
    And its output says "changes the live scenario ID-A-01"
