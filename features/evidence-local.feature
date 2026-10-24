@phase-1
Feature: An opted-in check reruns from a local record when nothing it ran on changed
  Slice 169, work item A of docs/plans/evidence-records.md (ADR-0060), steps
  A1 and A3 as q-102 amended A3, and of A2 only what a miss needs: the store,
  its lock taken on a miss, and the write by rename. What the store does
  under contention or damage is slice-170's (evidence-store.feature). A4,
  the commit-msg hook's lookup, is slice-171's (evidence-hook.feature) and
  is not specified here.

  Opting in. A done_when check, or a run: entry of ci.steps, opts in with an
  evidence mapping; "evidence: {}" opts in with nothing declared.
  - inputs: paths from the repository's top. A folder covers every file
    under it, hashed as path and content, sorted. A path holding "*", "?"
    or "[" is a glob, which config check refuses. A declared path git
    tracks is taken from the index like the rest of the tree, so declaring
    it is harmless; an untracked or ignored one is hashed by content and
    copied into the checkout from the working folder; a missing one is
    hashed as absent, which is not the same as empty, and is not copied.
  - env: variable names, hashed by value.
  - tools: programs resolved on the check's PATH and hashed by content; a
    missing one is hashed as absent, and the check runs and fails as it
    would without it.
  A check without evidence runs exactly as today: in the working folder,
  nothing looked up or written, its JSON unchanged.

  Where it runs. itos task and ci run, run by hand or by ci run in CI alike,
  look an opted-in check up by the index tree. A miss runs it in a
  disposable checkout of that tree under the git common dir's
  itos/snapshots, bug-112's mechanism, one checkout per missed check and
  never shared between checks, so nothing one check writes reaches another's
  run or record. The checkout holds the declared inputs and no unstaged
  edit or other untracked file. A hit makes no checkout. With nothing
  staged the index tree is HEAD's. When the checkout cannot be made (the
  snapshots folder unusable, a gitlink in the index), the check runs
  uncached in the working folder, as --working-tree would, its verdict and
  exit code unchanged: never exit 3. The commit-msg hook keeps its own
  exit 3 for the same failure (bug-112), since a commit's checks must judge
  the staged tree or nothing. itos task --working-tree runs as today, in
  the working folder, nothing looked up or recorded. ci run takes no
  --working-tree: it refuses the flag as an unknown flag, as today.
  Which checks run, and how each is
  read (the ledger, the config, cost, timeout, run: or fails:), stay the
  working tree's, as today: only where an opted-in check runs and its key
  are new.

  The key binds the tree, the command and its timeout, the policy, itos's
  own version, the declared tools, the OS and architecture, the bound
  environment and the declared inputs.
  - Not run: or fails:. A record holds the raw exit code and the output
    tail, and each check derives its verdict from the exit, so a run: and
    a fails: on one command share one record, as check.Key shares one run
    between them today.
  - The policy is the loaded config's settings after defaults (its tree
    map), never its path, its stealth flag, its trees, its VCS or any other
    runtime field, so every worktree of a repository shares the records.
  - The bound environment is the declared env and, when ci run applies
    ci.env, those names and values.
  - The OS, the architecture and itos's version cannot be varied by a
    scenario: a unit test of the key over GOOS, GOARCH and the version, the
    implementer's, proves those parts.
  - An after: push check is never looked up or recorded.

  Contract, which these scenarios judge:
  - Exit codes and verdicts. A hit replays the recorded exit, read by the
    check's own run: or fails:, so the command exits as a run would. A
    checkout that cannot be made never changes a verdict or an exit code.
  - Files, all in the git common dir's itos/evidence, never in the tree. A
    record is the folder <key> holding manifest.json (JSON), result.json
    and the completion marker complete. A miss takes the lock <key>.lock
    (internal/lock's), runs, writes the record in a folder named
    .tmp-<key>-<unique> and renames it into place, never overwriting one.
  - JSON. In itos task --json each check that opted in gains evidence:
    source ("local"), hit, recorded, key, tree, recorded_at when a record
    exists, reason when it ran but recorded nothing (here snapshot-unusable;
    slice-170 adds lock-timeout, store-unusable and already-recorded; slice-169
    also gives unkeyable and inconclusive), and provenance, whose
    fields are not judged. recorded is true only on the occurrence that ran
    and wrote the record. A check one invocation runs once for several
    tasks reports, on each later occurrence, the evidence of the run it
    reused: the same key and hit, recorded false. Two checks share one run
    only when their command, timeout and evidence declaration all match
    (the same inputs, env and tools, or both without one), the mode staying
    out; checks that differ, one opted in and one not among them, are
    separate runs, each keyed and recorded by its own declaration, so
    check.Key gains the declaration. ci run --json gains
    evidence, a list of the same, each with its command, in run order. Both
    gain unstaged, the count of tracked paths whose working copy differs
    from the index (untracked files not counted), whenever an opted-in
    check was looked up. A run with no opted-in check has none of these.
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
  # The steps that judge the folder a run was in compare real paths, with
  # symlinks and short names resolved (macOS's /private/var, Windows's
  # RUNNER~1). evidence-probe mark <path> logs its run the same way, writes
  # <path> holding "mark" in the folder it runs in, and exits 0. "The kept
  # key" is the evidence key a --json run named.
  Background:
    Given a repository whose ledger has the task "T-001"
    And the evidence probe is on PATH, logging its runs outside the repository

  # A1. The JSON's evidence fields and the record are the contract; the
  # probe's log is what shows the hit. Before the work the JSON has no
  # evidence, so the first evidence Then fails.
  @ID-EVID-01 @slice-169
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
  # ledger and config, which itos reads as today. The declared input is the
  # folder gen, so a file added under it counts as a change. A row whose
  # change the key ignored fails at the second run; one whose new run went
  # unrecorded, at the third. Before the work the first run's JSON names no
  # key, so the Then that keeps it fails. The OS, architecture and itos's
  # version are the key's unit test's, not a row here.
  @ID-EVID-02 @slice-169
  Scenario Outline: A change to anything the key binds misses, and the new run is recorded
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt gen/seed.txt
            evidence: { inputs: [gen], env: [PROBE_FLAVOUR], tools: [evidence-probe] }
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
      | the untracked file "gen/extra.txt" holding "one"                                                                 |
      | the working-tree ledger's check of "T-001" is changed to run "evidence-probe 0 input.txt gen/seed.txt README.md" |
      | the config's shell is the recording shell                                                                        |
      | PROBE_FLAVOUR is "b"                                                                                             |
      | the evidence probe on PATH is rewritten with the same behaviour in different bytes                               |

  # A1. The plain check beside the opted-in one runs every time, in the
  # working folder, and its JSON has no evidence. That half passes today
  # (the guard of "a check without evidence behaves as today"); the opted-in
  # check's single run is what fails before the work.
  @ID-EVID-03 @slice-169
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
  @ID-EVID-04 @slice-169 @guard
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
  # exit 3 is done, exit 0 failing. The mode is not in the key; that a run:
  # record answers a fails: check is @ID-EVID-28's. Before the work the
  # second run runs the probe again.
  @ID-EVID-05 @slice-169
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
  # entry of ci.steps, and refuses, naming it, a key evidence does not have
  # and an input that is a glob. Today the ledger and the config refuse
  # evidence itself, so the first Then fails.
  @ID-EVID-06 @slice-169
  Scenario Outline: config check accepts evidence on a task's check and a CI step, and refuses a key it lacks or a glob
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: { inputs: [gen/seed.txt, gen], env: [PROBE_FLAVOUR], tools: [evidence-probe] }
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
            evidence: <evidence>
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "<named>"

    Examples:
      | evidence                         | named         |
      | { inptus: [gen/seed.txt] }       | inptus        |
      | { inputs: ["gen/*.txt"] }        | gen/*.txt     |
      | { inputs: ["gen/seed?.txt"] }    | gen/seed?.txt |
      | { inputs: ["gen/[ab].txt"] }     | gen/[ab].txt  |

  # A3. The miss runs in a disposable checkout, as the commit-msg hook's
  # checks do (bug-112). The hit makes none: with a regular file where
  # itos/snapshots must go, a run that tried could not make one and would
  # run the probe uncached in the working folder (@ID-EVID-23), which the
  # log would show. Before the work the first run is in the working
  # folder, so the checkout Then fails.
  @ID-EVID-13 @slice-169
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
  @ID-EVID-14 @slice-169
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
  @ID-EVID-15 @slice-169
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

  # A3. ci run keys a named task's opted-in check as itos task does: the
  # same index tree (HEAD's, nothing staged), command and policy, and with
  # no ci.env the same bound environment, so it hits the record itos task
  # left. ITOS_FROM and ITOS_TO, which ci run gives every check, are no
  # declared env and are not in the key; ci.env is (@ID-EVID-27). Before the
  # work ci run runs the probe again.
  @ID-EVID-16 @slice-169
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
  @ID-EVID-17 @slice-169
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
  # file never reaches it, even one the check declares as an input (a
  # tracked input is the index's), nor does an untracked file it would read
  # but does not declare; an ignored input it declares is copied in. Only
  # tracked paths count as not staged: the JSON says 1 and the text says
  # so. Before the work the probe reads the unstaged "two".
  @ID-EVID-18 @slice-169
  Scenario: An unstaged edit and an undeclared untracked file never reach the run, and the unstaged edit is counted
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt notes.txt gen/seed.txt
            evidence: { inputs: [gen/seed.txt, input.txt] }
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
  @ID-EVID-19 @slice-169
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
  @ID-EVID-20 @slice-169
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
  @ID-EVID-21 @slice-169
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
  @ID-EVID-22 @slice-169
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

  # A3. A checkout that cannot be made is no reason to fail a check run by
  # hand: it runs uncached in the working folder, where it sees the
  # unstaged edit, as --working-tree would, and its exit is its own. The
  # hook still refuses the same states with exit 3 (@ID-HOOKSNAP-09).
  # Under a gitlink no repository with a submodule could otherwise run an
  # opted-in check by hand. Before the work the JSON has no evidence, so
  # its reason fails.
  @ID-EVID-23 @slice-169
  Scenario Outline: A checkout that cannot be made runs the check uncached in the working folder, never exit 3
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
    And the file "input.txt" is changed to hold "two"
    And <problem>
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And its JSON's "tasks" has one entry whose "status" is "<status>"
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in the working folder
    And its JSON's evidence for the task "T-001" has "recorded" false
    And its JSON's evidence for the task "T-001" has "reason" "snapshot-unusable"
    And the evidence store holds no record
    When itos runs "task T-001 --json"
    Then itos exits with code <exit>
    And the evidence probe ran 2 times

    Examples:
      | problem                                                                     | code | exit | status  |
      | the index contains a gitlink at "vendor/submodule" pointing to HEAD         | 0    | 0    | done    |
      | the fixture writes a regular file at the git common dir's "itos/snapshots"  | 1    | 1    | failing |

  # A1, A3. A declared input that is missing is hashed as absent and not
  # copied in, so the probe finds none; an empty file in its place is a
  # different input and misses; removing it again is the first input once
  # more, and hits the first record. Before the work the JSON has no
  # evidence, so the second run's evidence Then fails.
  @ID-EVID-24 @slice-169
  Scenario: A missing declared input is hashed as absent, not as empty, and is not copied in
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 gen/seed.txt
            evidence: { inputs: [gen/seed.txt] }
      """
    And the committed file ".gitignore" holding "gen/"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe's last run found no "gen/seed.txt"
    Given the empty untracked file "gen/seed.txt"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" false
    And the evidence probe's last run read "" from "gen/seed.txt"
    Given the working-tree copy of "gen/seed.txt" is missing
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "hit" true

  # A1. A declared tool the PATH lacks is hashed as absent: the check runs
  # and fails as the shell makes it (127, command not found, which every
  # sh reports alike), and that failure is recorded and replayed like any
  # other. Before the work the second run runs the probe again.
  @ID-EVID-25 @slice-169
  Scenario: A declared tool missing from PATH is hashed as absent, and the check fails as it would without it
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt; no-such-evidence-tool
            evidence: { tools: [no-such-evidence-tool] }
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 --json"
    Then itos exits with code 1
    And its JSON's "tasks" has one entry whose "status" is "failing"
    When itos runs "task T-001 --json"
    Then itos exits with code 1
    And its JSON's "tasks" has one entry whose "status" is "failing"
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true

  # A3. The policy is the config's settings, not its path, and the store is
  # the git common dir's, so a linked worktree on the same tree hits the
  # record the main checkout made. The worktree is added once the base is
  # committed, so it checks out the same tree. Before the work the
  # worktree's run runs the probe again.
  @ID-EVID-26 @slice-169
  Scenario: A linked worktree on the same tree hits the record the main checkout made
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
    And a linked worktree of the repository at "../wt"
    And itos has run "task T-001 --json"
    When itos runs the command line "task T-001 --json" in the linked worktree
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "hit" true

  # A3. ci.env is bound environment when ci run applies it, and itos task
  # does not apply it: with a ci.env value, ci run's key differs from the
  # record itos task left, so it misses, and its own record is hit next
  # time. The config holds ci.env for both runs, so the policy is the same
  # and only the bound environment differs. Before the work the JSON has
  # no evidence, so the hit Then fails.
  @ID-EVID-27 @slice-169
  Scenario: With ci.env, ci run misses the record a hand-run itos task left, and records its own
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: {}
      """
    And the config sets "ci.env.PROBE_CI" to "1"
    And the committed file "input.txt" holding "one"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    And itos has run "task T-001"
    When itos runs CI over the commits after the first with --json
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the CI command "evidence-probe 0 input.txt" has "hit" false
    When itos runs CI over the commits after the first with --json
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the CI command "evidence-probe 0 input.txt" has "hit" true

  # A1, A3. One command listed as run: by T-001 and fails: by T-002 is one
  # check with one record, keyed without the mode, each task deriving its
  # verdict from the recorded exit 3. In the invocation that runs it, T-002
  # reuses T-001's run and reports its evidence: the same key and hit,
  # recorded false. A later invocation for T-002 alone hits the record T-001's
  # run: made. Before the work the JSON has no evidence.
  @ID-EVID-28 @slice-169
  Scenario: A run: and a fails: on one command share one record, and a reused occurrence reports the run it reused
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 3
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - fails: evidence-probe 3
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 T-002 --json"
    Then itos exits with code 1
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-001" has "recorded" true
    And its JSON's evidence for the task "T-002" has "recorded" false
    And its JSON's evidence for the task "T-002" has "hit" false
    And its JSON's evidence for the tasks "T-001" and "T-002" has the same key
    And the evidence store holds 1 complete record
    When itos runs "task T-002 --json"
    Then itos exits with code 0
    And its JSON's "tasks" has one entry whose "status" is "done"
    And the evidence probe ran 1 time
    And its JSON's evidence for the task "T-002" has "hit" true

  # A3. Each missed check gets a checkout of its own, never one the
  # invocation shares as the hook's checks share theirs (@ID-HOOKSNAP-07),
  # so what one check writes never reaches another's run or record. T-001's
  # check writes marker.txt where it runs; T-002's, a different check,
  # looks for it. Before the work both run in the working folder, where the
  # marker stays, and a shared checkout would keep it too, so the Then
  # finding none fails.
  @ID-EVID-29 @slice-169
  Scenario: Each missed check runs in a checkout of its own, so one check's files never reach another's run
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe mark marker.txt
            evidence: {}
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 marker.txt
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 T-002 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run found no "marker.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And the evidence store holds 2 complete records

  # A1, A3. The evidence declaration is part of check.Key: one command
  # whose declarations differ is two checks, each run and each keyed and
  # recorded by its own declaration. A check with no declaration runs as
  # today and is never recorded; one declaring other inputs gets its own
  # key and record. Neither declared input exists, so both are hashed as
  # absent and differ only by path. Before the work check.Key shares one run
  # between them, so the probe runs once and that Then fails.
  @ID-EVID-30 @slice-169
  Scenario Outline: One command declared differently by two tasks is two runs, each keyed by its own declaration
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            evidence: { inputs: [gen/a.txt] }
      - id: T-002
        type: chore
        title: Trim
        done_when:
          - run: evidence-probe 0 input.txt
            <declaration>
      """
    And the committed file "input.txt" holding "one"
    When itos runs "task T-001 T-002 --json"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And its JSON's evidence for the task "T-001" has "recorded" true
    And <second>
    And the evidence store holds <records>

    Examples:
      | declaration                       | second                                                                           | records            |
      | # no evidence                     | its JSON gives the task "T-002"'s check "evidence-probe 0 input.txt" no evidence | 1 complete record  |
      | evidence: { inputs: [gen/b.txt] } | its JSON's evidence for the tasks "T-001" and "T-002" has different keys         | 2 complete records |
