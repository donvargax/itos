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
  # the items in the registry's order, with their kind, status and title,
  # whoever owns them; --json gives the same as items, each with its id and
  # title. Plain work list printed every item until v4.0.0, which made it the
  # open items alone and --all every one (slice 79, @ID-WORK-44).
  @ID-WORK-07 @slice-43
  Scenario: work list reads the registry where work.registry puts it
    Given work.registry is "plans/work.yaml"
    And the work registry at "plans/work.yaml" has the item "T-001" with the status "done"
    When itos runs "work list --all --json"
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

  # Bug 11: itos work took anything after it for the proposal's arguments,
  # so a subcommand it does not have (a typo, or work add on an itos without
  # it) printed what the person can start and exited 0, the arguments
  # ignored, as if the command had done what was asked.
  @ID-WORK-25 @bug-11
  Scenario: work refuses a subcommand it does not have, naming those it has
    When itos runs "work bogus --x"
    Then itos exits with code 2
    And its output says "bogus"
    And its output says "take, promote, done"

  # Bug 14, found by slice 54's post-landing review: work edit could not
  # replace a flow list the formatter had wrapped over several lines (T-073's
  # refs on this repository's registry), could not empty a block list, and
  # committed a change for --refs '' on an item with no refs. A list given is
  # now written as one flow list on its key's line, whatever the old one's
  # shape, and an absent list reads as an empty one.
  @ID-WORK-26 @bug-14
  Scenario: work edit replaces a refs list that spans several lines
    Given the work registry has the idea "p1-thing" owned by nobody, its refs a flow list over several lines
    When itos runs the command line "work edit p1-thing --refs features/a.feature"
    Then itos exits with code 0
    And the registry's item "p1-thing" has the refs "features/a.feature"

  @ID-WORK-27 @bug-14
  Scenario: work edit empties a refs list written as a block list
    Given the work registry has the idea "p1-thing" owned by nobody, its refs a block list of "a.md" and "b.md"
    When itos runs the command line "work edit p1-thing --refs ''"
    Then itos exits with code 0
    And the registry's item "p1-thing" has no refs

  @ID-WORK-28 @bug-14
  Scenario: work edit with empty refs on an item that has none changes nothing and commits nothing
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs the command line "work edit p1-thing --refs ''"
    Then itos exits with code 0
    And its output says "nothing to change"
    And the last commit's header is not "docs: edit p1-thing"

  # Slice 65 (the user's call, 2026-10-04): promoting an idea usually means it
  # has become something more specific, and promote kept the idea's title, so
  # slice 64 carried a title describing a plan the user had already replaced,
  # which misled them. --title gives the item its new title in the same
  # commit; without it the title is kept, as before.
  @ID-WORK-29 @slice-65
  Scenario: work promote --title gives the promoted item a new title
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs the command line "work promote p1-thing --as slice-7 --kind slice --title 'The thing, specified'"
    Then itos exits with code 0
    And the registry's item "slice-7" is titled "The thing, specified"

  # Slice 66 (the user's calls, 2026-10-04, p1-work-queue-order): the order
  # of the work lived in the handoff's prose, rewritten by hand. The registry
  # now holds it, as a top-level queue: list of item ids, one for the whole
  # repository, ideas included. itos work queue <id> puts an item at the top
  # (--top), before or after another (--before, --after), or takes it out
  # (--drop), committing the registry alone as the other registry commands
  # do (docs: queue <id>). itos work proposes what a person can start in the
  # queue's order, items the queue does not name after them; each person
  # sees only their part, since what they cannot take is not proposed. work
  # done takes a closed item out of the queue in its own commit, and work
  # check refuses a queue naming an item the registry does not have or one
  # twice.
  @ID-WORK-30 @slice-66
  Scenario: work queue --top puts an item first, and commits the registry alone
    Given the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "work queue slice-9 --top"
    Then itos exits with code 0
    And the registry's queue is "slice-9"
    And the last commit's header is "docs: queue slice-9"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-31 @slice-66
  Scenario: work proposes what the person can start in the queue's order, the unqueued after
    Given the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run "work queue slice-8 --top"
    And itos has run "work queue slice-9 --after slice-8"
    When itos runs "work --as someone"
    Then itos exits with code 0
    And its output says "slice-8" before "slice-9"
    And its output says "slice-9" before "slice-7"

  @ID-WORK-32 @slice-66
  Scenario: Each person sees their part of the queue, and not what another owns
    Given the work registry has the item "slice-8" owned by "ana" with the status "todo"
    And the work registry has the item "slice-9" owned by "bo" with the status "todo"
    And itos has run "work queue slice-8 --top"
    And itos has run "work queue slice-9 --after slice-8"
    When itos runs "work --as bo"
    Then itos exits with code 0
    And its output says "slice-9"
    And its output does not say "slice-8"

  @ID-WORK-33 @slice-66
  Scenario: work queue --drop takes an item out of the queue
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run "work queue slice-9 --top"
    When itos runs "work queue slice-9 --drop"
    Then itos exits with code 0
    And the registry's queue is empty

  @ID-WORK-34 @slice-66
  Scenario: work done takes the closed item out of the queue
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And itos has run "work queue slice-9 --top"
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's queue is empty

  @ID-WORK-35 @slice-66
  Scenario: work check refuses a queue naming an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the registry's queue names "slice-404"
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "slice-404"

  # Slice 76 (the user's calls, 2026-10-04): the registry is an index, as its
  # header always said a why is "a short reason, for an idea above all". An
  # idea's why is the registry's, since an idea has no spec yet; a slice's or
  # a bug's lives in its feature file (the description, the comments above
  # its scenarios), a task's in its ledger entry, a change's in its commit.
  # So work done drops the why of the item it closes, a done item holding its
  # index fields alone (id, title, kind, status, owner, phase, depends_on,
  # refs) at about 200 bytes, and the registry grows with the open work, not
  # the history; the dropped text stays in git's history. work show reads a
  # task's why from its ledger entry. A note on a slice or a task landed in
  # its why until v4.0.0, with a warning naming where its why belongs; slice
  # 79 refuses it (@ID-WORK-45).
  @ID-WORK-36 @slice-76
  Scenario: work done drops the why of the item it closes
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing" and the why "Because it was missing."
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and no why

  @ID-WORK-37 @slice-76
  Scenario: work show prints a task's why from its ledger entry
    Given the ledger's task "T-001" has the why "The ledger says why."
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    When itos runs "work show T-001"
    Then itos exits with code 0
    And its output says "The ledger says why."

  # Slice 77: work list's default changes in v4.0.0 (slice 79) to the open
  # items alone, so --all comes first, saying what the default says today,
  # and the plugin's titles, which need every item's title, ask for it before
  # the default changes (falling back to plain work list on an itos older
  # than this slice, which refuses --all).
  @ID-WORK-39 @slice-77
  Scenario: work list --all prints every item of the registry, done ones included
    Given the work registry has the item "T-001" with the status "done" and the item "slice-1" with the status "todo"
    When itos runs "work list --all"
    Then itos exits with code 0
    And its output says "T-001"
    And its output says "slice-1"

  # Slice 78 (the user's calls, 2026-10-04): nothing took an idea out of the
  # registry, so ideas only piled up. work drop <id> --why <reason> sets an
  # item that is not done to the status dropped, drops its why as work done
  # does, takes it out of the queue and commits the registry alone, "docs:
  # drop <id>", the reason as the commit's body: the commit is the record of
  # why. A dropped item stays in the registry, so its id is never given
  # again; work never proposes it and work take refuses it. An item a live
  # item depends on is refused, naming the dependants, so the person edits
  # them first.
  @ID-WORK-40 @slice-78
  Scenario: work drop marks an idea dropped, takes it out of the queue, and commits the reason
    Given the work registry has the idea "p1-thing" owned by nobody
    And the registry's queue names "p1-thing"
    When itos runs the command line "work drop p1-thing --why 'Superseded by slice 9.'"
    Then itos exits with code 0
    And the registry's item "p1-thing" has the status "dropped" and no why
    And the registry's queue does not name "p1-thing"
    And the last commit's header is "docs: drop p1-thing"
    And the last commit's body says "Superseded by slice 9."
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-41 @slice-78
  Scenario: work drop refuses a done item
    Given the work registry has the item "slice-9" owned by nobody with the status "done"
    When itos runs the command line "work drop slice-9 --why 'Too late.'"
    Then itos exits with code 1
    And its output says "done"
    And the registry's item "slice-9" has the status "done" and the owner "nobody"

  @ID-WORK-42 @slice-78
  Scenario: work drop refuses an item a live item depends on, naming it
    Given the work registry has the idea "p1-thing" owned by nobody
    And the work registry has the item "slice-3" owned by nobody with the status "todo", depending on "p1-thing"
    When itos runs the command line "work drop p1-thing --why 'Not needed.'"
    Then itos exits with code 1
    And its output says "slice-3"
    And the registry's item "p1-thing" has the status "todo" and the owner "nobody"

  @ID-WORK-43 @slice-78
  Scenario: work take refuses a dropped item
    Given the work registry has the item "slice-9" owned by nobody with the status "dropped"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 1
    And its output says "dropped"

  # Slice 79, v4.0.0 (the user's calls, 2026-10-04): every breaking change
  # waiting for a major lands in one push, so one release carries them all;
  # what each could ship first shipped as a minor (slices 76 to 78). This
  # flips the defaults: work list prints the open items alone (todo and
  # doing, a deferred one marked so), --all every one; work edit --note on a
  # slice or a task is refused, naming where its why goes; hooks.bin defaults
  # to itos, the global launcher (was v3-hooks-bin-default, after
  # p3-global-install-ci); and a stealth config declares the pre-push hook
  # without hooks.pre_push (was v3-stealth-pre-push). The last two are
  # @ID-CONFIG-26 and 27 and @ID-STEALTH-25.
  @ID-WORK-44 @slice-79
  Scenario: work list prints the open items alone, and --all every one
    Given the work registry has the item "T-001" with the status "done" and the item "slice-1" with the status "todo"
    And the work registry has the item "p1-gone" owned by nobody with the status "dropped"
    When itos runs "work list"
    Then itos exits with code 0
    And its output says "slice-1"
    And its output does not say "T-001"
    And its output does not say "p1-gone"
    When itos runs "work list --all"
    Then itos exits with code 0
    And its output says "T-001"

  @ID-WORK-45 @slice-79
  Scenario: work edit --note on a slice is refused, naming its feature file
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work edit slice-9 --note 'A decision.'"
    Then itos exits with code 1
    And its output says "feature file"
    And the registry is unchanged
