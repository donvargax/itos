@phase-1
Feature: Questions have a supported way to repair their work-item association
  # #49, q-74: explicit repair first. Automatic upgrade repair is separate
  # (p1-upgrade-repair-question-links), never inferred from "Was" prose.
  # decision relink <question-id>... takes exactly one of --item <id> or
  # --clear-item. Supplied targets use decision add's effective-registry
  # lookup, regardless of item kind, owner or lifecycle state. Clearing
  # needs no target lookup and removes the optional item key, not its text.
  # Only associations change: preserve question order, text, answers and
  # decision values. Open/answered remain the only question statuses;
  # recording an answer does not introduce another status.
  # One batch is all-or-nothing and makes one normal decision-data commit.
  # Single JSON uses question/commit; batch uses questions in request order
  # and the shared commit, null for unchanged or stealth operations.
  # Semantic no-ops do not rewrite the file or run commit hooks. No ID is
  # minted. A stale link is a committed fixture ("the committed file …",
  # past the hooks), since decision add refuses an item the registry lacks;
  # a hook a scenario measures is installed after the fixtures.
  # Until relink exists, Parse sends "decision relink …" to the list, which
  # refuses it as a usage error, exit 2, before reading anything
  # (internal/slices/decision/parse.go), so every exit-2 expectation below
  # also names its own diagnostic.

  Background:
    Given a repository whose ledger has the task "T-001"

  # slice-8's why says it was p1-gone, as a promotion's does; relink never
  # reads that prose (q-74), so q-1 names slice-9, the item the command line
  # names, and an implementation that resolved the rename would name slice-8
  # or refuse. slice-9 is done and someone else's: neither matters.
  @wip @ID-ASK-24 @slice-156
  Scenario: An answered question can be linked to an explicitly selected existing item
    Given the work registry has the item "slice-8" owned by "ana" with the status "todo" and the why "Was p1-gone."
    And the work registry has the item "slice-9" owned by "bea" with the status "done"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
      """
    And an unrelated change to "notes.md" is staged
    And an untracked file "scratch.txt"
    When itos runs "decision relink q-1 --item slice-9 --json"
    Then itos exits with code 0
    And its JSON's "question" is {"id": "q-1", "item": "slice-9", "question": "Which way?", "status": "answered", "answer": "This way."}
    And its JSON's commit is HEAD, one commit on top of the scenario's last
    And the last commit touches only "tasks/asks.yaml"
    And the file "tasks/asks.yaml" says "slice-9"
    And the file "tasks/asks.yaml" does not say "slice-8"
    And the file "tasks/asks.yaml" does not say "p1-gone"
    And the registry is unchanged
    And the file "tasks/phase-1.yaml" has the same contents as its committed version
    And "notes.md" is still staged
    And the file "scratch.txt" says "A note."
    And the file ".git/itos/ids.yaml" does not exist
    When itos checks the config
    Then itos exits with code 0
    And its output does not say "p1-gone"

  # q-2 keeps its own stale item, so "only q-3 and q-1" can fail. q-3's
  # record and the index are committed, and the record names q-3's old item,
  # so a relink that rewrote the record's context, or regenerated the index,
  # would change them.
  @wip @ID-ASK-25 @slice-156
  Scenario: A batch repairs selected questions while preserving their status and recorded decisions
    Given the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the committed file "docs/decisions/0001-use-labels.md" holding "Asked as q-3, about p1-gone."
    And the committed file "docs/decisions/README.md" holding "- [ADR-0001: Use labels](0001-use-labels.md)"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          question: "Labels?"
        - id: q-2
          item: "p1-gone"
          question: "Projects?"
          answer: "Not now."
          decision: none
        - id: q-3
          item: "p1-gone"
          question: "Labels or Projects?"
          answer: "Labels."
          decision: 1
      """
    When itos runs "decision relink q-3 q-1 --item slice-7 --json"
    Then itos exits with code 0
    And its JSON's "questions" is [{"id": "q-3", "item": "slice-7", "question": "Labels or Projects?", "status": "answered", "answer": "Labels.", "decision": 1}, {"id": "q-1", "item": "slice-7", "question": "Labels?", "status": "open"}]
    And its JSON's commit is HEAD, one commit on top of the scenario's last
    And the last commit touches only "tasks/asks.yaml"
    And the file "tasks/asks.yaml" has the line "  - id: q-2" after the line "  - id: q-1"
    And the file "tasks/asks.yaml" has the line "  - id: q-3" after the line "  - id: q-2"
    And the file "tasks/asks.yaml" says "p1-gone"
    And the file "tasks/asks.yaml" has the line "    decision: none" after the line "  - id: q-2"
    And the file "tasks/asks.yaml" has the line "    decision: 1" after the line "  - id: q-3"
    And the file "docs/decisions/0001-use-labels.md" has the same contents as its committed version
    And the file "docs/decisions/README.md" has the same contents as its committed version
    And git reports no change to the working tree or the index

  # The hook is installed after the fixture and records each run, so the
  # first repair shows it runs and the no-ops show it did not run again. A
  # no-op is found before any write is tried, so nothing in the repository,
  # the git folder included, changes.
  @wip @ID-ASK-26 @slice-156
  Scenario: Clearing links is explicit and repeated repairs are no-ops
    Given the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          question: "Why not?"
        - id: q-3
          item: "slice-7"
          question: "When?"
      """
    And the hooks folder holds a commit-msg hook that records it ran
    When itos runs "decision relink q-1 q-2 --clear-item --json"
    Then itos exits with code 0
    And its JSON's "questions" is [{"id": "q-1", "question": "Which way?", "status": "answered", "answer": "This way."}, {"id": "q-2", "question": "Why not?", "status": "open"}]
    And its JSON's commit is HEAD, one commit on top of the scenario's last
    And the last commit touches only "tasks/asks.yaml"
    And the file "tasks/asks.yaml" does not say "p1-gone"
    And the file "tasks/asks.yaml" says "slice-7"
    And the project's commit-msg hook ran once
    And the repository state is saved for comparison
    When itos runs "decision relink q-1 q-2 --clear-item --json"
    Then itos exits with code 0
    And its JSON's "commit" is null
    And the repository files index and HEAD match the saved state
    And the project's commit-msg hook ran once
    When itos runs "decision relink q-3 --item slice-7 --json"
    Then itos exits with code 0
    And its JSON's "question" is {"id": "q-3", "item": "slice-7", "question": "When?", "status": "open"}
    And its JSON's "commit" is null
    And the repository files index and HEAD match the saved state
    And the project's commit-msg hook ran once

  # The saved state is every file of the repository, the git folder's
  # included, so it holds the id counter (.git/itos/ids.yaml) too: nothing
  # is reserved. Every refusal comes before any write is tried, as decision
  # add's and answer's do. The first two rows are refusals, exit 1, in the
  # existing words of decision's ask-no-question and ask-unknown-item; the
  # rest are usage errors, each named by its own words, all new but the
  # last, which is the words every decision subcommand gives a bad id
  # (askID).
  @wip @ID-ASK-27 @slice-156
  Scenario Outline: Invalid selections or modes never leave a partial batch
    Given the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          question: "Why not?"
      """
    And the repository state is saved for comparison
    When itos runs "<command>"
    Then itos exits with code <code>
    And its output says "<says>"
    And the repository files index and HEAD match the saved state

    Examples:
      | command                                         | code | says                                                        |
      | decision relink q-1 q-999 --item slice-7        | 1    | there is no question q-999                                  |
      | decision relink q-1 q-2 --item nonexistent-item | 1    | no item nonexistent-item in tasks/work-items.yaml           |
      | decision relink q-1 q-1 --item slice-7          | 2    | decision relink names q-1 twice                             |
      | decision relink q-1                             | 2    | decision relink needs --item <id> or --clear-item           |
      | decision relink q-1 --item slice-7 --clear-item | 2    | decision relink takes --item <id> or --clear-item, not both |
      | decision relink --clear-item                    | 2    | decision relink needs <question-id>                         |
      | decision relink not-a-question --clear-item     | 2    | is no question id (q-<n>)                                   |

  # config check's text prints each warning's and error's message alone
  # (WARN/FAIL lines, internal/slices/config/run.go), never its fix, so the
  # advice is in the message, and the same advice in the JSON fix field.
  # <item-id> is the placeholder the person replaces.
  @wip @ID-ASK-28 @slice-156
  Scenario: Missing-item diagnostics offer the supported repair without changing their severity
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
      """
    When itos checks the config
    Then itos exits with code 0
    And its output says "WARN"
    And its output says "itos decision relink q-1 --item <item-id>"
    And its output says "itos decision relink q-1 --clear-item"
    When itos checks the config as JSON
    Then itos exits with code 0
    And its JSON's warning "asks-answered-unknown-item"'s "fix" says "itos decision relink q-1 --item <item-id>"
    And its JSON's warning "asks-answered-unknown-item"'s "fix" says "itos decision relink q-1 --clear-item"
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "p1-lost"
          question: "Why not?"
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "itos decision relink q-2 --item <item-id>"
    And its output says "itos decision relink q-2 --clear-item"
    When itos checks the config as JSON
    Then itos exits with code 1
    And its JSON's problem "asks-unknown-item"'s "fix" says "itos decision relink q-2 --item <item-id>"
    And its JSON's problem "asks-unknown-item"'s "fix" says "itos decision relink q-2 --clear-item"
    And the file "tasks/asks.yaml" has the same contents as its committed version

  # The change no commit holds is a comment line, which the questions file
  # parses with, so the refusal is the dirty-data rule's and not the strict
  # parser's; a write would drop the comment.
  @wip @ID-ASK-29 @slice-156
  Scenario: Repair does not overwrite dirty or malformed questions data
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
      """
    And the questions file has a change no commit holds
    When itos runs "decision relink q-1 --clear-item --json"
    Then itos exits with code 1
    And its JSON's problem "asks-file-uncommitted"'s "message" says "tasks/asks.yaml"
    And the file "tasks/asks.yaml" says "# a change no commit holds"
    And the file "tasks/asks.yaml" says "p1-gone"
    And no commit was made
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
          colour: "blue"
      """
    When itos runs "decision relink q-1 --clear-item"
    Then itos exits with code 2
    And its output says "tasks/asks.yaml"
    And its output says "colour"
    And the file "tasks/asks.yaml" has the same contents as its committed version
    And no commit was made

  @wip @ID-ASK-30 @slice-156
  Scenario: A rejected relink commit restores the questions and preserves unrelated staging
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "p1-lost"
          question: "Why not?"
          answer: "No time."
      """
    And a commit-msg hook that refuses every commit
    And an unrelated change to "notes.md" is staged
    When itos runs "decision relink q-1 q-2 --clear-item"
    Then itos exits with code 1
    And its output says "the hook refuses every commit"
    And its output says "tasks/asks.yaml is as it was"
    And the file "tasks/asks.yaml" has the same contents as its committed version
    And no commit was made
    And "notes.md" is still staged
    And the file "docs/decisions/README.md" does not exist

  # The private registry is missing for the clear (the file deleted), which
  # itemProblem would refuse as work-registry-missing had clearing looked the
  # target up. The stealth writers hold the registry's lock, the registry's
  # path with ".lock" (internal/lock); a lock given back is no file.
  @wip @ID-ASK-31 @slice-156
  Scenario: Stealth relinking uses private data and clearing requires no target registry lookup
    Given itos's config is kept in the git folder
    And the config sets "work.registry" to "private/items.yaml"
    And the config sets "work.asks" to "private/questions.yaml"
    And the file ".git/itos/private/items.yaml" holds the lines:
      """
      phases: { 1: null }
      items:
        - { id: slice-7, title: slice-7, phase: 1, owner: null, status: todo, depends_on: [] }
      """
    And the file ".git/itos/private/questions.yaml" holds the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "p1-gone"
          question: "Why not?"
      """
    When itos runs "decision relink q-1 q-2 --item slice-7 --json"
    Then itos exits with code 0
    And its JSON's "questions" is [{"id": "q-1", "item": "slice-7", "question": "Which way?", "status": "answered", "answer": "This way."}, {"id": "q-2", "item": "slice-7", "question": "Why not?", "status": "open"}]
    And its JSON's "commit" is null
    And the file ".git/itos/private/questions.yaml" says "slice-7"
    And the file ".git/itos/private/questions.yaml" does not say "p1-gone"
    And the file ".git/itos/decisions" does not exist
    And the file ".git/itos/private/items.yaml.lock" does not exist
    And no commit was made
    And git status shows nothing to commit
    When the private registry file ".git/itos/private/items.yaml" is deleted
    And itos runs "decision relink q-1 q-2 --clear-item --json"
    Then itos exits with code 0
    And its JSON's "questions" is [{"id": "q-1", "question": "Which way?", "status": "answered", "answer": "This way."}, {"id": "q-2", "question": "Why not?", "status": "open"}]
    And its JSON's "commit" is null
    And the file ".git/itos/private/questions.yaml" does not say "item:"
    And the file ".git/itos/private/items.yaml.lock" does not exist
    And no commit was made
    And git status shows nothing to commit
