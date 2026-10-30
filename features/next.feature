@phase-1
Feature: itos next, an item's state and next step from the facts in the repository
  The first slice of the deterministic coordinator (docs/plans/v9-coordinator.md):
  a driver asks itos for an item's next step, runs it, and asks again, so no
  session has to remember where an item stands. itos next <id> reads the
  registry, the specs (the scenarios tagged @<id> in the scenario kind's
  feature files, @wip ones included), the ledger and the open questions of
  itos decision, and prints the item's state and who acts next. It writes no
  file, makes no commit, and asks no forge and no remote (no gh, no fetch).

  The states, the first that holds winning:
  done (status done), parked (dropped, or deferred), blocked (an open
  question names the item), waiting (an item in its depends_on is not done),
  idea (kind idea), needs-spec (a slice or a bug no scenario is tagged for,
  or a task whose ledger entry has no done_when check), in-flight (a spec
  present, or status doing). An item with no kind is read by its id, as
  itos work show reads it: slice-<n> a slice, bug-<n> a bug, an id ledger.id
  matches a task. In-flight is this slice's stand-in: the next slice
  replaces it with the states only the forge knows (spec review,
  implementing, merging, merged).

  The next step is an actor and an action. Actors: driver, drafter,
  reviewer, implementer, answerer, person, forge, none. Actions: promote,
  specify, review, take, implement, land, close, answer, wait, observe,
  none. This slice gives done and parked none and none, blocked person
  answer, waiting none wait, idea person promote, needs-spec drafter
  specify, in-flight forge observe.

  Text: one line, "<id>: <state>; next: <actor> <action>", then
  "; run: <command>" when the step is a deterministic itos command.
  --json: {"schema":1,"ok":true,"item","kind","state","owner","next":{"actor","action","argv"?},"waits_on"?,"blocked_reason"?,"revision"},
  owner a handle or null; argv only for a deterministic itos command
  (needs-spec's is itos guide brief); waits_on the question ids a blocked
  item waits on, or the item ids a waiting one does, only what its state
  waits on; blocked_reason "decision" on a blocked item alone. revision is
  "sha256:" and the hex digest of exactly the facts read: the item's
  registry entry as written, its dependencies' statuses, the open questions
  naming it, its tagged scenarios and its ledger checks, so a commit that
  touches none of them leaves it as it was. Every state exits 0. An id the
  registry does not have is refused, exit 1, rule work-unknown-item, as
  itos work show refuses it: {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}.

  Background:
    Given a repository whose ledger has the task "T-001"

  # The owner is a handle, so a build that printed null for every owner
  # fails; the item has no scenario, so a build that judged needs-spec
  # before done fails too.
  @ID-NEXT-01 @wip @slice-203
  Scenario: A done item needs no one
    Given the work registry has the item "slice-9" owned by "ana" with the status "done"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "ok" is true
    And its JSON's "item" is "slice-9"
    And its JSON's "kind" is "slice"
    And its JSON's "state" is "done"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "none", "action": "none"}
    And its JSON has no "waits_on"

  # Both items lack a spec, so a build that ignored dropped or deferred
  # would say needs-spec.
  @ID-NEXT-02 @wip @slice-203
  Scenario: A dropped item and a deferred one are parked
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - id: slice-8
          title: Dropped
          phase: 1
          owner: null
          status: dropped
          depends_on: []
        - id: slice-9
          title: Deferred
          phase: 1
          owner: null
          status: todo
          depends_on: []
          deferred: until the next major release
      """
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "parked"
    And its JSON's "next" is {"actor": "none", "action": "none"}
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "parked"
    And its JSON's "next" is {"actor": "none", "action": "none"}

  # q-1 names the item but is answered and q-2 is open but names another
  # item: only q-3 holds it up, so waits_on is exactly ["q-3"].
  @ID-NEXT-03 @wip @slice-203
  Scenario: An open question naming the item blocks it on the person
    Given the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-9"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "slice-8"
          question: "Why not?"
        - id: q-3
          item: "slice-9"
          question: "When?"
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "waits_on" is ["q-3"]
    And its JSON's "blocked_reason" is "decision"

  # slice-7 is done and slice-8 is not: waits_on names slice-8 alone. The
  # item has no spec, so waiting is judged before needs-spec.
  @ID-NEXT-04 @wip @slice-203
  Scenario: An item whose dependency is not done waits for it
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-7, title: First, phase: 1, owner: null, status: done, depends_on: [] }
        - { id: slice-8, title: Second, phase: 1, owner: null, status: todo, depends_on: [] }
        - { id: slice-9, title: Third, phase: 1, owner: null, status: todo, depends_on: [slice-7, slice-8] }
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "waiting"
    And its JSON's "next" is {"actor": "none", "action": "wait"}
    And its JSON's "waits_on" is ["slice-8"]
    And its JSON has no "blocked_reason"

  @ID-NEXT-05 @wip @slice-203
  Scenario: An item both waiting on a dependency and named by an open question is blocked
    Given the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo", depending on "slice-8"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-9"
          question: "Which way?"
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "waits_on" is ["q-1"]

  # Which kind an idea becomes is a judgement, so the step has no command:
  # no argv, and no "; run:" in the text.
  @ID-NEXT-06 @wip @slice-203
  Scenario: An idea waits on the person to promote it
    Given the work registry has the idea "p1-next" owned by nobody
    When itos runs "next p1-next"
    Then itos exits with code 0
    And stdout contains the complete line "p1-next: idea; next: person promote" exactly once
    When itos runs "next p1-next --json"
    Then itos exits with code 0
    And its JSON's "kind" is "idea"
    And its JSON's "state" is "idea"
    And its JSON's "owner" is null
    And its JSON's "next" is {"actor": "person", "action": "promote"}

  # The scenario tagged @slice-90 is another item's: a build that matched
  # the tag by its prefix would find a spec for slice-9. T-001's ledger
  # entry, the Background's, has no done_when.
  @ID-NEXT-07 @wip @slice-203
  Scenario: A slice with no scenario and a task with no checks need a spec
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-90"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: needs-spec; next: drafter specify; run: itos guide brief" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "guide", "brief"]}
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "task"
    And its JSON's "state" is "needs-spec"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "guide", "brief"]}

  # A @wip scenario is a spec as much as a live one. slice-9 is doing and
  # someone's, T-001 todo and no one's: both are in flight, each with its
  # own owner.
  @ID-NEXT-08 @wip @slice-203
  Scenario: A slice with a scenario and a task with a check are in flight
    Given the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And the task "T-001" has the committed check "true"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's "owner" is null
    And its JSON's "next" is {"actor": "forge", "action": "observe"}

  # The commit between the runs touches the README alone, which is none of
  # the item's facts: a revision of HEAD, or of the whole tree, differs.
  @ID-NEXT-09 @wip @slice-203
  Scenario: The same facts give the same revision
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is "sha256:" and 64 lowercase hex digits
    And its JSON's revision is saved as "first"
    When the commit "chore: tidy the readme" on top of it
    And itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is the one saved as "first"

  # Neither change moves the state, which stays in-flight: a revision of
  # the state alone gives the same one each time.
  @ID-NEXT-10 @wip @slice-203
  Scenario: A change to the item's registry entry or a new scenario tagged for it gives a new revision
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is saved as "first"
    When itos has run "work edit slice-9 --title 'Say the next step'"
    And itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's revision differs from the one saved as "first"
    And its JSON's revision is saved as "second"
    When the committed feature file "features/b.feature" with the live scenario "@ID-B-01" tagged "@slice-9"
    And itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's revision differs from the one saved as "second"

  @ID-NEXT-11 @wip @slice-203
  Scenario: An id the registry does not have is refused
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "next slice-7 --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And its JSON's problem "work-unknown-item"'s "message" says "slice-7"
    When itos runs "next slice-7"
    Then itos exits with code 1
    And its output says "slice-7"

  # The saved state is every file of the clone, its git folder included,
  # so HEAD, the index and anything itos might keep in .git/itos are
  # compared. A gh on the PATH records any call, so asking the forge shows.
  @ID-NEXT-12 @wip @slice-203
  Scenario: itos next writes nothing, commits nothing and asks no forge
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And a recording gh on the PATH, which opens pull requests as "https://github.com/o/r/pull/1"
    And the repository state is saved for comparison
    When itos runs "next slice-9"
    Then itos exits with code 0
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And the repository files index and HEAD match the saved state
    And git reports no change to the working tree or the index
    And the recording gh was never called
