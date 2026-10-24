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
  was, whatever the branch (an unreadable config under pr is a later item).

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

  On main, the commits the push adds that touch only the work registry (a
  take, a close, a queue move, an idea: slice 56's rule) go straight to main
  as under trunk, so two agents never take one item. A push that adds any
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
  that touch only the registry (a take made on the branch) go to main and the
  rest to the item branch, in one git push --atomic: both land or neither,
  the pre-push hook judging both. A branch whose commits are all such goes
  to main alone: the item's branch is not pushed, no pull request is opened
  and no run waited for, as for registry commits pushed from main.

  Then, unless the branch adds nothing to main, its pull request. gh pr list
  --repo <repository> --head <item> --state open --json number,url finds the
  open one, [] when there is none; with none, gh pr create --repo
  <repository> --base <main> --head <item> --fill opens it, printing its
  address. Then gh pr merge <item> --repo <repository> --auto --rebase, never
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

  With ci.watch's provider, the push then waits for the CI run of the head
  it pushed, as trunk waits for main's: CI runs on pull_request, and the
  watch finds a run by its head commit whatever its event. It prints each
  job's result and the run's address and exits with the run's result, 0 or
  1, the commits and the pull request standing either way; --no-wait
  returns at once. When it asked GitHub to rebase the pull request, the head
  it pushed is no longer the pull request's: it reads the pull request's
  head with gh pr view <item> --repo <repository> --json headRefOid, again
  each ci.watch.interval until it is no longer the head it pushed (GitHub
  rebases after it answers), and waits for that head's run instead, so
  every push is watched; a head still unchanged at ci.watch.timeout exits
  75, as a run not finished does. Main's run after the merge is a later
  item.

  The wait is a report, not a gate: auto-merge is turned on before it, so
  what stops a red merge is the repository's required checks, which itos
  init does not set yet (a later item).

  --json adds "pull_request": {"url","number","created","updated"} after a
  push from an item's branch, created saying whether it opened the pull
  request, updated whether it asked GitHub to rebase it, beside ci and run
  when it waited; outcome gains work-on-main, not-item-branch,
  no-default-branch, no-github-repository and pull-request-failed.

  Not in this slice: work done is unchanged (rebase-merge keeps each commit's
  footers, so a main holding the merged commits closes the item as before;
  refusing to close while the pull request is open is p1-pr-items); itos
  init offering the repository's settings; main's run after the merge; a
  pull request main leaves behind after the last push, or GitHub cannot
  rebase; a registry commit made on the branch above the work, which rides
  the pull request.

  The recording gh stands in for GitHub: first on the PATH, with
  GITHUB_REPOSITORY owner/scratch, it records each call, answers pr list
  with the open pull requests of the --head branch as gh does ([] for none),
  and answers pr create with the address the scenario gives, holding that
  pull request open from then on. pr view's headRefOid is the remote's
  branch head, until pr update-branch moves it to the commit the scenario
  names. The remote is a bare repository whose HEAD
  names main.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # The slice's point: the work goes to the branch named for its item, never
  # to main, and GitHub is asked to open its pull request and merge it by
  # rebase once its checks pass. Fails before the build at the first gh
  # line: today the branch is pushed and gh never called.
  @ID-PUSHMODE-01 @slice-174 @wip
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
  @ID-PUSHMODE-02 @slice-174 @wip
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
  @ID-PUSHMODE-03 @slice-174 @wip
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
  @ID-PUSHMODE-04 @slice-174 @wip
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
  @ID-PUSHMODE-05 @slice-174 @wip
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

  # itos push again is how a push whose pull request was not opened (gh
  # missing or failing, as in @ID-PUSHMODE-11) is finished: with nothing to
  # push it still opens the pull request. Fails before the build at the gh
  # line: today nothing to push returns at once.
  @ID-PUSHMODE-06 @slice-174 @wip
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
  @ID-PUSHMODE-07 @slice-174 @wip
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
  @ID-PUSHMODE-08 @slice-174 @wip
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
  @ID-PUSHMODE-09 @slice-174 @guard @wip
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
  @ID-PUSHMODE-10 @slice-174 @wip
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
  @ID-PUSHMODE-11 @slice-174 @wip
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
  @ID-PUSHMODE-12 @slice-174 @wip
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
  @ID-PUSHMODE-13 @slice-174 @wip
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
  @ID-PUSHMODE-14 @slice-174 @guard @wip
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

  # trunk is the default: with no push.mode, a branch named for an item is
  # pushed to its own name as before, and GitHub is never asked for a pull
  # request. Passes today and must keep passing.
  @ID-PUSHMODE-15 @slice-174 @guard @wip
  Scenario: Without push.mode, a push from a branch named for an item pushes it as before, opening no pull request
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the recording gh was never called

  # trunk, written out, is the counterpart of @ID-PUSHMODE-08: work pushed
  # from main lands on main. Passes today, where no mode is read, and must
  # keep passing.
  @ID-PUSHMODE-16 @slice-174 @guard @wip
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
  @ID-PUSHMODE-17 @slice-174 @wip
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
  @ID-PUSHMODE-18 @slice-174 @wip
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
  @ID-PUSHMODE-19 @slice-174 @wip
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
  # environment, has none to name, and the push stops before anything is
  # fetched or pushed. Fails before the build at the exit code: today the
  # branch is pushed, exit 0.
  @ID-PUSHMODE-20 @slice-174 @wip
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

  # In a repository with several agents main has usually moved, so most
  # pushes ask GitHub to rebase: watching the head pushed would watch a run
  # that is cancelled or never counts (its SHA is not the pull request's).
  # The push reads the rebased head and waits for its run, so every push is
  # watched. Fails before the build at the fake GitHub step: today the
  # unknown key push leaves the config unread, so nothing is watched.
  @ID-PUSHMODE-21 @slice-174 @wip
  Scenario: Under pr with ci.watch, a push that asks GitHub to rebase its pull request waits for the rebased head's run
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
    And the recording gh's pr update-branch moves the pull request's head to "1111111111111111111111111111111111111111"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --json"
    Then itos exits with code 0
    And the fake GitHub was asked for the run of the commit "1111111111111111111111111111111111111111" alone
    And its JSON's "pull_request.updated" is true
    And its JSON's "ci" is "success"
    And the recording gh's "pr view" call has "--json headRefOid"
