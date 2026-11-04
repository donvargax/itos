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
  auto-merge, as GitHub's GraphQL gives them (a bot's without "[bot]");
  empty, it waits for none." Both are read only under push.mode
  pr. This repository turns them on (push.pr: {draft: true, review_by:
  [copilot-pull-request-reviewer]}) in a build commit of its own,
  once the ruleset below is applied, and rewrites AGENTS.md's Finishing for
  the --ready flow in the same push. Slice 187's build commit changes its
  review_by to [chatgpt-codex-connector] (Codex), Copilot's quota having run
  out.

  A plain itos push from an item's branch, under push.pr.draft true, does
  what slice 174 does up to the pull request (the rebase, the registry
  commits to main, the atomic push) and then: gh pr list --repo <repository>
  --head <item> --state open --json number,url,isDraft,title (isDraft
  and title are asked under either setting of draft); with none open, gh pr create --repo
  <repository> --base <main> --head <item> and the title and the description
  of push-mode-title.feature (slice 184), then --draft. It turns on no
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
  4. The reviews, with push.pr.review_by not empty: a verdict on the pull
     request by each login, for any commit, asked of GitHub at once and each
     ci.watch.interval until each login has one, the progress line "Waiting
     for reviews by <logins, comma-separated> on <address>." printed once
     where the watch's progress goes (stdout, stderr under --json, nowhere
     under -q). A login matches an author when the two are equal once a
     trailing "[bot]" is dropped from each, since GitHub's GraphQL names a
     bot without it (copilot-pull-request-reviewer) and its REST API with it
     (copilot-pull-request-reviewer[bot]). A verdict (slice 187) is either:
     a. A review by the login, of any state, that is not a refusal. A
        refusal is a review whose body's first line begins "Copilot" and has
        "was unable to review this pull request": Copilot's quota running
        out ("Copilot was unable to review this pull request because the
        user who requested the review has insufficient remaining quota.",
        which counted as the review on #158 and merged it unreviewed) or an
        error of its own ("Copilot encountered an error and was unable to
        review this pull request. You can try again by re-requesting a
        review."). Every real Copilot review here begins with the line
        "<!-- ccr-overview-v2 -->", but the rule is the refusal's words, not
        that marker, so a review by any other login (a person's) counts as
        it always did. Only for a login that wrote no state comment (b).
     b. A comment on the pull request (an issue comment, not a review) by
        the login with a line "<!-- codex-review-state:v1 <JSON> -->" whose
        JSON's code_review.status and security_review.status are both
        "completed"; the line is matched with the whitespace around it
        trimmed, so a trailing carriage return does not hide it. On a clean
        pass Codex (chatgpt-codex-connector) posts no review, only this
        comment, which it edits in place (status "running", then
        "completed"), and a +1 reaction (#159); its findings come as a
        review with threads, which step 5 judges. While either status is
        anything but "completed" there is no verdict and the wait goes on.
        The state's other fields (commit_sha, has_any_findings,
        has_unresolved_findings) are not read (slice 200 reads commit_sha,
        below), and a state comment by another login (one quoting Codex's)
        is not Codex's verdict.
     Which decides: when a review_by login wrote a comment with that state
     line, the state alone decides its verdict, and a review by it counts
     under (a) only when it wrote none. Codex posts its code review's
     findings as a review while its security review may still run, and that
     review counting at once would let auto-merge go on before the security
     findings land.
     Nothing is compared with the pull request's head (but a review-state
     comment's commit_sha since slice 200, below): a verdict on an
     earlier commit counts, as Copilot's reviews always did, with no
     re-review after a fix is pushed; resolved threads stay the gate on
     fixes. itos asks no reviewer for a review (no "@codex review" comment),
     but for slice 190's re-request of a review stalled, below.
     A login with no verdict and a refusal stops the wait at once, since
     waiting cannot bring a review nobody asks for again: exit 75, outcome
     review-refused, auto-merge not turned on, stderr (under --json too),
     for each such login in the config's order, "itos: <login, as the
     config writes it> gave no verdict on <address>: "<the first line of
     its last refusal>"; auto-merge is not turned on: request its review
     again, then itos push --ready again." A login with a verdict and a
     refusal (reviewed once requested again) counts: a refusal stays on the
     pull request. No verdict by each within ci.watch.timeout of the wait's
     start exits 75, outcome review-timeout, auto-merge not turned on,
     stderr (under --json too) "itos: no review by <the logins without
     one, comma-separated, as the config writes them> on <address> after
     <n> seconds (ci.watch.timeout), so auto-merge is not turned on; itos
     push --ready again waits again." (bug-164 changes what it says of a rerun.)
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
     (Slice 200: GitHub refusing auto-merge on a clean pull request, --ready
     merges it itself, below.)
  So under --ready the wait is a gate, not slice 174's report: auto-merge
  comes after the checks and the reviews, never before them.

  Steps 4 and 5 read one GitHub answer, asked through gh's GraphQL, since gh
  pr view has no review threads: gh api graphql -f query=<the query> -f
  owner=<owner> -f name=<name> -F number=<number> (-f keeps a numeric
  repository name a string; gh api takes no --repo, so the repository is
  the owner and name of the one every other gh call names), the query:
    query($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) { pullRequest(number: $number) {
    reviews(first: 100) { nodes { author { login } body } }
    comments(first: 100) { nodes { author { login } body } }
    reviewThreads(first: 100) { nodes { isResolved path line
    comments(first: 1) { nodes { body } } } } } } }
  (slice 187 added the reviews' body and the pull request's comments, for
  step 4's verdicts) sent as one line, each run of whitespace one space,
  since the recording gh keeps a call to one line and the steps below read
  it so. Step 4 asks it until the verdicts are there; step 5 judges the
  threads of its last answer, or of one asked then when review_by is empty.
  A pull request with more than 100 reviews, comments or threads is out of
  scope. A gh api
  call that fails, or an answer without that shape, is any gh call that
  fails: exit 3, outcome pull-request-failed, naming gh api graphql. The one
  exception is the pull request's comments: an answer without them is read as
  none, because v8.22.2's recorded answers carry none (and live GitHub always
  answers what the query asks). The number is gh pr list's, or the end of the address gh pr create printed.

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
  review-threads-unresolved, ready-without-draft and ready-on-main, and
  slice 187's review-refused. ci and
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
  confirm on the first draft; Codex, too, reviews on "Draft marked ready"),
  and its pull_request rule gains
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
  done waiting for a pull request to merge. Not in slice 187: a Codex
  state without security_review (a repository with Codex's security review
  off), which is no verdict until a scenario says otherwise; a notice of
  Codex's own that it cannot review, not seen yet, which waits as no
  verdict does; asking a reviewer for a review again.

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

  Slice 187's steps. The recording gh's api graphql answer gains
  "comments":{"nodes":[...]} beside reviews and reviewThreads, the comments
  the steps gave it, none by default, and each review node a "body", ""
  unless a step gives one. New steps: "the recording gh answers that the
  pull request has a review by <login> whose body is <body>", a review node
  with that author's login and body; "the recording gh answers that the pull
  request has a comment by <login> holding Codex's review state, its code
  review <status> and its security review <status>", a comment node with
  that author's login and the body of Codex's comment as #159 has it, its
  first lines "<!-- codex-pull-request-review-summary -->" and then
  "<!-- codex-review-state:v1 {"code_review":{"commit_sha":<any 40 hex
  digits; the clone's HEAD since slice 200>,"has_any_findings":false,"has_unresolved_findings":false,
  "status":<status>},"security_review":{the same, its status the second}}
  -->" (no space inside the JSON), then "## Codex Review Summary"; and the
  same step ending "for its first <n> api graphql calls, then both
  <status>", whose comment holds the two statuses given in the answers to
  the first n calls and that one status for both in every later answer.

  Slice 190: what --ready waits on, and a review that stalls. Seen
  2026-10-27 on #193: Codex's code review read running for 30 minutes while
  its security review completed in minutes, and every --ready call ran to
  the caller's 600000 ms cap with no line saying what it waited on, so the
  agent could not tell a stall from a slow run. Codex stalled on 3 of about
  16 reviews; the recovery ADR-0061 documents, one new "@codex review"
  comment after 30 minutes, was the coordinator's to do by hand.

  The waiting lines. While --ready waits (on a run, under ci.watch's
  provider, or on the reviews of a push.pr.review_by not empty) it prints,
  where the watch's progress goes (stdout, stderr under --json, nowhere
  under -q), the line "Waiting on <address> at <the head's first 7
  characters>: <part>, <part>." The head is the one whose run step 3
  watches. The parts, in order: "run <state>" when a run is watched, its
  state waiting until the run succeeded, then succeeded (a run that does
  not succeed ends the wait as before, the watch's own lines saying so);
  then, for each review_by login in the config's order and as the config
  writes it, "<login> <state>", its state not asked until step 4's first
  ask (the run is watched first), then waiting (neither a verdict nor a
  review-state comment), running (its review-state comment with a review
  not completed) or completed (its verdict). The line is printed once when
  the wait starts (before the run is watched, or before the first ask when
  no run is), and again after each look that changes a part; a look that
  changes nothing prints nothing, so there is no heartbeat. With review_by,
  the run's success and the first ask are one look: no line says "run
  succeeded" beside a login "not asked". The states are read as step 4
  reads verdicts: nothing is compared with the head, and the state's
  commit_sha is still not read (slice 200 reads it). Slice 179's "Waiting
  for reviews by ..."
  stays as it is.

  The re-request. push.pr.review_stall is a number of seconds, as
  ci.watch.timeout is, 0 by default, 0 or left out meaning off; it is read
  under push.mode pr alone, as review_by is. config check refuses one that
  is not a number, as it refuses any value of the wrong type; config get
  push.pr.review_stall prints 0 when the config leaves it out; the schema
  lists it with the words "The seconds a push.pr.review_by login's review
  may read running in its review-state comment (Codex's) before itos push
  --ready asks for it again, once per head; 0, the default, never." When a
  review_by login's review-state comment (step 4b) has read running for
  review_stall seconds, counted from the first look of this --ready that
  read it so, --ready asks for that review again: gh pr comment <item>
  --repo <repository> --body=<body>, the body the line "@codex review", an
  empty line, and the line "<!-- itos-review-request:v1 <login, as the
  config writes it> <the head's full SHA> -->"; and it says, where its
  progress goes, "Asked <login> to review <the head's first 7 characters>
  again on <address>: its review read running for <n> seconds
  (push.pr.review_stall)." Only a login that writes a review-state comment
  is asked again, and always with "@codex review": the state is Codex's, a
  running review is the only stall it shows, and no other reviewer's way to
  be asked again follows from it (Copilot's is a re-request through
  GitHub's reviewers). A login with no state comment waits as before, up to
  ci.watch.timeout.

  Once per head, ever: a comment on the pull request, by anyone, with the
  line "<!-- itos-review-request:v1 <login> <sha> -->" (the line trimmed,
  the login matched as review_by's are) for that login and the head's full
  SHA means it was asked already, by this --ready or an earlier one, and
  nothing is posted again. One naming another head counts for nothing. bug-164
  narrows this: a request older than the running review's start is not asking again.

  The stall. When the login's review still reads running review_stall
  seconds after it was asked again (counted from the re-request this
  --ready posted or, when an earlier one posted it, from this --ready's
  first look that read it running), --ready stops: exit 75, outcome
  review-stalled, auto-merge not turned on, stderr (under --json too)
  "itos: the review by <login> of <the head's first 7 characters> on
  <address> stalled: still running <n> seconds (push.pr.review_stall)
  after it was asked again, so auto-merge is not turned on; look at the
  reviewer, then itos push --ready again." 75, as review-refused has it:
  ADR-0036 keeps itos's exit codes to 0, 1, 2, 3, 70 and 75, a stall may
  pass when run again, and the outcome is what tells it from
  review-timeout. ci.watch.timeout still bounds the whole wait, so with
  review_stall at or past it the timeout comes first. --json: outcome gains
  review-stalled.

  Not in slice 190: a reviewer other than Codex asked again; a review that
  reads waiting (no state comment) for long, which ci.watch.timeout ends as
  before; comparing a state's commit_sha with the head.

  Slice 190's steps. The recording gh's pr comment is recorded and answers
  nothing, as any other call. New steps: "the recording gh answers that
  the pull request has a comment by <author> asking <login> to review the
  clone's HEAD again", a comment node with that author's login and the
  body "@codex review", an empty line, then "<!-- itos-review-request:v1
  <login> <the full SHA of the clone's HEAD when the step runs> -->"; the
  same step ending "to review the commit <sha> again", with that SHA
  instead; "the recording gh answers that the pull request has a comment by
  <login> holding Codex's review state, its code review <status> and its
  security review <status> until a pr comment call, then both <status>",
  whose comment holds the two statuses in the answer to every api graphql
  call before the recording gh's first pr comment call and the one status
  for both in every answer after it, the other comments the steps gave it
  in both; "the recording gh's <command> call has the full SHA of the
  clone's HEAD"; and "the recording gh was called as <command> once",
  which fails when it was never called so, or more than once.

  Slice 218: a review that never starts. Codex sometimes answers a request
  and never starts the review: its review-state comment keeps that review's
  status and commit null. On #247 (bug-157) it did so on every head, each
  security request got a code review's receipt, and --ready waited out
  ci.watch.timeout twice, saying only that the login was waiting. A second
  request has repeatedly started the review within minutes, on two
  repositories. This slice builds on slice 217 (the two reviews, each asked
  for by its own line, the stall counting the code review's request alone).

  bug-163 settled why, on #260 and #263 (features/review-request.feature): each
  of those security requests carried the marker, and Codex runs that as a code
  review. Codex's security request is now the line alone, then the marker alone,
  the marker's comment being the request all of slice 217's and this slice's
  rules count: once per head, the two requests, the clock, the address --ready
  prints and not_started.request gives. So a request by hand of the security
  review is both comments. The old form on a pull request counts for nothing,
  never reaching the two requests, so an upgrade leaves no pull request stopped
  at review-not-started on what never started anything.

  Not started. With push.pr.review_stall set, each of the two reviews in a
  review_by login's latest review-state comment is judged alone, the code
  review first, at each look, after that look's ci.watch.timeout and
  push.pr.ready_wait as the stall is. A review has not started, for the head,
  when a request for that head and that review is there (a comment reading as
  that review's, as slice 217 and bug-163 read one, or one this --ready posted),
  and the state gives that review's status as null, or no commit, or another
  commit, whatever its status, review_stall seconds after the latest such
  request. So the code review having completed does not count for the security
  review. A status other than null and completed for the head is a review
  under way: slice 190's stall, outcome review-stalled, as before. A review
  not started reads waiting in the waiting line. A login with no review-state
  comment is judged by nothing here, and waits up to ci.watch.timeout as
  before. Without push.pr.review_stall none of this runs.

  The second request. A review not started with one request for it is asked
  once more: gh pr comment with that review's request as slice 217 posts it,
  and --ready says, where its progress goes, "Asked <login> again, at <the
  address gh pr comment printed>, for its <code or security> review of <the
  head's first 7 characters>: it had not started <n> seconds
  (push.pr.review_stall) after the request." The clock then counts from it.

  The stop. A review not started with two requests for it ends the wait: exit
  75, outcome review-not-started, auto-merge not turned on, and stderr (under
  --json too) "itos: the <code or security> review by <login> of <the head's
  full SHA> on <address> has not started <n> seconds (push.pr.review_stall)
  after the request <the latest request comment's address>, so auto-merge is
  not turned on; look at the reviewer, then itos push --ready again." 75, as
  review-stalled has it: running again may pass, and the outcome tells it from
  review-timeout, which still comes first when ci.watch.timeout does. The full
  SHA, as ready-still-waiting's line gives it, since itos ci watch takes it.
  --json: outcome gains review-not-started, and a key of its own holds what no
  other outcome's JSON gives, since the others say it on stderr alone:
  "not_started" {"login","review","head","request"}, login as the config writes
  it, review "code" or "security", head the full SHA, request that address.

  Two requests for a review and a head, ever, across all --ready runs: that is
  the once-per-run rule. A --ready posts the second only when one request is
  there; finding two it posts nothing, and waits out the clock from the later
  one, or stops at its first look when that has run. Counted from the pull
  request and not per run, since itos next reads a pull request stopped here as
  handed-back, whose action is itos push --ready: a driver rerunning it would
  otherwise post at each review_stall until ci.watch.timeout. A third try is a
  new head, which starts afresh (a request counts for its own head alone), or
  a request by hand with the marker (the security review's, the line alone then
  the marker alone), which counts and restarts the clock.
  A request this --ready posted, once GitHub lists it, counts once, known by its
  address (what gh pr comment printed, the listed comment's url): adding the
  listed requests to the posted ones would read a first request as two, and
  stop with no second.

  The clock, kept by GitHub as slice 200's: from the latest request, by the
  comment's createdAt, a request this --ready posted counting from when it
  posted it until GitHub lists it, and a time GitHub does not give counting
  from the first look of this --ready that read the review not started. The
  request comments' addresses: PullStateQuery asks each comment's url, and
  gh pr comment prints the address of the one it made, which postRequest
  keeps. The --ready help and internal/slices/push/doc.go follow this rule.

  The schema's words for review_stall must say it has to exceed the reviewer's
  queue plus its review: a slow healthy reviewer still on the old commit reads
  as not started, at a cost of one extra request and a stop at twice
  review_stall.

  Two unit tests the implementer adds beside slice 190's in
  internal/slices/push/waiting_test.go. TestPushReadyNotStartedPastTheTimeout:
  at a look where ci.watch.timeout has passed and two requests are past
  review_stall, review-timeout comes first. TestPushReadyWhereTheSecondRequestFails:
  gh pr comment failing on the second request is any gh call failing, exit 3,
  pull-request-failed, nothing merged.

  Not in slice 218: a login's reviews starting with no request at all (slice
  217 leaves a fresh pull request unasked); a reviewer other than Codex; a
  state of itos next's own for this stop.

  Slice 218's steps. The steps giving a review a status of a commit (slice 200's
  and slice 217's "until a pr comment call, then both <status> of the clone's
  HEAD") take also null, unquoted, for a status, and no commit for a commit,
  each written into the state as JSON's null. New: the same state step ending
  "for its first <n> api graphql calls, then both <status> of the clone's
  HEAD", into the files slice 187's fills (gh-comments-early, gh-comments-left
  and gh-comments), which first copies what gh-comments holds so far into
  gh-comments-early, since nodes/21-comments-early.sh answers that file in place
  of gh-comments and the comments the steps before it gave must stay listed;
  slice 215's "requesting <line> of <login> at the clone's
  HEAD" ending ", posted <n> seconds ago, at <address>" (its url, createdAt and
  updatedAt); "the recording gh's pr
  comment prints <address> and the call's number", after which pr-comment.sh
  prints the address followed by how many pr comment calls there have been,
  the first being 1, and without which it prints nothing, as before; and "the
  recording gh lists each pr comment it takes", after which pr-comment.sh also
  adds to gh-comments a node by "someone" with the call's body, what it printed
  as its url and the call's time as its createdAt and updatedAt, as GitHub lists
  a comment made.

  bug-164: a stall counts from the review's start. Seen 2026-11-04 on slice-218's
  own run: at --ready's second request Codex's state flipped to code review
  running with its security review null, the stall counted from the code review's
  request, 20 minutes old, and --ready stopped review-stalled seconds after the
  review started, on every rerun until it completed. No wrong merge, but it
  misleads and stops a healthy review.

  The clock. stalled reads the review-state comment's last edit (slice 200) only
  while no code request is on the head; a request found replaces it, however old.
  Now a review under way started, as far as itos can tell, at that last edit, and
  a request replaces it only when made at or after it (the same second counting
  as after, GitHub's times being whole seconds). A request older than the start
  asked for the review now running, and did not ask for it again. So the clock is
  the later of the two, and a review that started seconds ago is not stalled,
  however old its request. An edit is any change to the comment, the other
  review's completion among them, so the start can read late, which gives a stuck
  review longer, and never early. With no last edit, or a request with no time,
  the clock is what it was: the request's time, else the first look of this
  --ready that read the review running. A rerun keeps nothing from the run before
  and reads the same times, so it does not stop a review started since.

  Stalled means what it meant: running review_stall seconds past that clock, asked
  again once, then review-stalled, as the schema's words for review_stall have it.
  Past the clock with the one request older than the start, --ready posts slice
  190's "@codex review" and says so, and stops review_stall after that request.
  With two requests for the head it posts nothing and stops: two requests for a
  review and a head, ever, is slice 218's and stays, so slice 190's once per head
  now reads: a request made since the review started, or a second. Bug-163's form
  for the security request is unchanged, and so is notStarted.

  The timeout's words. ci.watch.timeout counts from when the pull request was
  marked ready or its head pushed (slice 200), and is judged first, so a --ready
  past it looks once and returns: it does not wait. Its line now says so: "itos: no
  review by <logins> on <address> after <n> seconds (ci.watch.timeout), so
  auto-merge is not turned on; itos push --ready again turns it on if the reviews
  have come, else stops here at once, the wait counting from when the pull request
  was marked ready or its head pushed; a new head starts it afresh." Exit 75 and
  outcome review-timeout stay. Where GitHub gives neither time a rerun's clock
  starts at its first ask and it does wait; the line does not carve that out, as a
  pull request --ready marked ready has the ready event.

  The config. With ci.watch.timeout at its default 1800 judged first, this
  repository's review_stall of 1200 gives the second request but never reaches
  review-not-started at twice that. A build commit on this item sets
  ci.watch.timeout to 3000 beside push.pr.review_stall: 1200. Its other readers:
  ciwatch.Watch (itos ci watch, a plain itos push, itos work done), which
  push.pr.ready_wait does not bound, so a run that never ends holds them 20 minutes
  longer, while a run that ends is judged as before; ciwatch.WatchReady and
  rebasedHead (--ready), each bounded by ready_wait, so a run that never ends takes
  more reruns to reach its timeout, and this repository's push.pr.update merge asks
  for no rebase.

  Not in bug-164: a config check refusing a timeout under twice review_stall; a
  change to the order of the judges, ci.watch.timeout staying first; a new outcome.

  The rest is unit rows beside slice 200's, in
  TestPushReadyCountsTheStallFromGitHubsTimes: its row "asked long ago" (last edited
  30 seconds ago, the request 120, review_stall 60) now asks again at the fourth
  look and stops at the tenth, and new rows give two requests older than the last
  edit (no third, stopped when the edit is past review_stall), a request in the same
  second as the edit (the request's clock, no post), and an edit with a request with
  no time (the first look's clock). The row "asked from the last edit" keeps a build
  from counting from the edit alone, which would stop at the look after its request.
  The waits-again words in TestPushReady's review-timeout case (ready_test.go) follow
  the new line.

  ID-PUSHDRAFT-41, as released, is this bug: a request 120 seconds old beside a last
  edit 30 seconds old stops at the first look. Its rule stays, a rerun past the clock
  stops at the stall, so the fix changes its state's line to "last edited 130 seconds
  ago", the request made after the edit, and the fix's Changes: footer names it.

  bug-164's step: slice 218's state step for its first <n> api graphql calls
  with ", last edited <m> seconds ago" before ", for its first", the state of those
  calls being that long edited (createdAt and updatedAt) as of when the step ran, and
  the later state having no times.

  Slice 200: --ready finishes on its own. Seen 2026-10-25 on #146, #149
  and #156: GitHub refuses auto-merge on a pull request that is mergeable
  already ("Pull request is in clean status (enablePullRequestAutoMerge)"),
  the very state --ready waits for, and with strict required checks off a
  rerun stays clean, so the merge was made by hand with gh pr merge
  --rebase --match-head-commit, which an agent cannot run (Claude Code
  refuses it as a merge without review; inside --ready it is the gated
  path). Seen 2026-10-27 on slices 191, 195, 196 and 197: every --ready
  outlived the agents' 600000 ms foreground cap, and on #213 the fallback,
  itos ci watch on the pushed head, read a run cancelled, GitHub having
  rebased the pull request. Noted drafting slice 190, and by Codex on #210:
  a completed review of an earlier commit lets auto-merge go on after a
  push no reviewer read, and with no ci.watch provider the re-request's
  marker names the head from before GitHub's rebase.

  The merge, pinned and read back. gh pr merge --auto does not always turn
  on auto-merge: when gh reads the pull request's merge state as CLEAN,
  HAS_HOOKS or UNSTABLE it merges it at once (cli/cli's
  pkg/cmd/pr/merge/merge.go, isImmediatelyMergeable), saying nothing off a
  terminal, so --ready's step 6 can merge the head unpinned. So every merge
  call --ready makes carries --match-head-commit <the judged head's full
  SHA>, step 6's included: gh pr merge <item> --repo <repository> --auto
  --rebase --match-head-commit <sha>. The judged head is the one whose run
  step 3 watched (the head GitHub made when it rebased); with no ci.watch
  provider, the head it waited on (read as below when it asked for a
  rebase). It never merges a head it did not judge: GitHub refuses the
  merge when a push came in since, which is any gh call that fails (exit
  3, pull-request-failed). After the merge call that succeeded, --ready
  reads the pull request once, gh pr view <item> --repo <repository>
  --json state,mergedAt,autoMergeRequest, and says what happened: merged
  (state MERGED or a mergedAt), "Merged <address> by rebase at <the head's
  first 7 characters>: its checks and reviews had passed, and GitHub
  merged it at once." with --json's pull_request "merged" true (a new
  field) and auto_merge false; auto-merge on (an autoMergeRequest), "Auto-merge is on: ..." as
  before, merged false and auto_merge true; neither, pull-request-failed,
  exit 3, naming gh pr view: "the pull request is neither merged nor set
  to merge". An answer without state reads as the call asked (auto-merge
  on after --auto, merged after a direct merge), since v8.31.0's recorded
  pr view answers carry only headRefOid (as slice 187 reads an answer
  without comments). Exit 0 either way, outcome pushed or nothing-to-push.

  The clean-status refusal is a race: gh read a state that let it ask for
  auto-merge, and GitHub, by then, found the pull request clean. When
  step 6's call fails with words that have "is in clean status" (gh's whole
  line is "GraphQL: Pull request Pull request is in clean status
  (enablePullRequestAutoMerge)"), --ready has met every condition (the
  run, the reviews, the threads), so it merges once itself: gh pr merge
  <item> --repo <repository> --rebase --match-head-commit <sha>, then reads
  it back as above. If that refuses too, pull-request-failed, exit 3, as
  any gh call that fails. Any other failure of the --auto call is
  pull-request-failed, exit 3, as before, and nothing more is merged or
  asked.

  A refusal after a merge. An auto-merge an earlier --ready turned on may
  merge the pull request just before this --ready's own merge call, which
  GitHub then refuses ("GraphQL: Pull Request is not mergeable", seen on
  #209). So when a merge call fails, whatever its words, --ready reads the
  pull request once, gh pr view <item> --repo <repository> --json
  state,mergedAt,headRefOid, before it reports the failure. If it is merged
  (state MERGED or a mergedAt) and its headRefOid is the head --ready
  judged, the work is done: "Merged <address> by rebase at <the head's first
  7 characters>: GitHub had merged it already.", --json's pull_request
  "merged" true and auto_merge false, exit 0, outcome pushed or
  nothing-to-push. A pull request that is not merged, or is merged at
  another head, or a gh pr view that fails or answers what cannot be read,
  leaves the failure as it was, pull-request-failed, exit 3, with the merge
  call's words. The clean-status refusal's one direct merge comes first; the
  read is made when that fails too.

  The head's review. A review_by login's review-state comment (step 4b)
  counts for the head --ready judges alone: its code_review.commit_sha and
  its security_review.commit_sha must both be that head's full SHA (the
  head whose run step 3 watches, or the one it waits on with no provider).
  A state for another commit (either commit_sha not the head: Codex's for
  an earlier push) reads as waiting: no verdict and not running, and it
  still decides the login's verdict, a review by that login not counting
  under 4a beside it. The waiting line gives that login's state as
  waiting. Reviews (4a) are still counted for any commit, as Copilot's
  always were.

  The new head's request. Codex here reviews on ready_for_review alone, so
  after a fix pushed for its finding its state names the old commit and
  nothing would ever review the new head. So when a review_by login's
  latest review-state comment is for another commit, and no review in it
  that names the head reads anything but completed (a review of the head
  under way), --ready asks that login once for the head, at the look that
  reads it so: slice 190's request, gh pr comment <item> --repo
  <repository> --body=<"@codex review", an empty line, "<!--
  itos-review-request:v1 <login> <the head's full SHA> -->">, and it says,
  where its progress goes, "Asked <login> to review <the head's first 7
  characters> on <address>: its review state is for another commit." It
  posts it at that look before the look judges ci.watch.timeout or
  push.pr.ready_wait, so a wait already past its bound still asks. Then
  it waits as before, up to ci.watch.timeout (and push.pr.ready_wait). It
  does so whether or not push.pr.review_stall is set: asking for a head
  never reviewed is no stall. Once per head, as slice 190's: a comment on
  the pull request with that line for the login and the head (by anyone,
  posted by an earlier --ready or by hand), or a request this --ready
  posted, means it was asked already, and nothing is posted again. That
  request is the head's one: once Codex's state reads running for the head,
  slice 190 posts no second, and with review_stall set a review still
  running review_stall seconds after the request is the stall, as usual.
  Only a login with a review-state comment is asked: one with none waits as
  before.

  Every page of the comments. The pull request's comments are read to their
  last page, each page of a hundred asked for with the cursor the one before
  gave (gh api graphql again, -f cursor=<endCursor>), up to 20 pages, since a
  request for the head or a review state past the first hundred would
  otherwise be read as none, and a request posted again at every run once the
  pull request has that many. The reviews and the review threads are those
  of the first page.

  The wait's bound. push.pr.ready_wait is a number of seconds, 540 by
  default (Claude Code's 600 s foreground cap less a minute for the push
  before the wait), read under push.mode pr alone, as review_by is; 0 means
  no bound. config check refuses one that is not a number, as it refuses
  any value of the wrong type; config get push.pr.ready_wait prints 540
  when the config leaves it out; the schema lists it with the words "The
  seconds one itos push --ready may spend, counted from its start, before
  it stops waiting for GitHub's rebase, the run and the reviews with the
  outcome ready-still-waiting, so that it returns within its caller's time
  limit; itos push --ready again goes on. 0, no bound." It is one bound per
  call, counted from the call's start and never restarted by a wait: the
  waits inside it (for GitHub's rebase, step 3's run, step 4's reviews,
  slice 190's stall) share it, so time an earlier one took is gone for the
  later ones, and each ends at it having looked once at least, so a
  --ready whose run and reviews are there already never stops for it.
  ci.watch.timeout still bounds the run's and the reviews' waits, by the
  clocks below: whichever comes first decides the outcome. At ready_wait --ready
  stops: exit 75, outcome ready-still-waiting, auto-merge not turned on,
  the watch's own timeout line not printed, and stderr (under --json too)
  "itos: still waiting after <n> seconds (push.pr.ready_wait) on <address>
  at <the head's full SHA>: <the parts of the waiting line>; auto-merge is
  not turned on. itos push --ready again goes on waiting." The head is the
  one waited on: the head GitHub made once it is known (so itos ci watch
  <sha> follows the right run too); while GitHub's rebase has not shown,
  the head pushed, and the parts are "rebase waiting". A --ready run again
  continues where the last stopped, re-reading each state from GitHub and
  posting nothing posted already: the pull request is ready (no gh pr
  ready), the local branch takes the head GitHub made when the push
  rebases it onto the remote's item branch (so no second rebase is asked
  while main has not moved), and slice 190's request is once per head.
  --json: outcome gains ready-still-waiting.

  The clocks, kept by GitHub. ready_wait ends each call, so a clock counted
  from a call's own looks would restart at every rerun, and an agent
  running --ready again and again would never reach review-timeout or a
  stall. So under --ready the other clocks count from times GitHub keeps,
  read at the looks, and a rerun continues them: ci.watch.timeout for the
  run from the watched run's run_started_at (its latest attempt's start,
  so a rerun of the workflow starts it again), else its created_at;
  ci.watch.timeout for the reviews from when the pull request was marked
  ready or its head was pushed, whichever is later, both read once at the
  first ask: the last ReadyForReviewEvent's createdAt (in reviewQuery),
  and the push's time from gh api
  repos/<owner>/<name>/activity?ref=refs/heads/<item>&per_page=1, the
  first entry's timestamp when its after is the judged head's full SHA,
  else none (an activity call that fails or an answer that cannot be read
  is none too, not a failure: it only sets a clock); review_stall, before a request, from the review-state comment's
  updatedAt while it reads running (bug-164: after one too, when later than it; Codex edits it in place as its status
  changes), and after one from the request comment's createdAt (a request
  this --ready posted counting from when it posted it, until GitHub lists
  it). reviewQuery asks for each comment's createdAt and updatedAt and for
  the ready event's createdAt. A time GitHub does not give, or that cannot be read,
  counts from the first look of this call that read the thing, as before
  slice 200, so v8.31.0's recorded answers, which carry none, wait as they
  did. Outside --ready (a plain push's watch, itos ci watch) nothing
  changes.

  The head GitHub made, unwatched. With no ci.watch provider, once it has
  asked GitHub to rebase (step 2), --ready asks gh pr view <item> --repo
  <repository> --json headRefOid once, with no wait for the rebase. The
  head it answers counts only when it has the head of main the push
  fetched as an ancestor, as slice 175's rebasedHead judges one: then it is
  the head --ready judges and waits on (the waiting line names it, slice
  190's request marker names it, the review state is compared with it,
  the merge pins it). Else GitHub has not rebased yet, and --ready stops
  at once: exit 75, outcome ready-still-waiting, nothing merged, no review
  asked of GitHub and no request posted, stderr (under --json too) "itos:
  still waiting on <address> at <the head pushed's full SHA>: rebase
  waiting; GitHub has not rebased it onto <remote>/<main> yet, so
  auto-merge is not turned on. itos push --ready again goes on waiting." A
  gh pr view that fails is any gh call that fails. With a provider the head GitHub made is found
  as slice 175 finds it and printed as soon as it is ("GitHub rebased the
  pull request onto <main>: watching the run of <short>, its new head.",
  then the waiting line), as now.

  The implementing push changes command.go's help, doc.go and the schema
  for the rules above, and in a docs commit of its own AGENTS.md's
  Finishing: on ready-still-waiting, itos push --ready again, the ci watch
  fallback for a call cut off going.

  Not in slice 200: reading GitHub's mergeStateStatus before asking for
  auto-merge; a --ready that runs itself again after ready-still-waiting
  (the caller's loop); asking a login with no review-state comment
  (Copilot) to review a new head; a review (4a) of an earlier commit,
  which still counts.

  Slice 200's steps. Codex's review state (slice 187's steps, slice 190's
  "until a pr comment call" among them) names the clone's HEAD, its full SHA
  when the step runs, as both reviews' commit_sha, rather than a fixed 40
  hex digits. That changes what live scenarios set up, and the slice lands
  as a fix, as slice 196 did (a breaking feat would cut v9, which go.mod
  refuses): the fix commit makes the change and names in Changes:, with no
  BREAKING-CHANGE:, ID-PUSHDRAFT-17, 19, 25, 26, 28 and 29, whose v8.31.0
  steps give a commit_sha that is no head and expect a verdict or a review
  running, and 27, which expects no pr comment where the new head's
  request now posts one (and any other live case it alters); 18, 22 and
  23 pass either way (18 and 22 post that request too, and assert nothing
  of it). "ci.watch's provider is
  <provider>" gives the config a ci.watch of its own when no step gave one
  (interval 0, as the fake GitHub's), so that "ci.watch.timeout is <n>" can
  follow it with no provider. The recording gh's pr merge, as any call, can
  fail. New steps: "the recording gh's <command> calls with <text> fail,
  saying <words>", a call of the command whose recorded line has the text
  writing the words to stderr and exiting 1, every other call as before;
  "the recording gh's <command> call has the full SHA of the head of the
  remote's branch <branch>", as the step for the clone's HEAD, with the SHA
  that branch has in the remote when the step runs; "its output says
  <text> with the full SHA of the head of the remote's branch <branch>", the
  text then that SHA, as the step for the first commit's; "the recording gh
  answers that the pull request has a comment by <login> holding Codex's
  review state, its code review <status> of <commit> and its security
  review <status> of <commit>", each <commit> either the clone's HEAD (its
  full SHA when the step runs) or the commit <sha>; and "the recording gh
  answers that the pull request has a comment by <login> holding Codex's
  review state of the head of the remote's branch <branch> at each call,
  its code review <status> and its security review <status>", both
  commit_sha the SHA that branch has in the remote when api graphql is
  called, so after the recording gh's pr update-branch the head it made,
  and the same step ending "for its first <n> api graphql calls, then both
  <status>", as slice 187's.

  The read-back and the clocks need the fakes to know more. The recording
  gh keeps what its pr merge calls did: one that succeeds without --auto
  merges the branch's pull request (MERGED, mergedAt the call's time); one
  with --auto sets its auto-merge request, unless a step says it merges at
  once. Its pr view answers the fields its --json asks: headRefOid as
  before, and state, mergedAt (null unless merged) and autoMergeRequest
  (null, or {"mergeMethod":"REBASE"}) from that. Its api graphql answer
  gives a comment a createdAt and an updatedAt, and the pull request its
  ready event's createdAt, only when a step gave them, in the shape the
  query asks. It answers the api repos/owner/scratch/activity call with
  [] unless a step gave the head's push time, then with one entry whose
  after is the head the remote's branch has at the call and whose
  timestamp is that time. More new steps: "the recording gh's pr merge
  with --auto merges the pull request at once, as gh does when it reads it
  clean"; "the recording gh's <command> call with <text> has the full SHA
  of the clone's HEAD" (or "of the head of the remote's branch <branch>"),
  a call of the command that has both; "ci.watch.interval is <n>", as
  "ci.watch.timeout is <n>" is; "the watched run was started <n> seconds
  ago", the fake GitHub's run then carrying run_started_at, that long
  before the step ran (none otherwise); "the watched run was created <n>
  seconds ago", its created_at that long before the step ran and no
  run_started_at (the fake gives every other look a created_at of its
  own time, as a run just made has); "the recording gh answers that the
  pull request was marked ready <n> seconds ago and its head pushed <m>
  seconds ago", the ready event's createdAt and the activity entry's
  timestamp that long before the step ran; slice 187's Codex-state step ending ", last edited <n>
  seconds ago", the comment's createdAt and updatedAt that long before the
  step ran; and slice 190's "asking <login> to review the clone's HEAD
  again" ending ", posted <n> seconds ago", likewise; "the recording gh
  answers that the pull request has, on a second page of comments, a comment
  by <author> asking <login> to review the clone's HEAD again", which makes
  its answers give the comments the other steps gave as a first page whose
  pageInfo has a next page, cursor page2, and that comment as the second. For the refusal
  after a merge: "the recording gh's pull request for the branch <branch> is
  merged already", which makes its pr view answer state MERGED and a mergedAt
  for that branch, as if an auto-merge had merged it, whatever the merge
  calls did; its headRefOid is the remote branch's head as before.

  Slice 217: --ready asks for the review that lags. After a new head it asked
  Codex for the code review alone, "@codex review", and waited for both
  reviews of the head; with Codex's automatic reviews off nothing else starts
  the security review (features/review-request.feature), so the review state
  kept security_review on the old commit and --ready waited out its budget at
  every run, unless someone posted "@codex security review" by hand. It has
  happened on two repositories.

  So each of the two reviews in a review_by login's latest review-state
  comment is judged alone, at the look that reads it, whatever the other's
  commit or status: a review whose commit is not the head is asked for, a
  review with no commit counting as not the head, and a review of the head is
  not, whatever its status. "@codex review" asks for the
  code review and "@codex security review" for the security review, in that
  order, each as slice 200's request: gh pr comment <item> --repo <repository>
  --body=<the line, an empty line, "<!-- itos-review-request:v1 <login> <the
  head's full SHA> -->">; the security review's, since bug-163, two gh pr comments,
  the line alone and then the marker alone. This replaces
  slice 200's condition that no review of the head be under way: a state with
  the code review running for the head and the security review on an old
  commit waits for a request nothing else would post. The code review's
  request says as it did; the security review's says, where --ready's progress
  goes, "Asked <login> to review <the head's first 7 characters> for security
  on <address>: its review state is for another commit."

  Once per head, for each review, across separate --ready runs: a comment by
  anyone with that review's request for the login and the head, or a request
  this --ready posted, means that review was asked for already. The two
  requests are the ones itos review request posts and reads (slice-215): one
  marker line for both, the code review's under its line in one comment and the
  security review's alone in its own (bug-163), so a code review's request a
  released itos left on a pull request still reads as the code review's. The requests' text is
  internal/engine/reviewers', not a second copy of it.

  A login with no review-state comment yet is asked nothing, as before: a
  fresh pull request's first reviews are not this slice's.

  Not in slice 217: a review that never starts (slice-218); asking on
  ready_for_review; slice 190's stall, whose request stays "@codex review". The
  stall counts only the code review's request, so that a security request,
  on the pull request or posted by this --ready, neither takes the place of
  the stall's one "@codex review" nor starts the clock that ends in
  review-stalled.

  The implementer rewrites docs/ORCHESTRATING.md:107-108, which tells agents
  to post "@codex security review" by hand: with no marker that comment counts
  for nothing, so --ready would start a second security review. What replaces
  that line is `itos review request <item>`, since a fresh pull request is left
  unasked by --ready. The --ready help and internal/slices/push/doc.go follow
  this rule.

  Slice 217's steps are there already but one: the review state of a commit
  (slice 200), "requesting <line> of <login> at the clone's HEAD" or "at the
  commit <sha>" and "the recording gh's <command> calls are these lines" (slice
  215), and "a comment by <login> saying <text>" (slice 206). New: "the
  recording gh answers that the pull request has a comment by <login> holding
  Codex's review state, its code review <status> of <commit> and its security
  review <status> of <commit> until a pr comment call, then both <status> of
  the clone's HEAD", each <commit> as in slice 200's step and the clone's HEAD
  its full SHA when the step runs, into the files slice 190's "until a pr
  comment call" fills (gh-comments-before and gh-comments-after).

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The keys are push.pr's, read only under pr. Fails before the work at the
  # exit code: today config check refuses the unknown key push.pr, exit 2.
  @ID-PUSHDRAFT-01 @slice-179
  Scenario: config check accepts push.pr's draft and review_by
    Given the config sets "push.pr" to the YAML "{draft: true, review_by: ['copilot-pull-request-reviewer[bot]']}"
    When itos checks the config
    Then itos exits with code 0

  # Off by default, so pr pushes as slices 174 and 175 made it. Fails before
  # the work at the exit code: today config get refuses the unknown key, exit 2.
  @ID-PUSHDRAFT-02 @slice-179
  Scenario: config get prints push.pr.draft's default, false, when the config leaves it out
    When itos runs "config get push.pr.draft"
    Then itos exits with code 0
    And its output says "false"

  # Off by default: --ready asks no review again unless the config says when.
  # Fails before the work at the exit code: today config get refuses the
  # unknown key push.pr.review_stall, exit 2.
  @ID-PUSHDRAFT-24 @slice-190
  Scenario: config get prints push.pr.review_stall's default, 0, when the config leaves it out
    When itos runs "config get push.pr.review_stall"
    Then itos exits with code 0
    And its output says "0"

  # Bounded by default, inside Claude Code's 600 s foreground cap with a
  # minute left for the push. Fails before the work at the exit code: today
  # config get refuses the unknown key push.pr.ready_wait, exit 2.
  @ID-PUSHDRAFT-30 @slice-200
  Scenario: config get prints push.pr.ready_wait's default, 540, when the config leaves it out
    When itos runs "config get push.pr.ready_wait"
    Then itos exits with code 0
    And its output says "540"

  # The slice's point: a push as the agent works opens a draft, which nothing
  # judges or merges, so no auto-merge and no run, ci.watch or not. Fails
  # before the work at the pr create step: today push.pr is an unknown key,
  # so the config cannot be read and the push is a trunk push to main that
  # calls no gh.
  @ID-PUSHDRAFT-03 @slice-179
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
  @ID-PUSHDRAFT-04 @slice-179
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
  @ID-PUSHDRAFT-05 @slice-179
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
  @ID-PUSHDRAFT-06 @slice-179
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

  # gh pr merge --auto merges at once when gh reads the pull request clean
  # (cli/cli's isImmediatelyMergeable), saying nothing off a terminal: the
  # very state --ready waits for (#146, #149, #156). So the --auto call is
  # pinned to the head whose run --ready watched, and --ready reads the pull
  # request back and says it merged, not that auto-merge is on. Fails
  # before the work at the --match-head-commit step: today the --auto call
  # carries no head, and --ready reads nothing back.
  @ID-PUSHDRAFT-31 @slice-200
  Scenario: Under push.pr.draft, push --ready pins its --auto merge to the head whose run it watched and says GitHub merged it at once
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr merge with --auto merges the pull request at once, as gh does when it reads it clean
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call with "--match-head-commit" has the full SHA of the clone's HEAD
    And the recording gh's "pr merge" call has "--auto"
    And the recording gh was called as "pr merge" once
    And the recording gh's "pr view" call has "autoMergeRequest"
    And its output says "Merged https://github.example/o/r/pull/7 by rebase at "
    And its output does not say "Auto-merge is on"

  # The race: gh read a state that let it ask for auto-merge, and GitHub by
  # then found the pull request clean, refusing it. --ready merges once
  # itself, both calls pinned to the head it judged, the one GitHub made
  # when it rebased the pull request onto main, whose run it watched, never
  # the head pushed; --json says it merged, auto-merge off. Fails before the
  # work at the exit code: today the refusal is any gh call failing, exit 3,
  # pull-request-failed.
  @ID-PUSHDRAFT-32 @slice-200
  Scenario: Under push.pr.draft, push --ready whose --auto merge GitHub refuses as clean merges once itself at the head GitHub made, and --json says it merged
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the recording gh's "pr merge" calls with "--auto" fail, saying "GraphQL: Pull request Pull request is in clean status (enablePullRequestAutoMerge)"
    When itos runs "push --ready --json"
    Then itos exits with code 0
    And its JSON's "pull_request.merged" is true
    And its JSON's "pull_request.auto_merge" is false
    And the fake GitHub was asked for the run of the head of the remote's branch "slice-9"
    And the recording gh's "pr merge" call with "--auto" has the full SHA of the head of the remote's branch "slice-9"
    And the recording gh's "pr merge" call has "--repo owner/scratch --rebase --match-head-commit"
    And the recording gh's call with "pr merge slice-9 --repo owner/scratch --auto" comes before its call with "pr merge slice-9 --repo owner/scratch --rebase"

  # Only GitHub's word that the pull request is clean lets --ready merge it:
  # any other refusal of auto-merge (a repository that allows none, here)
  # is a gh call failing, as before, and nothing is merged. A guard: it
  # passes before the work and must after it.
  @ID-PUSHDRAFT-33 @slice-200 @guard
  Scenario: Under push.pr.draft, push --ready whose auto-merge GitHub refuses for another reason merges nothing and exits 3
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's "pr merge" calls with "--auto" fail, saying "GraphQL: Pull request Auto merge is not allowed for this repository (enablePullRequestAutoMerge)"
    When itos runs "push --ready --json"
    Then itos exits with code 3
    And its JSON's "outcome" is "pull-request-failed"
    And its output says "Auto merge is not allowed for this repository"
    And the recording gh was called as "pr merge" once

  # An auto-merge an earlier --ready turned on merged the pull request just
  # before this one's own merge call, which GitHub refused as not mergeable
  # (#209). The pull request is merged at the head --ready judged, so the
  # work is done and --ready says so rather than failing. Fails before the
  # work at the exit code: today the refusal is any gh call failing, exit 3,
  # pull-request-failed.
  @ID-PUSHDRAFT-44 @slice-200
  Scenario: Under push.pr.draft, push --ready whose merge call GitHub refuses because the pull request merged just before, at the head it judged, says it merged and exits 0
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's "pr merge" calls with "--auto" fail, saying "GraphQL: Pull Request is not mergeable"
    And the recording gh's pull request for the branch "slice-9" is merged already
    When itos runs "push --ready --json"
    Then itos exits with code 0
    And its JSON's "pull_request.merged" is true
    And its JSON's "pull_request.auto_merge" is false
    And the recording gh's "pr view" call has "state,mergedAt,headRefOid"
    And the recording gh was called as "pr merge" once

  # Under --ready the wait is a gate: a red run leaves auto-merge off. With
  # nothing to push, --ready still waits for the run of the pull request's
  # head, so a second --ready after a fix pushed plainly judges it. Fails
  # before the work at the exit code: today push refuses --ready, exit 2.
  @ID-PUSHDRAFT-07 @slice-179
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
  @ID-PUSHDRAFT-08 @slice-179
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
  @ID-PUSHDRAFT-09 @slice-179
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
  @ID-PUSHDRAFT-10 @slice-179
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
  @ID-PUSHDRAFT-11 @slice-179
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
  @ID-PUSHDRAFT-12 @slice-179
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
  @ID-PUSHDRAFT-13 @slice-179
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
  @ID-PUSHDRAFT-14 @slice-179
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
  @ID-PUSHDRAFT-15 @slice-179 @guard
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
  @ID-PUSHDRAFT-16 @slice-179 @guard
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

  # Codex posts no review on a clean pass: its verdict is the review-state
  # comment it edits in place, both reviews completed (#159). The config
  # writes the login as REST does, the comment's author is GraphQL's. Fails
  # before the work at the exit code: today only a review counts, so --ready
  # waits out ci.watch.timeout, exit 75.
  @ID-PUSHDRAFT-17 @slice-187
  Scenario: Under push.pr.draft, push --ready counts Codex's review-state comment with both reviews completed as its verdict and turns on auto-merge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["chatgpt-codex-connector[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" and its security review "completed"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "--auto"
    And its output says "Auto-merge is on"

  # While either of Codex's reviews runs there is no verdict yet: --ready
  # waits, and ci.watch.timeout ends the wait as for a review that never
  # came, naming the login. One row each way, so a build that reads only
  # one of the two statuses fails a row (review round 1 on #160). A guard:
  # it passes before the work (no review at all) and must after it, where
  # it holds that a state comment counts only once both reviews are
  # completed.
  @ID-PUSHDRAFT-18 @slice-187 @guard
  Scenario Outline: Under push.pr.draft, push --ready while either of Codex's reviews is still running waits out ci.watch.timeout and exits 75, turning on no auto-merge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "<code>" and its security review "<security>"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output says "no review by chatgpt-codex-connector"
    And the recording gh was never called as "pr merge"

    Examples:
      | code      | security  |
      | completed | running   |
      | running   | completed |

  # The wait reads the comment again at each look, as Codex edits it in
  # place: running on the first looks, completed after. ci.watch.timeout is
  # long enough for a few looks on a slow runner. Fails before the work at
  # the exit code: today no review by the login ever comes, so the wait ends
  # at ci.watch.timeout, exit 75.
  @ID-PUSHDRAFT-19 @slice-187
  Scenario: Under push.pr.draft, push --ready keeps waiting while Codex's reviews run and turns on auto-merge once its review-state comment says both completed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "running" for its first 2 api graphql calls, then both "completed"
    When itos runs "push --ready"
    Then itos exits with code 0
    And its output says "Waiting for reviews by chatgpt-codex-connector"
    And the recording gh's "pr merge" call has "--auto"

  # A review-state comment counts for the head --ready judges alone: Codex
  # edits one comment in place, and until it starts on a new push the
  # comment still says its review of an earlier one completed, which would
  # turn auto-merge on for commits no reviewer read. Both reviews'
  # commit_sha must be the head: one row each way, so a build that reads
  # only one fails a row, and one with both earlier. Such a state reads as
  # waiting, and since Codex here reviews on ready_for_review alone, nothing
  # would review the head: --ready asks for it once, with no
  # push.pr.review_stall, at the look that reads the state, before that
  # look judges ci.watch.timeout, then waits it out (the recording gh's
  # state never changes). Fails before the work at the exit code:
  # today commit_sha is not read, both reviews completed is the verdict and
  # auto-merge goes on, exit 0.
  @ID-PUSHDRAFT-34 @slice-200
  Scenario Outline: Under push.pr.draft, push --ready does not count Codex's review state for a commit other than the head, asks once for the head, and exits 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of <code> and its security review "completed" of <security>
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output says ": run succeeded, chatgpt-codex-connector waiting."
    And the recording gh was called as "pr comment" <calls> times
    And the recording gh's "pr comment" call has "<!-- itos-review-request:v1 chatgpt-codex-connector "
    And the recording gh's "pr comment" call has the full SHA of the clone's HEAD
    And its output says "Asked chatgpt-codex-connector to review "
    And the recording gh was never called as "pr merge"

    Examples:
      | code                                                  | security                                              | calls |
      | the clone's HEAD                                      | the commit "0123456789abcdef0123456789abcdef01234567" | 2     |
      | the commit "0123456789abcdef0123456789abcdef01234567" | the clone's HEAD                                      | 1     |

  # Once per head, as slice 190's request: a comment asking Codex for this
  # head's code review is on the pull request already (an earlier --ready's),
  # so a state whose code review is still of an older commit, its security
  # review of the head since slice 217, posts nothing more and waits. Fails before the
  # work at the exit code: today commit_sha is not read, both reviews
  # completed is the verdict and auto-merge goes on, exit 0.
  @ID-PUSHDRAFT-38 @slice-200
  Scenario: Under push.pr.draft, push --ready asks no second time for a head already asked of a login whose review state is for an older commit
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the clone's HEAD
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # The new head with neither review on it: both are asked for, the code
  # review first, each once over the many looks a second's wait makes (the
  # recording gh's comments do not grow, so a build keeping one "asked" per
  # login posts one, and one keeping none posts at every look). The lines are
  # exactly what itos review request posts. What is on the pull request asks
  # for neither: a request of an older commit, and "@codex security review"
  # with no marker, which names no head (ID-REVREQ-02 reads them so too).
  # Fails before the work at the pr comment lines: today only "@codex review"
  # is posted. The fix changes ID-PUSHDRAFT-34's first row, whose two requests
  # are no longer "once": the row moves here. 34's other rows stay: the second
  # now asks for the security review, the third keeps the case of a security
  # review completed for the head, which asks nothing: its one pr comment
  # leaves no room for a second, so it needs no scenario of its own here.
  @ID-PUSHDRAFT-47 @slice-217
  Scenario: Under push.pr.draft, push --ready asks for both of Codex's reviews, the code review first, when its review state names neither at the head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex review" of "chatgpt-codex-connector" at the commit "0123456789abcdef0123456789abcdef01234567"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the commit "0123456789abcdef0123456789abcdef01234567"
    And the recording gh answers that the pull request has a comment by "someone" saying "@codex security review"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And its output says "Asked chatgpt-codex-connector to review "
    And its output says " for security on https://github.example/o/r/pull/7: its review state is for another commit."
    And the recording gh was never called as "pr merge"

  # Each review is judged alone: only the one that lags is asked for, whatever
  # the other's status. The first row is the wait this slice ends: Codex
  # reviewed the code of the head and the state kept security_review on the old
  # commit. The last row's security review names no commit, which counts as not
  # the head. Fails before the work at the pr comment lines, for each row: today
  # the first and the last post "@codex review", for a code review that is done,
  # and the other two nothing, a review of the head reading as under way (slice
  # 200's underway, which judges the two together).
  @ID-PUSHDRAFT-48 @slice-217
  Scenario Outline: Under push.pr.draft, push --ready asks only for the one of Codex's reviews whose commit is not the head, whatever the other's status
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "<code>" of <code commit> and its security review "<security>" of <security commit>
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh's "pr comment" calls for "slice-9" are the comments of "<request>" of "chatgpt-codex-connector" at the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr merge"

    Examples:
      | code      | code commit                                           | security  | security commit                                       | request                |
      | completed | the clone's HEAD                                      | completed | the commit "0123456789abcdef0123456789abcdef01234567" | @codex security review |
      | running   | the clone's HEAD                                      | completed | the commit "0123456789abcdef0123456789abcdef01234567" | @codex security review |
      | completed | the commit "0123456789abcdef0123456789abcdef01234567" | running   | the clone's HEAD                                      | @codex review          |
      | completed | the clone's HEAD                                      |           | the commit ""                                         | @codex security review |

  # Once per head, for each review, across separate --ready runs. An earlier
  # run left one review's request on the pull request: that one is not posted
  # again and the other still is. The request on the pull request is the
  # comment a released itos leaves, the first row's as --ready and itos review
  # request have written it, so a code review's request reads as what it is and
  # holds back no security review; the marker line is the same in both and the
  # line above it tells them apart. Fails before the work at the pr comment
  # lines, for each row: today pullState.requested reads the marker alone, for
  # one request per login and head, so either comment posts nothing. The fix
  # changes ID-PUSHDRAFT-38 and 46, which read the code review's request as
  # all that is asked of a state on an older commit, expecting no pr comment:
  # each gives its security review's commit as the clone's HEAD, so that only
  # the code review lags and each keeps its point.
  @ID-PUSHDRAFT-49 @slice-217
  Scenario Outline: Under push.pr.draft, push --ready posts again no request of Codex's already on the pull request for the head, and still posts the other
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "<on the pull request>" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh's "pr comment" calls for "slice-9" are the comments of "<posted>" of "chatgpt-codex-connector" at the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr merge"

    Examples:
      | on the pull request    | posted                 |
      | @codex review          | @codex security review |
      | @codex security review | @codex review          |

  # The wait this slice ends, from end to end: the state keeps security_review
  # on an older commit until the request is posted, then reads both reviews
  # completed for the head, so --ready asks and finishes in one run. Fails
  # before the work at the pr comment lines: today any pr comment moves the
  # recording gh's state, so --ready turns on auto-merge, exit 0, having posted
  # "@codex review" for a code review that is done.
  @ID-PUSHDRAFT-52 @slice-217
  Scenario: Under push.pr.draft, push --ready asks for the security review that lags and turns on auto-merge once the state reads both completed for the head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" until a pr comment call, then both "completed" of the clone's HEAD
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh's "pr merge" call has "--auto"

  # A pull request with no review-state comment yet gets no request: a fresh
  # pull request's first reviews are not this slice's, and a duplicate request
  # there was seen in practice. Codex's other comment is no review state. A
  # guard: --ready passes before the work, since it asks only a login with a
  # review-state comment, and must after it, where the shared reader
  # (reviewers.Missing) counts a Codex with no state as unasked and would post
  # both.
  @ID-PUSHDRAFT-50 @slice-217 @guard
  Scenario: Under push.pr.draft, push --ready asks nothing of a Codex with no review-state comment yet, and waits
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" saying "To use Codex here, create an environment for this repo."
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # The agent reads what --ready waits on: a line when the wait starts and
  # one at each change, none while nothing changes. Codex's review runs at
  # the first two asks and completes at the third, so the line naming it
  # running is printed once, not at each ask, and the run's success comes
  # with the first ask's states, never beside a login not asked. Fails
  # before the work at the "Waiting on" output step: today --ready prints
  # only "Waiting for reviews by ...".
  @ID-PUSHDRAFT-25 @slice-190
  Scenario: Under push.pr.draft, push --ready says what it waits on when it starts and at each change of state, and nothing while nothing changes
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "running" for its first 2 api graphql calls, then both "completed"
    When itos runs "push --ready"
    Then itos exits with code 0
    And its output says "Waiting on https://github.example/o/r/pull/7 at "
    And its output says ": run waiting, chatgpt-codex-connector not asked." once, before ": run succeeded, chatgpt-codex-connector running."
    And its output says ": run succeeded, chatgpt-codex-connector running." once, before ": run succeeded, chatgpt-codex-connector completed."
    And its output does not say "run succeeded, chatgpt-codex-connector not asked."

  # --ready returns within its caller's time limit (slices 191 to 197
  # outlived Claude Code's 600 s cap): at push.pr.ready_wait it stops, exit
  # 75, naming what it waits on and the full SHA of the head GitHub made
  # when it rebased the pull request, whose run it watches, so a rerun, or
  # itos ci watch, follows the right run (on #213 the pushed head's read
  # cancelled). ci.watch.timeout is longer, and its line is not printed.
  # Fails before the work at the exit code: today push.pr.ready_wait is an
  # unknown key, so the config cannot be read and --ready is refused, exit
  # 1, ready-without-draft (and with the key known but not obeyed, the
  # watch runs to ci.watch.timeout, outcome nothing-to-push).
  @ID-PUSHDRAFT-35 @slice-200
  Scenario: Under push.pr.draft, push --ready stops at push.pr.ready_wait while the run of the head GitHub made goes on, exit 75, naming that head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          ready_wait: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run never finishes
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "ready-still-waiting"
    And its JSON's "pull_request.auto_merge" is false
    And its output says "(push.pr.ready_wait) on https://github.example/o/r/pull/7 at " with the full SHA of the head of the remote's branch "slice-9"
    And its output says ": run waiting; auto-merge is not turned on."
    And its output does not say "(ci.watch.timeout)"
    And the recording gh was never called as "pr merge"

  # The bound is the whole call's, the wait for the reviews included: with
  # no ci.watch provider no run is watched, and Copilot has not reviewed.
  # ci.watch.timeout is longer. Fails before the work at the exit code:
  # today push.pr.ready_wait is an unknown key, so --ready is refused, exit
  # 1 (and with the key known but not obeyed, the wait runs to
  # ci.watch.timeout, outcome review-timeout).
  @ID-PUSHDRAFT-36 @slice-200
  Scenario: Under push.pr.draft with no ci.watch provider, push --ready stops at push.pr.ready_wait while a review_by login has not reviewed, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
          ready_wait: 1
      """
    And ci.watch's provider is "none"
    And ci.watch.timeout is 10
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "ready-still-waiting"
    And its output says "(push.pr.ready_wait) on https://github.example/o/r/pull/7 at "
    And its output says "copilot-pull-request-reviewer[bot] waiting; auto-merge is not turned on."
    And the recording gh was never called as "pr merge"

  # ready_wait is one bound per call, counted from its start: the wait for
  # GitHub's rebase (gh pr view answering the old head four times, a
  # ci.watch.interval of 1 apart) uses four of its ten seconds, so the
  # reviews get the six left: asks at about 4, 5, ..., 10 seconds, seven at
  # most, and Codex's review, running at its first seven asks and completed
  # at the eighth, is still running when the bound ends it. A build that
  # gives each wait its own ten seconds asks eleven times in the reviews'
  # wait, reaches the eighth ask about seven seconds in and turns on
  # auto-merge, exit 0. The push before the waits has six seconds of slack
  # before the rebase wait would reach the bound. Fails before the work at the exit code:
  # today push.pr.ready_wait is an unknown key, so --ready is refused,
  # exit 1.
  @ID-PUSHDRAFT-40 @slice-200
  Scenario: Under push.pr.draft, push --ready counts push.pr.ready_wait from its start, so time the wait for GitHub's rebase took is gone for the reviews
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          ready_wait: 10
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.interval is 1
    And ci.watch.timeout is 30
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the recording gh's pr view answers the pull request's previous head for its first 4 calls after pr update-branch
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state of the head of the remote's branch "slice-9" at each call, its code review "running" and its security review "running" for its first 7 api graphql calls, then both "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "ready-still-waiting"
    And its output says "(push.pr.ready_wait) on https://github.example/o/r/pull/7 at " with the full SHA of the head of the remote's branch "slice-9"
    And its output says "chatgpt-codex-connector running; auto-merge is not turned on."
    And the recording gh was never called as "pr merge"

  # The review wait's clock is GitHub's, not the call's: the pull request
  # was marked ready, and its head then pushed (the repository's activity
  # for the branch), two minutes ago or more, past ci.watch.timeout, so a fresh
  # --ready with no review by Copilot ends at its first look with
  # review-timeout, and an agent running --ready again and again reaches
  # it. A build that counts from the call's first look waits, and
  # push.pr.ready_wait ends it, ready-still-waiting. Fails before the work
  # at the exit code: today push.pr.ready_wait is an unknown key, so --ready
  # is refused, exit 1.
  @ID-PUSHDRAFT-42 @slice-200
  Scenario: Under push.pr.draft, push --ready counts the review wait from when GitHub says the pull request was marked ready, so a rerun reaches review-timeout
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
          ready_wait: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 60
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request was marked ready 130 seconds ago and its head pushed 120 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output says "no review by copilot-pull-request-reviewer[bot]"
    And the recording gh was never called as "pr merge"

  # Past ci.watch.timeout a --ready looks once and returns, so its line no longer
  # says a rerun waits again, and says what it does. The outcome and the exit code
  # stay. Fails before the work at the "does not say" step: today the line ends
  # "itos push --ready again waits again.".
  @wip @ID-PUSHDRAFT-73 @bug-164
  Scenario: Under push.pr.draft, push --ready past ci.watch.timeout says a rerun stops at once unless the reviews have come, not that it waits again
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]"]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 60
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request was marked ready 130 seconds ago and its head pushed 120 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output does not say "waits again"
    And its output says "itos push --ready again turns it on if the reviews have come, else stops here at once"
    And its output says "a new head starts it afresh."
    And the recording gh was never called as "pr merge"

  # The run's clock is GitHub's too: the watched run's latest attempt
  # started two minutes ago (run_started_at), past ci.watch.timeout, so a
  # fresh --ready ends the watch
  # at its first look with the watch's own timeout. A build that counts
  # from the call's first look waits, and push.pr.ready_wait ends it,
  # ready-still-waiting. Fails before the work at the exit code: today
  # push.pr.ready_wait is an unknown key, so --ready is refused, exit 1.
  @ID-PUSHDRAFT-43 @slice-200
  Scenario: Under push.pr.draft, push --ready counts ci.watch.timeout from the watched run's start, so a rerun reaches the run's timeout
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          ready_wait: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 60
    And the watched run never finishes
    And the watched run was started 120 seconds ago
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "nothing-to-push"
    And its output says "(ci.watch.timeout)"
    And its output does not say "(push.pr.ready_wait)"
    And the recording gh was never called as "pr merge"

  # The run's clock falls back to its creation when GitHub gives no start
  # (the spec of slice 200, "else its created_at"): the watched run was made
  # two minutes ago and has no run_started_at, past ci.watch.timeout, so a
  # fresh --ready ends the watch at its first look with the watch's own
  # timeout, as 43's does. A build that counts from the call's first look
  # waits, and push.pr.ready_wait ends it, ready-still-waiting. Fails before
  # the work at the outcome step: today the clock is the call's, so the
  # call ends at push.pr.ready_wait, exit 75 too.
  @ID-PUSHDRAFT-45 @slice-200
  Scenario: Under push.pr.draft, push --ready counts ci.watch.timeout from the watched run's creation when it has no start, so a rerun reaches the run's timeout
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          ready_wait: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 60
    And the watched run never finishes
    And the watched run was created 120 seconds ago
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "nothing-to-push"
    And its output says "(ci.watch.timeout)"
    And its output does not say "(push.pr.ready_wait)"
    And the recording gh was never called as "pr merge"

  # A pull request with more than a hundred comments: the request for the
  # head sits on the second page, and the review state, for an older commit,
  # on the first. Read to the last page the request is seen, and nothing is
  # posted; read to the first alone it would be posted at every run. Fails
  # before the work at the cursor step: today only the first page is read, so
  # no call asks for the second (and the request is posted).
  @ID-PUSHDRAFT-46 @slice-200
  Scenario: Under push.pr.draft, push --ready reads every page of the pull request's comments, so a request for the head on the second is not posted again
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the clone's HEAD
    And the recording gh answers that the pull request has, on a second page of comments, a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh's "api graphql" call has "cursor=page2"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # Codex's code review stalls (#193): read running past
  # push.pr.review_stall, --ready asks for it again with one comment, the
  # recovery ADR-0061 documents, and Codex then reviews. Only a login whose
  # review-state comment reads running is asked again, always with
  # "@codex review": the state is Codex's, and no other reviewer's way to be
  # asked again follows from it (a Copilot that never reviews has no state
  # comment and waits, up to ci.watch.timeout, as before). A re-request
  # posted for an earlier head counts for nothing: once per head. Fails
  # before the work at the exit code: today push.pr.review_stall is an
  # unknown key, so the config cannot be read and --ready is refused, exit
  # 1, ready-without-draft (and with the key known, nothing asks again and
  # ci.watch.timeout ends the wait, exit 75).
  @ID-PUSHDRAFT-26 @slice-190
  Scenario: Under push.pr.draft, push --ready asks once again for a Codex review still running past push.pr.review_stall, and turns on auto-merge once it completes
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the commit "0123456789abcdef0123456789abcdef01234567" again
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed" until a pr comment call, then both "completed"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh was called as "pr comment" once
    And the recording gh's "pr comment" call has "slice-9"
    And the recording gh's "pr comment" call has "--repo owner/scratch"
    And the recording gh's "pr comment" call has "@codex review"
    And the recording gh's "pr comment" call has "<!-- itos-review-request:v1 chatgpt-codex-connector "
    And the recording gh's "pr comment" call has the full SHA of the clone's HEAD
    And its output says "Asked chatgpt-codex-connector to review "
    And its output says "(push.pr.review_stall)"
    And the recording gh's "pr merge" call has "--auto"

  # The stall counts only the code review's request: a security request for the
  # head, as itos review request leaves one, is not the stall's. So the code
  # review, running past push.pr.review_stall, is asked for once, and the state
  # then completes. Fails before the work at the exit code: today
  # pullState.requested reads the marker alone, so the security request counts
  # as the stall's, nothing is posted, and --ready ends at review_stall, exit
  # 75, review-stalled.
  @ID-PUSHDRAFT-51 @slice-217
  Scenario: Under push.pr.draft, push --ready past push.pr.review_stall asks for the code review even with a security request on the head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed" until a pr comment call, then both "completed"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And its output says "(push.pr.review_stall)"
    And the recording gh's "pr merge" call has "--auto"

  # The same for a security request this --ready posted: it is not the stall's
  # request either, so a build that records it as the login's would never post
  # "@codex review", ending at review_stall with the security request alone.
  # The code review runs for the head and the security review lags, so the
  # security request goes first; the state then reads both running for the head,
  # the code review is asked for at review_stall, and it still runs review_stall
  # seconds later. Fails before the work at the outcome: today the code review
  # running for the head holds back any request, the recording gh's state never
  # moves, and --ready ends at ci.watch.timeout, review-timeout.
  @ID-PUSHDRAFT-53 @slice-217
  Scenario: Under push.pr.draft, push --ready posts the stall's "@codex review" after a security request of its own, then ends review-stalled
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" of the clone's HEAD and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" until a pr comment call, then both "running" of the clone's HEAD
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh was never called as "pr merge"

  # Off by default: without push.pr.review_stall a running review is waited
  # for as slice 187 waits for it, up to ci.watch.timeout, and nothing asks
  # for it again. ci.watch.timeout is 2, so a build that asks again after a
  # default of a second or so fails here. A guard: it passes before the work
  # and must after it.
  @ID-PUSHDRAFT-27 @slice-190 @guard
  Scenario: Under push.pr.draft, push --ready without push.pr.review_stall never asks for a running review again and exits 75 at ci.watch.timeout
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 2
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # Once per head, ever: an earlier --ready asked Codex again for this head
  # (its comment is on the pull request), so this one posts nothing more,
  # and the review still running review_stall into this run is the stall.
  # Fails before the work at the exit code: today push.pr.review_stall is
  # an unknown key, so the config cannot be read and --ready is refused,
  # exit 1 (and with the key known, the wait runs to ci.watch.timeout,
  # outcome review-timeout).
  @ID-PUSHDRAFT-28 @slice-190
  Scenario: Under push.pr.draft, push --ready posts no second request for a head already asked again, and stops at the stall, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And its JSON's "pull_request.auto_merge" is false
    And the recording gh was never called as "pr comment"
    And its output says "the review by chatgpt-codex-connector of "
    And its output says "on https://github.example/o/r/pull/7 stalled"
    And the recording gh was never called as "pr merge"

  # Asking again did not bring the review: still running review_stall after
  # the request this run posted, --ready stops with the stall, an outcome
  # of its own beside review-timeout, having asked once. Fails before the
  # work at the exit code: today push.pr.review_stall is an unknown key, so
  # the config cannot be read and --ready is refused, exit 1 (and with the
  # key known, the wait runs to ci.watch.timeout, outcome review-timeout).
  @ID-PUSHDRAFT-29 @slice-190
  Scenario: Under push.pr.draft, push --ready stops, exit 75, when a review it asked for again still runs push.pr.review_stall later, having asked once
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh was called as "pr comment" once
    And its output says "on https://github.example/o/r/pull/7 stalled: still running"
    And its output says "after it was asked again"
    And the recording gh was never called as "pr merge"

  # The case seen in practice (bug-157), from end to end: the code review of the
  # head completed, the security review null with no commit, and only the code
  # review's request on the pull request, which is no request for the security
  # review. --ready asks for it (slice 217), asks once more review_stall later,
  # and stops review_stall after that, not at ci.watch.timeout. Rests on slice
  # 217: the first request, and the request above reading as the code review's.
  # The recording gh lists what --ready posts, as GitHub does, so a build adding
  # the listed requests to the posted ones reads its first as two, stops with no
  # second and names issuecomment-2, its marker. Fails before the work at the
  # outcome: today the marker alone reads as asked, nothing is posted and the wait ends at
  # ci.watch.timeout, review-timeout.
  @ID-PUSHDRAFT-54 @slice-218
  Scenario: Under push.pr.draft, push --ready asks once more for a review that has not started review_stall after its request, and stops review_stall after that, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr comment prints "https://github.example/o/r/pull/7#issuecomment-" and the call's number
    And the recording gh lists each pr comment it takes
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "pull_request.auto_merge" is false
    And its JSON's "not_started.login" is "chatgpt-codex-connector"
    And its JSON's "not_started.review" is "security"
    And its JSON's "not_started.head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-4"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And its output says "Asked chatgpt-codex-connector again, at https://github.example/o/r/pull/7#issuecomment-4, for its security review of "
    And its output says ": it had not started 1 seconds (push.pr.review_stall) after the request."
    And its output says "the security review by chatgpt-codex-connector of " with the full SHA of the head of the remote's branch "slice-9"
    And its output says " on https://github.example/o/r/pull/7 has not started 1 seconds (push.pr.review_stall) after the request https://github.example/o/r/pull/7#issuecomment-4, so auto-merge is not turned on"
    And its output does not say "(ci.watch.timeout)"
    And the recording gh was never called as "pr merge"

  # bug-163, from end to end: --ready's security request is the line alone then
  # the marker alone, twice, each pair making one request of the two, and what it
  # names is the marker's comment, the one the count reads. The recording gh lists
  # what --ready posts and numbers each call, so the second request's marker is the
  # fourth comment. A build counting the listed lines as requests stops short; one
  # keeping the line's address reads its own request, once listed, as a second.
  # Fails before the work at not_started.request: today each request is one comment,
  # so the stop names issuecomment-2.
  @ID-PUSHDRAFT-66 @bug-163
  Scenario: Under push.pr.draft, push --ready asks for the security review as the line alone then the marker alone, each pair one of its two requests
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr comment prints "https://github.example/o/r/pull/7#issuecomment-" and the call's number
    And the recording gh lists each pr comment it takes
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "not_started.review" is "security"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-4"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And its output says "Asked chatgpt-codex-connector again, at https://github.example/o/r/pull/7#issuecomment-4, for its security review of "
    And the recording gh was never called as "pr merge"

  # bug-163, across runs: an earlier --ready's two requests, each the line alone
  # and the marker alone, are the two a review and a head get, so this --ready posts
  # no third and stops at its first look, naming the later marker by its time,
  # GitHub listing it first here. Fails before the work at the outcome: today a
  # marker alone reads as no request, so it asks in the old form and
  # ci.watch.timeout ends the wait, review-timeout.
  @ID-PUSHDRAFT-67 @bug-163
  Scenario: Under push.pr.draft, push --ready counts each marker alone on the head as a security request, finding two posts no third and stops
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" saying "@codex security review"
    And the recording gh answers that the pull request has a comment by "someone" holding the request marker alone of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh answers that the pull request has a comment by "someone" saying "@codex security review"
    And the recording gh answers that the pull request has a comment by "someone" holding the request marker alone of "chatgpt-codex-connector" at the clone's HEAD, posted 300 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # bug-163, the upgrade: two requests in the old form, well past review_stall, on a
  # pull request a released itos asked on. They started nothing and count as no
  # request, so --ready asks in the form that works, once, and the review it brings
  # lands the pull request. Fails before the work at the exit code: today they are
  # the two requests, so --ready stops at its first look, 75, review-not-started,
  # asking nothing, and every later --ready does the same until a new head.
  @ID-PUSHDRAFT-68 @bug-163
  Scenario: Under push.pr.draft, push --ready counts the old marked security requests on the head as none, asks in the form that works and lands
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" holding the old marked request "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 300 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "someone" holding the old marked request "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit until a pr comment call, then both "completed" of the clone's HEAD
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh's "pr merge" call has "--auto"

  # What reads as not started, each review judged alone: its status null, or no
  # commit, or another commit whatever its status, a request for that review and
  # the head being on the pull request. The other review has completed for the
  # head in every row, and does not count. A status null of the head is no
  # running review, though both commits name the head: its row must not end
  # review-stalled. The request has no time, so the clock counts from the first
  # look. Rests on slice 217: a request reading as the one review's by its line.
  # Fails before the work at the outcome, for each row: the null of the head
  # reads as running today and ends review-stalled; the others wait out
  # ci.watch.timeout, review-timeout, the marker alone reading as asked.
  @ID-PUSHDRAFT-55 @slice-218
  Scenario Outline: Under push.pr.draft, push --ready reads a review as not started by its own status and commit, whatever the other review's
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr comment prints "https://github.example/o/r/pull/7#issuecomment-" and the call's number
    And the recording gh answers that the pull request has a comment by "someone" requesting "<request>" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review <code> and its security review <security>
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "not_started.review" is "<review>"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-<comment>"
    And the recording gh's "pr comment" calls for "slice-9" are the comments of "<request>" of "chatgpt-codex-connector" at the head of the remote's branch "slice-9"
    And its output says ": run succeeded, chatgpt-codex-connector waiting."
    And its output does not say "chatgpt-codex-connector running"
    And its output says "the <review> review by chatgpt-codex-connector of " with the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr merge"

    Examples:
      | request                | code                                                                 | security                                                             | review   | comment |
      | @codex security review | "completed" of the clone's HEAD                                      | null of no commit                                                    | security | 2       |
      | @codex security review | "completed" of the clone's HEAD                                      | null of the clone's HEAD                                             | security | 2       |
      | @codex security review | "completed" of the clone's HEAD                                      | "completed" of no commit                                             | security | 2       |
      | @codex security review | "completed" of the clone's HEAD                                      | "completed" of the commit "0123456789abcdef0123456789abcdef01234567" | security | 2       |
      | @codex security review | "completed" of the clone's HEAD                                      | "running" of the commit "0123456789abcdef0123456789abcdef01234567"   | security | 2       |
      | @codex review          | null of no commit                                                    | "completed" of the clone's HEAD                                      | code     | 1       |

  # The clock is GitHub's: a request made two minutes ago, past review_stall, by
  # an earlier --ready. This one asks once more at its first look and then counts
  # from what it posted, so ci.watch.timeout comes first. A build counting from
  # its own first look posts nothing; one that keeps counting from the listed
  # request stops at the next look, review-not-started. Rests on slice 217: the
  # request above reading as the security review's. Fails before the work at the
  # pr comment lines: today the marker alone reads as asked, nothing is posted.
  @ID-PUSHDRAFT-56 @slice-218
  Scenario: Under push.pr.draft, push --ready counts review_stall for a review not started from the request comment's time, then from the request it posted
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 2
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh was never called as "pr merge"

  # The once-per-run rule: two requests for a review and a head, ever. A later
  # --ready finding both, the later one past review_stall, posts no third and
  # stops at its first look, naming the later by its time, GitHub listing it
  # first here. It is what catches a build allowing a request per --ready, which
  # posts a third here. Rests on slice 217: a request reading as the review's.
  # Fails before the work at the outcome: today nothing ends the wait but
  # ci.watch.timeout, review-timeout.
  @ID-PUSHDRAFT-57 @slice-218
  Scenario: Under push.pr.draft, push --ready finding two requests for a review not started, past review_stall, posts no third and stops at its first look
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 300 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # The same while the clock has not run: the later request is five seconds old,
  # the earlier well past review_stall, so nothing is posted and ci.watch.timeout
  # comes first. A guard: it passes before the work, where the marker alone reads
  # as asked and nothing ends the wait but ci.watch.timeout, and must after it,
  # where a request older than review_stall can post or stop: a build counting
  # from the earlier request stops at its first look. A build allowing a request
  # per --ready passes it, the clock not having run: ID-PUSHDRAFT-57 catches one.
  @ID-PUSHDRAFT-58 @slice-218 @guard
  Scenario: Under push.pr.draft, push --ready finding two requests for a review not started posts no third while review_stall has not run from the later one
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 2
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 5 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-12"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # A new head starts afresh: the two requests an older commit reached count for
  # nothing. Both reviews lag here, each judged alone with its own two requests,
  # the code review first at each look, so the stop names it, by the fourth call.
  # Rests on slice 217: both first requests. Fails before the work at the outcome:
  # today one "@codex review" is posted and the wait ends at ci.watch.timeout,
  # review-timeout.
  @ID-PUSHDRAFT-59 @slice-218
  Scenario: Under push.pr.draft, push --ready counts requests for the head alone, and judges each of Codex's reviews with its own two
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr comment prints "https://github.example/o/r/pull/7#issuecomment-" and the call's number
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the commit "0123456789abcdef0123456789abcdef01234567"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the commit "0123456789abcdef0123456789abcdef01234567"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-not-started"
    And its JSON's "not_started.review" is "code"
    And its JSON's "not_started.request" is "https://github.example/o/r/pull/7#issuecomment-4"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh was never called as "pr merge"

  # What the second request is for: it starts the review, seen repeatedly, and
  # --ready goes on to the merge in the same run. Rests on slice 217: the request
  # reading as the security review's, and its "until a pr comment call" step.
  # Fails before the work at the exit code: today the marker alone reads as asked,
  # no pr comment is posted, the state never moves and ci.watch.timeout ends the
  # wait, exit 75.
  @ID-PUSHDRAFT-60 @slice-218
  Scenario: Under push.pr.draft, push --ready turns on auto-merge when the second request brings the review that had not started
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's pr comment prints "https://github.example/o/r/pull/7#issuecomment-" and the call's number
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit until a pr comment call, then both "completed" of the clone's HEAD
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh was called as "pr comment" 2 times
    And its output says "Asked chatgpt-codex-connector again, at https://github.example/o/r/pull/7#issuecomment-2, for its security review of "
    And the recording gh's "pr merge" call has "--auto"

  # A review that starts ends the not-started clock, and the ordinary path goes
  # on: it reads running after two looks, well inside review_stall, so no second
  # request goes, and it ends as slice 190's stall, not as a review not started.
  # Both requests are on the pull request, so the stall posts none either. A
  # guard: it passes before the work, where nothing posts and running stalls, and
  # must after it, where a build that latches "not started" at the first looks
  # posts the second request or stops review-not-started.
  @ID-PUSHDRAFT-61 @slice-218 @guard
  Scenario: Under push.pr.draft, push --ready stops counting a review as not started once it reads running, and goes on as for any running review
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 2
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit for its first 2 api graphql calls, then both "running" of the clone's HEAD
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And its output says "still running"
    And its output does not say "has not started"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # Off by default: without push.pr.review_stall a review not started is waited
  # for up to ci.watch.timeout and asked for no more, whatever the request's age.
  # A guard: it passes before the work and must after it, where a build reading a
  # review_stall of 0 as no time at all posts at the first look, or stops there.
  @ID-PUSHDRAFT-62 @slice-218 @guard
  Scenario: Under push.pr.draft, push --ready without push.pr.review_stall waits for a review not started up to ci.watch.timeout, asking no more
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output does not say "has not started"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # Only a review-state comment can say a review has not started: a login that
  # wrote none, with a request past review_stall, may be a reviewer with no state
  # at all, so it waits up to ci.watch.timeout as before. Codex's other comment is
  # no review state. A guard: it passes before the work and must after it, where a
  # build reading no state as a null one posts the second request or stops.
  @ID-PUSHDRAFT-63 @slice-218 @guard
  Scenario: Under push.pr.draft, push --ready does not read a login with no review-state comment as not started, and waits up to ci.watch.timeout
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 2
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 120 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" saying "To use Codex here, create an environment for this repo."
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # Judged alone, a stall and a review not started run side by side: the code
  # review runs for the head and the security review has not started. Each gets
  # its own one request review_stall in, the code review's first, and review_stall
  # later the code review, judged first, ends the wait as slice 190's stall. Rests
  # on slice 217: the stall counting the code review's request alone. Fails before
  # the work at the outcome: today a running code review holds back every request
  # and the stall reads both commits, so ci.watch.timeout ends the wait,
  # review-timeout.
  @ID-PUSHDRAFT-64 @slice-218
  Scenario: Under push.pr.draft, push --ready judges a running code review and a security review not started each on its own, the code review first
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" of the clone's HEAD and its security review null of no commit
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh was never called as "pr merge"

  # With no ci.watch provider nothing found the head GitHub made when it
  # rebased the pull request onto main (#210), so slice 190's request named
  # the head from before the rebase, and a later --ready could ask again for
  # the real one. --ready reads it once with gh pr view, compares Codex's
  # state with it (running on that head here) and names it when it asks
  # again. Fails before the work at the pr view step: today with no
  # provider gh pr view is never called, and the request names the clone's
  # HEAD (the exit code and the outcome pass today, the state's commit_sha
  # not being read).
  @ID-PUSHDRAFT-37 @slice-200
  Scenario: Under push.pr.draft with no ci.watch provider, push --ready reads the head GitHub made once and asks again for a stalled review of it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch's provider is "none"
    And ci.watch.timeout is 10
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state of the head of the remote's branch "slice-9" at each call, its code review "running" and its security review "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh was called as "pr view" once
    And the recording gh's "pr view" call has "--json headRefOid"
    And the recording gh's "pr comment" call has the full SHA of the head of the remote's branch "slice-9"

  # The one gh pr view after the rebase may answer the head from before it
  # (GitHub not caught up), which lacks main's head: --ready judges no head
  # it cannot trust, so it stops at once, ready-still-waiting with "rebase
  # waiting", asks GitHub nothing more, posts no request (Codex's state,
  # for an older commit, would ask for the head) and merges nothing; a rerun
  # finds the rebased head. Fails before the work at the exit code: today
  # with no provider gh pr view is never called, the state's commit_sha is
  # not read, both reviews completed is the verdict and auto-merge goes on,
  # exit 0.
  @ID-PUSHDRAFT-39 @slice-200
  Scenario: Under push.pr.draft with no ci.watch provider, push --ready whose one gh pr view answers a head without main's stops, rebase waiting, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch's provider is "none"
    And ci.watch.timeout is 10
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the recording gh's pr view answers the pull request's previous head for its first 1 calls after pr update-branch
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the commit "0123456789abcdef0123456789abcdef01234567" and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "ready-still-waiting"
    And its output says "rebase waiting"
    And the recording gh was called as "pr view" once
    And the recording gh was never called as "api graphql"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # The stall's clock is GitHub's: the request for this head was posted
  # two minutes ago, past push.pr.review_stall, and Codex's state, running
  # for the head, was last edited 30 seconds ago, after it. Once a request
  # exists the stall counts from its createdAt, so a fresh --ready stops at
  # its first look with the stall, posting nothing. A build that counts
  # from the state's updatedAt alone (30 seconds, 40 at ci.watch.timeout)
  # or from the call's first look never reaches 60 seconds within
  # ci.watch.timeout and ends review-timeout. Fails before the work
  # at the outcome step: today the stall counts from this call's first look
  # that read it running, so the wait ends at ci.watch.timeout,
  # review-timeout.
  @ID-PUSHDRAFT-41 @slice-200
  Scenario: Under push.pr.draft, push --ready counts push.pr.review_stall from GitHub's times, so a rerun past it stops at the stall
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 120 seconds ago
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed", last edited 30 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # The run seen on slice-218's own: the code and security requests are 20 minutes
  # old, review_stall, and Codex's state has just flipped to code review running
  # with its security review null. The stall counts from the review's start, the
  # state's last edit 5 seconds ago, so the code review is not stalled, while the
  # security review not started gets its second request alone, in bug-163's form,
  # and the review then completes. Fails before the work at the exit code: today
  # the code request replaces the last edit, so the first look stops, 75,
  # review-stalled.
  @wip @ID-PUSHDRAFT-69 @bug-164
  Scenario: Under push.pr.draft, push --ready does not stop a review that started seconds ago, however old its request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1200
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 1200 seconds ago
    And the recording gh answers that the pull request has a comment by "someone" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD, posted 1200 seconds ago, at "https://github.example/o/r/pull/7#issuecomment-11"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" of the clone's HEAD and its security review null of no commit, last edited 5 seconds ago, for its first 2 api graphql calls, then both "completed" of the clone's HEAD
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment slice-9 --repo owner/scratch --body=@codex security review
      pr comment slice-9 --repo owner/scratch --body=<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      """
    And the recording gh's "pr merge" call has "--auto"

  # A review truly stuck still stops, and is asked again first. The one request is
  # older than the review's start, so it asked for what is running, not for it
  # again; running review_stall past that start, --ready posts "@codex review", and
  # stops review_stall after it. Fails before the work at the pr comment count:
  # today the old request reads as asked again, so the first look stops with
  # nothing posted, and only a build that asks again passes.
  @wip @ID-PUSHDRAFT-70 @bug-164
  Scenario: Under push.pr.draft, push --ready asks once again for a review still running push.pr.review_stall past its start when its one request is older, then stops, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 300 seconds ago
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed", last edited 120 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh was called as "pr comment" once
    And the recording gh's "pr comment" call has "@codex review"
    And its output says "after it was asked again"
    And the recording gh was never called as "pr merge"

  # Two requests for a review and a head, ever, stay slice 218's: both older than
  # the review's start and it past review_stall, --ready posts no third and stops.
  # A guard: it passes before the work, the later request being past review_stall,
  # and must after it, where a build reading any request older than the start as no
  # asking at all posts a third.
  @wip @ID-PUSHDRAFT-71 @bug-164 @guard
  Scenario: Under push.pr.draft, push --ready posts no third request for a review running past its start with two requests older than it, and stops, exit 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 1
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 300 seconds ago
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 200 seconds ago
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed", last edited 120 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr merge"

  # A rerun after review-stalled reads the same clock. The first --ready stops at
  # a review running past review_stall, with two requests older than its start. The
  # review then starts afresh, its state edited 5 seconds ago, and the second
  # --ready goes on to the verdict, posting nothing. A build reading the edit only
  # for one request passes ID-69 and stops here. Fails before the work at the second
  # exit code: today the later request, 120 seconds old, is the clock, so the rerun
  # stops too, 75, review-stalled, until the review completes.
  @wip @ID-PUSHDRAFT-72 @bug-164
  Scenario: Under push.pr.draft, push --ready run again after review-stalled does not stop a review started since
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
          review_stall: 60
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 10
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 300 seconds ago
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again, posted 120 seconds ago
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed", last edited 90 seconds ago
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-stalled"
    Given the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" of the clone's HEAD and its security review "completed" of the clone's HEAD, last edited 5 seconds ago, for its first 2 api graphql calls, then both "completed" of the clone's HEAD
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh was never called as "pr comment"
    And the recording gh's "pr merge" call has "--auto"

  # Copilot out of quota posts a review that is a refusal, and #158 merged
  # on it unreviewed. A refusal is no verdict, and waiting cannot bring a
  # review nobody asks for again: --ready stops at once, quoting it.
  # ci.watch.timeout is short so that an implementation that waits instead
  # fails at the outcome rather than by hanging. Fails before the work at
  # the exit code: today the refusal counts as the review and auto-merge
  # goes on, exit 0.
  @ID-PUSHDRAFT-20 @slice-187
  Scenario: Under push.pr.draft, push --ready stops at a review_by login's refusal to review, exit 75, quoting it and turning on no auto-merge
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
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-refused"
    And its JSON's "pull_request.auto_merge" is false
    And its output says "copilot-pull-request-reviewer[bot] gave no verdict on https://github.example/o/r/pull/7"
    And its output says "Copilot was unable to review this pull request because"
    And its output says "request its review again"
    And the recording gh was never called as "pr merge"

  # Once its review is requested again Copilot reviews, and the refusal
  # stays on the pull request beside the review: a login with a verdict
  # counts whatever refusal it posted before. The review begins with
  # Copilot's overview marker, as every real one here does. A guard: it
  # passes before the work (any review counts) and must after it, where it
  # holds that a refusal stops the wait only for a login with no verdict.
  @ID-PUSHDRAFT-21 @slice-187 @guard
  Scenario: Under push.pr.draft, push --ready counts a login's review given after its refusal and turns on auto-merge
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
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "<!-- ccr-overview-v2 -->"
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "--auto"

  # When a review_by login wrote a review-state comment, the state alone
  # decides: Codex posts its code review's findings as a review while its
  # security review may still run, and that review counting at once would
  # turn auto-merge on before the security findings land. Fails before the
  # work at the exit code: today any review by the login counts, so
  # auto-merge goes on, exit 0.
  @ID-PUSHDRAFT-22 @slice-187
  Scenario: Under push.pr.draft, push --ready does not count a review by a login whose review-state comment says a review is still running, and exits 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "chatgpt-codex-connector" whose body is "### Codex Review"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" and its security review "running"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output says "no review by chatgpt-codex-connector"
    And the recording gh was never called as "pr merge"

  # A state comment counts only by the login it is the verdict of: one by
  # the pull request's author (the reviewer quoting Codex's, say), both
  # reviews completed, is not Codex's. A guard: it passes before the work,
  # where no comment is read at all, and is kept since the work makes
  # comments count, and a build that counts any author's would pass every
  # other scenario.
  @ID-PUSHDRAFT-23 @slice-187 @guard
  Scenario: Under push.pr.draft, push --ready does not count a review-state comment written by a login not in review_by, and exits 75
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And ci.watch.timeout is 1
    And the watched run's jobs "ci" and "platform" succeed
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" holding Codex's review state, its code review "completed" and its security review "completed"
    When itos runs "push --ready --json"
    Then itos exits with code 75
    And its JSON's "outcome" is "review-timeout"
    And its output says "no review by chatgpt-codex-connector"
    And the recording gh was never called as "pr merge"
