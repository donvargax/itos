@phase-1
Feature: itos review request asks each configured reviewer for the pull request's head once, and itos next names the step
  The person, 2026-10-30: the deterministic workflow asks for a review
  explicitly, never leaving it to a reviewer's automatic trigger. Codex's
  automatic reviews are off and Cursor's Bugbot runs only when mentioned, so
  a head nobody asked about is never reviewed, and --ready waits on a review
  that never comes. Today the one request itos makes is --ready's askForHead
  (slice 200): Codex's code review alone, only for a login whose review
  state names another commit. Wanted: asking is a step of the machine, one
  command that asks every configured reviewer for the head, once, and itos
  next naming it while the head has no request, so a review nobody asked for
  is a fact itos reads, not a slow review.

  The reviewers. push.pr.reviewers, a list of {login, kind}, read under
  push.mode pr alone, empty by default: login the reviewer's GitHub login
  (a bot's "[bot]" suffix may be written or not), kind the Go adapter that
  says how to ask that reviewer and how its review of a head reads. Kinds
  are code, never a format language in the config, since vendors change
  their formats silently; each adapter's reading is tested against the
  bodies its vendor really wrote (PR 242's, recorded beside it), so a new
  format is a case to record. config check accepts codex and bugbot and
  refuses any other kind, as it refuses any value outside a key's set
  ("push.pr.reviewers[0].kind should be one of codex, bugbot"). The key is
  additive: push.pr.review_by stays what --ready and itos/reviews wait for,
  unchanged, and names nobody to ask; reviewers names whom to ask, gating
  nothing. A repository lists Codex in both when it both asks Codex and
  waits for it. The schema's words for it: "The reviewers itos review
  request asks to review an item's pull request's head, each a GitHub login
  and its kind (codex or bugbot); empty, it asks none."

  The kinds. A request is a pull request comment of two paragraphs: the
  kind's request line, an empty line, and itos's request marker "<!--
  itos-review-request:v1 <login> <the head's full SHA> -->" (the marker
  askForHead posts; login as the config writes it). codex asks twice per
  head, in this order: the line "@codex review" (the code review), then the
  line "@codex security review" (the security review, which with automatic
  review off "@codex review" no longer starts, PR 242). bugbot asks once:
  "@cursor review". Logins are matched as --ready matches review_by's
  (sameLogin), a "[bot]" suffix ignored on either side, so the config's
  cursor[bot] is the author GitHub's GraphQL names cursor, and the marker's
  login is the config's. A request is on the head when a comment by anyone
  has that request line as its first line and the marker line naming the
  reviewer's login and the head's full SHA; a request line with no marker,
  or a marker for another head or another login, counts for nothing. The
  reviewer's own review of the head counts as its request, each request
  judged alone: for codex, the code review when the login's review-state
  comment, as --ready reads it (its last "<!-- codex-review-state:v1 {...}
  -->"), has a code_review whose commit_sha is the head, whatever its
  status, and the security review when its security_review's commit_sha is
  the head, never one for the other (in manual mode the code review is
  often at the head and the security review at an older commit), and any
  other comment of Codex's (its "To use Codex here, create an environment
  for this repo" reply to a mention, seen on #243) counts for nothing; for
  bugbot, a review whose author is the login, whose body starts with
  "<!-- BUGBOT_REVIEW -->" and whose commit is the head, so the person's
  empty-bodied reviews (a thread reply makes one) and any other review by
  the login count for nothing. A reviewer is requested for a head when each
  of its kind's requests is.

  itos review request <item> [--expect <revision>] [--json], under
  push.mode pr, from any branch or a detached HEAD (the item names the
  branch). Refused before any gh call, exit 1, each a problem of the house
  JSON under --json and the message on stderr without it: outside push.mode
  pr (trunk, no config, or one that cannot be read), review-without-pr-mode,
  "itos review: a request goes on an item's pull request, and this config
  does not set push.mode: pr; nothing was posted."; an item the registry
  does not have, work-unknown-item, as itos work show refuses it; a
  push.pr.reviewers that names no reviewer, review-no-reviewers, "itos
  review: push.pr.reviewers names no reviewer to ask; nothing was posted."
  (push.pr.review_by alone names none). Then gh, each call through the gh on
  the PATH and the repository push names: gh pr list --repo <repository>
  --head <item> --state open --json number,url, as review post asks (none
  open: review-no-pull-request, "itos review: <item> has no open pull
  request; itos push opens it. Nothing was posted."); the one gh api graphql
  question --ready, review post and next put (providers.PullStateQuery,
  whose reviews become "reviews(first: 100) { nodes { author { login } body
  commit { oid } } }" here, so a review's commit is asked of GitHub), paged
  as they page it, for the head (headRefOid), the comments and the
  reviews. Then each request of
  each reviewer, in the config's order and the kind's, that is not on the
  head is posted, one gh pr comment <number> --repo <repository>
  --body=<the request> each, with the user's gh: a request then comes from a
  person's login, which Bugbot needs (it ignores a command from a bot or an
  app). Nothing is fetched, rebased or pushed, and no GitHub review and no
  issue comment through gh api is posted.

  The outcomes. requested, exit 0, when it posted one request or more,
  saying for each "Asked <login> at <the head's first 7 characters> on
  <address>: <the request line>."; already, exit 0, when every request of
  every reviewer is on the head, nothing posted, "Every reviewer of <item>
  has a request or a review for <the first 7 characters> on <address>;
  nothing was posted." --json: {"schema":1,"ok":true,"outcome","item",
  "head","posted"}, head the full SHA, posted the requests posted in order,
  each {"login","kind","request"}, [] for already. Run again it posts only
  what is still missing, so a driver that lost an answer runs it again.
  --expect <revision>, as work take and work done take it (slice 208): the
  revision itos next <item> --json gave, judged after the forge is read and
  before anything is posted: already holds whatever the revision; else a
  revision that is not the item's current one, computed by next's own code,
  is review-stale, exit 1, nothing posted, "<item>'s revision is <current>,
  not <given>: what was known of it when this command was chosen no longer
  holds", the fix "run itos next <item> again, and act on what it says now";
  a revision not "sha256:" and 64 lowercase hex digits is a usage error. A
  gh that is missing or fails exits 3, review-pull-request-failed, naming
  the call and gh's words ("gh pr comment failed: HTTP 502: Bad Gateway"),
  the fix saying that itos review request <item> again asks only for what
  is still missing. The first failed call stops the command: the requests
  posted before it stay posted, and none after it is tried.

  The words. request is review's second word beside post. A usage error,
  exit 2, asking gh nothing: request with no item or two, a malformed
  --expect, or any other flag. itos --help lists "review request <item>
  [--expect <revision>]" and itos help review says what each kind posts.

  itos next (slice-209's states, under push.mode pr). With
  push.pr.reviewers not empty, an item slice-209 would call merging (a ready
  pull request, its checks passed or pending, every thread resolved) or
  handed-back (a draft, the implementer's accepted done at its head) is
  review-unrequested while a configured reviewer is not requested for the
  pull request's head: driver request, run itos review request <item>, the
  text "<item>: review-unrequested; next: driver request; run: itos review
  request <item>", --json's next {"actor":"driver","action":"request",
  "argv":["itos","review","request","<item>"]}. Asked at handed-back too, so
  the reviews of the head run before itos push --ready waits on them; asked
  again for every new head, a fix pushed in repair or GitHub's rebase. repair
  and every state slice-209 judges before merging are judged before it: a
  red check or an open thread is the implementer's first, and its fix makes
  a new head. A todo item's draft stays in spec review: the request is the
  change's, never the spec's. observed gains unrequested, the configured
  logins (as written) not requested for the head of the open pull request,
  in the config's order, [] with none configured or no open pull request;
  the revision digests it, so a request posted gives a new one. With
  push.pr.reviewers empty nothing of slice-209's changes.

  Not in this slice: folding --ready's askForHead and its review_stall
  re-ask into this command (p1-ready-hand-off); the per-head verdict model
  (requested, running, passed, findings, refused, stalled) and reading
  Bugbot's "Cursor Bugbot" check run; the itos/reviews status (slice-212);
  any kind but codex and bugbot.

  The steps. The recording gh is push-mode.feature's with slice-206's and
  slice-209's additions, and learns: "the recording gh answers that the pull
  request has a comment by <author> requesting <line> of <login> at the
  clone's HEAD" (or "at the commit <sha>"), a comment node whose body is
  the line, an empty line and the marker for that login and that SHA; "the
  recording gh answers that the pull request has a review by <login> of the
  clone's HEAD whose body is <text>" (or "of the commit <sha>"), a review
  node with that author, body and commit { oid }; and "the recording gh's
  <command> calls are these lines, in order, @HEAD:<branch>@ the full SHA of
  the head of the remote's branch <branch>:", a doc string: every recorded
  call starting with the command, exactly those lines in that order, the
  placeholder replaced, a line end in a call written as the two characters
  \n, as dispatch.sh records it.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # The slice's point: a ready pull request nobody asked about gets three
  # comments, Codex's two requests then Bugbot's, each its request line
  # and the marker, through gh pr comment with the pull request's number;
  # nothing else is posted, nothing pushed. Fails before the work at the
  # exit code: today review request is a usage error, exit 2.
  @ID-REVREQ-01 @slice-215
  Scenario: Under push.mode pr, review request asks each configured reviewer for the head: Codex's code and security reviews, then Bugbot's
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
            - { login: "cursor[bot]", kind: bugbot }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "review request slice-9"
    Then itos exits with code 0
    And its output says "Asked chatgpt-codex-connector at "
    And its output says " on https://github.example/o/r/pull/7: @codex review."
    And its output says " on https://github.example/o/r/pull/7: @codex security review."
    And its output says "Asked cursor[bot] at "
    And its output says " on https://github.example/o/r/pull/7: @cursor review."
    And the recording gh's "pr list" call has "--head slice-9"
    And the recording gh's "pr list" call has "--state open"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "api graphql" call does not have "mutation"
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment 7 --repo owner/scratch --body=@codex review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment 7 --repo owner/scratch --body=@codex security review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment 7 --repo owner/scratch --body=@cursor review\n\n<!-- itos-review-request:v1 cursor[bot] @HEAD:slice-9@ -->
      """
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"
    And the recording gh was never called as "pr review"
    And the recording gh was never called as "pr create"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # Each request is judged alone. Not on the head, so posted: Codex's
  # security review, asked at an older commit and in a comment with no
  # marker, Codex's reply that is no review state beside them; and
  # Bugbot's, whose marked review is of an older commit, while at the head
  # are only ana's empty review (a thread reply makes one) and a review by
  # cursor without Bugbot's marker. On it, so not posted: Codex's code
  # review, which someone asked for at the head. A build asking per login
  # would post nothing for Codex, one that counted a request line without
  # its marker or any comment of Codex's nothing for its security review,
  # and one that counted any review at the head nothing for Bugbot. Fails
  # before the work at the exit code: today review request is a usage
  # error, exit 2.
  @ID-REVREQ-02 @slice-215
  Scenario: Under push.mode pr, review request posts only the requests not on the head: one at an older head, without its marker or answered at an older head is posted again
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
            - { login: "cursor[bot]", kind: bugbot }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has a comment by "ana" requesting "@codex security review" of "chatgpt-codex-connector" at the commit "0123456789abcdef0123456789abcdef01234567"
    And the recording gh answers that the pull request has a comment by "ana" saying "@codex security review"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" saying "To use Codex here, create an environment for this repo."
    And the recording gh answers that the pull request has a review by "cursor" of the commit "0123456789abcdef0123456789abcdef01234567" whose body is "<!-- BUGBOT_REVIEW --> Bugbot found no new issues."
    And the recording gh answers that the pull request has a review by "ana" of the clone's HEAD whose body is ""
    And the recording gh answers that the pull request has a review by "cursor" of the clone's HEAD whose body is "Bugbot found no new issues."
    When itos runs "review request slice-9 --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "requested"
    And its JSON's "item" is "slice-9"
    And its JSON's "head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "posted" is [{"login": "chatgpt-codex-connector", "kind": "codex", "request": "@codex security review"}, {"login": "cursor[bot]", "kind": "bugbot", "request": "@cursor review"}]
    And the recording gh's "pr comment" calls are these lines, in order, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9":
      """
      pr comment 7 --repo owner/scratch --body=@codex security review\n\n<!-- itos-review-request:v1 chatgpt-codex-connector @HEAD:slice-9@ -->
      pr comment 7 --repo owner/scratch --body=@cursor review\n\n<!-- itos-review-request:v1 cursor[bot] @HEAD:slice-9@ -->
      """

  # A reviewer's own review of the head is its request: Codex's review
  # state names the head for both reviews, one still running, and
  # Bugbot's review is of the head, so nothing is posted. Already holds
  # whatever --expect says, as work take's does: the revision is no
  # item's, and a build judging it first would say review-stale. The
  # recording gh answers the nodes it was given whatever the query asks,
  # so the query is checked to ask for each review's commit: without it,
  # GitHub would never give Bugbot's review a commit to match. Fails
  # before the work at the exit code: today review request is a usage
  # error, exit 2.
  @ID-REVREQ-03 @slice-215
  Scenario: Under push.mode pr, review request with every reviewer's review of the head posts nothing and reports already, whatever --expect says
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
            - { login: "cursor[bot]", kind: bugbot }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review "running" of the clone's HEAD
    And the recording gh answers that the pull request has a review by "cursor" of the clone's HEAD whose body is "<!-- BUGBOT_REVIEW --> Bugbot found no new issues."
    When itos runs "review request slice-9"
    Then itos exits with code 0
    And its output says "Every reviewer of slice-9 has a request or a review for "
    And its output says " on https://github.example/o/r/pull/7; nothing was posted."
    And the recording gh's "api graphql" call has "body commit { oid }"
    And the recording gh was never called as "pr comment"
    When itos runs "review request slice-9 --json --expect sha256:0000000000000000000000000000000000000000000000000000000000000000"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "already"
    And its JSON's "posted" is []
    And its JSON's "head" holds the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "pr comment"

  # Rerun safety: the driver chose the request at one head and the branch
  # moved since, so what it knew no longer holds; the current revision is
  # named and nothing posted. With the current revision it asks, and for
  # the new head. The next runs only read the revisions; the spec is
  # tagged so next reads the forge. Fails before the work at the first
  # review request's exit code: today it is a usage error, exit 2.
  @ID-REVREQ-04 @slice-215
  Scenario: Under push.mode pr, review request --expect with a revision that is no longer the item's is refused as stale, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is saved as "first"
    Given the remote's branch "slice-9" has gained the commit "feat: add the widget" touching "widget.md"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is saved as "second"
    When itos runs "review request slice-9 --json" with --expect the revision saved as "first"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-stale"'s "message" names the revision saved as "second"
    And its JSON's problem "review-stale"'s "fix" says "itos next slice-9"
    And the recording gh was never called as "pr comment"
    When itos runs "review request slice-9 --json" with --expect the revision saved as "second"
    Then itos exits with code 0
    And its JSON's "outcome" is "requested"
    And the recording gh was called as "pr comment" 2 times
    And the recording gh's "pr comment" call has the full SHA of the head of the remote's branch "slice-9"

  # The second request fails: an environment failure, exit 3, naming the
  # call and gh's words; the first was posted before it, the failure stops
  # the command so Bugbot's is never tried, and the fix is to run it
  # again, which asks only for what is missing. Fails before the work at
  # the exit code: today review request is a usage error, exit 2.
  @ID-REVREQ-05 @slice-215
  Scenario: Under push.mode pr, review request stops at the first gh pr comment that fails, exit 3, naming the call, the requests before it posted and none after it tried
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
            - { login: "cursor[bot]", kind: bugbot }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh's "pr comment" calls with "@codex security review" fail, saying "HTTP 502: Bad Gateway"
    When itos runs "review request slice-9 --json"
    Then itos exits with code 3
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-pull-request-failed"'s "message" says "gh pr comment failed: HTTP 502: Bad Gateway"
    And its JSON's problem "review-pull-request-failed"'s "fix" says "itos review request slice-9"
    And the recording gh's call with "--body=@codex review" comes before its call with "--body=@codex security review"
    And the recording gh's "pr comment" call does not have "@cursor review"

  # What is wrong before gh is asked is refused without asking it: an item
  # the registry lacks, then a config whose review_by names Codex but
  # whose reviewers name nobody: review_by is whom --ready waits for, not
  # whom to ask. Fails before the work at the first exit code: today
  # review request is a usage error, exit 2.
  @ID-REVREQ-06 @slice-215
  Scenario: Under push.mode pr, review request for an unknown item, or with no push.pr.reviewers, is refused before gh is asked
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          review_by: [chatgpt-codex-connector]
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    When itos runs "review request slice-7 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-unknown-item"'s "message" says "slice-7"
    When itos runs "review request slice-9 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-no-reviewers"'s "message" says "push.pr.reviewers"
    When itos runs "review request slice-9"
    Then itos exits with code 1
    And its output says "push.pr.reviewers names no reviewer to ask; nothing was posted."
    And the recording gh was never called

  # Under trunk there is no pull request to ask on: refused, gh never
  # asked. Fails before the work at the exit code: today review request is
  # a usage error, exit 2.
  @ID-REVREQ-07 @slice-215
  Scenario: Outside push.mode pr, review request is refused, exit 1, asking gh nothing
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review request slice-9 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-without-pr-mode"'s "message" says "push.mode: pr"
    And the recording gh was never called

  # A branch not pushed has no pull request to ask on; nothing is asked
  # past gh pr list. Fails before the work at the exit code: today review
  # request is a usage error, exit 2.
  @ID-REVREQ-08 @slice-215
  Scenario: Under push.mode pr, review request with no open pull request for the item is refused, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "review request slice-9 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-no-pull-request"'s "message" says "slice-9 has no open pull request"
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh was never called as "api graphql"
    And the recording gh was never called as "pr comment"

  # The key is new and additive, read only under pr. Fails before the
  # work at the exit code: today config check refuses the unknown key
  # push.pr.reviewers, exit 2.
  @ID-REVREQ-09 @slice-215
  Scenario: config check accepts push.pr.reviewers of the kinds codex and bugbot
    Given the config sets "push.pr" to the YAML "{review_by: [chatgpt-codex-connector], reviewers: [{login: chatgpt-codex-connector, kind: codex}, {login: 'cursor[bot]', kind: bugbot}]}"
    When itos checks the config
    Then itos exits with code 0

  # A kind is an adapter in itos, never something the config describes:
  # one itos has none for is refused as any value outside a key's set is.
  # Fails before the work at the output step: today config check refuses
  # the unknown key push.pr.reviewers, exit 2, in other words.
  @ID-REVREQ-15 @slice-215
  Scenario: config check refuses a push.pr.reviewers kind that is neither codex nor bugbot
    Given the config sets "push.pr" to the YAML "{reviewers: [{login: copilot-pull-request-reviewer, kind: copilot}]}"
    When itos checks the config
    Then itos exits with code 2
    And its output says "push.pr.reviewers[0].kind should be one of codex, bugbot"

  # The help is where a driver's author looks first. Fails before the work
  # at the first output step: today itos --help has no review request.
  @ID-REVREQ-10 @slice-215
  Scenario: itos's help lists review request and help review says what each kind posts
    When itos runs "--help"
    Then itos exits with code 0
    And its output says "review request <item> [--expect <revision>]", whatever its line breaks
    When itos runs "help review"
    Then itos exits with code 0
    And its output says "@codex security review", whatever its line breaks
    And its output says "@cursor review", whatever its line breaks
    And its output says "itos-review-request:v1", whatever its line breaks

  # request takes an item, once, and --expect alone. A guard: every row is
  # exit 2 before the work too, request being no word of review, and must
  # stay one after it.
  @ID-REVREQ-11 @slice-215 @guard
  Scenario Outline: Under push.mode pr, review request with no item, two items, a malformed --expect or another flag is a usage error, exit 2
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    When itos runs "<command>"
    Then itos exits with code 2
    And the recording gh was never called

    Examples:
      | command                                         |
      | review request                                  |
      | review request slice-9 slice-8                  |
      | review request slice-9 --expect abc             |
      | review request slice-9 --verdict approve        |
      | review request slice-9 --at HEAD                |

  # next names the step on a ready pull request, after repair: a red check
  # is the implementer's first. Pending checks are no reason to wait. The
  # code review and Bugbot's requested, Codex's security review is still
  # missing, so Codex is still unrequested; once it is asked too, the item
  # is merging. Each request gives a new revision. Fails before the work
  # at the complete line naming review-unrequested: slice-209's next says
  # merging.
  @ID-REVREQ-12 @slice-215
  Scenario: Under push.mode pr, a ready pull request whose head a configured reviewer was not asked about is review-unrequested, the driver requests
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
            - { login: "cursor[bot]", kind: bugbot }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request's head's checks are "FAILURE"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    Given the recording gh answers that the pull request's head's checks are "PENDING"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: review-unrequested; next: driver request; run: itos review request slice-9" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "review-unrequested"
    And its JSON's "next" is {"actor": "driver", "action": "request", "argv": ["itos", "review", "request", "slice-9"]}
    And its JSON's "observed.unrequested" is ["chatgpt-codex-connector", "cursor[bot]"]
    And its JSON's "observed.checks" is "pending"
    And its JSON's revision is saved as "first"
    Given the recording gh answers that the pull request has a comment by "ana" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has a comment by "ana" requesting "@cursor review" of "cursor[bot]" at the clone's HEAD
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "review-unrequested"
    And its JSON's "observed.unrequested" is ["chatgpt-codex-connector"]
    And its JSON's revision differs from the one saved as "first"
    And its JSON's revision is saved as "second"
    Given the recording gh answers that the pull request has a comment by "ana" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON's "observed.unrequested" is []
    And its JSON's revision differs from the one saved as "second"

  # Handed back on a draft: the reviews are asked for before the driver
  # lands it, so --ready waits on reviews already running. Verified
  # 2026-10-31 on draft #243: both vendors answer a mention on a draft
  # (Bugbot's check run started 7 s after the request; Codex's summary
  # showed both reviews running, "Manual request"). Codex's own review
  # state stands for each request whose review names the head, and only
  # that one: its code review at the head and its security review at an
  # older commit, as manual mode leaves them, is still unrequested; a
  # build reading one commit_sha for both would say handed-back. Once the
  # security review is asked for at the head, handed back, the driver
  # lands it. Fails before the work at the first complete line:
  # slice-209's next says handed-back.
  @ID-REVREQ-13 @slice-215
  Scenario: Under push.mode pr, a handed-back draft whose head was not asked about is review-unrequested, and the reviewer's own review of the head counts as asked
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<head>","outcome":"done","head":"<head>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: review-unrequested; next: driver request; run: itos review request slice-9" exactly once
    Given the recording gh answers that the pull request has a comment by "chatgpt-codex-connector" holding Codex's review state, its code review "completed" of the clone's HEAD and its security review "completed" of the commit "0123456789abcdef0123456789abcdef01234567"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "review-unrequested"
    And its JSON's "observed.unrequested" is ["chatgpt-codex-connector"]
    Given the recording gh answers that the pull request has a comment by "ana" requesting "@codex security review" of "chatgpt-codex-connector" at the clone's HEAD
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: handed-back; next: driver land; run: itos push --ready" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "handed-back"
    And its JSON's "observed.unrequested" is []

  # The request is the change's, never the spec's: a todo item's draft
  # with no verdict stays in spec review, its reviewer the itos reviewer,
  # though observed says Codex was not asked about the head. Fails before
  # the work at observed.unrequested: slice-209's observed has no such key.
  @ID-REVREQ-14 @slice-215
  Scenario: Under push.mode pr, a todo item's draft stays in spec review whatever the reviewers were asked
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          reviewers:
            - { login: chatgpt-codex-connector, kind: codex }
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's "next" is {"actor": "reviewer", "action": "review", "argv": ["itos", "brief", "slice-9", "--role", "reviewer"]}
    And its JSON's "observed.unrequested" is ["chatgpt-codex-connector"]
