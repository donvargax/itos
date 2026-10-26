@phase-1
Feature: Under push.mode pr, CI trusts the green run of a merged pull request's head for main's run of the same commits, and runs only the prose steps
  GitHub merges a pull request by rebase: the head's commits are made anew
  on main, with new SHAs and the same messages, and, when main gained
  nothing since the head's base, the same trees. Main's run of them then
  runs all that the head's run ran on that tree, and judges nothing new.
  Seen on #160 (2026-10-26): the head b2a7c35d ran the full CI, 12 minutes
  on three platforms; the rebase merge put 0f7b4b43 on main with the same
  tree, 1a6179bb, each of its seven commits with its counterpart's tree,
  and main's run ran everything again, 9 minutes. Every merged pull
  request pays twice. Work done already trusts the head's run for a close
  (work-done-pr-trust.feature, slice 181); this is that trust for CI's own
  plan.

  Where it applies. itos ci plan <from> <to> and itos ci run <from> <to>,
  with or without --json and --tests, for a push's range: <from> neither
  empty nor all zeros, and an ancestor of <to>. Under push.mode pr, with
  ci.range's provider github and its repository_env set (ci range asks
  GitHub only then), and with ci.prose's steps, which judge the
  bookkeeping a trusted range may hold. Anywhere else nothing changes and
  GitHub is not asked: --nightly, --whole, ci plan with no range, trunk
  or no push.mode (as slice 181 asks nothing there), ci.range's provider
  none, no prose steps. ci scope does not ask: it asks no provider (slice
  137's rule). ci range is unchanged: main's range still starts at its
  last green run, so itos verify judges main's commits as before, and a
  trusted run, green, starts the next range.

  The merged pull request. GitHub is asked once, as ci.range's github
  provider asks it (GITHUB_API_URL, the repository and the token it
  reads): GET /repos/<repository>/commits/<t>/pulls, <t> the full SHA of
  <to>. The pull request that counts is the one with merged_at set and
  merge_commit_sha <t>: for a rebase merge GitHub names the last commit it
  made, so only a range that ends at a pull request's merge counts (for
  0f7b4b43 GitHub lists #160, merge_commit_sha 0f7b4b43, head.sha
  b2a7c35d). Its head <h> is its head.sha, <n> its number, <url> its
  html_url. None that counts (the open pull request every pull request's
  own run finds, or a commit pushed to main after the merge, which lists
  none) and a look that fails in any way change nothing and are said
  nowhere.

  The head. GitHub deletes a merged branch here, and CI's checkout fetches
  branches and tags, so the clone often lacks <h>. Then itos fetches it
  once, as work done does (slice 181): git fetch --quiet --no-tags
  <remote> refs/pull/<n>/head, naming no destination, never prompting for
  credentials, within ciwatch's FetchTimeout; <remote> is the current
  branch's upstream remote, else origin, as ci watch picks it. A fetch
  that fails, or after which the clone still lacks <h>, is no trust. A
  head the clone has is never fetched.

  The same commits. The head's commits are those <t> lacks (git rev-list
  --reverse <t>..<h>), the range's those of <from>..<to>, oldest first.
  The trust needs, checked in this order, the first that fails being the
  one said, all before GitHub is asked about a run:
  - no merge commit in either list, and at least one commit of the head's;
  - the range ends with as many commits as the head has, each in order
    with its counterpart's message, byte for byte (the rebase keeps them,
    so the footers main's plan reads name what the head's run's footers
    named);
  - every commit of the range before those, main's own, changes only the
    work registry (work.registry) and the ledger's files, found as slice
    181 finds them (git diff --name-only against its parent; an empty
    commit counts), and names no task and no test in its footers: its
    paths and its names would add to main's plan what no run trusted
    has judged;
  - each of the range's last commits differs from its counterpart only in
    the registry and the ledger's files: git diff --name-only
    <counterpart> <commit>, renames not followed, an empty list counting,
    so main's range changes, outside the bookkeeping, exactly what the
    head's range changed;
  - when <h> and <to> differ in a ledger file, every task the range's
    commits name has the same checks in the ledger at <h> and at <to>,
    compared as slice 181 compares the item's (bug 137): a check main gave
    a named task after the head's run would run in main's whole plan, and
    in no run the trust leaves.
  Endpoints alone are not enough: a code commit and its revert on main,
  neither green, would leave both ends' trees equal while today's plan
  of the range still runs what their footers name.

  The head's run. The newest run of ci.range.github.workflow whose head
  is <h>, on any branch, with its jobs, as ci.watch's provider reads a
  commit's run: the same workflow file whose green runs start main's
  ranges. It passes when it completed with the conclusion success and
  lists a job that was not skipped (a draft's run skips every job and
  judged nothing, slice 179's rule). One still going, failed, cancelled,
  absent, or that cannot be looked at, is no trust.

  What a trusted range runs. The plan a prose-only range gets: the prose
  steps, then the named tasks' static and prose: true checks, their other
  checks left out and listed, an after: push check pending, the tests of
  a task check merged into the run of named tests; no code proof, no
  smoke set, no tests the footers name. A named task the ledger lacks
  still fails the run first, as for any range. ci run --tests runs the
  prose plan's run of named tests, nothing for most ranges, as a platform
  job of a prose push does. itos verify, which a workflow runs beside the
  plan, is not ci's and judges the range as before.

  Why it is sound. The range's commits are the head's, with the same
  messages and, outside the registry and the ledger, the same trees,
  after main's own commits that touch only those files and name nothing.
  So the plan of main's run would name the tasks and tests the head's run
  named, run on a tree whose every other path the head's run checked, and
  its code proof would judge the lines the head's range changed. The
  head's run checked GitHub's merge ref, the head onto main as main was
  then; slice 181's reading holds here: every other path of that tree is
  the head's. The prose steps run on main's tree, so the bookkeeping is
  judged there (config check, in this repository), as for a push of
  bookkeeping alone. ADR-0057 makes the push proof CI's on every push: one
  seeded mutant of the changed functions and the changed lines' coverage.
  The head's push drew and judged them over these same changes; a trusted
  push brings no change that push did not, so its own draw, seeded from
  another start, would be a second sample of the same change, which the
  ADR does not ask for, and it is left out as a prose push leaves it out.
  What the rule accepts, as slice 181 accepts it: the registry stays
  bookkeeping though ci.wait_on_status reads it, so a named task whose
  item was todo in the head's run's tree and is doing at <to> had its
  checks wait in that run and does not get them in the trusted one; an
  item is taken on main before its pull request is marked ready, and the
  ready run's merge ref has the take.

  What it says. One line, which ci plan prints last (for a trusted range
  in place of the line that says why a configured code proof is not
  scheduled, otherwise before that line), and which ci run prints in its
  preamble where a prose-only range's "Only prose changed: ..." line goes
  (on stderr under --json); <h>, <t>, <c> and <hc> are short SHAs:
  - trusted: "Trusted: the pull request <url> merged its head <h> as <t>,
    the same commits with the same trees but for the work registry and
    the ledger, and the head's CI run passed: <run address>. Only the
    prose steps run, and the named tasks' static and `prose: true`
    checks." No "Only prose changed" line and no "Code proof not
    scheduled" line is printed beside it.
  - not trusted, once a merged pull request counts: "Not trusted: the
    pull request <url> merged its head <h> as <t>, but <reason>.", then
    the plan as today, <reason> the first that holds:
    - "this clone lacks <h> and could not fetch it from <remote>'s
      refs/pull/<n>/head";
    - "the range or the head holds a merge commit";
    - "the range does not end with the head's <k> commits", <k> their
      number, when it has fewer or a message differs;
    - "the range holds <c>, which is not the pull request's and changes
      <path>", followed by " and <m> more" when m more such paths are
      listed, or "... which is not the pull request's and names <id>",
      the first ID its footers name;
    - "<c> differs from the pull request's commit <hc> in <path>", the
      first path neither the registry nor a ledger file, " and <m> more"
      as above;
    - "<id>'s checks in the ledger differ from the head's";
    - "the ledger at <s> cannot be read", <s> the short SHA of <h> when
      its ledger cannot be read, else of <to>;
    - "the head's CI run <run address> has not passed", or "no CI run of
      the head was found" when there is none or the look failed.
  Nothing is said when no merged pull request counts.

  --json. ci plan --json, for a trusted range, has "prose" true and a new
  key, "trusted": {"pull_request": <url>, "head": <h's full SHA>, "run":
  <the run's address>}, absent otherwise, so no plan recorded before this
  slice changes. ci run --json says its line on stderr with its log; its
  JSON is unchanged. ci plan's and ci run's help say the trust, ci plan's
  --json line names "trusted", and internal/plan's doc.go says it where it
  says the prose-only range.

  The last release's promises. No scenario or corpus case of the last
  release gives itos a merged pull request (the fake GitHub answers any
  commit's pulls 404 until a step here lists one), so none changes: the
  feat names no case in Changes: and carries no BREAKING-CHANGE:.

  The implementer's unit tests, for what no scenario here shows: each
  condition of where it applies, GitHub not asked; an open pull request,
  none, one whose merge_commit_sha is another commit, and a look that
  fails, each saying nothing; a merge commit in the range and in the
  head's commits; a range with fewer commits than the head's, and one
  whose commit's message differs; a commit of main's own naming a task,
  and one naming a test; " and <m> more"; a ledger that cannot be read at
  <h> and at <to>; a head's run still going, one whose jobs were all
  skipped, none, and a look that fails; the fetch not run for a head the
  clone has, its exact arguments, and a fetch that fails; ci run --tests
  and ci run --json on a trusted range.

  Not in this slice: a squash or merge-commit merge, which this
  repository does not allow; a range whose end is a commit pushed after
  the merge, even a bookkeeping one; main's own commits that change prose,
  which the prose steps would judge; a range holding two pull requests'
  commits, each with a green head; a setting that turns the trust off.
  Nor is this repository's own CI, which picks its metadata route in
  tools/bin/ci-repo rather than through itos ci run: ci-repo's scope, plan,
  run and tests must apply the same rule through the same code (its scope
  "full" false, so the platform jobs and the launcher steps skip), and
  ci.yml must grant the pull-requests: read permission the look needs. A
  gate is ci's to change, so that is a ci task of its own; until it lands,
  main's runs here run everything as today.

  The steps. The world is slice 181's: the remote, the clone where itos
  runs, the item's branch, GitHub's rebase merge and the fake GitHub's
  run of the branch's head (watch.feature), which records it as a run of
  ci.yml, the workflow ci.range asks about here; that step's own ci.watch
  setup is not needed. The CI steps are "echo whole plan" and the prose
  steps "echo prose plan" over **/*.md, so which plan was made reads from
  the output; the item's commit touches src/app.go, no prose path, so
  today every range here plans the whole run. New steps:
  - "the fake GitHub lists the pull request <url> as rebase-merged into
    the remote's main from the head of the remote's branch <branch>" (or
    "from the head of the remote's ref <ref>", for a branch GitHub
    deleted): the fake GitHub answers GET
    /repos/<repository>/commits/<sha>/pulls, for <sha> the remote's main
    at that step, with one pull request: number the address's last
    segment, html_url the address, state closed, merged_at set,
    merge_commit_sha <sha>, head.sha the branch's (the ref's) head at that
    step, head.ref the branch, base.ref main; any other commit's pulls
    with an empty list.
  - "the remote's branch <branch> has the item <id>'s commit naming the
    task <task> and touching <path>, which the clone's main lacks": the
    existing step's commit, with Task: <task> beside its Item: footer.
  - "the remote has gained the commit <subject> deleting <path>": as the
    step that touches a path, removing it instead.
  - "itos plans CI over the commits the fast-forward brought", with
    --json in its ", with --json" form, and "itos runs CI over the commits
    the fast-forward brought": itos ci plan (ci run) <before> <after>, the
    full SHAs of the clone's main before and after its last fast-forward.
  What each Given leaves in the clone is slice 181's; a step that writes
  the config without committing it (the proof's) is followed by "the
  clone commits its changes and pushes them to the remote's main", so
  the fast-forwards find a clean working tree.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the CI steps are "echo whole plan"
    And the prose paths are "**/*.md" and the prose steps are "echo prose plan"
    And ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """

  # The slice's point: main gained a registry commit between the head's
  # base and the merge, which the range holds before the pull request's
  # commit, and the head's run is green. Fails before the work at "echo
  # prose plan": today the range touches src/app.go and plans the whole
  # run.
  @ID-CITRUST-01 @slice-188 @wip
  Scenario: Under pr, ci plan plans only the prose steps for a merged pull request whose head's run passed, when main gained only a registry commit
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "echo prose plan"
    And its output does not say "echo whole plan"
    And its output says "the head's CI run passed: https://ci.example/runs/pr"

  # The boundary, #160's case: main gained nothing, so each merged commit
  # has its counterpart's tree, an empty diff. --json names the trust.
  # Fails before the work at "prose": today the plan is the whole one,
  # prose false, with no trusted key.
  @ID-CITRUST-02 @slice-188 @wip
  Scenario: Under pr, ci plan --json names the trusted run and pull request when the merged commits have the head's trees
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought, with --json
    Then itos exits with code 0
    And its JSON's "prose" is true
    And its JSON's "trusted.run" is "https://ci.example/runs/pr"
    And its JSON's "trusted.pull_request" is "https://github.example/o/r/pull/7"

  # A ledger file differs and the registry does not. The range names no
  # task, so bug 137's comparison has nothing to compare. Fails before the
  # work at "echo prose plan".
  @ID-CITRUST-03 @slice-188 @wip
  Scenario: Under pr, ci plan trusts the merged pull request's green head when main gained only a ledger commit
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "echo prose plan"
    And its output does not say "echo whole plan"

  # ci run carries the trusted plan out: the prose steps run, and the code
  # proof does not, though the range touches proof.code's paths and the
  # provider would find a survivor (ADR-0057's draw was the head's push's).
  # Fails before the work at the exit code: today the proof runs and fails
  # the run, exit 1.
  @ID-CITRUST-04 @slice-188 @wip
  Scenario: Under pr, ci run of a trusted range runs the prose steps and not the code proof
    Given the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the clone commits its changes and pushes them to the remote's main
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos runs CI over the commits the fast-forward brought
    Then itos exits with code 0
    And the provider was not run
    And its output says "Only the prose steps run"
    And its output says "prose plan"
    And its output does not say "whole plan"

  # The head is only on the remote's refs/pull/7/head and its branch is
  # gone, as CI's checkout meets a merged head here: itos fetches it, then
  # trusts. Fails before the work at "echo prose plan".
  @ID-CITRUST-05 @slice-188 @wip
  Scenario: Under pr, ci plan fetches a merged head the clone lacks from the pull request's ref, then trusts its green run
    Given the remote's branch "slice-9" and its ref "refs/pull/7/head" have the item "slice-9"'s commit touching "src/app.go", made in a clone of their own
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main and deletes the branch
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's ref "refs/pull/7/head"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "echo prose plan"
    And its output does not say "echo whole plan"
    And its output says "the head's CI run passed: https://ci.example/runs/pr"

  # Main gained a code commit that its own run judged (the clone's main
  # has it before the merge, so the range starts after it), and the pull
  # request merged onto it without taking it in, as a merge onto a main
  # that moved can: the merged commit's tree is not the head's. Fails
  # before the work at "Not trusted": today nothing is said.
  @ID-CITRUST-06 @slice-188 @wip
  Scenario: Under pr, ci plan plans the whole run when the merged commit's tree differs from the head's in a code path, naming it
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the remote has gained the commit "fix: mend a thing" touching "NOTES.md"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "Not trusted: the pull request https://github.example/o/r/pull/7 merged its head"
    And its output says "in NOTES.md"
    And its output says "echo whole plan"

  # The commits and trees allow the trust, but the head's run failed:
  # main's run plans the whole range. Fails before the work at "has not
  # passed": today nothing is said.
  @ID-CITRUST-07 @slice-188 @wip
  Scenario: Under pr, ci plan plans the whole run when the merged pull request's head's run failed, saying so
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", failed
    And the remote has gained the commit "docs: add slice-2" adding the idea "slice-2" to the registry
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "but the head's CI run https://ci.example/runs/pr has not passed"
    And its output says "echo whole plan"

  # Bug 137's hole, for CI: the pull request's commit names T-001, and
  # main gave T-001 a late check after the head's run, so that run never
  # ran it while main's whole plan does. The paths allow the trust (main
  # gained a ledger commit); only the named task's checks stop it. Fails
  # before the work at "differ from the head's": today nothing is said.
  @ID-CITRUST-08 @slice-188 @wip
  Scenario: Under pr, ci plan plans the whole run when main gave a task the range names a check since the merged pull request's head
    Given the remote's branch "slice-9" has the item "slice-9"'s commit naming the task "T-001" and touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the remote has gained the commit "docs: add a check to T-001" adding the late check "exit 1" to the ledger's task "T-001"
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "but T-001's checks in the ledger differ from the head's"
    And its output says "echo whole plan"

  # Main gained a code commit and its revert inside the range, neither
  # with a green run of its own: both ends' trees match the head's, while
  # the range holds commits no trusted run judged, whose footers today's
  # plan would read. A build that compares the ends alone trusts it.
  # Fails before the work at "which is not the pull request's": today
  # nothing is said.
  @ID-CITRUST-09 @slice-188 @wip
  Scenario: Under pr, ci plan plans the whole run when the range holds a code commit of main's own, though it was reverted
    Given the remote's branch "slice-9" has the item "slice-9"'s commit touching "src/app.go", which the clone's main lacks
    And the fake GitHub's run of the head of the remote's branch "slice-9", "https://ci.example/runs/pr", succeeded
    And the remote has gained the commit "fix: add a note" touching "NOTES.md"
    And the remote has gained the commit "revert: take the note out" deleting "NOTES.md"
    And GitHub rebase-merges the remote's branch "slice-9" onto the remote's main
    And the fake GitHub lists the pull request "https://github.example/o/r/pull/7" as rebase-merged into the remote's main from the head of the remote's branch "slice-9"
    And the clone has fetched the remote
    And the clone fast-forwards its main to the branch "main"
    When itos plans CI over the commits the fast-forward brought
    Then itos exits with code 0
    And its output says "which is not the pull request's and changes NOTES.md"
    And its output says "echo whole plan"
