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
  @ID-COMMITCMD-07 @slice-36 @wip
  Scenario: A footer of free text the config declares has a flag of its name
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 --upgrading none -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Upgrading: none"

  @ID-COMMITCMD-08 @slice-36 @wip
  Scenario: A commit missing a footer its type requires is refused before git runs, naming the flag
    Given the config requires an "Upgrading" footer of free text for "chore"
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--upgrading"
    And no commit was made

  @ID-COMMITCMD-09 @slice-36 @wip
  Scenario: A commit missing its task is refused before git runs, naming --task
    When itos commits with the arguments "-m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "--task"
    And no commit was made

  @ID-COMMITCMD-10 @slice-36 @wip
  Scenario: --breaking writes a BREAKING-CHANGE footer git reads as a trailer
    When itos commits with the arguments "--task T-001 --breaking 'the readme moved' -m 'chore!: move the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "BREAKING-CHANGE: the readme moved"

  # Another trailer follows the task line: git already skips a trailer that
  # would land beside an identical one, so only this shape can double it.
  @ID-COMMITCMD-11 @slice-36 @wip
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
  @ID-COMMITCMD-12 @bug-6 @wip
  Scenario: An amend that changes only the message of a feat is judged by the paths of the commit it makes
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the config's feat commits must touch "features/**"
    And itos has committed with the arguments "--scenarios @ID-A-01 -m 'feat: greet'"
    When itos commits with the arguments "--amend --scenarios @ID-A-01 -m 'feat: greet everyone'"
    Then itos exits with code 0
