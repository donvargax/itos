@phase-1
Feature: itos review post records a reviewer's verdict on an item's pull request, bound to the head it judged
  The v9 deterministic coordinator's pilot (docs/plans/v9-coordinator.md,
  its question 3 settled 2026-10-30): a reviewer's verdict on an item is a
  fact itos next derives spec review from, so it must be written where every
  session reads it and bound to what was judged, never trusted from prose.
  Today a reviewer's verdict is a report in a session's hand-off, which the
  coordinator reads and remembers; nothing records which head it judged, so
  a drafter's push after the review passes as reviewed. Wanted: one command
  that records the verdict as an append-only pull request marker, a readable
  line for people and hidden JSON for programs, counted only from the people
  the repository names, and refused when the head it names is not the pull
  request's any more.

  itos review post <item> --verdict approve|changes --at <commit> [--stage
  spec|diff] [--body-file <path>], under push.mode pr, from any branch or a
  detached HEAD (the item names the branch), posts one plain issue comment
  on the item's pull request, the open pull request of the branch named for
  the item: not a GitHub review, since the person cannot approve their own
  pull request. Its body is the line "itos: <stage> review of <item> at
  <the head's first 7 characters>: <verdict>", then, with --body-file, an
  empty line and the file's text (its trailing line ends dropped, \r\n read
  as \n), then an empty line and the marker "<!-- itos:v1
  {"record":"verdict","item":"<item>","stage":"<stage>","verdict":"<verdict>","head":"<the full SHA>","round":<n>} -->",
  the JSON's keys in that order and nothing else in it. --stage is spec by
  default; diff is the review of the implementation. It says "Posted the
  <stage> review of <item> at <the first 7 characters>: <verdict>, round
  <n>, on <address>." Exit 0, outcome posted. --json:
  {"schema":1,"ok":true,"outcome":"posted","item","stage","verdict","head","round","comment"},
  head the full SHA, comment the posted comment's address as GitHub gives it
  (html_url).

  The head. --at names the commit judged: a full SHA (40 hex digits) is
  taken as it is, anything else (an abbreviated SHA, HEAD, HEAD~1, a
  branch) is what git resolves it to in the clone, so a reviewer who fetched
  the branch may say HEAD. One git cannot resolve is refused before any gh
  call (review-unknown-commit, below). The commit must be the pull request's
  current head: one that is not, because the branch was pushed again after
  the review, is refused with nothing posted, rule review-head-moved,
  "itos: <item>'s pull request <address> is at <the new full SHA>, not <the
  full SHA given>; nothing was posted.", the fix "review <the new head's
  first 7 characters> and run itos review post <item> --verdict <verdict>
  --at <the new full SHA>". So a verdict never names a head nobody can read
  the marker against.

  The round, and already. itos reads the pull request's comments as --ready
  reads them (push-mode-draft.feature; the same gh api graphql question,
  paged past a hundred comments) and counts the verdict markers of the same
  item and stage whose comment's author is a login among the people the
  config names (work.people; compared regardless of case, as GitHub's
  logins are, so a people file's Ana is GitHub's ana): the new marker's
  round is one more than that count. A marker by any other login, a marker
  of another stage or item, a comment whose marker is not JSON or whose
  record is not verdict (no error: it is prose), itos's review-request
  marker ("<!-- itos-review-request:v1 ... -->") and any prose, whatever
  words it holds, count for nothing. The markers are append-only and the
  latest counted one, in GitHub's order, is the verdict that stands: when
  it records this verdict for this head and stage, nothing is posted, "The
  <stage> review of <item> at <the first 7 characters>: <verdict> already,
  round <its round>, on <address>; nothing was posted.", exit 0, outcome
  already, --json's round that marker's and no comment member. Any other
  latest marker, another verdict at this head or any verdict at another
  head, makes no already: the verdict is posted as the next round, so
  approve, then changes, then approve again at one head are rounds 1, 2
  and 3, and a reader takes the last. With no people (a stealth config,
  which names none, or a people file missing or unreadable) every login
  counts, as the work registry reads such a repository (the person being
  the only one). Reading the markers back, for itos next, is a later slice.

  The gh calls, each through the gh on the PATH and the repository push
  names (ci.range.github.repository_env, else the remote's URL): gh pr list
  --repo <repository> --head <item> --state open --json number,url for the
  pull request, as push threads asks; --ready's one gh api graphql
  question (-f owner=<owner> -f name=<name> -F number=<number>; push's
  reviewQuery, which gains the pull request's headRefOid and moves beside
  providers.BranchPulls as the question both commands put, since a slice
  imports no other slice, so --ready and review post read one answer) for
  the head and the comments, each author's login and body, asked again for
  each further page; then gh api
  repos/<owner>/<name>/issues/<number>/comments -f body=<the body>, which
  posts the comment and answers it, its html_url the address --json gives.
  It fetches, rebases and pushes nothing, asks no identity provider (the
  comment's author is whoever gh is signed in as), and posts no GitHub
  review and no pr comment. A gh that is missing or fails exits 3, rule
  review-pull-request-failed, naming the call and gh's words ("gh pr list
  failed: HTTP 401: Bad credentials"), nothing posted.

  Refused, exit 1, nothing posted, each a problem of the house JSON under
  --json, {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]},
  and the message on stderr without it. Before any gh call: outside
  push.mode pr (trunk, no config, or one that cannot be read),
  review-without-pr-mode, "itos review: a verdict goes on an item's pull
  request, and this config does not set push.mode: pr; nothing was
  posted."; an item the registry does not have, work-unknown-item, as itos
  work show refuses it; a --verdict that is neither approve nor changes,
  exactly those words, review-bad-verdict, naming the word given; a
  --body-file that cannot be read, review-body-file, naming the path; an
  --at git cannot resolve, review-unknown-commit, naming it. Once gh pr
  list answers: no open pull request for the item, review-no-pull-request,
  "itos review: <item> has no open pull request; itos push opens it.
  Nothing was posted."; and review-head-moved, above.

  The words. review is a new command, post its only word. A usage error,
  exit 2, asking gh nothing: review alone or with another word, post with
  no item or two, without --verdict or without --at, --stage with a word
  that is neither spec nor diff, --body-file with no path, or any other
  flag. A bad --verdict is exit 1 with a problem, not exit 2, because a
  driver computes the verdict from a reviewer's answer and reads the
  problem JSON to say what came back; --stage is written by hand in the
  driver's own command line, so a wrong one is a usage error as any flag's
  value is. itos --help lists "review post <item> --verdict approve|changes
  --at <commit> [--stage spec|diff] [--body-file <path>]" and itos help
  review says what the marker is and what each refusal means.

  Not in this slice: reading the markers for itos next; a verdict on a
  merged or closed pull request (no open one is review-no-pull-request); a
  reviewer naming the pull request by its number; a comment past the
  pages --ready reads (commentPages); editing or deleting a marker;
  trunk mode, which has no pull request to mark.

  The steps. The recording gh is push-mode.feature's and learns: a before
  file answering an "api repos/<owner>/<name>/issues/<n>/comments -f
  body=..." call with {"id":1,"html_url":"<the address of the pull request
  numbered n>#issuecomment-1"}, exit 0, before any handler runs; a nodes
  file, nodes/06-head-ref-oid.sh, giving the pull request's "headRefOid":
  it maps the number asked to its branch through gh-pull-requests, as
  05-pull-request.sh does, and rev-parses refs/heads/<branch> in $REMOTE,
  so the head is the remote's at each call; "the recording gh answers that
  the pull request has a comment by <login> recording the <stage> review
  of <item> at the head of the remote's branch <branch> as round <n>:
  <verdict>", a comment node with that author and the body itos writes,
  its full SHA "@HEAD:<branch>@" and its 7 characters "@SHORT:<branch>@",
  both replaced by the branch's head on the remote at each call (the first
  as the comment heads of slice 200 are, the second by a nodes file of its
  own before them); the same ending "at the commit <sha> as round <n>:
  <verdict>", with that SHA and its first 7 characters written in; the
  same beginning "the recording gh answers that the pull request has, on a
  second page of comments, a comment by <login> recording ...", the node
  on the second page (gh-comments-page2, as slice 200's second-page step
  puts one); "the recording gh answers that the pull request has a comment
  by <login> saying <text>", a comment node with that author and body;
  "the recording gh's <command> call has the field "body" holding these
  lines, @HEAD:<branch>@ the full SHA of the head of the remote's branch
  <branch> and @SHORT:<branch>@ its first 7 characters:", a doc string,
  which takes the first call starting with the command, the text after
  its " -f body=" to the end of the recorded line (so body must be the
  call's last field, which the step's doc comment says), and fails unless
  it is the doc string's lines joined by the two characters \n (as
  dispatch.sh records a line end), the two placeholders replaced; and "its
  JSON's <key> holds the full SHA of the head of the remote's branch
  <branch>" (holds, not is: "its JSON's <key> is <value>" is promote's step,
  which godog would read this one as too).

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: one comment, its readable line and its marker exact,
  # the head the full SHA, round 1 on a pull request with no marker; a
  # plain issue comment, no GitHub review, no pr comment, nothing pushed.
  # Fails before the work at the exit code: today review is no command,
  # exit 2.
  @ID-REVIEW-01 @wip @slice-206
  Scenario: Under push.mode pr, review post approve at the pull request's head posts the readable line and the marker as round 1
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "review post slice-9 --verdict approve --at HEAD"
    Then itos exits with code 0
    And its output says "Posted the spec review of slice-9 at "
    And its output says ": approve, round 1, on https://github.example/o/r/pull/7."
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "api graphql" call does not have "mutation"
    And the recording gh's "api repos/owner/scratch/issues/7/comments" call has the field "body" holding these lines, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9" and @SHORT:slice-9@ its first 7 characters:
      """
      itos: spec review of slice-9 at @SHORT:slice-9@: approve

      <!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"@HEAD:slice-9@","round":1} -->
      """
    And the recording gh was called as "api repos/owner/scratch/issues/7/comments" once
    And the recording gh was never called as "pr comment"
    And the recording gh was never called as "pr review"
    And the recording gh was never called as "pr create"
    And the clone's HEAD is the head of the remote's branch "slice-9"

  # --json is what a driver reads: every field, the head full, the comment's
  # address as GitHub answers it. From main, where the reviewer's clone
  # sits: the item names the branch. --stage diff and --body-file: the
  # file's text sits between the line and the marker, its trailing line end
  # dropped. Fails before the work at the exit code: today review is no
  # command, exit 2.
  @ID-REVIEW-02 @wip @slice-206
  Scenario: Under push.mode pr, review post --stage diff --body-file --json from main posts the file's text between the line and the marker
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the file "review.md" holds the lines:
      """
      P2: the help says less than the code.
      The rest reads well.
      """
    When itos runs "review post slice-9 --verdict changes --at HEAD --stage diff --body-file review.md --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "posted"
    And its JSON's "item" is "slice-9"
    And its JSON's "stage" is "diff"
    And its JSON's "verdict" is "changes"
    And its JSON's "head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "round" is 1
    And its JSON's "comment" is "https://github.example/o/r/pull/7#issuecomment-1"
    And the recording gh's "api repos/owner/scratch/issues/7/comments" call has the field "body" holding these lines, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9" and @SHORT:slice-9@ its first 7 characters:
      """
      itos: diff review of slice-9 at @SHORT:slice-9@: changes

      P2: the help says less than the code.
      The rest reads well.

      <!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"diff","verdict":"changes","head":"@HEAD:slice-9@","round":1} -->
      """

  # The round counts the people's spec markers of this item alone: someone's
  # at an earlier head and ana's at this one make two, so this is round 3;
  # the people file spells her Ana, as GitHub's logins ignore case. Not
  # counted: someone's diff marker, someone's spec marker of slice-8, the
  # marker by a login the people file does not list (which also records
  # approve at this head, and is the latest, so a build counting it would
  # say already), the review-request marker, a marker that is not JSON (no
  # error) and ana's prose that says approve. Fails before the work at the
  # exit code: today review is no command, exit 2.
  @ID-REVIEW-03 @wip @slice-206
  Scenario: Under push.mode pr, the round is one more than the people's verdict markers for the item and stage, markers of outsiders, other items, other stages and prose not counted
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "Ana" and "someone"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-9" at the commit "0123456789abcdef0123456789abcdef01234567" as round 1: "changes"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 2: "changes"
    And the recording gh answers that the pull request has a comment by "someone" recording the "diff" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-8" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has a comment by "ana" saying "I approve of this spec; the second round reads well."
    And the recording gh answers that the pull request has a comment by "ana" saying "<!-- itos:v1 not json -->"
    And the recording gh answers that the pull request has a comment by "copilot-pull-request-reviewer" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 3: "approve"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "posted"
    And its JSON's "round" is 3
    And the recording gh's "api repos/owner/scratch/issues/7/comments" call has the field "body" holding these lines, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9" and @SHORT:slice-9@ its first 7 characters:
      """
      itos: spec review of slice-9 at @SHORT:slice-9@: approve

      <!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"@HEAD:slice-9@","round":3} -->
      """

  # Run twice, the second posts nothing: a driver that reran the reviewer
  # must not double the record. The marker is ana's, a person; the JSON has
  # no comment, since none was posted. Fails before the work at the exit
  # code: today review is no command, exit 2.
  @ID-REVIEW-04 @wip @slice-206
  Scenario: Under push.mode pr, the same verdict for the same head and stage again posts nothing and reports already
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "already"
    And its JSON's "verdict" is "approve"
    And its JSON's "round" is 1
    And its JSON has no "comment"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"
    And the recording gh was never called as "pr comment"
    When itos runs "review post slice-9 --verdict approve --at HEAD"
    Then itos exits with code 0
    And its output says "The spec review of slice-9 at "
    And its output says ": approve already, round 1, on https://github.example/o/r/pull/7; nothing was posted."

  # A reviewer that changes its mind at the same head: the markers are
  # append-only, so the new verdict is a new round and the later marker is
  # the one a reader takes. Fails before the work at the exit code: today
  # review is no command, exit 2.
  @ID-REVIEW-05 @wip @slice-206
  Scenario: Under push.mode pr, a different verdict at the same head posts a new marker as the next round
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "review post slice-9 --verdict changes --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "posted"
    And its JSON's "round" is 2
    And the recording gh's "api repos/owner/scratch/issues/7/comments" call has the field "body" holding these lines, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9" and @SHORT:slice-9@ its first 7 characters:
      """
      itos: spec review of slice-9 at @SHORT:slice-9@: changes

      <!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"changes","head":"@HEAD:slice-9@","round":2} -->
      """

  # Already is the latest counted marker's, not any earlier one's: ana
  # approved at this head, then asked for changes, so approve now is a new
  # verdict, round 3. A build that found the round 1 marker would say
  # already and post nothing. Fails before the work at the exit code: today
  # review is no command, exit 2.
  @ID-REVIEW-13 @wip @slice-206
  Scenario: Under push.mode pr, a verdict an earlier marker records but the latest does not is posted as the next round
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 2: "changes"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "posted"
    And its JSON's "round" is 3
    And the recording gh's "api repos/owner/scratch/issues/7/comments" call has the field "body" holding these lines, @HEAD:slice-9@ the full SHA of the head of the remote's branch "slice-9" and @SHORT:slice-9@ its first 7 characters:
      """
      itos: spec review of slice-9 at @SHORT:slice-9@: approve

      <!-- itos:v1 {"record":"verdict","item":"slice-9","stage":"spec","verdict":"approve","head":"@HEAD:slice-9@","round":3} -->
      """

  # With no people file every login is a person, as the work registry
  # reads such a repository: the outsider's marker is the latest and
  # records this verdict at this head, so nothing is posted. Fails before
  # the work at the exit code: today review is no command, exit 2.
  @ID-REVIEW-14 @wip @slice-206
  Scenario: With no people file, a marker by any login counts, so the same verdict at the head is already
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file is missing
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "copilot-pull-request-reviewer" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "already"
    And its JSON's "round" is 1
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"

  # A pull request with more than a hundred comments has its markers past
  # the first page: they are read as --ready reads them, so the marker on
  # the second page is found and nothing is posted twice. Fails before the
  # work at the exit code: today review is no command, exit 2.
  @ID-REVIEW-15 @wip @slice-206
  Scenario: Under push.mode pr, a marker on the second page of comments is read, so the same verdict there is already
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has, on a second page of comments, a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "already"
    And its JSON's "round" is 1
    And the recording gh's "api graphql" call has "cursor=page2"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"

  # The slice's other point: the branch was pushed again after the review,
  # so the commit judged is no longer the pull request's head; a verdict
  # for it would be read against a head nobody reviewed. The pull request's
  # head is the remote's, which gh gives, not the one the pull request was
  # opened at. A full SHA the clone has no commit for is taken as given
  # and judged the same way, not refused as unknown. Fails before the work
  # at the exit code: today review is no command, exit 2.
  @ID-REVIEW-06 @wip @slice-206
  Scenario: Under push.mode pr, review post --at a commit that is no longer the pull request's head is refused, naming the new head
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone has the commit "docs: answer the review" touching "features/widget.feature"
    And git pushes HEAD to the remote's new branch "slice-9"
    When itos runs "review post slice-9 --verdict approve --at HEAD~1 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-head-moved"'s "message" says "slice-9's pull request https://github.example/o/r/pull/7 is at "
    And its output says "is at " with the full SHA of the head of the remote's branch "slice-9"
    And its output says "--at " with the full SHA of the head of the remote's branch "slice-9"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"
    And the recording gh was never called as "pr comment"
    When itos runs "review post slice-9 --verdict approve --at 0123456789abcdef0123456789abcdef01234567 --json"
    Then itos exits with code 1
    And its JSON's problem "review-head-moved"'s "message" says "not 0123456789abcdef0123456789abcdef01234567"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"

  # A branch not pushed yet has no pull request to mark; the words say what
  # opens one, and nothing is asked past gh pr list. Fails before the work
  # at the exit code: today review is no command, exit 2.
  @ID-REVIEW-07 @wip @slice-206
  Scenario: Under push.mode pr, review post with no open pull request for the item is refused, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-no-pull-request"'s "message" says "slice-9 has no open pull request"
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh was never called as "api graphql"
    And the recording gh was never called as "api repos/owner/scratch/issues/7/comments"
    And the recording gh was never called as "pr create"

  # A gh that fails is an environment failure, exit 3, naming the call and
  # gh's words; nothing was posted. Fails before the work at the exit code:
  # today review is no command, exit 2.
  @ID-REVIEW-08 @wip @slice-206
  Scenario: Under push.mode pr, review post with a gh that fails exits 3, naming the call
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh fails every call, saying "HTTP 401: Bad credentials"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 3
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-pull-request-failed"'s "message" says "gh pr list failed: HTTP 401: Bad credentials"

  # What is wrong before gh is asked is refused without asking it: a word
  # that is no verdict (exactly approve or changes), an item the registry
  # lacks, a body file that cannot be read, a commit git cannot resolve.
  # Each names what it refused. Fails before the work at the exit code:
  # today review is no command, exit 2.
  @ID-REVIEW-09 @wip @slice-206
  Scenario Outline: Under push.mode pr, review post with a bad verdict, an unknown item, an unreadable body file or an unknown commit is refused before gh is asked
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    When itos runs "<command> --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "<rule>"'s "message" says "<named>"
    And the recording gh was never called

    Examples:
      | command                                                                | rule                  | named      |
      | review post slice-9 --verdict reject --at HEAD                         | review-bad-verdict    | reject     |
      | review post slice-9 --verdict Approve --at HEAD                        | review-bad-verdict    | Approve    |
      | review post slice-7 --verdict approve --at HEAD                        | work-unknown-item     | slice-7    |
      | review post slice-9 --verdict approve --at HEAD --body-file missing.md | review-body-file      | missing.md |
      | review post slice-9 --verdict approve --at nosuchref                   | review-unknown-commit | nosuchref  |

  # review has one word, post, in one form; anything else is a usage error
  # that asks gh nothing. A guard: every row is exit 2 before the work too,
  # review being no command, and must stay one after it.
  @ID-REVIEW-10 @wip @slice-206 @guard
  Scenario Outline: Under push.mode pr, review in any form but post with an item, --verdict and --at is a usage error, exit 2
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    When itos runs "<command>"
    Then itos exits with code 2
    And the recording gh was never called

    Examples:
      | command                                                         |
      | review                                                          |
      | review list                                                     |
      | review post                                                     |
      | review post slice-9 --at HEAD                                   |
      | review post slice-9 --verdict approve                           |
      | review post slice-9 slice-8 --verdict approve --at HEAD         |
      | review post slice-9 --verdict approve --at HEAD --stage code    |
      | review post slice-9 --verdict approve --at HEAD --body-file     |
      | review post slice-9 --verdict approve --at HEAD --ready         |

  # Under trunk there is no pull request to mark: refused, gh never asked,
  # whatever branch the clone is on. Fails before the work at the exit
  # code: today review is no command, exit 2.
  @ID-REVIEW-11 @wip @slice-206
  Scenario: Outside push.mode pr, review post is refused, exit 1, asking gh nothing
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "docs: specify slice-9" touching "features/widget.feature"
    When itos runs "review post slice-9 --verdict approve --at HEAD --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "review-without-pr-mode"'s "message" says "push.mode: pr"
    And the recording gh was never called

  # The help is where a reviewer looks first. Fails before the work at the
  # first output step: today itos --help has no review command.
  @ID-REVIEW-12 @wip @slice-206
  Scenario: itos's help lists review post and help review documents its flags
    When itos runs "--help"
    Then itos exits with code 0
    And its output says "review post <item> --verdict approve|changes --at <commit>", whatever its line breaks
    When itos runs "help review"
    Then itos exits with code 0
    And its output says "[--stage spec|diff] [--body-file <path>]", whatever its line breaks
    And its output says "itos:v1", whatever its line breaks
