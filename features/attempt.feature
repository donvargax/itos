@phase-1
Feature: itos attempt submit, an agent's result checked against its brief and against git
  The deterministic coordinator (docs/plans/v9-coordinator.md, the LLM
  steps' contracts) takes an agent's result as data, never as prose: a
  report is a claim, and today the coordinator reads the hand-off, decides
  whether to believe it, and nothing records which attempt it was. itos
  attempt submit <id> <result.json> reads the result an agent wrote, checks
  it against the brief the agent got (slice-204, itos brief) and against
  git, and records it in a per-clone journal, so that itos next (slice-203)
  can route the item from facts.

  The result is strict JSON with exactly these keys: schema (1), item,
  role (drafter or implementer; a reviewer's result comes with the forge
  facts later), request (the "sha256:" request itos brief <id> --role
  <role> --json gives), base (the full lowercase SHA of the commit the
  brief was computed at, the "base" of that JSON), attempt (a positive
  integer), outcome (done, blocked, needs_decision or spec_defect), head
  (a full lowercase SHA, required with done and refused with any other
  outcome, which claims no head) and notes (a string, optional, never
  judged). Any other key, a missing required one, a wrong type or value,
  or a file that is not one JSON object is malformed.

  For this, itos brief gains two things, which slice-204's command reads
  from a tree already: its --json names "base", the full SHA of the commit
  it computed the brief at, HEAD unless --at <rev> names another commit,
  which it then reads everything from instead of HEAD (the registry, the
  tagged scenarios, the ledger, the reads' blobs), so a driver on main
  briefs an implementer at the item's branch with --at origin/<id>. A rev
  git cannot resolve is refused, exit 1, brief-unknown-revision.

  The checks, in this order, the first failing one refusing with exit 1
  and {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}, the
  text output saying the rule, the message and the fix:
  attempt-malformed, the message naming what is wrong;
  attempt-stale, the result's item not the one on the command line (the
  message naming both), its base not a commit the clone has once the
  item's branch is fetched, or its request not the one itos brief
  computes at base for that item and role (the brief it echoes is not the
  one it was given), the fix saying to brief again;
  attempt-out-of-order, the attempt not one more than the results the
  journal holds for that item and role, the message saying
  "expected attempt <n>"; then, for outcome done, the claims, each
  checked against git and never against notes:
  attempt-head-not-pushed, the head not the tip of the item's branch on
  the remote (the branch named for the item, fetched first, so a
  remote-tracking ref the clone has not refreshed does not answer; a
  branch the remote lacks refuses nothing by itself), the message naming
  the tip when the branch has one;
  attempt-spec-missing, a drafter's head whose tree holds no scenario
  tagged @<id>, or one that is not @wip, the message naming it;
  attempt-not-red-first, an implementer's head whose commits past base
  (base..head, so the spec's own test commit never counts) hold no test
  commit before the first feat or fix, the message naming that commit;
  attempt-still-wip, an implementer's head whose tree still holds a
  scenario tagged @<id> and @wip, the message naming it.
  The scenarios at a head are read from its tree under the clone's own
  config, the scenario kind the clone declares, as itos work done reads
  them at HEAD: the head's itos.yaml is never read. The commit types are
  the headers' as the message rules read them.

  The journal is .git/itos/attempts/<id>.jsonl in the git folder (the
  common dir of a linked worktree), one line per result taken in:
  {"schema":1,"role","attempt","request","base","outcome","head","notes","verdict","sha256"},
  head null when the result has none, notes null when it has none and
  kept as data otherwise (a blocked or needs_decision record carries its
  reason; nothing judges it), sha256 "sha256:" and the hex digest of the
  result file's bytes. An accepted result is recorded with the verdict
  accepted. A malformed result naming a role is recorded too, with the
  verdict rotated, null for every key missing or of the wrong type and
  for attempt the number the journal gives it, so it counts and the
  agent's next try is one higher; the same bytes submitted again are
  refused attempt-malformed with nothing recorded, so a replayed file
  burns no attempt. One naming no role cannot be counted for anyone and
  is refused without a record; itos next must count such refusals (a
  later slice), or a driver loops on an agent that never names its role.
  Stale, out-of-order and claim refusals record nothing: the same attempt
  is resubmitted once fixed. A result the journal already holds (the same
  role, request, attempt, head and outcome, whatever its notes) is
  already, exit 0, nothing recorded; a same-attempt result that differs
  in any of those is attempt-out-of-order. The outcomes blocked,
  needs_decision and spec_defect are accepted with no claim checked;
  recording them is how itos next will route the item later.

  --json on success: {"schema":1,"ok":true,"outcome":"accepted|already","item","role","attempt","request","base","head"},
  head null when the result has none; text one line, "accepted
  <id>'s <role> attempt <n>, <outcome>", then " at <head>" when there is
  one, "already accepted" in front of it for already. An id the registry
  does not have is work-unknown-item, as itos work show refuses it, before
  the result is read; a result file that cannot be read is malformed,
  recorded for no one; a missing argument is a usage error, exit 2.

  Beyond the journal it writes nothing: no commit, no push, no registry
  edit, no forge; its only network is fetching the item's branch.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the committed file "AGENTS.md" holding "Stage only your own files."
    And the committed file ".gitignore" holding "result.json"
    And the committed feature file "features/other.feature" with the live scenario "@ID-O-01" tagged "@slice-8"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # The drafter is briefed on main, where no scenario is tagged for
  # slice-9 yet (the Background's features/other.feature declares the
  # scenario kind, tagged for another item), and pushes the spec on the
  # item's branch. The notes are prose and decide nothing. The journal
  # line's request, base and head are compared with the values the
  # scenario saved, so a build that recorded the attempt alone fails.
  @ID-ATTEMPT-01 @wip @slice-207
  Scenario: A drafter's done result is accepted and recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>","notes":"The spec is pushed."}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And stdout has exactly 1 line starting with "accepted"
    And its output says "accepted slice-9's drafter attempt 1, done at " with the full SHA of the head of the remote's branch "slice-9"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "schema" is 1
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "role" is "drafter"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 1
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "done"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "notes" is "The spec is pushed."
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "request" is the value saved as "request"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "base" is the value saved as "base"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is the value saved as "head"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "sha256" is the SHA-256 of the file "result.json"

  # The driver stays on main: it briefs the implementer at the item's
  # branch with --at, and the implementer's commits past that base are a
  # test commit, then the feat commit that turns the scenario live. The
  # spec's own test commit is before base and counts for nothing.
  @ID-ATTEMPT-02 @wip @slice-207
  Scenario: An implementer's done result is accepted, its JSON echoing the result
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    When itos runs "brief slice-9 --role implementer --json --at origin/slice-9"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone switches to its branch "slice-9"
    And the commit "test: add the steps" on top of it
    And the commit "feat: say the next step" turning the scenario "@ID-A-01" live on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "accepted"
    And its JSON's "item" is "slice-9"
    And its JSON's "role" is "implementer"
    And its JSON's "attempt" is 1
    And its JSON's "request" matches the one saved as "request"
    And its JSON's "base" matches the one saved as "base"
    And its JSON's "head" matches the one saved as "head"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "role" is "implementer"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "base" is the value saved as "base"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is the value saved as "head"

  # The second file differs from the first in its notes alone: the same
  # role, request, attempt, head and outcome is the same result.
  @ID-ATTEMPT-03 @wip @slice-207
  Scenario: The same accepted result again is already, nothing recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And its output says "accepted slice-9's drafter attempt 1, done at "
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>","notes":"Sent twice."}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "already"
    And its JSON's "attempt" is 1
    And its JSON's "head" matches the one saved as "head"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And its output says "already accepted slice-9's drafter attempt 1, done at " with the full SHA of the head of the remote's branch "slice-9"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line

  # The first file has a key the schema does not have, so it is rotated:
  # recorded with what it gave, and the corrected result must be attempt
  # 2. The same bytes again are refused without a second record. The spec
  # is pushed, so nothing but the key stands between the agent and
  # acceptance.
  @ID-ATTEMPT-04 @wip @slice-207
  Scenario: A malformed result is rotated once and the next try is one attempt higher
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>","commits":["<head>"]}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "commits"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "rotated"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "role" is "drafter"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 1
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "done"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "request" is the value saved as "request"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is the value saved as "head"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "notes" is null
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "sha256" is the SHA-256 of the file "result.json"
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "commits"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-out-of-order"'s "message" says "expected attempt 2"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    And its JSON's "attempt" is 2
    And ".git/itos/attempts/slice-9.jsonl" has exactly 2 lines
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 2

  # Eight shapes name a role and are each rotated, a missing or wrongly
  # typed key null in the record and attempt the journal's own number;
  # the ninth is no JSON object at all, names no role, and is refused
  # without a record: eight lines in the end. Each message names the key
  # at fault. The head is the pushed tip, so only the shape refuses.
  @ID-ATTEMPT-05 @wip @slice-207
  Scenario: Each malformed shape is refused, naming the key at fault
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "request"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "request" is null
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "base" is the value saved as "base"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":"1","outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "attempt"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 2
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":3,"outcome":"finished","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "outcome"
    And line 3 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is null
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":2,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":4,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "schema"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":5,"outcome":"done"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    And line 5 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is null
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":6,"outcome":"done","head":"abc1234"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":7,"outcome":"blocked","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":8,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "base"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 8 lines
    And line 8 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "rotated"
    And line 8 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 8
    And line 8 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "base" is null
    Given the file "result.json" holds the lines:
      """
      {"schema":1,"item":"slice-9",
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "result.json"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 8 lines
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-malformed"
    And its output says "result.json"

  # Main moves after the brief (AGENTS.md, a read, is reworded), and the
  # result is still accepted: the request is recomputed at its base, not
  # at HEAD. Then three real stalenesses, none recorded: a result naming
  # another item (both ids in the message; it is attempt 2 as well, so a
  # build that judged the order first fails: stale is the one problem), a
  # base the clone has no commit for, and the request of the brief at the
  # new HEAD with the old base, which is not the brief at that base.
  @ID-ATTEMPT-06 @wip @slice-207
  Scenario: A result is checked against the brief at its base, and one that does not match it is stale
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the committed file "AGENTS.md" holding "Stage only your own files, by path."
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-8","role":"drafter","request":"<request>","base":"<base>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's "problems" has exactly 1 entry
    And its JSON's problem "attempt-stale"'s "message" says "slice-8"
    And its JSON's problem "attempt-stale"'s "message" says "slice-9"
    And its JSON's problem "attempt-stale"'s "fix" says "itos brief slice-9 --role drafter"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-stale"
    And its output says "itos brief slice-9 --role drafter"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"0123456789abcdef0123456789abcdef01234567","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-stale"'s "message" says "0123456789abcdef0123456789abcdef01234567"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And its JSON's "request" differs from the one saved as "request"
    And the JSON's "request" is saved as "request"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-stale"'s "message" says "slice-9"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line

  # Nothing is recorded yet, so the first attempt is 1; once it is taken
  # in, the next is 2: 3 is out of order, and so is 1 again with another
  # outcome, which is not the result the journal holds. The head is the
  # pushed tip and the request current, so only the number refuses.
  @ID-ATTEMPT-07 @wip @slice-207
  Scenario: An attempt that is not the next one is refused, nothing recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-out-of-order"'s "message" says "expected attempt 1"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":3,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-out-of-order"
    And its output says "expected attempt 2"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"blocked"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-out-of-order"'s "message" says "expected attempt 2"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line

  # First the branch exists in the clone alone. Then it is pushed and
  # another clone adds a commit on top of it, which the clone's
  # remote-tracking ref does not know: a build that read origin/slice-9
  # without fetching would take the old head for the tip. The new tip
  # keeps the spec, so a result naming it is accepted once its objects are
  # fetched.
  @ID-ATTEMPT-08 @wip @slice-207
  Scenario: A head that is not the tip of the item's branch on the remote is refused
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-head-not-pushed"'s "message" says "slice-9"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    Given the clone switches to its branch "slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the clone has fetched the remote
    And the remote's branch "slice-9" has gained the commit "docs: add a note" touching "notes.md"
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-head-not-pushed"
    And its output says "the tip is " with the full SHA of the head of the remote's branch "slice-9"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    Given the full SHA of the head of the remote's branch "slice-9" is saved as "head"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    And its JSON's "head" matches the one saved as "head"

  # The first head's one scenario is another item's (@slice-90, which a
  # prefix match would take for slice-9's), and the notes claim otherwise;
  # the second head tags one for slice-9 but live, which a drafter may not
  # make.
  @ID-ATTEMPT-09 @wip @slice-207
  Scenario: A drafter's head with no wip scenario tagged for the item is refused
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-90"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>","notes":"@ID-A-01 is tagged @wip @slice-9 in features/a.feature."}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-spec-missing"'s "message" says "slice-9"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    Given the clone switches to its branch "slice-9"
    And the committed feature file "features/b.feature" with the live scenario "@ID-B-01" tagged "@slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-spec-missing"
    And its output says "@ID-B-01"
    And its output says "@wip"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist

  # Past base, the feat commit comes first and the test commit after it:
  # the scenario went live before any test could be red. The spec's own
  # commit, "test: a tagged scenario", is at base, so a build that judged
  # the commits past main would find a test commit first and accept the
  # head; one that looked for a test commit anywhere would too.
  @ID-ATTEMPT-10 @wip @slice-207
  Scenario: An implementer's head with no test commit past base before its first feat is refused
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    When itos runs "brief slice-9 --role implementer --json --at origin/slice-9"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone switches to its branch "slice-9"
    And the commit "feat: say the next step" turning the scenario "@ID-A-01" live on top of it
    And the commit "test: add the steps" on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-not-red-first"'s "message" says "feat: say the next step"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-not-red-first"

  # The test commit comes first and the feat commit after it, but the
  # feat touches the README alone: @ID-A-01 is still @wip at the head.
  @ID-ATTEMPT-11 @wip @slice-207
  Scenario: An implementer's head that leaves a scenario of the item wip is refused
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    When itos runs "brief slice-9 --role implementer --json --at origin/slice-9"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone switches to its branch "slice-9"
    And the commit "test: add the steps" on top of it
    And the commit "feat: say the next step" on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-still-wip"'s "message" says "@ID-A-01"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-still-wip"
    And its output says "@ID-A-01"

  # The implementer is briefed at the spec commit on the clone's own
  # branch, which the remote never gets, and the results claim no head: no
  # claim is checked, each is taken in with its notes, and the attempts
  # count up for the role. (That nothing is fetched for these is not
  # proved here: the remote lacking the branch refuses nothing.)
  @ID-ATTEMPT-12 @wip @slice-207
  Scenario: A blocked, needs_decision or spec_defect result is accepted without a head
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":1,"outcome":"blocked","notes":"The step needs a decision on the footer."}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "accepted"
    And its JSON's "role" is "implementer"
    And its JSON's "attempt" is 1
    And its JSON's "base" matches the one saved as "base"
    And its JSON's "head" is null
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":2,"outcome":"needs_decision"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And stdout contains the complete line "accepted slice-9's implementer attempt 2, needs_decision" exactly once
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","base":"<base>","attempt":3,"outcome":"spec_defect"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And stdout contains the complete line "accepted slice-9's implementer attempt 3, spec_defect" exactly once
    And ".git/itos/attempts/slice-9.jsonl" has exactly 3 lines
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "blocked"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is null
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "notes" is "The step needs a decision on the footer."
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "needs_decision"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "notes" is null
    And line 3 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "spec_defect"
    And line 3 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 3

  # The fetch moves the clone's remote-tracking ref and the journal is
  # written, so the whole-clone comparison the read-only commands use
  # cannot serve; what is checked instead is each thing it must leave
  # alone: the tree and index, the registry, the clone's HEAD, every ref
  # of the clone but the one the fetch refreshes, every ref of the remote
  # but the item's branch, which must end as it was, and a gh on the PATH
  # that records any call.
  @ID-ATTEMPT-13 @wip @slice-207
  Scenario: itos attempt submit writes only the journal, commits nothing, pushes nothing and asks no forge
    Given a recording gh on the PATH, which opens pull requests as "https://github.com/o/r/pull/1"
    And the commit "chore: tidy the readme" on top of it
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    And the JSON's "base" is saved as "base"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","base":"<base>","attempt":1,"outcome":"done","head":"<head>"}
      """
    And the refs of the clone and of the remote are saved for comparison
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And its output says "accepted slice-9's drafter attempt 1"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And git reports no change to the working tree or the index
    And the registry is unchanged
    And the clone's HEAD is the commit "chore: tidy the readme", with no rebase in progress
    And the clone's refs match the saved ones, all but "refs/remotes/origin/slice-9"
    And the remote's refs match the saved ones, all but "refs/heads/slice-9"
    And the remote's branch "slice-9" ends with "chore: tidy the readme" then "test: a tagged scenario", with no merge commit
    And the recording gh was never called

  # An unknown item is refused before the result is read, so the file need
  # not exist; a file that cannot be read names no role, so no journal
  # starts.
  @ID-ATTEMPT-14 @wip @slice-207
  Scenario: An unknown item, an unreadable result file and a missing argument are refused
    When itos runs "attempt submit slice-7 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-unknown-item"'s "message" says "slice-7"
    When itos runs "attempt submit slice-9 missing.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "missing.json"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    When itos runs "attempt submit slice-9"
    Then itos exits with code 2
    When itos runs "attempt submit"
    Then itos exits with code 2

  # This extends slice-204's itos brief (features/brief.feature) with what
  # attempt submit binds a result to: base, and --at. The brief at HEAD
  # names HEAD; once AGENTS.md is reworded on top, the brief at the new
  # HEAD names it with another request, and --at the older commit gives
  # that commit's brief again, its base and request those of the first
  # run. A rev git cannot resolve is refused.
  @ID-ATTEMPT-15 @wip @slice-207
  Scenario: itos brief --json names the commit it was computed at, and --at computes it at another
    Given the last commit SHA is saved as "first"
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And its JSON's "base" matches the one saved as "first"
    And the JSON's "request" is saved as "request-first"
    Given the committed file "AGENTS.md" holding "Stage only your own files, by path."
    And the last commit SHA is saved as "second"
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And its JSON's "base" matches the one saved as "second"
    And its JSON's "request" differs from the one saved as "request-first"
    When itos runs "brief slice-9 --role drafter --json --at HEAD~1"
    Then itos exits with code 0
    And its JSON's "base" matches the one saved as "first"
    And its JSON's "request" matches the one saved as "request-first"
    And its JSON's "reads" entry whose "path" is "AGENTS.md" has "blob" the git blob id of "AGENTS.md" at the commit saved as "first"
    When itos runs "brief slice-9 --role drafter --at HEAD~1"
    Then itos exits with code 0
    And its output says "Specify slice-9 (a slice) of"
    When itos runs "brief slice-9 --role drafter --json --at nowhere"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "brief-unknown-revision"'s "message" says "nowhere"
