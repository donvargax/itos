@phase-1
Feature: CI setup is discoverable without reading the implementation
  # #57: these examples explain existing planning and execution semantics;
  # they do not add default steps, alter selection, or change the planner.
  # A tests mapping uses the kind's run.whole template, but a ranged push
  # renders the merged selection through that kind's configured commands.
  # The accepted but unused step.whole option is a separate design issue,
  # p1-ci-step-whole-inert. Do not describe it as forcing a whole run here.
  # Keep the code-proof wording correction (#45) as its own item.

  @wip @ID-CI-40 @slice-151
  Scenario: CI's entry point directs a new consumer to the detailed step reference
    Given a folder outside a Git repository with no itos config
    When I run "itos ci --help"
    Then itos exits with code 0
    And its output points to "itos help ci run" for CI step forms and range inputs
    When I run "itos help ci"
    Then itos exits with code 0
    And it gives the same step-reference guidance
    And neither help invocation creates files or invokes a configured command

  @wip @ID-CI-41 @slice-151
  Scenario: The CI run reference explains each supported step form accurately
    Given a folder outside a Git repository with no itos config
    When I run "itos help ci run"
    Then itos exits with code 0
    And its output shows a plain string command in "ci.steps"
    And its output shows mappings with "run" and "cost: static" or "cost: late"
    And it explains that an explicit mapping cost overrides command-pattern classification
    And its output shows "tests: scenario" referring to a configured tests kind
    And it explains the merged named-test selection for a ranged push
    And it explains that a no-range run selects every test
    And it warns that the accepted "whole" step flag currently has no effect
    And it does not present that flag as a way to force every test
    When I run "itos ci run --help"
    Then it gives the same step forms and caveat

  @wip @ID-CI-42 @slice-151
  Scenario: Range inputs are explained as values supplied by itos
    Given a folder outside a Git repository with no itos config
    When I run "itos help ci run"
    Then itos exits with code 0
    And it explains that steps and task checks inherit "ITOS_FROM" and "ITOS_TO"
    And it shows "{from}" and "{to}" substitution in a step command
    And it explains that "ci plan" prints the substituted step command
    And it explains resolved commit SHAs and an empty start for an all-history range
    And it does not require a workflow to set these variables itself
    And no step or task check has run

  @wip @ID-CI-43 @slice-151
  Scenario: Defaults show disabled CI examples without introducing executable defaults
    Given a repository whose config has no "ci.steps"
    When I run "itos config check --print-defaults"
    Then itos exits with code 0
    And its YAML comments show a "ci.steps" example containing a string, a run-cost mapping and a tests-kind mapping
    And those comments explain the range environment variables and text placeholders
    And those comments warn that the accepted "whole" step flag currently has no effect
    And every line of the example is a YAML comment
    And parsing the output produces the same defaults mapping as the JSON defaults output
    And that mapping has no "ci.steps" key
    And no example command or test-kind listing has run
    And HEAD, the index and repository files are unchanged

  @wip @ID-CI-44 @slice-151
  Scenario Outline: Defaults examples remain available when a config cannot be loaded
    Given a folder whose itos config is <config>
    When I run "itos config check --print-defaults"
    Then itos exits with code 0
    And its YAML comments explain the CI step forms and range inputs
    And the parsed output contains the ordinary fallback defaults without executable example steps
    And it has not written or repaired a config

    Examples:
      | config    |
      | missing   |
      | malformed |

  @wip @ID-CI-45 @slice-151
  Scenario: Comment examples preserve configured and stealth defaults and the JSON contract
    Given a stealth repository with a custom configured ledger location
    And its configured commands and test adapter record any invocation
    When I obtain text and JSON output from "itos config check --print-defaults"
    Then both commands exit with code 0
    And the text comments contain the CI step examples
    And parsing that text gives the JSON object's "defaults" mapping
    And the existing stealth and ledger-location defaults are preserved
    And JSON has no examples or help fields and no newly defaulted CI steps
    And the adapter and configured commands have not run
    And neither the private nor project config, HEAD or index has changed
