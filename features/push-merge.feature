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

  Under rebase, or with no push.pr.update, itos push is as it is today; the
  pre-push hook's change below holds in every mode.

  Under merge, itos push from an item's branch fetches main and the item's
  branch as it does today (the fetch of main updating the clone's
  remote-tracking main, which the pre-push hook reads), then, in this order:

  1. It divides the commits the remote's item branch lacks (all of
     <merge-base of HEAD and main's head>..HEAD on a first push) as
     slice-192 does (push-mode-ledger.feature): the registry's commits to
     main, the others to the item's branch, each made again only when a
     registry commit is taken out from under it, on the commit below it that
     stays. Dividing first keeps the commits the remote's item branch has a
     true prefix of the list, which git's order without --topo-order
     guarantees only on linear history. On a first push the item's branch
     starts at the merge-base of HEAD and main's head, never at main's head:
     nothing is rebased onto main.
  2. When registry commits leave the branch, main gets them first, in a push
     of its own, as a branch of registry commits goes (Sent lines and all),
     and the clone's branch is reset to the item's head without them. They
     reach main before any merge is tried, so a conflict below never strands
     a take off main, and the merge the implementer commits never has a
     registry commit below it. The item's branch goes in a later push: the
     two are no longer one git push --atomic, and an item push the hook then
     refuses leaves main with the registry's commits, which is where they go
     whatever happens to the work. Registry commits directly above main that
     stay on the item's branch (slice-192's prefix) go with it as today.
  3. When the remote's item branch has commits the clone's branch lacks, it
     merges them in rather than rebasing onto them, as "Merge <remote>/<item>
     into <item>" ("Merge origin/slice-9 into slice-9"). A branch with no
     commit of its own past the remote's is fast-forwarded to it, with no
     merge commit.
  4. When the remote's main, as the push fetched it, has commits the branch
     lacks, it merges main's head into the branch, as "Merge <main> into
     <item>" ("Merge main into slice-9"): main's head after step 2, so the
     registry's commits it just sent come back to the item's branch under
     main's SHAs. When the branch has main as fetched already, nothing is
     merged, though step 2 moved main: the registry's commits reach the
     item's branch when it lands.

  itos makes each merge with git merge --no-verify --no-ff --no-edit -m
  <message>, so merge.* settings (merge.ff, a merge's editor) change nothing,
  as pull.* changes nothing for the rebase; its commit is judged at the
  pre-push hook, with the rest of the push. The commit rules pass its
  untyped first line: a merge is judged by its own changes (bug 30,
  internal/git/merge.go), and a clean merge has none.

  The push of the item's branch is then a fast-forward: never forced, no
  lease, and no commit the remote's item branch had changes its SHA. The
  clone's branch ends at the head pushed. Since that head has main's head, gh
  pr update-branch is never asked. The same merge of main is what --ready
  pushes before it waits: under merge, slice-214 never calls gh pr
  update-branch. How the pull request lands is slice-214's, and so is the
  auto-merge a push without push.pr.draft turns on: until slice-214, merge
  without push.pr.draft keeps today's auto-merge by rebase.

  A conflict in either merge stops the push before the item's branch is
  pushed (registry commits already sent to main in step 2 stay there): exit
  1, outcome merge-conflict, no gh called, and the merge left in progress in
  the clone for the implementer. The report names each conflicted path and
  what to run: resolve the files, git add them, commit it (itos commit or
  git commit) with a typed first line, since a merge with changes of its own
  needs a commit type (merge-type, as the commit rules already judge such a
  merge) and git's prepared "Merge ..." line has none, then itos push again;
  or git merge --abort. itos push again with the merge committed pushes it, a
  fast-forward like any other: main as fetched is then an ancestor of HEAD,
  and nothing is merged again.

  --onto-main under merge is refused before anything is fetched or pushed,
  exit 1, outcome onto-main-under-merge, saying that push.pr.update is merge,
  so itos push merges main in, and to run itos push. It is refused at the
  point where --onto-main outside push.mode pr is refused today, so --onto-main
  beside --tag or --ref is still a usage error first.

  merge-in-history keeps its reason: carrying a registry commit out of the
  item's branch makes the commits above it again on their first parents,
  which would drop what a merge among them brings in, its resolution
  included. Under merge it is refused only where that would happen: a
  registry commit to carry out that is an ancestor of a merge among the
  commits the push sends (a merge made by hand above a take, say). A registry
  commit no such merge descends from is carried to main as any other, and
  the merge stays on the item's branch under its own SHA: the case of a
  merge itos left for the implementer, committed and not yet pushed, with an
  idea added after it. Under rebase merge-in-history is as it is today.

  The pre-push hook, in every mode (it never reads push.pr.update), judges
  for a ref the remote has the commits <local> ^<base> --not --remotes,
  where <base> is the remote's commit of the ref: the commits a merge of main
  brings in are on the remote already, judged when they reached it, and not
  the pusher's, so a main commit a later rule would refuse does not block the
  merge. A new ref is judged as today, by the commits on no remote-tracking
  branch. The range checks and hooks.pre_push's per_base keep the ref's
  base as their {from}.

  --json's outcome gains merge-conflict and onto-main-under-merge.

  Not in this slice: landing by merge and --ready's wait (slice-214); this
  repository turning push.pr.update on.

  The recording gh and the steps are push-mode.feature's and
  push-mode-ledger.feature's, with four new ones. "the remote's branch
  <name> has the saved <name> commit" requires the commit "the last commit
  SHA is saved as <name>" saved from the clone's HEAD to be an ancestor of
  the head of the remote's branch, the same SHA. "the head of the remote's
  branch <name> is the merge commit <subject>" reads that head's subject and
  requires two parents. "the clone has a merge in progress" finds
  MERGE_HEAD by git rev-parse --git-path MERGE_HEAD in the clone. "the clone
  resolves the conflict in <path>, keeping both sides, and commits the merge
  as <subject>" first finds MERGE_HEAD the same way and fails, touching
  nothing, when there is none; then it is the rebase step's resolution (the
  conflict markers left out, the file staged) and git commit --no-verify -m
  <subject>. The config's push.pr.update is written by the config steps as
  any key.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # Both values pass, rebase written out as the default is. Fails before the
  # work at the exit code: today push.pr.update is an unknown key, exit 2.
  @ID-PUSHMERGE-01 @slice-213
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
  @ID-PUSHMERGE-02 @slice-213
  Scenario: config check refuses a push.pr.update other than rebase or merge, naming both
    Given the config sets "push.pr.update" to "squash"
    When itos checks the config
    Then itos exits with code 2
    And its output says "push.pr.update"
    And its output says "rebase, merge"

  # Rebase by default, so a repository that does not ask pushes as before.
  # Fails before the work at the exit code: today config get refuses the
  # unknown key, exit 2.
  @ID-PUSHMERGE-03 @slice-213
  Scenario: config get prints push.pr.update's default, rebase, when the config leaves it out
    When itos runs "config get push.pr.update"
    Then itos exits with code 0
    And its output says "rebase"

  # The slice's point: main moved past the pushed pull request, and itos
  # push, with nothing new in the clone, merges main in and pushes the merge
  # as a fast-forward: main's commit keeps its SHA on the item's branch, and
  # so does the commit pushed before. The real pre-push hook, installed last
  # so that no Given's push meets it, judges the push: main's commit is a
  # chore with no Task footer, which the scratch rules refuse (it reached
  # main past the hooks), and it is on the remote already, so the hook's
  # range, <local> ^<base> --not --remotes, leaves it out; the merge itos
  # writes passes untyped, having no changes of its own (bug 30). Fails
  # before the work at the exit code: today push.pr.update is an unknown
  # key, so the config cannot be read, the push is a trunk push to main, and
  # the pre-push hook, which cannot read the config either, refuses it.
  # (With the config readable and today's hook, the hook would judge the
  # chore in <base>..<local> and refuse the push.)
  @ID-PUSHMERGE-04 @slice-213
  Scenario: Under merge, a push from an item's branch main has moved past merges main in and pushes a fast-forward, the pre-push hook judging none of main's commits
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
    And the last commit SHA is saved as "pushed"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the build" touching "build.txt"
    And the pre-push hook is installed
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" has the remote main's commit "chore: tidy the build"
    And the remote's branch "slice-9" has the saved "pushed" commit
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"
    And its output says "https://github.example/o/r/pull/7"

  # Bug 158: the merge itos writes holds two sides' changes to neighbouring
  # lines of one file, which git merged cleanly into one hunk of its combined
  # diff, as on slice-209 (internal/providers/doc.go). Its untyped first line
  # passes, since its tree is git's own merge of its parents
  # (commit-msg.feature's bug-158 block). "the remote has gained the commit
  # <subject> writing <path> with the lines:" is "the remote has gained the
  # commit <subject> touching <path>" with <path> holding the docstring's
  # lines, and "the clone has the commit <subject> writing <path> with the
  # lines:" is "the clone has the commit <subject> touching <path>" the same
  # way. Fails before the work at the exit code: the real pre-push hook
  # judges the merge, whose combined diff has widget.md's one hunk, and
  # refuses it under merge-type, so the item's branch is not pushed.
  @bug-158 @ID-PUSHMERGE-16
  Scenario: Under merge, a merge of main whose two sides changed neighbouring lines of one file is pushed untyped, past the pre-push hook
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "docs: describe the widget" writing "widget.md" with the lines:
      """
      one
      two
      three
      four
      five
      """
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" writing "widget.md" with the lines:
      """
      one
      two, the item's
      three
      four
      five
      """
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the widget" writing "widget.md" with the lines:
      """
      one
      two
      three
      four, main's
      five
      """
    And the pre-push hook is installed
    When itos runs "push"
    Then itos exits with code 0
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And the remote's branch "slice-9" has the remote main's commit "chore: tidy the widget"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # A commit someone pushed to the item's branch (the coordinator's fix to
  # the spec, say) is merged in rather than the clone's new commit being
  # rebased onto it: the clone's commit reaches the remote under the SHA it
  # was made with. Fails before the work at the saved commit's step: today
  # the config cannot be read and the trunk push sends the clone's commits,
  # rebased, to main, so the remote's item branch lacks it.
  @ID-PUSHMERGE-05 @slice-213
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
  @ID-PUSHMERGE-06 @slice-213
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
  @ID-PUSHMERGE-07 @slice-213
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
  # goes to main first, in a push of its own, and the merge itos makes comes
  # after, of main's head with the take on it, so merge-in-history never
  # meets the merge and the take reaches the item's branch under main's SHA.
  # Fails before the work
  # at the item branch's main-commit step: today the config cannot be read
  # and the trunk push sends everything to main, the remote's item branch
  # left with the spec alone.
  @ID-PUSHMERGE-08 @slice-213
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
    And the remote's branch "slice-9" has the remote main's commit "docs: take slice-9"
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And its output says "docs: take slice-9 to origin/main."
    And its output does not say "merge-in-history"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # The merge itos left for the implementer, committed and not yet pushed,
  # then an idea added above it: the idea goes to main, and the merge, which
  # does not descend from it and is made again on nothing, goes to the item's
  # branch under its own SHA. Today's merge-in-history refuses any commits
  # that hold a merge and a registry commit; under merge it refuses only a
  # registry commit a merge descends from. The branch has main as fetched
  # already, so nothing is merged after the idea went to main, and the head
  # stays the implementer's merge. Fails before the work at the Given that
  # commits the merge: today the push before it stops in a rebase, not a
  # merge.
  @ID-PUSHMERGE-09 @slice-213
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
  @ID-PUSHMERGE-10 @slice-213
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
  @ID-PUSHMERGE-11 @slice-213 @guard
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

  # The implementer's usual first push, with a conflict: the take above the
  # pushed spec goes to main in a push of its own before any merge is tried,
  # so when the merge of main then stops on widget.md, the take is on main
  # already and the item's branch has neither it nor the work; the merge is
  # left in progress, no gh called. Fails before the work at the outcome:
  # today the config cannot be read and the trunk push's rebase onto main
  # stops on widget.md, rebase-stopped.
  @ID-PUSHMERGE-12 @slice-213
  Scenario: Under merge, a take on the branch reaches main before a merge of main that conflicts stops the push
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
    And the remote has gained the commit "docs: describe the widget" touching "widget.md"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "widget.md"
    And the remote's branch has "docs: take slice-9"
    And the remote's branch "slice-9" does not have "docs: take slice-9"
    And the remote's branch "slice-9" does not have "feat: add the widget"
    And the clone has a merge in progress
    And the recording gh was never called

  # @ID-PUSHMERGE-12 to its end: the implementer resolves the conflict and
  # commits the merge, which has no registry commit below it, the take
  # having left the branch before the merge was tried; itos push again
  # pushes it with no merge-in-history, and the take reaches the item's
  # branch through the merge, under main's SHA. Fails before the work at the
  # Given that commits the merge: today the push before it stops in a
  # rebase, not a merge.
  @ID-PUSHMERGE-13 @slice-213
  Scenario: Under merge, itos push again after a take went to main and the implementer committed the resolved merge pushes the merge
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
    And the remote has gained the commit "docs: describe the widget" touching "widget.md"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And itos has run "push", exiting with code 1
    And the clone resolves the conflict in "widget.md", keeping both sides, and commits the merge as "feat: merge main into slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And its output does not say "merge-in-history"
    And the head of the remote's branch "slice-9" is the merge commit "feat: merge main into slice-9"
    And the remote's branch "slice-9" has the remote main's commit "docs: take slice-9"
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # The order: the take is divided out and sent to main first, then the
  # remote's item commit is merged in. Dividing before that merge keeps the
  # commits the remote's item branch has a prefix of the list; main as
  # fetched is in the branch, so nothing of main is merged, and the take,
  # gone to main, is not on the item's branch. Fails before the work at the
  # head step: today the config cannot be read, the trunk push sends the
  # clone's commits to main, and the remote's item branch ends at "fix: mend
  # the widget".
  @ID-PUSHMERGE-14 @slice-213
  Scenario: Under merge, a take goes to main before the remote's item commit is merged in
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
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the remote's branch "slice-9" does not have "docs: take slice-9"
    And the head of the remote's branch "slice-9" is the merge commit "Merge origin/slice-9 into slice-9"
    And the remote has the branch "slice-9" with "test: cover the widget"
    And its output does not say "merge-in-history"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # A first push, the remote having no item's branch, with main moved since
  # the branch was made: the take goes to main, and the work starts at the
  # merge-base of the branch and main, never at main's head, then main is
  # merged in. Fails before the work at the head step: today the config
  # cannot be read and the trunk push sends everything to main, so the
  # remote has no branch "slice-9".
  @ID-PUSHMERGE-15 @slice-213
  Scenario: Under merge, a first push with a take sends it to main and merges main into the work rather than rebasing it
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
    And the remote has gained the commit "docs: tidy the docs" touching "docs.md"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the head of the remote's branch "slice-9" is the merge commit "Merge main into slice-9"
    And the remote's branch "slice-9" has the remote main's commit "docs: tidy the docs"
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the clone's HEAD is the head of the remote's branch "slice-9"
