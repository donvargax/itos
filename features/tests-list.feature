@phase-1
Feature: Live named tests have a directly consumable TSV listing
  # #22: tests list <kind> --tsv [--at <tree>] prints only live tests:
  # <bare ID><TAB><project-relative slash path><LF>, exactly two columns,
  # no heading, quoting, summary or fingerprint. Preserve listing order.
  # Resolve the effective kind root against the project root, then each
  # listed file against that root; a command kind without root uses the
  # project root. Do not hard-code features/ or use the invoking subfolder.
  # Validate every emitted row before writing any: nonempty ID and file,
  # unique live IDs, no TAB/CR/LF/NUL in either field, and no final path
  # outside the project. Such a projection failure exits 1 on stderr.
  # This validation belongs only to TSV; existing human/JSON protocols and
  # adapter errors remain unchanged. --tsv with --json is usage error 2.
  # Keep current --at and supports_at semantics and stderr warnings.
  # A command adapter may run once to list; no test runner is invoked.
  # Per-test fingerprints belong to p1-listed-test-fingerprints. Adopting
  # this output in itos-cc.yaml/removing the wrapper is separate gate work.

  Background:
    Given a repository whose ledger has the task "T-001"

  @wip @ID-TESTLIST-01 @slice-154
  Scenario: TSV prints exact live-test rows without changing the other listing formats
    Given the scenario kind has live IDs "ID-A-01" and "ID-B-01" in two feature files
    And another scenario is tagged @wip and a third feature is tagged @wip
    When itos runs "tests list scenario --tsv"
    Then itos exits with code 0
    And stdout consists only of the two live IDs and their project-relative files, separated by one literal tab per LF-terminated row
    And the rows retain the normal adapter listing order
    And stdout has no tag prefix, wip rows, heading, status words or extra column
    When I obtain the ordinary text and JSON listings of the same kind
    Then those listings retain their existing file paths, live flags and output shape
    And no test runner, hook or CI provider has run

  @wip @ID-TESTLIST-02 @slice-154
  Scenario Outline: TSV paths use the effective kind root and project root
    Given the scenario kind root is "specs with spaces/acceptance"
    And it contains the live scenario "ID-A-01" in "nested/a.feature"
    And the repository uses <layout> configuration
    When I run "itos tests list scenario --tsv" from a project subfolder
    Then itos exits with code 0
    And stdout is exactly ID "ID-A-01", one tab, path "specs with spaces/acceptance/nested/a.feature" and one LF
    And the path has slash separators on every supported platform
    And neither configuration data, HEAD nor the index changed

    Examples:
      | layout  |
      | project |
      | stealth |

  @wip @ID-TESTLIST-03 @slice-154
  Scenario: TSV uses the requested content tree without activating draft scenarios
    Given HEAD has a committed live scenario in "features/old.feature"
    And the index has moved it to "features/staged.feature" and added a live scenario
    And the working tree adds a third live scenario and a @wip scenario
    When I obtain TSV listings at HEAD, index and worktree
    Then each listing exits with code 0
    And each lists precisely its tree's live IDs and project-relative files
    And none lists the @wip scenario
    And the staged changes, unstaged changes and HEAD remain unchanged

  @wip @ID-TESTLIST-04 @slice-154
  Scenario: Command adapters need no Gherkin-specific formatting wrapper
    Given a command adapter lists live IDs "F-003" and "F-001" in that order, with an intervening non-live test
    And that kind has no configured root and lists project-relative files
    And its adapter protocol includes extra object and test fields
    When itos runs "tests list fixture --tsv"
    Then itos exits with code 0
    And stdout contains exactly the two live ID/file rows in adapter order with two tab-separated columns
    And the adapter was called once for "worktree"
    And no runner command was called
    When itos runs "tests list fixture --json"
    Then its JSON still preserves the extra adapter fields
    When the adapter with "supports_at: false" is listed as TSV at HEAD
    Then its existing working-tree fallback warning goes to stderr
    And stdout still contains only TSV rows

  @wip @ID-TESTLIST-05 @slice-154
  Scenario Outline: A row that TSV cannot represent refuses the entire projection
    Given a command adapter lists one valid live row followed by <problem>
    When itos runs "tests list fixture --tsv"
    Then itos exits with code 1
    And stdout is empty
    And stderr identifies the row and why it cannot be represented as project-relative TSV
    And no test runner has run

    Examples:
      | problem                                      |
      | a live ID containing a tab                    |
      | a live file containing a carriage return      |
      | a live file containing a newline              |
      | a live ID containing NUL                      |
      | a live test with an empty ID                  |
      | a live test with an empty file                |
      | a second live row with the same ID            |
      | a live file resolving outside the project     |
      | a live file under a root outside the project  |

  @wip @ID-TESTLIST-06 @slice-154
  Scenario: TSV is an explicit format with a valid empty listing
    Given a kind whose valid listing contains only non-live tests
    When itos runs "tests list fixture --tsv"
    Then itos exits with code 0 and empty stdout
    When itos runs "tests list fixture --tsv --json"
    Then itos exits with code 2 before invoking the adapter
    And its diagnostic names the mutually exclusive output formats
    When itos runs "help tests list"
    Then its help explains the two-column live-only TSV format and project-relative paths
    And it explains that warnings and diagnostics stay on stderr
