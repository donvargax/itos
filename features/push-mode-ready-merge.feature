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
  paths where its tree differs from git's automatic merge of its parents
  (bugs 30 and 158, commit-msg.feature), and a merge
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

  Slice-222: a merge of main that conflicts only because both sides added to
  the registry or a ledger. Two sessions appending there conflict on every
  such merge, by construction (#107), and no person should be needed.

  Where. Only the merge of main that itos push makes under push.pr.update
  merge, a plain push and --ready alike (mergeItem's bringUp of main, in
  update.go, which decides, commits and reports). Not the merge of the
  remote's own item branch, a rebase, git pull or work done's pull. A mode
  with no merge is untouched: push.pr.update rebase, and trunk. What stays
  p1-registry-merge's: those paths, and #107's comment's case, work done
  dropping a task's done_when or an item's why lines right above another
  session's append, whose hunk has base lines and so stops here.

  Which paths. When the merge stops, every conflicted path must be the work
  registry (work.registry) or a ledger file (as ledger.LayoutOf reads
  ledger.files), read from the config as the merge left it, never a path
  pattern. Any other conflicted path, or a conflict with no hunks on these
  paths (modify/delete, a rename), is exactly today's: merge left in progress,
  exit 1, merge-conflict. All or nothing.

  The hunks come from git's diff3 style of each path (git checkout
  --conflict=diff3, or the same text of its index stages), whatever
  merge.conflictStyle says. git's default style refines conflicts: two
  appended items sharing their phase, owner and status lines split into two
  hunks, and keeping each would leave the first item with its id and title
  alone and the second with both whys. diff3 does not refine. keep-both is a
  pure function of that text in internal/git, with a table test: where a
  hunk's base is empty, both sides purely adding at one place, it writes ours
  then theirs, byte for byte, adding no line, since each side already holds
  its own leading blank line. Any hunk with base lines stops: that is an
  item both sides changed, whether the lines are a status (which would parse
  as a key written twice) or a why's text (which would parse and pass). Its
  rows: one hunk; several; a hunk with base lines; markers left unmatched; no
  markers; a side with no final newline; a CRLF file, whose markers carry the
  CR, the result's endings the sides' own.

  The soundness check, in process, is config check's over the result:
  work.Problems and ledger.Findings' problems, all their rules, warnings not
  counted. A hunk with base lines, or an unsound result: git merge --abort,
  exit 1, merge-conflict, naming the paths and why, nothing pushed (registry
  commits already sent to main stay there). No merge is then in progress, so
  it says to run git merge origin/main by hand, resolve it, commit it with a
  typed first line and itos push again.

  The queue is a block list, so the rule above is its rule: ours then theirs
  where both sides add at one place. An id both sides queued at one place is
  git's single line, no conflict. Held twice by the kept hunks, it is
  work-queue-twice, and the merge aborts. A dequeue or a move beside another
  edit has base lines and stops, so keeping both never brings back an id a
  side took out. Git may still merge one id queued at two neighbouring places
  cleanly, twice: no conflict, so nothing here runs.

  The commit. Its type is read from the config, before anything is touched:
  docs first, as itos's registry writers type theirs, then the types the
  registry footer stands in for (in_place_of), in the config's order; the
  first whose path rules pass the conflicted paths (scope.Rules.Issues) and
  that message.Missing finds nothing missing for. feat and fix are never
  candidates: a merge is no release. None fits: today's behaviour, merge left
  in progress. The message is "<type>: merge main into <item>", with the
  registry footer ("Item: <item>") when the config has one and the merged
  registry holds the item. The type's helper is push's own, in update.go:
  slice-221 needs the same reading and is not in yet, so sharing it is for
  later. It commits past the hooks, as bringUp's own merge does, and is judged
  at pre-push. Its own paths (bugs 158, 159) are the conflicted paths and
  nothing else, so it passes as a typed by-hand merge passes.

  What it says: "Merged main into <item>, keeping both sides of <paths>.", and
  --json gains kept_both, the list of paths, only when it kept any.

  Line endings: keep-both adds no line, so it cannot mix them. The table's CRLF
  row and a contract test of the port's diff3 text under core.autocrlf=true
  hold it, on every OS; no scenario does. So do update_test.go's unit tests
  hold what needs no real git: no type fits, left in progress; the kept hunks
  holding a queue id twice, aborted; a modify/delete on the registry, left in
  progress. The type's table: this repository's config gives docs; docs and
  test barred with an Item footer gives chore; feat never.

  It lands as a feat, Upgrading: none: no released scenario or corpus case
  pins a registry or ledger conflict as merge-conflict, so no Changes. The
  implementing push updates itos push --help, doc.go's "push.pr.update merge"
  passage, and push-merge.feature's paragraph on a conflict in either merge.

  Slice-222's steps. Four are new. "the config's <type> commits may touch only
  <glob>" sets commits.scopes.<type>.only in the scratch config, in place of
  docs' own line for docs. "the file <path> on the remote's branch <name> has
  exactly the lines:" compares that blob at the branch's head with the
  docstring and a final newline. "the clone has no merge in progress" is the
  opposite of slice-213's. "the remote's branch <name> has gained the commit
  <subject> adding the idea <id> to the registry" is remoteBranchGains with
  addIdeaIn for its edit. The registries are written out with slice-213's
  "writing <path> with the lines:" where a scenario needs block items; the
  Background's are flow lines, one line an item.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo" and the title "Add the widget"

  # The slice's point: main moved, so --ready merges it in as a push does,
  # never asking gh pr update-branch, and turns on auto-merge by merge,
  # pinned to the merge head it pushed, with the message itos writes: the
  # item's title and the registry footer.
  @slice-214 @ID-READYMERGE-01
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
  @slice-214 @ID-READYMERGE-02
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

  # Slice-222's point, on block items as a real registry has them, which is
  # what git's default style would split: both sides queue an id at the top
  # and append an item. Each hunk's base is empty, so both are kept, ours
  # first, adding no line; the file is compared whole. The real pre-push hook
  # judges the merge, whose own path is the registry, and verify agrees. The
  # hook is installed last, so no Given's push meets it. Fails before the work
  # at the exit code: the merge stops and is left in progress, exit 1.
  @wip @slice-222 @ID-READYMERGE-07
  Scenario: Under push.pr.update merge, a merge of main where both sides only added to the registry keeps both, ours first, and pushes a typed merge past the pre-push hook and verify
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "docs: write the registry out" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      queue:
        - slice-9

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice
      """
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: add p1-mine" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      queue:
        - p1-mine
        - slice-9

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-mine
          title: Mine
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            My reason.
      """
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: add p1-theirs" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      queue:
        - p1-theirs
        - slice-9

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-theirs
          title: Theirs
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            Their reason.
      """
    And the pre-push hook is installed
    When itos runs "push --json"
    Then itos exits with code 0
    And its JSON's "kept_both" has a line saying "tasks/work-items.yaml"
    And the head of the remote's branch "slice-9" is the merge commit "docs: merge main into slice-9"
    And the remote's branch "slice-9" has the remote main's commit "docs: add p1-theirs"
    And the clone's HEAD is the head of the remote's branch "slice-9"
    And the file "tasks/work-items.yaml" on the remote's branch "slice-9" has exactly the lines:
      """
      phases:
        1: null

      queue:
        - p1-mine
        - p1-theirs
        - slice-9

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-mine
          title: Mine
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            My reason.

        - id: p1-theirs
          title: Theirs
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            Their reason.
      """
    When itos verifies every commit up to HEAD
    Then itos exits with code 0
    And its output says "commits pass the commit rules"

  # The type is read from the config. docs may not touch the registry here,
  # and test, which the Item footer stands in for and comes first, may not
  # either, so the merge is a chore. The Item footer is what lets a chore pass:
  # it needs a Task or an Item, and the real pre-push hook judges it, which
  # refuses a docs merge under scope-only and an untyped one under merge-type.
  # Fails before the work at the exit code: the merge stops, exit 1.
  @wip @slice-222 @ID-READYMERGE-08
  Scenario: Under push.pr.update merge, keeping both in a config whose docs commits may not touch the registry commits the merge under the type the config lets it take
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the config's "docs" commits may touch only "docs/**"
    And the config's "test" commits may touch only "features/**"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the remote has gained the commit "docs: add p1-theirs" adding the idea "p1-theirs" to the registry
    And the clone has the commit "docs: add p1-mine" adding the idea "p1-mine" to the registry
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the pre-push hook is installed
    When itos runs "push"
    Then itos exits with code 0
    And the head of the remote's branch "slice-9" is the merge commit "chore: merge main into slice-9"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # An item both sides changed. Both rewrite one line of its why, so the hunk
  # has a base line and the id is nowhere in it. Kept both, it would parse and
  # pass every rule (a status changed on both sides is a table row: it would
  # only be a key written twice), so this is the case the empty-base rule
  # alone stops. Aborted, not left in progress: the clone is back where it was.
  # Fails before the work at "both sides changed": today's words say to
  # resolve the files, and the merge is left in progress.
  @wip @slice-222 @ID-READYMERGE-09
  Scenario: Under push.pr.update merge, a merge of main where both sides changed one item is aborted, exit 1, saying why and how to go on, nothing pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "docs: write the registry out" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-gadget
          title: Add the gadget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            The first reason.
      """
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: edit p1-gadget here" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-gadget
          title: Add the gadget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            The reason as the item's branch has it.
      """
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: edit p1-gadget there" writing "tasks/work-items.yaml" with the lines:
      """
      phases:
        1: null

      items:
        - id: slice-9
          title: Add the widget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: slice

        - id: p1-gadget
          title: Add the gadget
          phase: 1
          owner: null
          status: todo
          depends_on: []
          kind: idea
          why: >
            The reason as main has it.
      """
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "tasks/work-items.yaml"
    And its output says "both sides changed"
    And its output says "git merge origin/main"
    And the clone has no merge in progress
    And the clone's HEAD is the commit "docs: edit p1-gadget here", with no rebase in progress
    And the remote's branch "slice-9" does not have "docs: edit p1-gadget there"
    And the recording gh was never called

  # The soundness check. Both sides mint one id, so each hunk's base is empty
  # and keep-both alone would land it; config check's rules refuse the result,
  # and the merge aborts naming the rule's words. Fails before the work at
  # "listed twice": today the merge is left in progress, with no such words.
  @wip @slice-222 @ID-READYMERGE-10
  Scenario: Under push.pr.update merge, a merge of main whose kept sides would hold one id twice is aborted, exit 1, naming the problem, nothing pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "docs: write the registry out" writing "tasks/work-items.yaml" with the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: Add the widget, phase: 1, owner: null, status: todo, kind: slice, depends_on: [] }
      """
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: add p1-same here" writing "tasks/work-items.yaml" with the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: Add the widget, phase: 1, owner: null, status: todo, kind: slice, depends_on: [] }
        - { id: p1-same, title: Mine, phase: 1, owner: null, status: todo, kind: idea, depends_on: [] }
      """
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: add p1-same there" writing "tasks/work-items.yaml" with the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: Add the widget, phase: 1, owner: null, status: todo, kind: slice, depends_on: [] }
        - { id: p1-same, title: Theirs, phase: 1, owner: null, status: todo, kind: idea, depends_on: [] }
      """
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "tasks/work-items.yaml"
    And its output says "p1-same: listed twice"
    And the clone has no merge in progress
    And the clone's HEAD is the commit "docs: add p1-same here", with no rebase in progress
    And the remote's branch "slice-9" does not have "docs: add p1-same there"
    And the recording gh was never called

  # What a ledger file is comes from the config: here its files are outside
  # tasks/, so a build reading a path pattern stops. A ledger file alone
  # conflicts, which registryPaths does not count as the registry's. docs may
  # touch them in this config, or no type would fit. Fails before the work at
  # the exit code: the merge stops, exit 1.
  @wip @slice-222 @ID-READYMERGE-11
  Scenario: Under push.pr.update merge, a merge of main where both sides only appended to a ledger file the config names keeps both and says so
    Given the ledger's files are "data/phase-{group}.yaml"
    And the config's "docs" commits may touch only "data/**"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          update: merge
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "docs: move the ledger" writing "data/phase-1.yaml" with the lines:
      """
      - { id: T-001, type: chore, title: Tidy }
      """
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: add T-002" writing "data/phase-1.yaml" with the lines:
      """
      - { id: T-001, type: chore, title: Tidy }
      - { id: T-002, type: chore, title: Sweep }
      """
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: add T-003" writing "data/phase-1.yaml" with the lines:
      """
      - { id: T-001, type: chore, title: Tidy }
      - { id: T-003, type: chore, title: Dust }
      """
    When itos runs "push"
    Then itos exits with code 0
    And its output says "keeping both sides of data/phase-1.yaml"
    And the head of the remote's branch "slice-9" is the merge commit "docs: merge main into slice-9"
    And the file "data/phase-1.yaml" on the remote's branch "slice-9" has exactly the lines:
      """
      - { id: T-001, type: chore, title: Tidy }
      - { id: T-002, type: chore, title: Sweep }
      - { id: T-003, type: chore, title: Dust }
      """

  # --ready merges main in as a push does, so it keeps both too, and the head
  # it judges and pins is the merge it made: the case bug-157's close took four
  # tries over by hand. Fails before the work at the exit code: the merge stops
  # before --ready waits for anything, exit 1.
  @wip @slice-222 @ID-READYMERGE-12
  Scenario: Under push.pr.update merge, push --ready keeps both sides of a registry conflict and turns on auto-merge at the merge it pushed
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
    And the remote has gained the commit "docs: add p1-theirs" adding the idea "p1-theirs" to the registry
    And the clone has the commit "docs: add p1-mine" adding the idea "p1-mine" to the registry
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the head of the remote's branch "slice-9" is the merge commit "docs: merge main into slice-9"
    And the remote's branch "slice-9" has the remote main's commit "docs: add p1-theirs"
    And the fake GitHub was asked for the run of the clone's HEAD
    And the recording gh's "pr merge" call with "--auto" has the full SHA of the clone's HEAD
    And the recording gh was never called as "pr update-branch"

  # A guard: all or nothing, and by the config's names. The registry's hunk is
  # one keep-both would take, and the other conflicted path sits under tasks/
  # and is no ledger file. So a build reading a path pattern lands both, at the
  # exit code; one resolving the registry alone no longer names it, at its
  # step; one aborting has no merge in progress. It passes before the work,
  # today's behaviour, and must after it.
  @wip @slice-222 @ID-READYMERGE-13 @guard
  Scenario: Under push.pr.update merge, a merge of main conflicting in the registry and another file under its folder is left in progress with both unresolved, as before
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
    And the remote has gained the commit "docs: add p1-theirs" adding the idea "p1-theirs" to the registry
    And the remote has gained the commit "docs: write their notes" touching "tasks/notes.md"
    And the clone has the commit "docs: add p1-mine" adding the idea "p1-mine" to the registry and touching "tasks/notes.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "tasks/work-items.yaml"
    And its output says "tasks/notes.md"
    And the clone has a merge in progress
    And the remote's branch "slice-9" does not have "docs: add p1-theirs"
    And the recording gh was never called

  # A guard: the merge of the remote's own item branch is not this. Someone
  # pushed there, so resolving it needs a look; main has not moved. A build
  # keeping both in any merge lands it, at the exit code. It passes before the
  # work and must after it.
  @wip @slice-222 @ID-READYMERGE-14 @guard
  Scenario: Under push.pr.update merge, a merge of the remote's own item branch conflicting in the registry is left in progress, as before
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
    And the remote's branch "slice-9" has gained the commit "docs: add p1-theirs" adding the idea "p1-theirs" to the registry
    And the clone has the commit "docs: add p1-mine" adding the idea "p1-mine" to the registry and touching "spec.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "merge-conflict"
    And its output says "tasks/work-items.yaml"
    And its output says "origin/slice-9"
    And the clone has a merge in progress
    And the recording gh was never called

  # A guard: with no push.pr.update there is no merge, so nothing here runs,
  # and the same conflict stops the rebase, for the person. A build keeping
  # both in a rebase lands it, at the exit code. It passes before the work and
  # must after it. Trunk's rebase is the same code, so no scenario of its own.
  @wip @slice-222 @ID-READYMERGE-15 @guard
  Scenario: Without push.pr.update, the same registry conflict stops the rebase, left in progress, as before
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
    And the remote's branch "slice-9" has gained the commit "docs: add p1-theirs" adding the idea "p1-theirs" to the registry
    And the clone has the commit "docs: add p1-mine" adding the idea "p1-mine" to the registry and touching "spec.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "rebase-stopped"
    And the clone has a rebase in progress
    And the recording gh was never called

  # A guard: verify judges a merge by its own changes (bug 30). The landing
  # as it happens under merge: the item's commits carry their Item: footer,
  # validated against the registry, itos's merge of main into the branch and
  # the merge GitHub lands on main titled after the item have no changes of
  # their own. It passes before the work and must after it.
  @slice-214 @ID-READYMERGE-03 @guard
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
  @slice-214 @ID-READYMERGE-04 @guard
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
  @slice-214 @ID-READYMERGE-05
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
  @slice-214 @ID-READYMERGE-06
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
