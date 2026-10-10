@phase-1
Feature: The next free ID, for a spec writer
  Writing a scenario or a task meant grepping features/ for the next
  free @ID-, @slice- and @bug- number, and the ledger for the next task ID (the
  user's calls, 2026-10-03, p1-tests-next-id-and-steps). itos tests next-id
  <kind> <stem> prints the next free tag of the kind whose name is the stem,
  a dash and a number: one past the highest the kind's files hold, live
  or @wip, and, for a stem the registry's ids also use (slice-<n>, bug-<n>),
  past the highest of those too, since an item can be named before its
  scenarios are written; a stem nothing uses yet starts at 1, padded to two
  digits where the kind's ID pattern is an ID- one, as the README's IDs are.
  itos task next-id prints the ledger's next task ID the same way, by
  ledger.id. Both judge nothing. task next-id only reads, as tests next-id
  did until slice 105: of an area it now claims the number it prints
  through the id counter (below), so a number a removed scenario held is
  not given again, and the README's rule against reusing one is the
  counter's.

  Background:
    Given a repository whose ledger has the tasks "T-001" and "T-002"

  @ID-NEXTID-01 @slice-60
  Scenario: tests next-id gives the number past the highest scenario ID of the area, live or @wip
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the @wip scenario "@ID-A-03"
    When itos runs "tests next-id scenario ID-A"
    Then itos exits with code 0
    And its output says "@ID-A-04"

  @ID-NEXTID-02 @slice-60
  Scenario: An area no scenario uses yet starts at 01
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos runs "tests next-id scenario ID-NEW"
    Then itos exits with code 0
    And its output says "@ID-NEW-01"

  @ID-NEXTID-03 @slice-60
  Scenario: A slice number counts the registry's items as well as the tags
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@slice-7"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "tests next-id scenario slice"
    Then itos exits with code 0
    And its output says "@slice-10"

  @ID-NEXTID-04 @slice-60
  Scenario: task next-id gives the ledger's next task ID
    When itos runs "task next-id"
    Then itos exits with code 0
    And its output says "T-003"

  @ID-NEXTID-05 @slice-60
  Scenario: tests next-id refuses a kind the config does not have
    When itos runs "tests next-id unit ID-A"
    Then itos exits with code 2
    And its output says "unit"

  # Slice 105 (the user's calls, 2026-10-08, q-27): a scenario id is minted
  # too, so two machines writing specs in one area never take the same
  # number. tests next-id of an area (a stem the kind's ID pattern names,
  # ID-<AREA>) claims the number through the id counter
  # (features/ids.feature), one past the higher of the area's counter and
  # the highest the kind's files hold, and --count <n> claims n in a row,
  # printing each. It no longer only reads: a number it printed is never
  # printed again, used or not. A stem the registry's items use (slice,
  # bug) still only reads, since those ids are minted by the commands that
  # create the items (slice 102). The README's rule against reusing a
  # removed scenario's number becomes the counter's.
  @ID-NEXTID-06 @slice-105
  Scenario: tests next-id claims the number it prints, so the next run gives the one after
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos runs "tests next-id scenario ID-A"
    Then its output says "@ID-A-02"
    When itos runs "tests next-id scenario ID-A"
    Then itos exits with code 0
    And its output says "@ID-A-03"

  @ID-NEXTID-07 @slice-105
  Scenario: tests next-id --count claims several numbers in a row
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    When itos runs "tests next-id scenario ID-A --count 3"
    Then itos exits with code 0
    And its output says "@ID-A-02"
    And its output says "@ID-A-04"
