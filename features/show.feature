@phase-1
Feature: itos work show, one item's spec and commits, the unit an agent reads and a reviewer reviews
  Which commits belong to a work item is itos's knowledge, not git's: a
  commit's footers name the item's scenarios (Scenarios:) or the item itself
  (Task:), and itos's own registry commits name it in their headers (docs:
  take <id>, docs: promote <idea> to <id>, docs: close <id>). A reviewer
  finding them with grep would get it differently each time, and wrong under
  a stealth config, whose footers are in notes. itos work show <id> prints
  the item (its kind, status, owner, title, why, depends_on and the items
  depending on it), its scenarios (those tagged @<id>, live or @wip), and its
  commits, oldest first, each with its short SHA and header; a later commit
  naming one of its scenarios, a fix say, is listed with them. --patch adds
  each commit's diff, so it is the whole of a review's input; --json gives
  the same as data. It reads, never writes (the user's calls, 2026-10-03: the
  agent's reading list in place of a filled brief, and the review unit of a
  read-only reviewer running beside the next implementer).

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-SHOW-01 @slice-57
  Scenario: work show lists a slice's commits by the scenarios their footers name, oldest first
    Given the work registry has the item "slice-9" owned by nobody with the status "doing"
    And the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-9"
    And the commit "feat: add the a" naming the scenarios "@ID-A-01" on top of it
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    And the commit "fix: mend the a" naming the scenarios "@ID-A-01" on top of it
    When itos runs "work show slice-9"
    Then itos exits with code 0
    And its output says "feat: add the a" before "fix: mend the a"
    And its output does not say "chore: tidy the readme"
    And its output says "@ID-A-01"

  # itos's own registry commits are read by their headers: docs: take, docs:
  # close, docs: add and docs: edit <id>, and docs: promote <idea> to <id>.
  @ID-SHOW-02 @slice-57
  Scenario: work show lists a task's commits by its Task footer, and its take by its header
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "docs: take T-001" touching only "tasks/work-items.yaml" on top of it
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    And the commit "docs: describe the build" on top of it
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "docs: take T-001" before "chore: tidy the readme"
    And its output does not say "docs: describe the build"

  @ID-SHOW-03 @slice-57
  Scenario: work show --patch adds each commit's diff
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs "work show T-001 --patch"
    Then itos exits with code 0
    And its output says "diff --git"
    And its output says "README.md"

  @ID-SHOW-04 @slice-57
  Scenario: work show --json gives the item and its commits as data
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "commits" has one entry whose "header" is "chore: tidy the readme"

  @ID-SHOW-05 @slice-57
  Scenario: work show refuses an item the registry does not have
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    When itos runs "work show slice-7"
    Then itos exits with code 1
    And its output says "slice-7"

  # Under a stealth config a commit's footers of IDs are in its note in
  # refs/notes/itos, never its message, which grep over the log would miss.
  @ID-SHOW-06 @slice-57
  Scenario: Under a stealth config work show finds a task's commits by their notes
    Given itos's config is kept in the git folder
    And the work registry beside the config has the item "T-001" owned by "someone" with the status "doing"
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the readme'"
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "chore: tidy the readme"

  # Slice 63: a commit whose Item footer names the item belongs to it, so a
  # spec written in a docs commit is part of what work show lists.
  @ID-SHOW-07 @slice-63
  Scenario: work show lists a commit by its Item footer
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    And the commit "docs: specify slice 9" naming the item "slice-9" in an Item footer on top of it
    When itos runs "work show slice-9"
    Then itos exits with code 0
    And its output says "docs: specify slice 9"

  # bug-107 (donvargax/itos#52, itos 8.0.0): task add wrote its why into the
  # registry item as well as the ledger entry, so work show printed the same
  # text twice, the ledger's and then the registry's, for every task task add
  # made. The registries written so far keep that copy, so work show prints a
  # registry why the same as the ledger's once; a registry why that says
  # something else (an idea's reasons kept through work promote) still prints
  # after the ledger's. task add stops writing the copy (@ID-TASK-16).
  @ID-SHOW-08 @bug-107
  Scenario: work show prints a task's why once when the registry holds the same why as the ledger
    Given the ledger's task "T-001" has the why "The ledger says why."
    And the work registry has the item "T-001" owned by "someone" with the status "todo" and the why "The ledger says why."
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "The ledger says why." once, before "Scenarios: none tagged @T-001"

  # Slice 136 (#65 and #75): inspection exposes definitions, never a verdict.
  # The checks setup below replaces only this task's done_when with the YAML
  # list in the doc string, preserving the background's ledger and config.
  # JSON retains the written ledger keys and order; omitted defaults stay
  # omitted. Historical checks removed at close belong to another item.
  @ID-SHOW-09 @slice-136 @wip
  Scenario: work show prints current check commands and their execution qualifiers in written order
    Given the work registry has the item "T-001" owned by nobody with the status "doing"
    And the ledger's task "T-001" has current checks:
      """
      - run: echo run > inspect-run-marker
        timeout: 9
        prose: true
        cost: static
      - fails: echo fail > inspect-fail-marker
        timeout: 11
      - run: echo after > inspect-after-marker
        after: push
        timeout: 13
      """
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "echo run > inspect-run-marker" before "echo fail > inspect-fail-marker"
    And its output says "echo fail > inspect-fail-marker" before "echo after > inspect-after-marker"
    And its output says "fails:"
    And its output says "timeout: 9"
    And its output says "prose: true"
    And its output says "cost: static"
    And its output says "after: push"

  @ID-SHOW-10 @slice-136 @wip
  Scenario: work show JSON exposes ordered check definitions with their configured fields
    Given the work registry has the item "T-001" owned by nobody with the status "doing"
    And the ledger's task "T-001" has current checks:
      """
      - run: echo run > inspect-run-marker
        timeout: 9
        prose: true
        cost: static
      - fails: echo fail > inspect-fail-marker
        timeout: 11
      - run: echo after > inspect-after-marker
        after: push
        timeout: 13
      """
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "checks" equals:
      """
      [
        {"run":"echo run > inspect-run-marker","timeout":9,"prose":true,"cost":"static"},
        {"fails":"echo fail > inspect-fail-marker","timeout":11},
        {"run":"echo after > inspect-after-marker","after":"push","timeout":13}
      ]
      """

  # The snapshot step records repository files and index state after setup;
  # the Then compares that state rather than taking a second snapshot.
  @ID-SHOW-11 @slice-136 @wip
  Scenario: exposing run fails and after push checks executes none of them and changes no repository files
    Given the work registry has the item "T-001" owned by nobody with the status "doing"
    And the ledger's task "T-001" has current checks:
      """
      - run: echo run > inspect-run-marker
      - fails: echo fail > inspect-fail-marker
      - run: echo after > inspect-after-marker
        after: push
      """
    And the repository files and index are saved for comparison
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "checks" has 3 entries
    And the file "inspect-run-marker" does not exist
    And the file "inspect-fail-marker" does not exist
    And the file "inspect-after-marker" does not exist
    And the repository files and index match the saved state

  @ID-SHOW-12 @slice-136 @wip
  Scenario: a closed task with no current checks exposes an empty list rather than an execution verdict
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    And the ledger's task "T-001" has current checks:
      """
      []
      """
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "checks" equals:
      """
      []
      """

  # Guard: ideas already use their registry why. This case is expected to
  # pass before the slice; adding task checks must preserve that behavior.
  @ID-SHOW-13 @slice-136 @guard @wip
  Scenario: an idea keeps its registry description and has no ledger check definitions
    Given the work registry has the item "inspect-idea" owned by nobody with the status "todo" and the why "Investigate inspection."
    When itos runs "work show inspect-idea --json"
    Then itos exits with code 0
    And its JSON's "item.why" is "Investigate inspection."
    And its JSON has no property "checks"

  # Relocation moves the background's ledger files and changes ledger.files;
  # it does not create a second ledger that the command could read by accident.
  @ID-SHOW-14 @slice-136 @wip
  Scenario: check inspection reads the configured ledger layout
    Given the ledger files are relocated to "acceptance/phase-{group}.yaml"
    And the work registry has the item "T-001" owned by nobody with the status "doing"
    And the ledger's task "T-001" has current checks:
      """
      - run: echo configured-layout
      """
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "checks" equals:
      """
      [{"run":"echo configured-layout"}]
      """

  # The background has T-001 only. Missing ledger data is unknown, not a
  # known-empty done_when; preserve work show's best-effort metadata view.
  @ID-SHOW-15 @slice-136 @wip
  Scenario: a registry task with no ledger entry reports unknown check definitions
    Given the work registry has the item "T-999" owned by nobody with the status "doing"
    When itos runs "work show T-999 --json"
    Then itos exits with code 0
    And its JSON's "checks" is null
