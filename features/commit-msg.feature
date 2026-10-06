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

  # Bug 30 (found by the code review of 07b0e6d; the user's calls,
  # 2026-10-05). itos read a commit's type as the first word of its header,
  # and the header lint, as commitlint does, ignores the headers git writes
  # itself: Revert "…", Merge …, fixup!, squash!, amend!, Reapply. Such a
  # header gave a type outside commits.types, so no path scope, no required
  # footer and no moves rule applied, in the hook and in verify, and verify's
  # --no-merges left a merge's own changes unjudged: any change could land
  # with no footer. Now Revert "…" (and Reapply "…") is judged as revert;
  # fixup!, squash! and amend! as the type of the header they name, after the
  # last such prefix; and a merge commit by its own changes, the ones no
  # parent brings: a merge with none passes whatever its header, as GitHub's
  # "Merge pull request" commits do, while one with changes of its own is
  # judged by its first line's type, and refused when that has none. Any
  # other header with no type in commits.types stays refused by the lint.
  @ID-CMSG-06 @bug-30
  Scenario: A Revert "…" header is judged as a revert, which needs its footer
    When the commit-msg hook checks the message "Revert \"chore: tidy the readme\""
    Then itos exits with code 1
    And its output says "Task"

  @ID-CMSG-07 @bug-30
  Scenario: A fixup! header is judged by the type of the header it names
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message:
      """
      fixup! chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "chore commits may not touch src/app.js"

  @ID-CMSG-08 @bug-30
  Scenario: verify judges a merge commit's own changes by its first line's type
    Given a merge commit "Merge branch 'topic'" on top of it, with a change of its own to "src/app.js"
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "Merge branch 'topic'"

  @ID-CMSG-09 @bug-30
  Scenario: verify passes a merge commit with no changes of its own, whatever its header
    Given a merge commit "Merge branch 'topic'" on top of it, with no change of its own
    When itos verifies every commit up to HEAD
    Then itos exits with code 0
