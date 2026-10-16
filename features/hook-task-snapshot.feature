@phase-1
Feature: Commit-msg task checks execute against the captured staged tree
  # #60, q-77: actual snapshot execution, not a warning-only workaround.
  # A task-checking invocation captures the exact active index, including a
  # Git-supplied alternate index, and materializes its tracked content in
  # one disposable directory shared by that invocation's selected checks.
  # Resolve task definitions/statuses against that same captured index.
  # Keep current enablement, cost/timeout policy, static-prefix selection,
  # after-push exclusion, written order, must-fail and lifecycle verdicts.
  # Set the children's working directory, not the parent process's CWD.
  # Relative source inputs are the index's; no unstaged, untracked, ignored
  # project input or local dependency cache is copied or linked into it.
  # Normal external toolchain/cache settings remain inherited. This is not
  # an OS sandbox for commands that explicitly access external inputs.
  # Preserve staged modes, symlinks and deletions using platform checkout
  # semantics. A gitlink is unsupported in the first delivery: refuse 3,
  # naming it, rather than hydrate/fetch or silently use a local submodule.
  # Give the directory independent Git metadata, index and writable object
  # storage; history objects may be read from the original store. HEAD and
  # branch identity stay the real pre-commit base (unborn/detached too),
  # and the private index holds the proposal. Never fabricate a commit.
  # Remove inherited repository-local Git overrides from check children;
  # ordinary Git discovery must work inside the snapshot and a nested repo.
  # Keep the existing ITOS_FOOTERS/ITOS_AMEND scrubbing. Project config
  # pointers into the original tree are remapped to the snapshot; private
  # effective config/data for stealth is captured separately, not published.
  # No registered Git worktree, stash, source-checkout swap, source ref or
  # source index update. Check-created files and Git mutations stay private
  # when addressed relative to the provided execution/Git context.
  # If the original index tree or HEAD changes during checking, reject 1
  # and ask for a retry, preserving the new user state. Setup/unsupported
  # state or cleanup failure exits 3 with its reason, never a fallback to
  # working-tree checks. Clean up on success, rejection and timeout.
  # Do not materialize when no runnable task check was selected. Document
  # this input boundary in hook help; do not add another execution mode.
  # Legacy done tasks retaining checks are valid reproduction fixtures:
  # construct their data before installing the real commit hook.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a valid pending commit message names "T-001" through its configured ledger footer

  @wip @ID-HOOKSNAP-01 @bug-112
  Scenario: An untracked file cannot make a done task's staged check pass
    Given the index says "T-001" is done and retains a static check requiring "gen/out.txt"
    And "gen/out.txt" exists only as an untracked working-tree file
    When the real commit-msg hook judges the pending commit
    Then itos exits with code 1 because the staged check cannot find that file
    And the rejection names "T-001" and its failing command
    And the source file, source index and HEAD are unchanged
    And no worktree or stash was created

  @wip @ID-HOOKSNAP-02 @bug-112
  Scenario: The staged version passes despite different or missing working-tree content
    Given the index says "T-001" is done with a static check requiring the text "staged" in "input.txt"
    And the index contains that text but the working tree contains different text
    When the real commit-msg hook judges the pending commit
    Then itos exits with code 0
    And the check read "staged" from its private execution directory
    And both versions and their partial staging are unchanged
    When the working-tree file is instead missing and the same index is checked
    Then the staged check still passes without restoring that source file

  @wip @ID-HOOKSNAP-03 @bug-112
  Scenario: Checkout content follows staged additions, deletions and modes rather than disk state
    Given the index adds a tracked executable, retains a symlink and deletes a tracked input
    And ignored and untracked copies of the deleted input remain in the original checkout
    And "T-001" has static checks inspecting those staged entries and the input's absence
    When the real commit-msg hook judges the pending commit
    Then itos uses the staged file content and index modes with platform checkout semantics
    And the deleted input and original ignored dependency/cache directories are absent from the snapshot
    And no original file or mode changed
    And no ignored input was copied or linked to make a check pass

  @wip @ID-HOOKSNAP-04 @bug-112
  Scenario: Git-aware checks see a private index against the real pre-commit HEAD
    Given "T-001" checks HEAD, branch identity, history and the staged diff
    And its check creates a private Git ref and stages a generated file in its execution directory
    When the real commit-msg hook judges the pending commit
    Then the observations identify the original HEAD and branch and the captured proposed diff
    And the private Git operations succeed without touching original refs, index or object writes
    And no synthetic proposed commit was created
    And the hook's subsequent original-repository observations still use the original context

  @wip @ID-HOOKSNAP-05 @bug-112
  Scenario: Git-supplied environment overrides cannot redirect check Git commands to the source index
    Given Git supplies an alternate index for the proposed commit
    And repository-local Git environment overrides refer to the original repository
    And a check records the snapshot's staged content then initializes and uses a nested scratch repository
    When the real commit-msg hook judges the pending commit
    Then it captured the proposed tree from that alternate index
    And the check's Git first discovers the private snapshot and then its independent nested repository
    And neither original index was changed by the checks
    And the check received neither ITOS_FOOTERS nor ITOS_AMEND

  @wip @ID-HOOKSNAP-06 @bug-112
  Scenario: Snapshotting preserves check selection, limits and lifecycle verdict rules
    Given selected tasks have ordered static, after-push and late checks with recording commands
    And the cases include a must-fail check, a failing done task and a failing open task
    When the real commit-msg hook judges each fixture's pending commit
    Then checks use one staged execution context per invocation
    And their existing written order and first-failure behavior are preserved
    And after-push checks and checks at or after the first late check do not run
    And must-fail semantics and the configured hook timeout cap are preserved
    And a failing done task rejects while a failing open task retains its existing advisory verdict
    And no current hook enablement or cost policy was changed by snapshotting

  @wip @ID-HOOKSNAP-07 @bug-112
  Scenario: Check-generated inputs stay available within the invocation and disappear afterward
    Given "T-001" has one static check generating a file and a later static check requiring it
    And its commands use the ordinary external toolchain cache environment
    When the real commit-msg hook judges the pending commit
    Then both checks run in the same snapshot and the second sees the generated file
    And the external cache settings remain available
    And no generated file appeared in the original checkout
    And the snapshot, its private Git metadata and captured output files have been removed

  @wip @ID-HOOKSNAP-08 @bug-112
  Scenario Outline: A changed proposal invalidates the staged check attempt without overwriting new work
    Given a static check is executing in the captured snapshot
    And an independent fixture observer changes the source <state> during that check
    When task checking finishes
    Then itos exits with code 1 and says the proposal changed and the commit must be retried
    And it preserves the observer's new source state and all working-tree edits
    And it does not claim its captured result validates the changed proposal
    And the private snapshot is removed

    Examples:
      | state      |
      | index tree |
      | HEAD       |

  @wip @ID-HOOKSNAP-09 @bug-112
  Scenario Outline: An unmaterializable staged tree fails before task checks without a working-tree fallback
    Given a runnable task check was selected but <problem>
    When the commit-msg hook attempts its snapshot
    Then itos exits with code 3 and identifies the setup problem
    And no task check ran against either tree
    And the source HEAD, staged content and working-tree files are unchanged
    And any partially created private snapshot is removed

    Examples:
      | problem                                             |
      | the index has unresolved entries                      |
      | the private execution directory cannot be created     |
      | a required staged object cannot be materialized       |
      | the index contains a submodule gitlink                |

  @wip @ID-HOOKSNAP-10 @bug-112
  Scenario: Rejection and timeout clean up the same isolated execution resources
    Given "T-001" is done with a retained check that fails and writes private artifacts
    When the real commit-msg hook judges the pending commit
    Then itos retains the ordinary done-task rejection and captured failing output
    And the snapshot and output-capture files are removed
    When a separate fixture's check instead exceeds its configured cap
    Then the existing timeout failure is reported and its snapshot is removed
    And no source files, refs, staging, stash or registered worktree changed in either attempt
    When cleanup itself fails in a separate controlled fixture
    Then itos reports exit 3 with the cleanup problem and remaining private location
    And it does not claim cleanup succeeded or delete any source path

  @wip @ID-HOOKSNAP-11 @bug-112
  Scenario: Stealth uses captured private task data with staged project files
    Given a stealth config has private task and registry data naming a retained done-task check
    And that check reads a project file whose staged and unstaged contents differ
    When the stealth commit-msg hook judges the pending commit
    Then the check uses captured private metadata and the staged project content
    And nested itos discovery resolves the corresponding private snapshot configuration
    And the original private config, ledger, registry and notes remain unchanged
    And project staging and working-tree content are unchanged
    And no private data was committed or copied into a tracked project file

  @wip @ID-HOOKSNAP-12 @bug-112
  Scenario Outline: No runnable check means no snapshot cost or new setup failure
    Given <condition>
    And the fixture would refuse creation of a snapshot directory
    When the otherwise valid commit-msg hook runs
    Then it retains its ordinary successful result without attempting a snapshot
    And no check, dependency hydration, worktree operation or source-checkout swap occurred

    Examples:
      | condition                                      |
      | commit-msg task_checks is disabled               |
      | the message names no ledger task                 |
      | the named task has no remaining acceptance checks |
      | every selected check is pending or late          |
