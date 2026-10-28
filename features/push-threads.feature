@phase-1
Feature: Under push.mode pr, itos push threads lists an item's open review threads and itos push thread replies to one and resolves it
  Under push.mode pr every review thread on an item's pull request must be
  answered and resolved before itos push --ready turns on auto-merge
  (slice 179, push-mode-draft.feature), and AGENTS.md's Finishing asks every
  implementer to do it for every thread. gh has no command for either: the
  thread's node id has to be read out of gh api graphql, then a reply and a
  resolve posted through two hand-written gh api graphql mutations naming
  it. T-328 (2026-10-27, #196) did that by hand for Codex's P1 thread.
  Wanted: one command to list the open threads with their ids, and one to
  answer a thread, and --ready's refusal pointing at both.

  itos push threads, from an item's branch under push.mode pr (draft or
  not), lists the review threads of the branch's open pull request that are
  not resolved, in GitHub's order: first "<address> has <n> unresolved
  review threads:", then one line each, "  <id> <path>:<line> <author>:
  <first line>", the thread's node id, where it is, the login of its first
  comment's author as GraphQL gives it (a bot's without "[bot]") and the
  first line of that comment, trimmed ("  <id> <path> <author>: ..." for an
  outdated thread, which GitHub gives no line, as --ready names one). With
  none open: "<address> has no unresolved review threads." Exit 0, outcome
  threads-listed. It fetches, rebases and pushes nothing, and posts nothing.
  --json: {"schema":1,"ok","outcome","pull_request"?,"threads"?},
  pull_request {"url","number"}, threads a list of {"id","path","line",
  "author","comment"}, comment the first line as above, line left out for an
  outdated thread, [] for none.

  itos push thread <id> --reply <text> [--resolve], from the same place,
  answers one thread of that pull request: it asks the pull request's
  threads first, and an id that is none of them, resolved or not, is
  refused, exit 1, outcome no-such-thread, "itos push: <id> is not a review
  thread of <address>; itos push threads lists its open ones.", nothing
  posted. Otherwise it posts the reply (the addPullRequestReviewThreadReply
  mutation, the thread's id and the text) and says "Replied to <id> on
  <address>."; with --resolve it then resolves the thread (the
  resolveReviewThread mutation) and says "Resolved <id> on <address>.". The
  reply and the resolve are two calls, the reply first, so a reply that
  fails leaves the thread open. --resolve without --reply resolves alone,
  posting no reply. Exit 0, outcome thread-answered. --json: {"schema":1,
  "ok","outcome","pull_request"?,"thread"?}, thread {"id","replied",
  "resolved"}.

  Both go through gh as every other GitHub call of push's does: gh pr list
  --repo <repository> --head <branch> --state open finds the pull request,
  and gh api graphql (no --repo; -f owner=<owner> -f name=<name> -F
  number=<number>, as --ready asks) its threads, with each thread's id and
  its first comment's author beside what --ready reads, the query on one
  line; the mutations are gh api graphql calls too, each on one line. A gh
  that is missing or fails exits 3, outcome pull-request-failed, naming the
  gh command and its words ("gh pr list failed: HTTP 401: Bad credentials"),
  and never saying the branch is pushed, since nothing was. The repository
  is the one push names (ci.range.github.repository_env, else the remote's
  URL; none, exit 3, no-github-repository).

  Where they have nothing to do, a refusal, exit 1, before any gh call:
  outside push.mode pr (trunk, no config, or one that cannot be read),
  outcome threads-without-pr-mode, "itos push: review threads are an item's
  pull request's, and this config does not set push.mode: pr; nothing was
  posted."; on main (the branch the remote's HEAD names, asked as push asks
  it), outcome threads-on-main, "itos push: HEAD is on <main>, which has no
  pull request; run it from the item's branch. Nothing was posted."; on a
  detached HEAD, outcome detached, as push refuses one. With no open pull
  request for the branch, once gh pr list says so, outcome no-pull-request,
  "itos push: <branch> has no open pull request; itos push opens it.
  Nothing was posted."

  The words. push took no positional argument before this slice; threads
  and thread are its only two, each first among the arguments. Any other
  positional stays a usage error, exit 2, and so is thread with no id, with
  two, or with neither --reply nor --resolve, --reply with no text,
  threads with an id, --reply or --resolve without thread, and either word
  beside --no-wait, --ready, --tag or --ref. itos push --help gives both in
  its usage, "itos push threads" and "itos push thread <id> --reply <text>
  [--resolve]", and says what each does; command.go's help and doc.go say
  it, and --json's outcomes gain the ones above.

  Slice 201 (was p2-push-thread-show). On #214 an implementer read Codex's
  whole finding with gh api graphql, since push threads gives a thread's
  first line only; on #210 the coordinator, on main, ruling on the threads
  an implementer left open, could use neither word there. A thread's node
  id names its own pull request, so push thread no longer asks the
  branch's: from any branch, main included, or a detached HEAD, it asks
  GitHub for the node itself, one gh api graphql call (-f query=<one line>
  -f id=<id>) giving, on a PullRequestReviewThread, its id, isResolved,
  path and line, its repository's nameWithOwner, its pull request's number
  and url, and its first 100 comments, each author's login and body. A node
  that is none, or no review thread, or a thread of another repository than
  the one push names (compared regardless of case, as GitHub's names are),
  is refused, exit 1, outcome no-such-thread, "itos push: <id> is not a
  review thread of <repository>; itos push threads lists a pull request's
  open ones. Nothing was posted.", with nothing posted; else the reply and
  the resolve go as before, naming the node's pull request. No gh pr list
  is asked for it.

  With neither --reply nor --resolve, itos push thread <id> shows the
  thread, resolved or not, posting nothing: "<id> <path>:<line> on
  <address>, unresolved:" ("resolved:" for a resolved one, "<id> <path> on
  ..." for an outdated one), then each comment, oldest first: its author's
  login and a colon on a line of its own, then each line of its body
  indented by two spaces, a line break of \r\n read as one of \n and an
  author GitHub no longer has shown as ghost. Exit 0, outcome thread-shown.
  --json: {"schema":1,"ok","outcome","pull_request"?,"thread"?}, thread
  {"id","path","line"?,"resolved","comments"}, comments a list of
  {"author","body"}, each body whole as GitHub gives it.

  itos push threads --pr <n> (--pr=<n> too) lists the open threads of the
  repository's pull request n, from any branch or a detached HEAD, asking
  no gh pr list: the same gh api graphql question, the pull request's
  number and url beside its threads, gives the address it names. A pull
  request GitHub does not have is its error, a gh that fails, exit 3.
  Without --pr, push threads keeps slice 191's rule, and on main its
  refusal reads "itos push: HEAD is on <main>, which has no pull request;
  run it from the item's branch, or name the pull request with --pr <n>.
  Nothing was posted." Both words still need push.mode pr. A --pr with no
  number, one that is not a whole number above 0, given twice, given to
  push thread, or given to push without threads, is a usage error, exit 2.
  The help's usage gains "itos push threads [--pr <n>]" and "itos push
  thread <id>", and says the show form shows the thread in full.

  This changes what slice 191's scenarios promised: ID-PUSHTHREAD-07, which
  ID-PUSHTHREAD-19 replaces since its words named the branch's pull request,
  goes; ID-PUSHTHREAD-08 and ID-PUSHTHREAD-10 lose their push thread rows
  (push thread is not refused on main and asks for no branch's pull
  request); ID-PUSHTHREAD-14 loses its row "push thread PRRT_1", the show
  form now. The commit that changes them names each in Changes:.

  --ready's refusal on unresolved threads (push-mode-draft.feature, step 5)
  keeps naming each thread and its "Address each ..." line, and adds after
  them "itos push threads lists them with their ids, and itos push thread
  <id> --reply <text> --resolve answers one." The implementer rewrites
  AGENTS.md's Finishing (step 3) to name the two commands, in a docs commit.

  Not in this slice: a pull request with more than 100 review threads, as
  --ready; editing or deleting a reply; unresolving a thread; replying to a
  pull request's comment that is not a review thread; listing the threads of
  another branch's pull request by its branch; a thread with more than 100
  comments (slice 201).

  The steps. The recording gh is push-mode.feature's, and learns two things.
  A thread node the steps give it may carry an id and its first comment's
  author: "the recording gh answers that the pull request has an unresolved
  (or a resolved) review thread <id> on <path> line <n> by <login>, its first
  comment the lines <first> and <second>", a thread node with "id", that
  isResolved, path and line, and one comment whose author's login is the
  one given and whose body is the two lines joined by a newline. Its api
  graphql answers a call that has "addPullRequestReviewThreadReply" with
  {"data":{"addPullRequestReviewThreadReply":{"comment":{"url":
  "https://github.example/o/r/pull/7#discussion_r1"}}}} and one that has
  "resolveReviewThread" with {"data":{"resolveReviewThread":{"thread":
  {"isResolved":true}}}}, every other api graphql call as before. New check:
  "the recording gh's call with <a> comes before its call with <b>", which
  fails when no call has either text, or the first call with b comes before
  the first with a.

  Slice 201's. The recording gh's api graphql answers a call with an
  argument id=<id> (the node question) with {"data":{"node":<node>}}: for a
  thread a step gave with that id, its node as the pull request's list has
  it, with "pullRequest" {"number","url"} of the pull request the steps gave
  the recording gh last before that thread, and "repository"
  {"nameWithOwner":"owner/scratch"}, the fake's; for any other id, null. Its
  answer to the pull request's question gains that pull request's "number"
  and "url" when it holds one whose address ends with the number asked.
  New steps: "the recording gh answers that the review thread <id> has a
  reply by <login> saying <text>", which adds a comment with that author and
  body after the thread's others, in both answers; and "the recording gh
  answers, asked for the node <id>, an unresolved review thread of the pull
  request <address> in the repository <owner/name> on <path> line <n> by
  <login>, its first comment the lines <first> and <second>", a node the
  node question alone answers, in no pull request's list.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"

  # The slice's point: the open threads, each with the id an answer names,
  # its place, its author and its first comment's first line; a resolved
  # one is left out, and nothing is pushed. Fails before the work at the
  # exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-01 @slice-191
  Scenario: Under push.mode pr, push threads lists the open review threads of the branch's pull request with their ids, pushing nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the pull request has a resolved review thread "PRRT_2" on "gadget.md" line 1 by "copilot-pull-request-reviewer", its first comment the lines "Fine now." and "Thanks."
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_3" on "widget.md" line 8 by "chatgpt-codex-connector", its first comment the lines "P1: this panics on an empty widget." and "Check the length first."
    When itos runs "push threads"
    Then itos exits with code 0
    And its output says "https://github.example/o/r/pull/7 has 2 unresolved review threads"
    And its output says "PRRT_1 widget.md:3 copilot-pull-request-reviewer: Rename this."
    And its output says "PRRT_3 widget.md:8 chatgpt-codex-connector: P1: this panics on an empty widget."
    And its output does not say "PRRT_2"
    And its output does not say "It clashes with the gadget."
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "api graphql" call does not have "mutation"
    And the remote has no branch "slice-9"

  # --json is what an agent reads: the ids to answer, exactly these fields.
  # Fails before the work at the exit code: today push takes no argument,
  # exit 2.
  @ID-PUSHTHREAD-02 @slice-191
  Scenario: Under push.mode pr, push threads --json gives each open thread's id, path, line, author and first comment line
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the pull request has a resolved review thread "PRRT_2" on "gadget.md" line 1 by "copilot-pull-request-reviewer", its first comment the lines "Fine now." and "Thanks."
    When itos runs "push threads --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "threads-listed"
    And its JSON's "pull_request.url" is "https://github.example/o/r/pull/7"
    And its JSON's "threads" is [{"id":"PRRT_1","path":"widget.md","line":3,"author":"copilot-pull-request-reviewer","comment":"Rename this."}]

  # Every thread answered is the state --ready waits for: said, exit 0.
  # Fails before the work at the exit code: today push takes no argument,
  # exit 2.
  @ID-PUSHTHREAD-03 @slice-191
  Scenario: Under push.mode pr, push threads with every thread resolved says none is open, exit 0
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a resolved review thread "PRRT_2" on "gadget.md" line 1 by "copilot-pull-request-reviewer", its first comment the lines "Fine now." and "Thanks."
    When itos runs "push threads"
    Then itos exits with code 0
    And its output says "https://github.example/o/r/pull/7 has no unresolved review threads."
    And its output does not say "PRRT_2"

  # The coordinator's case: on main, ruling on the threads an implementer
  # left open, it names the pull request; no branch's is looked for. Fails
  # before the work at the exit code: today threads takes no argument,
  # exit 2.
  @ID-PUSHTHREAD-15 @wip @slice-201
  Scenario: Under push.mode pr, push threads --pr <n> on main lists that pull request's open review threads, asking no gh pr list
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the pull request has a resolved review thread "PRRT_2" on "gadget.md" line 1 by "copilot-pull-request-reviewer", its first comment the lines "Fine now." and "Thanks."
    When itos runs "push threads --pr 7"
    Then itos exits with code 0
    And its output says "https://github.example/o/r/pull/7 has 1 unresolved review thread"
    And its output says "PRRT_1 widget.md:3 copilot-pull-request-reviewer: Rename this."
    And its output does not say "PRRT_2"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "api graphql" call does not have "mutation"
    And the recording gh was never called as "pr list"

  # The slice's first point: a thread read in full, every comment's author
  # and whole body, oldest first, without gh api graphql by hand; from
  # main, as the coordinator reads one. Fails before the work at the exit
  # code: today push thread with neither --reply nor --resolve is a usage
  # error, exit 2.
  @ID-PUSHTHREAD-16 @wip @slice-201
  Scenario: Under push.mode pr, push thread <id> with neither --reply nor --resolve shows the thread in full on main, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the review thread "PRRT_1" has a reply by "someone" saying "The name follows the gadget spec."
    When itos runs "push thread PRRT_1"
    Then itos exits with code 0
    And its output says "PRRT_1 widget.md:3 on https://github.example/o/r/pull/7, unresolved:"
    And its output says "copilot-pull-request-reviewer:\n  Rename this.\n  It clashes with the gadget.\nsomeone:\n  The name follows the gadget spec.", its backslash escapes read as Go's
    And the recording gh's "api graphql" call has "PRRT_1"
    And the recording gh's "api graphql" call does not have "mutation"
    And the recording gh was never called as "pr list"

  # --json is what an agent reads: each comment whole, and a resolved
  # thread shown as one. Fails before the work at the exit code: today push
  # thread with neither --reply nor --resolve is a usage error, exit 2.
  @ID-PUSHTHREAD-17 @wip @slice-201
  Scenario: Under push.mode pr, push thread <id> --json gives a resolved thread's place, state and every comment's author and body
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a resolved review thread "PRRT_2" on "gadget.md" line 1 by "copilot-pull-request-reviewer", its first comment the lines "Fine now." and "Thanks."
    And the recording gh answers that the review thread "PRRT_2" has a reply by "someone" saying "Agreed."
    When itos runs "push thread PRRT_2 --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "thread-shown"
    And its JSON's "pull_request" is {"url":"https://github.example/o/r/pull/7","number":7}
    And its JSON's "thread" is {"id":"PRRT_2","path":"gadget.md","line":1,"resolved":true,"comments":[{"author":"copilot-pull-request-reviewer","body":"Fine now.\nThanks."},{"author":"someone","body":"Agreed."}]}
    And the recording gh's "api graphql" call does not have "mutation"

  # The slice's other point: one command replies and resolves, the reply
  # first, so a reply that fails leaves the thread open. Fails before the
  # work at the exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-04 @slice-191
  Scenario: Under push.mode pr, push thread <id> --reply <text> --resolve posts the reply to the thread, then resolves it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_1 --reply 'Renamed it to gizmo.' --resolve"
    Then itos exits with code 0
    And the recording gh's "api graphql" call has "addPullRequestReviewThreadReply"
    And the recording gh's "api graphql" call has "Renamed it to gizmo."
    And the recording gh's "api graphql" call has "PRRT_1"
    And the recording gh's "api graphql" call has "resolveReviewThread"
    And the recording gh's call with "addPullRequestReviewThreadReply" comes before its call with "resolveReviewThread"
    And its output says "Replied to PRRT_1 on https://github.example/o/r/pull/7."
    And its output says "Resolved PRRT_1 on https://github.example/o/r/pull/7."

  # A finding the implementer disagrees with gets a reply and stays open
  # (AGENTS.md's Finishing, step 3): no --resolve, no resolve. Fails before
  # the work at the exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-05 @slice-191
  Scenario: Under push.mode pr, push thread <id> --reply <text> without --resolve replies and leaves the thread open
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_1 --reply 'The name follows the gadget spec, so it stays.'"
    Then itos exits with code 0
    And the recording gh's "api graphql" call has "addPullRequestReviewThreadReply"
    And the recording gh's "api graphql" call does not have "resolveReviewThread"
    And its output says "Replied to PRRT_1"
    And its output does not say "Resolved PRRT_1"

  # A fix pushed may say enough on its own: --resolve alone posts no reply.
  # Fails before the work at the exit code: today push takes no argument,
  # exit 2.
  @ID-PUSHTHREAD-06 @slice-191
  Scenario: Under push.mode pr, push thread <id> --resolve without --reply resolves the thread, posting no reply
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_1 --resolve"
    Then itos exits with code 0
    And the recording gh's "api graphql" call has "resolveReviewThread"
    And the recording gh's "api graphql" call does not have "addPullRequestReviewThreadReply"
    And its output says "Resolved PRRT_1 on https://github.example/o/r/pull/7."
    And its output does not say "Replied to"

  # The coordinator rules on a thread an implementer left open from main:
  # the thread's node names its pull request, so no branch's is looked for.
  # Fails before the work at the exit code: today push thread on main is
  # refused, exit 1, outcome threads-on-main.
  @ID-PUSHTHREAD-18 @wip @slice-201
  Scenario: Under push.mode pr, push thread <id> --reply <text> --resolve on main replies to the thread's own pull request, then resolves it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "chatgpt-codex-connector", its first comment the lines "P1: this panics on an empty widget." and "Check the length first."
    When itos runs "push thread PRRT_1 --reply 'Ruled: out of scope, filed as an idea.' --resolve"
    Then itos exits with code 0
    And the recording gh's "api graphql" call has "Ruled: out of scope, filed as an idea."
    And the recording gh's call with "addPullRequestReviewThreadReply" comes before its call with "resolveReviewThread"
    And its output says "Replied to PRRT_1 on https://github.example/o/r/pull/7."
    And its output says "Resolved PRRT_1 on https://github.example/o/r/pull/7."
    And the recording gh was never called as "pr list"

  # An id copied from another pull request, or mistyped, would post where
  # nobody looks: it is checked against this pull request's threads first.
  # Fails before the work at the exit code: today push takes no argument,
  # exit 2.
  @ID-PUSHTHREAD-07 @slice-191
  Scenario: Under push.mode pr, push thread with an id that is not a thread of the branch's pull request is refused, exit 1, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_9 --reply 'Done.' --resolve --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "no-such-thread"
    And its output says "PRRT_9 is not a review thread of https://github.example/o/r/pull/7"
    And the recording gh's "api graphql" call does not have "mutation"

  # ID-PUSHTHREAD-07's case once push thread asks GitHub for the node: an
  # id GitHub has no thread for is refused before anything is posted, the
  # words naming the repository, since no pull request was asked. Fails
  # before the work at the output step: today the words name the branch's
  # pull request.
  @ID-PUSHTHREAD-19 @wip @slice-201
  Scenario: Under push.mode pr, push thread with an id that names no review thread is refused, exit 1, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has an unresolved review thread "PRRT_1" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_9 --reply 'Done.' --resolve --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "no-such-thread"
    And its output says "PRRT_9 is not a review thread of owner/scratch"
    And the recording gh's "api graphql" call has "PRRT_9"
    And the recording gh's "api graphql" call does not have "mutation"

  # A thread id is GitHub's across repositories: one copied from another
  # repository's pull request would post where nobody here looks, so the
  # node's repository is checked first. From main, where the coordinator
  # works. Fails before the work at the "api graphql" step: today push
  # thread on main is refused before gh is asked.
  @ID-PUSHTHREAD-20 @wip @slice-201
  Scenario: Under push.mode pr, push thread with the id of another repository's review thread is refused, exit 1, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh answers, asked for the node "PRRT_7", an unresolved review thread of the pull request "https://github.example/other/r/pull/3" in the repository "other/r" on "widget.md" line 3 by "copilot-pull-request-reviewer", its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push thread PRRT_7 --reply 'Renamed.' --resolve --json"
    Then itos exits with code 1
    And the recording gh's "api graphql" call has "PRRT_7"
    And its JSON's "outcome" is "no-such-thread"
    And its output says "PRRT_7 is not a review thread of owner/scratch"
    And the recording gh's "api graphql" call does not have "mutation"

  # Main has no pull request, so neither word has a thread to read or
  # answer; refused before gh is asked anything. Fails before the work at
  # the exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-08 @slice-191
  Scenario Outline: Under push.mode pr, push threads and push thread on main are refused, exit 1, asking gh nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    When itos runs "<command> --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "threads-on-main"
    And its output says "HEAD is on main, which has no pull request"
    And the recording gh was never called

    Examples:
      | command                                         |
      | push threads                                    |
      | push thread PRRT_1 --reply 'Renamed.' --resolve |

  # On main push threads still has no branch's pull request to read, and
  # its refusal names the flag that reads one anyway. Fails before the work
  # at the "--pr <n>" step: today the refusal names only the item's branch.
  @ID-PUSHTHREAD-21 @wip @slice-201
  Scenario: Under push.mode pr, push threads on main without --pr is refused, exit 1, naming --pr and asking gh nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    When itos runs "push threads --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "threads-on-main"
    And its output says "name the pull request with --pr <n>"
    And the recording gh was never called

  # Under trunk there is no item's pull request to read: refused, gh never
  # asked, whatever branch the clone is on. Fails before the work at the
  # exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-09 @slice-191
  Scenario Outline: Outside push.mode pr, push threads and push thread are refused, exit 1, asking gh nothing
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    When itos runs "<command> --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "threads-without-pr-mode"
    And its output says "push.mode: pr"
    And the recording gh was never called

    Examples:
      | command                                         |
      | push threads                                    |
      | push thread PRRT_1 --reply 'Renamed.' --resolve |

  # A branch not pushed yet has no pull request; the words say what opens
  # one, and no thread is asked or answered. Fails before the work at the
  # exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-10 @slice-191
  Scenario Outline: Under push.mode pr, push threads and push thread with no open pull request for the branch are refused, exit 1, posting nothing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    When itos runs "<command> --json"
    Then itos exits with code 1
    And its JSON's "outcome" is "no-pull-request"
    And its output says "slice-9 has no open pull request"
    And the recording gh's "pr list" call has "slice-9"
    And the recording gh was never called as "api graphql"
    And the recording gh was never called as "pr create"

    Examples:
      | command                                         |
      | push threads                                    |
      | push thread PRRT_1 --reply 'Renamed.' --resolve |

  # A gh that fails is push's exit 3, naming the call and gh's words, but
  # nothing was pushed, so the words must not say the branch is. Fails
  # before the work at the exit code: today push takes no argument, exit 2.
  @ID-PUSHTHREAD-11 @slice-191
  Scenario: Under push.mode pr, push threads with a gh that fails exits 3, naming the call and saying nothing was pushed
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh fails every call, saying "HTTP 401: Bad credentials"
    And the clone is on the new branch "slice-9"
    When itos runs "push threads --json"
    Then itos exits with code 3
    And its JSON's "outcome" is "pull-request-failed"
    And its output says "gh pr list failed: HTTP 401: Bad credentials"
    And its output does not say "is pushed"

  # --ready's refusal keeps naming the threads and points at the commands
  # that answer them, so no implementer reaches for gh api graphql. Fails
  # before the work at the "itos push threads lists them" step: today the
  # refusal ends at "Address each ...".
  @ID-PUSHTHREAD-12 @slice-191
  Scenario: Under push.pr.draft, push --ready's refusal on an unresolved review thread names itos push threads and itos push thread
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
    And the recording gh answers that the pull request has an unresolved review thread on "widget.md" line 3, its first comment the lines "Rename this." and "It clashes with the gadget."
    When itos runs "push --ready"
    Then itos exits with code 1
    And its output says "widget.md:3 Rename this."
    And its output says "itos push threads lists them with their ids"
    And its output says "itos push thread <id> --reply <text> --resolve answers one"
    And the recording gh was never called as "pr merge"

  # The help is where an agent looks first. Fails before the work at the
  # first output step: today the help has neither word.
  @ID-PUSHTHREAD-13 @slice-191
  Scenario: push's help documents push threads and push thread
    When itos runs "help push"
    Then itos exits with code 0
    And its output says "itos push threads", whatever its line breaks
    And its output says "itos push thread <id> --reply <text> [--resolve]", whatever its line breaks

  # The help gives the two new forms. Fails before the work at the first
  # output step: today the usage has push threads with no --pr.
  @ID-PUSHTHREAD-22 @wip @slice-201
  Scenario: push's help documents push threads --pr and push thread's show form
    When itos runs "help push"
    Then itos exits with code 0
    And its output says "itos push threads [--pr <n>]", whatever its line breaks
    And its output says "shows the thread in full", whatever its line breaks

  # --pr names one pull request, by its number, to push threads alone; any
  # other use of it is a usage error that asks gh nothing. A guard: every
  # row is a usage error before the work too (threads takes no argument,
  # thread and push take no --pr), and must stay one after it.
  @ID-PUSHTHREAD-23 @wip @slice-201 @guard
  Scenario Outline: Under push.mode pr, --pr anywhere but once after push threads with a number above 0 is a usage error, exit 2
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    When itos runs "<command>"
    Then itos exits with code 2
    And the recording gh was never called

    Examples:
      | command                         |
      | push threads --pr               |
      | push threads --pr seven         |
      | push threads --pr 0             |
      | push threads --pr -7            |
      | push threads --pr 7 --pr 8      |
      | push threads --pr 7 PRRT_1      |
      | push threads --pr 7 --ready     |
      | push thread PRRT_1 --pr 7       |
      | push --pr 7                     |

  # threads and thread are push's only words, each in its one form; any
  # other positional, or a form neither takes, stays a usage error and asks
  # gh nothing. A guard: every row is a usage error before the work too,
  # and must stay one after it.
  @ID-PUSHTHREAD-14 @slice-191 @guard
  Scenario Outline: Under push.mode pr, push with a positional that is neither threads nor thread in its form is a usage error, exit 2
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "slice-9"
    When itos runs "<command>"
    Then itos exits with code 2
    And the recording gh was never called

    Examples:
      | command                                     |
      | push widgets                                |
      | push thread                                 |
      | push thread PRRT_1                          |
      | push thread PRRT_1 --reply                  |
      | push thread PRRT_1 PRRT_2 --resolve         |
      | push threads PRRT_1                         |
      | push threads --ready                        |
      | push --reply 'Renamed.'                     |
      | push --resolve                              |
      | push thread PRRT_1 --resolve --tag v1.0.0   |
