@phase-1
Feature: The work registry
  The work registry says who owns each group and each item, and what each
  waits on. It is itos's data, like the ledger, and is read every day, so its
  default home is beside the ledger, tasks/work-items.yaml, leaving docs/ for
  prose; work.registry in itos.yaml puts it anywhere else.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-WORK-01 @slice-5
  Scenario: Without work.registry, itos reads the registry at tasks/work-items.yaml
    Given the work registry at "tasks/work-items.yaml" has the item "T-001" with the status "doing"
    When itos checks the work registry
    Then itos exits with code 0
    And its output says "tasks/work-items.yaml: sound"

  @ID-WORK-02 @slice-5
  Scenario: work.registry overrides where the registry is
    Given work.registry is "plans/work.yaml"
    And the work registry at "plans/work.yaml" has the item "T-001" with the status "doing"
    When itos checks the work registry
    Then itos exits with code 0
    And its output says "plans/work.yaml: sound"

  # A project that kept its registry at the old default, docs/work-items.yaml,
  # learns on its first run of v0.2.0 where itos looks now.
  @ID-WORK-03 @slice-5
  Scenario: With no registry where itos looks, work check says where that is
    Given the work registry at "docs/work-items.yaml" has the item "T-001" with the status "doing"
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "tasks/work-items.yaml"
