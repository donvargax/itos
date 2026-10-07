@phase-1
Feature: itos draft, the coordinator's pending changes kept until no work is going on
  While an agent holds the checkout, the coordinator may not commit in it:
  a commit takes the whole index, the agent's staged files too, and a
  tracked file left changed stops the agent's itos push. So the specs,
  ideas and registry edits it writes meanwhile were kept in session scratch
  files, lost with the session and seen by no one (the user's call,
  2026-10-06, p1-drafts). itos draft keeps them instead, clone-local under
  itos's folder of the git common dir, as follow-ups are, never committed
  and shared by every worktree. A draft is one of two things: a change to
  files with the message to commit it with (-m), or an itos command line to run
  later (a work add, a work queue, a task add), which commits itself. A
  change is taken out of the working tree when it is drafted, so the
  checkout stays the agent's. itos draft promote applies the drafts in the
  order they were added, each through the hooks as any commit, and only
  when the checkout is clean: no tracked file has a change no commit holds,
  staged or not, and no rebase or merge is in progress: drafting never
  touches an agent's work, and the guide's
  "commit only between agents" becomes a gate. It stops at the first draft
  that cannot be applied, keeping it and those after it. itos status names
  the drafts waiting.

  # The registry lists a phase, so the drafted work add's item has one to
  # go in (work add refuses an item no phase holds).
  Background:
    Given a repository whose ledger has the task "T-001"
    And the work registry has the item "slice-1" owned by nobody with the status "done"

  @ID-DRAFT-01 @slice-96
  Scenario: draft add keeps an itos command line to run later, and git sees nothing
    When itos runs the command line "draft add queue-t1 -- work add p1-thing --title 'A thing' --why 'Because.'"
    Then itos exits with code 0
    And git status shows nothing to commit
    When itos runs "draft"
    Then itos exits with code 0
    And its output says "queue-t1"
    And its output says "work add p1-thing"

  @ID-DRAFT-02 @slice-96
  Scenario: draft add keeps a change to files with its message, and puts the files back as HEAD has them
    Given the committed file "notes.md" holding "Old."
    And the file "notes.md" is changed to hold "New."
    When itos runs the command line "draft add say-new -m 'docs: say new' notes.md"
    Then itos exits with code 0
    And the file "notes.md" says "Old."
    And git status shows nothing to commit
    When itos runs "draft"
    Then itos exits with code 0
    And its output says "say-new"
    And its output says "docs: say new"

  @ID-DRAFT-03 @slice-96
  Scenario: draft promote applies the drafts in the order they were added, each committed, and none is left
    Given the committed file "notes.md" holding "Old."
    And the file "notes.md" is changed to hold "New."
    And itos has run the command line "draft add say-new -m 'docs: say new' notes.md"
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    When itos runs "draft promote"
    Then itos exits with code 0
    And the file "notes.md" says "New."
    And the registry's item "p1-thing" is an idea titled "A thing" with the status "todo"
    And the last commit's header is "docs: add p1-thing"
    And the commit before the last has the header "docs: say new"
    And git status shows nothing to commit
    When itos runs "draft"
    Then itos exits with code 0
    And its output does not say "say-new"
    And its output does not say "add-thing"

  # An item's status is no gate (the user's call, 2026-10-06): items stay
  # doing for the coordinator's own work and for agents that have gone, so
  # "no item doing" deadlocked, a draft closing a doing item never promoted.
  # What collides with an agent is its index and work tree, which the clean
  # checkout guards.
  @ID-DRAFT-04 @slice-96
  Scenario: draft promote applies the drafts while an item is doing, an item's status being no gate
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    When itos runs "draft promote"
    Then itos exits with code 0
    And the registry's item "p1-thing" is an idea titled "A thing" with the status "todo"

  @ID-DRAFT-05 @slice-96
  Scenario: draft promote refuses while a tracked file has a change no commit holds, and applies nothing
    Given the committed file "notes.md" holding "Old."
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    And the file "notes.md" is changed to hold "Someone's work."
    When itos runs "draft promote"
    Then itos exits with code 1
    And its output says "notes.md"
    And the registry has no item "p1-thing"
    And the file "notes.md" says "Someone's work."

  # The tree moved since the draft was taken: what applied before it stays
  # committed, and it and the drafts after it wait, for the coordinator to
  # redo or drop.
  @ID-DRAFT-06 @slice-96
  Scenario: draft promote stops at a change that no longer applies, keeping it and the drafts after it
    Given the committed file "notes.md" holding "Old."
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    And the file "notes.md" is changed to hold "New."
    And itos has run the command line "draft add say-new -m 'docs: say new' notes.md"
    And itos has run the command line "draft add add-other -- work add p1-other --title 'Another' --why 'Because.'"
    And the committed file "notes.md" holding "Rewritten by someone."
    When itos runs "draft promote"
    Then itos exits with code 1
    And its output says "say-new"
    And the registry's item "p1-thing" is an idea titled "A thing" with the status "todo"
    And the registry has no item "p1-other"
    And the file "notes.md" says "Rewritten by someone."
    When itos runs "draft"
    Then itos exits with code 0
    And its output says "say-new" before "add-other"
    And its output does not say "add-thing"

  @ID-DRAFT-07 @slice-96
  Scenario: draft drop removes a draft without applying it
    Given itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    When itos runs "draft drop add-thing"
    Then itos exits with code 0
    And the registry has no item "p1-thing"
    When itos runs "draft"
    Then itos exits with code 0
    And its output does not say "add-thing"

  # Drafts live in the git common dir, so one drafted from an agent's
  # linked worktree is the main checkout's too.
  @ID-DRAFT-08 @slice-96
  Scenario: A draft added in a linked worktree is listed from the main checkout
    Given a linked worktree of the repository at "../wt"
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'" in the linked worktree
    When itos runs "draft"
    Then itos exits with code 0
    And its output says "add-thing"

  # A spec is often a new file, a feature file or a ledger, so a draft takes
  # files git does not track yet as well, and takes them out of the tree.
  @ID-DRAFT-10 @slice-96
  Scenario: A drafted new file leaves the working tree and comes back committed when promoted
    Given the untracked file "specs/new.md" holding "A new spec."
    When itos runs the command line "draft add new-spec -m 'docs: add a spec' specs/new.md"
    Then itos exits with code 0
    And the file "specs/new.md" does not exist
    When itos runs "draft promote"
    Then itos exits with code 0
    And the file "specs/new.md" says "A new spec."
    And the last commit's header is "docs: add a spec"

  @ID-DRAFT-11 @slice-96
  Scenario: itos status names the drafts waiting
    Given a clone of it, where itos runs
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    When itos runs "status --as someone"
    Then itos exits with code 0
    And its output says "add-thing"

  @ID-DRAFT-09 @slice-96
  Scenario: draft add refuses an id a draft already has
    Given itos has run the command line "draft add add-thing -- work add p1-thing --title 'A thing' --why 'Because.'"
    When itos runs the command line "draft add add-thing -- work add p1-other --title 'Another' --why 'Because.'"
    Then itos exits with code 1
    And its output says "add-thing"

  # A merge or a rebase stopped part way holds the index and the work tree
  # as much as a change does: a draft's commit there would conclude it. The
  # draft is a change, since a command's own commit refuses a merge itself.
  @ID-DRAFT-12 @slice-96
  Scenario: draft promote refuses while a rebase or a merge is in progress, and applies nothing
    Given the committed file "notes.md" holding "Old."
    And the file "notes.md" is changed to hold "New."
    And itos has run the command line "draft add say-new -m 'docs: say new' notes.md"
    And a merge is in progress, with no change to commit
    When itos runs "draft promote"
    Then itos exits with code 1
    And its output says "merge"
    And the file "notes.md" says "Old."
    And the last commit's header is not "docs: say new"
