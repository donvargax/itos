@phase-1
Feature: The work registry
  The work registry says who owns each group and each item, and what each
  waits on. It is itos's data, like the ledger, and is read every day, so its
  default home is beside the ledger, work-items.yaml in the folder
  ledger.files names (tasks/work-items.yaml for the default ledger), leaving
  docs/ for prose; work.registry in itos.yaml puts it anywhere else.

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

  # The default is beside the ledger: in the folder ledger.files names, not
  # tasks/ whatever the ledger's folder (the user's call, after slice 18).
  @ID-WORK-04 @slice-22
  Scenario: Without work.registry, itos reads the registry beside a ledger kept in another folder
    Given the ledger's files are "work/phase-{group}.yaml"
    And the work registry at "work/work-items.yaml" has the item "T-001" with the status "doing"
    When itos checks the work registry
    Then itos exits with code 0
    And its output says "work/work-items.yaml: sound"

  # Slice 35: a project's people file is itos init's to report (p3-itos-init);
  # every other command goes on without it, the session having no identity,
  # which is no error.
  @ID-WORK-05 @slice-35
  Scenario: work goes on without the people file, saying nothing of it
    Given the people file is missing
    When itos runs "work"
    Then itos exits with code 0
    And its output does not say "people"

  # Slice 43: itos work lists only what a person can start and what waits,
  # but the plugin's titles (T-066) need every item's title, done ones
  # included, from wherever work.registry puts the registry. work list prints
  # every item in the registry's order, with its kind, status and title,
  # whoever owns it; --json gives the same as items, each with its id and
  # title.
  @ID-WORK-06 @slice-43 @wip
  Scenario: work list prints every item of the registry, done ones included
    Given the work registry has the item "T-001" with the status "done" and the item "slice-1" with the status "todo"
    When itos runs "work list"
    Then itos exits with code 0
    And its output says "T-001"
    And its output says "slice-1"

  @ID-WORK-07 @slice-43 @wip
  Scenario: work list reads the registry where work.registry puts it
    Given work.registry is "plans/work.yaml"
    And the work registry at "plans/work.yaml" has the item "T-001" with the status "done"
    When itos runs "work list --json"
    Then itos exits with code 0
    And its JSON lists the item "T-001" with its title
