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

  # commitlint --edit lints nothing in a blank message file, and git then
  # aborts the commit for its empty message; the built-in lint rejected it
  # with type-empty and subject-empty instead, the one verdict T-063's
  # comparison with commitlint found them giving differently.
  @ID-HEADER-06 @bug-2
  Scenario: The commit-msg hook passes a blank message file, as commitlint does
    When the commit-msg hook checks the message ""
    Then itos exits with code 0

  # Bug 109 (issue #48; the user's go, 2026-10-12). The footer-leading-blank
  # warning named no line, so agents bisected their messages or reworded
  # correct prose to quiet it; it names the line, by its number in the
  # message and its text, and why it reads as a footer. Since bug 110 it
  # fires only on a footer glued to the body, which git does not read, so
  # the chore below has no Task footer either.
  @ID-HEADER-07 @bug-109 @bug-110
  Scenario: The footer-leading-blank warning names the footer glued to the body, and why
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      The paragraphs said the same thing twice.
      Task: T-001
      """
    Then itos exits with code 1
    And its output names the rule "footer-leading-blank"
    And its output says "line 4"
    And its output says "Task: T-001"
    And its output says "a word and a colon"

  # Bug 110: a line beginning with a word and " #" is prose, never a footer.
  @ID-HEADER-08 @bug-109 @bug-110
  Scenario: A body line beginning with a word and a hash draws no footer warning
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      The paragraphs said the same thing twice, so one starts
      with #, which is plain prose.

      Task: T-001
      """
    Then itos exits with code 0
    And its output does not say "footer-leading-blank"

  # Bug 110 (issue #21; the user's call, 2026-10-12): the lint reads the
  # footer as git reads trailers, the last paragraph, so a body line
  # beginning with a word and a colon, mid-paragraph, is prose and draws no
  # warning when the footers stand after a blank line.
  @ID-HEADER-09 @bug-110
  Scenario: A body line beginning with a word and a colon draws no footer warning
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      The paragraphs said the same thing twice, and the wrapped
      scenarios: line below the first is prose.

      Task: T-001
      """
    Then itos exits with code 0
    And its output does not say "footer-leading-blank"
