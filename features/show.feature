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

  @ID-SHOW-01 @slice-57 @wip
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

  @ID-SHOW-02 @slice-57 @wip
  Scenario: work show lists a task's commits by its Task footer, with its take and close
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "docs: take T-001" touching only "tasks/work-items.yaml" on top of it
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    And the commit "docs: describe the build" on top of it
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "docs: take T-001" before "chore: tidy the readme"
    And its output does not say "docs: describe the build"

  @ID-SHOW-03 @slice-57 @wip
  Scenario: work show --patch adds each commit's diff
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs "work show T-001 --patch"
    Then itos exits with code 0
    And its output says "diff --git"
    And its output says "README.md"

  @ID-SHOW-04 @slice-57 @wip
  Scenario: work show --json gives the item and its commits as data
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "commits" has one entry whose "header" is "chore: tidy the readme"

  @ID-SHOW-05 @slice-57 @wip
  Scenario: work show refuses an item the registry does not have
    Given the work registry has the item "T-001" owned by nobody with the status "todo"
    When itos runs "work show slice-7"
    Then itos exits with code 1
    And its output says "slice-7"

  # Under a stealth config a commit's footers of IDs are in its note in
  # refs/notes/itos, never its message, which grep over the log would miss.
  @ID-SHOW-06 @slice-57 @wip
  Scenario: Under a stealth config work show finds a task's commits by their notes
    Given itos's config is kept in the git folder
    And the work registry beside the config has the item "T-001" owned by "someone" with the status "doing"
    And a change to "README.md" is staged
    And itos has committed with the arguments "--task T-001 -m 'chore: tidy the readme'"
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "chore: tidy the readme"
