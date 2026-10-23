@phase-1
Feature: Commit-msg task checks execute against the captured staged tree
  # #60, q-77: capture the active index (including GIT_INDEX_FILE) once;
  # select task definitions/statuses and materialize files from that capture.
  # One invocation shares one disposable checkout at
  # <git common dir>/itos/snapshots/<unique>/tree, never in the working tree
  # or a registered worktree. Keep captures beside tree in the same unique
  # directory; remove both on success and a failing check. Do not change the
  # parent's CWD, source files, refs or index, or use a stash or checkout swap.
  # Give tree its own .git, index and writable objects. Read source history
  # through .git/objects/info/alternates, never the environment variable
  # GIT_ALTERNATE_OBJECT_DIRECTORIES. Strip exactly the names printed by
  # git rev-parse --local-env-vars from children; retain ITOS_FOOTERS and
  # ITOS_AMEND scrubbing. Full Git-context observations are a separate item.
  # Shell, cost policy and hooks.commit_msg.check_timeout stay the loaded
  # working-tree config's: changing execution inputs must not change policy.
  # Per-check mode/cost/timeout remain the captured task's. Config-pointer
  # remapping and stealth capture are separate work.
  # Copy only staged project entries, not local dependencies, ignored or
  # untracked inputs; inherit external toolchain/cache settings. This is not
  # an OS sandbox. Refuse gitlinks and setup failures with exit 3, never a
  # working-tree fallback. No runnable check means no snapshot attempt.
  # For a portable creation failure, make itos/snapshots a regular file:
  # a child cannot be created on Linux (even root), macOS or Windows.
  # Copy source core.symlinks and core.filemode values into snapshot-local
  # Git config before checkout. Unix fixtures enable both; Windows fixtures
  # disable both explicitly: link targets become regular files, and no
  # executable-bit assertion applies. Windows rows do not skip missing links.
  # Existing selection/verdict rules stay; work done is out of scope.
  # Commit retained-check task data as the fixture's base. The When
  # calls the real hook CLI directly, so exit codes and unchanged HEAD are
  # observable without git commit's own refusals or successful HEAD update.

  Background:
    Given a repository whose ledger has the task "T-001"
    And hooks.commit_msg.task_checks is true
    And snapshot probe commands are on PATH with an observation log outside the repository
    And check-output temporary files use an existing fixture-owned temporary directory

  # Today the check finds the untracked file in the working tree and exits
  # 0; the Then expecting the done-task rejection fails.
  @ID-HOOKSNAP-01 @bug-112
  Scenario: An untracked file cannot make a done task's staged check pass
    Given the task "T-001" has the static check "snapshot-probe require gen/out.txt"
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And the untracked file "gen/out.txt" holding "generated"
    And a change to "README.md" is staged
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "failing T-001"
    And its output says "snapshot-probe require gen/out.txt"
    And the probe recorded that "gen/out.txt" was absent from its execution directory
    And the source files, staged entries, HEAD and refs match the saved state
    And no stash or registered worktree was created

  # Today both rows read the working-tree input instead of "staged", so
  # the retained done-task check rejects; the Then expecting 0 fails.
  @ID-HOOKSNAP-02 @bug-112
  Scenario Outline: The staged version passes despite different or missing working-tree content
    Given the task "T-001" has the static check "snapshot-probe expect-text input.txt staged"
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is <disk_state>
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe recorded the text "staged" read from "input.txt"
    And the source files, staged entries, HEAD and refs match the saved state

    Examples:
      | disk_state                 |
      | replaced with "unstaged"   |
      | missing                    |

  # Every row stages different bytes from disk as well as the mode change.
  # Today the probe reads the disk version and rejects, including Windows
  # where an executable-bit-only assertion could not reproduce the bug.
  # The Then expecting 0 fails. Only rows for another OS are skipped.
  # snapshot-probe inspect-entries (a test helper the steps provide) compares
  # the folder it runs in with the index of the git repository found there:
  # every index entry's content and mode (by git ls-files -s and git cat-file)
  # must match the file on disk, and no regular file outside .git may exist
  # that the index lacks; on any mismatch it prints the path and exits 1.
  # Today the check runs in the working tree, whose disk differs from the
  # index here, so the exit-code Then fails before the fix.
  @ID-HOOKSNAP-03 @bug-112
  Scenario Outline: Staged additions, deletions and modes determine the checkout
    Given the snapshot fixture runs on <platform>
    And the task "T-001" has the static check "snapshot-probe inspect-entries"
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And the index adds "added.txt" containing "staged-addition", deletes "deleted.txt" and adds "mode-entry" with mode <mode>
    And the staged mode entry has <staged_content>
    And disk instead has different ordinary-file contents for "added.txt" and "mode-entry" and a regenerated "deleted.txt"
    And ignored "node_modules" and ".cache" directories exist only in the source checkout
    And the fixture sets source-repository-local core.filemode to <filemode> and core.symlinks to <symlinks>
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe recorded "staged-addition" in "added.txt" and no "deleted.txt", "node_modules" or ".cache"
    And the probe recorded "mode-entry" as <checkout_result>
    And the probe recorded snapshot-local core.filemode as <filemode> and core.symlinks as <symlinks>
    And the source files, modes, staged entries, HEAD and refs match the saved state

    Examples:
      | platform | mode   | staged_content               | filemode | symlinks | checkout_result                                      |
      | Linux    | 100755 | the text "staged-executable"  | true     | true     | an executable regular file with "staged-executable"   |
      | macOS    | 100755 | the text "staged-executable"  | true     | true     | an executable regular file with "staged-executable"   |
      | Windows  | 100755 | the text "staged-executable"  | false    | false    | a regular file with "staged-executable"               |
      | Linux    | 120000 | the link target "added.txt"   | true     | true     | a symlink whose target is "added.txt"                 |
      | macOS    | 120000 | the link target "added.txt"   | true     | true     | a symlink whose target is "added.txt"                 |
      | Windows  | 120000 | the link target "added.txt"   | false    | false    | a regular file containing the link text "added.txt"   |

  # Today both checks can pass, but their recorded CWD is the source
  # checkout. The Then requiring a shared disposable checkout fails; the
  # source-file assertion also detects the generated file left there.
  @ID-HOOKSNAP-07 @bug-112
  Scenario: Check-generated inputs are shared within the snapshot, not with the source checkout
    Given the task "T-001" has these static checks in written order:
      | command                                   |
      | snapshot-probe generate gen/out.txt        |
      | snapshot-probe require gen/out.txt         |
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And the fixture sets GOCACHE to an existing absolute directory outside the repository
    And a change to "README.md" is staged
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And both probes recorded the same disposable checkout under the source git common directory's "itos/snapshots", not the source checkout
    And the second probe recorded the first probe's generated "gen/out.txt"
    And both probes recorded GOCACHE unchanged from the fixture's value
    And the source files, staged entries, HEAD and refs match the saved state
    And the recorded snapshot directory and check-output temporary files no longer exist

  # Both rows leave a valid staged proposal and a check that passes on disk.
  # Today no snapshot is attempted and the hook returns 0, so the Then
  # expecting the setup/unsupported-state exit 3 fails.
  # Match only the itos-owned reason below and the identified location,
  # normalizing path separators; never match OS error text or its wording.
  @ID-HOOKSNAP-09 @bug-112
  Scenario Outline: Snapshot setup refuses an unusable location or unsupported gitlink before checks run
    Given the task "T-001" has the static check "snapshot-probe record-run"
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a change to "README.md" is staged
    And <problem>
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 3
    And its diagnostic contains "<reason>" and identifies <location>
    And no snapshot probe command ran
    And the source files, staged entries, HEAD and refs match the saved state
    And no partially created snapshot or check-output temporary file remains

    Examples:
      | problem                                                                  | reason                            | location                                 |
      | the fixture writes a regular file at the git common dir's "itos/snapshots" | cannot create task-check snapshot | the git common dir's "itos/snapshots"     |
      | the index contains a gitlink at "vendor/submodule" pointing to HEAD       | unsupported gitlink               | the staged path "vendor/submodule"       |

  # Guard: every row already passes today because no snapshot exists yet.
  # There is no honest pre-fix failing assertion for this negative contract;
  # do not invent a failure or claim this outline as red-first evidence.
  # "No remaining acceptance checks" is the row with done_when: [] and a
  # done registry item, not checks that exist but happen to pass or be late.
  # The empty footer cell inserts no footer; docs needs no Task footer here.
  @guard @ID-HOOKSNAP-12 @bug-112
  Scenario Outline: No runnable check means no snapshot cost or new setup failure
    Given hooks.commit_msg.task_checks is <task_checks>
    And the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when: <checks>
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a change to "README.md" is staged
    And the fixture writes a regular file at the git common dir's "itos/snapshots"
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      <commit_type>: check the proposal

      <footer>
      """
    Then itos exits with code 0
    And no snapshot probe command ran
    And the snapshot-location blocking file and source state are unchanged
    And its output does not say "cannot create task-check snapshot"

    Examples:
      | task_checks | checks                                                                  | commit_type | footer      |
      | false       | [{ run: 'snapshot-probe record-run', cost: static }]                      | chore       | Task: T-001 |
      | true        | [{ run: 'snapshot-probe record-run', cost: static }]                      | docs        |             |
      | true        | []                                                                      | chore       | Task: T-001 |
      | true        | [{ run: 'snapshot-probe record-run', cost: static, after: push }]         | chore       | Task: T-001 |
      | true        | [{ run: 'snapshot-probe record-run', cost: late }]                        | chore       | Task: T-001 |

  # Today the expected ordinary verdicts pass, but the probe records the
  # source checkout and leaves its artifact there. The Then requiring a
  # disposable checkout fails in both rows, before cleanup is asserted.
  # leave-artifacts writes gen/artifact.txt containing "private-artifact\n",
  # prints exactly "snapshot-artifact-output\n" to stdout and nothing to
  # stderr, records its observations, then exits with check_exit.
  @ID-HOOKSNAP-13 @bug-112
  Scenario Outline: Successful and failing checks remove their disposable resources
    Given the task "T-001" has the static check "snapshot-probe leave-artifacts <check_exit>"
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a change to "README.md" is staged
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code <hook_exit>
    And the probe recorded a disposable checkout under the source git common directory's "itos/snapshots", not the source checkout
    And the recorded snapshot directory, its private Git metadata, generated artifact and check-output temporary files no longer exist
    And its output <expected_output>
    And the source files, staged entries, HEAD and refs match the saved state
    And no stash or registered worktree was created

    Examples:
      | check_exit | hook_exit | expected_output                                                                               |
      | 0          | 0         | omits "snapshot-artifact-output" and any "failing T-001" report                                 |
      | 7          | 1         | includes "snapshot-artifact-output", "failing T-001" and "snapshot-probe leave-artifacts 7"      |
