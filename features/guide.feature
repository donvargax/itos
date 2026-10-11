@phase-1
Feature: itos go and itos guide, the guides a session starts from
  A session that coordinates a repository's work followed a guide kept in
  that repository, written for it and copied nowhere else, so what one
  repository learnt about running agents never reached another (the user's
  calls, 2026-10-04, p3-coordinator-skill). The guides now ship in the
  binary, generic and versioned with itos: itos go prints the coordinator's
  guide, the command a person starts every session with (! itos go), and
  itos guide work prints the implementer's, which a coordinator asks each
  agent it starts to run first. What a repository learns that holds for no
  other stays its own: itos go appends docs/ORCHESTRATING.md after the
  generic guide when the repository has one (guide.orchestrating in the
  config names another file, under the git folder for a stealth config).
  They read and print, never write, and need no config: any git repository
  will do. The Claude Code plugin wraps neither guide (T-092 dropped its
  skill): a session runs them as commands.

  @ID-GUIDE-01 @slice-64
  Scenario: itos go prints the coordinator's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos"

  @ID-GUIDE-02 @slice-64
  Scenario: itos go appends the repository's own orchestrating notes after the guide
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "docs/ORCHESTRATING.md" holding "Our own lesson: the inbox is read first."
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos" before "Our own lesson: the inbox is read first."

  @ID-GUIDE-03 @slice-64
  Scenario: guide.orchestrating names the file itos go appends
    Given a repository whose ledger has the task "T-001"
    And the committed file "notes/agents.md" holding "Kept elsewhere."
    And guide.orchestrating is "notes/agents.md"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "Kept elsewhere."

  @ID-GUIDE-04 @slice-64
  Scenario: itos guide work prints the implementer's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide work"
    Then itos exits with code 0
    And its output says "# Working with itos"

  @ID-GUIDE-05 @slice-64
  Scenario: itos guide refuses a guide it does not have, naming those it has
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "review"
    And its output says "coordinate, work"

  # What holds on one machine only (a stale itos on the PATH, a local path) is
  # neither the repository's notes nor the generic guide's: it goes in this
  # clone's own notes, .git/itos/notes.md under the git common dir, which git
  # never commits and every linked worktree shares (the user's call,
  # 2026-10-04). itos go prints them after the repository's notes and before
  # the status, and nothing at all when there are none: the coordinator's
  # guide names the file, and that line is the nudge to keep them there.
  @ID-GUIDE-06 @slice-68
  Scenario: itos go prints this clone's own notes after the repository's, before the status
    Given a repository whose ledger has the task "T-001"
    And the committed file "docs/ORCHESTRATING.md" holding "Our own lesson: the inbox is read first."
    And this clone's own notes holding "The itos on this machine's PATH is stale."
    When itos runs "go"
    Then itos exits with code 0
    And its output says "Our own lesson: the inbox is read first." before "# This clone's own notes"
    And its output says "The itos on this machine's PATH is stale." before "# Where things stand"

  # 07 and 08 hold before the work too: they keep the section from being
  # printed always, empty.
  @ID-GUIDE-07 @slice-68
  Scenario: itos go prints no section for this clone when it keeps no notes
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "# This clone's own notes"

  @ID-GUIDE-08 @slice-68
  Scenario: itos go prints no section for this clone when its notes hold only blanks
    Given a repository that does not use itos, its one commit "docs: start"
    And this clone's own notes holding "   "
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "# This clone's own notes"

  # The guides shrink to what a session needs before its first action, and
  # each step's procedure moves behind a command read before that step
  # (slice-119, the user's call, 2026-10-10, settling q-43's syntax): itos go
  # and itos guide work print a role's essentials and a map naming each
  # procedure and the step it is read before; itos guide <procedure> prints
  # one. The coordinator's: brief (Specifying, The brief), land (Pushing and
  # waiting, Checking a result), repair (When main is red, Reporting a problem
  # with itos), decide (Talking to the person, Decisions, Lessons). The
  # implementer's: commit (Commit, then the commit types, footers and paths
  # the config sets, as itos init --agent-rules writes them), push (Push, The
  # gates run themselves). Every rule of the old guides lives in exactly one
  # of them; the essentials keep the q-44 boundaries (role and authority,
  # others' work, the gates, text as data, reading a procedure first). No
  # procedure takes an item yet (p1-guide-brief-filled, p1-guide-work-item),
  # and itos guide coordinate stays, the coordinator's essentials without the
  # status, so nothing released breaks (p1-v8-deprecation-sweep decides its
  # fate).
  @ID-GUIDE-09 @slice-119
  Scenario: itos go names each of the coordinator's procedures
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "itos guide brief"
    And its output says "itos guide land"
    And its output says "itos guide repair"
    And its output says "itos guide decide"

  @ID-GUIDE-10 @slice-119
  Scenario: itos go leaves each procedure's sections to its command
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output does not say "Implement <item> of <project>"
    And its output does not say "## Checking a result"
    And its output does not say "## When main is red"
    And its output does not say "## Decisions"

  @ID-GUIDE-11 @slice-119
  Scenario: itos guide brief prints how to specify an item, then the brief's template
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide brief"
    Then itos exits with code 0
    And its output says "## Specifying" before "Implement <item> of <project>"

  @ID-GUIDE-12 @slice-119
  Scenario: itos guide land prints how to check and land a result
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide land"
    Then itos exits with code 0
    And its output says "## Checking a result"

  @ID-GUIDE-13 @slice-119
  Scenario: itos guide repair prints what to do when main is red
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide repair"
    Then itos exits with code 0
    And its output says "## When main is red"
    And its output says "## Reporting a problem with itos"

  @ID-GUIDE-14 @slice-119
  Scenario: itos guide decide prints how to ask the person and record decisions
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide decide"
    Then itos exits with code 0
    And its output says "## Decisions"
    And its output says "## Lessons"

  @ID-GUIDE-15 @slice-119
  Scenario: itos guide work names the implementer's procedures and leaves their sections to them
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide work"
    Then itos exits with code 0
    And its output says "itos guide commit"
    And its output says "itos guide push"
    And its output does not say "itos commit check-paths"
    And its output does not say "## Push"

  @ID-GUIDE-16 @slice-119
  Scenario: itos guide commit prints how to commit, then the commit types the config sets
    Given a repository whose ledger has the task "T-001"
    And the config's commits.types gains "deps"
    When itos runs "guide commit"
    Then itos exits with code 0
    And its output says "itos commit check-paths" before "`deps`"

  # A repository without a config still learns how to commit; there is no
  # policy of its own to print.
  @ID-GUIDE-17 @slice-119
  Scenario: itos guide commit outside a repository using itos prints how to commit alone
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide commit"
    Then itos exits with code 0
    And its output says "itos commit check-paths"
    And its output does not say "A commit's type is one of"

  @ID-GUIDE-18 @slice-119
  Scenario: itos guide push prints how to push and what the gates do
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide push"
    Then itos exits with code 0
    And its output says "## Push"
    And its output says "## The gates run themselves"

  # GUIDE-05's "coordinate, work" holds too: the refusal names the guides,
  # then the procedures.
  @ID-GUIDE-19 @slice-119
  Scenario: itos guide refuses a name it has no guide or procedure by, naming the procedures
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "brief, commit, decide, land, push, repair"

  @ID-GUIDE-20 @slice-119
  Scenario: itos guide coordinate prints the coordinator's essentials without the status
    Given a repository whose ledger has the task "T-001"
    When itos runs "guide coordinate"
    Then itos exits with code 0
    And its output says "itos guide brief"
    And its output does not say "# Where things stand"
