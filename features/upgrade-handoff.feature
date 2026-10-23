@phase-1
Feature: itos upgrade is handed to the release it moves to, when that release is newer
  itos upgrade is the launcher's own command: it runs as the itos on the
  PATH, which knows only the steps of its own release. A newer release can
  bring steps the running itos does not know, such as rewriting the agent
  rules block in its own words, so when the version upgrade moves to is
  newer than the running itos, the launcher fetches that version, as it
  fetches a pinned one, and runs its upgrade with the version named, the
  same flags passed on, and ITOS_UPGRADE_FROM set to the running itos's
  version. upgrade's exit code is the target's, and the launcher writes
  nothing itself. An itos run with ITOS_UPGRADE_FROM set never hands off
  again, so a release that is not the version it was fetched as cannot loop.

  The itos that does the upgrade rewrites a present agent rules block in its
  own words, as itos init --agent-rules does: only between the markers, only
  where a block is, the stealth copies too, never a file without one, and
  lists each file in what it edited. It commits nothing, as before.

  Background:
    Given a release server offering the versions "9.1.0", "9.2.0" and "9.3.0", each with an upgrading.json naming the one before it
    And a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server

  # Slice 126 (p1-upgrade-refreshes-agent-block, the user's calls of
  # 2026-10-11). The handoff is by version alone: any running itos older than
  # the target hands off, a dev build included, with no opt-in in the
  # release's data. So the feat changes every scenario of
  # upgrade-command.feature, whose running itos (the itos under test, a dev
  # build) is older than its 9.x releases: its Background gains "And the itos
  # that runs is the itos under test, as the version "9.3.0"", so they still
  # judge the walk, and the feat is breaking, naming them and any upgrade
  # corpus case in Changes. Flags (--json) pass through to the target; a
  # target that cannot be fetched exits 75 as @ID-UPGRADECMD-11 says. Not
  # covered by a scenario: the stealth copies (.git/itos/AGENTS.md,
  # AGENTS.override.md), which a unit test of the rewrite judges, since a
  # stealth config beside this Background's committed one is not a layout
  # upgrade meets. A 7.x launcher has no handoff: the v7 to v8 crossing stays
  # slice-120's Upgrading footer and slice-124's status line.
  @ID-UPGRADEHAND-01 @slice-126
  Scenario: A running itos older than the target hands upgrade to it, and exits with its code
    Given the itos that runs is the itos under test, as the version "9.2.0"
    And the version "9.3.0" exits with code 3
    When itos runs "upgrade"
    Then itos exits with code 3
    And the version "9.3.0" ran with the arguments "upgrade 9.3.0"
    And the version "9.3.0" ran with ITOS_UPGRADE_FROM "9.2.0"
    And the config is unchanged

  @ID-UPGRADEHAND-02 @slice-126
  Scenario: An upgrade already handed off is never handed off again
    Given the itos that runs is the itos under test, as the version "9.2.0"
    And ITOS_UPGRADE_FROM is "9.1.0"
    When itos runs "upgrade"
    Then itos exits with code 0
    And no version of the release server ran
    And the config's pin is the version "9.3.0" of the release server, with its checksums

  @ID-UPGRADEHAND-03 @slice-126
  Scenario: The release handed the upgrade rewrites a present block in its own words, the text outside the markers kept
    Given the release "9.3.0" is the itos under test
    And the itos that runs is the itos under test, as the version "9.2.0"
    And the committed file "AGENTS.md" holds the project's line "Be kind to the build." and a block an older itos wrote, saying "Task:" between the markers
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And the file "AGENTS.md" says "Stage only your own files, by explicit path" between the markers
    And the file "AGENTS.md" does not say "itos go" between the markers
    And the file "AGENTS.md" does not say "itos guide" between the markers
    And the file "AGENTS.md" does not say "Task:" between the markers
    And the file "AGENTS.md" has the line "Be kind to the build."
    And nothing was committed

  @ID-UPGRADEHAND-04 @slice-126
  Scenario: An itos already the newest upgrades itself, rewriting the block and listing AGENTS.md among its edits
    Given the itos that runs is the itos under test, as the version "9.3.0"
    And the committed file "AGENTS.md" holds the project's line "Be kind to the build." and a block an older itos wrote, saying "Task:" between the markers
    When itos runs "upgrade"
    Then itos exits with code 0
    And no version of the release server ran
    And the file "AGENTS.md" says "Stage only your own files, by explicit path" between the markers
    And its output says "AGENTS.md"

  @ID-UPGRADEHAND-05 @slice-126
  Scenario: A project with no block keeps its AGENTS.md as it is, and gets no block
    Given the itos that runs is the itos under test, as the version "9.3.0"
    And the committed file "AGENTS.md" holding the lines:
      """
      Be kind to the build.
      """
    When itos runs "upgrade"
    Then itos exits with code 0
    And the file "AGENTS.md" has the same contents as its committed version
