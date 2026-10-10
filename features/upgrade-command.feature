@phase-1
Feature: itos upgrade moves a project to a newer itos and walks it through what each release asks
  A project moves to a newer itos by moving its pin (itos pin) and doing
  what each release's Upgrading section asks since the version it left.
  itos upgrade [<version>] does both: it moves the pin to <version>, or with
  none to the newest, as itos pin does, and walks, oldest release first,
  every release after the old version up to the new one: its breaking
  changes, its Upgrading footers, the old scenarios and corpus cases it
  changes on purpose (Changes) and the config keys it adds, removes or gives
  another default. It reads them from each release's upgrading.json (T-091),
  whose previous names the release before it, so it walks back from the new
  release to the old one without listing releases through an API.

  It applies only what a release fixes exactly (the user's calls, 2026-10-04
  and 2026-10-06): the pin, the config's yaml-language-server schema line
  when it names a release's itos.schema.json, the install script,
  tools/bin/install-itos (v2.0.0's Upgrading), its version= line and each
  platform's sum= hash, from the new release's checksums.txt, and the config
  keys each release's published itos.schema.json declares renamed or
  removed, every comment and other line of the config kept (slice 113).
  Nothing a commit's text says is edited: the breaking changes and the
  Upgrading footers are what is left by hand, which it prints last, under
  By hand. It fetches everything before it writes anything, so a fetch that
  fails leaves every file as it was, and it commits nothing: the move is the
  project's own commit, as itos pin's is. Like itos pin, it is the
  launcher's own command and never runs the pinned version, which may
  predate it, and it edits the config itos finds, the stealth one included.

  The version it moves from is the pin's, or with no pin the install
  script's; with neither there is nothing to move. Moving to an older
  version is itos pin's, not an upgrade.

  --json: {"schema":1,"action":"upgraded"|"already"|"refused","from","to",
  "releases":[{"version","breaking","upgrading","changes","config"} or
  {"version","notes"} for a release with no upgrading.json],"edited":[paths],
  "applied":[{"version","kind":"renamed"|"removed","key","from"?,"value"?}],
  "manual":[{"version","text"}]}

  Background:
    Given a release server offering the versions "9.1.0", "9.2.0" and "9.3.0", each with an upgrading.json naming the one before it
    And a repository whose ledger has the task "T-001"

  @ID-UPGRADECMD-01 @slice-75
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

  @ID-UPGRADECMD-02 @slice-75
  Scenario: itos upgrade with a version stops there, and lists nothing a later release asks
    Given the config pins the version "9.1.0" of the release server
    And the release "9.2.0" asks "Move the records to docs/decisions."
    And the release "9.3.0" asks "Set work.decisions to the records' folder."
    When itos runs "upgrade 9.2.0"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And its output says "Move the records to docs/decisions."
    And its output does not say "Set work.decisions to the records' folder."

  @ID-UPGRADECMD-03 @slice-75
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
  @ID-UPGRADECMD-04 @slice-75
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
  @ID-UPGRADECMD-05 @slice-75
  Scenario: The install script moves to the new release's version and hashes, and no pin is added
    Given the repository has the install script of the version "9.1.0" of the release server
    When itos runs "upgrade"
    Then itos exits with code 0
    And the install script is the one of the version "9.3.0" of the release server
    And the config has no pin

  # Releases cut before T-091 publish no upgrading.json: the walk stops there
  # and names that release's notes, which the person reads back to the old
  # version by hand.
  @ID-UPGRADECMD-06 @slice-75
  Scenario: A release with no upgrading.json is named by its notes' address, and the pin still moves
    Given the config pins the version "9.1.0" of the release server
    And the release "9.2.0" has no upgrading.json
    And the release "9.3.0" asks "Set work.decisions to the records' folder."
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its output says "Set work.decisions to the records' folder."
    And its output says "/tag/v9.2.0"

  @ID-UPGRADECMD-08 @slice-75
  Scenario: A project already on the version asked for is left as it is
    Given the config pins the version "9.3.0" of the release server
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "already"
    And the config is unchanged

  @ID-UPGRADECMD-09 @slice-75
  Scenario: An older version than the pin is refused, naming itos pin
    Given the config pins the version "9.2.0" of the release server
    When itos runs "upgrade 9.1.0"
    Then itos exits with code 1
    And its output says "itos pin"
    And the config is unchanged

  @ID-UPGRADECMD-10 @slice-75
  Scenario: A project with neither a pin nor an install script is refused, naming itos pin
    When itos runs "upgrade"
    Then itos exits with code 1
    And its output says "itos pin"
    And the config has no pin

  # Slice 86 (decision 35): as pin's. @ID-UPGRADECMD-11 replaces
  # @ID-UPGRADECMD-07, which the feat removes, marked breaking.
  @ID-UPGRADECMD-11 @slice-86
  Scenario: A release server that cannot be reached exits 75, and no file is changed
    Given the config pins the version "9.1.0" of the release server
    And the repository has the install script of the version "9.1.0" of the release server
    And the release server cannot be reached
    When itos runs "upgrade"
    Then itos exits with code 75
    And the config is unchanged
    And the install script is the one of the version "9.1.0" of the release server

  # Slice 87 (the user's call, 2026-10-06): as itos pin latest.
  @ID-UPGRADECMD-12 @slice-87
  Scenario: itos upgrade latest moves the pin to the newest release, as itos upgrade does
    Given the config pins the version "9.1.0" of the release server
    When itos runs "upgrade latest"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums

  # Slice 113 (p1-upgrade-applies-config-steps, the user's calls of 2026-10-06,
  # resumed 2026-10-10). upgrade applies the config changes each crossed
  # release's published itos.schema.json declares, never anything read from
  # commit text: a key whose schema property declares x-itos-renamed-from
  # (<old path>) takes the old key's value and the old key goes; a key the
  # schema's top-level x-itos-removed lists is deleted, and upgrade prints the
  # value it held. It walks oldest release first, so a key renamed twice ends
  # under its newest name. It prints each edit as it makes it, release by
  # release, then ends with "By hand:" and every crossed release's Upgrading
  # and breaking text, or "By hand: nothing". The Upgrading: and
  # BREAKING-CHANGE: footers now mean only what is left by hand. A release
  # that publishes no schema, or one with no declarations, is walked as
  # today. The config editor learns to delete and rename a key; a layout it
  # cannot edit byte for byte (a flow mapping, an anchor) exits 2 with every
  # file untouched, the pin included, since it fetches and plans every edit
  # before it writes. --json adds "applied":[{"version","kind":"renamed"|
  # "removed","key","from"?,"value"?}] and "manual":[{"version","text"}].
  # Out of this slice: schema-contract refusing a key dropped without a
  # declaration (a ci task), the release notes split by hand and automatic,
  # and data steps for itos's committed files, such as backfilling
  # p1-work-item-dates (their own items).
  @ID-UPGRADECMD-13 @slice-113
  Scenario: A key the new release's schema declares renamed takes the old key's value
    Given the config pins the version "9.1.0" of the release server
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's "ci.watch.wait" is "600"
    And the config has no value at "ci.watch.timeout"
    And its output says "ci.watch.wait"
    And nothing was committed

  @ID-UPGRADECMD-14 @slice-113
  Scenario: A key the new release's schema declares removed is deleted, and upgrade prints the value it held
    Given the config pins the version "9.1.0" of the release server
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.3.0"'s schema declares "ci.watch.timeout" removed
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config has no value at "ci.watch.timeout"
    And its output says "ci.watch.timeout"
    And its output says "600"

  @ID-UPGRADECMD-15 @slice-113
  Scenario: A key renamed by two releases ends under the newer name, the releases applied oldest first
    Given the config pins the version "9.1.0" of the release server
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.2.0"'s schema declares "ci.watch.limit" renamed from "ci.watch.timeout"
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.limit"
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's "ci.watch.wait" is "600"
    And the config has no value at "ci.watch.timeout"
    And the config has no value at "ci.watch.limit"

  @ID-UPGRADECMD-16 @slice-113
  Scenario: upgrade prints its edits first, then By hand and what each crossed release still asks
    Given the config pins the version "9.1.0" of the release server
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.2.0" asks "Move the records to docs/decisions."
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "ci.watch.wait" before "By hand:"
    And its output says "By hand:" before "Move the records to docs/decisions."

  @ID-UPGRADECMD-17 @slice-113
  Scenario: When every change was applied, upgrade says nothing is left by hand
    Given the config pins the version "9.1.0" of the release server
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "By hand: nothing"

  @ID-UPGRADECMD-18 @slice-113
  Scenario: A key in a layout the editor cannot change byte for byte is refused with exit 2, and no file is changed
    Given the config pins the version "9.1.0" of the release server
    And the config holds "ci.watch.timeout" as "600" in a flow mapping
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 2
    And its output says "ci.watch.timeout"
    And the config is unchanged

  @ID-UPGRADECMD-19 @slice-113
  Scenario: A rename keeps the config's comments and every other line as they were
    Given the config pins the version "9.1.0" of the release server
    And the config has the comment "# Wait ten minutes for CI."
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config still has the comment "# Wait ten minutes for CI."
