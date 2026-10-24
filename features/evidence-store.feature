@phase-1
Feature: The local evidence store survives contention and damage
  Slice 170, step A2 of docs/plans/evidence-records.md (ADR-0060), split
  from slice-169 on its spec review. Slice 169 (evidence-local.feature)
  looks records up, takes a record's lock on a miss and writes the record
  by rename; this file is what the store does when another itos holds the
  lock, when the store cannot be written, and when what it finds is not a
  record it can trust. The rules of evidence-local.feature's description
  hold here unchanged.

  Contract, which these scenarios judge:
  - A lock not taken, or a store that cannot be written, never changes a
    verdict or an exit code: the check runs uncached, still in its
    disposable checkout of the index tree (only a checkout that cannot be
    made falls back to the working folder, slice-169's snapshot-unusable),
    its JSON's evidence saying recorded false and the reason, lock-timeout
    or store-unusable. "Fail closed" means never trusting the record.
    slice-169 already runs these cases so; this slice pins them.
  - Files, in the git common dir's itos/evidence. The lock is <key>.lock,
    internal/lock's, waited for 10 seconds; a writer that waited looks the
    record up again before it runs. Lookup takes no lock. A record is
    written in a folder named .tmp-<key>-<unique>, which no reader ever
    reads, and renamed into place, never overwritten. Before the rename, and
    again if it fails, the writer judges what is at the destination, the same
    on every OS (Windows renames over a file where Unix refuses): a complete record that matches its
    key is kept as it is, and the writer, whose check has run, reports its
    own verdict with hit false, recorded false, reason already-recorded and
    recorded_at the kept record's time; anything else is quarantined. A final record
    without its completion marker, or whose manifest does not parse or does
    not match its key, is renamed to <key>.<reason>-<UTC timestamp>, the
    reason markerless or corrupt, and the check reruns and records.

  # The evidence probe is evidence-local.feature's: each run appends its
  # arguments as one line to a log outside the repository, so a hit is a
  # run the log does not gain. It also carries out an order the fixture
  # left it (put something at the store, once). "The kept key" is the
  # evidence key a --json run named, which the fixture keeps to find that
  # record's files.
  Background:
    Given a repository whose ledger has the task "T-001"
    And the evidence probe is on PATH, logging its runs outside the repository

  # A2. The fixture plays the writer that got there first: it holds the
  # record's lock with the record moved aside, so the second itos misses
  # and must wait. The release step fails if that itos has already exited,
  # so a run that did not wait cannot pass; it waits a second first. Once
  # it holds the lock, itos looks again, finds the complete record and runs
  # nothing. slice-169 already does this, so the scenario pins it.
  @ID-EVID-07 @slice-170
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
  # internal/lock's message says. slice-169 already does this, so the
  # scenario pins it.
  @ID-EVID-08 @slice-170
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
  # check's: no exit 3, as the hook's snapshot failure would be. Each run runs
  # uncached, and the file is left as it was. slice-169 already does this,
  # so the scenario pins it.
  @ID-EVID-09 @slice-170
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
  # first row, which only pins slice-169, passes; the others fail at the
  # record's being complete.
  @ID-EVID-10 @slice-170
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
    And its JSON's evidence for the task "T-001" <evidence>
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" true

    Examples:
      | planted                                            | record                             | quarantine                                             | evidence                                                    |
      | the record moved aside                             | is the one the fixture moved aside | holds no quarantined entry                             | has "reason" "already-recorded" and the kept record's time  |
      | a regular file                                     | is complete                        | holds the kept key's entry quarantined as "corrupt"    | has "recorded" true and no "reason"                         |
      | the record moved aside, less its completion marker | is complete                        | holds the kept key's entry quarantined as "markerless" | has "recorded" true and no "reason"                         |

  # A2. A final record found broken at lookup is a miss: it is renamed
  # aside under its reason, the check reruns, and a fresh record takes its
  # place, which the next run hits. The damage is portable: a removed file,
  # a NUL byte, a regular file where the record's folder must be. Before
  # the work no entry is quarantined, so the Then that checks one fails.
  @ID-EVID-11 @slice-170
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
  # old temporary folders is not decided here. slice-169 already does
  # this, so the scenario pins it.
  @ID-EVID-12 @slice-170
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
