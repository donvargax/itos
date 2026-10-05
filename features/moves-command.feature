@phase-1
Feature: The built-in moves rule on a kind whose adapter is a command
  The built-in moves rule read feature files, so config check refused it on
  a kind whose adapter is a command, and a consumer whose scenarios are
  vitest tests re-implemented it in a range check of its own (issue #9).
  Slice 82 (the user's calls, 2026-10-05): on a command kind the rule
  compares what the adapter lists at both ends, which needs supports_at.
  Outside the types except_types names, a commit may not add or remove a
  live test, nor switch one between live and wip. The adapter may give
  each test a title, an optional string in the protocol; when it does, a
  live test's title may not change either. Bodies are not compared on a
  command kind: the adapter does not list them, and the consumer's rule
  lets a refactor change a test's body. config check refuses the rule on a
  command kind without supports_at, which could not list the range's start.
  A rename the project allows is still listed in allowed_renames.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the kind's adapter is a command listing the tests of "tests.json", supporting at
    And the committed tests.json lists the live test "@ID-A-01" titled "starts in the clearing"
    And the kind's range check is the built-in moves rule, except for "feat, fix"

  @ID-MOVESCMD-01 @slice-82 @wip
  Scenario: config check accepts the built-in moves rule on a command kind that supports at
    When itos checks the config
    Then itos exits with code 0

  @ID-MOVESCMD-02 @slice-82 @wip
  Scenario: The commit-msg hook refuses a test commit that turns a live test wip
    Given tests.json is staged listing the test "@ID-A-01" titled "starts in the clearing" as wip
    When the commit-msg hook checks the message:
      """
      test: park the clearing test

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "@ID-A-01"

  @ID-MOVESCMD-03 @slice-82 @wip
  Scenario: The commit-msg hook refuses a test commit that retitles a live test
    Given tests.json is staged listing the live test "@ID-A-01" titled "starts in the meadow"
    When the commit-msg hook checks the message:
      """
      test: rename the clearing test

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "@ID-A-01"

  @ID-MOVESCMD-04 @slice-82 @wip
  Scenario: A test commit that adds a wip test and leaves the live ones as they were passes
    Given tests.json is staged listing the live test "@ID-A-01" titled "starts in the clearing" and the wip test "@ID-A-02"
    When the commit-msg hook checks the message:
      """
      test: sketch the next test

      Task: T-001
      """
    Then itos exits with code 0

  @ID-MOVESCMD-05 @slice-82 @wip
  Scenario: config check refuses the built-in moves rule on a command kind that does not support at
    Given the kind's adapter does not support at
    When itos checks the config
    Then itos exits with code 2
    And its output says "supports_at"
