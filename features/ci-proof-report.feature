@phase-1
Feature: CI reports whether its configured ranged code proof is scheduled
  # #36 and the explicit-warning alternative in #35. This is reporting,
  # not a new whole-tree proof policy. Keep the exact selected commands,
  # ordering, execution, provider parsing, exit codes and JSON fields.
  # Text plan rows identify a selected step as "code proof" beside its
  # cost and command. That label is not a passed verdict. Full ci run logs
  # label the proof when actually reached, before running its command.
  # When proof.code exists but no automatic proof step was selected, text
  # plans and full-run logs explain why: no resolved start, an unreadable
  # range, or a readable range touching no configured proof paths. Prefer
  # the missing-start explanation when no start exists. Never describe an
  # unreadable range as proof that no code changed. Never execute a proof
  # merely to find out why it was omitted or invent a base for a whole run.
  # Existing JSON order[].proof is sufficient; full-run log messages go to
  # stderr with --json. No new machine fields or generated AGENTS CI list.
  # --tests-only and configured nightly execution retain their contracts.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the CI steps and named tests are recording commands that succeed

  @wip @ID-CI-46 @slice-155
  Scenario: A scheduled proof is named in the plan without claiming it passed
    Given proof.code has a recording check and matches "internal/**"
    And a readable commit range changes "internal/example.go"
    When I print the text CI plan for that range
    Then itos exits with code 0
    And the proof command appears once in the plan, labelled "code proof" with its cost
    And its resolved base and position before named tests are unchanged
    And the plan does not report a passing proof result
    And no configured check, step or test command has run
    When I run full CI for that range
    Then itos labels the code proof when reaching it
    And its proof command runs once at its existing position
    And the existing successful CI result is preserved

  @wip @ID-CI-47 @slice-155
  Scenario: A readable range with no matching code explains the omitted proof
    Given proof.code has a recording check and matches "internal/**"
    And a readable commit range changes only a file outside those paths
    When I print the text CI plan and run full CI for that range
    Then both commands exit with code 0
    And each explains that the configured code proof is not scheduled because no matching proof path changed
    And neither reports that a proof command ran or passed
    And the proof recording check has not run
    And every other planned command and final CI result retains its ordinary behavior

  @wip @ID-CI-48 @slice-155
  Scenario: A whole run explicitly reports that it lacks a start for ranged proof
    Given proof.code has a recording check and matches "internal/**"
    And no configured fallback supplies a range start
    When itos runs "ci plan --whole"
    Then itos exits with code 0
    And it says the configured ranged code proof is not scheduled because there is no range start
    And it does not describe this as a passing code proof
    When itos runs "ci run"
    Then itos exits with code 0 after the ordinary whole run
    And its log gives the same missing-start explanation before the final CI result
    And the proof recording check has not run
    And no root commit or empty-tree value was substituted as a fabricated proof base

  @wip @ID-CI-49 @slice-155
  Scenario: An unreadable range is not reported as having no matching changes
    Given proof.code has a recording check and matches "internal/**"
    And Git can resolve the endpoint commits but cannot enumerate the range
    And the existing planner therefore selected no automatic proof command
    When I print the text CI plan for that range
    Then itos retains its existing plan exit behavior
    And the proof explanation says the range could not be read
    And it does not claim that no matching code changed
    And it does not claim a passed proof or invent a replacement proof command

  @wip @ID-CI-50 @slice-155
  Scenario: Proof reporting keeps machine contracts and error outcomes intact
    Given proof.code has a recording check and matches "internal/**"
    And a readable commit range changes "internal/example.go"
    When I print the JSON CI plan for that range
    Then its existing selected order entry retains "proof: true"
    And its JSON fields and command strings retain their existing shape
    And no new proof-summary field replaces or supplements that marker
    When I run full CI for that range as JSON with a proof provider reporting a failing verdict
    Then itos exits with code 1
    And the code-proof label and diagnostic appear only in the existing stderr log
    And stdout remains one valid JSON result with the existing failure and problem fields
    When I run full CI as JSON for a readable range outside the proof paths
    Then the omitted-proof explanation is on stderr and the existing successful result is on stdout

  @wip @ID-CI-51 @slice-155
  Scenario: Reporting does not introduce a proof gate into modes that do not select it
    Given proof.code has a recording check and matches "internal/**"
    When I run CI with "--tests" for a matching range
    Then only the existing named-test run is executed
    And no new proof preamble or proof command is added
    When I plan and run the configured nightly
    Then its configured step sequence and output conventions are unchanged
    And no automatic ranged proof or omitted-proof warning is injected
    When I remove proof.code and print a ranged text CI plan
    Then no proof label or omitted-proof warning appears
    And the ordinary plan still contains all its existing steps
