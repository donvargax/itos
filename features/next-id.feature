@phase-1
Feature: The next free ID, for a spec writer
  Writing a scenario or a task meant grepping features/ for the next free
  @ID-, @slice- and @bug- number, and the ledger for the next task ID (the
  user's calls, 2026-10-03, p1-tests-next-id-and-steps). itos tests next-id
  <kind> <stem> prints the next free tag of the kind whose name is the stem,
  a dash and a number: one past the highest the kind's files hold, live or
  @wip, and, for a stem the registry's ids also use (slice-<n>, bug-<n>),
  past the highest of those too, since an item can be named before its
  scenarios are written; a stem nothing uses yet starts at 1, padded to two
  digits where the kind's ID pattern is an ID- one, as the README's IDs are.
  itos task next-id prints the ledger's next task ID the same way, by
  ledger.id. Help only: it reads, never writes, and judges nothing. A
  number a removed scenario held is not seen, since only the tree is read;
  the README's rule against reusing one stays the writer's.

  Background:
    Given a repository whose ledger has the tasks "T-001" and "T-002"

  @ID-NEXTID-01 @slice-60 @wip
  Scenario: tests next-id gives the number past the highest scenario ID of the area, live or @wip
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the @wip scenario "@ID-A-03"
    When itos runs "tests next-id scenario ID-A"
    Then itos exits with code 0
    And its output says "@ID-A-04"

  @ID-NEXTID-02 @slice-60 @wip
  Scenario: An area no scenario uses yet starts at 01
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos runs "tests next-id scenario ID-NEW"
    Then itos exits with code 0
    And its output says "@ID-NEW-01"

  @ID-NEXTID-03 @slice-60 @wip
  Scenario: A slice number counts the registry's items as well as the tags
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-7"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "tests next-id scenario slice"
    Then itos exits with code 0
    And its output says "@slice-10"

  @ID-NEXTID-04 @slice-60 @wip
  Scenario: task next-id gives the ledger's next task ID
    When itos runs "task next-id"
    Then itos exits with code 0
    And its output says "T-003"

  @ID-NEXTID-05 @slice-60 @wip
  Scenario: tests next-id refuses a kind the config does not have
    When itos runs "tests next-id unit ID-A"
    Then itos exits with code 2
    And its output says "unit"
