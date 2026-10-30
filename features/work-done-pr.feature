@phase-1
Feature: Under push.mode pr, work done closes an item only once its pull request has merged and main's run of it is green
  Slice 174 made push.mode pr: an item's work goes on a branch named for
  the item and reaches main through a pull request GitHub merges by rebase
  (new SHAs, each commit's footers kept), while registry commits go straight
  to main. It left work done unchanged, which under pr closes too early or
  never: from main, an item with no commit of its own in HEAD's history is
  judged by HEAD and closes before its pull request merges; run from the
  item's branch, its commits are judged by the pull request's run, and the
  close lands on the branch, above the work, where it rides the pull request.

  Under push.mode trunk, or with no push.mode, work done is what it was: it
  calls no gh and asks nothing of the remote it did not ask before. What
  follows is pr's alone.

  Main is asked of the remote as itos push asks it: the remote is HEAD's
  branch's upstream's, else origin, and main the branch its HEAD names (git
  ls-remote --symref <remote> HEAD), which the remote must have. work done
  runs on main, since its close is a registry commit, which goes straight to
  main, and its checks (the scenarios at HEAD, a task's checks on HEAD's
  tree) judge HEAD. So once work.Done has judged the registry, and before any
  other check, a HEAD on any other branch, or on none, is refused, exit 1,
  rule work-done-not-main, nothing written or asked of gh: "push.mode is pr,
  so work done runs on main, where its close goes, and HEAD is on <branch>"
  ("and HEAD is on no branch" when detached), fix "switch to main with git
  switch <main>, bring it up to date with git pull, then run itos work done
  <id> again". Refusing is simpler than judging main's state from the
  branch: that would read the scenarios, the task's checks and the item's
  commits at a ref other than HEAD, and still write the close on the branch.
  A remote that cannot be asked exits as itos push's fetch-failed does, 75
  when git cannot reach it and 3 otherwise, git's words on stderr, rule
  work-done-remote-failed: "push.mode is pr, so work done runs on main, and
  reading <remote>'s HEAD failed", fix "fix that, then run itos work done
  <id> again". A remote whose HEAD names no branch it has exits 3, rule
  work-done-no-main: "push.mode is pr, so work done runs on main, and
  <remote>'s HEAD names no branch <remote> has", fix "set the remote's
  default branch, then run itos work done <id> again".

  Then, after the scenarios at HEAD are found live (work-done-wip, as
  before), work done asks GitHub for the item's pull requests, with gh
  naming the repository with --repo as itos push does (ci.range.github's
  repository_env variable, else the remote's URL): gh pr list --repo
  <repository> --head <id> --state all --json
  state,mergedAt,number,url,headRefOid (slice 195, below, adds mergeCommit
  to the list). It asks every time, commits on main or not, since an item whose earlier work
  merged can have more in an open pull request. Of the pull requests listed,
  an open one counts first, then a merged one (mergedAt set), then one
  closed unmerged (state CLOSED, mergedAt null); among several of one kind,
  the first gh lists, its newest.

  An open pull request refuses the close whatever main holds, exit 1, rule
  work-done-pr-open: "<id>'s pull request <url> is open, so its work is not
  on <remote>/<main> and <id> is not done", fix "wait for it to merge, then
  run itos work done <id> again".

  Otherwise the item's commits on main decide. They are its own commits as
  slice 127 finds them in HEAD's history (by their footers, its registry
  commits aside), and the newest of them counts only when the remote's main
  as the clone last fetched it, <remote>/<main>, has it: under pr work
  reaches main only through a pull request, so a commit merged into main
  here alone has not landed. That needs no fetch: git pull, which brings in
  the merge, updates <remote>/<main> too. With such a commit, the item is
  judged as before, by that commit: unpushed finds nothing, the task's
  checks run, and with ci.watch its run is looked for, any run covering it
  that passed (slice 127), else ciwatch.Watch waits for it as it waits under
  trunk, a red one refused (work-done-ci, exit 1), one that does not end
  exit 75; without ci.watch CI is not checked, as before, said on stderr.
  The merge gave the commits new SHAs, so the run that judges them is
  main's, unless slice 181 (work-done-pr-trust.feature) lets the merged
  pull request's head's run judge them. Waiting rather than refusing while
  main's run is going is trunk's behaviour, reused unchanged.

  With no such commit, the pull request's kind gives the refusal, exit 1:
  - merged: rule work-done-pr-merged, "<id>'s pull request <url> has merged,
    but HEAD has no commit of <id> that <remote>/<main> has", fix "bring main
    up to date with git pull, then run itos work done <id> again" (slice
    195, below, keeps these words for a clone a pull would bring something
    and gives others when it would bring nothing);
  - closed unmerged: rule work-done-pr-closed, "<id>'s pull request <url> was
    closed without merging, so its work is not on <remote>/<main> and <id>
    is not done", fix "push its work again from the branch <id> with itos
    push, which opens a new pull request, then run itos work done <id>
    again";
  - none: rule work-done-no-pull-request, "<remote>/<main> has no commit of
    <id> and <id> has no pull request, so its work has not landed", fix
    "push its work from the branch <id> with itos push, which opens its pull
    request, then run itos work done <id> again". Under pr an item's work
    reaches main only through its pull request, so an item with neither has
    nothing landed; one that needs no work is dropped (itos work drop), not
    done.

  Slice 195: what the item's merged pull request merged is its own. Under
  pr an item's work reaches main only through its pull request, so what
  that merged is the item's work whatever its footers name. Bug 146's fix
  was two ci commits, which must name a task, so both named its gate task
  (Task: T-333) and neither named bug-146; its pull request merged, and
  work done refused it on a main that had them, as work-done-pr-merged,
  advising a git pull that changed nothing: an item with no commit of its
  own had no way to done. So the item's own commits are also the commits
  of the merged pull request picked above, found by the commit it merged
  as: gh's mergeCommit (its oid), which the one gh pr list call now asks
  for (--json state,mergedAt,number,url,headRefOid,mergeCommit), the
  commit GitHub moved the base branch to, for a rebase merge the last of
  the pull request's commits made anew. Only the newest of the item's own
  commits is judged, so that commit stands for them all: it counts when
  HEAD's history has it, and is the newest when no commit its footers link
  to the item comes after it there. A merge commit HEAD lacks, none (null,
  missing or empty), or no merged pull request adds nothing. The rest is
  as above, unchanged: the newest counts only when <remote>/<main> has it,
  and is then judged as any item's commit is (unpushed, a task's checks,
  main's run or slice 181's head's run, the close's body naming it). Only
  the merged pull request's commits join: commits that name a task, or
  anything but the item, are not its own however they reached main, so an
  item with no pull request is refused work-done-no-pull-request as
  before, and an open pull request refuses first, whatever merged before.

  And the refusal of a merged pull request advises git pull only when a
  pull would bring something: when HEAD or <remote>/<main> lacks the
  commit <remote>'s main names, as work done asked it of the remote above
  (git ls-remote, no fetch), the clone lacking that commit included. Then
  it says why: "<id>'s pull request <url> has merged, but HEAD has no
  commit of <id> that <remote>/<main> has, and <remote>'s <main> has moved
  since this clone last pulled it", the fix as before, "bring main up to
  date with git pull, then run itos work done <id> again". Otherwise the
  clone has the remote's main as it is now, and a pull brings nothing: the
  pull request merged elsewhere (another base branch), or gh named no
  commit it merged as. Then: "<id>'s pull request <url> has merged, but
  HEAD has no commit of <id> that <remote>/<main> has, though this clone
  has pulled <remote>'s <main> as it is now, so the pull request did not
  merge into <main>", fix "push its work again from the branch <id> with
  itos push, which opens a new pull request, then run itos work done <id>
  again". Both stay rule work-done-pr-merged, exit 1.

  With no gh on the PATH, a gh call that fails, or an answer that is not a
  list of pull requests, work done exits 3, nothing written, rule
  work-done-pull-request-failed, in push's pull-request-failed words:
  "looking for <id>'s pull request failed: gh pr list failed: <reason>",
  the reason "gh is not on the PATH", gh's words, or "its answer is not a
  list of pull requests: <error>", fix "fix that, then run itos work done
  <id> again". With no GitHub repository to name, exit 3, rule
  work-done-no-github-repository: "push.mode is pr, so work done asks for
  <id>'s pull request, and <remote> names no GitHub repository to ask in",
  fix "set <repository_env> to owner/name, or point the remote at a GitHub
  repository, then run itos work done <id> again".

  --json gives each refusal as work done's problems, with the rule named
  here; nothing else in its JSON changes.

  Not in this slice: a pull request opened from a fork; the close of a task with
  checks, which also touches its ledger, so itos push refuses it on main as
  work (slice 174's work-on-main).

  The recording gh is push-mode.feature's, first on the PATH with
  GITHUB_REPOSITORY owner/scratch. This slice has it answer pr list by its
  --state: open (gh's default) the open pull requests of the --head branch
  as before, all every one of them; each entry gives number, url, state
  (OPEN, MERGED or CLOSED) and mergedAt (null unless merged). A pull request
  it opens is OPEN. Slice 195 has it give mergeCommit, when --json asks
  for it, as {"oid": <sha>} for a pull request a step merged naming its
  merge commit, the head a branch of the remote had when the step ran, and
  null otherwise. The fake GitHub is ci.watch's (watch.feature). An item's
  commit is one with an Item: footer naming it, the config's registry footer
  turned on, as slice 127's scenarios make it.

  The implementing commit also rewords push-mode.feature's description
  where it says work done is unchanged under pr and names p1-pr-items: it
  points to this slice (slice-176) instead.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: from main, an item whose work is only on its branch,
  # its pull request open, is no longer judged by HEAD. The one gh call is
  # pinned here. Fails before the work at the exit code: today HEAD is
  # judged, CI is not watched, and slice-9 closes, exit 0.
  @ID-PRDONE-01 @slice-176
  Scenario: Under pr, work done from main refuses an item whose pull request is open, naming it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 is open"
    And the recording gh's "pr list" call has "--head slice-9"
    And the recording gh's "pr list" call has "--state all"
    And every call of the recording gh has "--repo owner/scratch"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Work merged earlier does not close an item with more work still in an
  # open pull request: gh is asked whatever main holds. Fails before the
  # work at the exit code: today the item's commit on main is judged and
  # slice-9 closes, exit 0.
  @ID-PRDONE-02 @slice-176
  Scenario: Under pr, work done refuses an item with an open pull request though main has a commit of it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has the item "slice-9"'s commit touching "README.md"
    And the recording gh has the pull request "https://github.example/o/r/pull/6" merged for the branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 is open"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Slice 195 makes a merged pull request's commits the item's own, and the
  # open one still refuses first: here an earlier pull request merged as
  # main's head, a commit naming only a task, which slice 195 would count.
  # A guard: it passes before the work, since the open pull request refuses
  # whatever main holds, and must after it.
  @ID-PRDONE-13 @slice-195 @guard
  Scenario: Under pr, work done refuses an item with an open pull request though an earlier one merged as main's head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "ci: gate the fix" naming the task "T-001" and touching "widget.md"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    And the recording gh has the pull request "https://github.example/o/r/pull/6" merged for the branch "slice-9", its merge commit the head of the remote's branch "main"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 is open"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # GitHub merged the pull request, but this clone's main has not pulled the
  # merge, so its commits, scenarios and checks are not HEAD's to judge.
  # Fails before the work at the exit code: today HEAD is judged and slice-9
  # closes, exit 0.
  @ID-PRDONE-03 @slice-176
  Scenario: Under pr, work done refuses an item whose pull request merged into a main the clone has not pulled, naming git pull
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's main has gained the item "slice-9"'s commit touching "widget.md", which the clone lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 has merged"
    And its output says "git pull"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # The pull request merged as a commit naming only a task, which the
  # remote's main gained and this clone has not pulled: a pull brings it,
  # so git pull is advised, saying why. Fails before the work at the step
  # on why: today's words name the pull and not that main has moved.
  @ID-PRDONE-15 @slice-195
  Scenario: Under pr, work done refuses an item whose pull request merged into a main the clone has not pulled, saying main has moved
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "ci: gate the fix" naming the task "T-001" and touching "widget.md"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9", its merge commit the head of the remote's branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 has merged"
    And its output says "origin's main has moved since this clone last pulled it"
    And its output says "git pull"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # The clone has the remote's main as it is now, and the pull request
  # merged as a commit main lacks (into another branch, here the one it
  # came from): a pull brings nothing, so none is advised, and the fix is a
  # new pull request. Fails before the work at the message, which today
  # says nothing of the pull, and its fix is "bring main up to date with
  # git pull".
  @ID-PRDONE-16 @slice-195
  Scenario: Under pr, work done --json refuses an item whose pull request merged outside an up-to-date main without advising git pull
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9", its merge commit the head of the remote's branch "slice-9"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 1
    And its JSON's problem "work-done-pr-merged"'s "message" says "has pulled origin's main as it is now"
    And its JSON's problem "work-done-pr-merged"'s "fix" says "itos push"
    And its output does not say "git pull"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # A pull request closed unmerged never put the work on main. --json names
  # the rule. Fails before the work at the exit code: today HEAD is judged
  # and slice-9 closes, exit 0.
  @ID-PRDONE-04 @slice-176
  Scenario: Under pr, work done --json refuses an item whose pull request was closed without merging
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" closed without merging for the branch "slice-9"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 1
    And its JSON's problem "work-done-pr-closed"'s "message" says "https://github.example/o/r/pull/7 was closed without merging"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Commits merged into main in this clone alone have not landed: the
  # newest of the item's commits counts only when origin/main has it. Fails
  # before the work at the exit code: today the commit is in HEAD's history
  # and on a remote branch, so it counts as pushed, and slice-9 closes,
  # exit 0.
  @ID-PRDONE-05 @slice-176
  Scenario: Under pr, work done refuses an item whose closed pull request's commits are on main here alone
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the clone fast-forwards its main to the branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" closed without merging for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 was closed without merging"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Neither a commit on main nor a pull request: nothing has landed, and the
  # fix is the push that opens the pull request. Fails before the work at
  # the exit code: today HEAD is judged and slice-9 closes, exit 0.
  @ID-PRDONE-06 @slice-176
  Scenario: Under pr, work done --json refuses an item with no commit on main and no pull request, naming itos push
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 1
    And its JSON's problem "work-done-no-pull-request"'s "fix" says "itos push"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Only a merged pull request's commits join the item's own (slice 195):
  # main's newest commit naming only a task, pulled here, is not the item's
  # without one, so the item with no pull request is refused as before. A
  # guard: it passes before the work and must after it.
  @ID-PRDONE-14 @slice-195 @guard
  Scenario: Under pr, work done --json refuses an item with no pull request though main has a commit naming only a task
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "ci: gate the fix" naming the task "T-001" and touching "widget.md"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 1
    And its JSON's problem "work-done-no-pull-request"'s "fix" says "itos push"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # The way through: the pull request merged and main pulled, the item's
  # commit is on origin/main, and its run on main is waited for while it is
  # going, as under trunk, then judges the close. Fails before the work at
  # the pr list step: today the close is made and gh is never asked.
  @ID-PRDONE-07 @slice-176
  Scenario: Under pr with ci.watch, work done closes an item whose pull request merged, waiting for main's run of its commit
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has the item "slice-9"'s commit touching "widget.md"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run finishes its job "ci" first and its job "platform" one poll later, both succeeding
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the recording gh's "pr list" call has "--state all"
    And its output says "https://ci.example/runs/1"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # Bug 146's shape: the item's branch held only a commit naming its gate
  # task, none naming the item; GitHub rebase-merged it and deleted the
  # branch, and the clone pulled main. The pull request's merge commit is
  # the item's own (slice 195), on origin/main, and main's run of it judges
  # the close. The branch is gone when gh is given the pull request, so gh
  # names no head and slice 181's trust does not come in. Fails before the
  # work at the exit code: today no commit of slice-9 is found and the
  # merged pull request is refused, work-done-pr-merged, exit 1.
  @ID-PRDONE-12 @slice-195
  Scenario: Under pr with ci.watch, work done closes an item whose merged pull request's commits name only a task, judging its merge commit
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the remote's branch "slice-9" has the commit "ci: gate the fix" naming the task "T-001" and touching "widget.md", which the clone's main lacks
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main and deletes the branch
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9", its merge commit the head of the remote's branch "main"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the recording gh's "pr list" call has "mergeCommit"
    And its output says "https://ci.example/runs/1"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # On the item's branch the close would ride the pull request and HEAD is
  # not main: refused before gh is asked, naming the switch. Fails before
  # the work at the exit code: today the branch's commit, pushed, is judged
  # and slice-9 closes on the branch, exit 0.
  @ID-PRDONE-08 @slice-176
  Scenario: Under pr, work done on the item's branch is refused, naming git switch main
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone switches to its branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "git switch main"
    And the recording gh was never called
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Slice 208 (work.feature's rerun safety): HEAD is on the item's branch,
  # which work done refuses today before any other check (@ID-PRDONE-08,
  # work-done-not-main). A build that asks the remote before judging the
  # revision gives that refusal, not work-stale. The title edit, committed
  # on the branch, moves the revision (@ID-NEXT-10). Nothing is written,
  # and gh is never called.
  @ID-PRDONE-17 @wip @slice-208
  Scenario: Under pr, a stale revision refuses work done before HEAD's branch is judged
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone switches to its branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is saved as "first"
    When itos has run "work edit slice-9 --title 'Say the next step'"
    And itos runs "work done slice-9 --json" with --expect the revision saved as "first"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-stale"'s "fix" says "itos next slice-9"
    And its output does not say "git switch main"
    And the recording gh was never called
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Without gh the pull request cannot be looked for, so nothing is closed;
  # exit 3, as itos push's pull-request-failed. Fails before the work at the
  # exit code: today HEAD is judged and slice-9 closes, exit 0.
  @ID-PRDONE-09 @slice-176
  Scenario: Under pr, with no gh on the PATH work done exits 3, naming gh pr list
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And no gh on the PATH
    And the GitHub repository in the environment is "owner/scratch"
    When itos runs "work done slice-9"
    Then itos exits with code 3
    And its output says "gh pr list failed: gh is not on the PATH"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # A gh that fails (signed out here) gives its words, under --json in the
  # problem's message. Fails before the work at the exit code: today HEAD
  # is judged and slice-9 closes, exit 0.
  @ID-PRDONE-10 @slice-176
  Scenario: Under pr, work done --json exits 3 with gh's words when gh pr list fails
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh fails every call, saying "HTTP 401: Bad credentials"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 3
    And its JSON's problem "work-done-pull-request-failed"'s "message" says "gh pr list failed: HTTP 401: Bad credentials"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # trunk is the default and stays what it was: an item with no commit of
  # its own on main is judged by HEAD, and GitHub is never asked about pull
  # requests, whatever it holds. A guard: it passes before the fix and must
  # after it.
  @ID-PRDONE-11 @slice-176 @guard
  Scenario: Without push.mode, work done calls no gh, closing an item whose branch has an open pull request as before
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the recording gh was never called
    And the registry's item "slice-9" has the status "done" and the owner "someone"
