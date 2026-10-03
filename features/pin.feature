@phase-3
Feature: A global itos runs the version a repository pins
  itos is installed once per machine rather than in each repository, and the
  installed binary is a launcher that is never rewritten: it picks the
  version to run, fetches that version's release into its cache when the
  cache does not have it, and runs it with the same arguments, handing back
  its exit code. A repository pins one exact version in itos.yaml,
  pin.version, with pin.checksums, the SHA-256 of that release's
  checksums.txt, which covers every platform's archive at once: the launcher
  checks checksums.txt against the pin and the archive against its line, so
  a release replaced after it was pinned never runs. requires stays the
  oldest itos that reads the config.

  ITOS_VERSION names the version to run, whatever the pin says, and the
  launcher sets it for the version it runs, so that version runs itself
  rather than launching again; an extension reads the same variable. A
  config with no pin runs the binary that was called, so a repository that
  installs its itos another way (an install script, a build from source)
  keeps the one it chose. ITOS_RELEASES names where releases come from,
  <base>/download/v<version>/<asset> (by default the GitHub releases of
  donvargax/itos), and ITOS_CACHE the cache (itos/ in the user's cache folder
  by default). A version that cannot be fetched or checked exits 3, a
  missing environment, and nothing runs in its place.

  Background:
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository whose ledger has the task "T-001"

  @ID-PIN-01 @slice-27 @wip
  Scenario: The pinned version is fetched and run with the arguments, and its exit code handed back
    Given the version "9.1.0" exits with code 4
    And the config pins the version "9.1.0" of the release server
    When itos runs "work --as someone"
    Then itos exits with code 4
    And the version "9.1.0" ran with the arguments "work --as someone"

  @ID-PIN-02 @slice-27 @wip
  Scenario: A pinned version already in the cache is run without fetching it again
    Given the config pins the version "9.1.0" of the release server
    And itos has already run "version"
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.1.0" ran with the arguments "version"
    And the release server was asked for nothing since the last run

  # The archive is untouched, so only checksums.txt can be what refuses it.
  @ID-PIN-03 @slice-27 @wip
  Scenario: A release whose checksums.txt is not the one pinned is refused, and nothing runs
    Given the config pins the version "9.1.0" of the release server
    And the release server's checksums.txt of "9.1.0" is replaced
    When itos runs "version"
    Then itos exits with code 3
    And its output says "checksums.txt"
    And no version of the release server ran

  # checksums.txt still matches the pin, so only its archive's line can be
  # what refuses it.
  @ID-PIN-04 @slice-27 @wip
  Scenario: An archive that is not the one its checksums.txt lists is refused, and nothing runs
    Given the config pins the version "9.1.0" of the release server
    And the release server's archive of "9.1.0" for this platform is replaced
    When itos runs "version"
    Then itos exits with code 3
    And no version of the release server ran

  @ID-PIN-05 @slice-27 @wip
  Scenario: A pinned version the release server does not have exits 3, naming it
    Given the config pins the version "9.3.0" with the checksums of "9.1.0"
    When itos runs "version"
    Then itos exits with code 3
    And its output says "9.3.0"
    And no version of the release server ran

  @ID-PIN-06 @slice-27 @wip
  Scenario: ITOS_VERSION runs the version it names instead of the pin
    Given the config pins the version "9.1.0" of the release server
    And ITOS_VERSION is "9.2.0"
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.2.0" ran with the arguments "version"

  @ID-PIN-07 @slice-27 @wip
  Scenario: The version the launcher runs is told it is the one to run, so it does not launch again
    Given the config pins the version "9.1.0" of the release server
    When itos runs "version"
    Then the version "9.1.0" ran with ITOS_VERSION "9.1.0"

  # A repository that installs itos another way, by an install script or a
  # build from source as this one does, keeps the binary it chose.
  @ID-PIN-08 @slice-27 @wip
  Scenario: A config with no pin runs the binary that was called
    When itos runs "version"
    Then itos exits with code 0
    And no version of the release server ran
    And the release server was asked for nothing

  # Issue #2: version failed outside a project, reading the itos.yaml it does
  # not need. A global itos is run outside projects first of all; only
  # version --check reads the config.
  @ID-PIN-09 @bug-4
  Scenario: itos version prints its version outside a project
    Given the repository has no itos.yaml
    And ITOS_NO_UPDATE is "1"
    When itos runs "version"
    Then itos exits with code 0
    And no version of the release server ran
    And its output says "itos "
    And its output does not say "itos.yaml"
