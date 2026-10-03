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
