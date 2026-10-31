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

  Where the spec is read. Under push.mode pr the item's work lives on the
  branch named for it, and the driver's clone stays on main (slice-207), so
  next reads a slice's or a bug's tagged scenarios at origin/<item> when
  the clone has that ref, else at HEAD; needs-spec holds only when neither
  holds a scenario tagged for the item. Nothing is fetched: the ref is the
  one the clone's last fetch or push left. This amends slice-203's "tagged
  scenarios at HEAD" for pr mode alone; under trunk, and for a task's
  ledger checks, nothing changes.

  Under push.mode pr, an item slice-203 would call in-flight (a spec present
  as read above) asks the forge, through the gh on the PATH and the
  repository push names: one gh pr list --repo <repository> --head <item>
  --state all --json number,url,state,isDraft,mergedAt,headRefOid for the
  branch named for the item, whose open pull request is the item's and,
  with none open, a merged one is; a closed unmerged one counts for
  nothing. Then, for an open one, the one gh api graphql question --ready
  and review post ask (reviewQuery in internal/slices/push/ready.go, with
  the pull request's headRefOid, its comments paged as --ready pages them,
  its review threads, and the head commit's statusCheckRollup state, which
  the query gains here), and never a mutation, a pr view, a pr checks or a
  fetch; a merged one is asked nothing more. The pull request's head is the
  graphql answer's headRefOid for an open one and gh pr list's for a merged
  one. It reads the attempt journal, .git/itos/attempts/<item>.jsonl in
  the git folder, as attempt submit writes it, one JSON object a line with
  the keys schema, role, attempt, request, base, outcome, head, notes,
  verdict and sha256: next reads role, attempt, outcome, head and verdict
  and ignores the others, and no journal is no record. Under push.mode
  trunk, in-flight stays as slice-203 has it, nothing asked. Every other
  state (done, parked, blocked on a question or a status, waiting, idea,
  needs-spec) is judged before and asks no forge, as slice-203 has it.

  The states, the first that holds winning, each with its actor, action
  and argv; the argv is given only for a deterministic itos command:
  merged (the pull request is merged and the registry does not say done):
  driver close, itos work done <item>;
  repair (the pull request is ready, not a draft, and the head's checks
  have failed or a review thread is unresolved): implementer implement,
  itos brief <item> --role implementer;
  merging (the pull request is ready and nothing failed, its checks
  passed or still pending, every thread resolved): forge observe;
  blocked (the journal's latest implementer record, at any head, has the
  outcome needs_decision or blocked and is not spent): person answer,
  blocked_reason "attempt", no waits_on and no argv, the person's step
  being itos decision add <the question> --item <item>, then itos
  decision answer. The k-th implementer record with either outcome, in
  journal order, is spent by the k-th question naming the item, in
  tasks/asks.yaml's order, once that question is answered; while it is
  open the item is slice-203's blocked "decision", judged before. So an
  answered question from before the record spent an earlier one and
  cannot spend this one, and a driver never briefs an implementer again
  that asked for a decision;
  spec-changes (the journal's latest implementer record has the outcome
  spec_defect and no drafter record comes after it: the spec goes back to
  the drafter whatever the item's status): drafter specify, itos brief
  <item> --role drafter. A drafter record later in the journal, with any
  outcome, spends it, and the states below apply again;
  handed-back (the pull request is a draft, the item is doing, and the
  journal's latest implementer record is accepted with the outcome done at
  the pull request's current head): driver land, itos push --ready;
  implementing (the item is doing, in any other case: no pull request yet,
  no journal, a latest implementer record at an older head or rotated):
  implementer implement, itos brief <item> --role implementer;
  spec-accepted (the item is todo and the latest counted spec verdict
  marker at the current head is approve): driver take, itos work take
  <item>;
  spec-changes (the item is todo and that verdict is changes): drafter
  specify, itos brief <item> --role drafter;
  spec-review (the item is todo and the pull request is open but no spec
  verdict marker is at its current head, none at all or only at older
  heads): reviewer review, itos brief <item> --role reviewer;
  spec-proposed (the item is todo, a spec is at the ref read and the
  branch has no open or merged pull request: a spec committed outside the
  item's pull request, or not pushed yet): drafter specify, itos brief
  <item> --role drafter, since the drafter's brief has it push the spec
  with itos push, which opens the draft.
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
  "threads_open":<n>, "attempt":{"role","attempt","verdict","outcome","head"}
  or null (the journal's latest implementer record, head null when it has
  none)}; pr.head the full SHA GitHub gives, read as above. checks reads
  the head's statusCheckRollup, the roll-up of every status context and
  check of the head, not ci.watch's single run: SUCCESS is success, FAILURE
  and ERROR failure, PENDING and EXPECTED pending, and no rollup, or an
  answer without one, none; so a red context unrelated to the run --ready
  watches is repair here even where --ready would merge. threads_open
  counts the review threads not resolved. The revision digests these forge
  facts, the journal's and the ref the scenarios were read at beside
  slice-203's, so a new head, a new verdict, a check's result or a new
  thread gives a new one, and the same facts the same.
  A gh that is missing or fails exits 3, rule next-forge-failed, naming
  the call and gh's words ("gh pr list failed: HTTP 401: Bad credentials"),
  under --json {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]},
  as review post's review-pull-request-failed does. It still writes no
  file, commits nothing, pushes nothing and fetches nothing.

  Not in this slice: a diff-stage verdict (the implementation's review is
  the forge's checks and threads here); budgets and the same failure
  twice; main's CI after the merge (merged is close, and work done judges
  main); a pull request named by number; trunk mode's states.

  The steps. The recording gh is push-mode.feature's with slice-206's
  additions (nodes/06-head-ref-oid.sh and the verdict marker comments) and
  learns one answer: "the recording gh answers that the pull request's
  head's checks are <state>" writes gh-checks, and a nodes file of its own,
  nodes/07-checks.sh, gives the pull request's commits(last: 1) with the
  head commit's statusCheckRollup, {"state": <the word>} when gh-checks is
  there and null otherwise, so an answer always has the shape. "the
  recording gh was called as <command> <n> times" counts the recorded
  calls starting with the command, exactly n, as "was called as <command>
  once" counts one. The journal is written as a file through slice-207's
  "the file <path> holds the lines, the saved values filled in", which
  creates the folders the path needs (filepath.FromSlash, so
  .git/itos/attempts/ is made on every platform). slice-206's "its JSON's
  <key> holds the full SHA of the head of the remote's branch <branch>"
  walks a dotted key as "its JSON's <key> is" does (jsonValueIs), so
  observed.pr.head and observed.attempt.head are read with it.

  slice-216 amends merging. itos push --ready marks the pull request ready
  before it turns auto-merge on, so after its exit 75, or under --no-wait,
  nothing merges a ready pull request and a driver told merging waits
  forever. merging now needs auto-merge on as well: the pull request is
  ready, auto-merge is enabled for it, nothing failed and every configured
  reviewer requested. A ready pull request with auto-merge off, nothing
  failed and every reviewer requested is handed-back, the driver's land
  step, itos push --ready, whatever the item's status or the journal says,
  since the ready pull request's states are judged before the journal's.
  merged, repair and review-unrequested keep their places before both; a
  draft's states, and trunk mode, are unchanged. Auto-merge is read in the
  one graphql question next already asks (providers.PullStateQuery gains
  the pull request's autoMergeRequest { enabledAt }: on when it is not
  null), never by another gh call. observed gains auto_merge, true or
  false, false when no open pull request was asked about; the revision
  digests it. ID-NEXTFORGE-08 and slice-215's ID-REVREQ-12, which say
  merging of a ready pull request without auto-merge, gain the Given that
  turns it on, and the commit that builds this names both in Changes:.
  The recording gh learns "the recording gh's pull request for the branch
  <branch> has auto-merge on", which writes gh-auto-merge-<branch>, the
  mark its pr merge --auto leaves and its pr view reads, and a nodes file,
  nodes/08-auto-merge.sh, gives the pull request numbered (its branch found
  in gh-pull-requests, as nodes/06-head-ref-oid.sh finds it)
  "autoMergeRequest": {"enabledAt": <a time>} when that mark is there and
  null otherwise, so an answer always has the field.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # The slice's point: a todo item with its spec on a draft pull request
  # and no verdict is under spec review. observed is exact: the head the
  # remote's, no verdict, no checks, no thread, no journal. Two calls, the
  # list with --state all and the one graphql question with the checks in
  # it, and nothing else: no view, no mutation, no fetch, no file. Fails
  # before the work at the exit code: today next is no command, exit 2.
  @ID-NEXTFORGE-01 @slice-209
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
    And the recording gh was called as "pr list" 2 times
    And the recording gh was called as "api graphql" 2 times
    And git reports no change to the working tree or the index
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist

  # The driver's clone is on main, where no scenario is tagged for the
  # item: the spec is at origin/slice-9, the ref the push left, so the
  # item is in spec review, not needs-spec, and the forge is asked. A
  # second scenario pushed to the branch changes the revision, the ref
  # read being digested, with no fetch asked of the clone. Main's config
  # declares the scenario kind, as a driver's does, through slice-8's
  # scenario committed there: the config next reads is the clone's.
  @ID-NEXTFORGE-16 @slice-209
  Scenario: Under push.mode pr, the spec is read at origin/<item> when the clone has that ref, so a clone on main asks the forge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-Z-01" tagged "@wip @slice-8"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the repository state is saved for comparison
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-review; next: reviewer review; run: itos brief slice-9 --role reviewer" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's revision is saved as "first"
    And the repository files index and HEAD match the saved state
    Given the clone switches to its branch "slice-9"
    And the committed feature file "features/b.feature" with the live scenario "@ID-B-01" tagged "@slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the repository state is saved for comparison
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's revision differs from the one saved as "first"
    And the repository files index and HEAD match the saved state
    And the recording gh was called as "pr list" 3 times
    And the recording gh was called as "api graphql" 3 times

  # ana's approve at the head is the latest counted marker: the later
  # changes is by a login the people file does not list, someone's changes
  # is of the diff stage and the other approve is of slice-8, so none of
  # them counts. A build that took the last marker whoever wrote it would
  # say spec-changes.
  @ID-NEXTFORGE-02 @slice-209
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
  @ID-NEXTFORGE-03 @slice-209
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
  @ID-NEXTFORGE-04 @slice-209
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
  @ID-NEXTFORGE-05 @slice-209
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
  # commit, no longer the head. Then the implementer's latest record asks
  # for a decision: the item is blocked on the person, never briefed
  # again, and its reason is the attempt, not a question or a status.
  # q-2, the first question naming slice-9 once answered, spends that
  # record (q-1 names slice-8 and counts for nothing): implementing again.
  # A third record, blocked, has no second question to spend it: blocked
  # again, q-2 being spent. Then the implementer found the spec defective:
  # it goes back to the drafter, the item still doing, until a drafter
  # record follows it, when it is implementing again. The owner is ana's,
  # as written. The journal lines are whole, as attempt submit writes
  # them; the keys next does not read are filled with anything.
  @ID-NEXTFORGE-06 @slice-209
  Scenario: Under push.mode pr, a doing item with no accepted implementer result at the pull request's head is implementing, blocked on an unspent needs_decision, spec-changes on an unspent spec_defect
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
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.attempt.role" is "implementer"
    And its JSON's "observed.attempt.attempt" is 1
    And its JSON's "observed.attempt.verdict" is "accepted"
    And its JSON's "observed.attempt.outcome" is "done"
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"needs_decision","head":null,"notes":"Which footer?","verdict":"accepted","sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: blocked; next: person answer" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "blocked_reason" is "attempt"
    And its JSON has no "waits_on"
    And its JSON's "observed.attempt" is {"role": "implementer", "attempt": 2, "verdict": "accepted", "outcome": "needs_decision", "head": null}
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-8"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "slice-9"
          question: "Which footer?"
          answer: "Item."
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON has no "blocked_reason"
    And its JSON's "observed.attempt.outcome" is "needs_decision"
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"needs_decision","head":null,"notes":"Which footer?","verdict":"accepted","sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222"}
      {"schema":1,"role":"implementer","attempt":3,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"blocked","head":null,"notes":null,"verdict":"accepted","sha256":"sha256:3333333333333333333333333333333333333333333333333333333333333333"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "blocked_reason" is "attempt"
    And its JSON has no "waits_on"
    And its JSON's "observed.attempt.attempt" is 3
    And its JSON's "observed.attempt.outcome" is "blocked"
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"spec_defect","head":null,"notes":"@ID-A-01 asks the impossible.","verdict":"accepted","sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-changes; next: drafter specify; run: itos brief slice-9 --role drafter" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-changes"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-9", "--role", "drafter"]}
    And its JSON has no "blocked_reason"
    And its JSON's "observed.attempt.outcome" is "spec_defect"
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"spec_defect","head":null,"notes":"@ID-A-01 asks the impossible.","verdict":"accepted","sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222"}
      {"schema":1,"role":"drafter","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":"The scenario is reworded.","verdict":"accepted","sha256":"sha256:4444444444444444444444444444444444444444444444444444444444444444"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.attempt.outcome" is "spec_defect"

  # The slice's other point: the implementer's accepted done at the pull
  # request's head hands the item back, and the driver's step is the
  # deterministic itos push --ready. The journal's latest implementer
  # record is the third line, past the drafter's and a rotated attempt,
  # and a drafter's line comes after it: observed.attempt is the
  # implementer's, not the journal's last line. The head is the remote's
  # branch tip, which the clone's HEAD is not.
  @ID-NEXTFORGE-07 @slice-209
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
      {"schema":1,"role":"drafter","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<spec>","notes":"The spec is pushed.","verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<head>","notes":null,"verdict":"rotated","sha256":"sha256:2222222222222222222222222222222222222222222222222222222222222222"}
      {"schema":1,"role":"implementer","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"done","head":"<head>","notes":null,"verdict":"accepted","sha256":"sha256:3333333333333333333333333333333333333333333333333333333333333333"}
      {"schema":1,"role":"drafter","attempt":2,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"blocked","head":null,"notes":null,"verdict":"accepted","sha256":"sha256:4444444444444444444444444444444444444444444444444444444444444444"}
      """
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: handed-back; next: driver land; run: itos push --ready" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "handed-back"
    And its JSON's "next" is {"actor": "driver", "action": "land", "argv": ["itos", "push", "--ready"]}
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "slice-9"
    And its JSON's "observed.attempt.role" is "implementer"
    And its JSON's "observed.attempt.attempt" is 2
    And its JSON's "observed.attempt.verdict" is "accepted"
    And its JSON's "observed.attempt.outcome" is "done"
    And its JSON's "observed.attempt.head" holds the full SHA of the head of the remote's branch "slice-9"

  # Once ready, the pull request is the forge's: the checks passed and the
  # one thread is resolved, so nothing is to be done but wait, no argv and
  # no "; run:". Then with the checks still pending it is merging as well.
  # The registry's status is doing and a journal says done at the head:
  # ready is judged before handed-back, so a build that read the journal
  # first would say handed-back.
  @ID-NEXTFORGE-08 @slice-209
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
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<head>","outcome":"done","head":"<head>","notes":null,"verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
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

  # slice-216: --ready marks the pull request ready before it turns
  # auto-merge on, so after its exit 75, or under --no-wait, a ready pull
  # request with nothing failed has nothing to merge it. merging is said
  # only once auto-merge is on; without it the step is handed-back's, the
  # driver lands it with itos push --ready. No journal here: the ready pull
  # request's states are judged before the journal's, so a build that let
  # it fall through to slice-209's handed-back, which needs the
  # implementer's accepted done at the head, would say implementing. One
  # graphql question a run and no pr view: auto-merge is read in it. Fails
  # before the work at the first complete line: slice-209's next says
  # merging.
  @wip @ID-NEXTFORGE-17 @slice-216
  Scenario: Under push.mode pr, a ready pull request with nothing failed is merging only once auto-merge is on; without it the driver lands it
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
    And the recording gh answers that the pull request has a resolved review thread on "gadget.md" line 1, its first comment the lines "Fine now." and "Thanks."
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: handed-back; next: driver land; run: itos push --ready" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "handed-back"
    And its JSON's "next" is {"actor": "driver", "action": "land", "argv": ["itos", "push", "--ready"]}
    And its JSON's "observed.pr.draft" is false
    And its JSON's "observed.auto_merge" is false
    And its JSON's "observed.checks" is "success"
    And its JSON's "observed.threads_open" is 0
    And its JSON's "observed.attempt" is null
    Given the recording gh's pull request for the branch "slice-9" has auto-merge on
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: merging; next: forge observe" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON's "observed.auto_merge" is true
    Given the recording gh answers that the pull request's head's checks are "PENDING"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "observed.checks" is "pending"
    And its JSON's "observed.auto_merge" is true
    And the recording gh's "api graphql" call has "autoMergeRequest"
    And the recording gh was called as "api graphql" 5 times
    And the recording gh was never called as "pr view"

  # auto-merge is a fact of its own: GitHub keeps it on while a required
  # check fails, and turning it on moves the revision though the state
  # stays repair. repair, then review-unrequested (slice-215), keep their
  # precedence over merging: a red check is the implementer's, then a
  # configured reviewer not asked about the head is the driver's to
  # request, auto-merge on or not. Fails before the work at the first
  # observed.auto_merge: slice-209's observed has no such key.
  @wip @ID-NEXTFORGE-18 @slice-216
  Scenario: Under push.mode pr, auto-merge turning on moves the revision, and repair and review-unrequested come before merging
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
    And the recording gh answers that the pull request's head's checks are "FAILURE"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    And its JSON's "observed.auto_merge" is false
    And its JSON's revision is saved as "off"
    Given the recording gh's pull request for the branch "slice-9" has auto-merge on
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "repair"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.auto_merge" is true
    And its JSON's revision differs from the one saved as "off"
    Given the recording gh answers that the pull request's head's checks are "PENDING"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "review-unrequested"
    And its JSON's "next" is {"actor": "driver", "action": "request", "argv": ["itos", "review", "request", "slice-9"]}
    And its JSON's "observed.auto_merge" is true

  # The path that left a driver waiting: push --ready --no-wait marks the
  # draft ready and turns on no auto-merge, as an exit 75 leaves it, so
  # next hands the item back to the driver to land; --ready run again turns
  # auto-merge on and next says merging. The recording gh's pr merge --auto
  # leaves the mark its graphql answer's autoMergeRequest reads, as its pr
  # view's does. Fails before the work at the complete line naming
  # handed-back: slice-209's next says merging.
  @wip @ID-NEXTFORGE-19 @slice-216
  Scenario: Under push.mode pr, after push --ready --no-wait the driver lands the item, and once push --ready turned auto-merge on it is merging
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push --ready --no-wait"
    Then itos exits with code 0
    And the recording gh's "pr ready" call has "slice-9"
    And the recording gh was never called as "pr merge"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: handed-back; next: driver land; run: itos push --ready" exactly once
    When itos runs "push --ready"
    Then itos exits with code 0
    And the recording gh's "pr merge" call has "--auto"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "observed.pr.draft" is false
    And its JSON's "observed.auto_merge" is true

  # A red check at the head of a ready pull request is the implementer's
  # to repair: FAILURE first, then ERROR, both failure. The registry says
  # todo and a marker approves the spec at the head, so a build that
  # judged the spec states before the ready pull request would say
  # spec-accepted.
  @ID-NEXTFORGE-09 @slice-209
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
  @ID-NEXTFORGE-10 @slice-209
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
  # request: the head is gh pr list's. Once the registry says done, done is
  # judged first and the forge is not asked at all: the list was called for
  # the two merged runs alone.
  @ID-NEXTFORGE-11 @slice-209
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
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "slice-9"
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
    And the recording gh was called as "pr list" 2 times

  # The spec is at HEAD, on main, the clone has no origin/slice-9, and the
  # branch's one pull request was closed without merging, which counts
  # for nothing: the spec is the drafter's to push, which opens the draft.
  # slice-8 has no spec at either place, so it needs one and asks nothing:
  # the list was called for slice-9's two runs alone, and no graphql
  # question for either.
  @ID-NEXTFORGE-12 @slice-209
  Scenario: Under push.mode pr, a todo item with a spec and no open or merged pull request is spec-proposed, the drafter's, and one with no spec asks no forge
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/3" closed without merging for the branch "slice-9"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: spec-proposed; next: drafter specify; run: itos brief slice-9 --role drafter" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-proposed"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-9", "--role", "drafter"]}
    And its JSON's "observed.pr" is null
    And its JSON's "observed.verdict" is null
    And its JSON's "observed.checks" is "none"
    And its JSON's "observed.threads_open" is 0
    And its JSON's "observed.attempt" is null
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON has no "observed"
    And the recording gh was called as "pr list" 2 times
    And the recording gh was never called as "api graphql"

  # No push.mode in the config: trunk, where the forge has nothing to say
  # of an item, so in-flight stays as slice-203 has it and the gh on the
  # PATH, holding a draft pull request for the branch, is never called.
  @ID-NEXTFORGE-13 @slice-209
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
  @ID-NEXTFORGE-14 @slice-209
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
  @ID-NEXTFORGE-15 @slice-209
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
