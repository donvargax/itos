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

  # Slice 63 (the user's calls, 2026-10-04, p1-work-show-spec-commits): an
  # Item footer names the registry item a commit belongs to, for the commits
  # no other footer ties to it: a spec written in a docs commit, a slice's
  # steps committed red first in a test commit, a chore it needs. Its source
  # is the work registry (source: registry), each id checked against the
  # registry the commit carries, with the rule item-footer. in_place_of says
  # which other footer it stands in for, and for which types: the config
  # below takes it in place of Task for test, docs and chore, so a slice,
  # which has no task, can commit its steps before its feat; every other
  # type still needs its Task.
  @ID-FOOT-07 @slice-63
  Scenario: A test commit naming a registry item in an Item footer needs no Task footer
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      test: add slice 9's steps

      Item: slice-9
      """
    Then itos exits with code 0

  @ID-FOOT-08 @slice-63
  Scenario: An Item footer naming an item the registry does not have is rejected
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      test: add slice 7's steps

      Item: slice-7
      """
    Then itos exits with code 1
    And its output says "slice-7"
    And its output names the rule "item-footer"

  @ID-FOOT-09 @slice-63
  Scenario: A type outside in_place_of still needs its Task footer beside an Item footer
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      refactor: split the parser

      Item: slice-9
      """
    Then itos exits with code 1
    And its output names the rule "task-footer"
