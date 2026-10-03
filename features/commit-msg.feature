@phase-1 @slice-2
Feature: The commit-msg hook
  itos hook commit-msg judges a commit before it is made: the paths its type
  may touch, the footers its type needs and the IDs they name, and the header,
  through the header lint the config delegates to. Its exit code is the hook's,
  so a commit it rejects is not made, and its output says why.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  @ID-CMSG-01
  Scenario: A header without a type is rejected, naming the rule it breaks
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message "update things"
    Then itos exits with code 1
    And its output names the rule "type-empty"

  @ID-CMSG-02
  Scenario: A sound message with the footer its type needs is accepted
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0

  @ID-CMSG-03
  Scenario: A footer naming a task the ledger does not have is rejected
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-999
      """
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output names the rule "task-footer"

  @ID-CMSG-04
  Scenario: A commit whose type may not touch a staged path is rejected
    Given a change to "tools/build.sh" is staged
    When the commit-msg hook checks the message "docs: describe the build"
    Then itos exits with code 1
    And its output says "docs commits may not touch tools/build.sh"

  # The hook's help called the header lint's delegate "commitlint here", this
  # repository's until it switched to the built-in lint (T-063). The help is
  # the binary's, read in every project, and which lint a project runs is its
  # config's to say.
  @ID-CMSG-05 @bug-3
  Scenario: The commit-msg hook's help names no project's header lint
    When itos prints the help of "hook commit-msg"
    Then itos exits with code 0
    And its output says "commits.header_lint"
    And its output does not say "commitlint"
