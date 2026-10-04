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
  @ID-WORK-06 @slice-43
  Scenario: work list prints every item of the registry, done ones included
    Given the work registry has the item "T-001" with the status "done" and the item "slice-1" with the status "todo"
    When itos runs "work list"
    Then itos exits with code 0
    And its output says "T-001"
    And its output says "slice-1"

  @ID-WORK-07 @slice-43
  Scenario: work list reads the registry where work.registry puts it
    Given work.registry is "plans/work.yaml"
    And the work registry at "plans/work.yaml" has the item "T-001" with the status "done"
    When itos runs "work list --json"
    Then itos exits with code 0
    And its JSON lists the item "T-001" with its title

  # Slices 52 and 53 (the user's calls, 2026-10-03): the registry is written
  # by commands, never by hand, and each command commits its own change, so
  # the coordinator neither edits YAML nor words a registry commit. Each
  # commit holds the registry alone, whatever else is staged, and goes
  # through the hooks as any commit does. Under a stealth config the registry
  # is not tracked, so the command writes it and commits nothing.

  @ID-WORK-08 @slice-52
  Scenario: work take sets an item in progress for the person, and commits it
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "doing" and the owner "someone"
    And the last commit's header is "docs: take slice-9"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-09 @slice-52
  Scenario: work take refuses an item someone else owns
    Given the work registry has the item "slice-9" owned by "another" with the status "todo"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 1
    And its output says "another"
    And the registry's item "slice-9" has the status "todo" and the owner "another"

  @ID-WORK-10 @slice-52
  Scenario: work take refuses an item whose dependencies are not done
    Given the work registry has the item "slice-8" owned by nobody with the status "doing"
    And the work registry has the item "slice-9" owned by nobody with the status "todo", depending on "slice-8"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 1
    And its output says "slice-8"

  # An idea is not yet specified; work promote makes it a slice or a task.
  @ID-WORK-11 @slice-52
  Scenario: work take refuses an idea, naming work promote
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work take p1-thing --as someone"
    Then itos exits with code 1
    And its output says "work promote"

  @ID-WORK-12 @slice-52
  Scenario: work promote renames an idea, makes it a slice, rewrites what depends on it, and commits
    Given the work registry has the idea "p1-thing" owned by nobody
    And the work registry has the item "slice-3" owned by nobody with the status "todo", depending on "p1-thing"
    When itos runs "work promote p1-thing --as slice-7 --kind slice"
    Then itos exits with code 0
    And the registry has no item "p1-thing"
    And the registry's item "slice-7" is a slice whose why starts with "Was p1-thing."
    And the registry's item "slice-3" depends on "slice-7"
    And the last commit touches only "tasks/work-items.yaml"

  # A file the person staged for their own commit stays staged, and out of
  # the registry's commit.
  @ID-WORK-13 @slice-52
  Scenario: A registry command commits the registry alone, leaving what else is staged
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And a change to "README.md" is staged
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 0
    And the last commit touches only "tasks/work-items.yaml"
    And "README.md" is still staged

  @ID-WORK-14 @slice-52
  Scenario: Under a stealth config a registry command writes the registry and commits nothing
    Given itos's config is kept in the git folder
    And the work registry beside the config has the item "slice-9" owned by "someone" with the status "todo"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 0
    And the work registry beside the config gives the item "slice-9" the status "doing"
    And git status shows nothing to commit

  # Slice 53: done is the landing's check, run by the agent after its push.
  # A slice's scenarios are those tagged @slice-<n> for the item slice-<n>.
  @ID-WORK-15 @slice-53
  Scenario: work done refuses a slice one of whose scenarios is still @wip, naming it
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And a feature file with the scenario "@ID-A-01" tagged "@slice-9 @wip"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "@ID-A-01"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  @ID-WORK-16 @slice-53
  Scenario: work done refuses while the branch has commits no remote has, naming itos push
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "itos push"

  @ID-WORK-17 @slice-53
  Scenario: work done refuses while HEAD's CI run failed, with ci.watch
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://ci.example/runs/1"

  @ID-WORK-18 @slice-53
  Scenario: work done, everything landed, marks the item done and commits it
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and the owner "someone"
    And the last commit's header is "docs: close slice-9"

  # Slice 54 (the user's calls, 2026-10-03): the rest of the registry is
  # written by commands too, as take, promote and done already are, so no one
  # edits it by hand. work add makes an item, an idea unless --kind says
  # otherwise, todo, owned by nobody unless --owner; work edit changes its
  # title, its depends_on or its refs, or appends a paragraph to its why
  # (--note). Each checks the result as work check does, refuses what would
  # not be sound, and commits the registry alone, as slice 52's commands do.
  @ID-WORK-19 @slice-54
  Scenario: work add makes an idea, todo and owned by nobody, and commits it
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add p1-thing --title 'Do the thing' --why 'Because it is missing.'"
    Then itos exits with code 0
    And the registry's item "p1-thing" is an idea titled "Do the thing" with the status "todo"
    And the last commit's header is "docs: add p1-thing"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-20 @slice-54
  Scenario: work add refuses an id another item has
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add slice-9 --title 'Again' --why 'A second one.'"
    Then itos exits with code 1
    And its output says "slice-9"
    And the registry's item "slice-9" has the status "todo" and the owner "nobody"

  @ID-WORK-21 @slice-54
  Scenario: work add refuses a dependency on an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add p1-thing --title 'Do the thing' --why 'Because.' --depends-on p1-missing"
    Then itos exits with code 1
    And its output says "p1-missing"
    And the registry has no item "p1-thing"

  @ID-WORK-22 @slice-54
  Scenario: work edit --note appends a paragraph to an item's why, and commits it
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs the command line "work edit p1-thing --note 'Seen again while building slice 54.'"
    Then itos exits with code 0
    And the registry's item "p1-thing" has a why ending with "Seen again while building slice 54."
    And the last commit's header is "docs: edit p1-thing"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-23 @slice-54
  Scenario: work edit replaces an item's title and dependencies
    Given the work registry has the item "slice-8" owned by nobody with the status "done"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work edit slice-9 --title 'A better title' --depends-on slice-8"
    Then itos exits with code 0
    And the registry's item "slice-9" is titled "A better title"
    And the registry's item "slice-9" depends on "slice-8"

  @ID-WORK-24 @slice-54
  Scenario: work edit refuses an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work edit slice-7 --note 'Nothing to note.'"
    Then itos exits with code 1
    And its output says "slice-7"
