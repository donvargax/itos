@phase-1
Feature: Under push.mode pr with push.pr.draft, pull requests open as drafts and itos push --ready marks them ready, waits for the checks and the reviews, then turns on auto-merge
  Under push.mode pr (slices 174 and 175, push-mode.feature) every push from
  an item's branch opens or updates its pull request, turns on auto-merge by
  rebase and waits for the run: CI judges every push, and a green run merges
  work nobody has reviewed. The person wants an agent to push as it works,
  with nothing judging or merging it, hand back a draft for review, and only
  then ask for the checks, Copilot's review and the merge (the person,
  2026-10-25, issue 124):

    the agent pushes (a draft, no CI); the coordinator has the Opus reviewer
    post its verdict on the pull request; the agent addresses it and runs
    itos push --ready, which waits for CI and Copilot's review; the agent
    addresses Copilot's findings (a fix, or a reply), resolves them, and
    runs itos push --ready again; the merge follows on green; the
    coordinator closes the item on main.

  The config. push.pr is a new section of push: push.pr.draft (a bool,
  false by default) and push.pr.review_by (a list of GitHub logins, empty by
  default). Left out, pr pushes exactly as slices 174 and 175 made it, so
  nothing changes for a repository that does not ask. config check accepts
  both, refusing a draft that is not a bool and a review_by that is not a
  list of strings as it refuses any value of the wrong type; config get
  push.pr.draft prints false and config get push.pr.review_by the empty list
  when the config leaves them out; the schema (internal/config/schema.go)
  lists both, with their words: draft "Whether itos push opens and updates
  an item's pull request as a draft, which nothing judges or merges until
  itos push --ready marks it ready (push.mode pr)." and review_by "The
  GitHub logins whose review itos push --ready waits for before it turns on
  auto-merge; empty, it waits for none." Both are read only under push.mode
  pr. This repository turns them on (push.pr: {draft: true, review_by:
  ["copilot-pull-request-reviewer[bot]"]}) in a build commit of its own,
  once the ruleset below is applied, and rewrites AGENTS.md's Finishing for
  the --ready flow in the same push.

  A plain itos push from an item's branch, under push.pr.draft true, does
  what slice 174 does up to the pull request (the rebase, the registry
  commits to main, the atomic push) and then: gh pr list --repo <repository>
  --head <item> --state open --json number,url,isDraft (isDraft is asked
  under either setting of draft); with none open, gh pr create --repo
  <repository> --base <main> --head <item> --fill --draft. It turns on no
  auto-merge (no gh pr merge), asks GitHub for no rebase (no gh pr
  update-branch, even when main has moved: nothing judges a draft, and
  --ready asks for it), and waits for no run, ci.watch or not. It never
  changes a pull request's draft state: one already ready stays ready. It
  prints the pushed line as before, "Pushed <short> to <remote>/<item>. Pull
  request: <address>" (or, with nothing to push, "Nothing to push: <item> is
  at <remote>/<item>. Pull request: <address>"), then, for a draft, "The
  pull request is a draft: nothing judges or merges it until itos push
  --ready marks it ready."; for one that is ready, "The pull request is
  ready: itos push --ready waits for its checks and reviews, then turns on
  auto-merge." Exit 0, outcome pushed (or nothing-to-push).

  itos push --ready, from an item's branch under push.pr.draft true, pushes
  as a plain push does, then:
  1. The pull request: none open, gh pr create as slice 174 makes it,
     without --draft (it opens ready, so there is nothing to mark); a draft,
     gh pr ready <item> --repo <repository>; one already ready, no call.
  2. When main has a commit the pull request's head lacks, gh pr
     update-branch <item> --repo <repository> --rebase, after gh pr ready,
     and the head GitHub makes is found as slice 175 finds it.
  3. Without --no-wait and with ci.watch's provider, the run of the pull
     request's head (the head pushed, the branch's head when nothing was
     pushed, or the head GitHub made) is watched as slice 174 watches it,
     whether or not anything was pushed. One rule is new, for --ready alone:
     a run of that head all of whose jobs were skipped, or that lists no job,
     is passed over as if GitHub had none yet, and looked for again each
     ci.watch.interval within ci.watch.timeout. It is the draft's run: CI ran
     on the draft's last push with every job skipped (the workflow below),
     GitHub may record its conclusion as success, and the run marking it
     ready starts on the same head comes seconds later. A run that does not
     succeed exits as the watch does (1 for a failed run, 75 for one that
     does not end), outcome pushed or nothing-to-push, and stderr adds
     "itos: auto-merge is not turned on: the run did not succeed. Fix what
     failed and push, or wait for the run, then itos push --ready again."
     With no ci.watch provider no run is waited for, and GitHub's required
     checks judge the merge.
  4. The reviews, with push.pr.review_by not empty: a review on the pull
     request by each login, of any state and any commit, asked of GitHub at
     once and each ci.watch.interval until each login has one, the progress
     line "Waiting for reviews by <logins, comma-separated> on <address>."
     printed once where the watch's progress goes (stdout, stderr under
     --json, nowhere under -q). A login matches a review's author when the
     two are equal once a trailing "[bot]" is dropped from each, since
     GitHub's GraphQL names a bot without it (copilot-pull-request-reviewer)
     and its REST API with it (copilot-pull-request-reviewer[bot]). No review
     by each within ci.watch.timeout of the wait's start exits 75, outcome
     review-timeout, auto-merge not turned on, stderr (under --json too)
     "itos: no review by <the logins without one, comma-separated, as the
     config writes them> on <address> after <n> seconds (ci.watch.timeout),
     so auto-merge is not turned on; itos push --ready again waits again."
  5. The review threads, always under --ready: a thread whose isResolved is
     false refuses the merge, exit 1, outcome review-threads-unresolved,
     auto-merge not turned on, stderr (under --json too, written as it runs)
     "itos push: auto-merge is not turned on: <address> has unresolved review
     threads:", then for each, in GitHub's order, "  <path>:<line> <the first
     line of its first comment>" ("  <path> ..." when GitHub gives no line,
     an outdated thread), then "Address each (fix it and push, or reply on
     the pull request), resolve it, then itos push --ready again." Resolved
     threads are not listed.
  6. gh pr merge <item> --repo <repository> --auto --rebase, as slice 174
     calls it, and "Auto-merge is on: GitHub merges <address> by rebase once
     its required checks pass." Exit 0, outcome pushed or nothing-to-push.
  So under --ready the wait is a gate, not slice 174's report: auto-merge
  comes after the checks and the reviews, never before them.

  Steps 4 and 5 read one GitHub answer, asked through gh's GraphQL, since gh
  pr view has no review threads: gh api graphql -f query=<the query> -f
  owner=<owner> -f name=<name> -F number=<number> (-f keeps a numeric
  repository name a string; gh api takes no --repo, so the repository is
  the owner and name of the one every other gh call names), the query:
    query($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) { pullRequest(number: $number) {
    reviews(first: 100) { nodes { author { login } } }
    reviewThreads(first: 100) { nodes { isResolved path line
    comments(first: 1) { nodes { body } } } } } } }
  sent as one line, each run of whitespace one space, since the recording
  gh keeps a call to one line and the steps below read it so. Step 4 asks
  it until the reviews are there; step 5 judges the threads of
  its last answer, or of one asked then when review_by is empty. A pull
  request with more than 100 reviews or threads is out of scope. A gh api
  call that fails, or an answer without that shape, is any gh call that
  fails: exit 3, outcome pull-request-failed, naming gh api graphql. The
  number is gh pr list's, or the end of the address gh pr create printed.

  With --no-wait, --ready does steps 1 and 2 and stops: no run, no review,
  no gh api, no auto-merge, since turning auto-merge on unwatched would let
  GitHub merge before Copilot's review (the ruleset requires the threads
  resolved, not a review). It prints the pushed line, then "<address> is
  ready for review; itos push --ready again waits for its checks and
  reviews, then turns on auto-merge." Exit 0. So an agent marks the pull
  request ready, starting CI and Copilot's review, and comes back with
  itos push --ready, which with nothing to push still runs steps 1 to 6 on
  the pull request's head.

  Where --ready has nothing to do. Outside push.mode pr with push.pr.draft
  true (trunk, no config, a config that cannot be read, or pr without
  draft) it is refused once the config is read, before anything is fetched
  or pushed, exit 1, outcome ready-without-draft: "itos push: --ready is
  for push.mode pr with push.pr.draft true, which this config does not set;
  nothing was fetched or pushed." and "Push without --ready, or set
  push.mode: pr and push.pr.draft: true." On main, which has no pull
  request, it is refused once main is known, before anything is fetched or
  pushed, exit 1, outcome ready-on-main: "itos push: --ready marks an item's
  pull request ready, and HEAD is on <main>, which has none; nothing was
  fetched or pushed." and "Run itos push --ready from the item's branch."
  With --tag or --ref it is a usage error, exit 2: "push --ready pushes an
  item's branch; it takes no --tag or --ref". On a branch naming no item it
  is not-item-branch, as before. A branch whose commits are all the
  registry's sends them to main and opens nothing, as before, --ready or
  not; one that adds nothing to main is nothing-to-push, as before.

  --json: pull_request gains "draft", whether the pull request is a draft
  once the push is done, and "auto_merge", whether this push turned on
  auto-merge (true for every pr push that calls gh pr merge, under draft
  false as before); outcome gains review-timeout,
  review-threads-unresolved, ready-without-draft and ready-on-main. ci and
  run are the watched run's, as before.

  work done (slice-176) needs no change: a draft pull request is OPEN to gh
  pr list (isDraft is a field of its own), so it refuses the close as
  work-done-pr-open, as for any open pull request (prLanded in
  internal/slices/work/donepr.go); the guard below holds that.

  CI, a ci commit the implementer makes. .github/workflows/ci.yml's trigger
  becomes
    pull_request: { types: [opened, synchronize, reopened, ready_for_review] }
  (GitHub's three defaults and ready_for_review), push to main unchanged;
  the ci and platform jobs each gain the job-level
    if: ${{ github.event_name != 'pull_request' || !github.event.pull_request.draft }}
  and release is left as it is (it runs on a push to main alone, where
  neither job is skipped). A draft's run then skips both jobs: no runner
  time, no verdict. A required check skipped on a draft is harmless, since
  a draft cannot merge whatever its checks say; ci (ubuntu-latest) reports
  as skipped, and the matrix job reports one skipped check under its
  unexpanded name, leaving platform (macos-latest) and platform
  (windows-latest) expected. Marking the pull request ready fires
  ready_for_review, whose run judges the same head for real and reports
  every required check; the TO and BASE the steps read come from
  github.event.pull_request in that event too. The same ci commit changes
  tools/selftest/ci-platform/platform_test.go, whose scopedContract refuses
  a conditional ci or platform job ("missing or conditional %s job"), to
  accept exactly that if: and no other.

  Consumers. itos writes no workflow for anyone: itos init and itos upgrade
  generate none, and the README documents only the launcher action (uses:
  donvargax/itos@...). The templates carry their own ci.yml, whose jobs run
  on every pull request: itos-template's (ci, platform, release) and
  go-template-itos's (ci). A project made from either that turns on
  push.pr.draft needs the same trigger and if:, a backport for the
  coordinator to file on each template.

  The repository side, which the person applies, with no scenario: the main
  ruleset gains the copilot_code_review rule with review_draft_pull_requests
  false (Copilot reviews once the pull request is ready, never a draft;
  confirm on the first draft), and its pull_request rule gains
  required_review_thread_resolution true, so GitHub too refuses a merge with
  a thread open, whoever turns auto-merge on.

  The implementing commit rewords push-mode.feature's description where it
  says the wait is a report, not a gate, to point here for push.pr.draft;
  command.go's help and doc.go say --ready.

  Not in this slice: making a ready pull request a draft again (gh pr ready
  --undo); a plain push to a ready pull request whose auto-merge is already
  on, which GitHub merges once green; reviews requested of a login (gh pr
  edit --add-reviewer), rather than waited for; a review whose state is
  CHANGES_REQUESTED, which counts as a review, its threads judging it; work
  done waiting for a pull request to merge.

  The steps. The recording gh is push-mode.feature's, and learns four
  things. Its list of pull requests keeps whether each is a draft: pr create
  with --draft holds the pull request open as a draft, pr list answers each
  entry with "isDraft" (true or false) beside its number, url, state and
  mergedAt, and pr ready <branch> makes that branch's pull request ready (no
  longer a draft), answering nothing. It answers api graphql with
  {"data":{"repository":{"pullRequest":{"reviews":{"nodes":[...]},
  "reviewThreads":{"nodes":[...]}}}}}, the reviews and threads the steps
  gave it, none by default, every call alike. New steps: "the recording gh
  has the draft pull request <address> open for the branch <branch>", which
  holds it as "has the pull request ... open" does, as a draft; "the
  recording gh answers that the pull request has a review by <login>", a
  review node whose author's login is that; "the recording gh answers that
  the pull request has an unresolved (or a resolved) review thread on
  <path> line <n>, its first comment the lines <first> and <second>", a
  thread node with isResolved false (true), that path and line, and one
  comment whose body is the two lines joined by a newline; "the recording
  gh's <command> call does not have <text>", which fails when no call of
  the command has been made, as well as when one has the text. The fake
  GitHub is ci.watch's (watch.feature), and learns one: "the watched run's
  first look has its jobs <a> and <b> skipped, as a draft's run has, and
  its later looks its job <a> failing and its job <b> succeeding", whose
  first look is a completed run, conclusion success, both jobs completed
  and skipped, and every later look the run finished with a failed and b
  succeeded.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The keys are push.pr's, read only under pr. Fails before the work at the
  # exit code: today config check refuses the unknown key push.pr, exit 2.
  @ID-PUSHDRAFT-01 @slice-179 @wip
  Scenario: config check accepts push.pr's draft and review_by
    Given the config sets "push.pr" to the YAML "{draft: true, review_by: ['copilot-pull-request-reviewer[bot]']}"
    When itos checks the config
    Then itos exits with code 0

  # Off by default, so pr pushes as slices 174 and 175 made it. Fails before
  # the work at the exit code: today config get refuses the unknown key, exit 2.
  @ID-PUSHDRAFT-02 @slice-179 @wip
  Scenario: config get prints push.pr.draft's default, false, when the config leaves it out
    When itos runs "config get push.pr.draft"
    Then itos exits with code 0
    And its output says "false"

  # The slice's point: a push as the agent works opens a draft, which nothing
  # judges or merges, so no auto-merge and no run, ci.watch or not. Fails
  # before the work at the pr create step: today push.pr is an unknown key,
  # so the config cannot be read and the push is a trunk push to main that
  # calls no gh.
  @ID-PUSHDRAFT-03 @slice-179 @wip
  Scenario: Under push.pr.draft, the first push from an item's branch opens its pull request as a draft, turning on no auto-merge and waiting for no run
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr create" call has "--draft"
    And the remote has the branch "slice-9" with "feat: add the widget"
    And the recording gh was never called as "pr merge"
    And the fake GitHub was never asked about a run
    And its output says "https://github.example/o/r/pull/1"
    And its output says "The pull request is a draft"

  # A later push adds to the open draft and asks nothing more of GitHub, not
  # even a rebase when main has moved: --ready asks for that. Fails before
  # the work at the pr list step: today the config cannot be read and gh is
  # never called.
  @ID-PUSHDRAFT-04 @slice-179 @wip
  Scenario: Under push.pr.draft, a push to an open draft that main has moved past opens nothing, turns on no auto-merge and asks for no rebase
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
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push"
    Then itos exits with code 0
    And the recording gh's "pr list" call has "isDraft"
    And the remote has the branch "slice-9" with "test: cover the widget"
    And the recording gh was never called as "pr create"
    And the recording gh was never called as "pr merge"
    And the recording gh was never called as "pr update-branch"
    And its output says "https://github.example/o/r/pull/7"

  # --json is the contract: an agent reads that the pull request is a draft
  # and that nothing turned on auto-merge or watched a run. Fails before the
  # work at the pull_request.draft step: today the push is a trunk push,
  # whose JSON has no pull_request.
  @ID-PUSHDRAFT-05 @slice-179 @wip
  Scenario: Under push.pr.draft, push --json says the pull request is a draft and auto-merge is off
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --json"
    Then itos exits with code 0
    And its JSON's "pull_request.draft" is true
    And its JSON's "pull_request.auto_merge" is false
    And its JSON's "pull_request.url" is "https://github.example/o/r/pull/1"
    And its JSON's "outcome" is "pushed"
    And its JSON has no "ci"

  # The slice's other point: --ready marks the draft ready, waits for the
  # run and for Copilot's review, finds no thread open, and only then turns
  # on auto-merge. The review's author is GraphQL's login, without [bot],
  # which matches the config's login with it. Fails before the work at the
  # exit code: today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-06 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready marks the draft ready, waits for its run and the review, then turns on auto-merge by rebase
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr ready" call has "slice-9"
    And the recording gh's "pr ready" call has "--repo owner/scratch"
    And the fake GitHub was asked for the run of the clone's HEAD
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "pr merge" call has "--auto"
    And the recording gh's "pr merge" call has "--rebase"
    And its output says "Auto-merge is on"

  # Under --ready the wait is a gate: a red run leaves auto-merge off. With
  # nothing to push, --ready still waits for the run of the pull request's
  # head, so a second --ready after a fix pushed plainly judges it. Fails
  # before the work at the exit code: today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-07 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready with nothing to push waits for the head's run and, when it fails, turns on no auto-merge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready"
    Then itos exits with code 1
    And the recording gh's "pr ready" call has "slice-9"
    And the fake GitHub was asked for the run of the clone's HEAD
    And its output says "ci: failure"
    And its output says "auto-merge is not turned on"
    And the recording gh was never called as "pr merge"

  # The draft's last push ran CI with every job skipped, and GitHub may call
  # that run a success; the run marking it ready starts on the same head a
  # moment later. --ready passes over the skipped run and judges the real
  # one, red here. Fails before the work at the exit code: today push
  # refuses --ready, exit 2; without the rule it would take the skipped run
  # and exit 0.
  @ID-PUSHDRAFT-08 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready passes over the draft's run whose jobs were all skipped and waits for the real one
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's first look has its jobs "ci" and "platform" skipped, as a draft's run has, and its later looks its job "ci" failing and its job "platform" succeeding
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready"
    Then itos exits with code 1
    And its output says "ci: failure"
    And the recording gh was never called as "pr merge"

  # The merge waits until Copilot's findings are addressed: an unresolved
  # thread leaves auto-merge off and names where it is, by the first line of
  # its first comment; a resolved one is not named. Said on stderr under
  # --json too. Fails before the work at the exit code: today push refuses
  # --ready, exit 2.
  @ID-PUSHDRAFT-09 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready with an unresolved review thread turns on no auto-merge and names the thread, exit 1
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer"
    And the recording gh answers that the pull request has an unresolved review thread on "widget.md" line 3, its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the pull request has a resolved review thread on "gadget.md" line 1, its first comment the lines "Fine now." and "Thanks."
    When itos runs "push --ready --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "review-threads-unresolved"
    And its JSON's "pull_request.auto_merge" is false
    And its output says "widget.md:3 Rename this."
    And its output does not say "It clashes with the gadget."
    And its output does not say "gadget.md"
    And its output says "itos push --ready again"
    And the recording gh was never called as "pr merge"

  # The review is waited for no longer than ci.watch.timeout: Copilot never
  # reviewing (the ruleset not applied, an outage) exits 75, auto-merge off,
  # naming the login as the config writes it. Fails before the work at the
  # exit code: today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-10 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready with no review by a review_by login within ci.watch.timeout exits 75, turning on no auto-merge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its JSON's "ci" is "success"
    And its output says "no review by copilot-pull-request-reviewer[bot]"
    And the recording gh was never called as "pr merge"

  # --ready --no-wait marks it ready, starting CI and Copilot's review, and
  # stops: auto-merge unwatched would let GitHub merge before the review.
  # Fails before the work at the exit code: today push refuses --ready,
  # exit 2.
  @ID-PUSHDRAFT-11 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready --no-wait marks the draft ready and turns on no auto-merge, saying how to finish
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone has the commit "test: cover the widget" touching "widget-test.md"
    When itos runs "push --ready --no-wait"
    Then itos exits with code 0
    And the recording gh's "pr ready" call has "slice-9"
    And the remote has the branch "slice-9" with "test: cover the widget"
    And the recording gh was never called as "pr merge"
    And the recording gh was never called as "api graphql"
    And the fake GitHub was never asked about a run
    And its output says "itos push --ready again"

  # With no pull request yet, --ready opens it ready, so there is nothing to
  # mark; without ci.watch no run is waited for, the threads are still asked
  # (none), and auto-merge goes on. Fails before the work at the exit code:
  # today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-12 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready from an item's branch with no pull request opens it ready and turns on auto-merge
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
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr create" call does not have "--draft"
    And the recording gh was never called as "pr ready"
    And the recording gh's "api graphql" call has "number=1"
    And the recording gh's "pr merge" call has "--auto"

  # Main has no pull request to mark; the refusal comes before anything is
  # fetched or pushed, so the take stays local. Fails before the work at the
  # exit code: today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-13 @slice-179 @wip
  Scenario: Under push.pr.draft, push --ready from main is refused, exit 1, pushing nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: take slice-9" making "bo" the owner of "slice-9"
    When itos runs "push --ready --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "ready-on-main"
    And the remote's branch does not have "docs: take slice-9"
    And the recording gh was never called

  # Under pr without draft every pull request opens ready and auto-merge goes
  # on at each push, so --ready has nothing to mark and would only add a gate
  # the config did not ask for. Fails before the work at the exit code: today
  # push refuses --ready as a usage error, exit 2.
  @ID-PUSHDRAFT-14 @slice-179 @wip
  Scenario: Under pr without push.pr.draft, push --ready is refused, exit 1, before anything is pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --ready --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "ready-without-draft"
    And the remote has no branch "slice-9"
    And the recording gh was never called

  # push.pr.draft is off by default: pr opens pull requests ready and turns
  # on auto-merge at each push, as slices 174 and 175 made it. A guard: it
  # passes before the fix and must after it.
  @ID-PUSHDRAFT-15 @slice-179 @wip @guard
  Scenario: Under pr without push.pr.draft, a push opens its pull request ready and turns on auto-merge, as before
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
    And the recording gh's "pr create" call does not have "--draft"
    And the recording gh was never called as "pr ready"
    And the recording gh's "pr merge" call has "--auto"

  # A draft is open to gh pr list (state OPEN, isDraft true), so work done
  # (slice-176) refuses the close as work-done-pr-open with no change. The
  # draft is the recording gh's, not the config's: work done reads no
  # push.pr, and a config naming it cannot be read before the work. A guard:
  # it passes before the fix and must after it.
  @ID-PUSHDRAFT-16 @slice-179 @wip @guard
  Scenario: Under pr, work done refuses an item whose pull request is an open draft
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the remote's branch "slice-9" has the item "slice-9"'s commit touching "widget.md", which the clone's main lacks
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://github.example/o/r/pull/7 is open"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"
