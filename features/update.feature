@phase-3
Feature: A global itos keeps to the newest release, and says when a pin falls behind
  The launcher (pin.feature) asks the release server for its newest version
  at most once an hour, reading <base>/latest/download/checksums.txt, whose
  archive names carry the version, and fetches that release into its cache.
  Where nothing is pinned and there is no itos.yaml (outside a project, a
  repository that does not use itos) it runs the newest release it has; it
  never rewrites the binary that was called. A config with no pin still runs
  the binary that was called. It asks only where the answer is used: where it
  runs the newest release, and where a pin may have fallen behind.

  Where a repository pins an older version than the newest, the launcher runs
  the pin and says so on stderr, at most once a day per repository:
  "itos 9.2.0 is out (this repository pins 9.1.0): <base>/tag/v9.2.0", the
  release's notes. Nothing more: no notice in --json's object, none aimed at
  agents, no config key to silence a team. CI (the CI variable set) never
  asks and never says; ITOS_NO_UPDATE=1 stops the asking and
  ITOS_NO_UPDATE_NOTICE=1 the saying. A release server that cannot be reached
  is not an error: the run goes on with what the cache has.

  Background:
    Given a release server offering the versions "9.1.0" and "9.2.0"

  @ID-UPDATE-01 @slice-28
  Scenario: Outside a project the newest release is fetched and run
    Given the repository has no itos.yaml
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.2.0" ran with the arguments "version"

  @ID-UPDATE-02 @slice-28
  Scenario: The release server is asked for its newest version at most once an hour
    Given the repository has no itos.yaml
    And itos has already run "version"
    When itos runs "version"
    Then the version "9.2.0" ran with the arguments "version"
    And the release server was asked for nothing since the last run

  @ID-UPDATE-03 @slice-28
  Scenario: CI never asks for the newest version
    Given the repository has no itos.yaml
    And CI is "true"
    When itos runs "version"
    Then itos exits with code 0
    And the release server was asked for nothing
    And no version of the release server ran

  @ID-UPDATE-04 @slice-28
  Scenario: ITOS_NO_UPDATE stops the asking
    Given the repository has no itos.yaml
    And ITOS_NO_UPDATE is "1"
    When itos runs "version"
    Then itos exits with code 0
    And the release server was asked for nothing
    And no version of the release server ran

  # The newest release is in the cache from the run outside the project, so
  # only the rule can be what keeps the called binary.
  @ID-UPDATE-05 @slice-28
  Scenario: A config with no pin runs the binary that was called, though a newer release is in the cache
    Given the repository has no itos.yaml
    And itos has already run "version"
    And a repository whose ledger has the task "T-001"
    When itos runs "version"
    Then itos exits with code 0
    And no version of the release server ran since the last run

  @ID-UPDATE-06 @slice-28
  Scenario: A repository pinning an older version runs its pin, and says the newer one is out
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.1.0" ran with the arguments "version"
    And its output says "itos 9.2.0 is out (this repository pins 9.1.0): "
    And its output says "/tag/v9.2.0"

  @ID-UPDATE-07 @slice-28
  Scenario: The notice is said at most once a day in a repository
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And itos has already run "version"
    When itos runs "version"
    Then the version "9.1.0" ran with the arguments "version"
    And its output does not say "is out"

  @ID-UPDATE-08 @slice-28
  Scenario: CI never says the newer version is out
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And CI is "true"
    When itos runs "version"
    Then the version "9.1.0" ran with the arguments "version"
    And its output does not say "is out"

  @ID-UPDATE-09 @slice-28
  Scenario: ITOS_NO_UPDATE_NOTICE silences the notice
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And ITOS_NO_UPDATE_NOTICE is "1"
    When itos runs "version"
    Then the version "9.1.0" ran with the arguments "version"
    And its output does not say "is out"

  @ID-UPDATE-10 @slice-28
  Scenario: A release server that cannot be reached is not an error
    Given the repository has no itos.yaml
    And the release server cannot be reached
    When itos runs "version"
    Then itos exits with code 0
    And no version of the release server ran

  # Slice 87 (the user's call, 2026-10-06): in a pinned project itos version
  # printed the pin's version alone, so an agent told of a new release saw
  # the old number with nothing saying it was the pin's: the newer-release
  # notice is said once a day, from a newest asked once a day, and those
  # throttles stay. The launcher, the itos that was called, says on itos
  # version that the repository pins its version and which version the
  # launcher is, whatever the notice; the version line itself is the pin's,
  # as before. Human output (decision 35), on stderr. ITOS_NO_UPDATE turns
  # the notice off, so only the new line can name the pin here.
  @ID-UPDATE-11 @slice-87
  Scenario: In a pinned project itos version names the pin and the itos that was called
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And ITOS_NO_UPDATE is "1"
    When itos runs "version"
    Then itos exits with code 0
    And the version "9.1.0" ran with the arguments "version"
    And its output says "pins 9.1.0"

  @ID-UPDATE-12 @slice-87
  Scenario: With no pin itos version names no pin
    Given a repository whose ledger has the task "T-001"
    And ITOS_NO_UPDATE is "1"
    When itos runs "version"
    Then itos exits with code 0
    And its output does not say "pins"

  # Bug 48 (the user's report, 2026-10-06): itos go in a repository pinning
  # 4.3.2 said "itos 5.0.2 is out" while 6.0.0 was out and the itos that said
  # it was 6.0.0. The notice read only the server's answer, kept for a day,
  # and eleven releases came out that day. The notice names the newest of
  # that answer, the releases the cache holds and the itos that runs, as
  # newest() already does for what runs; and the answer holds an hour, not a
  # day (the notice is still said at most once a day per repository).
  # @ID-UPDATE-02's name said once a day: the fix renames it and names it in
  # Changes.
  # The pin is cached too, so the scenario judges only the update question: the
  # pin's own fetch has nothing to do with it.
  @ID-UPDATE-13 @bug-48
  Scenario: The notice names a release the cache holds that is newer than the server's last answer
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And the launcher's last answer from the release server, a minute old, is "9.1.0"
    And the cache holds the release "9.1.0"
    And the cache holds the release "9.2.0"
    When itos runs "version"
    Then its output says "itos 9.2.0 is out (this repository pins 9.1.0): "
    And the release server was asked for nothing since the last run

  @ID-UPDATE-14 @bug-48
  Scenario: An answer from the release server older than an hour is asked again
    Given a repository whose ledger has the task "T-001"
    And the config pins the version "9.1.0" of the release server
    And the launcher's last answer from the release server, two hours old, is "9.1.0"
    When itos runs "version"
    Then its output says "itos 9.2.0 is out (this repository pins 9.1.0): "

  # Bug 50 (found by T-118's agent, 2026-10-07): version.Compare read the
  # first three numbers alone, so 9.2.0-rc.1 compared equal to 9.2.0 and to
  # 9.2.0-rc.2, against internal/version's own doc and semver: a repository
  # pinning a release candidate was never told the final release was out, and
  # itos upgrade from an rc walked no release. A pre-release sorts below its
  # release, and pre-releases of one release by their identifiers, numbers as
  # numbers (rc.10 above rc.2). It ships before the first v7 rc, so the
  # launcher every repository runs reads rcs right.
  @ID-UPDATE-15 @bug-50 @wip
  Scenario: A repository pinning a release candidate is told when its final release is out
    Given a repository whose ledger has the task "T-001"
    And the release server also offers the version "9.2.0-rc.1"
    And the config pins the version "9.2.0-rc.1" of the release server
    And the launcher's last answer from the release server, a minute old, is "9.2.0"
    When itos runs "version"
    Then its output says "itos 9.2.0 is out (this repository pins 9.2.0-rc.1): "
