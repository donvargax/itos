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
  predate it, though it is handed to the version it moves to when that is
  newer (upgrade-handoff.feature), and it edits the config itos finds, the
  stealth one included.

  The version it moves from is the pin's, or with no pin the install
  script's; with neither there is nothing to move. Moving to an older
  version is itos pin's, not an upgrade.

  --json: {"schema":1,"action":"upgraded"|"already"|"refused","from","to",
  "releases":[{"version","breaking","upgrading","changes","config"} or
  {"version","notes"} for a release with no upgrading.json],"edited":[paths],
  "applied":[{"version","kind":"renamed"|"removed","key","from"?,"value"?}],
  "manual":[{"version","text"}]}

  # Slice 126: upgrade is handed to the version it moves to when that is
  # newer than the itos that runs (upgrade-handoff.feature), so the itos
  # under test runs stamped as the newest of these releases, and these
  # scenarios judge the walk it does itself, whatever version it is built as.
  Background:
    Given a release server offering the versions "9.1.0", "9.2.0" and "9.3.0", each with an upgrading.json naming the one before it
    And a repository whose ledger has the task "T-001"
    And the itos that runs is the itos under test, as the version "9.3.0"

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

  # Slice 114: an upgrade that edits nothing keeps every comment, so the
  # renamed key is checked too.
  @ID-UPGRADECMD-19 @slice-113
  Scenario: A rename keeps the config's comments and every other line as they were
    Given the config pins the version "9.1.0" of the release server
    And the config has the comment "# Wait ten minutes for CI."
    And the config sets "ci.watch.timeout" to "600"
    And the release "9.3.0"'s schema declares "ci.watch.wait" renamed from "ci.watch.timeout"
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config still has the comment "# Wait ten minutes for CI."
    And the config's "ci.watch.wait" is "600"

  # Slice 150 (#63): a consumer's workflows still launched the action at the
  # old release's commit after itos upgrade, and nothing named those lines.
  # upgrade names each one and says what it runs; it moves none of them.
  #
  # Why it names rather than moves: a workflow pins the action by tag
  # (donvargax/itos@v<x.y.z>) or by commit (@<sha> # v<x.y.z>), and nothing
  # upgrade fetches gives a release's commit: checksums.txt, upgrading.json
  # (its commit fields are the commits that asked something, not the
  # release's) and itos.schema.json name none, and a release is cut without
  # a commit, so its tag is the one source (tools/bin/action-version reads
  # it with git ls-remote --tags from github.com, a call upgrade does not
  # make). Moving the tag lines alone would leave the commit lines, which
  # GitHub advises, and two behaviours where one sentence settles #63. A
  # line upgrade leaves need not move under a pin: the action installs a
  # launcher, the release its ref or its version input names (action.yml's
  # header), and the launcher runs the release the repository's pin names
  # (ADR-0024).
  #
  # What is scanned: the files directly in .github/workflows/ named *.yml or
  # *.yaml (the ones GitHub runs), in the folder itos runs in, where upgrade
  # reads tools/bin/install-itos; a missing folder is no workflow. Each is
  # read as YAML, never as lines: a uses: is named when it is a step's, under
  # jobs.<job>.steps, and its value is donvargax/itos@<ref>, owner and name
  # exactly as action.yml's header writes them. Not named: a comment, a run:
  # script, another owner's action or a fork, a local action (./), a
  # reusable workflow (a job's uses:, donvargax/itos/.github/workflows/...),
  # a subfolder, a file of another extension, a composite action under
  # .github/actions, a value with no @. Each is named by its file, the
  # one-based line of its uses: key, the ref after the @ as written (a
  # tag, a commit, a branch alike: nothing is resolved, a comment after it
  # is not read) and, when the step has with.version, that value as written,
  # an expression unevaluated. Sorted by file, then line. No workflow's env
  # (ITOS_VERSION, say) is read.
  #
  # What it says: the lines, one a line,
  #   .github/workflows/ci.yml:8: donvargax/itos@<ref>
  #   .github/workflows/nightly.yaml:9: donvargax/itos@<ref> (version: <input>)
  # then one sentence. Under a pin, that the action installs a launcher and
  # "the repository's pin chooses the itos that runs", naming the version it
  # pins now; with no pin (the install script's project), that the config
  # "pins no itos, so each of these lines runs the release it installs", and
  # that itos pin pins one. The words around the quoted ones are the
  # implementer's. They come after the releases and before By hand, which
  # stays what the releases ask (slice 113), and after "already on itos
  # <version>" when there is nothing to upgrade. A workflow that names none
  # prints nothing. A file that cannot be read, or is not YAML, is a line
  # naming it and why, beside the lines, and its uses: are not named; upgrade
  # still exits as it would, writes what it would and never lists a workflow
  # among the files it edited.
  #
  # Under a stealth config upgrade names no line: that config is never
  # committed, so no workflow reads its pin, and the workflows are those of a
  # team that does not use itos (config/stealth.go). upgrade has no
  # --dry-run. --json adds "action_uses":[{"file","line","ref",
  # "version_input"?}] and "action_scan_problems":[{"file","message"}] to
  # the object, both [] under a stealth config, with nothing to name and on a
  # refusal; every other key keeps its meaning. A release that cannot be
  # fetched, or a config edit refused, is reported as it is today, no
  # workflow read. The feat updates the feature's description and upgrade's
  # help to say this. No live scenario or corpus case changes: none of them
  # writes a workflow, and none compares upgrade's whole JSON object.
  #
  # Each fails before the work at the first step that reads the report: the
  # output does not name the line (20, 23, 32), or the JSON has no
  # action_uses (21, 22, 24, 31).
  @ID-UPGRADECMD-20 @slice-150 @wip
  Scenario: upgrade names each workflow line that launches the itos action, and says the pin chooses the itos that runs
    Given the config pins the version "9.1.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        check:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - uses: donvargax/itos@1111111111111111111111111111111111111111 # v9.1.0
            - run: itos verify
      """
    And the committed file ".github/workflows/nightly.yaml" holding the lines:
      """
      name: nightly
      on: push
      jobs:
        all:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - name: itos
              uses: donvargax/itos@v9.2.0
              with:
                version: 9.1.0
            - run: itos ci --nightly
      """
    When itos runs "upgrade"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its output says ".github/workflows/ci.yml:8: donvargax/itos@1111111111111111111111111111111111111111"
    And its output says ".github/workflows/nightly.yaml:9: donvargax/itos@v9.2.0 (version: 9.1.0)"
    And its output says "the repository's pin chooses the itos that runs" before "By hand:"
    And the file ".github/workflows/ci.yml" has the same contents as its committed version
    And the file ".github/workflows/nightly.yaml" has the same contents as its committed version
    And nothing was committed

  # A quoted value is the same uses: read as YAML; a branch is named as any
  # ref is, and an expression in with.version as written.
  @ID-UPGRADECMD-21 @slice-150 @wip
  Scenario: upgrade --json lists each workflow line that launches the itos action, sorted by file and line, and edits none
    Given the config pins the version "9.1.0" of the release server
    And the committed file ".github/workflows/b.yaml" holding the lines:
      """
      name: b
      on: push
      jobs:
        one:
          runs-on: ubuntu-latest
          steps:
            - uses: "donvargax/itos@v9.2.0"
      """
    And the committed file ".github/workflows/a.yml" holding the lines:
      """
      name: a
      on: push
      jobs:
        one:
          runs-on: ubuntu-latest
          steps:
            - uses: donvargax/itos@2222222222222222222222222222222222222222 # v9.1.0
              with:
                version: ${{ vars.ITOS_VERSION }}
        two:
          runs-on: ubuntu-latest
          steps:
            - uses: donvargax/itos@main
      """
    When itos runs "upgrade --json"
    Then itos exits with code 0
    And its JSON's "action" is "upgraded"
    And its JSON's "action_uses" is [{"file":".github/workflows/a.yml","line":7,"ref":"2222222222222222222222222222222222222222","version_input":"${{ vars.ITOS_VERSION }}"},{"file":".github/workflows/a.yml","line":13,"ref":"main"},{"file":".github/workflows/b.yaml","line":7,"ref":"v9.2.0"}]
    And its JSON's "action_scan_problems" is []
    And its JSON's "edited" is ["itos.yaml"]
    And the file ".github/workflows/a.yml" has the same contents as its committed version

  @ID-UPGRADECMD-22 @slice-150 @wip
  Scenario: Only a step's uses of the itos action is named: not a comment, a script, another action, a reusable workflow or a file GitHub does not run
    Given the config pins the version "9.1.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        release:
          uses: donvargax/itos/.github/workflows/release.yml@v9.1.0
        check:
          runs-on: ubuntu-latest
          steps:
            # - uses: donvargax/itos@v9.1.0
            - uses: someone/itos@v9.1.0
            - uses: donvargax/itos-cc@v9.1.0
            - uses: ./
            - run: |
                echo "uses: donvargax/itos@v9.1.0"
      """
    And the committed file ".github/workflows/old/ci.yml" holding the lines:
      """
      jobs:
        check:
          steps:
            - uses: donvargax/itos@v9.1.0
      """
    And the committed file ".github/workflows/README.md" holding "- uses: donvargax/itos@v9.1.0"
    And the committed file ".github/actions/setup/action.yml" holding the lines:
      """
      runs:
        using: composite
        steps:
          - uses: donvargax/itos@v9.1.0
      """
    When itos runs "upgrade --json"
    Then itos exits with code 0
    And its JSON's "action_uses" is []
    And its JSON's "action_scan_problems" is []

  @ID-UPGRADECMD-23 @slice-150 @wip
  Scenario: A project already on the version still has its workflow lines named, and nothing changes
    Given the config pins the version "9.3.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        check:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - uses: donvargax/itos@v9.1.0
      """
    When itos runs "upgrade"
    Then itos exits with code 0
    And its output says "already on itos 9.3.0"
    And its output says ".github/workflows/ci.yml:8: donvargax/itos@v9.1.0"
    And its output says "the repository's pin chooses the itos that runs"
    And the config is unchanged
    And the file ".github/workflows/ci.yml" has the same contents as its committed version

  @ID-UPGRADECMD-24 @slice-150 @wip
  Scenario: A workflow that is not YAML is named as a scan problem, the other workflows' lines are named, and the pin still moves
    Given the config pins the version "9.1.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        check:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - uses: donvargax/itos@v9.1.0
      """
    And the committed file ".github/workflows/bad.yml" holding the lines:
      """
      name: bad
      jobs:
        check:
          steps: [
      """
    When itos runs "upgrade --json"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its JSON's "action_uses" is [{"file":".github/workflows/ci.yml","line":8,"ref":"v9.1.0"}]
    And its JSON's "action_scan_problems" has one entry whose "file" is ".github/workflows/bad.yml"
    And its JSON's "edited" is ["itos.yaml"]
    And the file ".github/workflows/bad.yml" has the same contents as its committed version

  @ID-UPGRADECMD-31 @slice-150 @wip
  Scenario: Under a stealth config upgrade names no workflow line, since no workflow reads that config's pin
    Given itos's config is kept in the git folder
    And the config pins the version "9.1.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        check:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - uses: donvargax/itos@v9.1.0
      """
    When itos runs "upgrade --json"
    Then itos exits with code 0
    And the config's pin is the version "9.3.0" of the release server, with its checksums
    And its JSON's "action_uses" is []
    And its JSON's "action_scan_problems" is []

  # With no pin the launcher runs itself (ADR-0024, internal/launch's
  # unpinned), so the release each line installs is the one that runs.
  @ID-UPGRADECMD-32 @slice-150 @wip
  Scenario: With no pin, upgrade names each workflow line and says the release it installs is the itos that runs
    Given the repository has the install script of the version "9.1.0" of the release server
    And the committed file ".github/workflows/ci.yml" holding the lines:
      """
      name: ci
      on: push
      jobs:
        check:
          runs-on: ubuntu-latest
          steps:
            - uses: actions/checkout@v4
            - uses: donvargax/itos@v9.1.0
      """
    When itos runs "upgrade"
    Then itos exits with code 0
    And the install script is the one of the version "9.3.0" of the release server
    And its output says ".github/workflows/ci.yml:8: donvargax/itos@v9.1.0"
    And its output says "pins no itos, so each of these lines runs the release it installs"
    And its output does not say "the repository's pin chooses the itos that runs"
    And the file ".github/workflows/ci.yml" has the same contents as its committed version
