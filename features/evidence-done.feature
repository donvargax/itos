@phase-1
Feature: work done runs a task's static checks through the evidence engine and names its late ones as left to CI
  itos work done runs the static checks of a task it closes, every one of
  them in written order, a static check written below a late one included,
  and stops at the first failure, refusing the close, nothing written. A
  check's class is the cost rule's, so with ci.cost.keep_written_order a
  check below a late one is late. Each runs under its own timeout, its
  timeout: or ledger.check.timeout; hooks.commit_msg.check_timeout caps the
  hook alone. Late checks are CI's, which runs every check of the tasks a
  push names: work done never runs them, and names them as left to CI.
  Future work: once p1-evidence-ci lands, work done checks their CI record.
  The order of work done's refusals is unchanged: the item's scenarios that
  are still @wip, then commits no remote has, then the task's static checks,
  then the CI watch, so a failing check refuses before CI is asked.

  The tree. work done judges landed work, so an opted-in check is keyed by
  HEAD's tree, never the index's, and a miss runs in a checkout of HEAD's
  tree of its own under the git common dir's itos/snapshots. A staged change
  or an unstaged edit never reaches that run or its key, and work done says
  nothing of them. With nothing staged, HEAD's tree is the tree a hand-run
  itos task keys, and it is the tree the commit-msg hook keyed for the
  commit that made HEAD, so a hand run, the hook and work done each pay
  once. A checkout of HEAD's tree that cannot be made (a gitlink in it, the
  snapshots folder unusable) runs the check uncached in the working folder,
  as a hand run does, its exit its own, never the hook's exit 3. Everything
  else of the key, the store and the record is evidence-local.feature's.
  These checks are local feedback, as a hand run's are (ADR-0060): a record
  answers work done and proves nothing to CI.

  A check without evidence runs as today: in the working folder, in the
  commit-msg hook's environment, nothing looked up or written. An after: push
  check stays pending at work done, as today: neither run nor looked up.

  Contract, which these scenarios judge:
  - Exit codes. A failing static check, run or answered from its record,
    refuses with exit 1 and the rule work-done-task-check, the item left as
    it was.
  - Files: the records of evidence-local.feature, in the git common dir's
    itos/evidence. A miss at work done writes one.
  - Text, judged by a substring at most, on stderr under --json as work
    done's watch lines are. A failure prints the task, its title, the
    command and the output tail, as itos task prints a failure, and the
    problem's message names the command and its exit or its timeout. A hit
    says "cached evidence". Each late check is named on a line saying it is
    "left to CI".
  - JSON, proposed. The work-done-task-check problem gains task, command,
    exit (null when the check did not exit by itself), timed_out, and
    output, the tail a record keeps. Once an opted-in check was looked up,
    a close's or a refusal's JSON gains evidence, ci run's list: each entry
    the command and its evidence, in run order. A close gains left_to_ci,
    [{task, command}] for each late check, when the task has one. A run with
    neither has today's JSON.

  # evidence-probe is evidence-local.feature's: a hit is a run its log does
  # not gain. itos runs in a clone, and each scenario pushes its own setup
  # when ci.watch is set, so the item's commits are on the remote.
  Background:
    Given a repository whose ledger has the task "T-001"
    And the evidence probe is on PATH, logging its runs outside the repository
    And a clone of it, where itos runs

  # The hand run on a clean index keys HEAD's tree, which work done keys.
  @ID-EVID-46 @slice-173
  Scenario: work done answers an opted-in check from the record a hand run on the same tree left
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And itos has run "task T-001"
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output says "cached evidence"
    And the registry's item "T-001" has the status "done" and the owner "someone"

  # The hand run keys the index, which differs from HEAD, so work done misses.
  @ID-EVID-47 @slice-173
  Scenario: A miss at work done runs on HEAD's tree, never the index or the working copy, and records
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And "input.txt" is staged with the text "two"
    And the working-tree copy of "input.txt" is replaced with "three"
    And itos has run "task T-001"
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run read "one" from "input.txt"
    And the evidence probe's last run was in a disposable checkout under the git common dir's "itos/snapshots"
    And the evidence store holds 2 complete records

  # Failed results are reusable at work done as by hand.
  @ID-EVID-48 @slice-173
  Scenario: A cached failure refuses the close with the recorded output
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 1 input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "task T-001"
    Then itos exits with code 1
    When itos runs "work done T-001"
    Then itos exits with code 1
    And the evidence probe ran 1 time
    And its output says "cached evidence"
    And its output says "input.txt: one"
    And its output says "exited 1"
    And the registry's item "T-001" has the status "doing" and the owner "someone"

  # One report for every failing static check, opted in or not, failed or timed out.
  @ID-EVID-49 @slice-173
  Scenario Outline: A failing static check's --json problem names the task, the command, the exit and the output tail
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: <command>
            cost: static
            <extra>
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001 --json"
    Then itos exits with code 1
    And its JSON's "problems" has one entry whose "rule" is "work-done-task-check"
    And its JSON's problem "work-done-task-check" has "task" "T-001"
    And its JSON's problem "work-done-task-check" has "command" "<command>"
    And its JSON's problem "work-done-task-check" has "exit" <exit>
    And its JSON's problem "work-done-task-check" has "timed_out" <timed_out>
    And its JSON's problem "work-done-task-check"'s "output" says "<output>"
    And the registry's item "T-001" has the status "doing" and the owner "someone"

    Examples:
      | command                                           | extra         | exit | timed_out | output         |
      | evidence-probe 1 input.txt                        | evidence: {}  | 1    | false     | input.txt: one |
      | evidence-probe 1 input.txt                        | # no evidence | 1    | false     | input.txt: one |
      | echo started; sleep 3; evidence-probe 0 input.txt | timeout: 1    | null | true      | started        |

  # The check's own timeout outlasts the hook's cap, and work done waits for it.
  @ID-EVID-50 @slice-173
  Scenario: work done runs a check under its own timeout, not hooks.commit_msg.check_timeout
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: sleep 2; evidence-probe 0 input.txt
            cost: static
            timeout: 10
      """
    And the committed file "input.txt" holding "one"
    And hooks.commit_msg.check_timeout is 1
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And the registry's item "T-001" has the status "done" and the owner "someone"

  # The plain check is the last written, so the last run is its.
  @ID-EVID-51 @slice-173
  Scenario: At work done a check that does not opt in runs again in the working folder, beside one answered from its record
    Given the committed file "tasks/phase-1.yaml" holding the lines:
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
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And itos has run "task T-001"
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran "0 input.txt" 1 time
    And the evidence probe ran "0 README.md" 2 times
    And the evidence probe's last run was in the working folder

  # A project that opts nothing in sees today's work done, even after a hand run.
  @ID-EVID-52 @slice-173 @guard
  Scenario: With no opted-in check, work done runs the check in the working folder after a hand run, and stores nothing
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: static
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the file "input.txt" is changed to hold "two"
    And itos has run "task T-001"
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran 2 times
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in the working folder
    And its output does not say "cached evidence"
    And the evidence store holds no record

  # T-002's failing record of the same command would refuse the close if looked up.
  @ID-EVID-53 @slice-173 @guard
  Scenario: An opted-in after: push check stays pending at work done, beside a failing record of its command
    Given the committed file "tasks/phase-1.yaml" holding the lines:
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
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "task T-002"
    Then itos exits with code 1
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the evidence probe ran 1 time
    And its output does not say "cached evidence"
    And the registry's item "T-001" has the status "done" and the owner "someone"

  # Late checks are CI's: named, never run here, while a static check below one still runs.
  @ID-EVID-54 @slice-173
  Scenario: work done names a task's late checks as left to CI without running them, and runs a static check written below one
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: late
          - run: evidence-probe 0 README.md
            cost: static
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001 --json"
    Then itos exits with code 0
    And the evidence probe ran "0 input.txt" 0 times
    And the evidence probe ran "0 README.md" 1 time
    And its JSON's "left_to_ci" is [{"task": "T-001", "command": "evidence-probe 0 input.txt"}]
    And its output says "left to CI"
    And the registry's item "T-001" has the status "done" and the owner "someone"

  # Bug 144: "is late: left to CI" read as a warning, though late is a cost class and CI's run, judged just below, runs the check. Fails before the work at the output step: the line says only "is late: left to CI".
  @ID-EVID-57 @bug-144 @wip
  Scenario: work done says a late check is left to CI because it is late, and that the CI run below judges it
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 0 input.txt
            cost: late
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001"
    Then itos exits with code 0
    And its output says "T-001's check `evidence-probe 0 input.txt` is left to CI: it is late (ci.cost), so work done does not run it, and the CI run below judges it"
    And the registry's item "T-001" has the status "done" and the owner "someone"

  # A gitlink in HEAD's tree leaves no checkout to make, so the check runs where a hand run would.
  @ID-EVID-56 @slice-173
  Scenario Outline: A checkout of HEAD's tree that cannot be made runs the check uncached in the working folder, its exit its own
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe <code> input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And a gitlink at "vendor/submodule" pointing to HEAD is committed
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the file "input.txt" is changed to hold "two"
    When itos runs "work done T-001 --json"
    Then itos exits with code <exit>
    And the evidence probe ran 1 time
    And the evidence probe's last run read "two" from "input.txt"
    And the evidence probe's last run was in the working folder
    And its JSON's evidence for the command "evidence-probe <code> input.txt" has "reason" "snapshot-unusable"
    And the evidence store holds no record
    And the registry's item "T-001" has the status "<status>" and the owner "someone"

    Examples:
      | code | exit | status |
      | 0    | 0    | done   |
      | 1    | 1    | doing  |

  # A cached failure still refuses before the CI watch, which is never asked.
  @ID-EVID-55 @slice-173 @guard
  Scenario: A failing check refuses the close before the CI watch, whose red run goes unreported
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: evidence-probe 1 input.txt
            cost: static
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    When itos runs "task T-001"
    Then itos exits with code 1
    When itos runs "work done T-001 --json"
    Then itos exits with code 1
    And its JSON's "problems" has one entry whose "rule" is "work-done-task-check"
    And its output does not say "https://ci.example/runs/1"
