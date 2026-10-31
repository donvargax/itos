@phase-1
Feature: Under push.pr.update merge, itos push --ready brings the pull request up to main by a merge and lands it as a merge commit itos titles
  Under push.mode pr with push.pr.draft (push-mode-draft.feature) itos push
  --ready brings a pull request main has moved past up to date with gh pr
  update-branch --rebase and lands it with gh pr merge --auto --rebase
  --match-head-commit <the head judged>, or, when GitHub refuses auto-merge
  as "is in clean status", one gh pr merge --rebase --match-head-commit. Each
  is a rewrite of the item's branch, and so was every command the person
  had to run by hand on 2026-10-30: Claude Code's classifier refuses agents
  gh pr update-branch --rebase, itos push --onto-main and a force push to
  reword. A merge rewrites nothing. The person settled it for the pilot
  (docs/plans/v9-coordinator.md, Landing under pr, open question 1;
  p1-pr-merge-mode): merges, not rebases, opt in, so v8's flow stays the
  default. Slice-213 adds the key, push.pr.update (rebase, the default, or
  merge), and has a plain itos push merge main in; this slice is --ready's
  half.

  Under push.pr.update merge, --ready changes in three places, and nowhere
  else (the reviews, the threads, the waits, ready_wait and the read-back
  are as push-mode-draft.feature has them):
  1. The update. Where it asks gh pr update-branch <item> --repo
     <repository> --rebase today (main has a commit the head lacks), it asks
     gh pr update-branch <item> --repo <repository>, without --rebase:
     GitHub merges main into the branch. It then finds the head GitHub made
     and waits for its run as today (the first head that has main's head, as
     the push fetched it, as an ancestor: a merge has it as one), and that
     head is the one it judges and pins.
  2. The merge. gh pr merge <item> --repo <repository> --auto --merge
     --subject=<subject> --body=<body> --match-head-commit <the head
     judged>, and the clean-status fallback gh pr merge <item> --repo
     <repository> --merge --subject=<subject> --body=<body>
     --match-head-commit <the head judged>, in that order, the values joined
     to their flags as gh pr create's --title= and --body= are, so a title
     that starts with a dash is never read as a flag. The subject is the
     pull request's title as push-mode-title.feature makes it: "<item>: <the
     item's title in the registry>", cut to 255 characters and an ellipsis
     past what GitHub keeps, the item's id alone when the registry gives it
     no title. The body is the footer "Item: <item>" alone, so a reader of
     main's first parents gets the item from the merge commit without
     parsing the branch, and the subject has the title already.
  3. The words. What --ready says of a rebase says merge instead: "GitHub
     merged <remote>/<main> into the pull request: watching the run of
     <short>, its new head."; "Auto-merge is on: GitHub merges <address> by
     merge once its required checks pass."; "Merged <address> by merge at
     <short>: ..." (both endings); with no ci.watch provider, the stop before
     GitHub's update shows reads "update waiting; GitHub has not merged
     <remote>/<main> into it yet", and the waiting line's part "update
     waiting" where it reads "rebase waiting".
  Under push.pr.update rebase, or none, every call and every line is as
  today: the guard below keeps that.

  verify needs nothing new: it judges a merge commit by its own changes, the
  paths of its dense combined diff (bug 30, commit-msg.feature), and a merge
  GitHub makes of a branch up to date with main has none, so it passes
  whatever its message, and the item's commits beside it are judged as any
  commit is. A guard below holds it for the merge commit --ready lands.

  Not in this slice: a plain push's merging main in (slice-213); a plain
  push without push.pr.draft, which still turns on auto-merge by rebase and
  asks gh pr update-branch --rebase (slice-213 or after); this repository
  turning push.pr.update merge on (a later build commit); itos next and work
  show reading merges.

  The steps. "the config, committed and pushed to the remote's main, adds
  the YAML:" gives push.pr.update as slice-213 reads it. The recording gh
  learns one answer, a file of its own beside pr-update-branch.sh, which
  stays as it is: a before/ file that answers a pr update-branch call
  without --rebase as GitHub does, merging the remote's main into the
  remote's branch of the item in a clone of its own (git merge --no-edit,
  its message "Merge branch 'main' into <item>") and pushing it there with
  no force, the branch left as it is under "the recording gh's pr
  update-branch leaves the branch as it is", and gh-stale-calls copied to
  gh-stale-left as pr-update-branch.sh copies it; then it exits. One new
  step: "the clone has merged its branch <branch> with the merge commit
  <subject> and the body <body>", as the step without the body is, the body
  a second paragraph of the message (git merge -m <subject> -m <body>).

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo" and the title "Add the widget"

  # The slice's point: main moved, so --ready asks GitHub to merge it in
  # (update-branch with no --rebase), watches the run of the head GitHub
  # made and turns on auto-merge by merge, pinned to that head, with the
  # message itos writes.
  @wip @slice-214 @ID-READYMERGE-01
  Scenario: Under push.pr.update merge, push --ready on a draft main has moved past asks GitHub to merge main in and turns on auto-merge by merge at the head GitHub made, titled after the item
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr update-branch" call has "slice-9 --repo owner/scratch"
    And the recording gh's "pr update-branch" call does not have "--rebase"
    And the fake GitHub was asked for the run of the head of the remote's branch "slice-9"
    And the recording gh's "pr merge" call has "--repo owner/scratch --auto --merge --subject=slice-9: Add the widget --body=Item: slice-9 --match-head-commit"
    And the recording gh's "pr merge" call with "--auto" has the full SHA of the head of the remote's branch "slice-9"
    And the recording gh's "pr merge" call does not have "--rebase"
    And its output says "GitHub merged origin/main into the pull request: watching the run of "
    And its output says "Auto-merge is on: GitHub merges https://github.example/o/r/pull/7 by merge once its required checks pass."

  # The clean-status race under merge: GitHub refuses auto-merge on a pull
  # request mergeable already, and --ready merges it once itself, by merge,
  # with the same message, pinned to the head it judged (the clone's HEAD:
  # main has not moved, so nothing was updated).
  @wip @slice-214 @ID-READYMERGE-02
  Scenario: Under push.pr.update merge, push --ready whose auto-merge GitHub refuses as clean merges once itself by merge, titled after the item, and says it merged
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's "pr merge" calls with "--auto" fail, saying "GraphQL: Pull request Pull request is in clean status (enablePullRequestAutoMerge)"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "--repo owner/scratch --auto --merge --subject=slice-9: Add the widget --body=Item: slice-9 --match-head-commit"
    And the recording gh's "pr merge" call has "--repo owner/scratch --merge --subject=slice-9: Add the widget --body=Item: slice-9 --match-head-commit"
    And the recording gh's "pr merge" call with "--repo owner/scratch --merge" has the full SHA of the clone's HEAD
    And the recording gh's call with "pr merge slice-9 --repo owner/scratch --auto" comes before its call with "pr merge slice-9 --repo owner/scratch --merge"
    And the recording gh's "pr merge" call does not have "--rebase"
    And the recording gh was never called as "pr update-branch"
    And its output says "Merged https://github.example/o/r/pull/7 by merge at "

  # A guard: verify judges a merge by its own changes (bug 30), and the one
  # --ready lands, over a branch GitHub brought up to main by a merge, has
  # none, so it passes whatever its subject, beside item commits that pass
  # the rules. It passes before the work and must after it.
  @wip @slice-214 @ID-READYMERGE-03 @guard
  Scenario: A merge commit landed as --ready lands it under push.pr.update merge passes itos verify with the item's commits
    Given the clone is on the new branch "slice-9"
    And the clone has the commit "chore: add the widget" naming the task "T-001" and touching "widget.md"
    And the clone is back on its branch "main"
    And the clone has the commit "chore: tidy the docs" naming the task "T-001" and touching "docs.md"
    And the clone switches to its branch "slice-9"
    And the clone has merged its branch "main" with the merge commit "Merge branch 'main' into slice-9"
    And the clone is back on its branch "main"
    And the clone has merged its branch "slice-9" with the merge commit "slice-9: Add the widget" and the body "Item: slice-9"
    When itos verifies every commit up to HEAD
    Then itos exits with code 0
    And its output says "commits pass the commit rules"

  # A guard: push.pr.update rebase, written out, is today's flow, call for
  # call: update-branch --rebase, auto-merge by rebase, no subject or body.
  # It passes before the work (once slice-213 makes the key known) and must
  # after it.
  @wip @slice-214 @ID-READYMERGE-04 @guard
  Scenario: Under push.pr.update rebase, push --ready asks GitHub to rebase and turns on auto-merge by rebase as before, with no message of its own
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: rebase
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr update-branch" call has "slice-9 --repo owner/scratch --rebase"
    And the recording gh's "pr merge" call has "--repo owner/scratch --auto --rebase --match-head-commit"
    And the recording gh's "pr merge" call does not have "--subject"
    And the recording gh's "pr merge" call does not have "--body"
    And its output says "GitHub rebased the pull request onto origin/main"
    And its output says "Auto-merge is on: GitHub merges https://github.example/o/r/pull/7 by rebase once its required checks pass."
