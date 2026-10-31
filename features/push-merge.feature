@phase-1
Feature: Under push.mode pr with push.pr.update merge, itos push brings an item's branch up to main by merging it in, never rebasing a pushed commit
  Under push.mode pr (push-mode.feature) itos push rebases an item's branch
  onto the remote's item branch, and when main has moved past the pull
  request it asks GitHub to rebase it (gh pr update-branch --rebase), or the
  implementer runs itos push --onto-main, a rebase of the item's own pushed
  branch with a lease. Both rewrite pushed history, and both are refused to
  agents by Claude Code as destructive: on 2026-10-29 every command the
  person had to run by hand was such a rewrite (gh pr update-branch
  --rebase, itos push --onto-main, a force push to reword). A merge rewrites
  nothing. The person settled the v9 plan's open question 1
  (docs/plans/v9-coordinator.md, "Landing under pr") on 2026-10-30: merges,
  not rebases (p1-pr-merge-mode). v8's released merge-in-history refusal
  makes it opt-in, so nothing breaks before v9.

  The config. push.pr.update is rebase, the default, or merge, read under
  push.mode pr alone, as push.pr's other keys are. config check refuses any
  other value as it refuses an enum's, naming the key and both values
  ("push.pr.update should be one of rebase, merge"); config get
  push.pr.update prints rebase when the config leaves it out; the schema
  (internal/config/schema.go) lists it with its words: "How itos push brings
  an item's branch up to the remote's item branch and to main under
  push.mode pr: rebase (the default) rebases it, merge merges them in and
  never rewrites a pushed commit."

  Under rebase, or with no push.pr.update, nothing changes.

  Under merge, itos push from an item's branch fetches main and the item's
  branch as it does today, then, in this order:

  1. When the remote's item branch has commits the local branch lacks, it
     merges them in rather than rebasing onto them: one merge commit itos
     makes, "Merge <remote>/<item> into <item>" ("Merge origin/slice-9 into
     slice-9"). A local branch with no commit of its own past the remote's
     is fast-forwarded to it, with no merge commit.
  2. The registry's commits among those the remote's item branch lacks go
     to main as they do today (slice-192, push-mode-ledger.feature).
  3. When the remote's main has commits the branch lacks, it merges main's
     head, as the push fetched it, into the branch: one merge commit itos
     makes, "Merge <main> into <item>" ("Merge main into slice-9"). The
     commit rules pass it: a clean merge has no changes of its own, and a
     merge is judged by its own changes (bug 30, internal/git/merge.go), so
     its untyped first line is no problem. It is made after the registry's
     commits are divided out (step 2), so it is never among the commits a
     registry commit is carried out of (merge-in-history, below).

  The push is then a fast-forward of the remote's item branch: never forced,
  no lease, and no commit the remote's item branch had changes its SHA; the
  registry's commits and main go in the same git push --atomic as today. The
  clone's branch ends at the head pushed to the item's branch. A first push,
  the remote having no item branch, merges main in too rather than rebasing
  onto it. Since the head pushed then has main's head, gh pr update-branch
  is never asked. How the pull request lands (--ready, gh pr merge) is
  slice-214's.

  A conflict in either merge stops the push before anything is pushed: exit
  1, outcome merge-conflict, no gh called, and the merge left in progress in
  the clone for the implementer, who resolves it and commits it as an
  ordinary commit. The report names each conflicted path and what to run:
  resolve the files, git add them, git commit, then itos push again; or git
  merge --abort. A merge with changes of its own needs a commit type in its
  first line (merge-type, as the commit rules already judge such a merge), so
  the advice says to give the commit a typed first line rather than keep
  git's prepared "Merge ..." one. itos push again with the merge committed
  pushes it, a fast-forward like any other: the remote's main is then an
  ancestor of HEAD, and nothing is merged again.

  --onto-main under merge is refused before anything is fetched or pushed,
  exit 1, outcome onto-main-under-merge, saying that push.pr.update is merge,
  so itos push merges main in, and to run itos push. It is refused at the
  point where --onto-main outside push.mode pr is refused today, so --onto-main
  beside --tag or --ref is still a usage error first.

  merge-in-history keeps its reason: carrying a registry commit out of the
  item's branch makes the commits above it again on their first parents,
  which would drop what a merge among them brings in, its resolution
  included. Under merge it is refused only where that would happen: a
  registry commit to carry out that sits below a merge among the commits the
  push sends. A registry commit above every such merge is carried to main as
  any other, nothing being made again past the merge, and the merge stays on
  the item's branch under its own SHA. That is the case of a merge itos left
  for the implementer, committed and not yet pushed, with an idea added
  after it. A merge already on the remote's item branch is not among the
  commits the push sends and was never refused. A registry commit below a
  merge the implementer committed (a take made on the branch, then a first
  push whose merge of main stopped on a conflict) is still refused, its words
  unchanged: remaking a merge without losing its resolution is not in this
  slice. Under rebase merge-in-history is as it is today.

  --json's outcome gains merge-conflict and onto-main-under-merge.

  Not in this slice: --ready, updating and landing by merge (slice-214); the
  auto-merge a push without push.pr.draft turns on; this repository turning
  push.pr.update on.

  The recording gh and the steps are push-mode.feature's and
  push-mode-ledger.feature's, with five new ones. "the remote's branch
  <name> has the saved <name> commit" requires the commit "the last commit
  SHA is saved as <name>" saved from the clone's HEAD to be an ancestor of
  the head of the remote's branch, the same SHA. "the head of the remote's
  branch <name> is the merge commit <subject>" reads that head's subject and
  requires two parents. "the clone has a merge in progress" finds
  MERGE_HEAD in the clone's git folder. "the clone resolves the conflict in
  <path>, keeping both sides, and commits the merge as <subject>" is the
  rebase step's resolution (the conflict markers left out, the file staged),
  then git commit --no-verify -m <subject>; it fails, setting nothing up,
  when the clone has no merge in progress. The config's push.pr.update is
  written by the config steps as any key.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # Both values pass, rebase written out as the default is. Fails before the
  # work at the exit code: today push.pr.update is an unknown key, exit 2.
  @wip @ID-PUSHMERGE-01 @slice-213
  Scenario Outline: config check accepts push.pr.update <update>
    Given the config sets "push.pr.update" to "<update>"
    When itos checks the config
    Then itos exits with code 0

    Examples:
      | update |
      | rebase |
      | merge  |

  # squash is the merge GitHub offers that the commit rules cannot judge.
  # Fails before the work at the values: today the refusal is of an unknown
  # key, exit 2 as well, naming push.pr.update but not the two values.
  @wip @ID-PUSHMERGE-02 @slice-213
  Scenario: config check refuses a push.pr.update other than rebase or merge, naming both
    Given the config sets "push.pr.update" to "squash"
    When itos checks the config
    Then itos exits with code 2
    And its output says "push.pr.update"
    And its output says "rebase, merge"

  # Rebase by default, so a repository that does not ask pushes as before.
  # Fails before the work at the exit code: today config get refuses the
  # unknown key, exit 2.
  @wip @ID-PUSHMERGE-03 @slice-213
  Scenario: config get prints push.pr.update's default, rebase, when the config leaves it out
    When itos runs "config get push.pr.update"
    Then itos exits with code 0
    And its output says "rebase"

  # The slice's point: main moved past the pushed pull request, and itos
  # push, with nothing new in the clone, merges main in and pushes the merge
  # as a fast-forward: main's commit keeps its SHA on the item's branch, and
  # so does the commit pushed before. The real pre-push hook judges the
  # range: main's docs commit passes the scratch rules, and the merge itos
  # writes passes them untyped, having no changes of its own (bug 30). Fails
  # before the work at the exit code: today push.pr.update is an unknown
  # key, so the config cannot be read, the push is a trunk push to main, and
  # the pre-push hook, which cannot read the config either, refuses it.
  @wip @ID-PUSHMERGE-04 @slice-213
  Scenario: Under merge, a push from an item's branch main has moved past merges main in and pushes a fast-forward
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And the pre-push hook is installed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And the last commit SHA is saved as "pushed"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: tidy the docs" touching "docs.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" has the remote main's commit "docs: tidy the docs"
    And the remote's branch "slice-9" has the saved "pushed" commit
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"
    And its output says "https://github.example/o/r/pull/7"
    And the recording gh was never called as "pr update-branch"

  # A commit someone pushed to the item's branch (the coordinator's fix to
  # the spec, say) is merged in rather than the clone's new commit being
  # rebased onto it: the clone's commit reaches the remote under the SHA it
  # was made with. Fails before the work at the saved commit's step: today
  # the config cannot be read and the trunk push sends the clone's commits,
  # rebased, to main, so the remote's item branch lacks it.
  @wip @ID-PUSHMERGE-05 @slice-213
  Scenario: Under merge, a commit the remote's item branch has that the clone lacks is merged in, never rebased onto
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote's branch "slice-9" has gained the commit "fix: mend the widget" touching "mend.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    And the last commit SHA is saved as "work"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" has the saved "work" commit
    And the remote has the branch "slice-9" with "fix: mend the widget"
    And the head of the remote's branch "slice-9" is the merge commit "Merge origin/slice-9 into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # bug-151's case under merge: the pushed spec and a spec merged to main
  # first both write one feature file. The merge stops, left in progress for
  # the implementer, nothing pushed and no gh called, and the report names
  # the file and what to run next. Fails before the work at the outcome:
  # today the config cannot be read and the trunk push's rebase onto main
  # stops, outcome rebase-stopped.
  @wip @ID-PUSHMERGE-06 @slice-213
  Scenario: Under merge, a merge of main that conflicts is left in progress, exit 1, nothing pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: specify slice-8" touching "features/widget.feature"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "features/widget.feature"
    And its output says "git commit"
    And its output says "itos push again"
    And the clone has a merge in progress
    And the remote's branch "slice-9" does not have "docs: specify slice-8"
    And the recording gh was never called

  # The conflict to its end: the implementer resolves it and commits the
  # merge as an ordinary commit, typed since it has changes of its own, and
  # itos push again pushes it, a fast-forward the real pre-push hook judges
  # and passes; the spec pushed before keeps its SHA. Fails before the work
  # at the Given that commits the merge: today the push before it stops in a
  # rebase, not a merge, so there is no merge to commit.
  @wip @ID-PUSHMERGE-07 @slice-213
  Scenario: Under merge, itos push again after the implementer commits the resolved merge pushes it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And the last commit SHA is saved as "spec"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: specify slice-8" touching "features/widget.feature"
    And the pre-push hook is installed
    And itos has run "push", exiting with code 1
    And the clone resolves the conflict in "features/widget.feature", keeping both sides, and commits the merge as "docs: merge main into slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" has the remote main's commit "docs: specify slice-8"
    And the remote's branch "slice-9" has the saved "spec" commit
    And the head of the remote's branch "slice-9" is the merge commit "docs: merge main into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # The implementer's usual first push: the take made on the branch above
  # the pushed spec, the work above it, and main moved meanwhile. The take
  # goes to main as slice-192 sends it, and the merge itos makes comes after
  # that division, so merge-in-history never meets it. Fails before the work
  # at the item branch's main-commit step: today the config cannot be read
  # and the trunk push sends everything to main, the remote's item branch
  # left with the spec alone.
  @wip @ID-PUSHMERGE-08 @slice-213
  Scenario: Under merge, a take on the branch goes to main and the work to the item's branch with main merged in
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "spec.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: tidy the docs" touching "docs.md"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the remote's branch "slice-9" has the remote main's commit "docs: tidy the docs"
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And its output says "docs: take slice-9 to origin/main."
    And its output does not say "merge-in-history"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # The merge itos left for the implementer, committed and not yet pushed,
  # then an idea added above it: the idea goes to main, and the merge, which
  # sits below it and is made again on nothing, goes to the item's branch
  # under its own SHA. Today's merge-in-history refuses any commits that
  # hold a merge and a registry commit; under merge it refuses only a
  # registry commit below a merge. Fails before the work at the Given that
  # commits the merge: today the push before it stops in a rebase, not a
  # merge.
  @wip @ID-PUSHMERGE-09 @slice-213
  Scenario: Under merge, an idea added above the implementer's unpushed merge goes to main and the merge to the item's branch
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: specify slice-8" touching "features/widget.feature"
    And itos has run "push", exiting with code 1
    And the clone resolves the conflict in "features/widget.feature", keeping both sides, and commits the merge as "docs: merge main into slice-9"
    And the clone has the commit "docs: add p1-gadget" adding the idea "p1-gadget" to the registry
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: add p1-gadget"
    And the remote's branch "slice-9" does not have "docs: add p1-gadget"
    And the head of the remote's branch "slice-9" is the merge commit "docs: merge main into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"
    And its output does not say "merge-in-history"

  # --onto-main is the rebase merge replaces: under merge it is refused
  # before anything is fetched or pushed, saying itos push merges main in.
  # Fails before the work at the outcome: today the config cannot be read,
  # so --onto-main is refused as outside push.mode pr,
  # onto-main-without-pr.
  @wip @ID-PUSHMERGE-10 @slice-213
  Scenario: Under merge, push --onto-main is refused, exit 1, nothing pushed, saying itos push merges main in
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: tidy the docs" touching "docs.md"
    When itos runs "push --onto-main --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "onto-main-under-merge"
    And its output says "push.pr.update"
    And its output says "itos push merges main in"
    And the remote's branch "slice-9" does not have "docs: tidy the docs"
    And the recording gh was never called

  # With no push.pr.update the push rebases onto the remote's item branch as
  # it does today, no merge commit. A guard: it passes before the work and
  # must after it.
  @wip @ID-PUSHMERGE-11 @slice-213 @guard
  Scenario: Without push.pr.update, a push still rebases onto the remote's item branch, with no merge commit
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote's branch "slice-9" has gained the commit "fix: mend the widget" touching "mend.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "fix: mend the widget" then "test: cover the widget", with no merge commit
