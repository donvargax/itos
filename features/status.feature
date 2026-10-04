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

  @ID-STATUS-01 @slice-67
  Scenario: status prints the main branch's head and its CI run's result
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "# Where things stand"
    And its output says "https://ci.example/runs/1"
    And its output says "success"

  @ID-STATUS-02 @slice-67
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

  @ID-STATUS-03 @slice-67
  Scenario: status lists the questions still open
    Given the watched run's jobs "ci" and "platform" succeed
    And itos has run the command line "ask add 'Labels or Projects?'"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "q-1"

  @ID-STATUS-04 @slice-67
  Scenario: Where CI cannot be read, status says so and prints the rest
    Given the watch command prints "not json"
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "slice-9"
    And its output says "CI"

  @ID-STATUS-05 @slice-67
  Scenario: itos go ends with the status
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos" before "# Where things stand"

  # What is released and what is not was still read by hand (gh release list,
  # git log since the tag). status names the newest release, the highest tag
  # of the form v<semver> on the remote (asked of it as the head is, else as
  # last fetched), and the feat and fix commits on the remote branch since
  # it, the ones the next release would carry (the user's calls, 2026-10-04,
  # p1-status-releases-nightly). The last nightly and the last green run need
  # a provider each and wait on an item of their own.
  @ID-STATUS-06 @slice-70
  Scenario: status names the newest release, and the feat and fix commits since it
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote has gained the commit "docs: describe the archive" touching "b.md"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    And the clone has fetched the remote
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "v1.2.0" before "feat: add the archive"
    And its output does not say "docs: describe the archive"

  @ID-STATUS-07 @slice-70
  Scenario: With no release yet, status says so and prints the rest
    Given the watched run's jobs "ci" and "platform" succeed
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "no release yet"
    And its output says "slice-9"

  @ID-STATUS-08 @slice-70
  Scenario: Where the remote's head is not fetched here, status says its unreleased commits may be behind
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "v1.2.0"
    And its output says "may be behind"

  # A red nightly is a coordinator's first item, yet status read only the
  # head's own run, so a session found the nightly red by hand (2026-10-04,
  # p1-status-nightly-green). status names the last nightly's run, read once
  # as the head's is: by ci.watch.github.nightly_workflow for the github
  # provider, or ci.watch.nightly_command for the command one, which prints
  # the run as ci.watch.command does, given no commit. A config naming
  # neither prints no nightly line, and one that cannot be read is said in a
  # line, the rest still printed.
  @ID-STATUS-09 @slice-72
  Scenario: status names the last nightly's run and its result
    Given the watched run's jobs "ci" and "platform" succeed
    And ci.watch.nightly_command reports the run "https://ci.example/nightly/7", its job "nightly" failed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "https://ci.example/nightly/7"
    And its output says "failure"

  @ID-STATUS-10 @slice-72
  Scenario: With no nightly configured, status prints no nightly line
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "Nightly"

  # While main's run is going or red, a coordinator needs what main last
  # proved: status names the last commit whose run passed, as ci.range's
  # provider reads it for a push's range (the github provider's last green
  # run, or ci.range.command's first line), only when the head's run has not
  # passed (p1-status-last-green).
  @ID-STATUS-11 @slice-73 @wip
  Scenario: While the head's run is going, status names main's last green commit
    Given the watched run never finishes
    And the remote has gained the commit "fix: one" touching "a.txt"
    And the remote has gained the commit "feat: two" touching "b.txt"
    And the clone has fetched the remote
    And ci.range runs a command that prints the full SHA of the remote's commit "fix: one"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "fix: one"

  @ID-STATUS-12 @slice-73 @wip
  Scenario: When the head's run passed, status names no last green commit
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote has gained the commit "fix: one" touching "a.txt"
    And the clone has fetched the remote
    And ci.range runs a command that prints the full SHA of the remote's commit "fix: one"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "Last green"
