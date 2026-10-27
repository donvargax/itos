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

  # Slice 99 (the user's call, 2026-10-07; was p1-draft-edit-off-tree):
  # coordinators kept writing specs in scratch files while an agent ran,
  # though the guide says to draft them, because drafting a file change meant
  # first editing the file in the working tree, the agent's checkout, where it
  # stops the agent's push and can be taken by its commit. itos draft edit
  # <id> -m <message> <path>... gives the draft a copy of each path, HEAD's
  # content or empty for a new file, in the draft's own folder under the git
  # common dir, and prints where; the person edits the copies there, never the
  # tree. promote applies each copy's change against the HEAD it was taken
  # from, as for any change draft, so a tree that moved under it stops promote
  # at that draft. The working tree is neither read nor written until promote.
  @ID-DRAFT-13 @slice-99
  Scenario: draft edit gives the draft a copy of the file to edit, and touches nothing in the working tree
    Given the committed file "notes.md" holding "Old."
    And the file "notes.md" is changed to hold "An agent's work."
    When itos runs the command line "draft edit say-new -m 'docs: say new' notes.md"
    Then itos exits with code 0
    And the draft "say-new"'s copy of "notes.md" says "Old."
    And its output names the draft "say-new"'s copy of "notes.md"
    And the file "notes.md" says "An agent's work."

  @ID-DRAFT-14 @slice-99
  Scenario: draft promote commits the change made in the draft's copy, with its message
    Given the committed file "notes.md" holding "Old."
    And itos has run the command line "draft edit say-new -m 'docs: say new' notes.md"
    And the draft "say-new"'s copy of "notes.md" is changed to hold "New."
    When itos runs "draft promote"
    Then itos exits with code 0
    And the file "notes.md" says "New."
    And the last commit's header is "docs: say new"
    And git status shows nothing to commit

  @ID-DRAFT-15 @slice-99
  Scenario: draft edit of a file HEAD lacks starts it empty, and promote adds it
    When itos runs the command line "draft edit new-spec -m 'docs: add a spec' specs/new.md"
    Then itos exits with code 0
    And the file "specs/new.md" does not exist
    Given the draft "new-spec"'s copy of "specs/new.md" is changed to hold "A spec."
    When itos runs "draft promote"
    Then itos exits with code 0
    And the file "specs/new.md" says "A spec."
    And the last commit's header is "docs: add a spec"

  @ID-DRAFT-16 @slice-99
  Scenario: draft promote stops at an edited copy whose file HEAD changed since it was taken, keeping it
    Given the committed file "notes.md" holding "Old."
    And itos has run the command line "draft edit say-new -m 'docs: say new' notes.md"
    And the draft "say-new"'s copy of "notes.md" is changed to hold "New."
    And the committed file "notes.md" holding "Rewritten by someone."
    When itos runs "draft promote"
    Then itos exits with code 1
    And its output says "say-new"
    And the file "notes.md" says "Rewritten by someone."

  # Slice 103 (the user's calls, 2026-10-08, q-26 and q-27): itos mints
  # the ids it writes (slice 102), and a drafted command that creates an
  # item would otherwise get its id only at promote, while the coordinator
  # needs it now, for the spec, the brief and the drafts after it. So draft
  # add of a command that mints (work add --kind slice|task, task add, work
  # promote) mints its id at once, through the id counter
  # (features/ids.feature), prints it, and keeps it in the draft: promote
  # runs the command with that id. The id is real from the start: a later
  # draft, a drafted spec's tags and a brief simply write it, so no
  # placeholder is needed anywhere. A dropped draft gives its id back to no
  # one: it leaves a gap, as the counter never hands a number out twice.
  @ID-DRAFT-17 @slice-103
  Scenario: draft add of a command that mints prints the id at once, and promote creates the item under it
    When itos runs the command line "draft add thing -- work add --kind slice --title 'A thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-2"
    When itos runs "draft promote"
    Then itos exits with code 0
    And the registry's item "slice-2" is a slice titled "A thing" with the status "todo"

  @ID-DRAFT-18 @slice-103
  Scenario: A mint while a draft holds an id skips it
    Given itos has run the command line "draft add thing -- work add --kind slice --title 'A thing' --why 'Because.'"
    When itos runs the command line "work add --kind slice --title 'Another' --why 'Because.'"
    Then itos exits with code 0
    And its output says "slice-3"

  # Slice 139 (#78): creation checks decidable commit metadata before
  # taking changes out of the tree or creating private copies. The fixture
  # commits a policy requiring Scenarios and Upgrading on fix commits.
  # Message-taking When steps pass the doc string as one -m argument to
  # draft edit/add, not through shell interpolation.
  # Existence of future IDs, path/range rules and arbitrary external lint
  # execution remain outside creation-time metadata checking.
  @ID-DRAFT-19 @slice-139 @wip
  Scenario: draft edit rejects a missing required upgrading footer before creating copies
    Given a committed draft policy requiring scenario and upgrading footers for fixes
    And the draft store and repository state are saved for comparison
    When itos creates an edit draft named "missing-upgrade" for "features/a.feature" with this message:
      """
      fix: complete the snapshot specification

      Scenarios: ID-A-01
      """
    Then itos exits with code 1
    And its output says "Upgrading"
    And the draft "missing-upgrade" does not exist
    And the draft store and repository match the saved state

  @ID-DRAFT-20 @slice-139 @wip
  Scenario: invalid change-draft metadata leaves the caller's working changes and staging intact
    Given a committed draft policy requiring scenario and upgrading footers for fixes
    And the committed file "notes.md" holding "Old."
    And the file "notes.md" is changed to hold "New."
    And an unrelated change to "WORK.md" is staged
    And the draft store and repository state are saved for comparison
    When itos creates a change draft named "missing-upgrade" for "notes.md" with this message:
      """
      fix: clarify the acceptance notes

      Scenarios: ID-A-01
      """
    Then itos exits with code 1
    And its output says "Upgrading"
    And the file "notes.md" says "New."
    And the draft store and repository match the saved state

  @ID-DRAFT-21 @slice-139 @wip
  Scenario: draft creation diagnoses an absent required ID footer without resolving its IDs
    Given a committed draft policy requiring scenario and upgrading footers for fixes
    And the draft store and repository state are saved for comparison
    When itos creates an edit draft named "missing-link" for "features/a.feature" with this message:
      """
      fix: complete the snapshot specification

      Upgrading: none
      """
    Then itos exits with code 1
    And its output says "Scenarios"
    And the draft store and repository match the saved state

  # Guard: a valid message can name a scenario the draft will introduce.
  # The fixture's hooks, adapter and checks each record any execution; none
  # may run just to prepare a private draft. Expected to pass before the slice.
  @ID-DRAFT-22 @slice-139 @guard @wip
  Scenario: syntactically complete draft metadata can refer to future IDs without running external validators
    Given a committed draft policy requiring scenario and upgrading footers for fixes
    And the draft policy has recording hooks adapters and task checks
    And the draft store and repository state are saved for comparison
    When itos creates an edit draft named "future-scenario" for "features/a.feature" with this message:
      """
      fix: introduce the missing snapshot case

      Scenarios: ID-FUTURE-01
      Upgrading: none
      """
    Then itos exits with code 0
    And the draft "future-scenario" exists
    And no external draft validator ran
    And the working tree and index match the saved state

  # Guard: sources and in_place_of decide substitution, not the key's name.
  @ID-DRAFT-23 @slice-139 @guard @wip
  Scenario: metadata preflight accepts a custom registry footer that satisfies the required ledger footer
    Given a committed draft policy whose registry footer "Work" substitutes for "Task" on test commits
    When itos creates an edit draft named "test-steps" for "notes.md" with this message:
      """
      test: scaffold the snapshot assertions

      Work: future-slice
      """
    Then itos exits with code 0
    And the draft "test-steps" exists

  # Guard: early success cannot waive the policy in effect at promotion.
  # The complete draft fixture has valid changed contents and a valid
  # message under its original policy. The new policy is committed, so a
  # dirty-tree refusal cannot masquerade as the intended metadata failure.
  @ID-DRAFT-24 @slice-139 @guard @wip
  Scenario: promotion revalidates a message after the committed footer policy changes
    Given a complete edit draft named "snapshot-spec" whose message passes the committed draft policy
    And the draft policy now requires a "Migration" footer and that policy change is committed
    And the repository state is saved for comparison
    When itos runs "draft promote"
    Then itos exits with code 1
    And its output says "Migration"
    And the draft "snapshot-spec" exists
    And the repository files index and HEAD match the saved state

  # Slice 140 (#46, q-69): recognized minting commands share pure preflight
  # before direct reservation and before draft creation. The fixture commits
  # #105 (8.5.0) reports the same for work add with an undeclared tag;
  # p1-work-add-refusal-keeps-id (2026-10-17) is the same report again.
  # a valid policy, ledger and registry with p1-candidate as an idea; the
  # saved state includes the draft store, remote ID counter, files, index and
  # HEAD. Refusal means nonzero exit and the named diagnostic, not any error.
  # No fixture check may reserve an ID merely to inspect the counter.
  @ID-DRAFT-25 @slice-140 @wip
  Scenario Outline: malformed recognized minting commands are refused before reserving or storing anything
    Given a committed minting preflight fixture
    And the minting state is saved for comparison
    When itos attempts the minting command "<command>" in "<mode>" mode
    Then itos refuses the minting command naming "<diagnostic>"
    And the minting state matches the saved state

    Examples:
      | mode   | command                                                    | diagnostic |
      | direct | work add --kind slice --title 'Missing reason'              | why        |
      | draft  | work add --kind slice --title 'Missing reason'              | why        |
      | direct | task add --phase 1 --type build --title 'Task' --why 'Why.'  | check      |
      | draft  | task add --phase 1 --type build --title 'Task' --why 'Why.'  | check      |
      | direct | work promote p1-candidate --kind unsupported                | kind       |
      | draft  | work promote p1-candidate --kind unsupported                | kind       |

  # Each defect is the sole invalid input; every row is exercised both as
  # the direct command and as draft add candidate -- <command>. The command
  # fixture supplies all required arguments and never runs task checks.
  @ID-DRAFT-26 @slice-140 @wip
  Scenario Outline: decidable semantic failures do not consume IDs in direct or drafted commands
    Given a committed minting preflight fixture
    And its "<command>" input has only the defect "<defect>"
    And the minting state is saved for comparison
    When itos attempts that input directly and as a command draft in independent copies
    Then both attempts refuse with a diagnostic for "<defect>"
    And both copies retain the saved minting state

    Examples:
      | command      | defect                            |
      | work add     | undeclared tag                    |
      | work add     | nonexistent dependency            |
      | work add     | missing phase in a multiphase registry |
      | task add     | undeclared task type              |
      | task add     | nonexistent group                 |
      | work promote | missing source idea               |
      | work promote | malformed questions file          |
      | work promote | unresolved question reference     |

  # An earlier reserved item is a known future dependency; preflight must
  # not require its registry entry to exist yet. No simulated item is written.
  @ID-DRAFT-27 @slice-140 @wip
  Scenario: a later command draft can depend on the ID reserved by an earlier draft
    Given a committed minting preflight fixture
    And a valid earlier work-add draft named "first" has reserved an ID
    And the minting state is saved for comparison
    When itos drafts another valid work-add command depending on that reserved ID
    Then itos exits with code 0
    And the new draft has its own reserved ID
    And its output explains that dependency creation awaits promotion
    And no queued command or task check ran
    And the working tree index and HEAD match the saved minting state

    When itos runs "draft promote"
    Then itos exits with code 0
    And both items exist with the later item depending on the earlier item
    And neither promotion reserves another ID

  # The earlier edit is confined to private copies of the policy and adds
  # the tag future. Preflight recognizes that the tag check is affected,
  # without applying the edit, interpreting a projected queue or promising
  # success. Pending diagnostics identify the check and preceding draft.
  @ID-DRAFT-28 @slice-140 @wip
  Scenario: a tag declared by an earlier policy draft remains pending until promotion
    Given a committed minting preflight fixture without the tag "future"
    And a complete earlier edit draft named "declare-tag" adds the allowed tag "future"
    And the minting state is saved for comparison
    When itos runs the command line "draft add tagged -- work add --kind slice --phase 1 --title 'Tagged' --why 'Why.' --tags future"
    Then itos exits with code 0
    And the draft "tagged" has a reserved ID
    And its output names "declare-tag" and the pending tag validation
    And the committed policy still does not declare the tag "future"
    And no queued command or task check ran
    And the working tree index and HEAD match the saved minting state
    When itos runs "draft promote"
    Then itos exits with code 0
    And the reserved item exists with the tag "future"

  # An unrelated notes edit cannot excuse a known invalid tag. Conversely,
  # an opaque command's effects cannot be proven by running it: mutable
  # checks it might affect remain pending. This is intentionally bounded.
  @ID-DRAFT-29 @slice-140 @wip
  Scenario: unrelated drafts do not postpone a definitively invalid tag check
    Given a committed minting preflight fixture without the tag "future"
    And a complete earlier edit draft changes only "notes.md"
    And the minting state is saved for comparison
    When itos runs the command line "draft add tagged -- work add --kind slice --phase 1 --title 'Tagged' --why 'Why.' --tags future"
    Then itos refuses the minting command naming "future"
    And the minting state matches the saved state

  @ID-DRAFT-30 @slice-140 @wip
  Scenario: uncertain earlier command effects never suppress argument validation or execute the queue
    Given a committed minting preflight fixture
    And an earlier opaque command draft would record its execution
    And the minting state is saved for comparison
    When itos runs the command line "draft add invalid -- work add --kind slice --title 'Missing reason'"
    Then itos refuses the minting command naming "why"
    And no queued command or task check ran
    And the minting state matches the saved state

  # Final validation uses the actual state at promotion. The policy change
  # is committed after creation so a dirty-tree gate cannot cause this red.
  @ID-DRAFT-31 @slice-140 @wip
  Scenario: promotion rejects a reserved command whose tag became invalid and retains its reservation
    Given a committed minting preflight fixture declaring the tag "temporary"
    And a valid command draft named "tagged" reserves an item tagged "temporary"
    And a subsequent committed policy removes the tag "temporary"
    And the minting state is saved for comparison
    When itos runs "draft promote"
    Then itos refuses the minting command naming "temporary"
    And the draft "tagged" still has the same reserved ID
    And the minting state matches the saved state

  # Guard: pure preflight cannot make a remote counter and a local commit
  # transactional. A valid command still reserves before its hook runs;
  # failing that hook may leave the reservation consumed under ADR-0045.
  @ID-DRAFT-32 @slice-140 @guard @wip
  Scenario: a later hook failure rolls back files without recycling an already reserved ID
    Given a committed minting preflight fixture
    And a recording commit hook rejects the next otherwise valid work-add command
    And the minting state is saved for comparison
    When itos runs the command line "work add --kind slice --phase 1 --title 'Valid' --why 'Why.'"
    Then itos refuses the minting command naming the rejecting hook
    And the rejecting hook ran after one ID was reserved
    And the working tree index and HEAD match the saved minting state
    And the reserved ID remains consumed

  # Slice 141 (#46): draft add <id> --replace -- <itos args> replaces a
  # command draft in place, under the draft-store lock. Validate before
  # saving; preserve ID, reservation and queue position. The effective
  # repository and config path must identify the original reservation
  # domain. Same-kind recognized commands may share the retained ID;
  # changing between minting and non-minting is refused. A non-minting
  # command can replace another non-minting command in the same domain.
  # Replacement never executes a command, runs checks or reserves an ID.
  @ID-DRAFT-33 @slice-141 @wip
  Scenario: correcting a drafted task retains its ID and position and promotes the corrected checks
    Given a committed minting preflight fixture
    And valid command drafts named "before", "task" and "after" in that order
    And "task" contains a valid task-add command titled "Old" with one check
    And the minting state is saved for comparison
    When itos runs the command line "draft add task --replace -- task add --phase 1 --type build --title 'Corrected' --why 'Why.' --check 'check-one' --timeout 4 --check 'check-two'"
    Then itos exits with code 0
    And the draft "task" retains its ID reserved task ID and queue position
    And its stored command has the corrected title and ordered checks with their timeout
    And draft JSON lists the retained minted ID and corrected command
    And no additional ID was reserved
    And no queued command or task check ran
    And the working tree index and HEAD match the saved minting state
    When itos runs "draft promote"
    Then itos exits with code 0
    And the task with the retained ID has title "Corrected" and the corrected ordered checks
    And those drafts were applied in the order "before,task,after"
    And no additional ID was reserved

  # Invalid replacements are tested against the complete original store,
  # including command bytes and reservation, not merely draft existence.
  @ID-DRAFT-34 @slice-141 @wip
  Scenario Outline: invalid replacement leaves the original command draft intact
    Given a committed minting preflight fixture
    And a valid work-add command draft named "item" reserves a slice ID
    And the minting state is saved for comparison
    When itos runs the command line "draft add item --replace -- <command>"
    Then itos refuses the minting command naming "<diagnostic>"
    And the minting state matches the saved state

    Examples:
      | command                                                                                       | diagnostic |
      | work add --kind slice --title 'Incomplete'                                                     | why        |
      | work add --kind slice --phase 1 --title 'Tagged' --why 'Why.' --tags undeclared                   | undeclared |
      | work add --kind slice --phase 1 --title 'Dependent' --why 'Why.' --depends-on missing-item        | missing-item |

  # Domain fixtures use two valid, independently configured repositories
  # and a second valid config in the original repository. Refusal must
  # leave both domains' counters and files unchanged.
  @ID-DRAFT-35 @slice-141 @wip
  Scenario Outline: replacement cannot transfer a reservation to another kind or domain
    Given a committed minting preflight fixture
    And a valid work-add command draft named "item" reserves a slice ID
    And a valid replacement that would "<change>"
    And the minting state in both domains is saved for comparison
    When itos attempts that replacement with "draft add item --replace"
    Then itos refuses replacement naming "<diagnostic>"
    And the minting state in both domains matches the saved state

    Examples:
      | change                               | diagnostic |
      | mint a task instead of a slice       | kind       |
      | mint a bug instead of a slice        | kind       |
      | mint no item                         | kind       |
      | use a different repository           | repository |
      | use another config in the same repository | config |

  @ID-DRAFT-36 @slice-141 @wip
  Scenario Outline: replace requires an existing command draft and does not convert file drafts
    Given a committed minting preflight fixture
    And the target "target" is "<target>"
    And the minting state is saved for comparison
    When itos runs the command line "draft add target --replace -- work add --kind slice --phase 1 --title 'New' --why 'Why.'"
    Then itos refuses replacement naming "<diagnostic>"
    And the minting state matches the saved state

    Examples:
      | target                     | diagnostic |
      | absent                     | target     |
      | an edit draft with private copies | command |
      | a captured change draft    | command    |

  # The replacement's shared preflight sees only preceding drafts, not
  # commands after its preserved position. Later dependents keep their ID.
  @ID-DRAFT-37 @slice-141 @wip
  Scenario: replacing a reserved item keeps later drafted dependencies valid
    Given a committed minting preflight fixture
    And a valid earlier work-add draft named "first" has reserved an ID
    And a valid later command draft depends on that reserved ID
    And the minting state is saved for comparison
    When itos runs the command line "draft add first --replace -- work add --kind slice --phase 1 --title 'Corrected first' --why 'Why.'"
    Then itos exits with code 0
    And the draft "first" retains its ID reserved item ID and queue position
    And the later draft remains unchanged
    And no additional ID was reserved
    When itos runs "draft promote"
    Then itos exits with code 0
    And the reserved first item is titled "Corrected first"
    And the later item depends on the retained first item's ID
    And no additional ID was reserved

  # Non-minting command replacement does not acquire a reservation.
  # Without --replace the existing duplicate-name refusal still applies.
  @ID-DRAFT-38 @slice-141 @wip
  Scenario: a non-minting command can be corrected without changing duplicate-add behavior
    Given a committed minting preflight fixture
    And a command draft named "queue" stores "work queue slice-1"
    And the minting state is saved for comparison
    When itos runs the command line "draft add queue -- work queue p1-candidate"
    Then itos refuses replacement naming "queue"
    And the minting state matches the saved state
    When itos runs the command line "draft add queue --replace -- work queue p1-candidate"
    Then itos exits with code 0
    And the draft "queue" stores "work queue p1-candidate" in its original position
    And the draft "queue" has no minted ID
    And no additional ID was reserved
    And no queued command or task check ran
    And the working tree index and HEAD match the saved minting state

  # Slice 147 (#74, q-72):
  # draft edit <id> --task <task-id> --fields <list> -m <message>
  # creates a task-field draft; list is a nonempty, duplicate-free subset
  # of title,why,done_when (the ledger's acceptance-check field). Paths may
  # not accompany --task. The task must exist; no ID is minted. Creation
  # takes project data from HEAD, ignoring an agent's staged/unstaged edits;
  # stealth data has no committed copy and is snapshotted where configured.
  # It prints a private task.yaml path. JSON retains ok/draft/copies with
  # draft.kind=task, task=<id>, fields=<selected keys>, no minted property.
  # The fragment contains exactly selected keys, using null for absence;
  # null requests field removal subject to normal ledger validation. It may
  # not edit id/type/group/status/owner or add unselected keys.
  #
  # Promotion compares semantic field values, including absence, against
  # the saved base: unchanged private fields write nothing; current=base
  # accepts the edit; current=desired is already applied; anything else is
  # a conflict. done_when is one ordered field, not a per-check merge. A
  # changed title updates an existing registry title atomically, comparing
  # its separately captured base too. Other index fields and missing index
  # entries are never manufactured. Preserve comments outside changed
  # fields and task order. Changed check definitions cannot be applied to
  # a now-done task or recreate checks its closure removed. Independent
  # prose edits may merge after closure without reopening it.
  #
  # Re-resolve identity/domain; deletion, duplicate IDs or changed ledger
  # layout require recreating the draft, never guessing another target.
  # Validate the complete proposed ledger and current commit policy before
  # writing; final normal hooks still run. Refusal keeps the private draft
  # and remaining queue. A semantic no-op removes this draft without a
  # commit. Generic file drafts and selection order are unchanged. Fixtures
  # below install real hooks, commit valid setup and expose private copies
  # only through CLI results; snapshots include the complete store/counter.
  @ID-DRAFT-39 @slice-147 @wip
  Scenario: task draft creation copies only selected committed fields without taking an agent's edits
    Given a committed task-field draft fixture for "T-001"
    And the task's staged and unstaged ledger contents differ from HEAD
    And the minting state is saved for comparison
    When itos runs the command line "draft edit task-spec --task T-001 --fields why,done_when -m 'docs: clarify task acceptance' --json"
    Then itos exits with code 0
    And its JSON's "draft.kind" is "task"
    And its JSON's "draft.task" is "T-001"
    And its JSON's "draft.fields" is ["why","done_when"]
    And its JSON's "draft" has no property "minted"
    And its JSON's "copies" names one private task fragment
    And that fragment contains exactly the selected fields as HEAD holds them
    And no queued command task check or project hook ran
    And no additional ID was reserved
    And the working tree index and HEAD match the saved minting state

  @ID-DRAFT-40 @slice-147 @wip
  Scenario: promoting edited task fields preserves unrelated closure registry metadata and comments
    Given a committed task-field draft fixture with tasks "T-001,T-002"
    And the task draft "task-spec" selects "title,why" for "T-001"
    And its private fragment changes title and why
    And a subsequent commit closes T-002 removes T-002's checks and changes T-001's owner and queue position
    And that commit also changes T-001's unselected check definitions
    And the repository state is saved for comparison
    When itos runs "draft promote"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And T-001's ledger and existing registry titles equal the edited title
    And only T-001's selected ledger fields changed after the saved commit
    And T-002 stays done without checks
    And the saved owner queue position unselected checks comments and task order are preserved
    And the draft "task-spec" does not exist

  @ID-DRAFT-41 @slice-147 @wip
  Scenario Outline: conflicting edits to the same selected field keep the draft and apply nothing
    Given a committed task-field draft fixture for "T-001"
    And a task draft changes "<field>" from its base to a new value
    And a subsequent commit changes "<field>" to a different value
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote"
    Then itos refuses the task draft naming "T-001" and "<field>"
    And the draft store and repository match the saved state

    Examples:
      | field          |
      | title          |
      | why            |
      | done_when      |
      | registry title |

  # Selected fields that the private copy did not actually change are
  # ignored. An old unedited check list cannot resurrect removed checks.
  @ID-DRAFT-42 @slice-147 @wip
  Scenario: a prose-only task draft survives closure without restoring checks from its selected snapshot
    Given a committed task-field draft fixture for "T-001"
    And the task draft "task-spec" selects "why,done_when" for "T-001"
    And its private fragment changes only why
    And a subsequent commit closes T-001 and removes its checks
    When itos runs "draft promote"
    Then itos exits with code 0
    And T-001 has the edited ledger why
    And T-001 remains done with no done_when field
    And the draft "task-spec" does not exist

  @ID-DRAFT-43 @slice-147 @wip
  Scenario: edited checks cannot reopen a task that closed while its draft waited
    Given a committed task-field draft fixture for "T-001"
    And the task draft "task-spec" changes T-001's done_when
    And a subsequent commit closes T-001 and removes its checks
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote"
    Then itos refuses the task draft naming "T-001" and "done_when"
    And the draft store and repository match the saved state

  @ID-DRAFT-44 @slice-147 @wip
  Scenario: a rejected task-title promotion restores both ledger and index data and retains the draft
    Given a committed task-field draft fixture for "T-001"
    And the task draft "task-spec" changes T-001's title
    And a recording commit hook rejects the task-field promotion
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote"
    Then itos refuses the task draft naming the rejecting hook
    And the rejecting task-field hook ran exactly once
    And the draft store and repository match the saved state

  @ID-DRAFT-45 @slice-147 @wip
  Scenario Outline: invalid task-field selection creates no private copy or reservation
    Given a committed task-field draft fixture for "T-001"
    And the minting state is saved for comparison
    When itos runs the command line "draft edit invalid -m 'docs: clarify task acceptance' <arguments>"
    Then itos refuses the task draft naming "<diagnostic>"
    And the minting state matches the saved state

    Examples:
      | arguments                                      | diagnostic |
      | --task T-001                                   | fields     |
      | --task T-001 --fields ''                       | fields     |
      | --task T-001 --fields why,why                  | why        |
      | --task T-001 --fields status                   | status     |
      | --task T-001 --fields id                       | id         |
      | --task T-001 --fields why notes.md             | paths      |
      | --task T-999 --fields why                      | T-999      |
      | --fields why notes.md                         | task       |

  @ID-DRAFT-46 @slice-147 @wip
  Scenario: an already-applied semantic field edit retires its draft without another commit
    Given a committed task-field draft fixture for "T-001"
    And the task draft "task-spec" changes T-001's why
    And a subsequent commit independently applies exactly that desired why with different YAML formatting
    And the repository files index and HEAD are saved for comparison
    When itos runs "draft promote"
    Then itos exits with code 0
    And the draft "task-spec" does not exist
    And the repository files index and HEAD match the saved state

  @ID-DRAFT-47 @slice-147 @wip
  Scenario: task-field promotion follows the configured ledger layout and preserves written check definitions
    Given a committed task-field draft fixture for "T-001" with relocated ledger files
    And the task draft "task-spec" selects "done_when" for "T-001"
    And its private done_when is edited to:
      """
      - run: check-one
        timeout: 4
        prose: true
        cost: static
      - fails: check-two
        after: push
        timeout: 9
      """
    When itos runs "draft promote"
    Then itos exits with code 0
    And only the configured task entry receives exactly those ordered check definitions
    And no default-layout ledger was created

  @ID-DRAFT-48 @slice-147 @wip
  Scenario: a stealth task-field draft merges configured private data without committing it
    Given a committed task-field draft fixture for "T-001" using relocated stealth data
    And the task draft "task-spec" changes T-001's why
    And the repository files index and HEAD are saved for comparison
    When itos runs "draft promote"
    Then itos exits with code 0
    And the configured stealth task has the edited why
    And the draft "task-spec" does not exist
    And the working tree index and HEAD match the saved state

  @ID-DRAFT-49 @slice-147 @wip
  Scenario Outline: invalid fragments or changed targets refuse promotion without losing the private draft
    Given a committed task-field draft fixture for "T-001"
    And the task draft "task-spec" selects "why,done_when" for "T-001"
    And its pending task-field edit has "<problem>"
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote"
    Then itos refuses the task draft naming "<diagnostic>"
    And the draft store and repository match the saved state

    Examples:
      | problem                                  | diagnostic |
      | malformed private YAML                    | YAML       |
      | an unselected title key in its fragment   | title      |
      | a missing selected key in its fragment    | field      |
      | a check containing both run and fails     | done_when  |
      | duplicate task identities in the ledger   | T-001      |
      | the target task deleted by a later commit | T-001      |
      | changed effective ledger layout           | layout     |

  @ID-DRAFT-50 @slice-147 @wip
  Scenario: task-field draft creation shares pure commit-message preflight before creating copies
    Given a committed task-field draft fixture for "T-001"
    And its policy requires Upgrading on fixes
    And the minting state is saved for comparison
    When itos runs the command line "draft edit invalid --task T-001 --fields why -m 'fix: clarify task acceptance'"
    Then itos refuses the task draft naming "Upgrading"
    And no queued command task check or project hook ran
    And the minting state matches the saved state

  # Slice 148 (#76, q-73): draft promote [<draft-id>...] selects explicitly;
  # no arguments retains ordinary ordered promotion. Reject unknown or
  # duplicate IDs before any execution. Selected drafts run in queue order,
  # never argument order, with unselected entries/copies unchanged.
  #
  # Creation (add/edit, including task mode) accepts --depends-on <ids>
  # before --; IDs name existing earlier drafts, not work items. The list
  # is duplicate-free and stored in queue order. draft depends <id> --on
  # <ids> replaces that metadata alone; an empty value clears it. Preserve
  # copies, reservation and position. List JSON adds depends_on only when
  # nonempty. Command replacement preserves dependencies when --depends-on
  # is omitted; supplying it validates and replaces the dependency list
  # together with the command. The depends operation also edits dependencies
  # without changing command arguments or private copies. Neither mints.
  #
  # Explicit prerequisites must be selected or already successfully
  # promoted. Successful promotion retires satisfied edges from remaining
  # drafts; dropping a prerequisite with queued dependents refuses. Infer
  # edges from known reserved IDs and missing footer IDs supplied by earlier
  # known drafts too. Declarations cannot override inferred dependencies.
  #
  # Before any selected draft executes, check its skipped predecessors:
  # known file writes, typed task/field writes, referenced IDs and configured
  # validation inputs must establish independence. Support file drafts,
  # task-field drafts and parsed work-add effects initially. A known shared
  # field/path or relevant policy change blocks selection; disjoint typed
  # task fields can be independent within one ledger file. Changes to
  # checks used by the selected message's Task footer are relevant reads,
  # even when its edited prose field differs. Arbitrary command effects and
  # unsupported command footprints are unknown barriers, not empty scopes.
  # Opaque commands still run normally when no predecessor is skipped.
  # A different repository/config domain with unproven scope is unknown.
  # Never execute hooks, checks or commands merely to infer independence.
  # Recheck inputs before each application; no projected queue simulator or
  # promise of eventual promotability. Once execution starts, successful
  # commits remain on a later failure, as today. Keep JSON's existing ok,
  # promoted and problems fields and normal final validation/checkout gates.
  @ID-DRAFT-51 @slice-148 @wip
  Scenario: non-prefix selection promotes only independent drafts in stored queue order
    Given a committed selective-draft fixture with independent file drafts "first,second,third"
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote third first --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["first","third"]
    And only first and third were committed in that order
    And the remaining draft list is "second"
    And second's private files and metadata match the saved state

  @ID-DRAFT-52 @slice-148 @wip
  Scenario Outline: invalid selection refuses before even a valid selected predecessor runs
    Given a committed selective-draft fixture with independent file drafts "first,second"
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote <selection> --json"
    Then itos refuses draft selection naming "<diagnostic>"
    And its JSON's "promoted" is []
    And no queued command task check or project hook ran
    And the draft store and repository match the saved state

    Examples:
      | selection           | diagnostic |
      | first missing-draft | missing-draft |
      | first first         | first      |

  @ID-DRAFT-53 @slice-148 @wip
  Scenario: supported dependency edits preserve private copies reserved IDs and queue position
    Given a committed selective-draft fixture with an earlier file draft named "first"
    When itos runs the command line "draft add later --depends-on first -- work add --kind slice --phase 1 --title 'Later' --why 'Why.'"
    Then itos exits with code 0
    And draft JSON lists later's dependency on first
    Given the minting state is saved for comparison
    When itos runs the command line "draft depends later --on ''"
    Then itos exits with code 0
    And later's dependencies are empty
    And its command reserved ID private files and queue position are unchanged
    And no additional ID was reserved
    When itos runs "draft depends later --on first"
    Then itos exits with code 0
    And draft JSON lists later's dependency on first
    And no additional ID was reserved

  @ID-DRAFT-54 @slice-148 @wip
  Scenario Outline: dependency declarations reject unknown self duplicate and forward references before mutation
    Given a committed selective-draft fixture with independent file drafts "first,later"
    And the minting state is saved for comparison
    When itos runs the command line "<command>"
    Then itos refuses draft dependencies naming "<diagnostic>"
    And the minting state matches the saved state

    Examples:
      | command                                                                                                      | diagnostic |
      | draft depends first --on first                                                                               | first      |
      | draft depends first --on later                                                                               | later      |
      | draft depends later --on missing-draft                                                                       | missing-draft |
      | draft depends later --on first,first                                                                         | first      |
      | draft add new --depends-on missing-draft -- work add --kind slice --phase 1 --title 'New' --why 'Why.'          | missing-draft |

  @ID-DRAFT-55 @slice-148 @wip
  Scenario: explicit transitive prerequisites must be selected without automatically broadening the request
    Given a committed selective-draft fixture with independent file drafts "first,second,third"
    And second depends on first and third depends on second
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote third second --json"
    Then itos refuses draft selection naming "first"
    And its JSON's "promoted" is []
    And the draft store and repository match the saved state
    When itos runs "draft promote third first second --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["first","second","third"]

  @ID-DRAFT-56 @slice-148 @wip
  Scenario: prerequisites cannot be dropped while needed and successful promotion satisfies their edges
    Given a committed selective-draft fixture with independent file drafts "first,second,third"
    And third depends on first
    And the draft store and repository state are saved for comparison
    When itos runs "draft drop first"
    Then itos refuses draft dependencies naming "third"
    And the draft store and repository match the saved state
    When itos runs "draft promote first"
    Then itos exits with code 0
    And third no longer has an unsatisfied dependency on first
    When itos runs "draft promote third --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["third"]
    And the remaining draft list is "second"

  @ID-DRAFT-57 @slice-148 @wip
  Scenario: a reserved work-item dependency blocks selecting its dependent draft alone
    Given a committed selective-draft fixture with a work-add draft "first" holding a slice ID
    And a later work-add draft "later" depends on that reserved item ID without declaring a draft dependency
    And the minting state is saved for comparison
    When itos runs "draft promote later --json"
    Then itos refuses draft selection naming "first"
    And the minting state matches the saved state
    When itos runs "draft promote later first --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["first","later"]
    And the later item depends on the retained first item's ID
    And no additional ID was reserved

  @ID-DRAFT-58 @slice-148 @wip
  Scenario: a skipped policy draft cannot be treated as irrelevant to a later command's validation
    Given a committed selective-draft fixture with a private policy edit named "policy"
    And a later valid work-add draft named "idea"
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote idea --json"
    Then itos refuses draft selection naming "policy" and the configured policy path
    And no queued command task check or project hook ran
    And the draft store and repository match the saved state
    When itos runs "draft promote policy idea --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["policy","idea"]

  @ID-DRAFT-59 @slice-148 @wip
  Scenario: typed disjoint task fields can be selected independently within the same ledger file
    Given a committed selective-draft fixture for task "T-001"
    And a task-field draft "reason" changes only T-001's why
    And a later task-field draft "title" changes only T-001's title
    And neither message names task checks or missing footer IDs
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote title --json"
    Then itos exits with code 0
    And its JSON's "promoted" is ["title"]
    And T-001's ledger and registry have the edited title
    And its why still equals the saved committed why
    And reason's private files and metadata match the saved state

  # Footprint fixtures include a recording opaque command and recording
  # hooks/checks; none may be invoked during an independence refusal.
  @ID-DRAFT-60 @slice-148 @wip
  Scenario Outline: overlapping or unknown effects refuse unsafe skipping without probing by execution
    Given a committed selective-draft fixture whose skipped predecessor has "<effect>"
    And the draft store and repository state are saved for comparison
    When the later draft is selected alone as JSON
    Then itos refuses draft selection with rule "<rule>"
    And the refusal identifies the blocking predecessor and relevant scope
    And no queued command task check or project hook ran
    And the draft store and repository match the saved state

    Examples:
      | effect                                              | rule                    |
      | an ordinary file draft touching the same file        | draft-selection-overlap |
      | a task-field draft editing the same task field       | draft-selection-overlap |
      | changed checks named by the selected Task footer     | draft-selection-overlap |
      | an opaque earlier command                           | draft-selection-unknown |
      | unknown effects of the selected command across a skip | draft-selection-unknown |

  @ID-DRAFT-61 @slice-148 @wip
  Scenario: failure after selected execution starts keeps earlier success and all unselected work
    Given a committed selective-draft fixture with independent file drafts "first,second,third,fourth"
    And a recording commit hook rejects only third's valid commit
    And the draft store and repository state are saved for comparison
    When itos runs "draft promote fourth third first --json"
    Then itos exits with code 1
    And its JSON's "promoted" is ["first"]
    And exactly one successful commit was added by the command
    And third's failed file changes were restored
    And the remaining draft list is "second,third,fourth"
    And second's and fourth's private files and metadata match the saved state
    And fourth was not executed

  @ID-DRAFT-62 @slice-148 @wip
  Scenario: replacing a reserved command draft does not silently erase its prerequisites
    Given a committed selective-draft fixture with an earlier file draft named "first"
    And a later reserved work-add draft named "later" explicitly depends on first
    When itos runs the command line "draft add later --replace -- work add --kind slice --phase 1 --title 'Corrected' --why 'Why.'"
    Then itos exits with code 0
    And later still depends on first and retains its reserved ID
    Given the draft store and repository state are saved for comparison
    When itos runs "draft promote later --json"
    Then itos refuses draft selection naming "first"
    And the draft store and repository match the saved state

  # Bug 152, found by T-329's FuzzStrictYAML oracle: the drafts file, as the
  # follow-ups and questions files (ID-FOLLOW-16 in follow.feature, ID-ASK-43
  # in ask.feature), was read by decoding it with yaml.v3 into structs,
  # KnownFields on, and yaml.v3 skips a mapping entry whose key is null (~
  # or null) without decoding its value: a key written twice under it, which
  # value.Parse refuses (Map keys must be unique), was read, and the null
  # key itself was never reported as the unknown key it is ("drafts: []"
  # beside "~: 1" read as no drafts). yaml.v3 drops a null list item too,
  # from a list of drafts or of strings ("command: [~, status]" read as
  # "itos status"). The next command that changes the list saves what it
  # read whole, so the skipped entry is lost, and promote runs the misread
  # command.
  #
  # Each reader now reads its text through value.Parse first, as upgrade's
  # workflow reader does since K150b, and refuses what Parse refuses: a key
  # written twice in one mapping at any depth (under a null key too), a
  # syntax error, an alias inside its own anchor or one expanding past
  # Parse's bound (bug 39), a second document. The refusal is the one each
  # reader gives a file it cannot read today: an error naming the file,
  # "<path>: <Parse's error>" as yaml.v3's errors are given now (here
  # "<drafts.yaml's path>: Map keys must be unique at line 3, column 3"),
  # exit 2 (kind.Usage; followup's domain.Invalid, which the command line
  # gives kind.Usage), the file left as it is. What a reader refused before
  # keeps its own message: an empty file, a second document ("holds more
  # than one YAML document, and itos writes one ..."), an unknown key, the
  # schema's checks; Parse's ErrMultipleDocuments does not replace the
  # reader's words. Every command that reads these files inherits the
  # refusal: itos status, work show and work promote, and config check's
  # asks-invalid (exit 1) for the questions file.
  #
  # Beyond what Parse refuses, two things itos never writes are refused,
  # since yaml.v3 drops them and the next save loses them: a null key at any
  # depth, an unknown key like any other ("<path>: line 2, column 1 has a
  # null key, which no file itos writes has"); and a null list item at any
  # depth, in the list of drafts, threads, notes or questions or in a list
  # of strings ("<path>: line 3, column 15 has a null list item, which no
  # file itos writes has"). A null item is refused rather than kept as Parse
  # reads it: kept, it would be a draft, thread or question with nothing in
  # it, or an argument "" no one wrote, a value made up where the file holds
  # none. So the tolerance T-329's held targets gave a dropped null item
  # (their present()) goes: where Parse's value holds a null key (a key "")
  # or a null list item anywhere, the reader refuses. A null written as a
  # field's value (message: ~, notes: ~) is still read as the field's empty
  # value, as now: Parse's null and an omitted field are the same nothing,
  # and nothing is lost on save.
  #
  # Kept by design: a number or a boolean written where a string goes is
  # read as its text (id: 017 is "017", where Parse reads 17), which is what
  # was written and what Save writes back quoted, as the held targets say;
  # and a merge key (<<) stays yaml.v3's to merge, which they do not compare.
  #
  # The fix commit lands T-329's held targets, which that task moved here:
  # FuzzStrictYAMLDraft in internal/draft, FuzzStrictYAMLFollow in
  # internal/slices/followup/domain and FuzzStrictYAMLAsk in internal/ask,
  # each a strict_yaml_fuzz_test.go with its seeds under testdata/fuzz,
  # less present() and the null-key exclusion (nullKey), plus the failing
  # inputs as seeds in each target, with its own file's key: a key twice
  # under a null key ("~:\n  b: 1\n  b: 2\n"), a null key beside the list
  # ("drafts: []\n~: 1\n"), a null item in the list ("drafts:\n  -\n"), and
  # one in a list of strings (command: [~, a]; a thread's docs). No
  # scenario or corpus case of v8.25.2 writes a null key or a null list
  # item, and previous-release keeps every promise of v8.25.2 on this
  # spec's tree, so the fix needs no Changes: footer; previous-release says
  # so again with it.
  #
  # Before the fix it fails at "itos exits with code 2": draft.parse decodes
  # the text into File, skipping the ~ entry, so it reads no drafts; draft
  # add appends queue-t1 and Save writes the list whole, the ~ entry gone,
  # exit 0.
  @ID-DRAFT-63 @bug-152
  Scenario: A drafts file with a key written twice under a null key is refused, and left as it was
    Given the scratch file ".git/itos/drafts/drafts.yaml" contains exactly:
      """
      ~:
        b: 1
        b: 2
      """
    When itos runs the command line "draft add queue-t1 -- work add p1-thing --title 'A thing' --why 'Because.'"
    Then itos exits with code 2
    And its output says "drafts.yaml"
    And its output says "keys must be unique"
    And the file ".git/itos/drafts/drafts.yaml" says "b: 2"

  # Bug 152, as ID-DRAFT-63 says: a null list item, which yaml.v3 drops
  # from a list of strings, so the command draft [~, status] read as "itos
  # status", and draft promote would have run that. Refused, exit 2, naming
  # the file and the null list item. Before the fix it fails at "itos exits
  # with code 2": draft.parse reads the command [status] and itos draft
  # lists it, exit 0.
  @ID-DRAFT-64 @bug-152
  Scenario: A drafts file with a null list item is refused, not read without it
    Given the scratch file ".git/itos/drafts/drafts.yaml" contains exactly:
      """
      drafts:
        - id: say-status
          command: [~, status]
      """
    When itos runs "draft"
    Then itos exits with code 2
    And its output says "drafts.yaml"
    And its output says "null list item"
