@phase-1
Feature: An opted-in check reruns from a local record when nothing it ran on changed
  Slice 169, work item A of docs/plans/evidence-records.md (ADR-0060), steps
  A1 to A3 as q-102 amended A3. A4, the commit-msg hook's lookup, is
  p1-evidence-commit-msg and is not specified here.

  Opting in. A done_when check, or a run: entry of ci.steps, opts in with an
  evidence mapping: inputs, paths the tree does not hold, hashed by content
  (a missing one hashed as missing); env, variable names, hashed by value;
  tools, programs resolved on the check's PATH and hashed by content.
  "evidence: {}" opts in with none of them. A check without evidence runs
  exactly as today: in the working folder, nothing looked up or written,
  its JSON unchanged.

  Where it runs. Hand-run itos task and ci run look an opted-in check up by
  the index tree. A miss runs it in a disposable checkout of that tree
  under the git common dir's itos/snapshots (bug-112's), with the declared
  inputs copied in from the working folder, and no unstaged edit and no
  other untracked file; a hit makes no checkout. With nothing staged the
  index tree is HEAD's. --working-tree runs as today, in the working
  folder, nothing looked up or recorded. Which checks run and how each is
  read (the ledger, the config, cost, timeout, run: or fails:) stay the
  working tree's, as today: only where an opted-in check runs and its key
  are new.

  The key binds the tree, the command, run: or fails:, the timeout, the
  loaded config with its defaults, itos's own version, the declared tools,
  the OS and architecture, the declared env values and the declared
  inputs. An after: push check is never looked up or recorded.

  Contract, which these scenarios judge:
  - Exit codes and verdicts. A hit replays the recorded verdict, so the
    command exits as the run that made the record did. A store that cannot
    be used never changes a verdict or an exit code: the check runs
    uncached, as today.
  - Files, all in the git common dir's itos/evidence, never in the tree. A
    record is the folder <key> holding manifest.json (JSON, naming the OS
    and architecture by Go's names), result.json and the completion marker
    complete. It is written in a folder named .tmp-<key>-<unique> and
    renamed into place, never overwritten. The lock is <key>.lock,
    internal/lock's, waited for 10 seconds. A final record without its
    marker, or whose manifest does not parse or does not match its key, is
    renamed to <key>.<reason>-<UTC timestamp>, the reason markerless or
    corrupt, and the check reruns and records.
  - JSON. In itos task --json each check that opted in gains evidence:
    source ("local"), hit, recorded, key, tree, recorded_at when recorded,
    reason when it ran but recorded nothing (lock-timeout or
    store-unusable), and provenance, whose fields are not judged. ci run
    --json gains evidence, a list of the same, each with its command, in
    run order. Both gain unstaged, the count of tracked paths whose working
    copy differs from the index (untracked files not counted), whenever an
    opted-in check was looked up. A run with no opted-in check has none of
    these.
  - Loose text, judged by a substring at most. A hit says "cached evidence",
    a miss "running", and a run on the index that leaves changes out says
    how many changed paths were "not staged".

  # evidence-probe <exit> [<path>...] is a program the steps put on PATH.
  # Each run appends its arguments as one line to a log outside the
  # repository and records the folder it ran in and, for each path, its
  # text less the final newline, or that it is absent. It prints the same
  # as "<path>: <text>" lines, carries out an order the fixture left it
  # (put something at the store, once), and exits <exit>. Its log and
  # orders are outside the tree and are no declared input, so a hit is a
  # run the log does not gain, never one inferred from speed or wording.
  # "The kept key" is the evidence key a --json run named, which the
  # fixture keeps to find that record's files.
  Background:
    Given a repository whose ledger has the task "T-001"
    And the evidence probe is on PATH, logging its runs outside the repository

  # A1. The JSON's evidence fields and the record are the contract; the
  # probe's log is what shows the hit. Before the work the JSON has no
  # evidence, so the first evidence Then fails.
  @ID-EVID-01 @slice-169 @wip
  Scenario: An opted-in check's repeat answers from its record, its JSON saying so
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" false
    And its JSON's evidence for the task "T-001" has "recorded" true
    And its JSON's evidence for the task "T-001" has "source" "local"
    And its JSON's "unstaged" is 0
    And the evidence store holds 1 complete record
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true
    And its JSON's "tasks" has one entry whose "status" is "done"

  # A1. Each row changes one thing the key binds and nothing else. The
  # command, policy, env and toolchain rows leave the index tree as it was:
  # the command and the policy change unstaged, in the working tree's
  # ledger and config, which itos reads as today. A row whose change the
  # key ignored fails at the second run; one whose new run went unrecorded,
  # at the third. Before the work the first run's JSON names no key, so the
  # Then that keeps it fails.
  # Proposed: tools declares the toolchain (no plan text says how itos
  # learns it), and the OS row is observed through the record: a manifest
  # naming another OS no longer matches its key and is never used.
  @ID-EVID-02 @slice-169 @wip
  Scenario Outline: A change to anything the key binds misses, and the new run is recorded
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt gen/seed.txt
            evidence: { inputs: [gen/seed.txt], env: [PROBE_FLAVOUR], tools: [evidence-probe] }
      """
    And the committed file "input.txt" holding "one"
    And the untracked file "gen/seed.txt" holding "one"
    And PROBE_FLAVOUR is "a"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given <change>
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" false
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" true

    Examples:
      | change                                                                                                           |
      | "input.txt" is staged with the text "two"                                                                        |
      | the untracked file "gen/seed.txt" holding "two"                                                                  |
      | the working-tree ledger's check of "T-001" is changed to run "evidence-probe 0 input.txt gen/seed.txt README.md" |
      | the config's shell is the recording shell                                                                        |
      | PROBE_FLAVOUR is "b"                                                                                             |
      | the evidence probe on PATH is rewritten with the same behaviour in different bytes                               |
      | the fixture makes the kept key's manifest name another operating system                                          |

  # A1. The plain check beside the opted-in one runs every time, in the
  # working folder, and its JSON has no evidence. That half passes today
  # (the guard of "a check without evidence behaves as today"); the opted-in
  # check's single run is what fails before the work.
  @ID-EVID-03 @slice-169 @wip
  Scenario: A check that does not opt in runs every time, as today, beside one that does
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
          - run: evidence-probe 0 README.md
      """
    And the committed file "input.txt" holding "one"
    And itos has run "task T-001 --json"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran "0 input.txt" 1 time
    And the evidence probe ran "0 README.md" 2 times
    And the evidence probe's last run was in the working folder
    And its JSON gives the task "T-001"'s check "evidence-probe 0 README.md" no evidence

  # Guard: passes before the work and must keep passing. A project that opts
  # nothing in sees today's JSON (no unstaged count, no evidence) and gets no
  # store, even with an unstaged edit that would be counted.
  @ID-EVID-04 @slice-169 @guard @wip
  Scenario: With no opted-in check, itos task --json is today's and nothing is stored
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
      """
    And the committed file "input.txt" holding "one"
    And the file "input.txt" is changed to hold "two"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe's last run read "two" from "input.txt"
    And its JSON has no "unstaged"
    And its JSON gives the task "T-001"'s check "evidence-probe 0 input.txt" no evidence
    And the evidence store holds no record

  # A1. A record keeps the raw exit code, and fails: reads it its own way:
  # exit 3 is done, exit 0 failing. Run: or fails: is part of the key, as
  # the plan says, so a run: record never answers a fails: check. Before the
  # work the second run runs the probe again.
  @ID-EVID-05 @slice-169 @wip
  Scenario Outline: A fails: check replays its own verdict from its record
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - fails: evidence-probe <code>
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's "tasks" has one entry whose "status" is "<status>"
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's "tasks" has one entry whose "status" is "<status>"
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true

    Examples:
      | code | exit | status  |
      | 3    | 0    | done    |
      | 0    | 1    | failing |

  # A1. config check accepts evidence on a done_when check and on a run:
  # entry of ci.steps, and refuses a key evidence does not have, naming it.
  # Today the ledger and the config refuse evidence itself, so the first
  # Then fails.
  @ID-EVID-06 @slice-169 @wip
  Scenario: config check accepts evidence on a task's check and a CI step, and refuses a key it does not have
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: { inputs: [gen/seed.txt], env: [PROBE_FLAVOUR], tools: [evidence-probe] }
      """
    And the CI steps are the YAML:
      """
      - { run: evidence-probe 0 input.txt, cost: static, evidence: { inputs: [gen/seed.txt] } }
      """
    When itos checks the config
    Then itos exits with code 0
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: { inptus: [gen/seed.txt] }
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "inptus"

  # A2. The fixture plays the writer that got there first: it holds the
  # record's lock with the record moved aside, so the second itos misses
  # and must wait. The release step fails if that itos has already exited,
  # so a run that did not wait cannot pass; it waits a second first. Once
  # it holds the lock, itos looks again, finds the complete record and runs
  # nothing. Before the work the first run's JSON names no key, so the Then
  # that keeps it fails.
  @ID-EVID-07 @slice-169 @wip
  Scenario: A writer waiting on the lock reads the record the holder completed, and runs nothing
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given the fixture moves the kept key's record aside
    And the fixture makes the kept key's lock file
    When itos runs "task T-001 --json" in the background
    And while that itos still waits, the fixture puts the kept key's record back and removes its lock file
    And the background itos finishes
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true

  # A2. A lock no itos will release (one killed while holding it) costs the
  # 10-second wait, then the check runs uncached. "Fail closed" means never
  # trusting the record, not failing the check: each row's verdict and exit
  # are the check's own. The lock stays, for the person to remove, as
  # internal/lock's message says. Before the work the first run's JSON
  # names no key, so the Then that keeps it fails.
  @ID-EVID-08 @slice-169 @wip
  Scenario Outline: A stale lock never changes the verdict: the check runs uncached
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe <code> input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given the fixture moves the kept key's record aside
    And the fixture makes the kept key's lock file
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's "tasks" has one entry whose "status" is "<status>"
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "recorded" false
    And its JSON's evidence for the task "T-001" has "reason" "lock-timeout"
    And the evidence store holds no record
    And the kept key's lock file is still there

    Examples:
      | code | exit | status  |
      | 0    | 0    | done    |
      | 1    | 1    | failing |

  # A2. A store that cannot be made (a regular file where its folder must
  # go, which no OS lets a folder be made under) is never an error of the
  # check's: no exit 3, as a snapshot failure would be. Each run runs
  # uncached, and the file is left as it was. Before the work the JSON has
  # no evidence, so its reason fails.
  @ID-EVID-09 @slice-169 @wip
  Scenario Outline: A store that cannot be written never changes the verdict: the check runs uncached
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe <code> input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the fixture writes a regular file at the git common dir's "itos/evidence"
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's evidence for the task "T-001" has "reason" "store-unusable"
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's "tasks" has one entry whose "status" is "<status>"
    And the evidence probe ran 2 times
    And the git common dir's "itos/evidence" is still the fixture's regular file

    Examples:
      | code | exit | status  |
      | 0    | 0    | done    |
      | 1    | 1    | failing |

  # A2. The check itself puts something at its own record's place while it
  # runs, after itos's lookup missed: another writer won the race. Renaming
  # the finished temporary folder onto it fails on every OS: onto a folder
  # that is not empty, or onto a regular file. (Never an empty folder, which
  # a POSIX rename replaces.) itos then judges what is there: a complete,
  # matching record is kept as it is and used; anything else is quarantined
  # under its reason and replaced by this run's record. Either way no
  # temporary folder is left, and the third run hits. Before the work the
  # first run's JSON names no key, so the Then that keeps it fails.
  @ID-EVID-10 @slice-169 @wip
  Scenario Outline: A rename that fails judges what reached the destination before using it
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given the fixture moves the kept key's record aside
    And the check, as it runs, will put <planted> at the kept key
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the kept key's record <record>
    And the evidence store <quarantine>
    And the evidence store holds no temporary folder
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" true

    Examples:
      | planted                                            | record                             | quarantine                                             |
      | the record moved aside                             | is the one the fixture moved aside | holds no quarantined entry                             |
      | a regular file                                     | is complete                        | holds the kept key's entry quarantined as "corrupt"    |
      | the record moved aside, less its completion marker | is complete                        | holds the kept key's entry quarantined as "markerless" |

  # A2. A final record found broken at lookup is a miss: it is renamed
  # aside under its reason, the check reruns, and a fresh record takes its
  # place, which the next run hits. The damage is portable: a removed file,
  # a NUL byte, a regular file where the record's folder must be. Before
  # the work the first run's JSON names no key, so the Then that keeps it
  # fails.
  @ID-EVID-11 @slice-169 @wip
  Scenario Outline: A corrupt or markerless record is quarantined, and the check reruns and records
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given <damage>
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence store holds the kept key's entry quarantined as "<reason>"
    And the kept key's record is complete
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times

    Examples:
      | damage                                                                | reason     |
      | the fixture removes the completion marker from the kept key's record | markerless |
      | the fixture writes a NUL byte into the kept key's manifest            | corrupt    |
      | the fixture replaces the kept key's record with a regular file        | corrupt    |

  # A2. A temporary folder is a write not yet renamed into place, so a
  # reader never takes it for a record, even one holding a whole record:
  # the check reruns and records under its key. Whether itos ever cleans
  # old temporary folders is not decided here. Before the work the first
  # run's JSON names no key, so the Then that keeps it fails.
  @ID-EVID-12 @slice-169 @wip
  Scenario: An unfinished temporary folder is never read as a record
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has a key, which the fixture keeps
    Given the fixture moves the kept key's record into an unfinished temporary folder of the store
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" false
    And the kept key's record is complete

  # A3. The miss runs in a disposable checkout, as the commit-msg hook's
  # checks do (bug-112). The hit makes none: with a regular file where
  # itos/snapshots must go, a run that tried would stop with exit 3
  # (@ID-HOOKSNAP-09's refusal). Before the work the first run is in the
  # working folder, so the checkout Then fails.
  @ID-EVID-13 @slice-169 @wip
  Scenario: A miss runs in a checkout of the staged tree, and the repeat answers without one
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And its output says "running"
    Given the fixture writes a regular file at the git common dir's "itos/snapshots"
    When itos runs "task T-001"
    Then itos exits with code 0
    And its output lists "T-001" as "done"
    And the evidence probe ran 1 time
    And its output says "cached evidence"

  # A3. Failed results are reusable: the hit is failing, exit 1, and prints
  # the recorded output tail as the run printed the output. Before the work
  # the second run runs the probe again.
  @ID-EVID-14 @slice-169 @wip
  Scenario: A cached failure keeps its result, its exit code and its output
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 1 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001"
    Then itos exits with code 1
    And its output lists "T-001" as "failing"
    When itos runs "task T-001"
    Then itos exits with code 1
    And its output lists "T-001" as "failing"
    And the evidence probe ran 1 time
    And its output says "input.txt: one"
    And its output says "cached evidence"

  # A3. Two tasks listing one opted-in check share one run, as today
  # (@ID-TASK-01), and one record, which the next invocation hits for both.
  # Before the work the second invocation runs the probe again.
  @ID-EVID-15 @slice-169 @wip
  Scenario: A check two tasks share runs once and is recorded once
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And itos has run "task T-001 T-002"
    When itos runs the tasks "T-001" and "T-002"
    Then itos exits with code 0
    And its output lists "T-001" as "done"
    And its output lists "T-002" as "done"
    And the evidence probe ran 1 time
    And the evidence store holds 1 complete record

  # A3. ci run, by hand, keys a named task's opted-in check as itos task
  # does: the same index tree (HEAD's, nothing staged), command and policy,
  # so it hits the record itos task left. ITOS_FROM and ITOS_TO, which ci
  # run gives every check, are no declared env and are not in the key.
  # Before the work ci run runs the probe again.
  @ID-EVID-16 @slice-169 @wip
  Scenario: ci run answers a named task's check from the record a hand-run itos task left
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    And itos has run "task T-001"
    When itos runs CI over the commits after the first with --json
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the CI command "evidence-probe 0 input.txt" has "hit" true

  # A3. A run: entry of ci.steps opts in as a task's check does; a step
  # written as a plain string cannot, and runs every time. Before the work
  # the opted-in step runs twice.
  @ID-EVID-17 @slice-169 @wip
  Scenario: An opted-in CI step reruns from its record, and a plain step runs every time
    Given the CI steps are the YAML:
      """
      - { run: evidence-probe 0 input.txt, cost: static, evidence: {} }
      - evidence-probe 0 README.md
      """
    And the committed file "input.txt" holding "one"
    And itos has run "ci run --json"
    When itos runs CI over the commits after the first with --json
    Then itos exits with code 0
    And the evidence probe ran "0 input.txt" 1 time
    And the evidence probe ran "0 README.md" 2 times
    And its JSON's evidence for the CI command "evidence-probe 0 input.txt" has "hit" true
    And its JSON's "unstaged" is 0

  # A3, q-102. The run is on the staged tree: an unstaged edit to a tracked
  # input and an untracked file the check would read never reach it, while
  # an ignored input the check declares does. Only tracked paths count as
  # not staged: the JSON says 1 and the text says so. Before the work the
  # probe reads the unstaged "two".
  # Proposed: declared inputs are copied into the checkout (else a declared
  # untracked input could never be read there); untracked files are not
  # counted (the plan says "changed paths ... not staged").
  @ID-EVID-18 @slice-169 @wip
  Scenario: An unstaged edit and an undeclared untracked file never reach the run, and the unstaged edit is counted
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt notes.txt gen/seed.txt
            evidence: { inputs: [gen/seed.txt] }
      """
    And the committed file ".gitignore" holding "gen/"
    And the committed file "input.txt" holding "one"
    And the untracked file "gen/seed.txt" holding "seed"
    And the file "input.txt" is changed to hold "two"
    And the untracked file "notes.txt" holding "two"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the evidence probe's last run read "one" from "input.txt"
    And the evidence probe's last run found no "notes.txt"
    And the evidence probe's last run read "seed" from "gen/seed.txt"
    And its JSON's "unstaged" is 1
    When itos runs "task T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output says "not staged"

  # A3, q-102. The same edit or file made after a clean run leaves the key
  # as it was, so the next run hits: the next commit holds the same tree.
  # Before the work the second run runs the probe again.
  @ID-EVID-19 @slice-169 @wip
  Scenario Outline: An unstaged edit or an unrelated untracked file does not change the key
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt notes.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And itos has run "task T-001 --json"
    And <change>
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true
    And its JSON's "unstaged" is <unstaged>

    Examples:
      | change                                        | unstaged |
      | the file "input.txt" is changed to hold "two" | 1        |
      | the untracked file "notes.txt" holding "two"  | 0        |

  # A3. --working-tree is today's run, kept for a check that must see the
  # working folder: it reads the unstaged edit there, looks nothing up and
  # records nothing, so each run runs. Its JSON is today's. Before the work
  # the flag is unknown, exit 2.
  # Proposed: ci run takes --working-tree too, with the same meaning.
  @ID-EVID-20 @slice-169 @wip
  Scenario: --working-tree runs in the working folder, sees the unstaged edit and records nothing
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the file "input.txt" is changed to hold "two"
    When itos runs "task T-001 --working-tree --json"
    Then itos exits with code 0
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in the working folder
    And its JSON gives the task "T-001"'s check "evidence-probe 0 input.txt" no evidence
    And the evidence store holds no record
    When itos runs "task T-001 --working-tree"
    Then itos exits with code 0
    And the evidence probe ran 2 times

  # A3. An after: push check is never reusable: with no push it stays
  # pending and runs nothing, even though T-002's record of the same
  # command is there to be found. That half is a guard, passing today; the
  # hit for T-002 is what fails before the work.
  @ID-EVID-21 @slice-169 @wip
  Scenario: An opted-in after: push check stays pending beside a record of its command
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            after: push
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And itos has run "task T-002 --json"
    When itos runs "task T-001 T-002 --json"
    Then itos exits with code 0
    And its JSON's "tasks" has one entry whose "status" is "pending"
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-002" has "hit" true
    And its JSON gives the task "T-001"'s check "evidence-probe 0 input.txt" no evidence
    And the evidence store holds 1 complete record

  # A3, q-102. The key's tree is the index's: HEAD's when nothing is
  # staged, an unstaged edit making no difference, and the index's own
  # once a change is staged. Before the work the JSON has no evidence.
  # Proposed: evidence.tree is the tree's object name, as git write-tree
  # prints it.
  @ID-EVID-22 @slice-169 @wip
  Scenario Outline: The key's tree is the index's: HEAD's when nothing is staged
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And <staging>
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" names <tree>

    Examples:
      | staging                                        | tree                                 |
      | the file "README.md" is changed to hold "edit" | HEAD's tree                          |
      | a change to "README.md" is staged              | the index tree, which is not HEAD's  |
