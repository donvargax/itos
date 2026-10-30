@phase-1
Feature: itos next reads an in-flight item's state from the forge under push.mode pr
  The v9 deterministic coordinator's pilot (docs/plans/v9-coordinator.md,
  the machine and where state lives): slice-203's in-flight is a stand-in
  for the states only the forge and the attempt journal know, so a driver
  still has to remember whether a spec is under review, approved, being
  built, handed back, merging or merged. Wanted: itos next computes those
  states from the pull request, its verdict markers (slice-206), its
  checks and review threads, and the attempt journal (slice-207), so the
  next step is computed, never remembered.

  Under push.mode pr, an item slice-203 would call in-flight (a spec present
  at HEAD, as next reads scenarios and checks; the driver's clone sits on
  the item's branch, where itos brief and itos attempt submit read it) asks
  the forge, through the gh on the PATH and the repository push names: one
  gh pr list --repo <repository> --head <item> --state all --json
  number,url,state,isDraft,mergedAt,headRefOid for the branch named for the
  item, whose open pull request is the item's and, with none open, a merged
  one is, a closed unmerged one counting for nothing; then, for an open
  one, the one gh api graphql question --ready and review post ask
  (reviewQuery in internal/slices/push/ready.go, with the pull request's
  headRefOid, its comments paged as --ready pages them, its review threads,
  and the head commit's statusCheckRollup state, which the query gains
  here), and never a mutation, a pr view, a pr checks or a fetch. It reads
  the attempt journal, .git/itos/attempts/<item>.jsonl, as attempt submit
  writes it, and no journal is no record. Under push.mode trunk, in-flight
  stays as slice-203 has it, nothing asked. Every other state (done,
  parked, blocked, waiting, idea, needs-spec) is judged before and asks no
  forge, as slice-203 has it, so a clone whose HEAD holds no spec for the
  item says needs-spec and asks nothing.

  The states, the first that holds winning, each with its actor, action
  and argv; the argv is given only for a deterministic itos command:
  merged (the pull request is merged and the registry does not say done):
  driver close, itos work done <item>;
  repair (the pull request is ready, not a draft, and the head's checks
  have failed or a review thread is unresolved): implementer implement,
  itos brief <item> --role implementer;
  merging (the pull request is ready and nothing failed, its checks
  passed or still pending, every thread resolved): forge observe;
  handed-back (the pull request is a draft, the item is doing, and the
  journal's latest implementer record is accepted with the outcome done at
  the pull request's current head): driver land, itos push --ready;
  implementing (the item is doing, in any other case: no pull request yet,
  no journal, a latest implementer record at an older head, rotated, or
  with another outcome): implementer implement, itos brief <item> --role
  implementer;
  spec-accepted (the item is todo and the latest counted spec verdict
  marker at the current head is approve): driver take, itos work take
  <item>;
  spec-changes (the item is todo and that verdict is changes): drafter
  specify, itos brief <item> --role drafter;
  spec-review (the item is todo and the pull request is open but no spec
  verdict marker is at its current head, none at all or only at older
  heads): reviewer review, itos brief <item> --role reviewer;
  spec-proposed (the item is todo, a spec is at HEAD and the branch has no
  open or merged pull request: a spec committed outside the item's pull
  request, the coordinator's case): forge observe.
  Verdict markers are counted as review post counts them: the spec
  markers of this item whose author is a login among the people the config
  names, compared regardless of case, every login counting when there are
  no people; the latest counted one in GitHub's order, on any page, is the
  verdict that stands, and a marker of another stage, another item, an
  outsider or a comment that is no marker counts for nothing.

  The text stays one line, "<id>: <state>; next: <actor> <action>", then
  "; run: <command>" for a deterministic command. --json keeps slice-203's
  keys and, when the forge was asked, gains observed:
  {"pr":{"number","draft","head","merged"} or null with no pull request,
  "verdict":"approve"|"changes"|null (the latest counted spec verdict at
  the current head), "checks":"pending"|"failure"|"success"|"none",
  "threads_open":<n>, "attempt":{"role","attempt","verdict"} or null (the
  journal's latest record)}; head the full SHA GitHub gives (for a merged
  one, gh pr list's). checks reads the head's statusCheckRollup: SUCCESS is
  success, FAILURE and ERROR failure, PENDING and EXPECTED pending, and no
  rollup, or an answer without one, none. threads_open counts the review
  threads not resolved. The revision digests these forge facts and the
  journal's beside slice-203's, so a new head, a new verdict, a check's
  result or a new thread gives a new one, and the same facts the same.
  A gh that is missing or fails exits 3, rule next-forge-failed, naming
  the call and gh's words ("gh pr list failed: HTTP 401: Bad credentials"),
  under --json {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]},
  as review post's review-pull-request-failed does. It still writes no
  file, commits nothing, pushes nothing and fetches nothing.

  Not in this slice: routing a journal's blocked, needs_decision or
  spec_defect outcome (implementing until then); a diff-stage verdict (the
  implementation's review is the forge's checks and threads here); budgets
  and the same failure twice; main's CI after the merge (merged is close,
  and work done judges main); a pull request named by number; trunk mode's
  states.

  The steps. The recording gh is push-mode.feature's with slice-206's
  additions (nodes/06-head-ref-oid.sh and the verdict marker comments) and
  learns one answer: "the recording gh answers that the pull request's
  head's checks are <state>" writes gh-checks, and a nodes file of its own,
  nodes/07-checks.sh, gives the pull request's commits(last: 1) with the
  head commit's statusCheckRollup, {"state": <the word>} when gh-checks is
  there and null otherwise, so an answer always has the shape. The journal
  is written as a file through slice-207's "the file <path> holds the
  lines, the saved values filled in".

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # The slice's point: a todo item with its spec on a draft pull request
  # and no verdict is under spec review. observed is exact: the head the
  # remote's, no verdict, no checks, no thread, no journal. Two calls, the
  # list with --state all and the one graphql question with the checks in
  # it, and nothing else: no view, no mutation, no fetch, no file. Fails
  # before the work at the exit code: today next is no command, exit 2.
  @ID-NEXTFORGE-01 @wip @slice-209
  Scenario: Under push.mode pr, a todo item whose draft pull request has no verdict marker is in spec review
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-review; next: reviewer review; run: itos brief slice-9 --role reviewer" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "item" is "slice-9"
    And its JSON's "state" is "spec-review"
    And its JSON's "next" is {"actor": "reviewer", "action": "review", "argv": ["itos", "brief", "slice-9", "--role", "reviewer"]}
    And its JSON's "observed.pr.number" is 7
    And its JSON's "observed.pr.draft" is true
    And its JSON's "observed.pr.merged" is false
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "observed.verdict" is null
    And its JSON's "observed.checks" is "none"
    And its JSON's "observed.threads_open" is 0
    And its JSON's "observed.attempt" is null
    And the recording gh's "pr list" call has "--head slice-9"
    And the recording gh's "pr list" call has "--state all"
    And the recording gh's "pr list" call has "--repo owner/scratch"
    And the recording gh's "api graphql" call has "number=7"
    And the recording gh's "api graphql" call has "statusCheckRollup"
    And the recording gh's "api graphql" call does not have "mutation"
    And the recording gh was never called as "pr view"
    And the recording gh was never called as "pr checks"
    And the recording gh was never called as "pr create"
    And the recording gh was never called as "pr ready"
    And the recording gh was never called as "pr merge"
    And the recording gh was never called as "pr comment"
    And git reports no change to the working tree or the index
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist

  # ana's approve at the head is the latest counted marker: the later
  # changes is by a login the people file does not list, someone's changes
  # is of the diff stage and the other approve is of slice-8, so none of
  # them counts. A build that took the last marker whoever wrote it would
  # say spec-changes.
  @ID-NEXTFORGE-02 @wip @slice-209
  Scenario: Under push.mode pr, a todo item whose latest counted spec verdict at the head is approve is spec-accepted, the driver takes it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "Ana" and "someone"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "someone" recording the "diff" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "changes"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-8" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "copilot-pull-request-reviewer" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 2: "changes"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-accepted; next: driver take; run: itos work take slice-9" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-accepted"
    And its JSON's "next" is {"actor": "driver", "action": "take", "argv": ["itos", "work", "take", "slice-9"]}
    And its JSON's "observed.verdict" is "approve"
    And its JSON's "observed.pr.draft" is true

  # The markers are append-only and the latest counted one stands: ana
  # approved, then asked for changes. A build that found the first marker,
  # or any approve, would say spec-accepted.
  @ID-NEXTFORGE-03 @wip @slice-209
  Scenario: Under push.mode pr, a todo item whose latest counted spec verdict at the head is changes goes back to the drafter
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 2: "changes"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-changes; next: drafter specify; run: itos brief slice-9 --role drafter" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-changes"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-9", "--role", "drafter"]}
    And its JSON's "observed.verdict" is "changes"

  # A verdict is bound to the head it judged: someone's approve names a
  # commit that is not the pull request's head, so nothing stands and the
  # spec is to be reviewed again, verdict null. Then, with no people file,
  # every login counts, as review post and the work registry read such a
  # repository: Copilot's approve at the head is the verdict.
  @ID-NEXTFORGE-04 @wip @slice-209
  Scenario: Under push.mode pr, a verdict at an older head does not count, and with no people file any login's does
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file is missing
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-9" at the commit "0123456789abcdef0123456789abcdef01234567" as round 1: "approve"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's "next" is {"actor": "reviewer", "action": "review", "argv": ["itos", "brief", "slice-9", "--role", "reviewer"]}
    And its JSON's "observed.verdict" is null
    Given the recording gh answers that the pull request has a comment by "copilot-pull-request-reviewer" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 2: "approve"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-accepted"
    And its JSON's "observed.verdict" is "approve"

  # A pull request with more than a hundred comments has its markers past
  # the first page: they are read as --ready and review post read them, so
  # the approve on the second page stands. A build reading one page would
  # say spec-review.
  @ID-NEXTFORGE-05 @wip @slice-209
  Scenario: Under push.mode pr, a verdict marker on the second page of comments is read
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" asking "chatgpt-codex-connector" to review the clone's HEAD again
    And the recording gh answers that the pull request has, on a second page of comments, a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-accepted"
    And its JSON's "observed.verdict" is "approve"
    And the recording gh's "api graphql" call has "cursor=page2"

  # A doing item is being built whatever the pull request says short of a
  # hand-back: first with no pull request at all (pr null, the list asked,
  # no graphql question), then on a draft with no journal, then with a
  # journal whose latest implementer record is accepted but at the spec
  # commit, no longer the head, and last with a record at the head whose
  # outcome is blocked. The owner is ana's, as written.
  @ID-NEXTFORGE-06 @wip @slice-209
  Scenario: Under push.mode pr, a doing item with no accepted implementer result at the pull request's head is implementing
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "spec"
    And git pushes HEAD to the remote's new branch "slice-9"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: implementing; next: implementer implement; run: itos brief slice-9 --role implementer" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.pr" is null
    And its JSON's "observed.attempt" is null
    And the recording gh was never called as "api graphql"
    Given the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "observed.pr.number" is 7
    And its JSON's "observed.attempt" is null
    Given the remote's branch "slice-9" has gained the commit "feat: say the next step" touching "widget.md"
    And the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<spec>","verdict":"accepted"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "observed.attempt" is {"role": "implementer", "attempt": 1, "verdict": "accepted"}
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<spec>","verdict":"accepted"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"blocked","head":null,"verdict":"accepted"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "observed.attempt" is {"role": "implementer", "attempt": 2, "verdict": "accepted"}

  # The slice's other point: the implementer's accepted done at the pull
  # request's head hands the item back, and the driver's step is the
  # deterministic itos push --ready. The journal's latest implementer
  # record is the third line, past the drafter's and a rotated attempt;
  # the head is the remote's branch tip, which the clone's HEAD is not.
  @ID-NEXTFORGE-07 @wip @slice-209
  Scenario: Under push.mode pr, a doing item whose latest implementer result is accepted done at the pull request's head is handed back, the driver lands it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "spec"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone is on the new branch "work"
    And the clone has the commit "test: add the steps" touching "steps.md"
    And the clone has the commit "feat: say the next step" touching "widget.md"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "slice-9"
    And the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"drafter","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<spec>","verdict":"accepted"}
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<head>","verdict":"rotated"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<head>","verdict":"accepted"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: handed-back; next: driver land; run: itos push --ready" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "handed-back"
    And its JSON's "next" is {"actor": "driver", "action": "land", "argv": ["itos", "push", "--ready"]}
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "observed.attempt" is {"role": "implementer", "attempt": 2, "verdict": "accepted"}

  # Once ready, the pull request is the forge's: the checks passed and the
  # one thread is resolved, so nothing is to be done but wait, no argv and
  # no "; run:". Then with the checks still pending it is merging as well.
  # The registry's status is doing and a journal says done at the head:
  # ready is judged before handed-back, so a build that read the journal
  # first would say handed-back.
  @ID-NEXTFORGE-08 @wip @slice-209
  Scenario: Under push.mode pr, a ready pull request whose checks passed or are pending with every thread resolved is merging, the forge acts
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request's head's checks are "SUCCESS"
    And the recording gh answers that the pull request has a resolved review thread on "gadget.md" line 1, its first comment the lines "Fine now." and "Thanks."
    And the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","outcome":"done","head":"<head>","verdict":"accepted"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: merging; next: forge observe" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON's "observed.pr.draft" is false
    And its JSON's "observed.checks" is "success"
    And its JSON's "observed.threads_open" is 0
    Given the recording gh answers that the pull request's head's checks are "PENDING"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "observed.checks" is "pending"

  # A red check at the head of a ready pull request is the implementer's
  # to repair: FAILURE first, then ERROR, both failure. The registry says
  # todo and a marker approves the spec at the head, so a build that
  # judged the spec states before the ready pull request would say
  # spec-accepted.
  @ID-NEXTFORGE-09 @wip @slice-209
  Scenario: Under push.mode pr, a ready pull request whose head's checks failed is in repair, the implementer acts
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    And the recording gh answers that the pull request's head's checks are "FAILURE"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: repair; next: implementer implement; run: itos brief slice-9 --role implementer" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.checks" is "failure"
    And its JSON's "observed.verdict" is "approve"
    Given the recording gh answers that the pull request's head's checks are "ERROR"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    And its JSON's "observed.checks" is "failure"

  # The checks passed, but one of the two threads is unresolved: a review
  # finding is the implementer's to address before the forge can merge.
  # threads_open counts the unresolved one alone.
  @ID-NEXTFORGE-10 @wip @slice-209
  Scenario: Under push.mode pr, a ready pull request with an unresolved review thread is in repair
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the recording gh answers that the pull request's head's checks are "SUCCESS"
    And the recording gh answers that the pull request has an unresolved review thread on "widget.md" line 3, its first comment the lines "Rename this." and "It clashes with the gadget."
    And the recording gh answers that the pull request has a resolved review thread on "gadget.md" line 1, its first comment the lines "Fine now." and "Thanks."
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.checks" is "success"
    And its JSON's "observed.threads_open" is 1

  # The forge merged it and the registry still says doing: the driver
  # closes it, and the graphql question is not asked of a merged pull
  # request. Once the registry says done, done is judged first and the
  # forge is not asked at all: the list was called for the first run alone.
  @ID-NEXTFORGE-11 @wip @slice-209
  Scenario: Under push.mode pr, a merged pull request whose item the registry does not say done is merged, the driver closes it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/7" merged for the branch "slice-9"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: merged; next: driver close; run: itos work done slice-9" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merged"
    And its JSON's "next" is {"actor": "driver", "action": "close", "argv": ["itos", "work", "done", "slice-9"]}
    And its JSON's "observed.pr.number" is 7
    And its JSON's "observed.pr.merged" is true
    And the recording gh was never called as "api graphql"
    Given the file "tasks/work-items.yaml" holds the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: slice-9, phase: 1, owner: ana, status: done, depends_on: [] }
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "done"
    And its JSON has no "observed"
    And the recording gh was called as "pr list" once

  # The spec is at HEAD, on main, and the branch has no pull request, open
  # or merged: the coordinator committed it outside the item's pull
  # request, and nothing is to be done until one opens. slice-8 has no
  # spec, so it needs one and asks nothing: the list was called for slice-9
  # alone, and no graphql question for either.
  @ID-NEXTFORGE-12 @wip @slice-209
  Scenario: Under push.mode pr, a todo item with a spec and no pull request is spec-proposed, and one with no spec asks no forge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-proposed; next: forge observe" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-proposed"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON's "observed.pr" is null
    And its JSON's "observed.verdict" is null
    And its JSON's "observed.checks" is "none"
    And its JSON's "observed.threads_open" is 0
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON has no "observed"
    And the recording gh was called as "pr list" once
    And the recording gh was never called as "api graphql"

  # No push.mode in the config: trunk, where the forge has nothing to say
  # of an item, so in-flight stays as slice-203 has it and the gh on the
  # PATH, holding a draft pull request for the branch, is never called.
  @ID-NEXTFORGE-13 @wip @slice-209
  Scenario: Under push.mode trunk, an item with a spec stays in flight and the forge is not asked
    Given a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON has no "observed"
    And the recording gh was never called

  # The same forge facts twice give the same revision. Then each forge
  # fact alone moves it: a new head on the remote (the state still
  # spec-review), a check's result on the draft (still spec-review), a new
  # verdict (spec-accepted now). A build digesting slice-203's facts alone
  # would give "first" throughout.
  @ID-NEXTFORGE-14 @wip @slice-209
  Scenario: Under push.mode pr, the revision digests the forge's facts: a new head, a check result or a new verdict changes it
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the people file lists "ana" and "someone"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's revision is "sha256:" and 64 lowercase hex digits
    And its JSON's revision is saved as "first"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is the one saved as "first"
    Given the remote's branch "slice-9" has gained the commit "docs: answer the review" touching "features/widget.feature"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's revision differs from the one saved as "first"
    And its JSON's revision is saved as "second"
    Given the recording gh answers that the pull request's head's checks are "PENDING"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's revision differs from the one saved as "second"
    And its JSON's revision is saved as "third"
    Given the recording gh answers that the pull request has a comment by "ana" recording the "spec" review of "slice-9" at the head of the remote's branch "slice-9" as round 1: "approve"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-accepted"
    And its JSON's revision differs from the one saved as "third"

  # A gh that fails is an environment failure, exit 3, naming the call and
  # gh's words, under --json the house problems JSON and without it the
  # message in the output; no state is printed, nothing is written.
  @ID-NEXTFORGE-15 @wip @slice-209
  Scenario: Under push.mode pr, next with a gh that fails exits 3, naming the call
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the recording gh fails every call, saying "HTTP 401: Bad credentials"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 3
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "next-forge-failed"'s "message" says "gh pr list failed: HTTP 401: Bad credentials"
    And its JSON has no "state"
    When itos runs "next slice-9"
    Then itos exits with code 3
    And its output says "next-forge-failed"
    And its output says "gh pr list failed: HTTP 401: Bad credentials"
    And git reports no change to the working tree or the index
