@phase-1
Feature: The footer rules are itos's, whatever the header lint
  commits.footers says which footer each commit type needs and what its IDs
  must be. itos runs those rules itself, in the commit-msg hook, in commit
  check-message and in verify, beside the header lint. It does this whether
  or not commits.header_lint delegates to a command. Until now a configured
  delegate replaced them. They ran only if the delegate's own config carried
  them (this repository's commitlint plugin), and a delegate without that
  plugin skipped them without a word. The delegate judges the header and
  body, itos judges the footers, and both report before the hook exits.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged

  # commitlint's conventional config carries no footer rule, so only itos can
  # reject this footer.
  @ID-FOOT-01 @slice-10
  Scenario: With a header lint delegate, a footer naming an unknown task is rejected
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-999
      """
    Then itos exits with code 1
    And its output says "unknown tasks: T-999"
    And its output names the rule "task-footer"

  @ID-FOOT-02 @slice-10
  Scenario: With a header lint delegate, a commit without the footer its type needs is rejected
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output names the rule "task-footer"

  @ID-FOOT-03 @slice-10
  Scenario: The header lint's problems and the footer rules' are both reported
    Given the header lint is commitlint's conventional config
    When the commit-msg hook checks the message:
      """
      update things

      Task: T-999
      """
    Then itos exits with code 1
    And its output names the rule "type-empty"
    And its output names the rule "task-footer"

  @ID-FOOT-04 @slice-10
  Scenario: verify, with a header lint delegate, rejects a commit without the footer its type needs
    Given the header lint is commitlint's conventional config
    And the commit "chore: tidy the readme" on top of it
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output names the rule "task-footer"

  # The footer rules read the footer's source even for a message that names
  # no ID; with the ledger folder gone that was a raw ENOENT, exit 2.
  @ID-FOOT-05 @slice-13
  Scenario: A message that names no task needs no ledger
    Given the ledger folder is missing
    When the commit-msg hook checks the message "docs: write the readme"
    Then itos exits with code 0

  @ID-FOOT-06 @slice-13
  Scenario: A footer naming a task when the ledger folder is missing is one problem naming the folder
    Given the ledger folder is missing
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 2
    And its output says "tasks"
    And its output does not say "ENOENT"

  # Slice 63 (the user's calls, 2026-10-04, p1-work-show-spec-commits): an
  # Item footer names the registry item a commit belongs to, for the commits
  # no other footer ties to it: a spec written in a docs commit, a slice's
  # steps committed red first in a test commit, a chore it needs. Its source
  # is the work registry (source: registry), each id checked against the
  # registry the commit carries, with the rule item-footer. in_place_of says
  # which other footer it stands in for, and for which types: the config
  # below takes it in place of Task for test, docs and chore, so a slice,
  # which has no task, can commit its steps before its feat; every other
  # type still needs its Task.
  @ID-FOOT-07 @slice-63
  Scenario: A test commit naming a registry item in an Item footer needs no Task footer
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      test: add slice 9's steps

      Item: slice-9
      """
    Then itos exits with code 0

  @ID-FOOT-08 @slice-63
  Scenario: An Item footer naming an item the registry does not have is rejected
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      test: add slice 7's steps

      Item: slice-7
      """
    Then itos exits with code 1
    And its output says "slice-7"
    And its output names the rule "item-footer"

  @ID-FOOT-09 @slice-63
  Scenario: A type outside in_place_of still needs its Task footer beside an Item footer
    Given the config has an Item footer of registry items, taken in place of Task for test, docs and chore
    And the work registry has the item "slice-9" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      refactor: split the parser

      Item: slice-9
      """
    Then itos exits with code 1
    And its output names the rule "task-footer"

  # Slice 194 (the person, 2026-10-27). A build or ci commit could name only a
  # ledger task. A gate change (three lines out of itos.yaml's config set),
  # T-190's toolchain bump briefed with itos commit --item, and a review's
  # comment-only rewording of a doc.go were each refused "build commits need
  # --task <id>" (or ci, or refactor), and an agent borrowed a done task, or
  # the coordinator added one, to get past it. The rule: a build or ci commit
  # may name the registry item it belongs to in Item:, an open bug, slice,
  # task or idea, as test, docs and chore already may, and then needs no
  # Task:; Task: is taken as today. Its done is judged as work done judges
  # any Item: commit, by the commit landing green: a gate change needs no
  # ledger task with a check of its own.
  #
  # Where it is said. Which types an Item footer stands in for Task in is the
  # policy's already: commits.footers.<key>.in_place_of (slice 63), which
  # itos commit --item, the commit-msg hook, check-message and verify all
  # read. So it is a config switch, not a default: commits.footers has no
  # built-in default, the starter itos init writes declares no Item footer,
  # and widening in_place_of in the binary would take Item: in place of
  # Task: for types a consumer's policy leaves out of its list.
  # This repository's itos.yaml turns it on, in_place_of { Task: [test, docs,
  # chore, refactor, build, ci] } with must_be_live: true: refactor too,
  # since build may touch no Go file here and a comment-only Go change is a
  # refactor. --item <task> on such a commit writes Item: <task>, since every
  # ledger task is a registry item (kind task).
  #
  # What is new in itos is "open". must_be_live, until now read only for a
  # footer of named tests (a @wip scenario is not live), is read for a footer
  # of registry items too: an item whose status is done or dropped, in the
  # registry the footer is read against (read_at), is not live, and a footer
  # with must_be_live naming one is refused under its rule (item-footer) with
  # the line "items done or dropped: <ids>", after any unknown items, as a
  # test not live is. todo, doing and blocked are open. It holds for every
  # type the footer validates (validate_for), test, docs and chore included:
  # a done item's work is finished, and a later commit belongs to a new item.
  # A registry footer without must_be_live takes any item the registry lists,
  # done ones too, as today, so no consumer's config changes meaning. The
  # config schema's words for must_be_live and the footer's line in the agent
  # rules (itos guide) say so.
  #
  # The config of these scenarios: Task from the ledger for every type but
  # feat, fix, docs and style, and an Item footer of registry items with
  # must_be_live, taken in place of Task for test, docs, chore, build and ci.
  #
  # Guard: passes before the work but for its new config step, and must keep
  # passing. in_place_of already serves any type it lists; this one holds
  # that "open" refuses no open item.
  @ID-FOOT-12 @slice-194 @wip
  Scenario: A build commit naming an open bug in its Item footer needs no Task footer
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the work registry has the item "bug-5" owned by nobody with the status "todo"
    When the commit-msg hook checks the message:
      """
      build: build with the next Go patch release

      Item: bug-5
      """
    Then itos exits with code 0

  # Guard, as ID-FOOT-12: an idea, which has neither scenarios nor a task,
  # is a gate change's item.
  @ID-FOOT-13 @slice-194 @wip
  Scenario: A ci commit naming an open idea in its Item footer needs no Task footer
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the work registry has the idea "p1-tidy-ci" owned by nobody
    When the commit-msg hook checks the message:
      """
      ci: drop the hooks path the config set no longer needs

      Item: p1-tidy-ci
      """
    Then itos exits with code 0

  # Guard, as ID-FOOT-12: the trap T-190's agent met. --item naming a
  # ledger task, which the registry lists as a task, writes Item:, never
  # Task:, and the hook takes it.
  @ID-FOOT-14 @slice-194 @wip
  Scenario: itos commit --item naming a task on a ci commit writes the Item footer
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the commit-msg hook is installed
    And the work registry has the task "T-001" owned by nobody with the status "doing" and no why
    When itos commits with the arguments "--item T-001 -m 'ci: run the linter on every push'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Item: T-001"
    And the message of HEAD does not say "Task:"

  # Before the work must_be_live is read for no registry footer: the hook
  # takes the done item and exits 0, so this fails at the exit code. The
  # Item footer gives an ID, so Task: is not asked for: only item-footer
  # refuses.
  @ID-FOOT-15 @slice-194 @wip
  Scenario: An Item footer naming a done item is refused
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the work registry has the item "bug-5" owned by nobody with the status "done"
    When the commit-msg hook checks the message:
      """
      build: build with the next Go patch release

      Item: bug-5
      """
    Then itos exits with code 1
    And its output says "items done or dropped: bug-5"
    And its output names the rule "item-footer"
    And its output does not say "task-footer"

  # A dropped item, in a docs commit: "open" holds for the types Item: stood
  # in for before this slice too. Fails before the work at the exit code, as
  # ID-FOOT-15.
  @ID-FOOT-16 @slice-194 @wip
  Scenario: An Item footer naming a dropped item is refused, in a docs commit too
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the work registry has the item "p1-tidy-ci" owned by nobody with the status "dropped"
    When the commit-msg hook checks the message:
      """
      docs: say why the hooks path is set

      Item: p1-tidy-ci
      """
    Then itos exits with code 1
    And its output says "items done or dropped: p1-tidy-ci"
    And its output names the rule "item-footer"

  # Guard, as ID-FOOT-12: standing in for Task: on a ci commit does not
  # spare the Item footer its own check (ID-FOOT-08's words).
  @ID-FOOT-17 @slice-194 @wip
  Scenario: A ci commit's Item footer naming an item the registry does not have is refused
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And the work registry has the item "bug-5" owned by nobody with the status "todo"
    When the commit-msg hook checks the message:
      """
      ci: drop the hooks path the config set no longer needs

      Item: p1-no-such-idea
      """
    Then itos exits with code 1
    And its output says "unknown items: p1-no-such-idea"
    And its output names the rule "item-footer"
    And its output does not say "task-footer"

  # Guard, as ID-FOOT-12: Item: is an alternative, Task: is still taken.
  @ID-FOOT-18 @slice-194 @wip
  Scenario: A ci commit naming a task in its Task footer is still accepted
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    When the commit-msg hook checks the message:
      """
      ci: run the linter on every push

      Task: T-001
      """
    Then itos exits with code 0

  # Guard, as ID-FOOT-12: feat and fix are unchanged. Item: stands in for
  # Task: alone, never for the Scenarios: a fix needs.
  @ID-FOOT-19 @slice-194 @wip
  Scenario: A fix naming an open item in its Item footer still needs its Scenarios footer
    Given the config has an Item footer of open registry items, taken in place of Task for test, docs, chore, build and ci
    And a feature file "a.feature" with the live scenario "@ID-A-01"
    And the config's Scenarios footer is required for feat and fix
    And the work registry has the item "bug-5" owned by nobody with the status "doing"
    When the commit-msg hook checks the message:
      """
      fix: stop the parser dropping the last line

      Item: bug-5
      """
    Then itos exits with code 1
    And its output names the rule "scenarios-footer"

  # Bug 110 (issue #21; the user's call, 2026-10-12): footers follow git. A
  # line beginning with a footer's key was read as that footer wherever it
  # stood, so a wrapped body line beginning "Task:" named tasks (slice 127's
  # feat was refused so), and a footer above more prose counted where git's
  # trailers do not see it. The footers are the message's last paragraph,
  # when its lines are trailers, as git interpret-trailers reads them.
  @ID-FOOT-10 @bug-110
  Scenario: A body line beginning with a footer's key is prose, not that footer
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Name the work in the footers, with
      Task: or Item: as the config asks.

      Task: T-001
      """
    Then itos exits with code 0

  @ID-FOOT-11 @bug-110
  Scenario: A footer above the last paragraph is not read, as git's trailers do not read it
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001

      The readme said the same thing twice.
      """
    Then itos exits with code 1
    And its output names the rule "task-footer"
