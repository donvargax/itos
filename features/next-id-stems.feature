@phase-1
Feature: An unambiguous bare scenario area gets a correction before any ID is reserved
  # #59, q-75: refuse rather than silently normalize. This first fix uses
  # the current ID- area model: the pattern guarantees an ID- prefix and
  # ID-<input> passes the existing area predicate, while <input> does not.
  # Do not infer prefixes for arbitrary regexes or alternatives with other
  # prefixes. Existing full stems retain their normal claim behavior.
  # Auxiliary previews are distinct: slice and bug are always preserved;
  # an existing raw tag/listing/registry series is preserved too. Thus a
  # custom tag series is not reinterpreted as a new scenario-ID request.
  # Other existing preview behavior stays as it was. A rejected shorthand
  # exits 2, prints no ID, names the supplied stem and configured pattern,
  # and shows a valid corrected command retaining --count when supplied.
  # It never reaches the counter. JSON errors keep their existing shape.
  # Read the kind's data at most once for discovery; do not run tests.

  Background:
    Given a repository whose ledger has the task "T-001"
    And tests.scenario.id is "ID-[A-Z]+-\\d+"
    And the feature files have the live scenario "@ID-SEC-01"
    And the ID counter records every request

  @wip @ID-NEXTID-08 @slice-157
  Scenario Outline: A mistaken bare area is refused with the actual usable stem
    Given no raw "SEC" auxiliary series exists in the kind's tags or registry
    When itos runs "<command>"
    Then itos exits with code 2
    And stdout contains no ID
    And its diagnostic names "SEC" and the configured ID pattern
    And its diagnostic shows "<correction>" as the corrected command
    And it has not printed "@SEC-1" as a result
    And the ID counter received no request
    And HEAD, index and files are unchanged

    Examples:
      | command                                   | correction                                        |
      | tests next-id scenario SEC                 | itos tests next-id scenario ID-SEC                 |
      | tests next-id scenario SEC --count 2       | itos tests next-id scenario ID-SEC --count 2       |

  @wip @ID-NEXTID-09 @slice-157
  Scenario: The correction preserves the existing allocation and width rules
    Given the counter has no prior reservation for "ID-SEC"
    When itos runs "tests next-id scenario SEC --count 2"
    Then itos exits with code 2 without a reservation
    When itos runs "tests next-id scenario ID-SEC --count 2 --json"
    Then itos exits with code 0
    And its JSON retains stem "ID-SEC", claimed true and IDs "@ID-SEC-02" and "@ID-SEC-03"
    And those IDs match the configured pattern after removing the tag prefix
    And the counter recorded only that valid two-ID claim
    When itos runs "tests next-id scenario ID-SEC"
    Then its next claimed ID is "@ID-SEC-04"

  @wip @ID-NEXTID-10 @slice-157
  Scenario: Auxiliary series are not mistaken for shorthand scenario areas
    Given tests.scenario.id also accepts lowercase area letters after "ID-"
    And a scenario has the auxiliary tag "@batch-7"
    And the registry has "slice-9" and "bug-4"
    When I preview the next IDs for stems "slice", "bug" and "batch"
    Then every preview exits with code 0
    And the respective IDs are "@slice-10", "@bug-5" and "@batch-8"
    And their JSON reports claimed false with the original stem
    And the ID counter received no request
    When I request --count for an auxiliary preview
    Then it retains its existing usage-error behavior without a reservation

  @wip @ID-NEXTID-11 @slice-157
  Scenario: A command-adapter kind receives the same correction without duplicate listing
    Given the scenario kind uses a command adapter listing "ID-SEC-01"
    And its configured ID pattern guarantees the "ID-" prefix
    And no raw "SEC" series exists in that listing or the registry
    When itos runs "tests next-id scenario SEC"
    Then itos exits with code 2 and suggests "ID-SEC"
    And the adapter was invoked at most once to list the current tree
    And no adapter runner or counter request occurred

  @wip @ID-NEXTID-12 @slice-157
  Scenario: Refusal remains useful in JSON and with an unavailable counter
    Given the ID counter cannot be reached
    And no raw "SEC" auxiliary series exists in the kind's tags or registry
    When itos runs "tests next-id scenario SEC --json"
    Then itos exits with code 2 using the existing JSON usage-error contract
    And its error names the valid full-stem correction
    And no minted id or ids result is emitted
    And the counter received no request
    When itos runs "tests next-id scenario SEC -q"
    Then it still exits with code 2 and reports the correction as an error

  @wip @ID-NEXTID-13 @slice-157
  Scenario: The correction rule is documented without guessing arbitrary regex prefixes
    Given a kind whose ID pattern admits different literal prefixes and no unique supported correction
    When I request a stem that retains the existing auxiliary-preview behavior
    Then itos does not invent a full-stem correction or silently claim a different series
    And its existing preview result and claimed flag are preserved
    When itos runs "help tests next-id"
    Then its help demonstrates the full stem "ID-SEC"
    And it explains the refused unambiguous bare-area spelling
    And it distinguishes claimed scenario IDs from read-only auxiliary tag previews
