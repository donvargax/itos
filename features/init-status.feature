@phase-1
Feature: A rerun reports the existing config separately from requested setup writes
  # #50: initReport prints its config result before plugin/shim/rules
  # offers run. Say only that the existing config was kept, not that init
  # as a whole wrote nothing. Do not reorder offers, add a mode, alter
  # defaults or change what any offer writes. JSON retains its contract,
  # which the init corpus cases already hold: no scenario of its own.
  # A fix must name the previous release's affected non-help corpus cases
  # in Changes: the cases beginning "init with a config there writes
  # nothing" and "run again where nothing is missing" in init.yaml.
  # No rewriting published notes or the historical "two-line" wording.

  Background:
    Given a repository with a valid existing itos config and installed hooks
    And all release/update observations use local fixtures

  @wip @ID-INITSTATUS-01 @bug-114
  Scenario: A requested rules rewrite is not preceded by a global wrote-nothing claim
    Given AGENTS.md has a stale marked itos rules block
    When itos runs "init --agent-rules --plugin no --no-git-shim"
    Then itos exits with code 0
    And its first existing-config status explicitly says that the config contents were kept
    And it does not say that init as a whole wrote nothing
    And the stale marked rules block is updated using the existing rules behavior
    And the rules result separately reports what it wrote

  @wip @ID-INITSTATUS-02 @bug-114
  Scenario: A current rules block keeps its own truthful result
    Given AGENTS.md and its Claude import already contain the current rules
    When itos runs "init --agent-rules --plugin no --no-git-shim"
    Then itos exits with code 0
    And its first existing-config status explicitly says that the config contents were kept
    And its rules result separately says that the block is already current
    And no files changed
