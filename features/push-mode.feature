@phase-1
Feature: push.mode pr, an item's work on its own branch merged by rebase, claims staying on main
  Trunk-based work pushes every commit to main, so a red commit there blocks
  every agent behind it: each later push's CI inherits it (#113), and work
  done refuses an item whose newest commit is red. push.mode (new) chooses
  how itos push lands work: trunk, the default, as before; or pr, where an
  item's work goes on a short-lived branch named for the item and reaches
  main through a pull request GitHub merges by rebase once its checks pass
  (ADR-0039, q-19). Both are maintained ways of working, a repository
  choosing by its needs (the user, 2026-10-23); pr is not the end state.

  config check accepts push.mode trunk or pr and refuses any other value,
  naming the key and both modes; config get push.mode prints trunk when the
  config leaves it out; the schema lists it (config.feature). Under trunk,
  with no config, or with a config that cannot be read, itos push is what it
  was: it calls no gh and opens no pull request, and asks the remote's HEAD
  only for a branch with no upstream, which goes to the remote's default
  branch (bug-136, push.feature); an unreadable config under pr is a later
  item.

  Under pr, itos push reads the config before it fetches. The remote is the
  branch's upstream's, else origin, as before. Main is the branch the
  remote's HEAD names, asked of the remote itself (git ls-remote --symref
  <remote> HEAD), since a clone's refs/remotes/<remote>/HEAD is missing
  after git remote add. An ls-remote that fails exits by the kind git's
  message gives it, as the fetch does: 75 for a remote it cannot reach, 3
  for one that is no repository (outcome fetch-failed). A remote that
  answers without a HEAD naming a branch it has is refused before anything
  is fetched, exit 3 (outcome no-default-branch). The GitHub repository is
  the watch's (ci.range.github.repository_env's variable, else the remote's
  URL), and every gh call names it with --repo, so gh never picks another
  remote, a fork's upstream say. With none, a push from an item's branch
  that has work for a pull request is refused once rebased, before
  anything is pushed, exit 3 (outcome no-github-repository); one that
  sends only registry commits to main calls no gh and needs none.

  On main, the commits the push adds that are the registry's (a take, a
  close, a queue move, an idea: slice 56's rule, widened by slice-177 to the
  ledger edits made beside the registry, push-mode-ledger.feature) go
  straight to main as under trunk, so two agents never take one item. A push that adds any
  other commit is refused once rebased onto main, exit 1, nothing pushed
  (outcome work-on-main), saying to carry the work to its item's branch with
  git switch -c <item>. --ref naming main is judged the same way, against
  the remote's main, so the refusal has no way round; --ref naming any other
  branch, and --tag, push what they name as under trunk.

  On any other branch, the branch's name is the item: it must be the id of
  an item of the work registry, else the push is refused before anything is
  fetched, exit 1 (outcome not-item-branch), saying to rename the branch with
  git branch -m <item>. The branch goes to the remote's branch of its own
  name, whatever its upstream says (a branch made from origin/main tracks
  main). It is rebased onto the remote's item branch when there is one, else
  onto main, so a first push starts up to date with main and a later one
  never needs force. Of the commits it adds, those directly above main's head
  that are the registry's (a take made on the branch; slice-177) go to main
  and the rest to the item branch, in one git push --atomic: both land or neither,
  the pre-push hook judging both. A branch whose commits are all such goes
  to main alone: the item's branch is not pushed, no pull request is opened
  and no run waited for, as for registry commits pushed from main.

  Then, unless the branch adds nothing to main, its pull request. gh pr list
  --repo <repository> --head <item> --state open --json number,url finds the
  open one, [] when there is none; with none, gh pr create --repo
  <repository> --base <main> --head <item> opens it (its title and its
  description are push-mode-title.feature's, slice 184, which replaced
  --fill), printing its address. Then gh pr merge <item> --repo <repository> --auto --rebase, never
  squash (each commit is a changelog entry with its footers, and GitHub's
  squash appends a Co-authored-by paragraph that pushes them out of the
  trailer block); and, when main has a commit the pushed head lacks, gh pr
  update-branch <item> --repo <repository> --rebase, so GitHub rebases the
  branch rather than itos forcing it. These run with nothing to push too, so
  itos push again finishes a push whose pull request was not opened. The
  push prints the pull request's address.

  With no gh on the PATH, or a gh call that fails (signed out, a refusal),
  the branch is pushed already: exit 3 (outcome pull-request-failed), gh's
  words and the gh command that failed named, and itos push again to finish
  it.

  With ci.watch's provider, the push then waits for the CI run of the pull
  request's head, as trunk waits for main's: CI runs on pull_request, and
  the watch finds a run by its head commit whatever its event. It prints
  each job's result and the run's address and exits with the run's result,
  0 or 1, the commits and the pull request standing either way; --no-wait
  returns at once. The head is the one it pushed, unless it asked GitHub to
  rebase the pull request: then it is the head GitHub makes (slice 175,
  below). Main's run after the merge is a later item.

  After gh pr update-branch (slice 175) the head pushed is no longer the
  pull request's, and the one GitHub makes is not known when gh answers:
  GitHub rebases afterwards, and until it has, gh pr view can still answer
  the head pushed or, before GitHub has seen the push, the previous push's
  head, whose run may be green for commits CI never judged. So the push asks
  gh pr view <item> --repo <repository> --json headRefOid at once, and again
  each ci.watch.interval, and takes the first head that has main's head, as
  the push fetched it, as an ancestor. The polling runs inside the wait
  (internal/slices/push), after the config is loaded, the Watcher built
  and the "Pushed ..." line printed, so a Watcher that cannot be built
  fails as it does today, before any gh pr view. Neither stale answer has it:
  update-branch is asked for only when the head pushed lacks main's head,
  and the head pushed holds the previous one. A head the clone lacks is
  first fetched quietly from the remote's item branch, as the watch fetches
  a head it lacks (bug 41); one still missing counts as no answer. Whatever
  update-branch printed, "already up-to-date" included, this rule judges: a
  head that already has main's head is taken at the first answer. Then it
  says "GitHub rebased the pull request onto <remote>/<main>: watching the
  run of <sha>, its new head." (sha the watch's seven characters), where the
  watch's progress goes (stdout, stderr under --json, nowhere under -q), and
  watches that head's run as any pushed head's, under its own
  ci.watch.timeout from then on; --json's ci and run are that run's, beside
  pull_request with updated true.

  No such head within ci.watch.timeout of the update-branch call (GitHub
  never rebasing) exits 75, outcome pushed, as a run that never appears
  does: stderr says "itos: GitHub has not rebased the pull request onto
  <remote>/<main> after <n> seconds (ci.watch.timeout), so no CI run was
  watched; its checks are at <address>.", and --json has ci timeout and no
  run. A gh pr view that fails is any gh call that fails: exit 3, outcome
  pull-request-failed, naming gh pr view. With --no-wait, or with no ci.watch
  provider, there is no gh pr view; a push with nothing to push waits for no
  run, as before. Slice 174's line "No CI run is waited for: the pull
  request's checks at <address> are GitHub's to judge." goes: a push under
  no ci.watch says nothing of runs, as a trunk push does not. The scenarios
  ID-PUSHMODE-23 to ID-PUSHMODE-27 replace ID-PUSHMODE-21 and
  ID-PUSHMODE-22, whose promises (exit 0, no run asked, the line, --json
  with neither ci nor run) the change breaks: it deletes both and names
  them in Changes:.

  itos push --onto-main (slice-193) brings an item's branch up to main
  itself: the one sanctioned rewrite of an item's own branch. GitHub cannot
  rebase a pull request whose pushed commits conflict with main's (bug-151,
  PR #201: two specs appending to one feature file's end), and gh pr
  update-branch, which brings one main left behind up to date (bug-148, PR
  #207, whose implementer could not run red without main's steps), is
  refused to agents as destructive; both needed the person's force-push.
  Under push.mode pr, on an item's branch, it fetches main and the item's
  branch as a push does. Then it takes the remote's item head first,
  rebasing onto it, only when that head has a commit main lacks of which
  HEAD's commits main lacks hold no counterpart, by the counterpart rule of
  the pre-push hook (pre-push.feature: the same author, author date and
  message, not a patch-id, which a resolved conflict changes); so a commit
  someone pushed there is kept, and a branch already rebased onto main, as
  one the person finished after a stopped --onto-main is, is never replayed
  onto the remote's old head. Then it rebases the commits main lacks onto
  the remote's main, unless HEAD has main's head already, and pushes the
  item's branch with git push --force-with-lease=refs/heads/<item>:<the
  item's head it fetched> when that head is no ancestor of HEAD, as a push
  does when it is, the registry's commits directly above main going to
  main in the same atomic push as ever, the pre-push hook judging both
  (pre-push.feature accepts a rewrite of a branch that is not main and
  drops no commit, slice-193). An item's branch the remote moved after the
  fetch fails the lease: the remote is left as it was, exit 1, outcome
  push-failed, saying to run itos push --onto-main again. A rebase that
  stops is left in progress for the person as itos push leaves one,
  nothing pushed, exit 1, outcome rebase-stopped, its advice naming itos
  push --onto-main again rather than itos push, which would rebase the
  rebased branch onto the remote's old head. A branch the remote does not
  have yet is a first push, rebased onto main as ever. After the push all is a push's:
  the pull request found or opened, auto-merge, no update-branch since the
  head pushed has main's, the wait for its run, and --no-wait and --ready
  with it as without.

  Outside push.mode pr (trunk, no config, a config that cannot be read)
  --onto-main is refused once the config is read, before anything is
  fetched or pushed, exit 1, outcome onto-main-without-pr; on main, once
  main is known, exit 1, outcome onto-main-on-main. Exit 1, as --ready's
  refusals are (push-mode-draft.feature): the arguments are well formed,
  and only the repository makes them pointless. With --tag or --ref it is a
  usage error, exit 2: "push --onto-main rebases an item's branch onto
  main; it takes no --tag or --ref". No other force path opens: --force,
  -f, --force-with-lease and a + refspec stay usage errors beside it, the
  lease being itos's own, on the head it fetched.

  Without push.pr.draft the wait is a report, not a gate: auto-merge is
  turned on before it, so what stops a red merge is the repository's required
  checks, which itos init does not set yet (a later item). With push.pr.draft
  (push-mode-draft.feature, slice 179) itos push opens the pull request as a
  draft and turns on no auto-merge, and itos push --ready makes the wait a
  gate: auto-merge comes after the checks and the reviews, never before.

  --json adds "pull_request": {"url","number","created","updated"} after a
  push from an item's branch, created saying whether it opened the pull
  request, updated whether it asked GitHub to rebase it (slice 179 adds
  draft and auto_merge), beside ci and run when it waited; outcome gains work-on-main, not-item-branch,
  no-default-branch, no-github-repository and pull-request-failed, and
  slice-193 adds onto-main-without-pr and onto-main-on-main.

  Not in this slice: work done under pr, which closes an item only once its
  pull request has merged (rebase-merge keeps each commit's footers, so a
  main holding the merged commits is how it finds them; slice-176,
  work-done-pr.feature); itos init offering the repository's settings; main's run after the merge; a
  pull request main leaves behind after the last push, or GitHub cannot
  rebase, unless someone runs itos push --onto-main (slice-193); a registry commit made on the branch above the work, which rides
  the pull request.

  The recording gh stands in for GitHub: first on the PATH, with
  GITHUB_REPOSITORY owner/scratch, it records each call, answers pr list
  with the pull requests of the --head branch that its --state names (open,
  gh's default, merged, closed or all) as gh does ([] for none; slice-176),
  and answers pr create with the address the scenario gives, holding that
  pull request open from then on. pr view answers {"headRefOid": <sha>},
  the remote's branch's head; pr update-branch rebases the remote's branch
  onto the remote's main, as GitHub does, its commits gaining new SHAs,
  unless a step says it leaves the branch as it is (GitHub never
  rebasing). Another step makes pr view answer the pull request's previous
  head, the remote's branch's head when the step gave the recording gh the
  pull request, for its first n calls after pr update-branch: GitHub not
  having seen the push yet. The remote is a bare repository whose HEAD
  names main.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # The slice's point: the work goes to the branch named for its item, never
  # to main, and GitHub is asked to open its pull request and merge it by
  # rebase once its checks pass. Fails before the build at the first gh
  # line: today the branch is pushed and gh never called.
  @ID-PUSHMODE-01 @slice-174
  Scenario: Under pr, a push from the item's branch pushes that branch, opens its pull request and turns on auto-merge by rebase
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the remote's branch does not have "feat: add the widget"
    And the recording gh's "pr list" call has "--head slice-9"
    And the recording gh's "pr list" call has "--state open"
    And the recording gh's "pr create" call has "--base main"
    And the recording gh's "pr create" call has "--head slice-9"
    And the recording gh's "pr merge" call has "slice-9"
    And the recording gh's "pr merge" call has "--auto"
    And the recording gh's "pr merge" call has "--rebase"
    And every call of the recording gh has "--repo owner/scratch"
    And its output says "https://github.example/o/r/pull/1"

  # The branch was made from a main the remote has moved past: its first
  # push rebases it onto the remote's main, so the pull request starts up to
  # date and a later push stays a fast-forward. Fails before the build at the
  # last step: today a new branch is pushed as it is, with no rebase.
  @ID-PUSHMODE-02 @slice-174
  Scenario: Under pr, the first push of an item's branch rebases it onto the remote's main
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "chore: tidy the docs" then "feat: add the widget", with no merge commit

  # Claims stay on main (ADR-0039): a take committed on the item's branch,
  # directly above main, goes to main in the same atomic push as the work
  # goes to the branch, so the lock is main's whichever branch took it.
  # Fails before the build at the first remote step: today both commits go
  # to the new branch and main has neither.
  @ID-PUSHMODE-03 @slice-174
  Scenario: Under pr, a take on the item's branch goes to main and the work to the branch
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the remote's branch does not have "feat: add the widget"
    And the remote has the branch "slice-9" with "feat: add the widget"

  # A later push of a branch whose pull request is open adds its commits to
  # that pull request and opens no other. Fails before the build at the
  # address: today the commits are pushed and no pull request is named.
  @ID-PUSHMODE-04 @slice-174
  Scenario: Under pr, a push to an item's branch whose pull request is open names it and opens no other
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "slice-9" with "test: cover the widget"
    And its output says "https://github.example/o/r/pull/7"
    And the recording gh was never called as "pr create"

  # Auto-merge never updates a branch main left behind, and the ruleset
  # requires it up to date, so the pull request would wait forever. itos
  # asks GitHub to rebase it (q-19) instead of rebasing locally, which would
  # need a force: the push itself rebases onto the remote's item branch and
  # is a fast-forward, exit 0. Fails before the build at the gh line.
  @ID-PUSHMODE-05 @slice-174
  Scenario: Under pr, a push to a pull request main has moved past asks GitHub to rebase it, never forcing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "slice-9" with "test: cover the widget"
    And the recording gh's "pr update-branch" call has "slice-9"
    And the recording gh's "pr update-branch" call has "--rebase"
    And every call of the recording gh has "--repo owner/scratch"

  # The slice's point (bug-151's PR #201, bug-148's PR #207): itos itself
  # brings an item's branch main has moved past up to main, with a lease on
  # the head it fetched, and the real pre-push hook, installed here, accepts
  # the rewrite since it drops no commit. Main's "chore: tidy the docs" has no
  # Task footer, which the commit rules want of a chore: the hook judges an
  # accepted rewrite as it judges a new branch, by the commits on no
  # remote-tracking branch (ID-PREPUSH-07), so main's commits, on the remote
  # already, are not the pusher's, and judging them would refuse this push.
  # The head pushed has main's, so GitHub is asked for no rebase. Fails
  # before the work at the exit code: today --onto-main is a usage error,
  # exit 2.
  @ID-PUSHMODE-28 @slice-193 @wip
  Scenario: Under pr, push --onto-main rebases an item's branch main has moved past onto main and pushes it, the pre-push hook accepting the rewrite
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And the pre-push hook is installed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --onto-main"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "chore: tidy the docs" then "feat: add the widget", with no merge commit
    And its output says "https://github.example/o/r/pull/7"
    And the recording gh was never called as "pr update-branch"

  # A commit pushed to the item's branch since the clone last had it (the
  # coordinator's fix to the spec, say) is the item's work too: it is a
  # commit main lacks with no counterpart among HEAD's, so --onto-main takes
  # the remote's item head first, then rebases the whole onto main, and the
  # rewrite drops nothing. The pre-push
  # hook is the scenarios' stand-in, which passes anything, so the remote
  # shows what itos itself pushed. Fails before the work at the exit code:
  # today --onto-main is a usage error, exit 2.
  @ID-PUSHMODE-29 @slice-193 @wip
  Scenario: Under pr, push --onto-main keeps a commit the remote's item branch has that the clone lacks
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote's branch "slice-9" has gained the commit "fix: mend the widget" touching "mend.md"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --onto-main"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "feat: add the widget" then "fix: mend the widget", with no merge commit
    And the remote has the branch "slice-9" with "chore: tidy the docs"

  # bug-151's case (PR #201): the item's pushed spec and a spec merged to
  # main first both write one feature file, so the rebase onto main stops.
  # It is left in progress for the person, nothing pushed and no gh called,
  # and the advice names itos push --onto-main again: a plain itos push
  # would rebase the rebased branch onto the remote's old head. Fails before
  # the work at the exit code: today --onto-main is a usage error, exit 2.
  @ID-PUSHMODE-30 @slice-193 @wip
  Scenario: Under pr, push --onto-main whose rebase stops on a conflict with main leaves the rebase in progress and pushes nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: specify slice-8" touching "features/widget.feature"
    When itos runs "push --onto-main"
    Then itos exits with code 1
    And its output says "git rebase --continue"
    And its output says "itos push --onto-main again"
    And the clone has a rebase in progress
    And the remote's branch "slice-9" does not have "docs: specify slice-8"
    And the recording gh was never called

  # bug-151's case to its end: the person resolves the conflict
  # @ID-PUSHMODE-30 stops on and continues the rebase, and --onto-main again
  # pushes. The remote's item head, the old spec, has its counterpart in
  # HEAD's resolved spec (same author, author date and message), so it is
  # not taken first: replaying the rebased branch onto it would conflict
  # again. The real pre-push hook accepts the rewrite by the same rule,
  # though the resolved commit's patch-id is not the old one's (its diff now
  # adds a line after main's); a patch-id rule would refuse it as dropping
  # the spec. Fails before the work at the Given that runs the first
  # --onto-main: today it is a usage error, exit 2, where the step wants 1.
  @ID-PUSHMODE-38 @slice-193 @wip
  Scenario: Under pr, push --onto-main again after the person resolves its stopped rebase pushes main's commit then the item's resolved one
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And the pre-push hook is installed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "docs: specify slice-8" touching "features/widget.feature"
    And itos has run "push --onto-main", exiting with code 1
    And the clone resolves the conflict in "features/widget.feature", keeping both sides, and continues the rebase
    When itos runs "push --onto-main"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "docs: specify slice-8" then "docs: specify slice-9", with no merge commit
    And its output says "https://github.example/o/r/pull/7"

  # The lease: another clone pushes the item's branch after itos fetched it
  # (while itos rebases, between its fetch and its push), so the head itos
  # would replace is not the one it rebased from. git refuses the push and
  # the remote keeps the other clone's commit, which a plain force would
  # have dropped; nothing else changes. Fails before the work at the exit
  # code: today --onto-main is a usage error, exit 2.
  @ID-PUSHMODE-31 @slice-193 @wip
  Scenario: Under pr, push --onto-main whose lease no longer holds is refused, the remote's item branch left as it was
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And another clone pushes the commit "fix: mend the widget" touching "mend.md" to the remote's branch "slice-9" while itos rebases
    When itos runs "push --onto-main"
    Then itos exits with code 1
    And its output says "itos push --onto-main again"
    And the remote has the branch "slice-9" with "fix: mend the widget"
    And the remote's branch "slice-9" does not have "chore: tidy the docs"
    And the recording gh was never called

  # After the push all is a push's: with ci.watch it waits for the run of
  # the head it pushed, which has main's head, so GitHub is asked for no
  # rebase and gh pr view for no other head. Fails before the work at the
  # exit code: today --onto-main is a usage error, exit 2.
  @ID-PUSHMODE-32 @slice-193 @wip
  Scenario: Under pr with ci.watch, push --onto-main waits for the run of the head it pushed and exits 1 when it fails
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --onto-main"
    Then itos exits with code 1
    And its output says "ci: failure"
    And the fake GitHub was asked for the run of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr update-branch"
    And the recording gh was never called as "pr view"

  # --onto-main combines with --no-wait, which returns once the branch is
  # pushed and its pull request asked for. Fails before the work at the
  # exit code: today --onto-main is a usage error, exit 2.
  @ID-PUSHMODE-33 @slice-193 @wip
  Scenario: Under pr with ci.watch, push --onto-main --no-wait pushes the rebased branch and waits for no run
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --onto-main --no-wait"
    Then itos exits with code 0
    And the remote's branch "slice-9" ends with "chore: tidy the docs" then "feat: add the widget", with no merge commit
    And the fake GitHub was never asked about a run

  # Only pr has items' branches to rewrite: elsewhere --onto-main is
  # refused before anything is fetched or pushed, exit 1 as --ready's
  # refusals are, since the arguments are well formed. No push.mode here, so
  # trunk. Fails before the work at the exit code: today exit 2.
  @ID-PUSHMODE-34 @slice-193 @wip
  Scenario: Outside push.mode pr, push --onto-main is refused, exit 1, nothing pushed
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --onto-main --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "onto-main-without-pr"
    And the remote has no branch "slice-9"
    And the remote's branch does not have "feat: add the widget"
    And the recording gh was never called

  # Main is no item's branch and is never rewritten: --onto-main from main
  # is refused once main is known, even for a take that a plain push sends.
  # Fails before the work at the exit code: today exit 2.
  @ID-PUSHMODE-35 @slice-193 @wip
  Scenario: Under pr, push --onto-main from main is refused, exit 1, nothing pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push --onto-main --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "onto-main-on-main"
    And the remote's branch does not have "docs: take slice-9"
    And the recording gh was never called

  # --onto-main pushes an item's branch, so --tag and --ref, which push what
  # they name, have no place beside it. Fails before the work at the
  # output: today it is a usage error too, but one that names no --tag or
  # --ref.
  @ID-PUSHMODE-36 @slice-193 @wip
  Scenario Outline: push --onto-main with <named> is a usage error
    Given the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --onto-main <named>"
    Then itos exits with code 2
    And its output says "push --onto-main rebases an item's branch onto main; it takes no --tag or --ref"
    And the remote has no branch "slice-9"

    Examples:
      | named         |
      | --tag v1.0.0  |
      | --ref slice-9 |

  # No other force path opens beside --onto-main: its lease is itos's own,
  # on the head it fetched, never one passed in. A guard: each is refused
  # today, --onto-main with it, and must stay refused.
  @ID-PUSHMODE-37 @slice-193 @guard @wip
  Scenario Outline: push --onto-main with <force> is a usage error, never forcing
    Given the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --onto-main <force>"
    Then itos exits with code 2
    And its output says "push never forces"
    And the remote has no branch "slice-9"

    Examples:
      | force              |
      | --force            |
      | -f                 |
      | --force-with-lease |
      | +slice-9           |

  # itos push again is how a push whose pull request was not opened (gh
  # missing or failing, as in @ID-PUSHMODE-11) is finished: with nothing to
  # push it still opens the pull request. Fails before the build at the gh
  # line: today nothing to push returns at once.
  @ID-PUSHMODE-06 @slice-174
  Scenario: Under pr, an item's branch the remote has with no pull request gets one, though there is nothing to push
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--head slice-9"
    And its output says "https://github.example/o/r/pull/1"

  # git switch -c slice-9 origin/main makes main the branch's upstream, and
  # today's itos push pushes to the upstream's branch: the work would land
  # on main, which pr mode exists to keep it off. An item's branch goes to
  # its own name whatever its upstream says. Fails before the build at the
  # main step: today the commit is pushed to main.
  @ID-PUSHMODE-07 @slice-174
  Scenario: Under pr, an item's branch tracking main is pushed to its own branch, not to main
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9", tracking the remote's main
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch does not have "feat: add the widget"
    And the remote has the branch "slice-9" with "feat: add the widget"

  # Under pr only the registry's commits go to main from a clone. Work
  # committed on main is refused with the way to its item's branch, before
  # the push, so main has none of it. Fails before the build at the exit
  # code: today it is pushed to main, exit 0.
  @ID-PUSHMODE-08 @slice-174
  Scenario: Under pr, a push from main of a commit touching more than the registry is refused, naming git switch -c
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git switch -c"
    And the remote's branch does not have "feat: add the widget"
    And the recording gh was never called

  # Claims, queue moves and the coordinator's registry commits stay direct
  # to main (ADR-0039), so two agents never take one item. Passes today,
  # where no mode is read, and must keep passing once pr mode refuses work
  # on main.
  @ID-PUSHMODE-09 @slice-174 @guard
  Scenario: Under pr, a push from main of commits touching only the registry goes straight to main, opening no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the recording gh was never called

  # The branch's name is how itos push knows the item: a branch naming no
  # item of the registry is refused before anything is fetched or pushed,
  # with the rename that fixes it. Fails before the build at the exit code:
  # today the branch is pushed, exit 0.
  @ID-PUSHMODE-10 @slice-174
  Scenario: Under pr, a push from a branch that names no item is refused, naming git branch -m
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "tidy"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git branch -m"
    And the remote has no branch "tidy"
    And the recording gh was never called

  # Without gh the commits are pushed all the same, so nothing is lost; the
  # exit says the pull request is not opened, and itos push again opens it
  # (@ID-PUSHMODE-06). Fails before the build at the exit code: today the
  # branch is pushed, exit 0.
  @ID-PUSHMODE-11 @slice-174
  Scenario: Under pr, with no gh on the PATH the item's branch is pushed and the push exits 3, saying how to finish
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And no gh on the PATH
    And the GitHub repository in the environment is "owner/scratch"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 3
    And the remote has the branch "slice-9" with "feat: add the widget"
    And its output says "gh pr create"
    And its output says "itos push again"

  # CI runs on pull_request (ci.yml), and the watch finds a run by its head
  # commit whatever its event, so under ci.watch the push waits for the
  # pushed head's run as trunk waits for main's, and a red run exits 1,
  # naming the failed job, with the branch and its pull request standing. A
  # report, not a gate: auto-merge is on already, and only the repository's
  # required checks (init's settings, a later item) stop a red merge. Fails before the build at the exit code: today the
  # unknown key push leaves the config unread after the push, so nothing is
  # watched and it exits 0; once push.mode is a known key, at the address of
  # the pull request.
  @ID-PUSHMODE-12 @slice-174
  Scenario: Under pr with ci.watch, a push from an item's branch waits for the pushed head's run and exits 1 when it fails
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "ci: failure"
    And the fake GitHub was asked for the run of the clone's HEAD
    And the remote has the branch "slice-9" with "feat: add the widget"
    And its output says "https://github.example/o/r/pull/1"

  # --json is the contract (ADR-0035): an agent reads the pull request's
  # address from it. Fails before the build at the pull_request line: today
  # the JSON has no such key.
  @ID-PUSHMODE-13 @slice-174
  Scenario: Under pr, push --json names the pull request it opened
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "pushed"
    And its JSON's "pull_request.url" is "https://github.example/o/r/pull/1"
    And its JSON's "pull_request.created" is true

  # --ref pushes a branch for another platform's CI (slice 131), which is no
  # item's work: it opens no pull request under pr either. Passes today and
  # must keep passing.
  @ID-PUSHMODE-14 @slice-174 @guard
  Scenario: Under pr, push --ref pushes the branch it names and opens no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the branch "ci/windows" with the commit "docs: try windows" touching "README.md"
    When itos runs "push --ref ci/windows"
    Then itos exits with code 0
    And the remote has the branch "ci/windows" with "docs: try windows"
    And the recording gh was never called

  # trunk is the default: with no push.mode, itos push is what it was and
  # GitHub is never asked for a pull request, whatever the branch is called.
  # A branch with no upstream goes to the remote's default branch under trunk
  # (bug-136); this scenario's check is that gh is never called. Passes today
  # and must keep passing.
  @ID-PUSHMODE-15 @slice-174 @guard
  Scenario: Without push.mode, a push from a branch named for an item calls no gh and opens no pull request
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh was never called

  # trunk, written out, is the counterpart of @ID-PUSHMODE-08: work pushed
  # from main lands on main. Passes today, where no mode is read, and must
  # keep passing.
  @ID-PUSHMODE-16 @slice-174 @guard
  Scenario: Under trunk, a push from main of any commit goes to main, opening no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: trunk
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "feat: add the widget"
    And the recording gh was never called

  # --ref main would push work to main past @ID-PUSHMODE-08's refusal, so
  # under pr it is judged as a push from main against the remote's main.
  # Fails before the build at the exit code: today --ref main pushes, exit 0.
  @ID-PUSHMODE-17 @slice-174
  Scenario: Under pr, push --ref naming main is refused like work pushed from main
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --ref main"
    Then itos exits with code 1
    And its output says "git switch -c"
    And the remote's branch does not have "feat: add the widget"

  # A branch holding only a take has no work for a pull request: the take
  # goes to main, where claims live, and the item's branch is not pushed. It
  # calls no gh, so it needs no GitHub repository (@ID-PUSHMODE-20 is the
  # refusal for a branch with work). Fails before the build at the main
  # step: today the take goes to the new branch.
  @ID-PUSHMODE-18 @slice-174
  Scenario: Under pr, an item's branch holding only registry commits sends them to main and opens no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And no GitHub repository in the environment
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: take slice-9"
    And the remote has no branch "slice-9"
    And the recording gh was never called

  # Main is asked of the remote, never guessed: a remote whose HEAD names a
  # branch it does not have gives no main, and the push stops before
  # anything is fetched or pushed. Fails before the build at the exit code:
  # today the branch is pushed, exit 0.
  @ID-PUSHMODE-19 @slice-174
  Scenario: Under pr, a remote whose HEAD names no branch it has is refused, exit 3
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's HEAD names the branch "trunk", which it does not have
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 3
    And its JSON's "outcome" is "no-default-branch"
    And the remote has no branch "slice-9"
    And the recording gh was never called

  # Every gh call names the repository itos pushes to; a remote that names
  # no GitHub repository (here a folder), with no repository in the
  # environment, has none to name, and the push stops once rebased, before
  # anything is pushed. Fails before the build at the exit code: today the
  # branch is pushed, exit 0.
  @ID-PUSHMODE-20 @slice-174
  Scenario: Under pr, a push with no GitHub repository to name is refused, exit 3
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And no GitHub repository in the environment
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 3
    And its JSON's "outcome" is "no-github-repository"
    And the remote has no branch "slice-9"
    And the recording gh was never called

  # The slice's point: in a repository with several agents main has usually
  # moved, so most pushes ask GitHub to rebase, and the run that judges the
  # pull request is the rebased head's. The push waits for it and exits with
  # its result, red here. Fails before the work at the exit code: today the
  # push waits for no run and exits 0.
  @ID-PUSHMODE-23 @slice-175
  Scenario: Under pr with ci.watch, a push that asks GitHub to rebase its pull request waits for the run of the head GitHub made
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 1
    And the fake GitHub was asked for the run of the head of the remote's branch "slice-9"
    And its output says "ci: failure"
    And its output says "GitHub rebased the pull request onto origin/main"
    And the recording gh's "pr view" call has "slice-9"
    And the recording gh's "pr view" call has "--json headRefOid"
    And every call of the recording gh has "--repo owner/scratch"

  # The race slice 174 stopped at: until GitHub has seen the push, gh pr view
  # answers the previous push's head, which differs from the head pushed and
  # may have a green run, for commits CI never judged. It lacks main's head,
  # so the push passes it over and watches the head GitHub made alone; --json
  # carries that run as ci and run. Fails before the work at the fake GitHub
  # step: today no run is asked about.
  @ID-PUSHMODE-24 @slice-175
  Scenario: Under pr with ci.watch, a push never takes the previous push's head that gh pr view still answers for the rebased one
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr view answers the pull request's previous head for its first 2 calls after pr update-branch
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --json"
    Then itos exits with code 0
    And the fake GitHub was asked for the run of the head of the remote's branch "slice-9"
    And its JSON's "outcome" is "pushed"
    And its JSON's "pull_request.updated" is true
    And its JSON's "ci" is "success"
    And its JSON's "run.url" is "https://ci.example/runs/1"

  # GitHub may never rebase (a conflict it cannot resolve, an outage): the
  # push looks for the new head no longer than ci.watch.timeout, then exits
  # 75 as a run that never appears does, naming where the checks are, the
  # branch and its pull request standing. Fails before the work at the exit
  # code: today it exits 0.
  @ID-PUSHMODE-25 @slice-175
  Scenario: Under pr with ci.watch, a pull request GitHub never rebases exits 75 at ci.watch.timeout, watching no run
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh's pr update-branch leaves the branch as it is
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --json"
    Then itos exits with code 75
    And its output says "GitHub has not rebased the pull request onto origin/main"
    And its output says "its checks are at https://github.example/o/r/pull/7"
    And the fake GitHub was never asked about a run
    And its JSON's "outcome" is "pushed"
    And its JSON's "ci" is "timeout"
    And its JSON has no "run"
    And the remote has the branch "slice-9" with "test: cover the widget"

  # --no-wait returns once the pull request is asked for, so it never looks
  # for the head GitHub makes. A guard: it passes before the fix and must
  # after it.
  @ID-PUSHMODE-26 @slice-175 @guard
  Scenario: Under pr with ci.watch, push --no-wait after asking GitHub to rebase never calls gh pr view
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the recording gh's "pr update-branch" call has "slice-9"
    And the recording gh was never called as "pr view"
    And the fake GitHub was never asked about a run

  # The head is looked for only to watch its run: with no ci.watch provider
  # (none, the default) there is none to watch, so no gh pr view. A guard: it
  # passes before the fix and must after it.
  @ID-PUSHMODE-27 @slice-175 @guard
  Scenario: Under pr without ci.watch, a push that asks GitHub to rebase never calls gh pr view
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr update-branch" call has "slice-9"
    And the recording gh was never called as "pr view"
