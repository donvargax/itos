@phase-1
Feature: Flags and arguments, parsed from a spec
  Slice 88 (the user's calls, 2026-10-06, decision of q-16): each command
  parsed its flags by hand, with no spec. Some commands refused an unknown
  flag and others ignored it with exit 0; --flag=value was dropped
  everywhere but work --as=; a global flag was taken out wherever it stood,
  even as another flag's value; and a ref argument was used unchecked. Every
  command now parses its flags from a declared spec (docs/CLI.md, rules 19
  to 21 and 25). Slice 89 adds itos help's unknown topic here.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-CLI-01 @slice-88
  Scenario: An unknown flag is refused with exit 2, naming it
    When itos runs "task list --bogus"
    Then itos exits with code 2
    And its output says "--bogus"

  @ID-CLI-02 @slice-88
  Scenario: --flag=value is read as --flag value
    When itos runs "task list --group=9"
    Then its output does not say "T-001"

  @ID-CLI-03 @slice-88
  Scenario: A flag's value is never read as a global flag
    When itos runs "task list --group --json"
    Then itos exits with code 2
    And its output says "--group"

  @ID-CLI-04 @slice-88
  Scenario: A ref that names no commit is refused with exit 2, naming it
    When itos runs "ci plan nosuchref HEAD"
    Then itos exits with code 2
    And its output says "nosuchref"

  @ID-CLI-05 @slice-88
  Scenario: verify refuses a ref that names no commit in a line for people, not git's command line
    When itos runs "verify nosuchref HEAD"
    Then itos exits with code 2
    And its output says "nosuchref"
    And its output does not say "Command failed"

  @ID-CLI-06 @slice-89 @wip
  Scenario: itos help with an unknown topic exits 2
    When itos runs "help nosuch"
    Then itos exits with code 2
    And its output says "nosuch"
