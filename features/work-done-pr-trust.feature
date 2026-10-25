@phase-1
Feature: Under push.mode pr, work done trusts the merged pull request's green run when main gained only registry and ledger changes
  Slice 176 (work-done-pr.feature) made work done, under push.mode pr, close
  an item only once its pull request has merged and main's run of the
  item's newest commit there has passed, waiting for that run while it goes.
  GitHub merges by rebase: the merged commits are the pull request's, with
  new SHAs, on top of whatever main gained since the head's base, so their
  tree is the head's plus main's gains. When main gained nothing, or only
  work registry and ledger changes, the pull request's own green run has
  already judged all that a run judges of that tree, and the wait for
  main's run, 6½ minutes and more on each close here, proves nothing new.
  Once the ruleset's strict status checks are off, a pull request may merge
  onto a main that moved after its run, so the trees decide, never a
  setting.

  Where it applies. Under pr with ci.watch, once slice 176's rules let the
  close go on (no open pull request; the item's newest own commit, here
  <landing>, on <remote>/<main> as last fetched), and once the unpushed
  check and a task's checks have passed, work done asks whether the
  merged pull request's head may judge <landing> before it looks for
  main's run. Without ci.watch nothing changes: CI is not checked, said on
  stderr as before, and no tree is compared. Under trunk, or with no
  push.mode, nothing changes and no gh is called.

  The head. The one gh call slice 176 makes gains headRefOid in its --json:
  gh pr list --repo <repository> --head <id> --state all --json
  state,mergedAt,number,url,headRefOid, the same call whatever ci.watch
  says. The pull request that counts is the merged one slice 176 picks
  (mergedAt set, the first gh lists, its newest), and its head is its
  headRefOid: for a merged pull request, the last head GitHub had when it
  merged, the head itos push --ready waited for, one GitHub made by
  update-branch included. That head is never on main, since the rebase
  gave its commits new SHAs, so it is never matched to a commit of main by
  SHA, message or patch: the comparison below is the whole proof,
  whichever of the pull request's commits <landing> is. A merged pull
  request gh lists with no headRefOid (missing or empty), or no merged
  pull request at all (the item's commit reached main some other way),
  changes nothing and is said nowhere.

  The comparison. git diff --name-only <head> <landing>, the two commits'
  trees whole, renames not followed, so a rename lists both paths. The
  head may judge <landing> when every path listed is the work registry
  (work.registry) or a ledger file, found as slice 177 finds one: a path in
  ledger.files' folder whose name its pattern matches, its {group} by
  ledger.group.pattern, none with no ledger section, both read from the
  config as loaded, never a path written into itos. An empty list counts:
  the trees are equal, main gained nothing. Unlike slice 177's rule for a
  commit, the registry need not be among the paths, so a tree that differs
  in a ledger file alone counts too: issue 134's decision that a ledger
  change is bookkeeping, since the full plan runs a task's checks only for
  a task its range names, and itos verify and config check, which judge a
  ledger edit, run in main's own run of the commit that made it. A head the
  clone does not have, or a diff git cannot give, is no comparison: work
  done waits for main's run, and fetches nothing.

  Why it is sound. A run judges its tree and the commits of its range. The
  rebase keeps every commit's message, so the head's run judged the
  messages main's run would; what main gained beside them is registry and
  ledger edits, judged by main's runs of their own commits.

  The head's run. Once the comparison holds, the head's run is looked at
  once, never waited for: the newest run of ci.watch.github.workflow whose
  head is the head's SHA, on any branch, as ci.watch's provider looks up
  a commit's run (here the pull request's run once it was marked ready). It
  judges <landing> when it completed with the conclusion success and
  lists a job that was not skipped: a run whose jobs were all skipped, or
  that lists none, is a draft's and judged nothing (slice 179's rule).
  Main's run is then neither looked at nor waited for, and the close goes
  on with the head's run as its CI.

  Otherwise slice 176's way, unchanged: main's runs, any run covering
  <landing> that passed, else the wait, a red one refused (work-done-ci).
  A head's run that failed, is still going, is not there, or cannot be
  looked at refuses nothing and waits for nothing: the head's run can
  spare the wait for main's run, never stand in for main's verdict when it
  is no pass. Refusing on a red head's run would put the pull request's
  verdict above main's, the authority slice 176 named, for a pull request
  that merged regardless (an admin's merge, a check not required); and a
  merged pull request's head's run still going (a re-run) is a race no
  better judged than by main's run of the merged commit.

  What it says. One line, where the watch's progress goes (stdout, stderr
  under --json, nowhere under -q), before the run's end line; <h> and <l>
  are the short SHAs of the head and <landing>, <on> is <remote>/<main>:
  - trusted: "<id>'s pull request <url> merged with the head <h>, and <l>,
    <id>'s newest commit on <on>, has its tree but for the work registry
    and the ledger, so the head's CI run judges <l>", then the run's end
    as ciwatch says it, "CI passed: <run address>";
  - the trees differ: "<id>'s pull request <url> merged with the head <h>,
    and <l>, <id>'s newest commit on <on>, differs from it in <path>, so
    <on>'s CI run judges <l>", <path> the first path git lists that is
    neither the registry nor a ledger file, followed by " and <n> more"
    when n more such paths are listed;
  - the head's run is no pass: the trusted line's start, to "but for the
    work registry and the ledger", then ", but the head's CI run <run
    address> has not passed, so <on>'s CI run judges <l>", or ", but no CI
    run of the head was found, so <on>'s CI run judges <l>" when there is
    none or the look failed;
  - the head is not in the clone: "<id>'s pull request <url> merged with
    the head <h>, which this clone does not have, so <on>'s CI run judges
    <l>".
  After any line but the trusted one, slice 176's look and wait follow,
  saying what they say today. The close commit's body, when the head's
  run judged it, names that run in place of slice 127's sentence: "The CI
  run of <h>, the head of <id>'s pull request <url>, whose tree <l> has but
  for the work registry and the ledger, passed: <run address>."

  --json. When the head's run judged the close, ci is success and run is
  the head's run, as the watch gives a run. A new key, pull_request, comes
  beside ci and run wherever they are given (the close, and work-done-ci's
  refusal) once the merged pull request has a head and ci.watch is on:
  {"url": <its address>, "head": <its head's full SHA>, "trusted": <true
  when the head's run judged the close, else false>}. It is absent
  otherwise: trunk, no ci.watch, no merged pull request, none with a head.
  Nothing else in the JSON changes.

  Slice 176's promises. Its scenarios keep theirs. ID-PRDONE-07, its one
  close that waits for main's run, gives the recording gh its merged pull
  request before the remote has any branch slice-9, so gh lists it with no
  head and work done waits for main's run as the scenario says; no other
  scenario of its reaches the run, and the last release's steps (v8.17.0's)
  give no head at all. What no longer holds is a sentence of its
  description, "The merge gave the commits new SHAs, so the run that
  judges them is main's, never the pull request's": the implementing
  commit rewords it to point here, and work done's help and the work
  package's doc.go say the trust where they say main's run. On this
  reading the feat names no case in Changes: and carries no
  BREAKING-CHANGE:.

  Not in this slice: fetching a head the clone lacks (GitHub keeps it as
  refs/pull/<number>/head after it deletes a merged branch, as this
  repository has it do); looking at the head's run when the trees differ;
  a squash or merge-commit merge, which this repository does not allow
  (the comparison would hold for one; whether work done finds the item's
  commit on main, by its footers, is slice 176's question).

  The steps. The recording gh is push-mode.feature's, the fake GitHub
  ci.watch's (watch.feature), and an item's commit one with an Item:
  footer, as slice 176's steps make it. The recording gh learns one
  thing: pr list gives each pull request's headRefOid, the head the
  remote's branch had when a step gave the recording gh that pull request
  (the head it already keeps for pr view), when the call's --json names
  headRefOid and the branch had a head then; a pull request given before
  its branch exists has none. New steps: "GitHub rebase-merges the
  remote's branch <branch> onto the remote's main", which, in a clone of
  its own as pr update-branch rebases, rebases the branch's commits that
  main lacks onto the remote's main, each made anew with the committer
  GitHub <noreply@github.com>, so every SHA is new even when main
  gained nothing, and pushes them to the remote's main past the hooks,
  leaving the branch where it was; it fails when a commit kept its SHA.
  "the remote has gained the commit <subject> adding the task <id> to the
  ledger", which, in the other clone the registry steps commit in, adds
  the task's line, with no checks, to the scratch ledger's one file, and
  commits that file alone and pushes it to the remote's main past the
  hooks. "the fake GitHub's run of the head of the remote's branch
  <branch>, <address>, succeeded" (or failed): a completed run of the
  watched workflow at the address, of the head the remote's branch has at
  that step, on the branch <branch> as a pull request's run is, its jobs
  ci and platform succeeding (ci failing), newer than every run before
  it, ci.watch set up to ask the fake GitHub as the step for an item's
  commit's run sets it up when no step has. Reused: the clone takes the
  merge with "the clone has fetched the remote" then "the clone
  fast-forwards its main to the branch main" (git merge --ff-only
  origin/main). In the trusted scenarios main's run never finishes and
  ci.watch.timeout is 3, so a wait for it ends in exit 75 within seconds.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: main gained a registry commit between the head's
  # base and the merge, and the head's green run judges the close. Main's
  # run never finishes, so only the trust closes in time. Fails before the
  # work at the exit code: today main's run is waited for until
  # ci.watch.timeout, exit 75.
  @ID-PRTRUST-01 @slice-181 @wip
  Scenario: Under pr, work done closes on the merged pull request's green run when main gained only a registry commit since its head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "so the head's CI run judges"
    And its output says "CI passed: https://ci.example/runs/pr"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # The boundary: main gained nothing, so the merged tree is the head's,
  # an empty diff, and it counts. --json names the run and the pull
  # request. Fails before the work at the exit code: today main's run is
  # waited for until ci.watch.timeout, exit 75.
  @ID-PRTRUST-02 @slice-181 @wip
  Scenario: Under pr, work done --json closes on the head's green run when the merged tree is the head's, naming the pull request as trusted
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 0
    And its JSON's "ci" is "success"
    And its JSON's "run.url" is "https://ci.example/runs/pr"
    And its JSON's "pull_request.url" is "https://github.example/o/r/pull/7"
    And its JSON's "pull_request.trusted" is true
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # A ledger file differs and the registry does not: the comparison is not
  # slice 177's commit rule, which needs the registry among the paths, and
  # it reads ledger.files from the config. Fails before the work at the
  # exit code: today main's run is waited for until ci.watch.timeout,
  # exit 75.
  @ID-PRTRUST-03 @slice-181 @wip
  Scenario: Under pr, work done closes on the head's green run when main gained only a ledger commit since its head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "so the head's CI run judges"
    And its output says "CI passed: https://ci.example/runs/pr"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # Main gained code, so the head's run did not judge the merged tree:
  # main's run does, as slice 176 says, and the line names the path. The
  # head's run is green and is not looked at. Fails before the work at the
  # "differs from it in NOTES.md" step: today main's run closes it, saying
  # nothing of the pull request's head.
  @ID-PRTRUST-04 @slice-181 @wip
  Scenario: Under pr, work done waits for main's run when main gained a code commit since the head, naming the path
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "fix: mend a thing" touching "NOTES.md"
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "differs from it in NOTES.md"
    And its output says "CI passed: https://ci.example/runs/1"
    And its output does not say "https://ci.example/runs/pr"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # The trees allow the trust, but the head's run failed: that refuses
  # nothing, and main's green run closes it. Fails before the work at the
  # "has not passed" step: today main's run closes it, saying nothing of
  # the head's run, and the JSON has no pull_request.
  @ID-PRTRUST-05 @slice-181 @wip
  Scenario: Under pr, work done --json waits for main's run when the merged pull request's head's run failed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", failed
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9 --json"
    Then itos exits with code 0
    And its output says "but the head's CI run https://ci.example/runs/pr has not passed"
    And its JSON's "run.url" is "https://ci.example/runs/1"
    And its JSON's "pull_request.trusted" is false
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # Without ci.watch CI is not checked, and no tree is compared or line
  # said, though the trust's conditions hold. A guard: it passes before
  # the work and must after it.
  @ID-PRTRUST-06 @slice-181 @wip @guard
  Scenario: Under pr without ci.watch, work done closes a merged item as before, comparing no trees
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "ci.watch.provider is none, so CI was not checked"
    And its output does not say "CI run judges"
    And the registry's item "slice-9" has the status "done" and the owner "someone"
