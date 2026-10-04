@phase-1
Feature: itos status, where the work stands
  A coordinator kept where the work stands by hand in a handoff file,
  rewritten after every landing: main's green commit and run, what is
  released, what is in progress, what comes next and what waits on the
  person. itos status prints it from what already records it (the user's
  calls, 2026-10-04, p1-handoff-status), for the person it works for (--as,
  else the identity provider, as itos work): the remote branch's head and
  its CI run's result, read once through ci.watch and never waited for; the
  person's items in progress; the next ones they can start, in the queue's
  order; and the questions still open. It reads, never writes. What cannot
  be reached (no remote, no CI provider) is said in a line, and the rest
  still prints, exit 0. itos go prints it after the guides, so a session
  starts from one command. This first slice is the state alone; a newer
  major release joins it with itos upgrade.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"

  @ID-STATUS-01 @slice-67 @wip
  Scenario: status prints the main branch's head and its CI run's result
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "# Where things stand"
    And its output says "https://ci.example/runs/1"
    And its output says "success"

  @ID-STATUS-02 @slice-67 @wip
  Scenario: status lists the person's items in progress and the next ones they can start, in the queue's order
    Given the watched run's jobs "ci" and "platform" succeed
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And itos has run "work queue slice-8 --top"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "slice-9" before "slice-8"
    And its output says "slice-8" before "slice-7"

  @ID-STATUS-03 @slice-67 @wip
  Scenario: status lists the questions still open
    Given the watched run's jobs "ci" and "platform" succeed
    And itos has run the command line "ask add 'Labels or Projects?'"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "q-1"

  @ID-STATUS-04 @slice-67 @wip
  Scenario: Where CI cannot be read, status says so and prints the rest
    Given the watch command prints "not json"
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "slice-9"
    And its output says "CI"

  @ID-STATUS-05 @slice-67 @wip
  Scenario: itos go ends with the status
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos" before "# Where things stand"
