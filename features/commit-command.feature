@phase-3
Feature: itos commit, a commit whose footers itos writes
  itos commit runs git commit with the arguments it does not read itself,
  and writes the footers from its own flags: --task <id> for the ledger
  footer (Task: by default, or what the config calls it) and --scenarios
  <ids> for the scenario footer, each as git's own --trailer, so they land
  in the message whether it comes from -m, -F or the editor. The hooks run
  as for any commit and judge the footers the same way, so a footer itos
  writes is held to the rules a typed one is; a refused commit leaves no
  commit. In stealth mode (stealth.feature) the same flags write a note
  instead, so one habit serves both modes.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the commit-msg hook is installed
    And a change to "README.md" is staged

  @ID-COMMITCMD-01 @slice-31
  Scenario: --task writes the ledger footer into the commit's message
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001"

  @ID-COMMITCMD-02 @slice-31
  Scenario: A footer itos writes is judged by the hook like a typed one, and a refused commit leaves none
    When itos commits with the arguments "--task T-999 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "T-999"
    And no commit was made

  @ID-COMMITCMD-03 @slice-31
  Scenario: --scenarios writes the scenario footer
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos commits with the arguments "--scenarios @ID-A-01 -m 'feat: greet'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Scenarios: @ID-A-01"

  @ID-COMMITCMD-04 @slice-31
  Scenario: The arguments itos does not read are git commit's
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme' -m 'Why it changed.'"
    Then itos exits with code 0
    And the message of HEAD says "Why it changed."

  # Slice 31 passed every argument that is not a subcommand to git commit, so
  # a mistyped subcommand became a pathspec: git's confusing error, or, where
  # the word names a file, a commit of that file. A bare first word is
  # itos's: the subcommand, or the usage error it was before slice 31. Paths
  # for git still go after a flag or --.
  @ID-COMMITCMD-05 @bug-5
  Scenario: A first word that is not a subcommand is a usage error, and commits nothing
    Given a change to "check-mesage" is staged
    When itos commits with the arguments "check-mesage -m 'chore: tidy the readme'"
    Then itos exits with code 2
    And its output says "unknown command: commit check-mesage"
    And no commit was made

  @ID-COMMITCMD-06 @bug-5
  Scenario: Paths after -- are git commit's
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme' -- README.md"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001"

  # Slice 36: every footer a commit needs (p3-commit-content-flags). Links
  # (the ledger and scenario footers) and content (a footer of free text,
  # BREAKING-CHANGE) share git's trailer block, and the config's source tells
  # them apart: each footer of free text the config declares gets a flag of
  # its name in lower case, --breaking writes BREAKING-CHANGE:, the form git
  # reads as a trailer ("BREAKING CHANGE:", with its space, makes git see no
  # trailers in the block at all), and a commit missing a footer its type
  # requires is refused before git runs, naming the flag. The hook still
  # judges commits made any other way.
  @ID-COMMITCMD-07 @slice-36
  Scenario: A footer of free text the config declares has a flag of its name
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 --upgrading none -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Upgrading: none"

  @ID-COMMITCMD-08 @slice-36
  Scenario: A commit missing a footer its type requires is refused before git runs, naming the flag
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--upgrading"
    And no commit was made

  @ID-COMMITCMD-09 @slice-36
  Scenario: A commit missing its task is refused before git runs, naming --task
    When itos commits with the arguments "-m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--task"
    And no commit was made

  @ID-COMMITCMD-10 @slice-36
  Scenario: --breaking writes a BREAKING-CHANGE footer git reads as a trailer
    When itos commits with the arguments "--task T-001 --breaking 'the readme moved' -m 'chore!: move the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "BREAKING-CHANGE: the readme moved"

  # Another trailer follows the task line: git already skips a trailer that
  # would land beside an identical one, so only this shape can double it.
  @ID-COMMITCMD-11 @slice-36
  Scenario: An amend through itos commit keeps one footer, not two
    Given itos has committed with the arguments "--task T-001 --trailer 'Reviewed-by: someone' -m 'chore: tidy the readme'"
    When itos commits with the arguments "--amend --no-edit --task T-001"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-001" once

  # Bug 6: the hook judged an amend's staged paths against HEAD, the commit
  # being replaced, rather than against its parent, so an amend changing only
  # the message of a feat was refused for touching nothing (slice 32 and 34
  # met it). An amend is judged as the commit it makes: its parent's tree
  # against the new one. The scratch config gives feat no path rule of its
  # own, so the scenario sets one, or the amend would pass either way.
  @ID-COMMITCMD-12 @bug-6
  Scenario: An amend that changes only the message of a feat is judged by the paths of the commit it makes
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the config's feat commits must touch "features/**"
    And itos has committed with the arguments "--scenarios @ID-A-01 -m 'feat: greet'"
    When itos commits with the arguments "--amend --scenarios @ID-A-01 -m 'feat: greet everyone'"
    Then itos exits with code 0

  # Slice 58 (p1-commit-wrap): a body line over the header lint's
  # body-max-line-length (100 under config-conventional) refused the commit,
  # and every message after one such refusal was checked by hand. itos
  # commit now breaks each body line longer than the limit at word
  # boundaries before git sees the message, from -m and -F alike (the editor's
  # message is left to the hook). The limit is the built-in lint's; a config
  # with no header lint, or one that delegates to a command whose limit itos
  # cannot read, wraps nothing (the coordinator's call, 2026-10-04). A list
  # item's continuation lines are indented to its text, and a line within the
  # limit, an indented line, the header and the footers are left as written.
  # It never joins lines, so a message already wrapped is unchanged. A word
  # longer than the limit stays whole, for the lint to judge.
  @ID-COMMITCMD-13 @slice-58
  Scenario: A body line longer than the lint's limit is wrapped at word boundaries, and the commit is accepted
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line of 150 characters
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message is longer than 100 characters
    And HEAD's message body has the same words as the file's, in order

  @ID-COMMITCMD-14 @slice-58
  Scenario: A list item longer than the limit wraps under its own text
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body list item of 150 characters
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message is longer than 100 characters
    And every line of HEAD's message body after the item's first starts with two spaces

  # Passes before the slice; it guards the wrapping against joining lines.
  @ID-COMMITCMD-15 @slice-58
  Scenario: Body lines within the limit are left as written, never joined
    Given the message file "msg.txt" with the header "chore: tidy the readme" and the body lines "Line one." and "Line two."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD has the line "Line one."
    And the message of HEAD has the line "Line two."

  # Bug 15, found by slice 58's post-landing review: the wrap broke a body
  # line at any space, so a continuation line could begin with a footer
  # token or a breaking-change note (the lint, the footer rules and a
  # release then read the rest of the body as footers, and an ID footer's
  # words as IDs), or with git's comment char (git's strip cleanup then
  # drops the line). A break that would start a line so moves back a word,
  # or the line runs over the limit when no earlier break is left.
  @ID-COMMITCMD-16 @bug-15
  Scenario: A wrap never starts a line with a breaking-change note
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line whose wrap would start a line with "BREAKING CHANGE: the keys stay."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 -F msg.txt"
    Then itos exits with code 0
    And no line of HEAD's message starts with "BREAKING CHANGE:"

  @ID-COMMITCMD-17 @bug-15
  Scenario: A wrap never starts a line with git's comment char, so strip cleanup keeps every word
    Given the message file "msg.txt" with the header "chore: tidy the readme" and a body line whose wrap would start a line with "#123 for the rest."
    And the header lint is itos's built-in one
    When itos commits with the arguments "--task T-001 --cleanup=strip -F msg.txt"
    Then itos exits with code 0
    And the message of HEAD says "#123 for the rest."

  # Slice 63: --item writes the Item footer, as --task writes the Task one.
  @ID-COMMITCMD-18 @slice-63
  Scenario: --item writes the Item footer into the commit's message
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When itos commits with the arguments "--item slice-9 -m 'test: add slice 9 steps'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Item: slice-9"

  # Slice 81 (issue #10; the user's call, 2026-10-05). A scope could say
  # only, never and must_touch, so "nothing under src/ but a game's themes"
  # could not be written, and a consumer enforced it with a range check of
  # its own, which itos commit check-paths does not run: planning a split
  # with it approved paths the hook then refused. A scope's except list,
  # beside never, takes paths back out of it: a path matching never is
  # refused unless it matches except. except applies to never alone, and
  # config check refuses an except in a scope that has no never. The hook,
  # verify and check-paths read the same scopes.
  @ID-COMMITCMD-19 @slice-81 @wip
  Scenario: check-paths lets a path through that a scope's except takes out of its never
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    When itos runs "commit check-paths --type chore src/games/cielo/themes/colours.ts"
    Then itos exits with code 0

  @ID-COMMITCMD-20 @slice-81 @wip
  Scenario: check-paths refuses a path under never that except does not name
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    When itos runs "commit check-paths --type chore src/games/cielo/sim/nudo.ts"
    Then itos exits with code 1
    And its output says "src/games/cielo/sim/nudo.ts"

  @ID-COMMITCMD-21 @slice-81 @wip
  Scenario: The commit-msg hook takes a scope's except as check-paths does
    Given the config's chore commits may never touch "src/**" except "src/games/*/themes/**"
    And a change to "src/games/cielo/themes/colours.ts" is staged
    When the commit-msg hook checks the message:
      """
      chore: tune the colours

      Task: T-001
      """
    Then itos exits with code 0
