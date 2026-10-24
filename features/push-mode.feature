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
  was, whatever the branch.

  Under pr, itos push reads the config before it fetches. Main is the
  remote's default branch, the one the remote's HEAD names; the remote is
  the branch's upstream's, else origin, as before.

  On main, the commits the push adds that touch only the work registry (a
  take, a close, a queue move, an idea: slice 56's rule) go straight to main
  as under trunk, so two agents never take one item. A push that adds any
  other commit is refused once rebased onto main, exit 1, nothing pushed
  (outcome work-on-main), saying to carry the work to its item's branch with
  git switch -c <item>.

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
  the pre-push hook judging both.

  Then, unless the branch adds nothing to main, its pull request: gh pr
  create --base <main> --head <item> --fill when no pull request of the
  branch is open; gh pr merge <item> --auto --rebase, never squash (each
  commit is a changelog entry with its footers, and GitHub's squash appends a
  Co-authored-by paragraph that pushes them out of the trailer block); and,
  when main has a commit the pushed head lacks, gh pr update-branch <item>
  --rebase, so GitHub rebases the branch rather than itos forcing it. These
  run with nothing to push too, so itos push again finishes a push whose
  pull request was not opened. The push prints the pull request's address.

  itos reads no remote URL: gh finds the GitHub repository off the clone's
  remotes, by its own rules. With no gh on the PATH, or a gh call that fails
  (signed out, no GitHub remote, a refusal), the branch is pushed already:
  exit 3 (outcome pull-request-failed), gh's words and the gh command that
  failed named, and itos push again to finish it.

  No CI run is waited for, whatever ci.watch says and --no-wait or not: the
  branch has no run until CI runs on pull_request, a later item; GitHub shows
  the pull request's checks at its address, and auto-merge waits for them.
  --tag and --ref push what they name and nothing else, as under trunk.

  --json adds "pull_request": {"url","number","created","updated"} after a
  push from an item's branch, created saying whether it opened the pull
  request, updated whether it asked GitHub to rebase it; outcome gains
  work-on-main, not-item-branch and pull-request-failed.

  Not in this slice: work done is unchanged (rebase-merge keeps each commit's
  footers, so a main holding the merged commits closes the item as before;
  refusing to close while the pull request is open is p1-pr-items); itos
  init offering the repository's settings; CI on pull_request and watching a
  pull request's checks; a pull request GitHub cannot rebase onto main; a
  registry commit made on the branch above the work, which rides the pull
  request.

  The recording gh stands in for GitHub: first on the PATH, it records each
  call, answers pr create with the address the scenario gives and then holds
  that pull request open for pr view and pr list; the remote is a bare
  repository whose HEAD names main.

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
    And the recording gh's "pr create" call has "--base main"
    And the recording gh's "pr create" call has "--head slice-9"
    And the recording gh's "pr merge" call has "slice-9"
    And the recording gh's "pr merge" call has "--auto"
    And the recording gh's "pr merge" call has "--rebase"
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
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 3
    And the remote has the branch "slice-9" with "feat: add the widget"
    And its output says "gh pr create"
    And its output says "itos push again"

  # CI runs on pushes to main today; a branch has no run to wait for until
  # CI runs on pull_request, a later item. With ci.watch set, the push still
  # waits for none and names the pull request, where GitHub shows its
  # checks. Fails before the build at the fake GitHub line: today the push
  # waits for the pushed commit's run.
  @ID-PUSHMODE-12 @slice-174 @wip
  Scenario: Under pr, a push from an item's branch waits for no CI run, whatever ci.watch says
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the fake GitHub was never asked about a run
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
