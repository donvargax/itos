@phase-3
Feature: A footer that says what a consumer changes
  A release's Upgrading section was written by reading the commits after
  the fact, and nothing could tell when it missed one: the notes check sees
  changed defaults, not a behaviour change. A footer whose source is free
  text records it where the change is made: commits.footers.<name> with
  source: text, required for the types the config names, carrying either
  what a consumer must do or the word none. It is required only of commits
  after the footer's since, so a history written before the rule still
  verifies; the release then gathers the range's footers into its notes.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged
    And the config requires an "Upgrading" footer of free text for "chore"

  @ID-UPGRADE-01 @slice-26 @wip
  Scenario: A commit without the free-text footer its type needs is rejected, naming the footer
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "Upgrading"

  @ID-UPGRADE-02 @slice-26 @wip
  Scenario: The word none is a free-text footer that says a consumer changes nothing
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      Upgrading: none
      """
    Then itos exits with code 0

  @ID-UPGRADE-03 @slice-26 @wip
  Scenario: A free-text footer is accepted with whatever it says
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      Upgrading: rename commits.lint to commits.header_lint
      """
    Then itos exits with code 0

  # Without a since of its own, adding a required footer would turn verify
  # red on every commit written before the rule existed.
  @ID-UPGRADE-04 @slice-26 @wip
  Scenario: verify does not require the footer of a commit before the footer's since
    Given the commit "chore: tidy the readme" naming the task "T-001"
    And the "Upgrading" footer is required only after HEAD
    When itos verifies every commit up to HEAD
    Then itos exits with code 0

  # The release gathers what each commit asked of a consumer: the footers of
  # the range, one per line with the commit it came from; a none asks nothing.
  @ID-UPGRADE-05 @slice-26 @wip
  Scenario: itos lists a range's free-text footers with their commits, skipping those that say none
    Given the commit "chore: rename the key" naming the task "T-001" with the footer "Upgrading: rename commits.lint to commits.header_lint"
    And the commit "chore: tidy the readme" naming the task "T-001" with the footer "Upgrading: none"
    When itos lists the "Upgrading" footers of the commits after the first
    Then itos exits with code 0
    And its output says "rename commits.lint to commits.header_lint"
    And its output does not say "none"
