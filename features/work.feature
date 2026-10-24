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
    When itos runs "work promote p1-thing --kind slice"
    Then itos exits with code 0
    And the registry has no item "p1-thing"
    And the registry's item "slice-4" is a slice whose why starts with "Was p1-thing."
    And the registry's item "slice-3" depends on "slice-4"
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

  # Bug 130, found by slice 138's unit tests: the check every registry writer
  # makes before it writes, one check they all share (the tracker's Ready),
  # took an error from git status for a file with changes, so a git that
  # fails refused with "<file> has changes no commit holds", naming the wrong
  # cause. git's failure is an error, naming git's command and what git said,
  # and it exits 70, as a git that fails while work add reserves an ID does
  # (@ID-IDS-08, bug 119): the repository is located and the config loaded by
  # then, so no kind fits. The wrap, kind.Wrap(kind.Unknown, …), sits in the
  # tracker adapter's Files, as Files.counter's does for bug 119. Exit 1
  # stays for policy, a file the person changed or a commit git or a hook
  # refused (work-commit-failed). git's words are kept by the VCS's
  # PathStatus itself, read through the adapter's output (port.Failed, which
  # holds git's stderr) in place of git.Read, which drops it: one call
  # changed, no second status read beside it. Its other caller, itos init's
  # excludeIfShown, prints the error after "Could not list … in the git
  # folder's info/exclude:" and goes on, so its exit code is unchanged; only
  # the words after the colon change, which no scenario or corpus case reads.
  # The git that fails is the one itos runs (ITOS_GIT), set after the
  # registry is committed, and fails git status alone: the hooks check before
  # it still finds itos's hooks declared, and git ls-files still finds the
  # registry tracked, so git status's failure alone decides the outcome. The
  # fixture's words are judged, not itos's: what git said reaches the person
  # (ADR-0052).
  @ID-WORK-151 @bug-130
  Scenario: A git status that fails is reported as git's failure, exit 70, not as changes no commit holds
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the git itos runs fails git status, saying "the fake git refuses status"
    When itos runs "work take slice-9 --as someone"
    Then itos exits with code 70
    And its output says "git status"
    And its output says "the fake git refuses status"
    And its output does not say "has changes no commit holds"
    And the registry is unchanged

  # Bug 130's guard: a registry changed and the change not committed is still
  # refused as before, exit 1 with the rule work-registry-uncommitted. An
  # edit, not a deletion as in @ID-TASK-09: the file is there and git tracks
  # it, so git status's line alone decides, the path the fix touches. The
  # change is a YAML comment, so the registry stays sound and no earlier
  # refusal answers instead. Expected to pass before the fix.
  @ID-WORK-152 @bug-130 @guard
  Scenario: A registry edit no commit holds is still refused as changes no commit holds
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the registry has a change no commit holds
    When itos runs "work take slice-9 --as someone --json"
    Then itos exits with code 1
    And its JSON's problem "work-registry-uncommitted"'s "message" says "tasks/work-items.yaml"
    And the registry's item "slice-9" has the status "todo" and the owner "nobody"
    And the last commit's header is not "docs: take slice-9"

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
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://ci.example/runs/1"

  @ID-WORK-18 @slice-53
  Scenario: work done, everything landed, marks the item done and commits it
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
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

  # Bug 124: bug 11's fix read --as=<handle>, and an empty handle, --as= or
  # --as '' (what --as "$UNSET" passes), was taken as no --as at all, so the
  # session fell back to the identity provider and was proposed whoever gh
  # is signed in as, exit 0: bug 11's failure mode, another person's work
  # printed as the session's. An empty handle is refused as --as with none
  # is (exit 2), before the provider is asked. The scratch config names no
  # work.identity, so the provider is github; with two people it asks gh,
  # signed in as ana, so a fallback says "Working for ana." and proposes
  # ana's slice-8.
  @ID-WORK-145 @bug-124
  Scenario Outline: work refuses an empty --as rather than asking who the session is
    Given the people file lists "ana" and "bo"
    And a gh on the PATH signed in as "ana"
    And the work registry has the item "slice-8" owned by "ana" with the status "todo"
    When itos runs "work <empty handle>"
    Then itos exits with code 2
    And its output says "--as"
    And its output does not say "slice-8"

    Examples:
      | empty handle |
      | --as=        |
      | --as ''      |

  # Bug 124: work take and work give-back read --as as every registry
  # command reads its flags, each with a value, so an empty one is refused
  # (exit 2) today; they must keep refusing it once the proposal does. The
  # people file the scratch repository starts with lists only someone, who
  # owns slice-9 in progress: a take that fell back to who the people make
  # the session would change nothing and exit 0, a give-back would set it
  # todo.
  @ID-WORK-146 @bug-124 @guard
  Scenario Outline: work take and work give-back refuse an empty --as, writing nothing
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs the command line "<command>"
    Then itos exits with code 2
    And its output says "--as"
    And the registry is unchanged

    Examples:
      | command                                         |
      | work take slice-9 --as=                         |
      | work give-back slice-9 --why 'Blocked.' --as '' |

  # Bug 124: work check [<file>] judges one registry, the config's or the
  # file named, and its --json is that one file's, but it judged its first
  # argument and dropped the rest: work check a.yaml b.yaml said a.yaml was
  # sound and exited 0, b.yaml never read. A second file is a usage error
  # (exit 2) naming it, as work show refuses a second id; judging several
  # was never promised and would change --json's shape. plans/other.yaml is
  # not there, so judged it would exit 1 and ignored it exits 0. The words
  # naming it are new.
  @ID-WORK-147 @bug-124
  Scenario: work check refuses a second file rather than ignoring it
    Given the work registry at "plans/work.yaml" has the item "T-001" with the status "doing"
    When itos runs "work check plans/work.yaml plans/other.yaml"
    Then itos exits with code 2
    And its output says "plans/other.yaml"

  # Bug 124: the one file work check is named is still the one judged. No
  # scenario named a file before; work.registry is not set, so only the
  # name reaches plans/work.yaml.
  @ID-WORK-148 @bug-124 @guard
  Scenario: work check judges the one file it is named
    Given the work registry at "plans/work.yaml" has the item "T-001" with the status "doing"
    When itos runs "work check plans/work.yaml"
    Then itos exits with code 0
    And its output says "plans/work.yaml: sound"

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
    When itos runs the command line "work promote p1-thing --kind slice --title 'The thing, specified'"
    Then itos exits with code 0
    And the registry's item "slice-1" is titled "The thing, specified"

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
  Scenario: work queue --remove takes an item out of the queue
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run "work queue slice-9 --top"
    When itos runs "work queue slice-9 --remove"
    Then itos exits with code 0
    And the registry's queue is empty

  @ID-WORK-34 @slice-66
  Scenario: work done takes the closed item out of the queue
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And itos has run "work queue slice-9 --top"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
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
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
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

  # Bug 25 (the user's call, 2026-10-05): slice 79 read "open" as todo and
  # doing, so plain work list left a blocked item out, though it is work
  # still to be done, only waiting. work.statuses is the project's to set, so
  # the rule is by the two statuses itos knows to be closed: plain work list
  # prints every item whose status is neither done nor dropped, blocked and
  # any status a project adds among them.
  @ID-WORK-46 @bug-25
  Scenario: work list prints a blocked item among the open ones
    Given the work registry has the item "T-001" with the status "done" and the item "slice-1" with the status "blocked"
    When itos runs "work list"
    Then itos exits with code 0
    And its output says "slice-1"
    And its output does not say "T-001"

  # Bug 46 (the user's calls, 2026-10-05). Who the session is came from the
  # identity provider, github by default, which ran gh api user with no
  # timeout and stdin inherited, on every itos work, work take, itos status
  # and itos go in a project, whatever the people file said: a gh call on
  # every command of a solo project, and a hang when gh waited on a keychain
  # prompt, a sign-in or a proxy (a work take killed after a minute, on a
  # stealth-free root on macOS). Now a people file listing one login makes
  # that login the session without asking; with no people file nobody is
  # asked and an item taken keeps its owner, as under a stealth config; only
  # with several people is the provider asked, and gh then gets a few seconds
  # and no stdin, a gh that does not answer refused with words naming --as.
  @ID-WORK-47 @bug-46
  Scenario: With one person in the people file, work take takes the item for them without asking gh
    Given the people file lists only "solo"
    And no identity can be looked up
    And the work registry has the item "slice-1" owned by nobody with the status "todo"
    When itos runs "work take slice-1"
    Then itos exits with code 0
    And its output says "solo"

  @ID-WORK-48 @bug-46
  Scenario: With several people, a gh that does not answer is given up on, naming --as
    Given the people file lists "solo" and "other"
    And a gh on the PATH that never answers
    And the work registry has the item "slice-1" owned by nobody with the status "todo"
    When itos runs "work take slice-1"
    Then itos exits with code 3
    And its output says "--as"

  @ID-WORK-49 @bug-46
  Scenario: With no people file, work take asks nobody
    Given the people file is missing
    And no identity can be looked up
    And the work registry has the item "slice-1" owned by nobody with the status "todo"
    When itos runs "work take slice-1"
    Then itos exits with code 0

  @ID-WORK-50 @bug-46
  Scenario: With one person in the people file, itos status names their work without asking gh
    Given the people file lists only "solo"
    And no identity can be looked up
    When itos runs "status"
    Then itos exits with code 0
    And its output says "solo"
    And its output does not say "gh"

  # Bug 133 (#88, 8.5.0). Bug 46 made every item the session's when no one
  # is in the people: with no people file, an unreadable one, or one
  # listing no one, and no --as, work take asks no identity provider, succeeds, and leaves the
  # item's owner as it is, null for an item nobody owned. Its line read
  # "slice-1 is in progress: committed …", where a take for someone says
  # "is in progress for someone", so nothing said the take recorded no
  # owner. The line says it in the same place: "<id> is in progress with
  # no owner recorded: <what was committed>", and for a take that changes
  # nothing "<id> is already in progress with no owner recorded; nothing to
  # change". The rule is the owner the item has after the take, so a
  # stealth take of an unowned item says it too; give-back's line, which
  # shares the " for <owner>" words, does not change. The Background's
  # scratch config names the people file people.yaml and no work.identity,
  # so the provider would be github; the people file is removed, and the gh
  # first on the PATH is signed out, so a take that asked it would exit 3
  # (@ID-WORK-49 holds that none is asked). Fails at the output step: today
  # the lines are "slice-1 is in progress: committed …" and "slice-1 is
  # already in progress; nothing to change".
  @ID-WORK-153 @bug-133 @wip
  Scenario Outline: With no people file, work take says it records no owner
    Given the people file is missing
    And no identity can be looked up
    And the work registry has the item "slice-1" owned by nobody with the status "<status>"
    When itos runs "work take slice-1"
    Then itos exits with code 0
    And its output says "<line>"

    Examples:
      | status | line                                                     |
      | todo   | slice-1 is in progress with no owner recorded: committed |
      | doing  | slice-1 is already in progress with no owner recorded;   |

  # Bug 133: the take keeps bug 46's design, succeeding with the owner left
  # null, and its --json does not change: the item's owner null already
  # says no owner was recorded, and a key for it would change schema 1's
  # shape for one case. Givens as @ID-WORK-153's. Passes before the fix.
  @ID-WORK-154 @bug-133 @wip @guard
  Scenario: With no people file, work take still succeeds and commits, its JSON's owner null
    Given the people file is missing
    And no identity can be looked up
    And the work registry has the item "slice-1" owned by nobody with the status "todo"
    When itos runs "work take slice-1 --json"
    Then itos exits with code 0
    And its JSON's "ok" is true
    And its JSON's "item.status" is "doing"
    And its JSON's "item.owner" is null
    And the registry's item "slice-1" has the status "doing" and the owner "nobody"
    And the last commit's header is "docs: take slice-1"

  # Bug 133: take's and give-back's help promised exit 3 whenever the
  # person is nobody, with only a stealth config spared, yet both find the
  # person the same way (whoActs), and with no one in the people (a people
  # file missing, unreadable or listing no one, as Registry.Nobody reads
  # it) a take or a give-back with no --as succeeds. Each help names the
  # case in the same sentence, beside the stealth one, which bug 46 made the
  # same. Take's help ends: "Under a stealth config the registry is written
  # and nothing committed. Under a stealth config, or with no one in the
  # people (no people file, an unreadable one, or one listing no one), with
  # no --as nobody is asked and nothing exits 3: every item is the
  # session's, its owner is left as it is, and the line says "no owner
  # recorded" when it has none." Give-back's: "Under a stealth config the
  # registry is written and nothing committed. Under a stealth config, or
  # with no one in the people (no people file, an unreadable one, or one
  # listing no one), with no --as nobody is asked and nothing exits 3: every
  # item is the session's." Give-back's line does not change, its owner
  # kept. No corpus case pins help's words (help.yaml pins exit 0 and the
  # subcommands named), so this scenario does; its step folds the line
  # breaks the help is wrapped at. Help is printed before any config is
  # read, so it needs no Given. Fails at the first output step.
  @ID-WORK-155 @bug-133 @wip
  Scenario Outline: work take's and give-back's help name the write with no one in the people
    When itos runs "help work <command>"
    Then itos exits with code 0
    And its output says "or with no one in the people (no people file, an unreadable one, or one listing no one), with no --as nobody is asked and nothing exits 3", whatever its line breaks
    And its output says "<words>", whatever its line breaks

    Examples:
      | command   | words                                         |
      | take      | no owner recorded                             |
      | give-back | nothing exits 3: every item is the session's. |

  # Bug 133: work show printed "owner: nobody" for an item with no owner,
  # so an item a take with no one in the people put in progress read as in
  # progress for nobody. It prints the owner as it prints every field with
  # no value (kind: -) and as work list's owner column does: "-", whatever
  # the status, so a todo item's line reads "owner: -" too. status names no
  # owner of an item in progress (its "(unowned)" marks only the next items
  # nobody owns). --json does not change: owner null. The registry is what
  # such a take leaves (@ID-WORK-154); work show asks no one who the
  # session is. Fails at the first output step: today "owner: nobody,".
  @ID-WORK-156 @bug-133 @wip
  Scenario Outline: work show prints no owner as "-", not as nobody
    Given the people file is missing
    And the work registry has the item "slice-1" owned by nobody with the status "<status>"
    When itos runs "work show slice-1"
    Then itos exits with code 0
    And its output says "status: <status>, owner: -,"
    And its output does not say "owner: nobody"

    Examples:
      | status |
      | doing  |
      | todo   |

  # Bug 34 (found by the code review of 07b0e6d). work done read the
  # registry, waited minutes for the task's checks and CI, then wrote back
  # what it had read, so a registry commit made meanwhile (a take, a queue
  # change, someone's edit pulled in) was reverted by its close commit. work
  # done reads the registry again after the wait and closes the item in what
  # it reads then. Two registry writers at the same instant is not this bug.
  # The change lands when the fake GitHub is first asked for the run.
  @ID-WORK-51 @bug-34
  Scenario: work done keeps a registry change committed while it waited for CI
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the work registry has the item "slice-2" owned by nobody with the status "todo"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And while work done waits for CI, a commit takes "slice-2" for "other"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and the owner "someone"
    And the registry's item "slice-2" has the status "doing" and the owner "other"

  # Slice 89 (decision of q-16): --as named a person in work, status and
  # work take but a new ID in work promote, and work queue --drop chose
  # another action than work queue. They are work promote --id and work
  # queue --remove. Slice 102 replaces the caller-chosen --id with a minted
  # one; the old scenario now proves promotion needs no numbered ID.
  @ID-WORK-52 @slice-89
  Scenario: work promote mints an id, renames an idea and makes it a slice
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work promote p1-thing --kind slice"
    Then itos exits with code 0
    And the registry has no item "p1-thing"
    And the registry's item "slice-1" is a slice whose why starts with "Was p1-thing."

  @ID-WORK-53 @slice-89
  Scenario: work promote --as exits 2, naming --id
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work promote p1-thing --as slice-7 --kind slice"
    Then itos exits with code 2
    And its output says "--id"
    And the registry is unchanged

  @ID-WORK-55 @slice-89
  Scenario: work queue --drop exits 2, naming --remove
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run "work queue slice-9 --top"
    When itos runs "work queue slice-9 --drop"
    Then itos exits with code 2
    And its output says "--remove"
    And the registry's queue names "slice-9"

  # Slice 93 (the coordinator's call, 2026-10-06): closing v6.0.0's ten items
  # took ten pushes and ten full CI runs, over an hour. work done refused
  # while HEAD had a commit no remote had and judged HEAD's run, so each
  # close, itself a registry-only commit, made the next one wait on a push and
  # a run of its own. A commit touching only the work registry changes
  # nothing a run judges (push does not wait for its run, slice 56). work done
  # passes over such commits at HEAD, pushed or not, and judges the run of the
  # newest commit that touches more; one itos push after the last close lands
  # them all. A commit no remote has that touches anything else still refuses
  # (@ID-WORK-16).
  @ID-WORK-56 @slice-93
  Scenario: work done passes over commits no remote has that touch only the work registry
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the work registry has the item "slice-2" owned by nobody with the status "todo"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the clone has a commit no remote has that takes "slice-2" for "other"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and the owner "someone"
    And the fake GitHub was asked for the run of the remote's head
    And the fake GitHub was never asked for the run of the clone's HEAD

  @ID-WORK-57 @slice-93
  Scenario: work done judges the run of the newest commit that touches more than the registry
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the work registry has the item "slice-2" owned by nobody with the status "todo"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And the clone has a commit no remote has that takes "slice-2" for "other"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://ci.example/runs/1"

  # Slice 97 (the user's question, 2026-10-06, p1-item-tags): some 200 items
  # cluster by area (upgrade, plugin, stealth, guard, json, windows) that their
  # ids and titles do not always name. The config declares the tags a
  # registry may use (work.tags), so a typo is refused rather than becoming a
  # tag of its own; an item carries a list of them, given and replaced as its
  # refs are (--tags on work add and work edit, an empty --tags emptying it);
  # work list --tag filters; itos work and itos status show an item's tags
  # beside its id. A registry with no work.tags in its config accepts no tag. The
  # scenarios that add an item first give the registry one, so it lists a phase.
  @ID-WORK-58 @slice-97
  Scenario: work add gives an item tags the config declares
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the config's work.tags is "plugin, stealth"
    When itos runs the command line "work add p1-thing --title 'A thing' --why 'Because.' --tags plugin,stealth"
    Then itos exits with code 0
    And the registry's item "p1-thing" has the tags "plugin, stealth"

  @ID-WORK-59 @slice-97
  Scenario: work add refuses a tag the config does not declare, and adds nothing
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the config's work.tags is "plugin, stealth"
    When itos runs the command line "work add p1-thing --title 'A thing' --why 'Because.' --tags plgin"
    Then itos exits with code 1
    And its output says "plgin"
    And the registry has no item "p1-thing"

  @ID-WORK-60 @slice-97
  Scenario: work check refuses a registry whose item carries an undeclared tag
    Given the config's work.tags is "plugin"
    And the work registry has the item "slice-9" owned by nobody with the status "todo" and the tags "windows"
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "windows"

  @ID-WORK-61 @slice-97
  Scenario: work edit replaces an item's tags, and an empty --tags empties them
    Given the config's work.tags is "plugin, stealth"
    And the work registry has the idea "p1-thing" owned by nobody
    And itos has run the command line "work edit p1-thing --tags plugin"
    When itos runs the command line "work edit p1-thing --tags stealth"
    Then itos exits with code 0
    And the registry's item "p1-thing" has the tags "stealth"
    When itos runs the command line "work edit p1-thing --tags ''"
    Then itos exits with code 0
    And the registry's item "p1-thing" has no tags

  @ID-WORK-62 @slice-97
  Scenario: work list --tag lists only the open items carrying the tag
    Given the config's work.tags is "plugin, stealth"
    And the work registry has the item "slice-1" owned by nobody with the status "todo" and the tags "plugin"
    And the work registry has the item "slice-2" owned by nobody with the status "todo" and the tags "stealth"
    When itos runs "work list --tag plugin"
    Then itos exits with code 0
    And its output says "slice-1"
    And its output does not say "slice-2"

  @ID-WORK-63 @slice-97
  Scenario: itos work shows an item's tags beside its id
    Given the config's work.tags is "plugin"
    And the work registry has the item "slice-1" owned by nobody with the status "todo" and the tags "plugin"
    When itos runs "work --as someone"
    Then itos exits with code 0
    And its output says "slice-1" before "plugin"

  @ID-WORK-64 @slice-97
  Scenario: Without work.tags in the config, work add refuses any tag
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add p1-thing --title 'A thing' --why 'Because.' --tags plugin"
    Then itos exits with code 1
    And its output says "work.tags"

  # Slice 98 (the user's call, 2026-10-07; was p1-work-defer, reported by an
  # agent on 2026-10-05): an item was deferred only by editing the registry's
  # deferred key by hand, which the working rules forbid for status and owner;
  # the coordinator did it twice on 2026-10-06. itos work defer sets deferred
  # to the reason and commits the registry alone, the reason the commit's body,
  # as work drop does; itos work resume takes it off the same way. Two
  # commands rather than a --lift flag: docs/CLI.md rule 18, a flag changes an
  # action and never selects another. A deferred item keeps its status and its
  # place in the queue, as a hand-written deferred key does; work and status
  # already list it apart.
  @ID-WORK-65 @slice-98
  Scenario: work defer sets an item deferred with its reason, and commits the registry alone
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs the command line "work defer p1-thing --why 'Waits for the next major release.'"
    Then itos exits with code 0
    And the registry's item "p1-thing" is deferred with the reason "Waits for the next major release."
    And the registry's item "p1-thing" has the status "todo" and the owner "nobody"
    And the last commit's header is "docs: defer p1-thing"
    And the last commit's body says "Waits for the next major release."
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-66 @slice-98
  Scenario: work resume takes the deferral off, and commits the registry alone
    Given the work registry has the idea "p1-thing" owned by nobody
    And itos has run the command line "work defer p1-thing --why 'Waits for the next major release.'"
    When itos runs "work resume p1-thing"
    Then itos exits with code 0
    And the registry's item "p1-thing" is not deferred
    And the last commit's header is "docs: resume p1-thing"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-67 @slice-98
  Scenario: work defer refuses an item already deferred, naming its reason
    Given the work registry has the idea "p1-thing" owned by nobody
    And itos has run the command line "work defer p1-thing --why 'Waits for the next major release.'"
    When itos runs the command line "work defer p1-thing --why 'Another reason.'"
    Then itos exits with code 1
    And its output says "Waits for the next major release."
    And the registry's item "p1-thing" is deferred with the reason "Waits for the next major release."

  @ID-WORK-68 @slice-98
  Scenario: work resume refuses an item that is not deferred
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work resume p1-thing"
    Then itos exits with code 1
    And its output says "not deferred"

  @ID-WORK-69 @slice-98
  Scenario: work defer refuses a done item
    Given the work registry has the item "slice-9" owned by nobody with the status "done"
    When itos runs the command line "work defer slice-9 --why 'Too late.'"
    Then itos exits with code 1
    And its output says "done"
    And the registry's item "slice-9" is not deferred

  @ID-WORK-70 @slice-98
  Scenario: work defer needs a reason
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work defer p1-thing"
    Then itos exits with code 2
    And its output says "--why"
    And the registry's item "p1-thing" is not deferred

  # Bug 37 (the code review of 07b0e6d, 2026-10-05): work done wrote the
  # close without asking whether the registry would stay sound, so it closed
  # an item that was still todo, or one whose dependency was open, leaving a
  # registry work check refuses; under a stealth config every later write then
  # refused. work done refuses, writing nothing, what work check would refuse
  # after it: an item that is not doing, an item a dependency of which is not
  # done.
  @ID-WORK-71 @bug-37
  Scenario: work done refuses an item that is not doing, and writes nothing
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "todo"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "todo"
    And the registry's item "slice-9" has the status "todo" and the owner "someone"
    And the last commit's header is not "docs: close slice-9"

  @ID-WORK-72 @bug-37
  Scenario: work done refuses an item whose dependency is not done, naming it, and writes nothing
    Given a clone of it, where itos runs
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by "someone" with the status "doing", depending on "slice-8"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "slice-8"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"
    And the last commit's header is not "docs: close slice-9"

  @ID-WORK-73 @bug-39
  Scenario: work check refuses a registry whose alias is inside its own anchor, never crashing
    Given the committed file "tasks/work-items.yaml" holding the lines:
      """
      phases: { 1: null }
      items: &x [*x]
      """
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "alias"

  # Bug 49 (found 2026-10-07, closing bug-36): a push carrying a fix and a
  # registry-only commit after it gets one CI run, for its head. Since slice
  # 93 work done passes over the registry-only head and judges the fix, which
  # has no run of its own, so it waited for a run that never came until
  # ci.watch.timeout. The run of a pushed head that has the judged commit as
  # an ancestor judges it, as for a run a newer push cancelled (bug 41): work
  # done takes the newest such run on the branch, and names it.
  @ID-WORK-74 @bug-49
  Scenario: work done judges a commit pushed with later registry-only commits by the run of the head they were pushed with
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the remote's head touches only the work registry, pushed in one push with the commit before it
    And the fake GitHub has a run of the remote's head alone
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and the owner "someone"
    And its output says "https://ci.example/runs/1"

  @ID-WORK-75 @bug-49
  Scenario: work done refuses when the run of the head that covers the judged commit failed
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's job "ci" fails and its job "platform" succeeds
    And the remote's head touches only the work registry, pushed in one push with the commit before it
    And the fake GitHub has a run of the remote's head alone
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://ci.example/runs/1"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Slice 100 (decision 42 and docs/plans/v7-proof-of-done.md, the user's
  # calls, 2026-10-07): a task's proof is computed from its diff, never
  # written by the model. It ran at work done, over the item's commits; slice
  # 106 moved it into ci run, over the range (ci.feature's @ID-CI-20 to 23),
  # and @ID-WORK-87 says so, so the scenarios that held work done's proof
  # (@ID-WORK-76 to 79, released in v7.0.0-rc.1) are gone. The check's --json
  # is itos-cc's machine contract (its docs/CLI.md, v0.5.0):
  # {"schema":1,"ok":false,"problems":[{"rule","message","fix",…}]}, rules
  # such as mutation.survived, mutation.uncovered, mutation.missing and
  # mutation.stale, the subject in keys like file, line and function; a fake
  # provider answers in that shape (features/proof_test.go), and the run
  # prints each problem's rule and message. Its exit 2 (itos-cc's usage or
  # config error), 3, 70 and 75 are all "cannot run" to itos, which fails the
  # run as any other step that cannot.
  #
  # Slice 101 (docs/plans/v7-proof-of-done.md, the rule): a task's checks are
  # the agent's progress while the task is open. work done runs them, as it
  # does since slice 53, and once it closes the task they leave the ledger,
  # in the close commit itself: the commit touches the ledger file and the
  # registry, and its body says the checks were removed and that the commit
  # before it still holds them. Nothing reruns a done task's checks; git's
  # history keeps them. The task's why, its spec, stays in its entry. A task
  # with no checks closes as before, the ledger untouched. Under a stealth
  # config the ledger is written beside the registry and nothing committed.
  # The checks are committed before the clone, so the remote has them and
  # work done finds nothing unpushed.
  @ID-WORK-80 @slice-101
  Scenario: work done removes the checks of the task it closes from the ledger, in the close commit
    Given the task "T-001" has the committed check "true"
    And the ledger's task "T-001" has the why "The ledger says why."
    And a clone of it, where itos runs
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001"
    Then itos exits with code 0
    And the registry's item "T-001" has the status "done" and the owner "someone"
    And the ledger's task "T-001" has no checks and the why "The ledger says why."
    And the last commit's header is "docs: close T-001"
    And the last commit touches only "tasks/phase-1.yaml" and "tasks/work-items.yaml"

  # Slice 102 (the user's calls, 2026-10-08, q-26 and q-27): itos mints
  # every id it writes, so no spec writer reads the next number off the
  # files. work add --kind slice|task, task add and work promote take no id
  # and mint one through the id counter (features/ids.feature): one past the
  # higher of the counter and the highest the repository holds (the
  # registry's items and the kind's tags for a slice, ledger.id over the
  # ledger and the registry for a task). Each prints the id it minted, and
  # its --json carries it as item.id. An id given is refused as a usage
  # error (exit 2), nothing written: no caller picks a number, so none picks
  # a taken one. An idea keeps the name its author gives (work add <name>),
  # never cited in a footer; work promote mints its new id. This changes
  # what work add, task add and work promote accept: a v7 break, its old
  # scenarios and corpus cases named in Changes. A repository with no remote
  # keeps its counter in itos's git folder, so these scenarios mint locally.
  #
  # Slice 104 (q-26): a bug is a kind of its own, --kind bug minting bug-<n>
  # past the registry's bugs and the @bug tags, which work done treats as a
  # slice: its scenarios are those tagged @bug-<n>. Items named bug-<n> of
  # kind slice, made before this, stay valid.
  @ID-WORK-81 @slice-102
  Scenario: work add --kind slice mints the next slice id, past the registry's highest
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add --kind slice --title 'Do the thing' --why 'Because it is missing.'"
    Then itos exits with code 0
    And its output says "slice-10"
    And the registry's item "slice-10" is a slice titled "Do the thing" with the status "todo"
    And the last commit's header is "docs: add slice-10"

  @ID-WORK-82 @slice-104
  Scenario: work add --kind bug mints a bug, past the highest bug id the registry and the @bug tags hold
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01" tagged "@bug-7"
    And the work registry has the item "bug-5" owned by nobody with the status "done"
    When itos runs the command line "work add --kind bug --title 'It breaks' --why 'Found today.'"
    Then itos exits with code 0
    And the registry's item "bug-8" is a bug titled "It breaks" with the status "todo"

  @ID-WORK-83 @slice-102
  Scenario: work add refuses an id for a slice or a task, and writes nothing
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "work add slice-10 --kind slice --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 2
    And the registry has no item "slice-10"

  @ID-WORK-84 @slice-102
  Scenario: work promote mints the new id and renames the idea everywhere it is named
    Given the work registry has the idea "p1-thing" owned by nobody
    And the work registry has the item "slice-9" owned by nobody with the status "todo", depending on "p1-thing"
    When itos runs "work promote p1-thing --kind slice"
    Then itos exits with code 0
    And its output says "slice-10"
    And the registry has no item "p1-thing"
    And the registry's item "slice-10" is a slice whose why starts with "Was p1-thing."
    And the registry's item "slice-9" depends on "slice-10"

  @ID-WORK-85 @slice-102
  Scenario: work promote refuses an id, and writes nothing
    Given the work registry has the idea "p1-thing" owned by nobody
    When itos runs "work promote p1-thing --id slice-7 --kind slice"
    Then itos exits with code 2
    And the registry has no item "slice-7"

  @ID-WORK-88 @slice-102
  Scenario: work add --kind task mints the ledger's next task id
    Given the work registry has the item "T-001" owned by nobody with the status "done"
    When itos runs the command line "work add --kind task --phase 1 --title 'Do the thing' --why 'Because.'"
    Then itos exits with code 0
    And its output says "T-002"
    And the registry's item "T-002" is a task titled "Do the thing" with the status "todo"

  @ID-WORK-86 @slice-104
  Scenario: work done refuses a bug one of whose scenarios is still @wip, as it refuses a slice
    Given the work registry has the bug "bug-8" owned by "someone" with the status "doing"
    And a feature file with the scenario "@ID-A-01" tagged "@bug-8 @wip"
    When itos runs "work done bug-8"
    Then itos exits with code 1
    And its output says "@ID-A-01"

  # Bug 129 (issue #111): work done read an item's scenarios by their tag
  # lines alone, so an outline tagged with the item counted as live while an
  # Examples block under it was still tagged @wip, rows the runner skips
  # (godog gives a row its block's tags, and every run excludes @wip), and
  # the close went through, exit 0, without a word. A part of the item's
  # scenario still @wip is a scenario not yet live: work done refuses it as
  # it refuses a @wip scenario (exit 1, the rule work-done-wip, nothing
  # written), naming the scenario and the block, by its name ("Examples:
  # <name>"), or by its line in the file when it has none, so the person
  # finds the rows left. The fixture is features/tagged.feature, committed
  # at HEAD under the scratch config's scenario kind, as the step for a
  # tagged scenario writes it: the outline @ID-A-01 tagged @slice-9, whose
  # first block, "Examples: live", has no tag and one row, and whose second
  # has the name and the tag line the step gives and one row of its own.
  # No remote and no ci.watch: today the close is made, exit 0, so the
  # exit code fails first. Naming the block is new text.
  @ID-WORK-149 @bug-129
  Scenario: work done refuses a slice one of whose outlines still has an Examples block tagged @wip, naming it
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And a feature file with the outline "@ID-A-01" tagged "@slice-9", one Examples block live and the block "waiting on the harness" tagged "@wip"
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "@ID-A-01"
    And its output says "waiting on the harness"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Bug 129: an outline none of whose Examples blocks is @wip closes as
  # today, and the close names no block. Its second block carries a tag
  # other than @wip, so a fix that took any tagged Examples block for one
  # still @wip fails here. The same fixture as @ID-WORK-149.
  @ID-WORK-150 @bug-129 @guard
  Scenario: work done closes a slice whose outline has a tagged Examples block that is not @wip
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And a feature file with the outline "@ID-A-01" tagged "@slice-9", one Examples block live and the block "slow rows" tagged "@slow"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output does not say "slow rows"
    And the registry's item "slice-9" has the status "done" and the owner "someone"
    And the last commit's header is "docs: close slice-9"

  # Slice 106 (issue #28, the comment above @ID-CI-20 in ci.feature): the
  # code proof moved into CI, so work done no longer runs proof.code.check:
  # the green run it requires judged the proof of every commit in its range.
  # This makes @ID-WORK-76, @ID-WORK-77 and @ID-WORK-79 (slice 100, released
  # in v7.0.0-rc.1) untrue, and @ID-WORK-78 moot: the feat removes them,
  # naming each in Changes with a BREAKING-CHANGE footer. It also settles
  # issue #26, an item's {base} taking in other items' commits, which only
  # work done's own range had.
  @ID-WORK-87 @slice-106
  Scenario: work done does not run the code proof, which the item's green CI run already judged
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the remote has the item "slice-9"'s commit touching "src/app.go"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the provider was not run

  # Slice 118 (p1-work-edit-several-ids, 2026-10-10): grooming a backlog is
  # one change applied to many items, and one command per item made one
  # commit each, every one running the hooks (tagging about 110 items would
  # have been 110 commits). work edit, drop, defer, resume and queue take
  # several ids: the same change for each, all checked before anything is
  # written, refused whole (exit 1, naming every id refused and why) if any
  # one would be, and committed once, the registry alone. One id behaves as
  # today. The header names the ids, "docs: drop p1-a, p1-b", when that fits
  # the header's 100 characters, else "docs: drop 12 items"; the body lists
  # every id. A change that is the item's own is refused with several ids,
  # exit 2: work edit --title. work queue places the ids in the order given:
  # --top puts them first, --before and --after put them together at that
  # place, --remove takes each out. --json gives "items":[…] for each.
  @ID-WORK-89 @slice-118
  Scenario: work edit with several ids gives each the same tags, in one commit
    Given the config's work.tags is "plugin, stealth"
    And the work registry has the idea "p1-a" owned by nobody
    And the work registry has the idea "p1-b" owned by nobody
    When itos runs the command line "work edit p1-a p1-b --tags plugin"
    Then itos exits with code 0
    And the registry's item "p1-a" has the tags "plugin"
    And the registry's item "p1-b" has the tags "plugin"
    And the last commit's header is "docs: edit p1-a, p1-b"
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-90 @slice-118
  Scenario: One id that would be refused refuses them all, naming it, and nothing is written
    Given the config's work.tags is "plugin, stealth"
    And the work registry has the idea "p1-a" owned by nobody
    When itos runs the command line "work edit p1-a p1-nope --tags plugin"
    Then itos exits with code 1
    And its output says "p1-nope"
    And the registry's item "p1-a" has no tags

  @ID-WORK-91 @slice-118
  Scenario: work drop with several ids drops each with the one reason, in one commit
    Given the work registry has the idea "p1-a" owned by nobody
    And the work registry has the idea "p1-b" owned by nobody
    When itos runs the command line "work drop p1-a p1-b --why 'Merged into slice 9.'"
    Then itos exits with code 0
    And the registry's item "p1-a" has the status "dropped" and no why
    And the registry's item "p1-b" has the status "dropped" and no why
    And the last commit's header is "docs: drop p1-a, p1-b"
    And the last commit's body says "Merged into slice 9."

  @ID-WORK-92 @slice-118
  Scenario: work queue with several ids puts them first in the order given
    Given the work registry has the item "slice-7" owned by nobody with the status "todo"
    And the work registry has the item "slice-8" owned by nobody with the status "todo"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the registry's queue names "slice-7"
    When itos runs "work queue slice-9 slice-8 --top"
    Then itos exits with code 0
    And the registry's queue is "slice-9, slice-8, slice-7"

  @ID-WORK-93 @slice-118
  Scenario: work edit --title with several ids exits 2, naming --title
    Given the work registry has the idea "p1-a" owned by nobody
    And the work registry has the idea "p1-b" owned by nobody
    When itos runs the command line "work edit p1-a p1-b --title 'One title'"
    Then itos exits with code 2
    And its output says "--title"

  # Issue #42 (2026-10-10, from go-template-itos; v8, the user's call
  # 2026-10-10): an agent took an item, found it blocked, and the person
  # chose to defer it, but work defer takes only todo and nothing moved
  # doing back, while the rules forbid editing a status by hand; itos guide
  # repair already tells a coordinator to give a reverted item back.
  # itos work give-back <id> --why '…' undoes a take: the status goes back to
  # todo, the owner stays (an owner is who the item is for, in progress or
  # not), the reason is the commit's body, as work drop and work defer write
  # theirs, and the registry is committed alone. It refuses an item that is
  # not in progress and one someone else owns, as work take refuses it. The
  # coordinator's call (2026-10-11): a command of its own rather than work
  # defer taking doing, so a given-back item can be taken again as well as
  # deferred, and repair names the command.
  @ID-WORK-94 @slice-122
  Scenario: work give-back puts an item in progress back to todo, keeping its owner, and commits the reason
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    When itos runs the command line "work give-back slice-9 --why 'Blocked on another project.' --as someone"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "todo" and the owner "someone"
    And the last commit's header is "docs: give back slice-9"
    And the last commit's body says "Blocked on another project."
    And the last commit touches only "tasks/work-items.yaml"

  @ID-WORK-95 @slice-122
  Scenario: work give-back refuses an item that is not in progress, naming its status
    Given the work registry has the item "slice-9" owned by "someone" with the status "todo"
    When itos runs the command line "work give-back slice-9 --why 'Blocked.' --as someone"
    Then itos exits with code 1
    And its output says "todo"

  @ID-WORK-96 @slice-122
  Scenario: work give-back refuses an item someone else has in progress
    Given the work registry has the item "slice-9" owned by "another" with the status "doing"
    When itos runs the command line "work give-back slice-9 --why 'Blocked.' --as someone"
    Then itos exits with code 1
    And its output says "another"
    And the registry's item "slice-9" has the status "doing" and the owner "another"

  @ID-WORK-97 @slice-122
  Scenario: An item given back can be deferred
    Given the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And itos has run the command line "work give-back slice-9 --why 'Blocked on another project.' --as someone"
    When itos runs the command line "work defer slice-9 --why 'Waits for the other project.'"
    Then itos exits with code 0
    And the registry's item "slice-9" is deferred with the reason "Waits for the other project."
    And the registry's item "slice-9" has the status "todo" and the owner "someone"

  # Slice 127 (p1-work-done-passes-own-closes, the user's call, 2026-10-11):
  # closing T-153 to T-156 and slice-126 took five pushes and five CI runs,
  # since slice 93's pass-over covers only registry-only commits and a task's
  # close also touches its ledger (slice 101), and T-153 waited on a red run
  # that came after its own green one. work done now judges the item's own
  # commits, the ones that name it (an Item:, Task: or Scenarios: footer, as
  # itos work show lists them): each must be pushed, and some run whose head
  # is the item's newest commit or has it as an ancestor must have passed,
  # since a later run's range (itos ci range, from the last green run) leaves
  # those commits out, so a later red is not the item's. Commits no remote
  # has that are not the item's are passed over, so closes chain and one push
  # lands them. An item with no commit of its own is judged as before, by the
  # newest commit at HEAD that touches more than the registry (@ID-WORK-16,
  # 17, 56, 57). The covering-run lookup is bug 49's (@ID-WORK-74), widened
  # from the newest covering run to any covering run that passed.
  @ID-WORK-98 @slice-127
  Scenario: work done passes over a commit no remote has that is not the item's own
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the remote has the item "slice-9"'s commit touching "README.md"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "docs: plan the next release" touching "docs/plan.md"
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  @ID-WORK-99 @slice-127
  Scenario: Two items close one after the other with no push between them
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the work registry has the item "T-001" owned by "someone" with the status "doing"
    And the remote has the item "slice-9"'s commit touching "README.md"
    And the remote has the item "T-001"'s commit touching "NOTES.md"
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "work done T-001"
    Then itos exits with code 0
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And the registry's item "T-001" has the status "done" and the owner "someone"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  @ID-WORK-100 @slice-127
  Scenario: work done closes an item whose own run passed though a later run failed
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the remote has the item "slice-9"'s commit touching "README.md"
    And the remote has gained the commit "feat: something else" touching "other.txt"
    And the fake GitHub's run of the item "slice-9"'s commit, "https://ci.example/runs/1", succeeded
    And the fake GitHub's run of the remote's head, "https://ci.example/runs/2", failed
    When itos runs "work done slice-9"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/1"
    And the registry's item "slice-9" has the status "done" and the owner "someone"

  @ID-WORK-101 @slice-127
  Scenario: work done refuses while the item's own commit is not pushed, naming itos push
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the clone has the item "slice-9"'s commit touching "README.md", not pushed
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "itos push"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  @ID-WORK-102 @slice-127
  Scenario: work done refuses when every run covering the item's own commit failed
    Given a clone of it, where itos runs
    And the work registry has the item "slice-9" owned by "someone" with the status "doing"
    And the remote has the item "slice-9"'s commit touching "README.md"
    And the fake GitHub's run of the item "slice-9"'s commit, "https://ci.example/runs/1", failed
    When itos runs "work done slice-9"
    Then itos exits with code 1
    And its output says "https://ci.example/runs/1"
    And the registry's item "slice-9" has the status "doing" and the owner "someone"

  # Bug 105: see config.feature's comment above @ID-CONFIG-59.
  @ID-WORK-103 @bug-105
  Scenario: work promote renames the idea in the questions that name it
    Given the work registry has the idea "p1-thing" owned by nobody
    And itos has already run "decision add Which-way --item p1-thing"
    When itos runs "work promote p1-thing --kind slice"
    Then itos exits with code 0
    When itos runs "decision show q-1"
    Then its output says "slice-1"

  # Slice 138 (#54, q-63): task promotion creates a usable ledger task in
  # the same commit as the rename and reference rewrites. --type and at
  # least one --check are required; --timeout follows its own check.
  # Group/title/why come from the idea, with --title/--why overrides.
  # The final why belongs in the ledger, not a new registry copy.
  # Task-only flags on slice promotion are errors. Preflight never runs
  # check commands and rejects invalid input before reserving an ID.
  # A later hook failure may leave a reserved-number gap (ADR-0045).
  #
  # The fixture below commits a valid idea in phase 1, declares any people
  # and tags it uses, supplies completed dependencies, and records its
  # initial metadata. It also supplies a queued position, a dependent item
  # and a linked question so every reference rewrite is observable.
  @ID-WORK-104 @slice-138
  Scenario: task promotion creates the ledger entry and preserves the idea's metadata and references
    Given a committed task-promotion fixture for the idea "p1-archive"
    When itos runs "work promote p1-archive --kind task --type build --check 'echo archive > promotion-check-marker' --json"
    Then itos exits with code 0
    And its JSON's "item.kind" is "task"
    And the promoted ledger task has type "build" and the idea's title and why
    And the promoted item preserves the idea's phase owner dependencies refs tags and queue position
    And every dependency and question naming "p1-archive" names the promoted task instead
    And the promoted registry item has no why copy
    And a commit message naming the promoted task passes its configured ledger-footer check
    And the file "promotion-check-marker" does not exist

  @ID-WORK-105 @slice-138
  Scenario: task promotion accepts a current specification and ordered checks with per-check timeouts
    Given a committed task-promotion fixture for the idea "p1-archive"
    When itos runs "work promote p1-archive --kind task --type build --title 'Build reviewed archives' --why 'Package the reviewed inputs.' --check 'echo first' --timeout 9 --check 'echo second' --timeout 11 --json"
    Then itos exits with code 0
    And the promoted ledger task has title "Build reviewed archives" and why "Package the reviewed inputs."
    And the promoted ledger task's checks equal:
      """
      [{"run":"echo first","timeout":9},{"run":"echo second","timeout":11}]
      """
    And the promotion commit contains the ledger registry and rewritten questions together

  # Task-only slice flags and malformed timeout cases are regression guards
  # inside the outline; their existing rejection must remain unchanged.
  @ID-WORK-106 @slice-138
  Scenario Outline: invalid promotion arguments change neither the repository nor the ID counter
    Given a committed task-promotion fixture for the idea "p1-archive"
    And the promotion state is saved for comparison
    When itos runs "work promote p1-archive --kind <kind> <arguments>"
    Then itos exits with code 2
    And the promotion files index HEAD and ID counter match the saved state

    Examples:
      | kind  | arguments                                  |
      | task  |                                            |
      | task  | --type build                               |
      | task  | --check true                               |
      | task  | --type build --check ''                     |
      | task  | --type build --timeout 9 --check true       |
      | task  | --type build --check true --timeout 0       |
      | task  | --type build --check true --timeout invalid |
      | task  | --type build --check true --id T-999        |
      | slice | --type build --check true                  |

  # Each defect is deliberately committed before the snapshot, so refusal
  # must preserve that input rather than silently repair the fixture.
  @ID-WORK-107 @slice-138
  Scenario Outline: semantic promotion failures are reported before reserving a task ID
    Given a committed task-promotion fixture for the idea "p1-archive"
    And the promotion fixture has "<problem>"
    And the promotion state is saved for comparison
    When itos runs "work promote p1-archive --kind task --type <type> --check true"
    Then itos exits with code 1
    And the promotion files index HEAD and ID counter match the saved state

    Examples:
      | problem                                  | type        |
      | an undeclared commit type                | undeclared  |
      | an idea phase outside the ledger pattern | build       |
      | an unknown dependency                    | build       |
      | an unreadable linked questions file      | build       |
      | an unstaged edit to the target ledger    | build       |

  # The hook records that it ran, then refuses the final promotion commit.
  # This must reach Git; a parser refusal cannot satisfy the scenario.
  @ID-WORK-108 @slice-138
  Scenario: a rejected promotion commit restores all participating files and leaves unrelated staging alone
    Given a committed task-promotion fixture for the idea "p1-archive"
    And a configured hook that records and rejects promotion commits
    And an unrelated change to "NOTES.md" is staged
    And the promotion state is saved for comparison
    When itos runs "work promote p1-archive --kind task --type build --check true"
    Then itos exits with code 1
    And the rejecting promotion hook ran
    And the promotion files index and HEAD match the saved state

  # The stealth setup relocates config, ledger, registry and questions
  # together, so the default worktree ledger cannot satisfy the assertions.
  @ID-WORK-109 @slice-138
  Scenario: promotion under a stealth config writes the complete task and references without a commit
    Given a committed task-promotion fixture for the idea "p1-archive"
    And the promotion fixture uses a stealth config
    And the promotion state is saved for comparison
    When itos runs "work promote p1-archive --kind task --type build --check true --json"
    Then itos exits with code 0
    And the configured stealth ledger contains the promoted task
    And every dependency and question naming "p1-archive" names the promoted task instead
    And its JSON's "commit" is null
    And HEAD and the working tree are unchanged by the promotion

  # Guard: ordinary slice promotion already works and must remain usable
  # without task metadata. This case is expected to pass before the slice.
  @ID-WORK-110 @slice-138 @guard
  Scenario: slice promotion continues without task fields and creates no ledger entry
    Given a committed task-promotion fixture for the idea "p1-archive"
    When itos runs "work promote p1-archive --kind slice --json"
    Then itos exits with code 0
    And its JSON's "item.kind" is "slice"
    And the promoted slice has no ledger entry

  # Bug 111 (#67): changing only the verb makes a removal recognizable in
  # git log without reading its body. --remove is the current spelling;
  # the report's --drop was renamed in v6. Keep historical queue headers
  # readable. General discovery of multi-item commits belongs separately
  # to p1-work-show-several-ids, not this header correction.
  # Decided after review (2026-10-23): work done's own queue-removal commit,
  # the second commit it makes until slice 142 folds it into the close, says
  # "docs: dequeue <id>" too; internal/slices/work/done_test.go pins it (it
  # pins "docs: queue slice-2" today). work queue's help and its verb list
  # name "docs: dequeue <id>" and "docs: dequeue <n> items" for --remove.
  # Insertions and moves keep "docs: queue": write_test.go and @ID-WORK-30
  # already pin that, so no guard scenario repeats it.
  @ID-WORK-111 @bug-111
  Scenario: explicit queue removal records a dequeue header that work show recognizes
    Given a committed queue fixture with the items "slice-1,slice-2" queued in that order
    When itos runs "work queue slice-1 --remove --json"
    Then itos exits with code 0
    And the last commit's header is "docs: dequeue slice-1"
    And the queue is "slice-2"
    And its JSON's "queue" is ["slice-2"]
    And the last commit changes only the configured registry
    Given the last commit SHA is saved as "dequeue"
    When itos runs "work show slice-1 --json"
    Then itos exits with code 0
    And the shown history includes the saved "dequeue" commit

  # A historical two-commit close can leave a completed item queued if its
  # second commit fails. Explicit removal must still repair that state.
  # Do not promise a second queue commit from work done: slice 142 removes
  # that failure boundary by including dequeue in the close itself.
  @ID-WORK-112 @bug-111
  Scenario: explicit dequeue repairs a previously completed item still left in the queue
    Given a committed queue fixture with the completed task "T-001" still queued
    When itos runs "work queue T-001 --remove --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And the last commit's header is "docs: dequeue T-001"
    And the registry's item "T-001" has the status "done"
    And the queue does not contain "T-001"
    And no ledger data was changed

  # Use the same 100-unit header limit and counted fallback as the other
  # multi-item writers. The fixture supplies valid distinct item IDs and
  # retains their full list in the commit body when the header is counted.
  @ID-WORK-113 @bug-111
  Scenario Outline: multi-item removal uses the dequeue verb with the normal header length convention
    Given a committed queue fixture for "<items>"
    When all those items are removed by one work-queue command
    Then itos exits with code 0
    And exactly one commit was added by the command
    And the last commit's header is "<header>"
    And the commit body names every removed item
    And none of those items remains queued

    Examples:
      | items                                | header                           |
      | slice-1 and slice-2                   | docs: dequeue slice-1, slice-2   |
      | slice-1 to slice-12                   | docs: dequeue 12 items          |

  @ID-WORK-115 @bug-111 @guard
  Scenario: an old queue-removal header remains discoverable and repeated removal is a no-op
    Given a committed queue fixture with "slice-1" already removed by a historical "docs: queue slice-1" commit
    And that historical removal commit SHA is saved as "old-removal"
    And the repository state is saved for comparison
    When itos runs "work queue slice-1 --remove"
    Then itos exits with code 0
    And the repository files index and HEAD match the saved state
    When itos runs "work show slice-1 --json"
    Then itos exits with code 0
    And the shown history includes the saved "old-removal" commit

  # Slice 142 (#73, structural delivery): closure is one local transaction,
  # after all existing landing gates. No stored receipt, evidence-reuse rule
  # or new success claim is introduced. The task fixture owns T-001, has
  # meaningful passing checks, and configures a fake exact-revision green
  # CI for published implementation commits. It records all command-created
  # commits separately from fixture setup. Queue removal belongs to this
  # close commit and does not create a second dequeue commit.
  @ID-WORK-116 @slice-142 @wip
  Scenario: task closure records status check removal and dequeue in one commit
    Given a committed landing fixture for the queued task "T-001"
    And an unrelated change to "NOTES.md" is staged
    And the completion state is saved for comparison
    When itos runs "work done T-001 --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And the last commit's header is "docs: close T-001"
    And that commit marks "T-001" done removes its current checks and removes it from the queue
    And the task's ledger why is preserved
    And the close body names the validated revision successful CI and pre-close check-definition revision
    And its JSON's "commit" identifies that single close commit
    And its JSON's "queue_commit" identifies that same close commit
    And the unrelated staging matches the saved completion state

  @ID-WORK-117 @slice-142 @wip
  Scenario Outline: one closure transaction handles slices tasks without checks and unqueued items
    Given a committed landing fixture for "<item>" with "<state>"
    And the completion state is saved for comparison
    When itos runs "work done <item> --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And the registry's item "<item>" has the status "done"
    And "<item>" is not queued
    And no unrelated ledger entry was changed
    And any emitted queue_commit identifies the same commit as commit

    Examples:
      | item    | state                          |
      | slice-1 | queued implemented slice       |
      | T-001   | queued task without checks     |
      | T-001   | unqueued task with checks      |

  # All other inputs remain valid. CI failure uses a fake provider with a
  # completed red exact-revision run; no network timeout can satisfy it.
  @ID-WORK-118 @slice-142 @guard @wip
  Scenario Outline: a failed landing gate leaves the entire closure unapplied
    Given a committed landing fixture for the queued task "T-001"
    And only its landing condition "<condition>" is invalid
    And the completion state is saved for comparison
    When itos runs "work done T-001"
    Then itos refuses completion naming "<diagnostic>"
    And the registry ledger queue index and HEAD match the saved completion state

    Examples:
      | condition                         | diagnostic |
      | a tagged scenario remains wip     | wip        |
      | its implementation is unpublished | pushed     |
      | its acceptance command fails      | T-001      |
      | its configured CI run failed      | CI         |

  # The hook records invocation before failing, distinguishing transaction
  # rollback from refusal before the close was attempted.
  @ID-WORK-119 @slice-142 @wip
  Scenario: a rejected close commit restores status checks queue and unrelated staging
    Given a committed landing fixture for the queued task "T-001"
    And a recording commit hook rejects its closure
    And an unrelated change to "NOTES.md" is staged
    And the completion state is saved for comparison
    When itos runs "work done T-001"
    Then itos refuses completion naming the rejecting hook
    And the closure hook ran exactly once
    And the registry ledger queue index and HEAD match the saved completion state

  # The deterministic provider callback commits a separate allowed change
  # while work done waits, then returns the original implementation's green
  # result. Snapshot comparisons start at that callback commit, so preserving
  # a legitimate concurrent edit is distinct from rolling it back.
  @ID-WORK-120 @slice-142 @wip
  Scenario Outline: closure refreshes participating data after waiting instead of overwriting concurrent changes
    Given a committed landing fixture for the queued task "T-001"
    And while CI is awaited a committed change will "<change>"
    When itos runs "work done T-001"
    Then completion has outcome "<outcome>"
    And the committed concurrent change remains intact
    And a successful close contains current validation and current data
    And a refused close changes no files index or HEAD after the concurrent commit

    Examples:
      | change                                  | outcome |
      | edit an unrelated item's title          | success |
      | add a different item to the queue       | success |
      | transfer T-001 to a different owner     | success |
      | replace T-001's checks with a failing check | refusal |

  # The configured stealth ledger and registry are outside the tracked
  # working tree. Their current checks still run; no success is fabricated.
  @ID-WORK-121 @slice-142 @wip
  Scenario: stealth closure applies the same complete data change without a commit
    Given a committed landing fixture for the queued task "T-001" using relocated stealth data
    And the completion state is saved for comparison
    When itos runs "work done T-001 --json"
    Then itos exits with code 0
    And the configured task is done with its checks removed and is not queued
    And the task's ledger why is preserved
    And its JSON's "commit" is null
    And any emitted queue_commit is null
    And the working tree index and HEAD match the saved completion state

  # Slice 143 (#70), after slice 142: work done <id>... is all-or-nothing.
  # Parse and validate every target before any write; perform landing gates
  # in input order and preserve their existing failure/temporary exit codes.
  # No dependency or absorbed idea is implicitly included. Re-read and
  # revalidate all targets after waits, as in slice 142. Shared task checks
  # use only the task runner's existing command/timeout identity rules.
  # New multi-item success JSON has schema, ok, items in distinct input
  # order, changed IDs, queue, one commit and results in input order.
  # Each result has id and changed; a validated changing item additionally
  # has the existing ci/run/superseded fields. Already-done items get no
  # fabricated fresh validation result. Single-item JSON stays unchanged.
  @ID-WORK-122 @slice-143 @wip
  Scenario: a mixed batch closes every validated item with one shared commit
    Given a committed batch-landing fixture for the queued items "T-001,slice-1,T-002"
    And an unrelated item is also queued
    And the completion state is saved for comparison
    When itos runs "work done T-001 slice-1 T-002 --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And the last commit's header is "docs: close T-001, slice-1, T-002"
    And all three items are done with only their current task checks removed
    And the unrelated queued item and ledger entries are unchanged
    And its JSON lists items and results in the order "T-001,slice-1,T-002"
    And its JSON's "changed" is ["T-001","slice-1","T-002"]
    And its JSON's "commit" identifies the shared close commit
    And every changed item's result identifies its own successful validation
    And the close body records each item's validation and pre-close check-definition revision

  # Each defect belongs only to T-002; T-001 is otherwise closable. The
  # assertion must reach the named gate rather than fail on an invalid
  # fixture or on parsing the newly supported second positional argument.
  @ID-WORK-123 @slice-143 @wip
  Scenario Outline: one invalid member prevents every batch mutation
    Given a committed batch-landing fixture for the queued items "T-001,T-002"
    And only "T-002" has completion defect "<defect>"
    And the completion state is saved for comparison
    When itos runs "work done T-001 T-002 --json"
    Then the completion batch refuses "T-002" for "<defect>"
    And the registry ledger queue index and HEAD match the saved completion state

    Examples:
      | defect                     |
      | unknown item               |
      | dropped item               |
      | not started and still todo |
      | live work still marked wip |
      | unpublished implementation |
      | failing acceptance check   |

  @ID-WORK-124 @slice-143 @wip
  Scenario: a sibling's green CI cannot close an item with its own red required run
    Given a committed batch-landing fixture for the queued items "T-001,T-002"
    And their own implementation revisions have distinct configured CI runs
    And only T-001's required run succeeds while T-002's required run fails
    And the completion state is saved for comparison
    When itos runs "work done T-001 T-002 --json"
    Then the completion batch refuses "T-002" for its failed CI
    And the registry ledger queue index and HEAD match the saved completion state

  @ID-WORK-125 @slice-143 @wip
  Scenario: already-done members remain unchanged and a repeated completed batch adds no commit
    Given a committed batch-landing fixture where "T-001" is already done and "T-002" is closable
    When itos runs "work done T-001 T-002 --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And its JSON's "changed" is ["T-002"]
    And the result for "T-001" is unchanged with no new validation claim
    Given the completion state is saved for comparison
    When itos runs "work done T-001 T-002 --json"
    Then itos exits with code 0
    And its JSON's "changed" is []
    And its JSON's "commit" is null
    And the registry ledger queue index and HEAD match the saved completion state

  # The sibling parser rejects duplicates; its comment saying each once
  # is not a promise to silently deduplicate the supplied arguments.
  @ID-WORK-126 @slice-143 @wip
  Scenario: a repeated ID is a usage refusal before checking or closing any item
    Given a committed batch-landing fixture for the queued items "T-001,T-002"
    And the completion state is saved for comparison
    When itos runs "work done T-001 T-002 T-001"
    Then itos exits with code 2
    And its output says "T-001"
    And no completion check or CI provider ran
    And the registry ledger queue index and HEAD match the saved completion state

  # Place the two tasks in distinct configured ledger files, so a rollback
  # that repairs only one task file cannot satisfy the snapshot comparison.
  @ID-WORK-127 @slice-143 @wip
  Scenario: a hook-rejected batch restores every ledger file and caller staging
    Given a committed batch-landing fixture for the queued items "T-001,T-002" in separate ledger files
    And a recording commit hook rejects the combined closure
    And an unrelated change to "NOTES.md" is staged
    And the completion state is saved for comparison
    When itos runs "work done T-001 T-002"
    Then itos refuses completion naming the rejecting hook
    And the closure hook ran exactly once
    And the registry all ledgers queue index and HEAD match the saved completion state

  @ID-WORK-128 @slice-143 @wip
  Scenario: a stealth batch writes every configured closure together without making a commit
    Given a committed batch-landing fixture for the queued items "T-001,slice-1,T-002" using relocated stealth data
    And the completion state is saved for comparison
    When itos runs "work done T-001 slice-1 T-002 --json"
    Then itos exits with code 0
    And all three items are done with only their current task checks removed
    And none of the three items remains queued
    And its JSON's "changed" is ["T-001","slice-1","T-002"]
    And its JSON's "commit" is null
    And the working tree index and HEAD match the saved completion state

  # Slice 149 (#62), after batch completion: work done <idea>... --by
  # <completed-item> records explicit absorption, not abandonment. Sources
  # must be ideas; the target must be a distinct done task/slice/bug. No
  # inference from matching prose and no automatic closure of other ideas.
  # All existing registry invariants still apply. Preserve source owners
  # and dependencies; mark done, remove source why and dequeue atomically.
  # Add absorbed_by=<target> to each source. Do not rerun the completed
  # target's checks/CI or claim a fresh implementation proof for the idea.
  # Ordinary completion without --by retains its current implementation
  # gates, and --by cannot close unfinished concrete work.
  #
  # Registry validation accepts absorbed_by only on a done idea pointing
  # to an existing done concrete item. Same target on a repeated close is
  # unchanged; a different/absent historical relation cannot be reassigned
  # through this command. Single-item JSON uses item/commit; batches retain
  # ordered items/changed/results and one commit. Absorption results carry
  # id, changed and absorbed_by, without fabricated ci/run fields.
  # Work show prints the source relationship and lists the target's
  # absorbed_ideas as registry-ordered IDs in JSON, omitted when none.
  #
  # Fixtures commit valid setup before installing actual hooks. Deliberate
  # future-format/corrupt registry Givens write data directly, not through
  # the unimplemented command. A recording fake CI and a completed target's
  # legacy retained marker check detect unintended replay. Hook rejection
  # cases reach a real hook and snapshot the full registry/index/HEAD.
  @ID-WORK-129 @slice-149 @wip
  Scenario: an explicit completed-work link closes an absorbed idea without pretending it implemented a new slice
    Given a committed absorption fixture with queued idea "p1-extra" and completed task "T-001"
    And the completion state is saved for comparison
    When itos runs "work done p1-extra"
    Then itos refuses completion naming "idea"
    And the registry ledger queue index and HEAD match the saved completion state
    When itos runs "work done p1-extra --by T-001 --json"
    Then itos exits with code 0
    And exactly one commit was added after the saved state
    And the idea "p1-extra" is done with absorbed_by "T-001"
    And its old why is removed and it is not queued
    And its owner and dependencies remain unchanged
    And its JSON's "item.absorbed_by" is "T-001"
    And its JSON's "commit" identifies that close commit
    And the close body names the absorbed idea and completed target
    And no completed-target acceptance check or CI lookup ran
    And no ledger data was changed

  @ID-WORK-130 @slice-149 @wip
  Scenario: one explicit batch closes selected absorbed ideas and frees their dependents
    Given a committed absorption fixture with queued ideas "p1-first,p1-second,p1-unselected" and completed task "T-001"
    And another todo item depends on "p1-first"
    And the completion state is saved for comparison
    When itos runs "work done p1-second p1-first --by T-001 --json"
    Then itos exits with code 0
    And exactly one commit was added by the command
    And both selected ideas are done with absorbed_by "T-001" and are not queued
    And its JSON lists items and results in the order "p1-second,p1-first"
    And its JSON's "changed" is ["p1-second","p1-first"]
    And no absorption result claims a new successful CI run
    And "p1-unselected" and the dependent item's status remain unchanged
    And the dependent item is no longer blocked by "p1-first"

  # The fixture supplies existing objects in each named state. Unknown
  # target is deliberately absent. Every other input is valid, so a
  # diagnostic must identify the actual invalid source/target condition.
  @ID-WORK-131 @slice-149 @wip
  Scenario Outline: invalid absorption never partially closes a batch or bypasses concrete work gates
    Given a committed absorption fixture for "<sources>" and target "<target>" with "<condition>"
    And the completion state is saved for comparison
    When itos runs "work done <sources> --by <target> --json"
    Then itos refuses absorption for "<condition>"
    And the registry ledger queue index and HEAD match the saved completion state

    Examples:
      | sources          | target      | condition                    |
      | p1-a p1-b        | T-999       | unknown target               |
      | p1-a p1-b        | T-002       | target still doing           |
      | p1-a p1-b        | T-003       | target dropped               |
      | p1-a             | p1-previous | target is another idea       |
      | p1-a             | p1-a        | source is its own target     |
      | slice-1          | T-001       | source is a concrete slice   |
      | T-002            | T-001       | source is a concrete task    |
      | p1-a p1-deferred | T-001       | a source is deferred         |
      | p1-a p1-dropped  | T-001       | a source is dropped          |

  @ID-WORK-132 @slice-149 @wip
  Scenario: inspection exposes absorption in both directions without rewriting historical reasons
    Given a committed absorption fixture with ideas "p1-first,p1-second" absorbed by completed task "T-001"
    And the repository files index and HEAD are saved for comparison
    When itos runs "work show T-001 --json"
    Then itos exits with code 0
    And its JSON's "absorbed_ideas" is ["p1-first","p1-second"]
    When itos runs "work show p1-first"
    Then itos exits with code 0
    And its output identifies "T-001" as the work that absorbed this idea
    When itos runs "work show p1-first --json"
    Then itos exits with code 0
    And its JSON's "item.absorbed_by" is "T-001"
    And the repository files index and HEAD match the saved state

  @ID-WORK-133 @slice-149 @wip
  Scenario: repeated absorption is a no-op and cannot silently rewrite who completed the idea
    Given a committed absorption fixture with idea "p1-extra" absorbed by completed task "T-001"
    And another concrete task "T-002" is done
    And the completion state is saved for comparison
    When itos runs "work done p1-extra --by T-001 --json"
    Then itos exits with code 0
    And its JSON's "commit" is null
    And the registry ledger queue index and HEAD match the saved completion state
    When itos runs "work done p1-extra --by T-002 --json"
    Then itos refuses absorption for changing its existing target
    And the registry ledger queue index and HEAD match the saved completion state

  @ID-WORK-134 @slice-149 @wip
  Scenario Outline: registry validation rejects dangling or semantically invalid absorption relationships
    Given a committed absorption fixture whose absorbed_by relation has "<condition>"
    When itos runs "work check --json"
    Then itos exits with code 1
    And a reported problem has rule "work-absorption"
    And that problem identifies the source and invalid target or state

    Examples:
      | condition                       |
      | a missing target                |
      | a target not yet done           |
      | a target that is an idea        |
      | a source not yet done           |
      | a source that is a task         |
      | a non-string target value       |

  @ID-WORK-135 @slice-149 @wip
  Scenario: a rejected absorption commit restores every source and its queue placement
    Given a committed absorption fixture with queued ideas "p1-first,p1-second" and completed task "T-001"
    And a recording commit hook rejects the combined absorption close
    And an unrelated change to "NOTES.md" is staged
    And the completion state is saved for comparison
    When itos runs "work done p1-first p1-second --by T-001"
    Then itos refuses completion naming the rejecting hook
    And the closure hook ran exactly once
    And the registry ledger queue index and HEAD match the saved completion state

  @ID-WORK-136 @slice-149 @wip
  Scenario: stealth absorption writes the configured registry without committing or altering the completed task
    Given a committed absorption fixture with queued idea "p1-extra" and completed task "T-001" using relocated stealth data
    And the completion state is saved for comparison
    When itos runs "work done p1-extra --by T-001 --json"
    Then itos exits with code 0
    And the configured idea is done with absorbed_by "T-001" and is not queued
    And its JSON's "commit" is null
    And the completed task and its ledger data are unchanged
    And the working tree index and HEAD match the saved completion state
