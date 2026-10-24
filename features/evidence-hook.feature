@phase-1
Feature: The commit-msg hook answers an opted-in check from the record of the tree it commits
  Slice 171, step A4 of docs/plans/evidence-records.md (ADR-0060). bug-112
  (hook-task-snapshot.feature) runs the checks of the tasks a commit names
  in a disposable checkout of the captured index; slice 169
  (evidence-local.feature) keys an opted-in check by the index tree and
  records it. Here the hook does the same lookup on the tree it captured, so
  a run on the same index, a hand-run itos task or an earlier commit attempt,
  leaves a record the hook hits, and a check runs once. The rules of
  evidence-local.feature's description hold here unchanged: the opt-in, the
  key, the store, the record's files. What the store does under contention
  and damage is evidence-store.feature's (slice 170), and holds in the hook
  as it does by hand: a lock not taken or a store that cannot be written
  runs the check in its own checkout, unrecorded, never an exit 3. A check
  without evidence runs in the hook exactly as today.

  The key. The hook keys an opted-in check by the tree it captured, the
  active index, GIT_INDEX_FILE's when git commit names one (a commit of
  paths, git commit -- <path>, commits a temporary index). Every other part
  is what itos task binds for the same check: the command, the declaration,
  the policy (the working tree's config, which the hook loads as itos task
  does), the binary, the declared environment and the declared inputs and
  tools. So a hand run on the index the commit holds and the hook's lookup
  are one key, and the record either makes is the other's hit. Which checks
  run, and their command, mode and cost, are the captured task's, as
  bug-112 reads them; only static checks before the first late one run in
  the hook, so every check here says cost: static.
  - The timeout the key binds is the check's own, its timeout: or the
    ledger's ledger.check.timeout, as itos task binds it; never
    hooks.commit_msg.check_timeout, which caps how long a check may hold a
    commit. A miss runs under that cap as today; a run the cap stops is not
    recorded (a timeout is no result to reuse), and a hit holds the commit
    no time at all.
  - Declared untracked inputs are hashed and copied into the check's own
    checkout, as by hand; an undeclared untracked file never reaches it, as
    bug-112 promises for every check (@ID-HOOKSNAP-01).
  - An after: push check is not run by the hook, as today, and is never
    looked up.

  Where a check runs. A hit runs nothing and needs no checkout. A missed
  opted-in check runs in a checkout of the captured tree of its own, never
  the one the hook's other checks share, and is recorded. Checks without
  evidence keep bug-112's one shared checkout (@ID-HOOKSNAP-07). When every
  runnable check is a hit the hook makes no snapshot at all. When any check
  must run, a checkout that cannot be made (the snapshots folder unusable,
  a gitlink in the index) refuses the commit with bug-112's exit 3 before
  any check runs, never falling back to the working folder as a hand run
  does (@ID-EVID-23). Each opted-in check is looked up again at its turn, so
  a missed check that an earlier named task of the same hook run ran and
  recorded is a hit for a later task that shares it: a check two named
  tasks share runs once, as slice-169's shared check does (@ID-EVID-15).

  Contract, which these scenarios judge:
  - Exit codes. A hit replays the recorded exit, read by the check's own
    run: or fails:, and the hook judges it as it judges a run today: a
    failing check of a done task rejects the commit, exit 1; of a task in
    progress it is reported and the commit goes through.
  - Files: the records of evidence-local.feature, in the git common dir's
    itos/evidence. A miss in the hook writes one; itos task on the same
    index then hits it.
  - Loose text, judged by a substring at most. A hit says "cached evidence".
    A failing hit prints its report as a failing run does, the recorded
    output tail among it.
  - No machine-readable output. The hook has none today and gains none.

  # evidence-probe is evidence-local.feature's (its comment above that
  # Background says what it logs): a hit is a run its log does not gain,
  # never one inferred from speed or wording. "The commit-msg hook checks the
  # message" runs itos hook commit-msg <file>, the command the hook git runs
  # executes, as commit-tasks.feature and hook-task-snapshot.feature drive it;
  # it makes no commit, so two of them in a row are two attempts on one
  # index. @ID-EVID-34 and @ID-EVID-35 commit for real, through git commit
  # and the commit-msg hook installed in the fixture's repository.
  Background:
    Given a repository whose ledger has the task "T-001"
    And the evidence probe is on PATH, logging its runs outside the repository

  # A4. The hand run is on the staged tree (q-102), the index the commit
  # holds, so the hook's lookup is the same key, and the hook runs nothing.
  # The hook caps the check at hooks.commit_msg.check_timeout (60) and
  # itos task at the ledger's 600: the key's timeout is the check's own,
  # or the two would never meet (@ID-EVID-44 pins it). Proposed: the hit's
  # line, on stderr, names the task and the command and says "cached
  # evidence" with the recorded verdict, exit and time, as itos task's does;
  # a passing check prints nothing else, as today. Before the work the hook
  # runs the probe again.
  @ID-EVID-31 @slice-171
  Scenario: A commit of the index a hand-run itos task checked answers from its record and runs nothing
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And itos has run "task T-001"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output says "cached evidence"

  # A4's own scenario from the plan. The first attempt misses, runs in its
  # checkout of the staged tree and cannot see the unstaged "three"; the
  # second, on the same index, hits. A miss prints nothing new: proposed, no
  # "running" line, so a commit whose checks pass stays as quiet as today.
  # itos task on that index then hits the hook's record: the record, read
  # through itos task --json, is how the hook's work is seen by a program.
  # Before the work the store holds no record after the first attempt.
  @ID-EVID-32 @slice-171
  Scenario: A commit attempt runs the check on the staged snapshot, unstaged edit left out, and the next attempt on that index hits
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And "input.txt" is staged with the text "two"
    And the working-tree copy of "input.txt" is replaced with "three"
    When the commit-msg hook checks the message:
      """
      chore: take the new input

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And the evidence store holds 1 complete record
    And its output does not say "cached evidence"
    When the commit-msg hook checks the message:
      """
      chore: take the new input

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output says "cached evidence"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true
    And its JSON's "unstaged" is 1

  # A4. The index changed after the hand run, so the commit's tree is
  # another key: the hook misses, runs on the tree it commits, and records
  # it beside the hand run's record. Before the work the store holds only
  # the hand run's record.
  @ID-EVID-33 @slice-171
  Scenario: A commit of another index than the hand run's misses, runs in its snapshot and records, and the next attempt hits
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And itos has run "task T-001"
    And "input.txt" is staged with the text "two"
    When the commit-msg hook checks the message:
      """
      chore: take the new input

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And the evidence store holds 2 complete records
    When the commit-msg hook checks the message:
      """
      chore: take the new input

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 2 times

  # A4, end to end: git commit runs the commit-msg hook the fixture
  # installed, which runs the itos under test, and the commit is made
  # without the check running again. Before the work the hook runs the
  # probe a second time.
  @ID-EVID-34 @slice-171
  Scenario: A real commit through the installed hook hits the hand run's record and is made
    Given the commit-msg hook is installed
    And the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And itos has run "task T-001"
    When itos commits with the arguments "--task T-001 -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the message of HEAD has the footer "Task: T-001"

  # A4. A commit of a path makes git commit a temporary index, HEAD's tree
  # with the path's working copy, and hand it to the hook in
  # GIT_INDEX_FILE: that is the tree the commit holds and the hook keys
  # (bug-112 captures the active index). It is not the index the hand run
  # keyed (HEAD's, nothing staged), so the hook misses and runs on the
  # unstaged "two" the commit takes in. Once the commit is made the index
  # tree is HEAD's, which is the hook's key, so itos task hits the hook's
  # record. Before the work the hook records nothing and that run misses.
  @ID-EVID-35 @slice-171
  Scenario: A commit of a path keys the tree the commit holds, not the index, and itos task after it hits the hook's record
    Given the commit-msg hook is installed
    And the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And the file "input.txt" is changed to hold "two"
    And itos has run "task T-001"
    When itos commits with the arguments "--task T-001 -m 'chore: take the new input' -- input.txt"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" true

  # A4, opt-in. The plain check beside the opted-in one runs at every
  # attempt, in the hook's shared checkout, as bug-112 runs it; the
  # opted-in check runs once. That half passes today; the opted-in check's
  # single run is what fails before the work.
  @ID-EVID-36 @slice-171
  Scenario: In the hook a check that does not opt in runs at every attempt, beside one that runs once
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
          - run: evidence-probe 0 README.md
            cost: static
      """
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran "0 input.txt" 1 time
    And the evidence probe ran "0 README.md" 2 times
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"

  # Guard: passes before the work and must keep passing. With no opted-in
  # check the hook is today's, even after a hand run of the same check on
  # the same index: it runs the check at every attempt, looks nothing up,
  # says nothing of evidence and stores nothing.
  @ID-EVID-37 @slice-171 @guard
  Scenario: With no opted-in check the hook runs the check at every attempt and stores nothing
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And itos has run "task T-001"
    And the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 3 times
    And its output does not say "cached evidence"
    And the evidence store holds no record

  # A4. Failed results are reusable in the hook as by hand (@ID-EVID-14): a
  # failing record of a done task rejects the commit, exit 1, and one of a
  # task in progress is reported and lets it through, each printing the
  # failure report a run prints, the recorded output tail among it. Before
  # the work the hook runs the probe again.
  @ID-EVID-38 @slice-171
  Scenario Outline: A failing record judges the commit as a failing run does, with the recorded output
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 1 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "<status>"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    When itos runs "task T-001"
    Then itos exits with code 1
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code <exit>
    And the evidence probe ran 1 time
    And its output says "failing T-001"
    And its output says "input.txt: one"
    And its output says "cached evidence"

    Examples:
      | status | exit |
      | done   | 1    |
      | doing  | 0    |

  # A4, bug-112. Proposed: a hit makes no snapshot, so when every runnable
  # check is a hit the hook makes none at all and an unusable snapshots
  # folder refuses nothing. Once a check must run, here T-002's check that
  # does not opt in, the hook refuses with bug-112's exit 3 before any check
  # runs, as today. Before the work the hook makes its snapshot whatever the
  # checks are, and the first attempt exits 3.
  @ID-EVID-39 @slice-171
  Scenario: A hit needs no snapshot, and a check that must run still meets bug-112's refusal
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 README.md
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And itos has run "task T-001"
    And the fixture writes a regular file at the git common dir's "itos/snapshots"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output says "cached evidence"
    And its output does not say "cannot create task-check snapshot"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001, T-002
      """
    Then itos exits with code 3
    And its diagnostic contains "cannot create task-check snapshot" and identifies the git common dir's "itos/snapshots"
    And the evidence probe ran 1 time

  # Guard: passes before the work and must keep passing. A missed opted-in
  # check whose checkout cannot be made is refused as bug-112 refuses it
  # (@ID-HOOKSNAP-09), exit 3, nothing run and nothing recorded, never run
  # uncached in the working folder as a hand run would (@ID-EVID-23): a
  # commit's checks judge the staged tree or nothing. Under a gitlink there
  # is never a hit to spare the snapshot: a hand run there records nothing
  # (@ID-EVID-23) and the hook refuses, so no record of such a tree is made.
  @ID-EVID-40 @slice-171 @guard
  Scenario Outline: A missed opted-in check whose checkout cannot be made refuses the commit with exit 3, as today
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    And <problem>
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 3
    And its diagnostic contains "<reason>" and identifies <location>
    And the evidence probe ran 0 times
    And the evidence store holds no record

    Examples:
      | problem                                                                    | reason                            | location                              |
      | the fixture writes a regular file at the git common dir's "itos/snapshots" | cannot create task-check snapshot | the git common dir's "itos/snapshots" |
      | the index contains a gitlink at "vendor/submodule" pointing to HEAD        | unsupported gitlink               | the staged path "vendor/submodule"    |

  # Guard: passes before the work and must keep passing. The hook does not
  # run an after: push check (it waits for the push) and never looks one
  # up: T-002's failing record of the same command is there to be found,
  # and replaying it would reject the commit of a done task. With no
  # runnable check the hook makes no snapshot, as today (@ID-HOOKSNAP-12).
  @ID-EVID-41 @slice-171 @guard
  Scenario: An opted-in after: push check is never looked up by the hook, beside a failing record of its command
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 1 input.txt
            cost: static
            after: push
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 1 input.txt
            cost: static
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    When itos runs "task T-002"
    Then itos exits with code 1
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output does not say "cached evidence"
    And the evidence store holds 1 complete record

  # A4, decided: bug-112 shares one checkout between the hook's checks
  # (@ID-HOOKSNAP-07), slice 169 gives each missed check one of its own
  # (@ID-EVID-29). In the hook each missed opted-in check gets its own, so
  # its record holds a run on the captured tree alone and never depends on
  # what an earlier check wrote, which the key does not bind; checks
  # without evidence keep the shared one, so a plain check that reads what
  # a plain check above it wrote still finds it (@ID-HOOKSNAP-07). In each
  # row the first check writes marker.txt where it runs and the second, a
  # different check, looks for it. Before the work every row shares one
  # checkout, where the marker stays, so the Then finding none fails.
  @ID-EVID-42 @slice-171
  Scenario Outline: Each missed opted-in check runs in a checkout of its own, never the one the hook's other checks share
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe mark marker.txt
            cost: static
            <first>
          - run: evidence-probe 0 marker.txt
            cost: static
            <second>
      """
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run found no "marker.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And the evidence store holds <records>

    Examples:
      | first         | second        | records            |
      | # no evidence | evidence: {}  | 1 complete record  |
      | evidence: {}  | # no evidence | 1 complete record  |
      | evidence: {}  | evidence: {}  | 2 complete records |

  # A4, A1. A declared untracked input is copied into the opted-in check's
  # checkout in the hook and hashed into its key, as by hand (@ID-EVID-18):
  # bug-112 copies only staged entries, which still holds for an undeclared
  # untracked file. A change to the declared input misses. Before the work
  # the hook's checkout lacks gen/seed.txt, so the first read Then fails.
  @ID-EVID-43 @slice-171
  Scenario: A declared untracked input reaches the hook's run and its key, and an undeclared one does not
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt notes.txt gen/seed.txt
            cost: static
            evidence: { inputs: [gen/seed.txt] }
      """
    And the committed file ".gitignore" holding "gen/"
    And the committed file "input.txt" holding "one"
    And the untracked file "gen/seed.txt" holding "seed"
    And the untracked file "notes.txt" holding "two"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the evidence probe's last run read "seed" from "gen/seed.txt"
    And the evidence probe's last run found no "notes.txt"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    Given the untracked file "gen/seed.txt" holding "next"
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run read "next" from "gen/seed.txt"

  # A4. Proposed: the key's timeout is the check's own, never
  # hooks.commit_msg.check_timeout. A run the cap stops in the hook is
  # failing, as today (@ID-CTASK-05), and not recorded, so the probe, which
  # the sleep keeps from being reached, never ran. itos task runs it to the
  # end under the ledger's 600 seconds and records it, and the hook then
  # hits that record although the run outlasted the cap. Before the work the
  # last attempt runs the check again and the cap stops it, exit 1.
  @ID-EVID-44 @slice-171
  Scenario: The key binds the check's own timeout, so a hand run's record answers a check the hook's cap would stop
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: sleep 3; evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And hooks.commit_msg.check_timeout is 1
    And the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "done"
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "failing T-001"
    And the evidence probe ran 0 times
    And the evidence store holds no record
    When itos runs "task T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time

  # A4, decided by the coordinator on review: within one hook run a missed
  # opted-in check is looked up again at each task's turn, so T-002 hits
  # the record T-001's run made a moment before, and the command runs once.
  # Before the work the hook runs each task's checks on its own, with no
  # run shared between them, so the probe runs twice.
  @ID-EVID-45 @slice-171
  Scenario: A missed check two named tasks share runs once in the hook, the later task hitting the earlier's record
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001, T-002
      """
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the evidence store holds 1 complete record
