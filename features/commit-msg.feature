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

  # Bug 31 (found by the code review of 07b0e6d, by two reviewers apart).
  # itos read git's lists of paths (diff --name-only, ls-files, ls-tree)
  # without -z or core.quotePath=false, and git C-quotes a path holding a
  # letter outside ASCII, a quote, a backslash or a control character
  # ("src/caf\303\251.js"). A never rule missed such a path and an only rule
  # refused it, and a feature file so named vanished from what the moves
  # rule and the footers read at the index and at a commit. Every list of
  # paths itos reads from git is read NUL-separated (or unquoted), so a path
  # is matched as it is named.
  @ID-CMSG-10 @bug-31
  Scenario: A never rule refuses a path whose name git would quote
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/café.js" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/café.js"

  @ID-CMSG-11 @bug-31
  Scenario: An only rule lets through a path whose name git would quote
    Given a change to "docs/é.md" is staged
    When the commit-msg hook checks the message "docs: say it in French"
    Then itos exits with code 0

  # Bug 32 (found by the code review of 07b0e6d). The commit-msg hook read
  # the staged paths with git's rename detection on, so a rename listed only
  # its new path: git mv of code into docs/ passed a docs commit. Its
  # --diff-filter=ACMRD also left out T, a path whose type changed (a file
  # made a symlink), which verify, reading diff-tree, saw. Every list of
  # changed paths is read with --no-renames and every change type, so a
  # rename is its old path's deletion and its new path's addition. The
  # symlink is staged in the index alone (git update-index), so no
  # filesystem link is needed on Windows.
  @ID-CMSG-12 @bug-32
  Scenario: A docs commit that renames code into docs/ is refused, naming the old path
    Given "src/app.js" is committed
    And "src/app.js" is staged renamed to "docs/app.md"
    When the commit-msg hook checks the message "docs: move the app into the docs"
    Then itos exits with code 1
    And its output says "src/app.js"

  @ID-CMSG-13 @bug-32
  Scenario: A never rule refuses a path whose type changed
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And "src/app.js" is committed
    And "src/app.js" is staged as a symlink
    When the commit-msg hook checks the message:
      """
      chore: link the app

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/app.js"

  # Bug 36 (found by the code review of 07b0e6d). The commit-msg hook read
  # the commit's type from the message file's first line, while the header
  # lint read the header git will store, after its cleanup: a message whose
  # first line was blank or a comment (as an editor leaves it) passed the
  # lint and hid its type from the path rules and the required footers. The
  # hook reads the type from the same cleaned header the lint reads. A
  # docstring may not keep a leading blank line, hence the second scenario's
  # own step.
  @ID-CMSG-14 @bug-36
  Scenario: A message whose first line is a comment is held to its type's path rules
    Given the config's chore commits may never touch "src/**" except "src/themes/**"
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message:
      """
      # Please enter the commit message for your changes.
      chore: tidy the app

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "src/app.js"

  @ID-CMSG-15 @bug-36
  Scenario: A message whose first line is blank is held to its type's footers
    When the commit-msg hook checks a message whose first line is blank, then "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"

  # Bug 139 (issue #133, met on slice-179's PR #130): an implementer typed
  # `itos commit --task none` out of habit, since Upgrading: takes none, and
  # the commit-msg hook said "unknown tasks: none", which reads as if a task
  # named none were missing from the ledger.
  #
  # Where it is said. One rule says it: the ledger footer's check in
  # internal/message (CheckFooter, run by FooterProblems), which every path
  # that judges a message's footers runs: this hook (LintFile), itos commit,
  # which makes no ID check of its own and is refused by the hook git runs
  # (commit-command.feature), itos commit check-message and itos verify (both
  # message.Check). So the fix is in that rule, and every one of the four
  # says the same words; a fix in itos commit's flags alone would leave a
  # typed "Task: none" in the old trap. CI's plan is not one of them: it
  # takes only the footer IDs the ledger's id pattern matches (IDsIn), so
  # none never reaches its unknown tasks.
  #
  # The rule. When an ID the ledger footer names is unknown to the ledger and
  # is the word none, in any case (none, None, NONE: the same habit), the
  # footer's problem is no longer "unknown tasks: none" but
  #   the Task: footer takes a ledger task's id, and has no none
  # with the footer's key as the config names it in place of Task. Where
  # another footer of IDs stands in for the ledger footer in a commit of this
  # type (its in_place_of names the ledger footer for the type, as the
  # Item: footer does for test, docs and chore in the config of the
  # scenarios), the sentence goes on
  #   ; a chore commit names its registry item in Item: instead, with itos commit --item <id>
  # (the type, the stand-in's key and its flag as the config gives them; a
  # stand-in no itos commit flag writes is named without the "with …" part),
  # since the text output prints the sentence and not the fix. A type no
  # stand-in serves gets the first sentence alone.
  #
  # What stays. The rule (task-footer) and the exit code (1, the hook's and
  # git's refusal; 1 for check-message and verify) are today's. A ledger
  # that really has a task whose id is none is not judged: only an unknown
  # ID is. Every other unknown ID is reported in today's words, "unknown
  # tasks: T-999", with no --item hint (ID-CMSG-03, ID-FOOT-01 and
  # ID-COMMITCMD-33 hold that). A list holding none beside other IDs names
  # none once in the sentence above and still checks the others: an unknown
  # one is a second problem of the same rule after it, "unknown tasks:
  # T-999", with its own fix; a known one adds nothing, and the message is
  # still refused for the none. The required footer check is unchanged:
  # "Task: none" gives an ID, so it is never "chore commits need a …".
  #
  # --json: the problem keeps its shape, {"rule":"task-footer","message",
  # "fix"} (with "level":"error" under the built-in header lint or a
  # delegate). Its message is the sentence above; its fix is "name the task
  # the commit belongs to by an id <ledger folder>/ has" (ID-COMMITCMD-34),
  # and, where a stand-in serves, goes on ", or the registry item in Item:
  # with itos commit --item <id>". The fix of an unknown ID that is not none
  # is today's.
  #
  # Before the fix the hook says "unknown tasks: none" under the rule
  # task-footer and exits 1, so this scenario fails at the step that its
  # output says "has no none".
  @ID-CMSG-16 @bug-139 @wip
  Scenario: A ledger footer naming none says the footer takes a task's id, not that a task is unknown
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: none
      """
    Then itos exits with code 1
    And its output says "the Task: footer takes a ledger task's id, and has no none"
    And its output names the rule "task-footer"
    And its output does not say "unknown tasks"
    And its output does not say "--item"

  # none is matched whatever its case. Before the fix the hook says "unknown
  # tasks: None": the scenario fails at the step that its output says "has
  # no none".
  @ID-CMSG-17 @bug-139 @wip
  Scenario: A ledger footer naming None in another case is told the same
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: None
      """
    Then itos exits with code 1
    And its output says "has no none"
    And its output does not say "unknown tasks: None"

  # none beside an unknown task: none is named, and T-999 is still checked
  # and reported in today's words. Before the fix the hook says "unknown
  # tasks: none, T-999": the scenario fails at the step that its output says
  # "has no none".
  @ID-CMSG-18 @bug-139 @wip
  Scenario: A ledger footer naming none beside an unknown task names none and still reports the task
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: none, T-999
      """
    Then itos exits with code 1
    And its output says "has no none"
    And its output says "unknown tasks: T-999"
    And its output does not say "unknown tasks: none"
