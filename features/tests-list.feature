@phase-1
Feature: Live named tests have a directly consumable TSV listing
  # #22: tests list <kind> --tsv [--at <tree>] prints only live tests:
  # <bare ID><TAB><project-relative slash path><LF>, and for a kind that
  # gives its tests a fingerprint (a Gherkin kind, slice-166) a third
  # column, <TAB><fingerprint>, as itos-cc v0.24.0 and later read it; no
  # heading, quoting or summary. Preserve listing order.
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
  # The fingerprint is slice-166's (#112); itos-cc.yaml's mutation.tests.list
  # runs this listing itself, with no wrapper between (T-279, with this
  # slice; the user, 2026-10-22).

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-TESTLIST-01 @slice-154
  Scenario: TSV prints exact live-test rows without changing the other listing formats
    Given the scenario kind has live IDs "ID-A-01" and "ID-B-01" in two feature files
    And another scenario is tagged @wip and a third feature is tagged @wip
    When itos runs "tests list scenario --tsv"
    Then itos exits with code 0
    And stdout consists only of the two live IDs, their project-relative files and their fingerprints, the columns separated by one literal tab, one LF-terminated row each
    And the rows retain the normal adapter listing order
    And stdout has no tag prefix, wip rows, heading, status words or other column
    When I obtain the ordinary text and JSON listings of the same kind
    Then those listings retain their existing file paths, live flags and output shape
    And no test runner, hook or CI provider has run

  @ID-TESTLIST-02 @slice-154
  Scenario Outline: TSV paths use the effective kind root and project root
    Given the scenario kind root is "specs with spaces/acceptance"
    And it contains the live scenario "ID-A-01" in "nested/a.feature"
    And the repository uses <layout> configuration
    When I run "itos tests list scenario --tsv" from a project subfolder
    Then itos exits with code 0
    And stdout is exactly ID "ID-A-01", one tab, path "specs with spaces/acceptance/nested/a.feature", one tab, its fingerprint and one LF
    And the path has slash separators on every supported platform
    And neither configuration data, HEAD nor the index changed

    Examples:
      | layout  |
      | project |
      | stealth |

  @ID-TESTLIST-03 @slice-154
  Scenario: TSV uses the requested content tree without activating draft scenarios
    Given HEAD has a committed live scenario in "features/old.feature"
    And the index has moved it to "features/staged.feature" and added a live scenario
    And the working tree adds a third live scenario and a @wip scenario
    When I obtain TSV listings at HEAD, index and worktree
    Then each listing exits with code 0
    And each lists precisely its tree's live IDs and project-relative files
    And none lists the @wip scenario
    And the staged changes, unstaged changes and HEAD remain unchanged

  @ID-TESTLIST-04 @slice-154
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

  @ID-TESTLIST-05 @slice-154
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

  @ID-TESTLIST-06 @slice-154
  Scenario: TSV is an explicit format with a valid empty listing
    Given a kind whose valid listing contains only non-live tests
    When itos runs "tests list fixture --tsv"
    Then itos exits with code 0 and empty stdout
    When itos runs "tests list fixture --tsv --json"
    Then itos exits with code 2 before invoking the adapter
    And its diagnostic names the mutually exclusive output formats
    When itos runs "help tests list"
    Then its help explains the live-only TSV format, the fingerprint column a Gherkin kind adds, and project-relative paths
    And it explains that warnings and diagnostics stay on stderr

  # Slice 166 (#112, the user, 2026-10-22: as soon as possible): itos-cc dates
  # a kill by a listed test by the test's file, so any edit to a feature file
  # (a comment, a new @wip scenario beside live ones) stales every kill any of
  # its scenarios made, and the code proof goes red after spec commits no
  # test can see. Since itos-cc v0.24.0
  # a listed test may carry a fingerprint, opaque to itos-cc, and its kill
  # holds while the fingerprint does. tests list --json gives each test of a
  # Gherkin kind a "fingerprint": the SHA-256, in lowercase hex, of a
  # canonical text of what the scenario runs: its effective tags (its own and
  # its feature's, sorted, once each), the Background steps before it, then
  # its steps, each step its text with its doc string or table rows, the
  # keyword left out (godog matches step text alone), and for an outline
  # its Examples' header and rows. Never its name, a comment, the feature's
  # description or another scenario; so moving it unchanged to a file with
  # the same Background and feature tags keeps it. The canonical text starts
  # with a version line, so a later change of what counts is a new version,
  # not a silent shift. A command adapter's tests carry none. The text
  # listing is unchanged. tests list --tsv prints it as the third column
  # (slice 154).
  @ID-TESTLIST-07 @slice-166
  Scenario: tests list --json gives each scenario a fingerprint of what it runs
    Given the scenario kind has live IDs "ID-A-01" and "ID-B-01" in two feature files
    When itos runs "tests list scenario --json"
    Then itos exits with code 0
    And each listed test has a fingerprint of 64 lowercase hex digits
    And "ID-A-01" and "ID-B-01" have different fingerprints

  @ID-TESTLIST-08 @slice-166
  Scenario Outline: A scenario's fingerprint holds through edits that change nothing it runs
    Given the scenario kind has live IDs "ID-A-01" and "ID-B-01" in two feature files
    And itos has listed the scenario kind's fingerprints
    When <edit>
    Then the fingerprint of "ID-A-01" is unchanged

    Examples:
      | edit                                                                  |
      | a comment is added above the scenario "ID-A-01"                       |
      | the description of the feature holding "ID-A-01" is reworded          |
      | a @wip scenario is added to the feature file holding "ID-A-01"        |
      | the scenario "ID-A-01" is renamed                                     |
      | the scenario "ID-A-01" moves unchanged to the feature file of "ID-B-01" |

  @ID-TESTLIST-09 @slice-166
  Scenario Outline: A scenario's fingerprint changes with what it runs
    Given the scenario kind has live IDs "ID-A-01" and "ID-B-01" in two feature files
    And itos has listed the scenario kind's fingerprints
    When <edit>
    Then the fingerprint of "ID-A-01" has changed
    And the fingerprint of "ID-B-01" is unchanged

    Examples:
      | edit                                                               |
      | a step of the scenario "ID-A-01" is reworded                       |
      | a table row of a step of the scenario "ID-A-01" is changed         |
      | the Background of the feature holding "ID-A-01" gains a step       |
      | the scenario "ID-A-01" gains the tag "@slow"                       |
      | the feature holding "ID-A-01" gains the tag "@slow"                |

  @ID-TESTLIST-10 @slice-166
  Scenario: An outline's fingerprint changes with its Examples
    Given the scenario kind has the live outline "ID-A-01" with two Examples rows
    And itos has listed the scenario kind's fingerprints
    When a row of the Examples of "ID-A-01" is changed
    Then the fingerprint of "ID-A-01" has changed

  @ID-TESTLIST-11 @slice-166
  Scenario: A command adapter's tests carry no fingerprint, and the text listing is unchanged
    Given a kind whose command adapter lists the live test "T-A" in "a.txt"
    When itos runs "tests list fixture --json"
    Then itos exits with code 0
    And the listed test "T-A" has no fingerprint
    When itos runs "tests list scenario"
    Then its output does not say "fingerprint"
