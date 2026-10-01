@phase-1
Feature: The footer rules are itos's, whatever the header lint
  commits.footers says which footer each commit type needs and what its IDs
  must be. itos runs those rules itself, in the commit-msg hook, in commit
  check-message and in verify, beside the header lint. It does this whether
  or not commits.header_lint delegates to a command. Until now a configured
  delegate replaced them. They ran only if the delegate's own config carried
  them (this repository's commitlint plugin), and a delegate without that
  plugin skipped them without a word. The delegate judges the header and
  body, itos judges the footers, and both report before the hook exits.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  # commitlint's conventional config carries no footer rule, so only itos can
  # reject this footer.
  @ID-FOOT-01 @slice-10
  Scenario: With a header lint delegate, a footer naming an unknown task is rejected
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-999
      """
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output names the rule "task-footer"

  @ID-FOOT-02 @slice-10
  Scenario: With a header lint delegate, a commit without the footer its type needs is rejected
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output names the rule "task-footer"

  @ID-FOOT-03 @slice-10
  Scenario: The header lint's problems and the footer rules' are both reported
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message:
      """
      update things

      Task: T-999
      """
    Then itos exits with code 1
    And its output names the rule "type-empty"
    And its output names the rule "task-footer"

  @ID-FOOT-04 @slice-10
  Scenario: verify, with a header lint delegate, rejects a commit without the footer its type needs
    Given the header lint is commitlint's conventional config
    And the commit "chore: tidy the readme" on top of it
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output names the rule "task-footer"

  # The footer rules read the footer's source even for a message that names
  # no ID; with the ledger folder gone that was a raw ENOENT, exit 2.
  @ID-FOOT-05 @slice-13
  Scenario: A message that names no task needs no ledger
    Given the ledger folder is missing
    When the commit-msg hook checks the message "docs: write the readme"
    Then itos exits with code 0

  @ID-FOOT-06 @slice-13
  Scenario: A footer naming a task when the ledger folder is missing is one problem naming the folder
    Given the ledger folder is missing
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"
