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
  missing environment, or 75 when the release server cannot be reached
  (slice 86), and nothing runs in its place.

  Background:
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository whose ledger has the task "T-001"

  @ID-PIN-01 @slice-27
  Scenario: The pinned version is fetched and run with the arguments, and its exit code handed back
    Given the version "9.1.0" exits with code 4
    And the config pins the version "9.1.0" of the release server
    When itos runs "work --as someone"
    Then itos exits with code 4
    And the version "9.1.0" ran with the arguments "work --as someone"

  @ID-PIN-02 @slice-27
  Scenario: A pinned version already in the cache is run without fetching it again
    Given the config pins the version "9.1.0" of the release server
    And itos has already run "version"
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.1.0" ran with the arguments "version"
    And the release server was asked for nothing since the last run

  # The archive is untouched, so only checksums.txt can be what refuses it.
  @ID-PIN-03 @slice-27
  Scenario: A release whose checksums.txt is not the one pinned is refused, and nothing runs
    Given the config pins the version "9.1.0" of the release server
    And the release server's checksums.txt of "9.1.0" is replaced
    When itos runs "version"
    Then itos exits with code 3
    And its output says "checksums.txt"
    And no version of the release server ran

  # checksums.txt still matches the pin, so only its archive's line can be
  # what refuses it.
  @ID-PIN-04 @slice-27
  Scenario: An archive that is not the one its checksums.txt lists is refused, and nothing runs
    Given the config pins the version "9.1.0" of the release server
    And the release server's archive of "9.1.0" for this platform is replaced
    When itos runs "version"
    Then itos exits with code 3
    And no version of the release server ran

  @ID-PIN-05 @slice-27
  Scenario: A pinned version the release server does not have exits 3, naming it
    Given the config pins the version "9.3.0" with the checksums of "9.1.0"
    When itos runs "version"
    Then itos exits with code 3
    And its output says "9.3.0"
    And no version of the release server ran

  @ID-PIN-06 @slice-27
  Scenario: ITOS_VERSION runs the version it names instead of the pin
    Given the config pins the version "9.1.0" of the release server
    And ITOS_VERSION is "9.2.0"
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.2.0" ran with the arguments "version"

  @ID-PIN-07 @slice-27
  Scenario: The version the launcher runs is told it is the one to run, so it does not launch again
    Given the config pins the version "9.1.0" of the release server
    When itos runs "version"
    Then the version "9.1.0" ran with ITOS_VERSION "9.1.0"

  # A repository that installs itos another way, by an install script or a
  # build from source as this one does, keeps the binary it chose.
  @ID-PIN-08 @slice-27
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

  # Slice 40: the launcher finds the config as itos does, so a global itos run
  # from a subfolder of a pinned repository runs the pin, not the newest.
  @ID-PIN-10 @slice-40
  Scenario: From a subfolder, the launcher runs the version the repository pins
    Given the config pins the version "9.1.0" of the release server
    And a "sub" folder
    When itos runs "version" from "sub"
    Then the version "9.1.0" ran with the arguments "version"

  # p3-pin-bump: moving a pin meant downloading the release's checksums.txt
  # and hashing it by hand. itos pin is the launcher's own command, as
  # git-shim install is: it never runs the pinned version, which may predate
  # it, and the pin is what it changes. It writes the two keys in the config
  # it finds (itos.yaml, or the stealth config in the git folder), leaving
  # every other line as it was, and commits nothing: the bump is the
  # project's own build commit, with whatever footer the project asks.
  @ID-PIN-11 @slice-47
  Scenario: itos pin moves the pin to the newest release, its checksums with it, and names the notes to read
    Given the config pins the version "9.1.0" of the release server
    When itos runs "pin"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And its output says "9.1.0"
    And its output says "/tag/v9.2.0"
    And no version of the release server ran

  @ID-PIN-12 @slice-47
  Scenario: itos pin with a version pins that version, in a config that pinned nothing
    When itos runs "pin 9.1.0"
    Then itos exits with code 0
    And the config's pin is the version "9.1.0" of the release server, with its checksums

  @ID-PIN-13 @slice-47
  Scenario: itos pin leaves every other line of the config as it was
    Given the config pins the version "9.1.0" of the release server
    And the config has the comment "# the project's own note"
    When itos runs "pin"
    Then itos exits with code 0
    And the config still has the comment "# the project's own note"
    And itos checks the config
    And itos exits with code 0

  @ID-PIN-14 @slice-47
  Scenario: A version the release server does not have exits 3, naming it, and the config is untouched
    Given the config pins the version "9.1.0" of the release server
    When itos runs "pin 9.3.0"
    Then itos exits with code 3
    And its output says "9.3.0"
    And the config's pin is the version "9.1.0" of the release server, with its checksums

  @ID-PIN-16 @slice-47
  Scenario: A pin already on the version asked for is left as it is
    Given the config pins the version "9.2.0" of the release server
    When itos runs "pin"
    Then itos exits with code 0
    And its output says "already"
    And the config is unchanged

  # Slice 86 (decision 35): a release server that cannot be reached may
  # answer when run again, so pin exits 75. @ID-PIN-17 replaces @ID-PIN-15,
  # which the feat removes, marked breaking.
  @ID-PIN-17 @slice-86
  Scenario: A release server that cannot be reached exits 75, and the config is untouched
    Given the config pins the version "9.1.0" of the release server
    And the release server cannot be reached
    When itos runs "pin"
    Then itos exits with code 75
    And the config's pin is the version "9.1.0" of the release server, with its checksums

  # Slice 87 (the user's calls, 2026-10-06): Go tools and most CLIs answer
  # --version and take latest for the newest release, and itos refused both
  # as usage errors (exit 2). itos --version is itos version: the launcher
  # reads a first argument --version as the command version before anything
  # else, so a project's pinned release is handed "version" and answers even
  # if it predates --version. -v and -V stay unknown. itos pin latest and
  # itos upgrade latest are itos pin and itos upgrade with no version.
  @ID-PIN-18 @slice-87
  Scenario: itos --version prints its version outside a project
    Given the repository has no itos.yaml
    And ITOS_NO_UPDATE is "1"
    When itos runs "--version"
    Then itos exits with code 0
    And its output says "itos "

  @ID-PIN-19 @slice-87
  Scenario: itos --version in a pinned project hands the pinned release the command version
    Given the config pins the version "9.1.0" of the release server
    When itos runs "--version"
    Then the version "9.1.0" ran with the arguments "version"

  @ID-PIN-20 @slice-87
  Scenario: itos pin latest moves the pin to the newest release, as itos pin does
    Given the config pins the version "9.1.0" of the release server
    When itos runs "pin latest"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums

  # Bug 44 (the code review of 07b0e6d, 2026-10-05): the launcher's store
  # removed the release folder before writing it, without first checking
  # whether another run had just cached it, so of two first runs of a newly
  # pinned release (parallel Claude Code tool calls after a pin moves) one
  # could delete the binary the other was about to run, which then failed
  # with exit 3 (on Windows the removal fails on the running exe). A run that
  # finds the release cached by another while it fetched uses that one; two
  # first runs both run the pin. The fake release server holds the first
  # archive download until the other run has cached the release, so the race
  # is forced every time, not left to chance, and the scenario judges that
  # the release cached first is kept, not replaced (the folder itself, not a
  # copy: a run about to exec the binary in it must find it there).
  @ID-PIN-21 @bug-44 @wip
  Scenario: Two first runs of a newly pinned release both run it, neither undoing the other
    Given the config pins the version "9.1.0" of the release server
    And the release server holds the first download of a release until the other run has cached it
    When two runs of itos "version" start at once
    Then both runs exit with code 0
    And the version "9.1.0" ran with the arguments "version" twice
    And the release the other run cached is still the one in the cache
