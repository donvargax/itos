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
  # minted. Stale fixtures are committed before hooks are installed, rather
  # than constructing invalid links through decision add.

  Background:
    Given a repository whose ledger has the task "T-001"

  @wip @ID-ASK-24 @slice-156
  Scenario: An answered question can be linked to an explicitly selected existing item
    Given the answered question "q-1" names an old item absent from the registry
    And the registry has an existing done item owned by another person
    And an unrelated staged file and an untracked file exist
    When I relink "q-1" to that existing item with "decision relink" as JSON
    Then itos exits with code 0
    And JSON's question names the selected item and retains its question, answer and answered status
    And JSON's commit is the one new commit, which changes only the configured questions file
    And the registry, ledger and unrelated files and staging are unchanged
    And config check no longer reports that question's missing-item warning
    And no rename was inferred and no ID was reserved

  @wip @ID-ASK-25 @slice-156
  Scenario: A batch repairs selected questions while preserving their status and recorded decisions
    Given questions "q-1", "q-2" and "q-3" occur in that order in the questions file
    And "q-1" is open, "q-2" is answered with decision none, and "q-3" is answered with a recorded decision number
    And "slice-7" is an existing item
    When itos runs "decision relink q-3 q-1 --item slice-7 --json"
    Then itos exits with code 0
    And the questions remain in their original file order
    And only "q-3" and "q-1" now name "slice-7"
    And all question texts, answers and decision values are unchanged
    And JSON's questions are "q-3" then "q-1", with one shared commit
    And no architecture record or its index changed

  @wip @ID-ASK-26 @slice-156
  Scenario: Clearing links is explicit and repeated repairs are no-ops
    Given answered question "q-1" names a missing item and open question "q-2" has no item
    When itos runs "decision relink q-1 q-2 --clear-item --json"
    Then itos exits with code 0 and creates one questions-file commit
    And neither question has an item key in the file or JSON
    And their text, answer, status and decision values are unchanged
    When itos runs "decision relink q-1 q-2 --clear-item --json" again
    Then itos exits with code 0 and JSON's commit is null
    And the questions file, HEAD and index are byte-for-byte unchanged
    And no commit hook ran for that no-op
    When I relink a question to the existing item it already names
    Then that operation is also an unchanged success with no commit

  @wip @ID-ASK-27 @slice-156
  Scenario Outline: Invalid selections or modes never leave a partial batch
    Given valid questions "q-1" and "q-2" and existing item "slice-7"
    When itos runs "<command>"
    Then itos exits with code <code>
    And the questions file, all other files, HEAD and index are unchanged
    And no question or work ID was reserved

    Examples:
      | command                                                        | code |
      | decision relink q-1 q-999 --item slice-7                         | 1    |
      | decision relink q-1 q-2 --item nonexistent-item                  | 1    |
      | decision relink q-1 q-1 --item slice-7                           | 2    |
      | decision relink q-1                                             | 2    |
      | decision relink q-1 --item slice-7 --clear-item                  | 2    |
      | decision relink --clear-item                                   | 2    |
      | decision relink not-a-question --clear-item                     | 2    |

  @wip @ID-ASK-28 @slice-156
  Scenario: Missing-item diagnostics offer the supported repair without changing their severity
    Given an answered question "q-1" names a missing item and every other config input is sound
    When itos runs "config check"
    Then itos exits with code 0 and reports that association as a warning
    And the warning shows "decision relink q-1 --item" with a clearly marked existing-item placeholder
    And the warning shows the runnable command "itos decision relink q-1 --clear-item"
    When I obtain the same config check as JSON
    Then the existing "asks-answered-unknown-item" warning has the supported repair advice in its fix field
    When an open question "q-2" also names a missing item and I run config check
    Then itos exits with code 1 and retains the existing "asks-unknown-item" error
    And the error's text and JSON fix offer the corresponding relink and clear commands for "q-2"
    And neither diagnostic rewrites links itself

  @wip @ID-ASK-29 @slice-156
  Scenario: Repair does not overwrite dirty or malformed questions data
    Given "q-1" exists in the configured questions file
    And that file has uncommitted edits
    When itos runs "decision relink q-1 --clear-item"
    Then itos refuses using the existing dirty-data convention and preserves all edits
    When the fixture instead supplies malformed questions data with an unknown field
    And itos runs "decision relink q-1 --clear-item"
    Then itos exits with code 2 using the existing strict questions-file diagnostic
    And no field was dropped, no file was rewritten and no commit was created

  @wip @ID-ASK-30 @slice-156
  Scenario: A rejected relink commit restores the questions and preserves unrelated staging
    Given answered questions "q-1" and "q-2" name missing items
    And a commit hook rejects a relink commit
    And an unrelated file is staged
    When itos runs "decision relink q-1 q-2 --clear-item"
    Then itos reports the commit rejection
    And both associations and the questions file bytes are restored
    And HEAD and the unrelated staged file are unchanged
    And no partial repair or architecture-record edit remains

  @wip @ID-ASK-31 @slice-156
  Scenario: Stealth relinking uses private data and clearing requires no target registry lookup
    Given a stealth config with custom private questions and registry locations
    And private questions "q-1" and "q-2" and an existing private work item
    When I relink those questions to that work item as JSON
    Then itos exits with code 0 with the two updated questions and a null commit
    And only the private questions file changed
    And project files, architecture records, HEAD and index are unchanged
    When the target registry is unavailable and I clear those associations
    Then the clear operation needs no target lookup
    And it preserves the ordinary private-write validation and locking rules
