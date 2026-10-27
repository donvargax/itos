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
  <landing>, which slice 195 makes the merged pull request's merge commit
  when HEAD has it, on <remote>/<main> as last fetched), and once the unpushed
  check and a task's checks have passed, work done asks whether the
  merged pull request's head may judge <landing> before it looks for
  main's run. Without ci.watch nothing changes: CI is not checked, said on
  stderr as before, and no tree is compared. Under trunk, or with no
  push.mode, nothing changes and no gh is called.

  The head. The one gh call slice 176 makes gains headRefOid in its --json:
  gh pr list --repo <repository> --head <id> --state all --json
  state,mergedAt,number,url,headRefOid (slice 195 adds mergeCommit,
  work-done-pr.feature), the same call whatever ci.watch says. The pull
  request that counts is the merged one slice 176 picks (mergedAt set, the first gh lists, its newest), and its head is its
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
  ledger edit, run in main's own run of the commit that made it. A diff
  git cannot give is no comparison: work done waits for main's run. For
  a task the comparison has one more condition, its own checks (bug 137,
  below).

  A head the clone lacks. The head GitHub merges is often one its
  update-branch made (itos push --ready asks for it whenever main moved),
  which never reaches the clone, and GitHub deletes a merged branch when
  the repository says so, as this one does, so git pull does not bring it
  either. So when the clone does not have the head's commit, work done
  fetches it once from the remote, before it compares: git fetch --quiet
  --no-tags <remote> refs/pull/<n>/head, <n> the number gh pr list gave
  for the merged pull request, the ref GitHub keeps for every pull
  request after its branch is gone. It names no destination, so no branch
  and no remote-tracking ref is updated (git writes FETCH_HEAD alone); it
  never prompts for credentials, as the remote is asked under pr (slice
  176), and is given ciwatch's fetchTimeout. A fetch that fails, for any
  reason and whatever git's exit, or one after which the clone still does
  not have the head (the ref names another commit, or none), is no
  comparison: work done waits for main's run, the line saying why, and
  the exit code is what main's run makes it. A head the clone has is
  never fetched.

  Why it is sound. A run judges its tree and the commits of its range.
  The rebase keeps every commit's message, so the head's run judged the
  messages main's run would; what main gained beside them is registry and
  ledger edits, judged by main's runs of their own commits. The tree the
  head's run judged is not quite the head's: a pull_request run checks out
  GitHub's merge ref, refs/pull/<n>/merge, the head merged onto main as
  main was when the run started. Main only gains commits, so main of that
  moment lies between the head's base and the main the pull request merged
  onto, and the run's tree is the head's plus part of main's gains. The
  comparison says the gains all together touch only the registry and the
  ledger, so every other path of the run's tree is the head's, and the
  merged tree's too. The one way past that is main gaining a change to
  another path and its revert, the change before the run and the revert
  after it; that is accepted (guardrails over hardening), since both
  commits had main's runs of their own. After update-branch the head's
  base is main of that moment, and the merge ref's tree is the head's own.

  The item's own checks (bug 137, issue 141). The paths are not all a
  run judges: CI runs a task's checks only for a task its range names
  (plan.For runs TasksIn's), read from the ledger of the tree it runs
  on. A ledger commit that gives the item's own task a check and reaches
  main after the head's run started names no task, or, naming it, runs
  it on a tree without the item's work; main's run of the merged
  commits, which name the task, runs it on the merged tree, and slice
  176 waited for that run. Trusted, the head's run closes the task
  though it never had the check, the close takes the task's checks out
  of the ledger, and no later run names them: a late check, which work
  done leaves to CI and never runs, ran nowhere. So for an item that is
  a task of the ledger (ledger.id set and the item's id matching it, as
  work done decides that it runs a task's checks), when git lists a
  ledger file the comparison also reads the ledger at the head and at
  <landing>, each as itos ci plan --data-at reads a commit's ledger (the
  config as loaded, the ledger files that commit's tree has), and holds
  only when the item's checks are the same at both: the entries with
  its id, in the order the ledger reads them, as many at both, each
  pair's done_when equal check by check in every key the ledger reads
  of a check (run, fails, after, timeout, cost, prose, evidence,
  snapshot), and neither commit with an entry of the id whose checks
  cannot be read (a done_when that is no list, a check with no command).
  No entry at either is the same. Otherwise <on>'s CI run judges
  <landing>, the head's run not looked at. A ledger work done cannot
  read at either commit (a file that is not YAML, or not a list) is no
  comparison: main's run judges, a line saying so. The paths are still
  compared first, so a tree that differs in another path says that and
  reads no ledger; a slice or a bug, which has no entry, reads none and
  is unchanged, and so is a task when git lists no ledger file.

  What the rule leaves alone, each weighed by what waiting for main's
  run would have stopped:
  - The item's title, why and type, and the file its entry is in: the
    plan reads a task's checks, naming a failing one by its title
    alone; itos verify holds a commit's Task: to the ledger by its ID
    alone (ledger.IDs); config check judges an entry's form in
    main's run of the commit that changed it, as above. So none of them
    changes a verdict of main's run of the merged commits, and none is
    compared; nor is the entry's text, so a reformatted entry is the
    same entry.
  - Another task's entry, a check main gave a task the pull request's
    commits name included: main's run runs that check whether or not
    work done waits, CI running on every push to main, and the check
    stays in the ledger, so each later run of main, whose range starts
    at the last green run, names the task again and stays red until it
    passes, and the check still gates its own task's close. Only the
    item's close takes checks out, so only the item's can be lost.
  - The task removed and added again: compared as each commit has it.
    The rebase puts the pull request's commits on main as the merge
    found it, so every merged commit's ledger is <landing>'s but for
    the pull request's own edits: the same checks at both is trusted;
    other checks, or no entry at <landing> (removed for good), is
    main's run's to judge.
  - A check given in a file other than the entry's: a second entry of
    the id there makes the entries differ in number, and the plan runs
    every entry of a named id; config check refuses the duplicate in
    its own commit's run.
  - ledger.files, ledger.id, ledger.group or any key of the config:
    itos.yaml is no bookkeeping path, so the trees already differ, the
    ledger is read with the one config both commits have, and an entry
    moved to a file ledger.files does not match is a path that is no
    ledger file.
  - work done's own checks. It runs the task's static checks before it
    looks at CI, from the ledger the clone has, which after the
    fast-forward has main's check, so a static check main added is run
    here; and the close's write refuses a ledger file, of a task with
    checks, that is not sound. But those runs are local feedback, which
    a record a hand run left can answer, and slice 176 made CI's run the
    proof: they close none of this for a static check, and nothing for
    a late one, which work done says it leaves to CI. An after: push
    check is pending in a push's run and in work done alike, so the
    trust changes nothing for it; it is compared with the rest, the
    rule being simpler for it.
  Weighed and not taken: comparing only the checks work done does not
  run (the late ones) would put a local run, or a hand run's record, in
  CI's place; trusting when the checks at <landing> are among the
  head's, a removal asking no new verdict, is sound but saves a wait
  only when a check is taken off an open item's task on main, and
  equality is the simpler rule; comparing the item's whole entry, as
  issue 141 asks, would make a why edited while the pull request is
  open cost the wait for nothing.

  What the rule accepts. A check main gave the task after the head's
  base and before the head's run started was in the run's tree, the
  merge ref, but is not in the head, so work done waits for main's run
  though the head's run judged it; after update-branch, which itos
  push --ready asks for whenever main moved, the head has it. And the
  registry, which stays bookkeeping though ci.wait_on_status reads it:
  a named task whose status that lists at the run's tree is held back,
  its checks not run, so an item taken on main only after its head's
  run started had them left out of that run. That is not compared:
  work done closes only an item that is doing, the item is taken
  before its pull request is marked ready, and the ready run's tree
  has the take (guardrails over hardening).

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
  - the item's checks differ (bug 137): the trusted line's start, to
    "but for the work registry and the ledger", then ", but <id>'s
    checks in the ledger differ from the head's, so <on>'s CI run
    judges <l>";
  - the ledger cannot be read (bug 137): the same start, then ", but
    the ledger at <s> cannot be read, so <on>'s CI run judges <l>", <s>
    the short SHA of the head when its ledger cannot be read, else <l>;
  - the head is not in the clone, after the fetch: "<id>'s pull request
    <url> merged with the head <h>, which this clone does not have and
    could not fetch from <remote>'s refs/pull/<n>/head, so <on>'s CI run
    judges <l>". A head fetched says nothing of the fetch: the comparison's
    line follows as for a head the clone had.
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

  The implementer's unit tests, for what no scenario here shows: the close
  commit's body sentence when the head's run judged it, and slice 127's
  sentence otherwise; the "no CI run of the head was found" line, for a
  head with no run and for a look that fails; a head's run still going,
  and one whose jobs were all skipped or that lists none, each no pass;
  " and <n> more" after the first path; -q saying no line; pull_request
  absent with no merged pull request, with a merged one gh gives no
  headRefOid for, and without ci.watch, and present in work-done-ci's
  refusal; the fetch: not run for a head the clone has, its exact
  arguments, and a fetch that fails or after which the head is still
  missing, each giving the could-not-fetch line and main's run; a diff git
  cannot give.

  Not in this slice: looking at the head's run when the trees differ; a
  squash or merge-commit merge, which this repository does not allow (the
  comparison would hold for one; whether work done finds the item's commit
  on main, by its footers, is slice 176's question).

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
  "GitHub rebase-merges the remote's branch <branch> onto the remote's
  main and deletes the branch" does the same, then deletes the remote's
  branch, as GitHub deletes a merged branch. "the remote's branch <branch>
  and its ref <ref> have the item <id>'s commit touching <path>, made in a
  clone of their own": the config's registry footer turned on as the
  branch step turns it on, then, in a new clone of the remote's main that
  no other step uses, the item's commit as the branch step makes it,
  pushed past the hooks to the remote's refs/heads/<branch> and to <ref>
  (refs/pull/7/head here, as GitHub keeps a pull request's head), so the
  clone where itos runs never has that commit's object until something
  fetches it. "the clone commits its changes and pushes them to the
  remote's main": every change of the clone's working tree (git add -A),
  committed past the hooks naming the ledger's first task as the config's
  commits do, and pushed to the remote's main. "the remote has gained the
  commit <subject> adding the task <id> to the ledger", which, as the
  registry steps do (the clone's HEAD pushed to the remote's main first,
  the other clone then reset to it), adds the task's line, with no checks,
  in the other clone to the scratch ledger's one file where ledger.files
  puts it, and commits that file alone and pushes it to the remote's main
  past the hooks. "the fake GitHub's run of the head of the remote's branch
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

  What each Given leaves in the clone, since the fast-forward needs a
  clean working tree. The Background's registry step commits the registry
  and the people in the clone past the hooks (commitLeavingStaged, as in
  any project), and the config step pushes that commit to the remote's
  main with its own, before any branch is made, so the head's base has
  the registry and PRTRUST-02's diff is empty. The config steps commit
  itos.yaml alone and push; the branch step commits only its file, on a
  branch it deletes, and pushes; the registry, code and ledger steps, the
  pull request's ref step and both merge steps commit in other clones; the recording gh's and the fake
  GitHub's steps touch no repository. "the ledger's files are <pattern>"
  is the one that leaves changes uncommitted (the ledger moved, the config
  rewritten), so PRTRUST-03 commits and pushes them before its branch.

  Bug 137's fix. Its scenarios are PRTRUST-08, which fails before the
  work, and the guard PRTRUST-09. The fix names no case in Changes:, as
  no scenario or case of v8.19.0 closes a task under pr on a merged
  head, and says Upgrading: none. It adds the condition where work
  done's help and the work package's doc.go say the trust ("when the
  commit's tree differs from the head's only in the work registry and
  the ledger files"). The implementer's unit tests, for what no
  scenario here shows: the cannot-be-read line, for the head and for
  <landing>; an entry moved to another ledger file with its checks,
  trusted; the entry gone at <landing>, and gone and added again with
  other checks, each main's run, and added again with the same checks,
  trusted; a second entry of the id in another file; a check whose
  timeout, cost or after alone changed; an entry whose checks cannot be
  read; a slice whose tree differs in a ledger file that cannot be
  read, trusted, no ledger read; a task when git lists the registry
  alone, no ledger read; --json's pull_request, trusted false, when the
  checks differ.

  Bug 137's steps. Its scenarios close the Background's task T-001:
  "the task <id> has the committed check <command>" (reused) gives it a
  static check, here "true", which work done runs and which passes,
  committed in the clone with the ledger alone (PRTRUST-09 gives the
  task T-002 one the same way, adding it), and the registry step
  makes the item T-001 doing as it makes slice-9, both before the
  config step pushes the clone's commits, so the head's base has them.
  New steps, each editing the scratch ledger's one file, where
  ledger.files puts it, in the other clone the registry steps use,
  brought up to the remote's main (cloned, or fetched and reset to
  origin/main), the clone's HEAD pushed to the remote's main first
  unless the remote's main has it already, so that one may follow
  another; the file committed alone past the hooks and pushed to the
  remote's main; each failing when the file has no task of the id:
  "the remote has gained the commit <subject> adding the late check
  <command> to the ledger's task <id>", which appends { run: <command>,
  cost: late } to the task's done_when, making one when it has none,
  its other checks and keys as they were; "the remote has gained the
  commit <subject> giving the ledger's task <id> the why <why>", which
  sets the task's why, its other keys and checks as they were. Either
  may write the file in another form, so long as the ledger reads every
  other task as before. The item's commit is the branch step's, linked
  to T-001 by its Item: footer: the rule reads no footer, so which
  footer names the task is not these scenarios' question.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: main gained a registry commit between the head's
  # base and the merge, and the head's green run judges the close. Main's
  # run never finishes, so only the trust closes in time. Fails before the
  # work at the exit code: today main's run is waited for until
  # ci.watch.timeout, exit 75.
  @ID-PRTRUST-01 @slice-181
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
  @ID-PRTRUST-02 @slice-181
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
  # it reads ledger.files from the config: the ledger is moved off itos's
  # default name, so a tasks/phase-<n>.yaml written into itos fails here.
  # It stays in tasks/, beside the registry, since without work.registry
  # itos looks for the registry beside the ledger (ID-WORK-04). The move
  # is committed and pushed before the branch, so only the task differs.
  # Fails before the work at the exit code: today main's run is waited for
  # until ci.watch.timeout, exit 75.
  @ID-PRTRUST-03 @slice-181
  Scenario: Under pr, work done closes on the head's green run when main gained only a ledger commit since its head
    Given the ledger's files are "tasks/ledger-{group}.yaml"
    And the clone commits its changes and pushes them to the remote's main
    And the config, committed and pushed to the remote's main, adds the YAML:
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
  @ID-PRTRUST-04 @slice-181
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
  @ID-PRTRUST-05 @slice-181
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
  @ID-PRTRUST-06 @slice-181 @guard
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

  # The head GitHub merged is only on the remote, as one update-branch made
  # is, and its branch is gone, so git pull did not bring it: work done
  # fetches refs/pull/7/head, then compares and trusts. Fails before the
  # work at the exit code: today main's run is waited for until
  # ci.watch.timeout, exit 75.
  @ID-PRTRUST-07 @slice-181
  Scenario: Under pr, work done fetches a merged head the clone lacks from the pull request's ref, then closes on its green run
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "slice-9" and its ref "refs/pull/7/head" have the item "slice-9"'s commit touching "widget.md", made in a clone of their own
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main and deletes the branch
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "so the head's CI run judges"
    And its output says "CI passed: https://ci.example/runs/pr"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  # Bug 137's hole: after the head's run, main gave the item's own task a
  # check, so that run never had it, while main's run of the merged
  # commits would run it. The check is late, so work done leaves it to CI
  # and never runs it, and it fails: the close the trust makes today
  # lands on a check that ran nowhere and takes it out of the ledger. The
  # paths alone allow the trust (main gained a ledger commit), so only the
  # item's checks stop it, and main's run, which never finishes, is waited
  # for. Fails before the work at the exit code: today the head's run
  # closes it, exit 0.
  @ID-PRTRUST-08 @bug-137
  Scenario: Under pr, work done waits for main's run when main gave the item's own task a check since the merged pull request's head
    Given the task "T-001" has the committed check "true"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "T-001" has the item "T-001"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "T-001", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "T-001"
    And the remote has gained the commit "docs: add a check to T-001" adding the late check "exit 1" to the ledger's task "T-001"
    And GitHub rebase-merges the remote's branch "T-001" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done T-001"
    Then itos exits with code 75
    And its output says "but T-001's checks in the ledger differ from the head's, so"
    And its output does not say "https://ci.example/runs/pr"
    And the registry's item "T-001" has the status "doing" and the owner "someone"

  # Bug 137's boundary: main's ledger commits since the head are still
  # bookkeeping when the item's checks are the same at both. Another task,
  # T-002, gained a late check, which gates T-002's own close and stays in
  # the ledger, and the item's own entry gained a why, which no run
  # judges. A guard: it passes before the work and must after it, so a fix
  # that compares the ledger files, or the item's whole entry, fails it.
  @ID-PRTRUST-09 @bug-137 @guard
  Scenario: Under pr, work done still closes a task on the head's green run when main changed only another task's checks and the task's own why
    Given the task "T-001" has the committed check "true"
    And the task "T-002" has the committed check "true"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run never finishes
    And ci.watch.timeout is 3
    And the remote's branch "T-001" has the item "T-001"'s commit touching "widget.md", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "T-001", "https://ci.example/runs/pr", succeeded
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "T-001"
    And the remote has gained the commit "docs: add a check to T-002" adding the late check "exit 1" to the ledger's task "T-002"
    And the remote has gained the commit "docs: say why T-001" giving the ledger's task "T-001" the why "Main says why."
    And GitHub rebase-merges the remote's branch "T-001" onto the remote's main
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs "work done T-001"
    Then itos exits with code 0
    And its output says "so the head's CI run judges"
    And its output says "CI passed: https://ci.example/runs/pr"
    And the registry's item "T-001" has the status "done" and the owner "someone"
