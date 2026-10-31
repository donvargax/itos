@phase-1
Feature: itos review status posts the itos/reviews commit status on an item's pull request's head, judged as --ready judges the reviews
  The person's answer to q-105 (2026-10-30; docs/plans/v9-coordinator.md,
  open questions 4 and 6): the wait for the reviews moves from itos push
  --ready to the forge, as a required status check, so that --ready can
  hand the pull request off and return (p1-ready-hand-off). Today the only
  judge of whether push.pr.review_by's logins reviewed a head is --ready's
  wait, which holds a session for as long as Codex takes; nothing the forge
  can require says it. Wanted: one command that reads the head's reviews as
  --ready reads them and writes the result where branch protection can
  require it, a commit status on that head, so a workflow can run it on
  every pull request event and the merge waits on GitHub, not on a session.

  itos review status <item>, under push.mode pr, from any branch or a
  detached HEAD: the item names the branch, and its open pull request is
  the one judged, found as review post finds it (gh pr list --repo
  <repository> --head <item> --state open --json number,url, the repository
  the one push names: ci.range.github.repository_env, else the remote's
  URL). Then the one question --ready and review post put
  (providers.AskPullState: gh api graphql, paged past a hundred comments),
  whose answer gives the head (headRefOid) and the reviews and comments.

  The judging is --ready's, exactly (push.feature's slice 179, 187 and
  200 rules, push-mode-draft.feature): for each push.pr.review_by login, a
  trailing "[bot]" dropped from both sides, a login that wrote a Codex
  review-state comment has the verdict that comment gives and no other,
  both its reviews completed and both of the head; any other login's
  verdict is a review that is not Copilot's refusal ("Copilot ... was
  unable to review this pull request"), whatever commit it was of, since
  the question asks a review for no commit and --ready counts it so; a
  login whose only reviews are refusals has refused. For the implementer:
  the judging must be --ready's code, never a second reading of it. Since
  a slice imports no other slice, pullState.verdicts and what it calls
  (stateOf, refusalLine, sameLogin, in internal/slices/push/ready.go) move
  to internal/providers beside PullStateQuery, as slice-206 moved the
  question, and --ready's awaitReviews calls them there. --ready's waiting,
  asking for the head and stall rules stay its own.

  The status. One GitHub commit status on the head that answer gave, never
  another commit (not the clone's HEAD, not its remote-tracking branch):
  gh api repos/<owner>/<name>/statuses/<the head's full SHA> -f
  state=<state> -f context=itos/reviews -f description=<description>, the
  fields in that order, the description last, and no target_url. gh's exit
  0 is the status written; its answer is not read. The state: failure when
  a review_by login has refused, whatever the others' state (--ready's
  review-refused stops there too, since no review comes unless someone asks
  again); else pending when a login has no verdict for the head; else
  success. The description names the logins as the config writes them, in
  its order: success "Reviewed by <every login>.", pending "Waiting on
  <the logins with no verdict>.", failure "Refused to review: <the logins
  that refused>.", the logins joined by ", ". GitHub refuses (422) a
  status whose description is over 140 characters, it does not cut one,
  so itos cuts a longer one at a whole login: the first logins, as many as
  fit, then " and <n> more." for the n left out, the whole at most 140
  characters; when not even the
  first login fits, "<the same beginning><n> logins.". The output and
  --json name every login, uncut.

  It asks nobody for a review (no "@codex review", no gh pr comment, no
  issue comment), waits for nothing, merges and marks nothing, and fetches,
  rebases and pushes nothing: one forge question and one status write, so
  running it again posts the same state again and is harmless. Unresolved
  review threads are not its business: GitHub's conversation-resolution
  rule holds the merge on them.

  It says one line, "Posted itos/reviews <state> on <the head's first 7
  characters> of <the pull request's address>: " and then "reviewed by
  <every login>." for success, "waiting on <the logins with no verdict>."
  for pending, and for failure each login that refused with the first line
  of its last refusal, "<login> refused to review: "<line>"", joined by
  "; ", then ".". Exit 0 whenever a status was posted, whatever its state.
  --json: {"schema":1,"ok":true,"outcome","item","state","head",
  "pull_request":{"url","number"},"description","missing","refused"},
  outcome reviewed (success), review-pending (pending) or review-refused
  (failure, the word --ready's outcome uses), head the full SHA, missing
  every login with no verdict in the config's order (those that refused
  among them), refused those that refused, each [] for none, description
  the one posted.

  Refused, exit 1, nothing posted, each a problem of the house JSON as
  review post's are ({"schema":1,"ok":false,"problems":[{"rule","message",
  "fix"}]}, the message on stderr without --json). Before gh is asked, in
  this order: outside push.mode pr (trunk, no config, or one that cannot be
  read), review-without-pr-mode, "itos review: the reviews' status goes on
  an item's pull request, and this config does not set push.mode: pr;
  nothing was posted."; push.pr.review_by empty or absent,
  review-without-review-by, "itos review: push.pr.review_by names no login
  whose review to judge; nothing was posted.", its fix to list them there;
  an item the registry does not have, work-unknown-item, as itos work show
  refuses it. Once gh pr list answers: no open pull request for the item,
  review-no-pull-request, review post's words ("itos review: <item> has no
  open pull request; itos push opens it. Nothing was posted."). Exit 3,
  rule review-pull-request-failed, nothing posted past the call that
  failed: a gh that is missing or fails, naming the call and gh's words
  ("gh api repos/<owner>/<name>/statuses/<sha> failed: HTTP 403: Resource
  not accessible by integration"), and an answer to the question without
  the shape asked for or without a head; no-github-repository, exit 3, a
  remote that names no GitHub repository. A usage error, exit 2, asking gh
  nothing: status with no item or two, or with any flag (--verdict, --at,
  --stage, --body-file are post's alone). itos --help lists "review status
  <item>" and itos help review says what the status is, its context
  itos/reviews and its three states.

  Not in this slice: the workflow that runs it on pull request events
  (p1-reviews-status-workflow), which also decides what to do on a branch
  that names no item; adding itos/reviews to main's branch protection (the
  person's); --ready's hand-off (p1-ready-hand-off), which leaves --ready's
  own wait as it is until then; a review of a plain reviewer bound to a
  commit (the question asks none, so --ready and this count any review).

  The steps. The recording gh is push-mode.feature's, unchanged: the status
  call goes to api-repos.sh, which answers nothing and exits 0 for a path
  it does not know, and itos reads no answer from it, so no handler is
  added. New steps, in a step file of their own: "the recording gh posted
  <n> commit status(es), (each) the call <line>, @HEAD:<b>@ the full SHA of
  the head of the remote's branch <b>", which takes the recorded calls
  starting "api repos/" that hold "/statuses/", fails unless there are
  exactly n, and fails unless each one is exactly <line> (the whole
  recorded line, as dispatch.sh records it), "@HEAD:<b>@" in it replaced by
  the full SHA of the head the remote's branch has when the step runs; so
  the line "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=<s>
  -f context=itos/reviews -f description=<d>" holds the field order and
  that no other field, target_url among them, is sent; "the recording gh posted
  no commit status", no such call (the recording gh was never called as
  <command> cannot say it: it matches a command followed by a space, and
  the status call's path goes on with "/<sha>", so it would pass whatever
  was posted); and "its output says <text> with the first 7 characters of
  the head of the remote's branch <b>", the sibling of readywait_test.go's
  full-SHA step. The rest exist: the config, the recording gh, its pull
  requests, its reviews and Codex's review-state comments, its failing
  calls, "the remote's branch <b> has gained the commit ..." (another
  clone's push, which the clone never fetches) and "the clone's HEAD is
  detached at its commit".

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: every review_by login has reviewed the head, so one
  # status, itos/reviews success, goes on the head GitHub gives, after one
  # forge question and nothing else: no comment, no request for a review,
  # no merge, no ready. Copilot's refusal sits beside its later review,
  # which counts (--ready's rule, PUSHDRAFT-21), and the config's
  # "copilot-pull-request-reviewer[bot]" matches GraphQL's login without
  # [bot]. Run from main: the item names the branch. Fails before the work
  # at the exit code: today review's only word is post, so review status is
  # a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-01
  Scenario: Under push.mode pr, review status posts itos/reviews success on the pull request's head once every review_by login has reviewed it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector, "copilot-pull-request-reviewer[bot]"]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" and its security review "completed"
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "<!-- ccr-overview-v2 -->"
    When itos runs "review status slice-9"
    Then itos exits with code 0
    And its output says "Posted itos/reviews success on " with the first 7 characters of the head of the remote's branch "slice-9"
    And its output says " of https://github.example/o/r/pull/7: reviewed by chatgpt-codex-connector, copilot-pull-request-reviewer[bot]."
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=success -f context=itos/reviews -f description=Reviewed by chatgpt-codex-connector, copilot-pull-request-reviewer[bot].", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh was called as "api graphql" once
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"
    And the recording gh was never called as "pr merge"
    And the recording gh was never called as "pr ready"

  # Codex's code review still runs on the head: no verdict yet, so pending,
  # the description naming the login still missing, and nothing waited for.
  # Run again, it asks the same question and posts the same state, so the
  # workflow may run it on every event. --json is what a driver reads.
  # Fails before the work at the exit code: today review status is a usage
  # error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-02
  Scenario: Under push.mode pr, review status posts itos/reviews pending naming the review_by logins with no verdict for the head, and the same again when run again
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector, "copilot-pull-request-reviewer[bot]"]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "completed"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "review-pending"
    And its JSON's "item" is "slice-9"
    And its JSON's "state" is "pending"
    And its JSON's "head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "pull_request.url" is "https://github.example/o/r/pull/7"
    And its JSON's "pull_request.number" is 7
    And its JSON's "description" is "Waiting on chatgpt-codex-connector."
    And its JSON's "missing" is ["chatgpt-codex-connector"]
    And its JSON's "refused" is []
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=pending -f context=itos/reviews -f description=Waiting on chatgpt-codex-connector.", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr comment"
    When itos runs "review status slice-9"
    Then itos exits with code 0
    And its output says "Posted itos/reviews pending on " with the first 7 characters of the head of the remote's branch "slice-9"
    And its output says ": waiting on chatgpt-codex-connector."
    And the recording gh posted 2 commit statuses, each the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=pending -f context=itos/reviews -f description=Waiting on chatgpt-codex-connector.", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr comment"

  # The head is GitHub's, read in the same answer as the reviews, never the
  # clone's: someone pushed to the branch after Codex reviewed, and the
  # clone, detached at the reviewed commit, never fetched it. Codex's state
  # names the older commit, so it counts for nothing on the new head:
  # pending, on the new head. --ready would ask Codex once for the head
  # (PUSHDRAFT-34); review status asks nobody. A build that took the
  # clone's HEAD (or its remote-tracking branch, which the clone's own push
  # left at the older commit) would post success there. A draft: the
  # workflow runs on a draft's pushes too. Fails before the work at the
  # exit code: today review status is a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-03
  Scenario: Under push.mode pr, review status from a detached HEAD judges and marks the head GitHub gives, so a review state for an older commit is pending there, and asks nobody for a review
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" and its security review "completed"
    And the remote's branch "slice-9" has gained the commit "fix: answer the review" touching "widget.md"
    And the clone's HEAD is detached at its commit
    When itos runs "review status slice-9 --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "review-pending"
    And its JSON's "head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "missing" is ["chatgpt-codex-connector"]
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=pending -f context=itos/reviews -f description=Waiting on chatgpt-codex-connector.", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"

  # A login that wrote a review-state comment has the verdict that comment
  # gives and no other: Codex posts its code review's findings as a review
  # while its security review still runs, and that review must not count
  # (--ready's rule, PUSHDRAFT-22). A build that counted any review by the
  # login would post success. Fails before the work at the exit code: today
  # review status is a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-04
  Scenario: Under push.mode pr, review status does not count a review by a login whose review-state comment says a review still runs, and posts pending
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "chatgpt-codex-connector"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" and its security review "running"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "review-pending"
    And its JSON's "state" is "pending"
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=pending -f context=itos/reviews -f description=Waiting on chatgpt-codex-connector.", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"

  # Copilot's only review is its refusal: no review comes unless someone
  # asks again, so failure, not pending, though Codex's review still runs
  # (--ready's review-refused, PUSHDRAFT-20, stops there too). missing names
  # both logins, refused the one that refused; the line quotes the refusal
  # as --ready does. A build that ranked pending above failure would post
  # pending. Fails before the work at the exit code: today review status is
  # a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-05
  Scenario: Under push.mode pr, review status posts itos/reviews failure when a review_by login's only reviews are refusals, whatever the others' state
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: ["copilot-pull-request-reviewer[bot]", chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a review by "copilot-pull-request-reviewer" whose body is "Copilot was unable to review this pull request because the user who requested the review has insufficient remaining quota."
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "running" and its security review "running"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "review-refused"
    And its JSON's "state" is "failure"
    And its JSON's "missing" is ["copilot-pull-request-reviewer[bot]", "chatgpt-codex-connector"]
    And its JSON's "refused" is ["copilot-pull-request-reviewer[bot]"]
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=failure -f context=itos/reviews -f description=Refused to review: copilot-pull-request-reviewer[bot].", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"
    When itos runs "review status slice-9"
    Then itos exits with code 0
    And its output says "Posted itos/reviews failure on " with the first 7 characters of the head of the remote's branch "slice-9"
    And its output says "copilot-pull-request-reviewer[bot] refused to review: "
    And its output says "Copilot was unable to review this pull request because"
    And the recording gh was never called as "pr comment"

  # GitHub refuses (422) a status whose description is over 140
  # characters, so the list is cut at a whole login, the rest counted. Here three logins fit exactly, 140
  # characters with " and 1 more.", and all four (170) do not; a build that
  # stops one short of the limit keeps two, one that cuts mid-login or
  # appends an ellipsis ends otherwise. --json keeps every login. Nobody
  # has reviewed yet. Fails before the work at the exit code: today review
  # status is a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-06
  Scenario: Under push.mode pr, review status cuts a description past 140 characters at a whole login and counts the logins left out
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by:
            - reviewer-one-with-a-login-this-long-ab
            - reviewer-two-with-a-login-this-long-ab
            - reviewer-three-with-a-login-this-long
            - reviewer-four-with-a-login-this-long-ab
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "review-pending"
    And its JSON's "missing" is ["reviewer-one-with-a-login-this-long-ab", "reviewer-two-with-a-login-this-long-ab", "reviewer-three-with-a-login-this-long", "reviewer-four-with-a-login-this-long-ab"]
    And its JSON's "description" is "Waiting on reviewer-one-with-a-login-this-long-ab, reviewer-two-with-a-login-this-long-ab, reviewer-three-with-a-login-this-long and 1 more."
    And the recording gh posted 1 commit status, the call "api repos/owner/scratch/statuses/@HEAD:slice-9@ -f state=pending -f context=itos/reviews -f description=Waiting on reviewer-one-with-a-login-this-long-ab, reviewer-two-with-a-login-this-long-ab, reviewer-three-with-a-login-this-long and 1 more.", @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9"

  # A branch not pushed yet has no pull request to judge: refused, review
  # post's words, nothing asked past gh pr list and nothing posted. Fails
  # before the work at the exit code: today review status is a usage
  # error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-07
  Scenario: Under push.mode pr, review status with no open pull request for the item is refused, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-no-pull-request"'s "message" says "slice-9 has no open pull request"
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh was never called as "api graphql"
    And the recording gh posted no commit status

  # The workflow's token may lack statuses: write; GitHub refuses the
  # write, gh fails, and that is an environment failure, exit 3, naming
  # the call and gh's words, as review post reports gh. Fails before the
  # work at the exit code: today review status is a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-08
  Scenario: Under push.mode pr, review status whose status write gh fails exits 3, naming the call
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's "api" calls with "/statuses/" fail, saying "HTTP 403: Resource not accessible by integration"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 3
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-pull-request-failed"'s "message" says "gh api repos/owner/scratch/statuses/"
    And its JSON's problem "review-pull-request-failed"'s "message" says "HTTP 403: Resource not accessible by integration"

  # Under trunk there is no pull request to mark: refused, gh never asked.
  # Fails before the work at the exit code: today review status is a usage
  # error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-09
  Scenario: Outside push.mode pr, review status is refused, exit 1, asking gh nothing
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review status slice-9 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-without-pr-mode"'s "message" says "push.mode: pr"
    And the recording gh was never called

  # With no review_by there is nobody whose review to judge, and a success
  # posted for nobody would let branch protection pass any head: refused
  # before gh is asked. The order is pinned: review_by is judged before
  # the item, so slice-7, which the registry lacks, is refused for the
  # missing review_by alone, never as work-unknown-item. Fails before the
  # work at the exit code, each row: today review status is a usage error,
  # exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-10
  Scenario Outline: Under push.mode pr with no push.pr.review_by, review status of any item, in the registry or not, is refused for review_by, exit 1, asking gh nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review status <item> --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-without-review-by"'s "message" says "push.pr.review_by"
    And its output does not say "work-unknown-item"
    And the recording gh was never called

    Examples:
      | item    |
      | slice-9 |
      | slice-7 |

  # The item names the branch, so one the registry lacks is refused as
  # itos work show refuses it, before gh is asked. Fails before the work at
  # the exit code: today review status is a usage error, exit 2.
  @wip @slice-212 @ID-REVIEWSTATUS-11
  Scenario: Under push.mode pr, review status of an item the registry does not have is refused, exit 1, asking gh nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review status slice-7 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-unknown-item"'s "message" says "slice-7"
    And the recording gh was never called

  # status takes one item and no flag but --json: post's flags are post's.
  # A guard: every row is exit 2 before the work too, review's only word
  # being post, and must stay one after it.
  @wip @slice-212 @ID-REVIEWSTATUS-12 @guard
  Scenario Outline: Under push.mode pr, review status with no item, two items or any flag is a usage error, exit 2
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "<command>"
    Then itos exits with code 2
    And the recording gh was never called

    Examples:
      | command                                 |
      | review status                           |
      | review status slice-9 slice-8           |
      | review status slice-9 --at HEAD         |
      | review status slice-9 --verdict approve |
      | review status slice-9 --stage spec      |

  # The help is where the workflow's author looks first. Fails before the
  # work at the first output step: today itos --help lists review post
  # alone.
  @wip @slice-212 @ID-REVIEWSTATUS-13
  Scenario: itos's help lists review status and help review documents the itos/reviews status
    When itos runs "--help"
    Then itos exits with code 0
    And its output says "review status <item>", whatever its line breaks
    When itos runs "help review"
    Then itos exits with code 0
    And its output says "itos/reviews", whatever its line breaks
