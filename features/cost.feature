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

  # Bug 27 (issue #4; was p1-ledger-pattern-static-after-late). With
  # ci.cost.keep_written_order, a task's checks never run before the ones
  # written above them, so a static check below a late one runs late, and
  # the hook and the nightly's done tasks leave it out. config check said so
  # for a check marked cost: static (ledger-static-after-late) but not for
  # one static by a ci.cost.static pattern, which is left out just the same,
  # without a word. It now names that one too. As a warning, not a problem
  # (the coordinator's call, 2026-10-05): the explicit mark asks for
  # something the order cannot give, while a pattern match is the cost
  # rule's own reading, and failing ledgers that pass today would turn a
  # patch into a break for every consumer holding one.
  @ID-COST-04 @bug-27
  Scenario: config check warns of a check static by ci.cost.static written below a late one
    Given ci.cost.static is "^itos work check$"
    And ci.cost.keep_written_order is true
    And the task "T-001" has the check "exit 0", then the check "itos work check"
    When itos checks the config
    Then itos exits with code 0
    And its output says "static by ci.cost.static"
