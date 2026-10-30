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
  <role> --json gives), attempt (a positive integer), outcome (done,
  blocked, needs_decision or spec_defect), head (a full lowercase SHA,
  required with done and refused with any other outcome, which claims no
  head) and notes (a string, optional, never judged). Any other key, a
  missing required one, a wrong type or value, or a file that is not one
  JSON object is malformed.

  The checks, in this order, the first failing one refusing with exit 1
  and {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}, the
  text output saying the message:
  attempt-malformed, the message naming what is wrong;
  attempt-stale, the result's item not the one on the command line, or its
  request not the one itos brief computes now for that item and role at
  the clone's HEAD (the brief changed since the agent got it: a read's
  blob, the spec commit), the fix saying to brief again;
  attempt-out-of-order, the attempt not one more than the results the
  journal holds for that item and role, the message naming the attempt
  expected; then, for outcome done, the claims, each checked against git
  and never against notes:
  attempt-head-not-pushed, the head not the tip of the item's branch on
  the remote (the branch named for the item, fetched first, so a
  remote-tracking ref the clone has not refreshed does not answer), the
  message naming the tip when the branch has one;
  attempt-spec-missing, a drafter's head whose tree holds no scenario
  tagged @<id>, or one that is not @wip, the message naming it;
  attempt-not-red-first, an implementer's head whose commits past the
  remote's main hold no test commit before the first feat or fix, the
  message naming that commit;
  attempt-still-wip, an implementer's head whose tree still holds a
  scenario tagged @<id> and @wip, the message naming it.
  The scenarios at a head are read from its tree, as itos work done reads
  them at HEAD; the commit types are the headers' as the message rules
  read them.

  The journal is .git/itos/attempts/<id>.jsonl in the git folder (the
  common dir of a linked worktree), one line per result taken in:
  {"schema":1,"role","attempt","request","outcome","head","verdict"},
  head null when the result has none. An accepted result is recorded with
  the verdict accepted. A malformed result naming a role is recorded too,
  with the verdict rotated, its attempt the number the journal gives it,
  so it counts and the agent's next try is one higher; one naming no role
  cannot be counted for anyone and is refused without a record. Stale,
  out-of-order and claim refusals record nothing: the same attempt is
  resubmitted once fixed. A result the journal already holds (the same
  role, request, attempt and head, whatever its notes) is already, exit 0,
  nothing recorded. The outcomes blocked, needs_decision and spec_defect
  are accepted with no claim checked and no fetch; recording them is how
  itos next will route the item later.

  --json on success: {"schema":1,"ok":true,"outcome":"accepted|already","item","role","attempt","request","head"},
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
    And the work registry has the item "slice-9" owned by nobody with the status "todo"

  # The drafter is briefed on main, where no scenario is tagged yet, and
  # pushes the spec on the item's branch; the clone's main has not moved,
  # so the request the result echoes is still the one itos computes. The
  # notes are prose and decide nothing. The journal line's request and
  # head are compared with the values the scenario saved, so a build that
  # recorded the attempt alone fails.
  @ID-ATTEMPT-01 @wip @slice-207
  Scenario: A drafter's done result is accepted and recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>","notes":"The spec is pushed."}
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
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "request" is the value saved as "request"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is the value saved as "head"

  # The implementer is briefed on the item's branch at the spec commit,
  # where the driver's clone stays while the work is pushed from the
  # branch "work": a test commit, then the feat commit that turns the
  # scenario live. The request is recomputed at that HEAD, the spec
  # commit still the brief's.
  @ID-ATTEMPT-02 @wip @slice-207
  Scenario: An implementer's done result is accepted, its JSON echoing the result
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "work"
    And the commit "test: add the steps" on top of it
    And the commit "feat: say the next step" turning the scenario "@ID-A-01" live on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "slice-9"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "accepted"
    And its JSON's "item" is "slice-9"
    And its JSON's "role" is "implementer"
    And its JSON's "attempt" is 1
    And its JSON's "request" matches the one saved as "request"
    And its JSON's "head" matches the one saved as "head"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "role" is "implementer"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is the value saved as "head"

  # The second file differs from the first in its notes alone: the same
  # role, request, attempt and head is the same result.
  @ID-ATTEMPT-03 @wip @slice-207
  Scenario: The same accepted result again is already, nothing recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And its output says "accepted slice-9's drafter attempt 1, done at "
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>","notes":"Sent twice."}
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
  # recorded, and the corrected result must be attempt 2. The spec is
  # pushed, so nothing but the key stands between the agent and acceptance.
  @ID-ATTEMPT-04 @wip @slice-207
  Scenario: A malformed result is rotated and the next try is one attempt higher
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>","commits":["<head>"]}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "commits"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "rotated"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "role" is "drafter"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 1
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-out-of-order"'s "message" says "2"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    And its JSON's "attempt" is 2
    And ".git/itos/attempts/slice-9.jsonl" has exactly 2 lines
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 2

  # Seven shapes name a role and are each rotated; the eighth is no JSON
  # object at all, names no role, and is refused without a record: seven
  # lines in the end. Each message names the key at fault. The head is the
  # pushed tip, so only the shape refuses.
  @ID-ATTEMPT-05 @wip @slice-207
  Scenario: Each malformed shape is refused, naming the key at fault
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "request"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":"1","outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "attempt"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":3,"outcome":"finished","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "outcome"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":2,"item":"slice-9","role":"drafter","request":"<request>","attempt":4,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "schema"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":5,"outcome":"done"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":6,"outcome":"done","head":"abc1234"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":7,"outcome":"blocked","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-malformed"'s "message" says "head"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 7 lines
    And line 7 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "rotated"
    And line 7 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 7
    Given the file "result.json" holds the lines:
      """
      {"schema":1,"item":"slice-9",
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-malformed"'s "message" says "result.json"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 7 lines
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-malformed"
    And its output says "result.json"

  # AGENTS.md, a read of the brief, is reworded on main after the brief:
  # the request itos computes now differs. The result is attempt 2 as
  # well, so a build that judged the order first fails: stale is the one
  # problem. Then, re-briefed, a result naming another item is stale too,
  # the request being the item on the command line's.
  @ID-ATTEMPT-06 @wip @slice-207
  Scenario: A result whose request is no longer the brief's is stale, nothing recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the committed file "AGENTS.md" holding "Stage only your own files, by path."
    And the last commit SHA is saved as "head"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's "problems" has exactly 1 entry
    And its JSON's problem "attempt-stale"'s "message" says "slice-9"
    And its JSON's problem "attempt-stale"'s "fix" says "itos brief slice-9 --role drafter"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-stale"
    And its output says "itos brief slice-9 --role drafter"
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-8","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And its JSON's problem "attempt-stale"'s "message" says "slice-8"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist

  # Nothing is recorded yet, so the first attempt is 1; once it is taken
  # in, the next is 2, and 3 is as out of order as 2 was. The head is the
  # pushed tip and the request current, so only the number refuses.
  @ID-ATTEMPT-07 @wip @slice-207
  Scenario: An attempt that is not the next one is refused, nothing recorded
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":2,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "attempt-out-of-order"'s "message" says "1"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And its JSON's "outcome" is "accepted"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":3,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-out-of-order"
    And its output says "2"
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
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
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
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
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
    Given the clone is on the new branch "slice-9"
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-90"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>","notes":"@ID-A-01 is tagged @wip @slice-9 in features/a.feature."}
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
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 1
    And its output says "attempt-spec-missing"
    And its output says "@ID-B-01"
    And its output says "@wip"
    And the file ".git/itos/attempts/slice-9.jsonl" does not exist

  # The feat commit comes first and the test commit after it: the
  # scenario went live before any test could be red. Nothing else is
  # wrong with the head, so a build that read the commits in the other
  # order, or only looked for a test commit somewhere, accepts it.
  @ID-ATTEMPT-10 @wip @slice-207
  Scenario: An implementer's head with no test commit before its first feat is refused
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "work"
    And the commit "feat: say the next step" turning the scenario "@ID-A-01" live on top of it
    And the commit "test: add the steps" on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "slice-9"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
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
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "work"
    And the commit "test: add the steps" on top of it
    And the commit "feat: say the next step" on top of it
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "slice-9"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
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

  # The remote has no branch for the item and the results claim no head:
  # nothing is fetched and no claim is checked, each is taken in, and the
  # attempts count up for the role.
  @ID-ATTEMPT-12 @wip @slice-207
  Scenario: A blocked, needs_decision or spec_defect result is accepted without a head
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "brief slice-9 --role implementer --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":1,"outcome":"blocked","notes":"The step needs a decision on the footer."}
      """
    When itos runs "attempt submit slice-9 result.json --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And its JSON's "outcome" is "accepted"
    And its JSON's "role" is "implementer"
    And its JSON's "attempt" is 1
    And its JSON's "head" is null
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":2,"outcome":"needs_decision"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And stdout contains the complete line "accepted slice-9's implementer attempt 2, needs_decision" exactly once
    Given the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"implementer","request":"<request>","attempt":3,"outcome":"spec_defect"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And stdout contains the complete line "accepted slice-9's implementer attempt 3, spec_defect" exactly once
    And ".git/itos/attempts/slice-9.jsonl" has exactly 3 lines
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "blocked"
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "head" is null
    And line 1 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "verdict" is "accepted"
    And line 2 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "needs_decision"
    And line 3 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "outcome" is "spec_defect"
    And line 3 of the file ".git/itos/attempts/slice-9.jsonl" is a JSON object whose "attempt" is 3

  # The fetch moves the clone's remote-tracking ref and the journal is
  # written, so the whole-clone comparison the read-only commands use
  # cannot serve; what is checked instead is each thing it must leave
  # alone: the tree and index, the registry, the clone's HEAD, the remote's
  # branch, and a gh on the PATH that records any call.
  @ID-ATTEMPT-13 @wip @slice-207
  Scenario: itos attempt submit writes only the journal, commits nothing, pushes nothing and asks no forge
    Given a recording gh on the PATH, which opens pull requests as "https://github.com/o/r/pull/1"
    And the commit "chore: tidy the readme" on top of it
    When itos runs "brief slice-9 --role drafter --json"
    Then itos exits with code 0
    And the JSON's "request" is saved as "request"
    Given the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "head"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the clone switches to its branch "main"
    And the file "result.json" holds the lines, the saved values filled in:
      """
      {"schema":1,"item":"slice-9","role":"drafter","request":"<request>","attempt":1,"outcome":"done","head":"<head>"}
      """
    When itos runs "attempt submit slice-9 result.json"
    Then itos exits with code 0
    And its output says "accepted slice-9's drafter attempt 1"
    And ".git/itos/attempts/slice-9.jsonl" has exactly 1 line
    And git reports no change to the working tree or the index
    And the registry is unchanged
    And the clone's HEAD is the commit "chore: tidy the readme", with no rebase in progress
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
