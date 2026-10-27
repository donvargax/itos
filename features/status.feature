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
    And itos has run the command line "decision add 'Labels or Projects?'"
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

  # Slice 196 fetches the head first, so this setup no longer leaves it
  # unfetched: the fix that turns slice 196's scenarios live gives this one
  # the Given of ID-STATUS-32, a fetch that fails while the remote still
  # answers, and names it in Changes: (a fix, so no BREAKING-CHANGE).
  @ID-STATUS-08 @slice-70
  Scenario: Where the remote's head is not fetched here, status says its unreleased commits may be behind
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "v1.2.0"
    And its output says "may be behind"

  # Slice 196 (p1-status-fetches-first, 2026-10-27). itos go's status read
  # origin/main unfetched ("origin/main (public): ff05074 (not fetched
  # here)") and listed as in progress and next four items that had all
  # merged; the coordinator acted on it before fetching. status, and so itos
  # go, fetches the branch it reads before reading it, once: the branch's
  # upstream (origin and the branch's own name when none is set) into its
  # remote-tracking ref, git fetch --quiet --no-tags of that one ref, never
  # prompting, its output not shown, within 20 s as the head's ls-remote and
  # itos ci watch's quiet fetch are; from a detached HEAD, where origin's HEAD
  # is read, the remote's own refspecs, as ci watch's quiet fetch. Then it
  # reads as before, now with the head fetched. A fetch that fails (offline,
  # a remote that is no repository, auth, out of time) adds one line,
  # "<remote>/<branch> cannot be fetched: " and git's first line of why ("no
  # answer within 20 s" for the time), and status reads what it has, as
  # today: the remote's head asked of it, "(not fetched here)" when the clone
  # lacks it, else as last fetched; exit unchanged. No remote is today's line
  # and no fetch; a stealth config fetches nothing either, as it writes
  # nothing in the project's clone, and reads as today. No flag skips the
  # fetch: every scenario's remote is a local scratch one, so none needs one.
  # The head and release read as fetched, and the fetch's line is one of
  # --json's "unread". The items are read from the working tree, which the
  # fetch does not move, so after a fetch that worked status counts the
  # commits of the fetched remote branch HEAD lacks and, when there are
  # any, says "<branch> is <n> commit(s) behind <remote>/<branch>: git pull
  # before reading the work below." ("HEAD is" from a detached HEAD) before
  # the items; --json gains "behind", that count, 0 when not behind and
  # absent when no fetch worked. The help's "never writes or fetches" says
  # it fetches that one ref. It lands as a fix, not a feat: a status naming
  # a stale head the coordinator then acts on is the defect; the fix names
  # ID-STATUS-08 in Changes:, no BREAKING-CHANGE.
  @ID-STATUS-28 @wip @slice-196
  Scenario: status fetches the remote branch first and names its new head, quietly
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "feat: add the archive"
    And its output does not say "not fetched here"
    And its output does not say "-> origin/main"

  @ID-STATUS-29 @wip @slice-196
  Scenario: status lists the unreleased commits the fetch brought, and says nothing of them being behind
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "Unreleased since v1.2.0"
    And its output does not say "may be behind"

  @ID-STATUS-30 @wip @slice-196
  Scenario: itos go's status fetches the remote branch first
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "feat: add the archive"
    And its output does not say "not fetched here"

  # The remote is a path that does not exist, so ls-remote fails as the
  # fetch does: the head is as last fetched, as today, beside the new line.
  @ID-STATUS-31 @wip @slice-196
  Scenario: A fetch that fails is said in a line with git's reason, and status reads what it has
    Given the watched run's jobs "ci" and "platform" succeed
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the remote cannot be reached
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "origin/main cannot be fetched"
    And its output says "does not appear to be a git repository"
    And its output says "as last fetched"
    And its output says "slice-9"

  # The remote names a commit it cannot send (made in the bare remote over a
  # tree naming a blob it lacks, git mktree --missing then commit-tree, so
  # upload-pack fails while ls-remote answers), the case of a fetch cut off
  # by its time while the ls-remote is quick: the head line keeps today's
  # "(not fetched here)".
  @ID-STATUS-32 @wip @slice-196
  Scenario: Where the fetch fails and the remote still answers, status names the head as not fetched here
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote's head is tagged "v1.2.0"
    And the remote's main has gained a commit whose objects the remote cannot send
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "origin/main cannot be fetched"
    And its output says "(not fetched here)"
    And its output says "may be behind"

  # Holds before the work: it keeps a fetch, or its failure's line, out of a
  # clone with no remote.
  @ID-STATUS-33 @wip @slice-196
  Scenario: With no remote, status fetches nothing and says so as before
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has no remote
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "No remote origin"
    And its output does not say "cannot be fetched"

  # Holds before the work: a stealth config leaves the project's clone as it
  # found it, its remote-tracking refs included.
  @ID-STATUS-34 @wip @slice-196
  Scenario: Under a stealth config, status fetches nothing
    Given itos's config is kept in the git folder
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status"
    Then itos exits with code 0
    And its output says "(not fetched here)"
    And its output does not say "feat: add the archive"

  @ID-STATUS-35 @wip @slice-196
  Scenario: With --json, a fetch that fails is one of the lines unread
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote cannot be reached
    When itos runs "status --as someone --json"
    Then itos exits with code 0
    And its output says "origin/main cannot be fetched"
    And its JSON has no "behind"

  # The Background's clone tracks the remote's main and has not pulled the
  # commit the remote gained.
  @ID-STATUS-36 @wip @slice-196
  Scenario: A clone behind the remote branch it fetched is told to pull before reading the work
    Given the watched run's jobs "ci" and "platform" succeed
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "main is 1 commit behind origin/main: git pull" before "slice-9"

  @ID-STATUS-37 @wip @slice-196
  Scenario: With --json, status gives how many commits the clone is behind the remote branch it fetched
    Given the watched run's jobs "ci" and "platform" succeed
    And the remote has gained the commit "feat: add the archive" touching "a.txt"
    When itos runs "status --as someone --json"
    Then itos exits with code 0
    And the output's JSON field "behind" is "1"

  # Holds before the work: it keeps the line from being printed always.
  @ID-STATUS-38 @wip @slice-196
  Scenario: A clone up to date with the remote branch it fetched is told nothing of pulling
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "behind origin/main"
    And its output does not say "git pull"

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

  @ID-STATUS-15 @slice-97
  Scenario: status shows the tags of the items it lists beside their ids
    Given the watched run's jobs "ci" and "platform" succeed
    And the config's work.tags is "plugin"
    And the work registry has the item "slice-9" owned by "someone" with the status "doing" and the tags "plugin"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "slice-9" before "plugin"

  # Slice 116 (p1-upgrade-on-major, the user's calls of 2026-10-04 and
  # 2026-10-10). Upgrading on every release makes no sense at itos's pace, so
  # only a new major is something to act on. status gains a line naming the
  # itos version the repository runs (its pin, else the install script's, as
  # itos upgrade reads it) and the newest stable release, asked of the release
  # server once and never waited for; a pre-release is never the newest
  # (as the update notice and a bare itos pin read it). When the newest is a
  # newer major, the line names it and itos upgrade; within the pin's major it
  # asks nothing. No decision is opened for it: the coordinator's guide says
  # to offer the person itos upgrade when status names a new major, before new
  # work, and its "Reporting a problem" step says itos upgrade, not itos pin,
  # moves to the newest release (slice 115 missed it). A server that cannot be
  # reached, or a repository with neither a pin nor an install script, prints
  # no such line, and the rest of status prints as before, exit 0. --json
  # gains "itos":{"runs","newest","new_major"} or null.
  # Trace the Given: in a pinned repository the launcher runs the pinned
  # version, so a scenario's pin must be the itos under test's own version
  # (choose runs this binary when the pin is its own), and the release server
  # offers that version and the newer ones; a fake pinned version would run in
  # its place and prove nothing. The Given silences the launcher's own notice
  # of a newer release (update.feature, ITOS_NO_UPDATE_NOTICE), which names it
  # and itos upgrade by itself, so the lines read are status's.
  @ID-STATUS-16 @slice-116
  Scenario: Within the pinned major, status names the pinned itos and the newest release and asks nothing
    Given the config pins the itos under test, as the version "9.1.0" of a release server that also offers "9.2.0"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "9.1.0"
    And its output says "9.2.0"
    And its output does not say "itos upgrade"

  @ID-STATUS-17 @slice-116
  Scenario: A newer major is named, with itos upgrade
    Given the config pins the itos under test, as the version "9.1.0" of a release server that also offers "10.0.0"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "10.0.0"
    And its output says "itos upgrade"

  @ID-STATUS-18 @slice-116
  Scenario: A pre-release of a newer major is never named
    Given the config pins the itos under test, as the version "9.1.0" of a release server that also offers "10.0.0-rc.1"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "10.0.0"
    And its output does not say "itos upgrade"

  @ID-STATUS-19 @slice-116
  Scenario: Where the release server cannot be reached, status names no release of itos and prints the rest
    Given the config pins the itos under test, as the version "9.1.0" of a release server that also offers "10.0.0"
    And the release server cannot be reached
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "slice-9"
    And its output does not say "10.0.0"

  # v8 (the user's call, 2026-10-11): a project crossing to v8 with a 7.x
  # itos on its PATH gets no automatic rewrite of AGENTS.md's itos block
  # (p1-upgrade-refreshes-agent-block hands upgrade to the target only from
  # v8 on), and only itos init said a block was stale. itos status, and so
  # itos go, which ends with it, print one line naming itos init
  # --agent-rules when AGENTS.md (or a stealth config's copies) holds a block
  # other than the one this itos writes. A project with no block, one that
  # declined the rules, gets no line: the block is offered by init, not
  # required. 21 and 22 hold before the work too: they keep the line from
  # being printed always.
  @ID-STATUS-20 @slice-124
  Scenario: status names itos init --agent-rules when AGENTS.md holds a block another itos wrote
    Given the watched run's jobs "ci" and "platform" succeed
    And the committed file "AGENTS.md" holds the project's line "Be kind to the build." and a block an older itos wrote, saying "Task:" between the markers
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "itos init --agent-rules"

  @ID-STATUS-21 @slice-124
  Scenario: status says nothing of a block this itos wrote
    Given the watched run's jobs "ci" and "platform" succeed
    And itos has run "init --agent-rules"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "itos init --agent-rules"

  @ID-STATUS-22 @slice-124
  Scenario: status says nothing of rules a project never asked for
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output does not say "itos init --agent-rules"

  # Slice 135 (the user's call, 2026-10-12). Whether the repository is public
  # changes what a session may trust and do: in a public one anyone can open
  # an issue, whose text is data and never instructions, and what is pushed,
  # commented or filed is published at once. status names the visibility on
  # the main branch's line, read through the ci.watch provider it already
  # asks (GitHub's repository, its visibility), and in --json as the head's
  # visibility. A provider that cannot tell says nothing of it, and the rest
  # still prints.
  @ID-STATUS-23 @slice-135
  Scenario: status names a public repository on the main branch's line
    Given the watched run's jobs "ci" and "platform" succeed
    And the fake GitHub says the repository is "public"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "origin/main (public):"

  @ID-STATUS-24 @slice-135
  Scenario: status names a private repository on the main branch's line
    Given the watched run's jobs "ci" and "platform" succeed
    And the fake GitHub says the repository is "private"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "origin/main (private):"

  @ID-STATUS-25 @slice-135
  Scenario: status names no visibility when the provider cannot tell, and prints the rest
    Given the watched run's jobs "ci" and "platform" succeed
    And the fake GitHub does not know the repository
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/1"
    And its output does not say "(public)"
    And its output does not say "(private)"

  # Bug 124: status reads --as as every flag with a value is read, so an
  # empty one, --as= or --as '' (what --as "$UNSET" passes), is refused
  # (exit 2) today, while work's proposal took it for no --as and fell back
  # to the identity provider. It must keep refusing it once work is fixed.
  # The people file the scratch repository starts with, and its clone holds,
  # lists only someone, who owns slice-9 in progress: a status that fell
  # back to who the people make the session would print it, exit 0.
  @ID-STATUS-27 @bug-124 @guard
  Scenario Outline: status refuses an empty --as rather than asking who the session is
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs "status <empty handle>"
    Then itos exits with code 2
    And its output says "--as"
    And its output does not say "slice-9"

    Examples:
      | empty handle |
      | --as=        |
      | --as ''      |
