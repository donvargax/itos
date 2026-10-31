@phase-1
Feature: Under push.pr.update merge, itos push --ready brings the pull request up to main by a merge and lands it as a merge commit itos titles
  Under push.mode pr with push.pr.draft (push-mode-draft.feature) itos push
  --ready brings a pull request main has moved past up to date with gh pr
  update-branch --rebase and lands it with gh pr merge --auto --rebase
  --match-head-commit <the head judged>, or, when GitHub refuses auto-merge
  as "is in clean status", one gh pr merge --rebase --match-head-commit; a
  plain push without push.pr.draft turns on auto-merge with gh pr merge
  --auto --rebase. Each lands the item by rewriting its commits, and the
  update is a rewrite of the item's branch, as was every command the person
  had to run by hand on 2026-10-30: Claude Code's classifier refuses agents
  gh pr update-branch --rebase, itos push --onto-main and a force push to
  reword. A merge rewrites nothing. The person settled it for the pilot
  (docs/plans/v9-coordinator.md, Landing under pr, open question 1;
  p1-pr-merge-mode): merges, not rebases, opt in, so v8's flow stays the
  default. Slice-213 (push-merge.feature) adds the key, push.pr.update
  (rebase, the default, or merge), and has itos push merge main into the
  item's branch locally and push the merge as a fast-forward; this slice is
  how the pull request lands.

  Under push.pr.update merge:
  1. The update. --ready brings the branch up to main as a plain push does,
     by itos's own merge of main (slice-213), and never asks gh pr
     update-branch: one mechanism, explicit and local (p1-v9-explicit-steps).
     The head --ready pushed then has main's head, so the head it judges,
     watches the run of and pins is the head it pushed (the branch's head
     when nothing was pushed), as for a pull request main had not moved past.
     The "rebase waiting" stop and GitHub's rebased head never arise.
  2. The merge. --ready's call is gh pr merge <item> --repo <repository>
     --auto --merge --subject=<subject> --body=<body> --match-head-commit
     <the head judged>, and the clean-status fallback gh pr merge <item>
     --repo <repository> --merge --subject=<subject> --body=<body>
     --match-head-commit <the head judged>, in that order, the values joined
     to their flags as gh pr create's --title= and --body= are, so a title
     that starts with a dash is never read as a flag. A plain push without
     push.pr.draft, which turns on auto-merge itself (pr.go, pullRequest),
     calls gh pr merge <item> --repo <repository> --auto --merge
     --subject=<subject> --body=<body>, and, as today, no
     --match-head-commit.
  3. The subject is the pull request's title, computed as describe computes
     it for gh pr create (push-mode-title.feature): "<item>: <the item's
     title>", read from the registry at the head the push sends (the
     branch's head when nothing was pushed), cut to 255 characters and an
     ellipsis past what GitHub keeps, the item's id alone when the registry
     there gives it no title. It is never read from GitHub: a pull request
     retitled by hand lands under the registry's title.
  4. The body is the item's footer: the key of the first footer in
     commits.footers whose source is the work registry, as itos commit
     --item picks it, then ": <item>" ("Item: slice-9"), alone, so a reader
     of main's first parents gets the item from the merge commit without
     parsing the branch. A config with no such footer passes no --body, and
     GitHub writes its own.
  5. The words. What says rebase says merge: "Auto-merge is on: GitHub
     merges <address> by merge once its required checks pass."; "Merged
     <address> by merge at <short>: ..." (both endings); and ghFailed's
     advice on a plain push, "turns on auto-merge (gh pr merge --auto
     --merge)".
  Under push.pr.update rebase, or none, every call and every line is as
  today: the guard below keeps that.

  verify needs nothing new: it judges a merge commit by its own changes, the
  paths of its dense combined diff (bug 30, commit-msg.feature), and a merge
  of a branch up to date with main has none, so it passes whatever its
  message, and the item's commits beside it, each with its Item: footer,
  are judged as any commit is. A guard below holds it for the landing.

  The implementing push updates itos push --help (command.go): the
  push.mode pr paragraph, where it turns on auto-merge by rebase (gh pr
  merge --auto --rebase) and asks gh pr update-branch --rebase, and the
  --ready paragraph, where it asks gh pr update-branch --rebase and merges
  with gh pr merge --auto --rebase --match-head-commit or one gh pr merge
  --rebase --match-head-commit; and doc.go's passages on pullRequest's
  auto-merge by rebase and its update-branch, on --ready's update when main
  moved, and on merge.go's merge by rebase and its clean-status fallback.
  Each gains what push.pr.update merge does instead.

  Not in this slice: a plain push's merging main in (slice-213); this
  repository turning push.pr.update merge on (a later build commit); itos
  next and work show reading merges.

  The steps. "the config, committed and pushed to the remote's main, adds
  the YAML:" gives push.pr.update as slice-213 reads it, and commits the
  Item footer that "the config has an Item footer of registry items, taken
  in place of Task for test, docs and chore", written before it, adds.
  "the head of the remote's branch <name> is the merge commit <subject>" is
  slice-213's. Nothing else is new.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo" and the title "Add the widget"

  # The slice's point: main moved, so --ready merges it in as a push does,
  # never asking gh pr update-branch, and turns on auto-merge by merge,
  # pinned to the merge head it pushed, with the message itos writes: the
  # item's title and the registry footer.
  @wip @slice-214 @ID-READYMERGE-01
  Scenario: Under push.pr.update merge, push --ready on a draft main has moved past merges main in, pushes, and turns on auto-merge by merge at the head it pushed, titled after the item
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the config, committed and pushed to the remote's main, adds the YAML:
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
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And the recording gh was never called as "pr update-branch"
    And the fake GitHub was asked for the run of the clone's HEAD
    And the recording gh's "pr merge" call has "--repo owner/scratch --auto --merge --subject=slice-9: Add the widget --body=Item: slice-9 --match-head-commit"
    And the recording gh's "pr merge" call with "--auto" has the full SHA of the clone's HEAD
    And the recording gh's "pr merge" call does not have "--rebase"
    And its output says "Auto-merge is on: GitHub merges https://github.example/o/r/pull/7 by merge once its required checks pass."

  # The clean-status race under merge: GitHub refuses auto-merge on a pull
  # request mergeable already, and --ready merges it once itself, by merge,
  # with the same message, pinned to the head it judged (the clone's HEAD:
  # main has not moved).
  @wip @slice-214 @ID-READYMERGE-02
  Scenario: Under push.pr.update merge, push --ready whose auto-merge GitHub refuses as clean merges once itself by merge, titled after the item, and says it merged
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the config, committed and pushed to the remote's main, adds the YAML:
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

  # A guard: verify judges a merge by its own changes (bug 30). The landing
  # as it happens under merge: the item's commits carry their Item: footer,
  # validated against the registry, itos's merge of main into the branch and
  # the merge GitHub lands on main titled after the item have no changes of
  # their own. It passes before the work and must after it.
  @wip @slice-214 @ID-READYMERGE-03 @guard
  Scenario: A merge commit landed as --ready lands it under push.pr.update merge passes itos verify with the item's commits and their Item footers
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And the clone is on the new branch "slice-9"
    And the commit "docs: specify slice-9, the widget" naming the task "slice-9" in the footer "Item" on top of it
    And the commit "test: cover the widget" naming the task "slice-9" in the footer "Item" on top of it
    And the clone is back on its branch "main"
    And the clone has the commit "chore: tidy the docs" naming the task "T-001" and touching "docs.md"
    And the clone switches to its branch "slice-9"
    And the clone has merged its branch "main" with the merge commit "Merge main into slice-9"
    And the clone is back on its branch "main"
    And the clone has merged its branch "slice-9" with the merge commit "slice-9: Add the widget"
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

  # Without push.pr.draft a plain push turns on auto-merge itself: under
  # merge, by merge, with itos's message, and, as today, pinned to no head
  # (a push's auto-merge waits for nothing to judge).
  @wip @slice-214 @ID-READYMERGE-05
  Scenario: Under push.pr.update merge without push.pr.draft, the first push from an item's branch turns on auto-merge by merge, titled after the item
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "slice-9 --repo owner/scratch --auto --merge --subject=slice-9: Add the widget --body=Item: slice-9"
    And the recording gh's "pr merge" call does not have "--rebase"
    And the recording gh's "pr merge" call does not have "--match-head-commit"

  # The body is the registry footer's, never a hardcoded Item: a config
  # with no footer of registry items passes no --body, and GitHub writes its
  # own; the subject is still the item's title.
  @wip @slice-214 @ID-READYMERGE-06
  Scenario: Under push.pr.update merge, push --ready with no registry footer in the config lands by merge titled after the item, passing no body
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
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "--repo owner/scratch --auto --merge --subject=slice-9: Add the widget --match-head-commit"
    And the recording gh's "pr merge" call does not have "--body"
