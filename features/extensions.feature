@phase-3
Feature: Extensions, a command itos does not have run as itos-<cmd> from the PATH
  A command itos does not have runs the program itos-<cmd> found on the PATH,
  and only the PATH, with the rest of the arguments, handing back its exit
  code; a built-in command always wins. Global flags written before the
  command apply first, and reach the extension as ITOS_CONFIG and ITOS_ROOT
  (absolute paths), ITOS_JSON (1 with --json), ITOS_BIN (the itos binary that
  ran it, to call back) and ITOS_VERSION (that itos's version, which a call
  back through a global itos keeps to, pin.feature); the extension runs in
  the root. Everything after the command's name is the extension's, given
  to it unread, as git does, so an extension takes flags of its own with
  any name. itos --help lists the extensions found, and itos help <cmd>
  runs itos-<cmd> --help. A command neither built in nor found is still a
  usage error.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-EXT-01 @slice-29 @wip
  Scenario: A command itos does not have runs itos-<cmd> from the PATH with the rest of the arguments, and its exit code is handed back
    Given the extension "itos-hello" on the PATH exits with code 5
    When itos runs "hello a --b c"
    Then itos exits with code 5
    And the extension "itos-hello" ran with the arguments "a --b c"

  @ID-EXT-02 @slice-29 @wip
  Scenario: A built-in command wins over an extension of the same name
    Given the extension "itos-work" on the PATH exits with code 5
    When itos runs "work check"
    Then itos exits with code 0
    And the extension "itos-work" did not run

  @ID-EXT-03 @slice-29 @wip
  Scenario: A command neither built in nor on the PATH is a usage error
    When itos runs "frob"
    Then itos exits with code 2
    And its output says "unknown command: frob"

  @ID-EXT-04 @slice-29 @wip
  Scenario: Global flags before the command apply first and reach the extension
    Given the extension "itos-hello" on the PATH exits with code 0
    And a "sub" folder
    When itos runs "--root sub --json hello"
    Then the extension "itos-hello" ran with the arguments ""
    And the extension "itos-hello" ran in "sub"
    And the extension "itos-hello" ran with ITOS_JSON "1"
    And the extension "itos-hello" ran with ITOS_ROOT ending in "/sub"
    And the extension "itos-hello" ran with ITOS_CONFIG ending in "/sub/itos.yaml"

  # git's rule: what follows the command's name is the extension's, so an
  # extension can take a --json or --config of its own.
  @ID-EXT-05 @slice-29 @wip
  Scenario: Flags after the command's name are the extension's, unread by itos
    Given the extension "itos-hello" on the PATH exits with code 0
    When itos runs "hello --json --help"
    Then the extension "itos-hello" ran with the arguments "--json --help"
    And the extension "itos-hello" ran with ITOS_JSON ""

  @ID-EXT-06 @slice-29 @wip
  Scenario: An extension can call back the itos that ran it, at the same version
    Given the extension "itos-back" on the PATH runs "$ITOS_BIN version"
    When itos runs "back"
    Then itos exits with code 0
    And the extension "itos-back" was told the version its call back printed

  @ID-EXT-07 @slice-29 @wip
  Scenario: itos --help lists the extensions found on the PATH
    Given the extension "itos-hello" on the PATH exits with code 0
    When itos prints the help of ""
    Then itos exits with code 0
    And its output says "Extensions"
    And its output says "hello"

  @ID-EXT-08 @slice-29 @wip
  Scenario: itos help <cmd> runs the extension's own help
    Given the extension "itos-hello" on the PATH exits with code 0
    When itos runs "help hello"
    Then the extension "itos-hello" ran with the arguments "--help"
