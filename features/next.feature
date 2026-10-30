@phase-1
Feature: itos next, an item's state and next step from the facts in the repository
  The first slice of the deterministic coordinator (docs/plans/v9-coordinator.md):
  a driver asks itos for an item's next step, runs it, and asks again, so no
  session has to remember where an item stands. itos next <id> reads the
  registry, the specs (the scenarios tagged @<id> in the scenario kind's
  feature files, @wip ones included), the ledger and the open questions of
  itos decision, and prints the item's state and who acts next. It writes no
  file, makes no commit, and asks no forge and no remote (no gh, no fetch).
  Each fact is read where its existing reader reads it: the registry, the
  ledger and the questions from the working tree, the tagged scenarios at
  HEAD, as itos work done reads them.

  The states, the first that holds winning:
  done (status done), parked (dropped, or deferred), blocked (status
  blocked, or an open question names the item), waiting (an item in its
  depends_on is not done), idea (kind idea), needs-spec (a slice or a bug no
  scenario is tagged for, or a task whose ledger entry has no done_when
  check), in-flight (a spec present). A repository whose config declares no
  scenario kind has no scenario for any item, so a slice or a bug there
  needs a spec. An item with no kind is read by its id, by next's own rule:
  slice-<n> a slice, bug-<n> a bug, an id ledger.id matches a task.
  In-flight is this slice's stand-in: the next slice replaces it with the
  states only the forge knows (spec review, implementing, merging, merged).

  The next step is an actor and an action. Actors: driver, drafter,
  reviewer, implementer, answerer, person, forge, none. Actions: promote,
  specify, review, take, implement, land, close, answer, wait, observe,
  none. This slice gives done and parked none and none, blocked person
  answer, waiting none wait, idea person promote, needs-spec drafter
  specify, in-flight forge observe.

  Text: one line, "<id>: <state>; next: <actor> <action>", then
  "; run: <command>" when the step is a deterministic itos command.
  --json: {"schema":1,"ok":true,"item","kind","state","owner","next":{"actor","action","argv"?},"waits_on"?,"blocked_reason"?,"revision"},
  owner the item's own owner as written, null when it is null, never its
  group's; argv only for a deterministic itos command (needs-spec's is itos
  brief <id> --role drafter); waits_on only what its state waits on: the open questions'
  ids for an item a question blocks, the ids blocked_by lists for one whose
  status is blocked, none when its blocked_by is prose (as the registry's
  header describes it) or absent, the undone dependencies' ids for a
  waiting one; blocked_reason, on a blocked item alone, "decision" for an
  open question and "status" for the registry's status blocked, the open
  question winning when both hold. revision is
  "sha256:" and the hex digest of exactly the facts read: the item's
  registry entry as written, its dependencies' statuses, the open questions
  naming it, its tagged scenarios and its ledger checks, so a commit that
  touches none of them leaves it as it was. Every state exits 0. An id the
  registry does not have is refused, exit 1, rule work-unknown-item, as
  itos work show refuses it: {"schema":1,"ok":false,"problems":[{"rule","message","fix"}]}.

  Background:
    Given a repository whose ledger has the task "T-001"

  # The owner is a handle, so a build that printed null for every owner
  # fails; the item has no scenario and an open question names it, so a
  # build that judged needs-spec or blocked before done fails too.
  @ID-NEXT-01 @wip @slice-203
  Scenario: A done item needs no one
    Given the work registry has the item "slice-9" owned by "ana" with the status "done"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-9"
          question: "Which way?"
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "ok" is true
    And its JSON's "item" is "slice-9"
    And its JSON's "kind" is "slice"
    And its JSON's "state" is "done"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "none", "action": "none"}
    And its JSON has no "waits_on"

  # Both items lack a spec, an open question names the dropped one and the
  # deferred one depends on an item not done, so a build that judged
  # needs-spec, blocked or waiting before parked fails.
  @ID-NEXT-02 @wip @slice-203
  Scenario: A dropped item and a deferred one are parked
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - id: slice-7
          title: Undone
          phase: 1
          owner: null
          status: todo
          depends_on: []
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
          depends_on: [slice-7]
          deferred: until the next major release
      """
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-8"
          question: "Why drop it?"
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
  # entry, the Background's, has no done_when. slice-8 is doing and ana's:
  # a status says nothing of a spec, so it needs one too, its owner kept.
  @ID-NEXT-07 @wip @slice-203
  Scenario: A slice with no scenario, even one in progress, and a task with no checks need a spec
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by "ana" with the status "doing"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-90"
    When itos runs "next slice-9"
    Then itos exits with code 0
    And stdout contains the complete line "slice-9: needs-spec; next: drafter specify; run: itos brief slice-9 --role drafter" exactly once
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-9", "--role", "drafter"]}
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON's "owner" is "ana"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-8", "--role", "drafter"]}
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "task"
    And its JSON's "state" is "needs-spec"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "T-001", "--role", "drafter"]}

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

  # Between the runs, another item's title changes, an open question names
  # that other item, and a commit touches the README: none is a fact of
  # slice-9's, so a revision of HEAD, of the whole registry or of every
  # open question differs.
  @ID-NEXT-09 @wip @slice-203
  Scenario: The same facts give the same revision
    Given the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is "sha256:" and 64 lowercase hex digits
    And its JSON's revision is saved as "first"
    When itos has run "work edit slice-8 --title 'Another title'"
    And itos has run "decision add 'Which way?' --item slice-8"
    And the commit "chore: tidy the readme" on top of it
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
  # git status refreshes the index file, so itos next reads git through
  # tree reads alone, never git status.
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

  # bug-3 has no kind key, so its kind comes from its id. Until the
  # committed scenario, the config declares no scenario kind at all.
  @ID-NEXT-13 @wip @slice-203
  Scenario: A bug needs a spec until a scenario is tagged for it, then is in flight
    Given the work registry has the item "bug-3" owned by nobody with the status "todo"
    When itos runs "next bug-3 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "bug"
    And its JSON's "state" is "needs-spec"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "bug-3", "--role", "drafter"]}
    When the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@bug-3"
    And itos runs "next bug-3 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "bug"
    And its JSON's "state" is "in-flight"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}

  # slice-7 is not done. slice-8 depends on it but lists no blocked_by: it
  # is blocked, not waiting, and waits on nothing, since waits_on comes
  # from blocked_by alone. slice-9's blocked_by names slice-7; slice-10's is
  # prose, as the registry's header describes it, and names nothing.
  # slice-11 is both blocked and named by an open question: the question
  # wins, so its reason is decision and it waits on q-1, not on slice-7.
  @ID-NEXT-14 @wip @slice-203
  Scenario: The registry's status blocked blocks the item on the person, an open question first
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items:
        - id: slice-7
          title: Undone
          phase: 1
          owner: null
          status: todo
          depends_on: []
        - id: slice-8
          title: Blocked alone
          phase: 1
          owner: null
          status: blocked
          depends_on: [slice-7]
        - id: slice-9
          title: Blocked by another
          phase: 1
          owner: null
          status: blocked
          depends_on: []
          blocked_by: [slice-7]
        - id: slice-10
          title: Blocked outside
          phase: 1
          owner: null
          status: blocked
          depends_on: []
          blocked_by: waiting on the vendor
        - id: slice-11
          title: Blocked twice
          phase: 1
          owner: null
          status: blocked
          depends_on: []
          blocked_by: [slice-7]
      """
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "slice-11"
          question: "Which way?"
      """
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "blocked_reason" is "status"
    And its JSON has no "waits_on"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "blocked_reason" is "status"
    And its JSON's "waits_on" is ["slice-7"]
    When itos runs "next slice-10 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "blocked_reason" is "status"
    And its JSON has no "waits_on"
    When itos runs "next slice-11 --json"
    Then itos exits with code 0
    And its JSON's "state" is "blocked"
    And its JSON's "next" is {"actor": "person", "action": "answer"}
    And its JSON's "blocked_reason" is "decision"
    And its JSON's "waits_on" is ["q-1"]

  # The scenario kind is declared by a committed feature file tagged for
  # another item. The uncommitted feature file is not at HEAD, where the
  # tagged scenarios are read, so neither the state nor the revision moves;
  # the uncommitted registry is the working tree's, where the registry is
  # read, so both move.
  @ID-NEXT-15 @wip @slice-203
  Scenario: An uncommitted registry edit counts and an uncommitted feature file does not
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-90"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON's revision is saved as "first"
    When the feature file "features/c.feature" with the scenario "@ID-C-01 @slice-9"
    And itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "needs-spec"
    And its JSON's revision is the one saved as "first"
    When the file "tasks/work-items.yaml" holds the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-9, title: slice-9, phase: 1, owner: null, status: done, depends_on: [] }
      """
    And itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "done"
    And its JSON's revision differs from the one saved as "first"
