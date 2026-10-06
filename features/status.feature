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
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"

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
    And itos has run the command line "question add 'Labels or Projects?'"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "q-1"

  @ID-STATUS-04 @slice-67
  Scenario: Where CI cannot be read, status says so and prints the rest
    Given the fake GitHub refuses the token
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
  # vX.Y.Z on the remote (three numbers and nothing else, bug 20; asked of it
  # as the head is, else as last fetched), and the feat and fix commits on the remote branch since
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
  # as the head's is: by ci.watch.github.nightly_workflow, its newest run on
  # the branch (ci.watch.nightly_command went with the command provider in
  # v5.0.0, slice 85). A config naming none prints no nightly line, and one
  # that cannot be read is said in a line, the rest still printed.
  @ID-STATUS-09 @slice-72
  Scenario: status names the last nightly's run and its result
    Given the watched run's jobs "ci" and "platform" succeed
    And ci.watch's nightly workflow on the fake GitHub has the run "https://ci.example/nightly/7", its job "nightly" failed
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
  # run), only when the head's run has not passed (p1-status-last-green).
  @ID-STATUS-11 @slice-73
  Scenario: While the head's run is going, status names main's last green commit
    Given the watched run never finishes
    And the remote has gained the commit "fix: one" touching "a.txt"
    And the remote has gained the commit "feat: two" touching "b.txt"
    And the clone has fetched the remote
    And the fake GitHub has a green run of the remote's commit "fix: one"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "fix: one"

  @ID-STATUS-12 @slice-73
  Scenario: When the head's run passed, status names no last green commit
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote has gained the commit "fix: one" touching "a.txt"
    And the clone has fetched the remote
    And the fake GitHub has a green run of the remote's commit "fix: one"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "Last green"

  # status took the highest v<semver> tag, a prerelease included, as the
  # newest release, while the release cut counts from the newest plain
  # vX.Y.Z and itos never cuts a prerelease, so with an -rc tag status would
  # list commits since a tag the next release does not count from (found by
  # T-088's agent, 2026-10-04). status reads the newest release by the cut's
  # rule, from the one copy in internal/release.
  @ID-STATUS-13 @bug-20
  Scenario: status counts from the newest plain release, never a prerelease, as the release cut does
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote has gained the commit "docs: describe the archive" touching "b.md"
    And the remote's head is tagged "v1.3.0-rc.1"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    And the clone has fetched the remote
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "Newest release: v1.2.0"
    And its output does not say "v1.3.0-rc.1"

  # Bug 24 (p1-status-last-green-walk, the user's call, 2026-10-05). Since
  # bug 23 ci range's github provider walks the head's first parents and asks
  # each commit's own runs, because GitHub's list of a branch's runs answered
  # stale (issue #8). status's look at the last green commit still took the
  # newest success from that list, so it could name a commit a day old. It
  # reads it as ci range does now: from the head of the branch as fetched,
  # that head included, along its first parents, at most 100, the first
  # commit with a green run of ci.range.github's workflow; none found is
  # "Last green: none found", as before. Its looks at GitHub, this one and
  # the nightly's, ask GITHUB_API_URL when it is set, else
  # https://api.github.com, as ci range and ci watch do since bug 23.
  @ID-STATUS-14 @bug-24
  Scenario: status names the nearest commit with a green run, whatever the list of runs says
    Given the watched run never finishes
    And ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And the remote has gained the commit "fix: one" touching "a.txt"
    And the remote has gained the commit "feat: two" touching "b.txt"
    And the clone has fetched the remote
    And the fake GitHub's list of runs names a green run of the first commit alone
    And the fake GitHub has a green run of the remote's commit "fix: one"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "Last green:"
    And its output says "fix: one"
