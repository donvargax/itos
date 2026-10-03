@phase-3
Feature: itos init, a repository made ready for itos
  Adopting itos meant writing an itos.yaml by hand, a ledger and a registry
  beside it, a pin from a release's checksums.txt, then installing the
  hooks. itos init does it in one command, in the repository it runs in, or
  in a folder that is not one yet, after git init. Where there is no config
  it writes a starter one (the user's call, 2026-10-03): the Conventional
  Commits types, a Task footer required of every type but feat and fix, a
  ledger and a work registry under tasks/, and, when features/ holds feature
  files, a scenario kind whose Scenarios footer feat and fix need, with a
  smoke set naming one live scenario of each file; no path scopes, which are
  each project's own. commits.since names HEAD, so history written before
  itos is never judged. hooks.bin is itos, the global launcher, and the pin
  the newest release, as itos pin writes it; where the release server cannot
  be reached it pins nothing and says how to (a config with no pin runs the
  binary that was called). Then it installs the hooks, as hooks install
  does. With --stealth all of it goes under the git folder and the hooks into
  the git config, nothing the project tracks touched.

  Where a config already is, init writes nothing: it reports what is missing
  (the config's problems, a hook not installed) and exits 1 when anything
  is, 0 when nothing is, so run again it doubles as a check. init is the
  launcher's own command, as itos pin is: where there is no config there is
  no pin to hand the run to. Offering the Claude Code plugin and the git shim,
  and saying a pin has fallen behind, are slices 49 and 50.

  # The repository has a commit and no itos file at all; the scenarios
  # without a release server cannot reach one, so init pins nothing there.
  @ID-INIT-01 @slice-48 @wip
  Scenario: In an existing repository init writes a config the checks accept, starting at HEAD, and installs the hooks
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config's commits.since is HEAD's full SHA
    And the file ".git/hooks/commit-msg" calls itos
    And the file ".git/hooks/pre-push" calls itos
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-02 @slice-48 @wip
  Scenario: The starter config asks a Task footer of a chore, and no footer of a feat without feature files
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"
    When the commit-msg hook checks the message "feat: add a page"
    Then itos exits with code 0

  @ID-INIT-03 @slice-48 @wip
  Scenario: With feature files the starter config adds the scenarios, their footer and a smoke set the smoke rule accepts
    Given a repository that does not use itos, its one commit "docs: start"
    And the feature file "features/pages.feature" with the scenario "@ID-PAGE-01"
    When itos runs "init"
    Then itos exits with code 0
    When itos runs "tests smoke check scenario"
    Then itos exits with code 0
    Given a change to "README.md" is staged
    When the commit-msg hook checks the message "feat: add a page"
    Then itos exits with code 1
    And its output says "Scenarios"

  @ID-INIT-04 @slice-48 @wip
  Scenario: In a folder that is not a repository init runs git init first, and starts nowhere
    Given a folder that is not a git repository
    When itos runs "init"
    Then itos exits with code 0
    And the folder is a git repository
    And the config has no commits.since
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-05 @slice-48 @wip
  Scenario: With a release server init pins its newest release
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And no version of the release server ran

  @ID-INIT-06 @slice-48 @wip
  Scenario: Where the release server cannot be reached init pins nothing and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config has no pin
    And its output says "itos pin"

  @ID-INIT-07 @slice-48 @wip
  Scenario: Run again, init changes nothing and finds nothing missing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    When itos runs "init"
    Then itos exits with code 0
    And no file changed since the last run

  @ID-INIT-08 @slice-48 @wip
  Scenario: Run again where a hook is missing, init reports it and installs nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the file ".git/hooks/pre-push" is removed
    When itos runs "init"
    Then itos exits with code 1
    And its output says "pre-push"
    And its output says "itos hooks install"
    And the file ".git/hooks/pre-push" does not exist

  @ID-INIT-09 @slice-48 @wip
  Scenario: With --stealth init writes everything under the git folder and declares the hooks in the git config
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth"
    Then itos exits with code 0
    And git status shows nothing to commit
    And the git config declares a "commit-msg" hook that runs itos
    When itos checks the config
    Then itos exits with code 0
