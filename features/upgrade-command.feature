@phase-1
Feature: itos upgrade moves a project to a newer itos and walks it through what each release asks
  A project moves to a newer itos by moving its pin (itos pin) and doing
  what each release's Upgrading section asks since the version it left.
  itos upgrade [<version>] does both: it moves the pin to <version>, or with
  none to the newest, as itos pin does, and prints, oldest release first,
  what every release after the old version up to the new one asks: its
  breaking changes, its Upgrading footers, the old scenarios and corpus
  cases it changes on purpose (Changes) and the config keys it adds, removes
  or gives another default. It reads them from each release's upgrading.json
  (T-091), whose previous names the release before it, so it walks back from
  the new release to the old one without listing releases through an API.

  It applies only the text a release fixes exactly (the user's calls,
  2026-10-04): the pin, the config's yaml-language-server schema line when
  it names a release's itos.schema.json, and the install script,
  tools/bin/install-itos (v2.0.0's Upgrading), its version= line and each
  platform's sum= hash, from the new release's checksums.txt. What the
  commits ask and the config's key changes are listed for the person to do,
  never edited. It fetches everything before it writes anything, so a fetch
  that fails leaves every file as it was, and it commits nothing: the move is
  the project's own commit, as itos pin's is. Like itos pin, it is the
  launcher's own command and never runs the pinned version, which may
  predate it, and it edits the config itos finds, the stealth one included.

  The version it moves from is the pin's, or with no pin the install
  script's; with neither there is nothing to move. Moving to an older
  version is itos pin's, not an upgrade.

  --json: {"schema":1,"action":"upgraded"|"already"|"refused","from","to",
  "releases":[{"version","breaking","upgrading","changes","config"} or
  {"version","notes"} for a release with no upgrading.json],"edited":[paths]}

  Background:
    Given a release server offering the versions "9.1.0", "9.2.0" and "9.3.0", each with an upgrading.json naming the one before it
    And a repository whose ledger has the task "T-001"

  @ID-UPGRADECMD-01 @slice-75 @wip
  Scenario: itos upgrade moves the pin to the newest release and lists what each release since the old pin asks, oldest first
    Given the config pins the version "9.1.0" of the release server
    And the release "9.2.0" asks "Move the records to docs/decisions."
    And the release "9.3.0" asks "Set work.decisions to the records' folder."
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its output says "Move the records to docs/decisions." before "Set work.decisions to the records' folder."
    And nothing was committed
    And no version of the release server ran

  @ID-UPGRADECMD-02 @slice-75 @wip
  Scenario: itos upgrade with a version stops there, and lists nothing a later release asks
    Given the config pins the version "9.1.0" of the release server
    And the release "9.2.0" asks "Move the records to docs/decisions."
    And the release "9.3.0" asks "Set work.decisions to the records' folder."
    When itos runs "upgrade 9.2.0"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And its output says "Move the records to docs/decisions."
    And its output does not say "Set work.decisions to the records' folder."

  @ID-UPGRADECMD-03 @slice-75 @wip
  Scenario: A release's breaking changes, the cases it changes on purpose and its config keys are listed too
    Given the config pins the version "9.2.0" of the release server
    And the release "9.3.0" breaks with "ask record writes MADR 4 records."
    And the release "9.3.0" changes "@ID-ASK-12" on purpose
    And the release "9.3.0" removes the config key "work.adr"
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "ask record writes MADR 4 records."
    And its output says "@ID-ASK-12"
    And its output says "work.adr"

  # The line the release notes ask a project to keep at the config's top: only
  # the version in .../download/v<version>/itos.schema.json changes, whatever
  # the address before it. A schema line naming anything else is left alone.
  @ID-UPGRADECMD-04 @slice-75 @wip
  Scenario: The config's schema line moves to the new release's schema
    Given the config pins the version "9.1.0" of the release server
    And the config's first line is the schema line of the version "9.1.0" of the release server
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's first line is now the schema line of the version "9.3.0" of the release server

  # A project that installs itos with v2.0.0's install script and pins
  # nothing: the script's version is the one it moves from, and no pin is
  # added. A script itos cannot edit so (no version= line, a platform whose
  # archive the new checksums.txt lacks) exits 2, every file untouched.
  @ID-UPGRADECMD-05 @slice-75 @wip
  Scenario: The install script moves to the new release's version and hashes, and no pin is added
    Given the repository has the install script of the version "9.1.0" of the release server
    When itos runs "upgrade"
    Then itos exits with code 0
    And the install script is the one of the version "9.3.0" of the release server
    And the config has no pin

  # Releases cut before T-091 publish no upgrading.json: the walk stops there
  # and names that release's notes, which the person reads back to the old
  # version by hand.
  @ID-UPGRADECMD-06 @slice-75 @wip
  Scenario: A release with no upgrading.json is named by its notes' address, and the pin still moves
    Given the config pins the version "9.1.0" of the release server
    And the release "9.2.0" has no upgrading.json
    And the release "9.3.0" asks "Set work.decisions to the records' folder."
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its output says "Set work.decisions to the records' folder."
    And its output says "/tag/v9.2.0"

  @ID-UPGRADECMD-07 @slice-75 @wip
  Scenario: A release server that cannot be reached exits 3, and no file is changed
    Given the config pins the version "9.1.0" of the release server
    And the repository has the install script of the version "9.1.0" of the release server
    And the release server cannot be reached
    When itos runs "upgrade"
    Then itos exits with code 3
    And the config is unchanged
    And the install script is the one of the version "9.1.0" of the release server

  @ID-UPGRADECMD-08 @slice-75 @wip
  Scenario: A project already on the version asked for is left as it is
    Given the config pins the version "9.3.0" of the release server
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "already"
    And the config is unchanged

  @ID-UPGRADECMD-09 @slice-75 @wip
  Scenario: An older version than the pin is refused, naming itos pin
    Given the config pins the version "9.2.0" of the release server
    When itos runs "upgrade 9.1.0"
    Then itos exits with code 1
    And its output says "itos pin"
    And the config is unchanged

  @ID-UPGRADECMD-10 @slice-75 @wip
  Scenario: A project with neither a pin nor an install script is refused, naming itos pin
    When itos runs "upgrade"
    Then itos exits with code 1
    And its output says "itos pin"
    And the config has no pin
