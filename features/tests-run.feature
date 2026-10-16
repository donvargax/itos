@phase-1
Feature: Run selected live tests from the current working tree
  # Issue #58, q-82. Forms: tests run <kind> <ID>... or
  # tests run <kind> --item <registry-id>. Exactly one nonempty selection;
  # no implicit whole-suite run, --at, runner-argument passthrough or WIP
  # flag in this delivery. Unknown/duplicate options and mixed modes are 2.
  # List the named kind once at worktree, including uncommitted edits, using
  # its existing adapter. Accept bare IDs or its configured tag prefix,
  # normalize/deduplicate in request order, and require every explicit ID to
  # exist uniquely and be live before starting the runner. Unknown, ambiguous
  # or WIP-only requested IDs are policy exit 1, with no partial run.
  # --item requires an existing registry item but does not require ownership
  # or a particular lifecycle status. Built-in Gherkin only: use work show's
  # exact @<item> association on file-header/scenario tag lines, restricted
  # to this kind and current worktree, in adapter order. Do not collect IDs
  # from another kind. Run its live matches and report WIP matches skipped;
  # no live matches is exit 1. Command adapters have no item-tag protocol:
  # --item is usage 2 with an explicit-ID alternative, not a guessed mapping.
  # Render exactly one IDs selection with the existing CommandFor contract:
  # run.ids_pattern, tag-prefix removal, ID joining and ShellWord escaping
  # into run.select. Never append select to run.whole or fall back to whole.
  # Missing required templates/config is 2 before test execution. Do not
  # invent a generator: projects keep setup in their configured run command.
  # Execute through the configured shell in the effective project directory,
  # inheriting its normal environment and stdin. Text streams runner output
  # normally and identifies selected IDs/command; JSON sends child output to
  # stderr and emits one schema/ok report with kind, bare ids, skipped_wip,
  # command, exit_code (integer|null) and signal (string|null). Successful
  # child exit is 0; preserve an actual nonnegative child exit code on
  # failure. Failure to start or signal termination is 3 with diagnostics.
  # These are runner outcomes, not red-proof or independent execution-count
  # evidence. No special timeout/retry policy is introduced here.
  # itos itself does not edit tags/files, stage/commit, close/take work, wait
  # for hosted CI, create a worktree, stash or swap the checkout. A configured
  # test command can produce its normal generated files. Stealth and --root
  # follow their existing config/CWD rules. Current red-first instructions
  # remain unchanged; this command does not promise to execute WIP tests.

  @wip @slice-162 @ID-TESTRUN-01
  Scenario: Explicit live IDs use the configured selection once on uncommitted inputs
    Given a kind with live tests and a recording run.select command
    And a live test and its input have uncommitted working-tree changes
    When itos runs tests run for that test's bare ID
    Then the adapter lists "worktree" once
    And one runner receives the existing CommandFor ID-selection command
    And the runner reads the uncommitted input
    And no whole-suite command ran

  @wip @slice-162 @ID-TESTRUN-02
  Scenario: Prefix normalization deduplicates IDs without changing template escaping
    Given a kind with a custom ID tag prefix and selection templates
    When explicit IDs repeat in bare and prefixed form
    Then their first-seen normalized order is used once
    And the command matches the existing CommandFor and ShellWord rendering
    And no user ID becomes a separate shell command

  @wip @slice-162 @ID-TESTRUN-03
  Scenario: An item selects only its live Gherkin tests in the named kind
    Given an existing registry item tagged on live and WIP scenarios and a feature header
    And another configured kind has scenarios tagged with that item too
    When tests run selects the first kind with --item
    Then exactly its live item-tagged tests run in adapter order
    And its WIP matches are reported as skipped_wip
    And tests from the other kind are not selected
    And the registry item's ownership and status are unchanged

  @wip @slice-162 @ID-TESTRUN-04
  Scenario: Unknown ambiguous and WIP explicit IDs prevent a partial run
    Given a list containing a valid live test and unknown duplicate-ID or WIP selection cases
    When each request combines the live ID with an invalid selection
    Then itos exits with code 1 before starting the runner
    And the diagnostic identifies the unresolved ambiguous or non-live ID
    And it does not silently drop the invalid test or select the whole suite

  @wip @slice-162 @ID-TESTRUN-05
  Scenario: An item with no live matches is not a successful empty execution
    Given an existing item with only WIP tests in the selected kind
    When tests run selects it with --item
    Then itos exits with code 1 and explains the live-only boundary
    And no runner starts and no @wip tag is edited
    And a missing item or an item with no matching tests also refuses without running

  @wip @slice-162 @ID-TESTRUN-06
  Scenario: Command adapters support explicit IDs without pretending they expose item tags
    Given a command adapter with unique live IDs and no tag metadata
    When tests run receives explicit IDs
    Then its list and configured selection command run normally
    When tests run receives --item instead
    Then itos exits with code 2 and suggests explicit IDs
    And no item ownership or arbitrary file-name inference is attempted

  @wip @slice-162 @ID-TESTRUN-07
  Scenario: Missing templates and invalid invocation never fall back to all tests
    Given a kind with run.whole but without the templates required for selection
    When tests run receives a valid live ID
    Then itos exits with code 2 before the runner starts
    And an empty selection mixed selection --at or an unsupported WIP flag also exits 2
    And the error explains the missing template or unsupported invocation

  @wip @slice-162 @ID-TESTRUN-08
  Scenario: JSON keeps runner output separate and preserves actual failure status
    Given a selected runner that prints text and exits with code 7
    When tests run executes it with --json
    Then itos exits with code 7
    And stdout contains one JSON report with ok false and exit_code 7
    And runner text appears only on stderr
    And no test result is interpreted as expected red success
    And a missing shell or signal-killed runner is reported with code 3

  @wip @slice-162 @ID-TESTRUN-09
  Scenario: Root and stealth execution preserve the shared checkout and metadata
    Given a private stealth config with a recording live-test runner
    And unrelated staged unstaged and untracked changes in the selected project
    When tests run is invoked from elsewhere with --root selecting that project
    Then the runner uses that project's effective directory and normal environment
    And itos creates no commit stash worktree hosted-CI wait or lifecycle transition
    And itos leaves all existing changes and scenario tags intact
