@phase-1
Feature: The cost rule knows itos however the project calls it
  ci.cost.static's patterns say which commands take seconds, and the cost
  rule gives every other command the late class. The ledger and ci.steps
  write itos's own commands the way the project calls it, hooks.bin (itos,
  the global launcher, by default; a repository that runs its own build sets
  it, as tools/bin/itos), not always as "itos". So the cost rule reads a
  command whose first word is hooks.bin as if that word were itos: with
  hooks.bin tools/bin/itos, a pattern written "^itos work check" matches
  "tools/bin/itos work check", and the config does not have to spell out
  each way the binary is called. A command that starts any other way is
  matched as it is written.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  # The hook runs only a named task's static checks. Whether the recording
  # script ran tells you which class the cost rule gave its command.
  @ID-COST-01 @slice-8
  Scenario: A pattern written for itos matches a command that starts with hooks.bin
    Given "bin/itos" is a script that records it ran
    And hooks.bin is "bin/itos"
    And ci.cost.static is "^itos work check$"
    And the task "T-001" has the check "bin/itos work check"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the recording check ran

  @ID-COST-03 @slice-8
  Scenario: A command under another path is matched as written, and stays late
    Given "other/itos" is a script that records it ran
    And hooks.bin is "bin/itos"
    And ci.cost.static is "^itos work check$"
    And the task "T-001" has the check "other/itos work check"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the recording check did not run
