@phase-3
Feature: The built-in header lint
  itos judged a commit's header through a delegate, commitlint with
  config-conventional, which needs Node. The Go binary is meant to need no
  runtime, so itos lints the header itself when commits.header_lint says
  use: builtin: the rules config-conventional holds as errors, with the
  types commits.types lists, reported with commitlint's rule ids and words
  so that a script reading either sees the same; its warnings are printed
  and do not reject. The delegate stays for a project that names one
  (use: command, or hook and stdin as before), and the footer rules are
  itos's whichever lints the header.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged
    And the header lint is itos's built-in one

  @ID-HEADER-01 @slice-25
  Scenario: The built-in lint rejects a header without a type, naming the rule
    When the commit-msg hook checks the message "update things"
    Then itos exits with code 1
    And its output names the rule "type-empty"

  # The type list is commits.types, not config-conventional's own, so a
  # project's types are the ones the lint allows.
  @ID-HEADER-02 @slice-25
  Scenario: The built-in lint rejects a type commits.types does not list
    When the commit-msg hook checks the message:
      """
      feature: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output names the rule "type-enum"

  @ID-HEADER-03 @slice-25
  Scenario: The built-in lint rejects a header longer than 100 characters
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme so that every paragraph says one thing and nothing in it repeats another one again

      Task: T-001
      """
    Then itos exits with code 1
    And its output names the rule "header-max-length"

  # No delegate is configured and none is installed: the scratch repository
  # has no node_modules, so a pass here is the built-in lint's alone.
  @ID-HEADER-04 @slice-25
  Scenario: The built-in lint accepts a sound message with no delegate
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0

  # config-conventional holds a missing blank line before the body as a
  # warning: printed, never a rejection.
  @ID-HEADER-05 @slice-25
  Scenario: A body with no blank line before it is a warning, not a rejection
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme
      the paragraphs said the same thing twice

      Task: T-001
      """
    Then itos exits with code 0
    And its output names the rule "body-leading-blank"
