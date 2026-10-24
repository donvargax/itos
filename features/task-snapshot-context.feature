@phase-1
Feature: A task check's snapshot holds the repository's git context, and a check can opt out to the working folder
  bug-123, issue #119. Since 8.7.5 (bug-112, hook-task-snapshot.feature)
  the commit-msg hook runs the static checks of the tasks a commit names in
  a checkout made by git init, the source's objects through an alternate,
  and the captured index tree. Since 8.12.0 itos task and work done run an
  opted-in check in the same checkout (evidence-local.feature,
  evidence-done.feature). That checkout has no HEAD, history, refs or
  remotes, so a check that reads its repository, and passes in the working
  folder, fails there; nothing brings back 8.7.4's behaviour. This file
  holds the git context hook-task-snapshot.feature left to separate work,
  and the opt-out.

  The git context. Every task snapshot, whichever command makes it (the
  hook, and itos task, work done and ci run for an opted-in check), holds:
  - HEAD as the folder itos runs in has it: on the same branch, an unborn
    one included, or detached at the same commit. In a linked worktree it
    is that worktree's HEAD, not the main checkout's.
  - The history, through the source's objects, read through the alternate
    as today.
  - Every ref git for-each-ref lists in that folder (branches, tags,
    remote-tracking refs, notes), at its value when the snapshot is made.
  - The remotes' config (remote.<name>.*) and the upstream of HEAD's branch
    (branch.<name>.remote and branch.<name>.merge). Of the rest of the
    source's local config, only bug-112's core.filemode and core.symlinks.
  - The index: the captured tree, as today. No commit is made for the
    proposal: HEAD is the source's own commit, so git diff --cached HEAD
    shows the proposal as it does in the working folder.
  Read-only towards the source. The snapshot's HEAD, refs, index, config
  and objects are its own. A check may write any of them (commit, tag,
  update-ref, read-tree, config, hash-object -w), and none of it reaches
  the source's HEAD, refs, reflogs, index, local config or objects. Nothing
  is shared through a registered worktree or an environment variable
  (bug-112's rule). Everything else of bug-112 stands: only the staged
  files, no untracked or ignored file, exit 3 when the hook cannot make the
  snapshot (a context that cannot be read is such a failure), and removal
  after the run.

  The opt-out, a new key. A done_when check takes snapshot: true or false,
  true when left out. A check with snapshot: false runs in the working
  folder, always: in the hook, in itos task, in work done and in ci run. In
  the hook that is where 8.7.4 ran every hook check: in the working folder,
  in the hook's environment, where its unstaged edits and its untracked and
  ignored files (a locally installed tool) are in reach. Elsewhere a check
  without evidence already runs in the working folder, so it is an opted-in
  check that it moves there. The choice is the check's, not the task's or
  the run's: in one hook run a
  check with snapshot: false runs in the working folder and the checks
  beside it without one run in the snapshot, in written order, whichever
  comes first. A hook run whose runnable checks all opt out makes no
  snapshot. There is no repository-wide switch. config check refuses any
  other value (ledger-check-value), and snapshot: false beside evidence
  (ledger-check-evidence), since a run in the working folder cannot be
  keyed by a tree. A ledger is read without config check, so at run time,
  in the hook, itos task, work done and ci run alike: only the boolean
  false opts out, and any other value, a non-boolean one included, is the
  default and runs in the snapshot, the stricter place; snapshot: false
  beside evidence runs in the working folder as a check without evidence
  does, nothing looked up or recorded, since the author said the check
  cannot run in a snapshot and a record keyed by a tree it did not run on
  would be false. check.Key, the per-run reuse of itos task and ci run,
  gains snapshot when evidence is set, so an opted-out run and an opted-in
  run of the same command and declaration are never merged; without
  evidence both run in the working folder and stay one run. The hook
  shares no run between checks.

  The evidence key: HEAD is a declared input. An evidence declaration takes
  a new key, head: true or false, false when left out, wherever evidence is
  declared (a done_when check or a ci.steps entry). A check declaring
  head: true has HEAD's commit, or that HEAD is unborn, in its key beside
  the tree, so a record made at one HEAD never answers it at another, even
  on the same tree. The commit fixes HEAD's whole history, so a check that
  reads HEAD or its history (HEAD~1, read-tree HEAD, ci scope HEAD~1 HEAD)
  declares head: true and answers only from a record of that commit. Every
  other opted-in check stays keyed as today, by the tree and what it
  declares, though it runs with the same git context: opting in promises
  that its result depends on nothing it does not declare (ADR-0060), and an
  undeclared read of HEAD is the author's mistake, as an undeclared input
  or variable is. So the hook's record, keyed at the commit's parent, still
  answers itos task and work done once the commit is made (ID-EVID-35,
  evidence-done.feature's "each pay once"). head: true is part of the
  declaration, so check.Key tells it from the same command without it. The
  branch, the other refs and the remotes are never keyed: they move
  independently of the commit, and binding them would end reuse between
  worktrees (a linked worktree is on its own branch at the same commit,
  ID-EVID-26) and after every fetch; a check that depends on them, or on a
  remote's live state (ls-remote), is not deterministic and does not opt
  in. config check accepts head on a ci.steps entry's evidence as on a
  task's check, and refuses a head that is not true or false
  (ledger-check-evidence). The engine that binds it is one for itos task,
  work done, the hook and ci run; these scenarios judge the binding on
  itos task. At run time a head that is not the boolean true is false.

  Contract, which these scenarios judge:
  - Exit codes. A check that reads the repository passes or fails in the
    snapshot as it does in the working folder; the hook's verdicts and its
    exit 3 are bug-112's.
  - Files. Nothing new in the source; the snapshot is under the git common
    dir's itos/snapshots, as today. The manifest.json of a record whose
    check declares head: true gains head, the commit or "unborn".
  - JSON. config check --json names ledger-check-value or
    ledger-check-evidence for a refused snapshot key, and
    ledger-check-evidence for a head that is not true or false.
  - Text, judged by a substring at most: a refused head says "head is
    true or false".

  # itos runs in the clone's top folder (a linked worktree's, where a
  # scenario says), under the scratch config, hooks.commit_msg.task_checks
  # true. The itos under test is ITOS_BIN's binary; a check that runs itos
  # finds it on the PATH where a scenario puts it there. origin is the bare
  # repository the clone was made from, and the clone's branch tracks it.
  # snapshot-probe is hook-task-snapshot.feature's, with two new modes, each
  # writing into its run's folder of the observation log:
  # - git-context records head (git rev-parse --verify -q HEAD, or
  #   "unborn"), branch (git symbolic-ref -q HEAD, or "detached"), parent
  #   (HEAD~1's commit, or "none"), index (git write-tree), refs (git
  #   for-each-ref --format='%(refname) %(objectname)'), remotes (git config
  #   --get-regexp '^remote\.', sorted) and upstream (git rev-parse
  #   --symbolic-full-name @{u}, or "none"), then exits 0.
  # - write-git, stopping at the first write that fails: update-ref
  #   refs/heads/probe-branch HEAD; tag probe-tag; update-ref the upstream's
  #   remote-tracking ref to HEAD~1; config remote.origin.url /nowhere;
  #   hash-object -w of "snapshot-private-object"; commit --allow-empty
  #   (the fixture's fixed author), recording both object ids; read-tree
  #   --empty; symbolic-ref HEAD refs/heads/probe-branch; exits 0 when every
  #   write succeeded and non-zero at the first that failed, so a snapshot
  #   without HEAD fails the check.
  # A step comparing the probe's record with "the source checkout" or "the
  # linked worktree" runs the same git commands there when it judges; one
  # comparing it with "the saved source state" reads what "the fixture saves
  # the source state" recorded by the same commands, for a command such as
  # work done that commits after its checks ran. A step judging the folder a
  # run was in compares real paths, symlinks and short names resolved
  # (macOS's /private/tmp), as hook-task-snapshot.feature's steps do.
  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And hooks.commit_msg.task_checks is true
    And snapshot probe commands are on PATH with an observation log outside the repository

  # The issue's three git kinds, and its repro (rev-parse HEAD~1). The base
  # commit gives HEAD a parent. Today each check fails in the snapshot (no
  # HEAD, no HEAD~1, no remote origin), the done task rejects, and the Then
  # expecting exit 0 fails with exit 1.
  @ID-SNAPCTX-01 @bug-123 @wip
  Scenario Outline: A done task's static check that reads the repository passes in the snapshot as in the working folder
    Given the itos under test is first on the PATH as itos
    And the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: <check>
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a change to "README.md" is staged
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And its output does not say "failing T-001"
    And the source files, staged entries, HEAD and refs match the saved state

    Examples:
      | check                            |
      | git rev-parse --verify HEAD~1    |
      | git read-tree HEAD               |
      | itos ci scope HEAD~1 HEAD        |
      | git ls-remote --exit-code origin |

  # What the check sees, recorded and compared with the source, in the three
  # states of HEAD. "fresh" is no default branch name, so the unborn row
  # tells it from git init's own unborn HEAD. Today the probe records an
  # unborn HEAD on git init's default branch: the HEAD Then fails in every
  # row (no commit in the first two, the wrong branch in the third).
  @ID-SNAPCTX-02 @bug-123 @wip
  Scenario Outline: The snapshot has the source's HEAD, refs, remotes and upstream, and the staged tree as its index, whether HEAD is on a branch, detached or unborn
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe git-context
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And the source's HEAD is <head>
    And a change to "README.md" is staged
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe's last run recorded HEAD, its branch and its parent as the source checkout has them
    And the probe's last run recorded the source checkout's refs, remote config and upstream
    And the probe's last run recorded the staged tree as its index

    Examples:
      | head                              |
      | on its branch                     |
      | detached at its commit            |
      | unborn, on the new branch "fresh" |

  # Every write the probe makes succeeds in the snapshot, privately. Today
  # its first write, update-ref ... HEAD, fails for want of a HEAD, and the
  # Then expecting exit 0 fails with exit 1. After the fix, a snapshot that
  # shared refs, the index, the config or the object store with the source
  # fails the state Thens.
  @ID-SNAPCTX-03 @bug-123 @wip
  Scenario: A check's writes to HEAD, refs, the index, the config and objects stay in the snapshot
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe write-git
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a change to "README.md" is staged
    And the fixture saves the source state
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the source files, staged entries, HEAD and refs match the saved state
    And the source checkout's local git config matches the saved state
    And the source checkout's objects hold none of the objects the probe wrote

  # The issue's fourth kind: a tool installed in an ignored folder, which no
  # snapshot holds, beside a check that reads an unstaged edit. Both opt
  # out, so the hook needs no snapshot, and the blocked snapshots location
  # would refuse with exit 3 if it made one. Today the ledger's unknown key
  # is ignored, the hook tries a snapshot at the blocked location, and the
  # Then expecting exit 0 fails with exit 3.
  @ID-SNAPCTX-04 @bug-123 @wip
  Scenario: A check with snapshot: false runs in the working folder, where its ignored tools and unstaged edits are, and needs no snapshot
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: node_modules/.bin/lint
            cost: static
            snapshot: false
          - run: snapshot-probe expect-text input.txt unstaged
            cost: static
            snapshot: false
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And the working folder has the ignored program "node_modules/.bin/lint", which exits 0
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is replaced with "unstaged"
    And the fixture writes a regular file at the git common dir's "itos/snapshots"
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe recorded the text "unstaged" read from "input.txt"
    And the probe recorded the source checkout as its execution directory

  # snapshot is a new key of a check, true or false. Today config check
  # refuses it as an unknown key, so the first Then expecting exit 0 fails
  # with exit 1.
  @ID-SNAPCTX-05 @bug-123 @wip
  Scenario Outline: config check accepts snapshot: true or false on a task's check, and refuses another value or snapshot: false beside evidence
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - { run: snapshot-probe record-run, cost: static, snapshot: false }
          - { run: snapshot-probe record-run, cost: static, snapshot: true }
      """
    When itos checks the config
    Then itos exits with code 0
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - { run: snapshot-probe record-run, cost: static, <keys> }
      """
    When itos runs "config check --json"
    Then itos exits with code 1
    And its JSON's "problems" has one entry whose "rule" is "<rule>"

    Examples:
      | keys                          | rule                  |
      | snapshot: "no"                | ledger-check-value    |
      | snapshot: false, evidence: {} | ledger-check-evidence |

  # The same context outside the hook. Each row runs the opted-in check on a
  # miss in a checkout of its own: itos task on the index (HEAD's tree here,
  # nothing staged), work done on HEAD's tree. The work done row's setup is
  # evidence-done.feature's, the item's commits on the remote; the itos task
  # row ignores it. work done commits the close after its checks ran, moving
  # HEAD and the branch, so the probe is compared with the state saved
  # before the When. Today the probe records an unborn HEAD, so the HEAD
  # Then fails in both rows.
  @ID-SNAPCTX-06 @bug-123 @wip
  Scenario Outline: A hand-run itos task and work done run an opted-in check in a snapshot with the source's git context
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe git-context
            cost: static
            evidence: {}
      """
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the fixture saves the source state
    When itos runs "<command>"
    Then itos exits with code 0
    And the snapshot probe ran 1 time
    And the probe recorded a disposable checkout under the source git common directory's "itos/snapshots", not the source checkout
    And the probe's last run recorded HEAD, its branch and its parent as the saved source state has them
    And the probe's last run recorded the refs, remote config and upstream of the saved source state

    Examples:
      | command         |
      | task T-001      |
      | work done T-001 |

  # head: true binds HEAD's commit. The hand run keys the staged tree at the
  # base commit; git commit then commits exactly that index, so the tree is
  # the same and only HEAD has moved (the scratch hooks are the fixture's
  # stand-ins, so the commit runs no check). Today a run ignores the unknown
  # key head, opting in as evidence: {} would, and the second run hits the
  # first run's record: the Then expecting "hit" false fails.
  @ID-SNAPCTX-07 @bug-123 @wip
  Scenario: A check declaring head: true misses at a new HEAD on the same tree
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe git-context
            evidence: { head: true }
      """
    And a change to "README.md" is staged
    And itos has run "task T-001 --json"
    And git commits with the message "chore: tidy the readme"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has "hit" false
    And the snapshot probe ran 2 times
    And the probe's last run recorded HEAD, its branch and its parent as the source checkout has them
    And the manifest of the record the last JSON named has "head" equal to the source checkout's HEAD commit

  # Guard: SNAPCTX-07's setup without head, which keeps the key tree-only,
  # as ID-EVID-35's hook-then-commit reuse needs. Today the second run hits
  # too, so this passes; it fails a fix that keys HEAD for every check.
  @ID-SNAPCTX-09 @bug-123 @guard @wip
  Scenario: A check that does not declare head still hits its record at a new HEAD on the same tree
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe git-context
            evidence: {}
      """
    And a change to "README.md" is staged
    And itos has run "task T-001 --json"
    And git commits with the message "chore: tidy the readme"
    When itos runs "task T-001 --json"
    Then itos exits with code 0
    And its JSON's evidence for the task "T-001" has "hit" true
    And the snapshot probe ran 1 time
    And the manifest of the record the last JSON named has no "head"

  # head is a new key of evidence, on a task's check and on a ci.steps
  # entry alike. Today config check refuses it as an unknown key of
  # evidence, so the first Then expecting exit 0 fails with exit 1. The
  # refusal's rule is the same today; its wording is not.
  @ID-SNAPCTX-10 @bug-123 @wip
  Scenario: config check accepts evidence's head as true or false, and refuses another value
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - { run: snapshot-probe record-run, evidence: { head: true } }
          - { run: snapshot-probe git-context, evidence: { head: false } }
      """
    And the CI steps are the YAML:
      """
      - { run: snapshot-probe git-context, cost: static, evidence: { head: true } }
      """
    When itos checks the config
    Then itos exits with code 0
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - { run: snapshot-probe record-run, evidence: { head: "yes" } }
      """
    When itos runs "config check --json"
    Then itos exits with code 1
    And its JSON's "problems" has one entry whose "rule" is "ledger-check-evidence"
    And its output says "head is true or false"

  # git worktree add makes the branch "wt" at the main checkout's commit,
  # and the worktree's index holds the committed ledger, so the hook there
  # names T-001 with nothing staged. A snapshot that took the common dir's
  # HEAD would record the main checkout's branch. Today the probe records an
  # unborn HEAD: the HEAD Then fails.
  @ID-SNAPCTX-08 @bug-123 @wip
  Scenario: In a linked worktree the snapshot's HEAD is that worktree's, not the main checkout's
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe git-context
            cost: static
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And a linked worktree of the repository at "../wt"
    When the commit-msg hook checks the message in the linked worktree:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe's last run recorded HEAD, its branch and its parent as the linked worktree has them

  # The choice is per check. input.txt is staged as "staged" and reads
  # "unstaged" on disk, so each check passes only in its own place: the
  # opted-out one in the working folder, the other in the snapshot. The rows
  # swap the order, so neither "the first check decides" nor "once a
  # snapshot exists everything runs there" passes. Today the key is ignored
  # and both run in the snapshot: the opted-out check fails, the done task
  # rejects, and the Then expecting exit 0 fails with exit 1.
  @ID-SNAPCTX-11 @bug-123 @wip
  Scenario Outline: In one hook run a check with snapshot: false runs in the working folder and the check beside it in the snapshot
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - <first>
          - <second>
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is replaced with "unstaged"
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And its output does not say "failing T-001"

    Examples:
      | first                                                                                 | second                                                                                |
      | { run: snapshot-probe expect-text input.txt unstaged, cost: static, snapshot: false } | { run: snapshot-probe expect-text input.txt staged, cost: static }                    |
      | { run: snapshot-probe expect-text input.txt staged, cost: static }                    | { run: snapshot-probe expect-text input.txt unstaged, cost: static, snapshot: false } |

  # A ledger config check would refuse, read by the hook as it is: the
  # opt-out wins over evidence, so the check runs in the working folder,
  # where it reads the unstaged edit, and nothing is recorded. Today the
  # key is ignored and the opted-in check runs in its snapshot, reads
  # "staged" and fails: the Then expecting exit 0 fails with exit 1.
  @ID-SNAPCTX-12 @bug-123 @wip
  Scenario: At run time snapshot: false beside evidence runs in the working folder, nothing looked up or recorded
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe expect-text input.txt unstaged
            cost: static
            snapshot: false
            evidence: {}
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is replaced with "unstaged"
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe recorded the source checkout as its execution directory
    And its output does not say "cached evidence"
    And the evidence store holds no record

  # Guard: a non-boolean snapshot is the default at run time, so the check
  # runs in the snapshot, reads the staged "staged" and passes. Today the
  # key is ignored and the check runs in the snapshot, so this passes; it
  # fails a fix that reads any value but true as an opt-out.
  @ID-SNAPCTX-13 @bug-123 @guard @wip
  Scenario: At run time a snapshot value that is not a boolean runs the check in the snapshot
    Given the staged file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe expect-text input.txt staged
            cost: static
            snapshot: "no"
      """
    And the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "done" is staged
    And the fixture commits its retained-check task data as the base
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is replaced with "unstaged"
    When the commit-msg hook checks the message:
      """
      chore: check the proposal

      Task: T-001
      """
    Then itos exits with code 0
    And the probe recorded the text "staged" read from "input.txt"
    And the probe recorded a disposable checkout under the source git common directory's "itos/snapshots", not the source checkout

  # SNAPCTX-12's rule outside the hook: snapshot: false means the working
  # folder everywhere. The input.txt staged, then changed on disk, is
  # evidence-local.feature's setup, so only a run in the working folder
  # reads "unstaged". The work done row's setup is evidence-done.feature's;
  # work done keys HEAD's tree, which holds "one", so a run in its snapshot
  # fails too. Today the key is ignored and each command runs the opted-in
  # check in a snapshot of its own, where the probe reads the staged or
  # committed text and exits 1: the Then expecting exit 0 fails.
  @ID-SNAPCTX-14 @bug-123 @wip
  Scenario Outline: itos task and work done run an opted-in check with snapshot: false in the working folder, nothing looked up or recorded
    Given the committed file "tasks/phase-1.yaml" holding the lines:
      """
      - id: T-001
        type: chore
        title: Tidy
        done_when:
          - run: snapshot-probe expect-text input.txt unstaged
            cost: static
            snapshot: false
            evidence: {}
      """
    And the committed file "input.txt" holding "one"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And "input.txt" is staged with the text "staged"
    And the working-tree copy of "input.txt" is replaced with "unstaged"
    When itos runs "<command>"
    Then itos exits with code 0
    And the probe recorded the text "unstaged" read from "input.txt"
    And the probe recorded the source checkout as its execution directory
    And its output does not say "cached evidence"
    And the evidence store holds no record

    Examples:
      | command         |
      | task T-001      |
      | work done T-001 |
