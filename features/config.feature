@phase-1
Feature: Every key the config accepts is one itos reads
  itos.yaml is strict: itos config check rejects a key it does not know. A key
  it accepts must then change what itos does. One that is validated and never
  read is a promise the tool does not keep, and the Go port, which copies the
  contract, would copy the promise. So each accepted key is read, and a key
  whose feature is not built yet is not accepted until it is.

  Background:
    Given a repository whose ledger has the task "T-001"

  # The recording shell is a script that appends each command it is given to a
  # file, then runs it with sh -c: what it recorded is what went through it.
  @ID-CONFIG-01 @slice-3
  Scenario: A task's check runs through the shell the config names
    Given the config's shell is the recording shell
    And the task "T-001" has the check "exit 0"
    When itos runs the task "T-001"
    Then itos exits with code 0
    And the recording shell ran "exit 0"

  @ID-CONFIG-02 @slice-3
  Scenario: A CI step runs through the shell the config names
    Given the config's shell is the recording shell
    And the CI steps are "exit 0"
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 0
    And the recording shell ran "exit 0"

  @ID-CONFIG-03 @slice-3
  Scenario: The header lint's delegate runs through the shell the config names
    Given the config's shell is the recording shell
    And the header lint is the command "exit 0"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message "docs: write the readme"
    Then itos exits with code 0
    And the recording shell ran "exit 0"

  @ID-CONFIG-04 @slice-3
  Scenario: A range check runs through the shell the config names
    Given the config's shell is the recording shell
    And a range check that records where its range starts
    And the commit "docs: write the readme" on top of it
    When itos verifies every commit up to HEAD
    Then the recording shell ran the range check

  # The second step writes a file, so whether it ran is read from the tree.
  # A failing step makes ci run exit 1, naming its own code (slice 86).
  @ID-CONFIG-05 @slice-3
  Scenario: CI stops at the first failing step by default
    Given the CI steps are "exit 3" then a step that records it ran
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 1
    And its output says "CI failed at: exit 3"
    And the recording step did not run

  @ID-CONFIG-06 @slice-3
  Scenario: CI goes on past a failing step when ci.stop_at_first_failure is false, and still fails
    Given the CI steps are "exit 3" then a step that records it ran
    And ci.stop_at_first_failure is false
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 1
    And its output says "CI failed at: exit 3"
    And the recording step ran

  @ID-CONFIG-07 @slice-4
  Scenario: work check rejects a status that work.statuses does not list
    Given work.statuses is "todo, done"
    And the work registry has the item "T-001" with the status "doing"
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "unknown status"

  @ID-CONFIG-08 @slice-4
  Scenario: The work registry's owners per group are read from the key work.groups_key names
    Given work.groups_key is "milestones"
    And the work registry gives the group "1" to the owner "ana" under "milestones"
    When itos checks the work registry
    Then itos exits with code 0

  @ID-CONFIG-09 @slice-4
  Scenario: Without smoke.every_file, a feature file with live scenarios may have no smoke test
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the live scenario "@ID-B-01"
    And the smoke set lists only "@ID-A-01"
    And smoke.every_file is false
    When itos checks the smoke set
    Then itos exits with code 0

  @ID-CONFIG-10 @slice-4
  Scenario: With smoke.every_file, a feature file with live scenarios and no smoke test is rejected
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And a feature file "b.feature" with the live scenario "@ID-B-01"
    And the smoke set lists only "@ID-A-01"
    And smoke.every_file is true
    When itos checks the smoke set
    Then itos exits with code 1
    And its output says "b.feature"

  # Keys for features not built yet: they come back with the feature (a second
  # way to tell a commit is pushed). alongside would run the built-in header
  # lint beside a delegate, warning only; it stays refused, since holding the
  # built-in lint to commitlint's verdicts is a comparison over a history
  # (T-063), not a mode a project's commits run in.
  @ID-CONFIG-12 @slice-4
  Scenario: config check rejects commits.header_lint.alongside, which nothing reads yet
    Given the config sets "commits.header_lint.alongside" to "builtin"
    When itos checks the config
    Then itos exits with code 2
    And its output says "commits.header_lint.alongside"

  @ID-CONFIG-13 @slice-4
  Scenario: config check rejects ledger.check.pushed, which nothing reads yet
    Given the config sets "ledger.check.pushed" to "remote-branch-contains-head"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.check.pushed"

  # use chooses the header lint, builtin or command (slice 25); it was refused
  # until the built-in lint gave it a second value. One it does not have is
  # refused, naming the key.
  @ID-CONFIG-14 @slice-4
  Scenario: config check rejects a commits.header_lint.use other than builtin or command
    Given the config sets "commits.header_lint.use" to "alongside"
    When itos checks the config
    Then itos exits with code 2
    And its output says "commits.header_lint.use"

  # config check --print-defaults prints one table, but each tool wrote its own
  # fallback where it read a key, so the two drifted: the table gives
  # tag_prefix "@", while the smoke rule took no prefix and compared
  # "@ID-A-01" with "ID-A-01".
  @ID-CONFIG-15 @slice-19
  Scenario: Without tests.<kind>.tag_prefix, the smoke rule reads a smoke ID with the default prefix
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And the kind leaves out tag_prefix
    When itos checks the smoke set
    Then itos exits with code 0

  # ledger.group.label names what a ledger's groups are (phases here). It was
  # accepted and read by nothing: the tools said "phase" whatever it was.
  @ID-CONFIG-17 @slice-21
  Scenario: work check names a group the registry does not list by ledger.group.label
    Given ledger.group.label is "milestone"
    And the work registry has the item "T-001" in the group "7", which it does not list
    When itos checks the work registry
    Then itos exits with code 1
    And its output says "milestone 7 is not listed"

  @ID-CONFIG-18 @slice-21
  Scenario: itos task takes a group by ledger.group.label as a flag
    Given ledger.group.label is "milestone"
    And the task "T-001" has the check "exit 0"
    When itos runs the tasks of the group "1" by the flag "--milestone"
    Then itos exits with code 0
    And its output lists "T-001" as "done"

  # Slice 35: config check validates files, so it says a people file is
  # missing, as a warning that never fails it.
  @ID-CONFIG-19 @slice-35
  Scenario: config check warns of a missing people file, and passes
    Given the people file is missing
    When itos checks the config
    Then itos exits with code 0
    And its output says "people"

  # Bug 133: with no people file a work take with no --as records no owner
  # (bug 46). config check's warning of the missing file, which itos init
  # notes too, says so where it says the owners go unchecked: "no people
  # file at people.yaml (work.people), so the registry's owners go
  # unchecked and work take with no --as records no owner". The warning of
  # an unreadable people file, the other case with no one in the people
  # that warns, ends in the same words. A people file listing no one warns
  # of nothing today; that is not this bug's. The corpus pins the
  # warning's rule and area, not its words. Fails at the output step.
  @ID-CONFIG-70 @bug-133
  Scenario: config check's warning of a missing people file says a take records no owner
    Given the people file is missing
    When itos checks the config
    Then itos exits with code 0
    And its output says "work take with no --as records no owner"

  # Slice 40: itos read itos.yaml only from the working folder, so a run from
  # a subfolder found no config. With no --config, no ITOS_CONFIG and no
  # --root, a run from inside a repository finds the itos.yaml at its top
  # (git rev-parse --show-toplevel) and runs as if started there, so every
  # path the config names means what it means at the top.
  @ID-CONFIG-20 @slice-40
  Scenario: From a subfolder, itos reads the itos.yaml at the repository's top and runs as from there
    Given a "sub" folder
    When itos runs the task "T-001" from "sub"
    Then itos exits with code 0
    And its output says "T-001"

  # Slice 45: a program that works beside itos (the Claude Code plugin, a
  # script) needs a config value as itos reads it, defaults applied and the
  # config found as everywhere else (a subfolder, a stealth config), and
  # parsing itos.yaml itself gets those wrong. itos config get <key> prints a
  # key's value, a dotted path, as itos would use it; --json gives it as
  # {"schema": 1, "key", "value"}. A key itos does not know is a usage error.
  # Slice 114: bin/own-itos is a value the default, tools/bin/itos, does not
  # contain, so printing the default fails.
  @ID-CONFIG-21 @slice-45
  Scenario: config get prints a key's value as the config sets it
    Given hooks.bin is "bin/own-itos"
    When itos runs "config get hooks.bin"
    Then itos exits with code 0
    And its output says "bin/own-itos"

  @ID-CONFIG-22 @slice-45
  Scenario: config get prints a key's default when the config leaves it out
    When itos runs "config get hooks.commit_msg.check_timeout"
    Then itos exits with code 0
    And its output says "60"

  @ID-CONFIG-23 @slice-45
  Scenario: config get refuses a key itos does not know
    When itos runs "config get hooks.no_such_key"
    Then itos exits with code 2
    And its output says "hooks.no_such_key"

  # Decision records are itos's data too: a record written or edited by
  # hand could share another's number, or say it was superseded by one that
  # does not exist, and nothing said so (p1-adr-checks, 2026-10-04). config
  # check holds the folder work.decisions names to both rules, and the
  # commit-msg hook runs it when a record is staged. The index is not held to
  # the folder: itos decision record writes it whole at every record.
  @ID-CONFIG-24 @slice-74
  Scenario: config check refuses two decision records with the same number
    Given the committed file "docs/decisions/0001-use-go.md" holding "# Use Go"
    And the committed file "docs/decisions/0001-use-rust.md" holding "# Use Rust"
    When itos runs "config check"
    Then itos exits with code 1
    And its output says "0001"

  @ID-CONFIG-25 @slice-74
  Scenario: config check refuses a record superseded by one the folder does not have
    Given the committed file "docs/decisions/0001-use-go.md" holding the lines:
      """
      ---
      status: superseded by ADR-0009
      ---

      # Use Go
      """
    When itos runs "config check"
    Then itos exits with code 1
    And its output says "ADR-0009"

  # Slice 79, v4.0.0 (was v3-hooks-bin-default; the user's calls, 2026-10-03
  # and 2026-10-05): hooks.bin defaults to itos, the global launcher, so a
  # repository with no hooks.bin runs the global itos and its pin, and a new
  # repository gets no default naming a script it lacks; a CI runner gets
  # one with the action of T-093. The key stays, internal and unsupported,
  # for a repository that must run its own build, as this one does.
  @ID-CONFIG-26 @slice-79
  Scenario: config check --print-defaults lists hooks.bin's default, the global itos
    When itos prints the defaults of the config
    Then itos exits with code 0
    And its output says "bin: itos"
    And its output does not say "bin: tools/bin/itos"

  # Bug 26 (issue #11; was p1-ci-watch-alone; the user's call, 2026-10-05).
  # The schema required ci.steps whenever a ci section was written, so a
  # repository adopting itos step by step, its commit gates first and its CI
  # plan later, could not let itos status, itos push and itos ci watch read
  # its CI through ci.watch, and a stealth config, with no CI plan of its
  # own, had to write steps: []. ci.steps is needed only by the commands
  # that run the plan, ci plan and ci run, so it is required there, as a
  # section a tool cannot work without, and ci.watch and ci.range are each
  # valid without it.
  @ID-CONFIG-28 @bug-26
  Scenario: A ci section holding only a watch passes config check
    Given the config's ci section holds only a github watch of "ci.yml"
    When itos checks the config
    Then itos exits with code 0

  @ID-CONFIG-29 @bug-26
  Scenario: Without ci.steps, ci plan says the plan is missing
    Given the config's ci section holds only a github watch of "ci.yml"
    When itos plans CI over the commits after the first
    Then its output says "ci.steps is missing"

  # Slice 85, v5.0.0 (the user's calls, 2026-10-05). The code review of
  # 07b0e6d found itos running a repository's own commands without anyone
  # choosing to: the git shim's push ran ci.watch.command, and itos go and
  # itos status ran ci.range.command, ci.watch's commands and work.identity's
  # command, the first things a person runs in a clone. Running itos task, ci
  # run or installed hooks is a choice to run a repository's commands, as
  # make or npm test is; these were not. v5 removes the command provider of
  # ci.range, ci.watch (its command and nightly_command) and work.identity,
  # leaving github or none, and --as for who a session works for; config
  # check refuses the provider, and the keys only it read, saying they were
  # removed in v5. It also removes ci.range.github.branch, which nothing
  # reads since bug 29 counts a commit's green run on any branch (if a tool
  # still reads it, the key stays and this scenario goes). The live
  # scenarios that fake CI or identity with a command (watch.feature,
  # push.feature, status.feature, the work scenarios with an identity
  # command, and their corpus cases) move to the fake GitHub of bug 23, or
  # go where a github one says the same, each named with --breaking. The
  # module path moves to its /v5, as T-094 moved it to /v4. Providers for other hosts come back behind a trust gate:
  # p3-command-providers-back.
  @ID-CONFIG-30 @slice-85
  Scenario: config check refuses ci.range's command provider, removed in v5
    Given the config sets "ci.range.provider" to "command"
    When itos checks the config
    Then itos exits with code 2
    And its output says "removed in v5"

  @ID-CONFIG-31 @slice-85
  Scenario: config check refuses ci.watch's command provider, removed in v5
    Given the config sets "ci.watch.provider" to "command"
    When itos checks the config
    Then itos exits with code 2
    And its output says "removed in v5"

  @ID-CONFIG-32 @slice-85
  Scenario: config check refuses work.identity's command provider, removed in v5
    Given the config sets "work.identity.provider" to "command"
    When itos checks the config
    Then itos exits with code 2
    And its output says "removed in v5"

  @ID-CONFIG-33 @slice-85
  Scenario: config check refuses ci.range.github.branch, removed in v5
    Given the config sets "ci.range.github.branch" to "main"
    When itos checks the config
    Then itos exits with code 2
    And its output says "removed in v5"

  # Slice 89 (decision of q-16): hooks install and the hook group differed
  # by one letter. With the guard moved out, itos hook holds git's hooks
  # alone, and installing them is itos hook install. The step "itos installs
  # the hooks" runs the new name; the feat rewrites what names the old one.
  @ID-CONFIG-37 @slice-89
  Scenario: itos hooks install exits 2, naming itos hook install
    When itos runs "hooks install"
    Then itos exits with code 2
    And its output says "itos hook install"

  # Slice 91 (q-17, the user's call, 2026-10-06): itos knew six hook managers
  # (Vite+, husky, lefthook, pre-commit, prek, plain git), detected them by
  # their marker files and wrote its shims in each one's layout, with
  # hooks.manager and --manager to override the detection. It now declares
  # its two hooks in the git config alone, the project's hook files and
  # settings untouched, so it needs to know no manager. The marker detection,
  # the per-manager writing, --manager and hooks.manager go; the feat removes
  # the scenarios that tested a manager, marked breaking.
  @ID-CONFIG-38 @slice-91
  Scenario: hook install declares itos's hooks in the git config and touches no hook file
    Given the project's hooks are in ".husky" by core.hooksPath, with a commit-msg hook that records it ran
    When itos runs "hook install"
    Then itos exits with code 0
    And the git config declares a "commit-msg" hook that runs itos
    And the git config declares a "pre-push" hook that runs itos
    And the file ".husky/commit-msg" does not name "itos hook"
    And core.hooksPath is still ".husky"

  @ID-CONFIG-39 @slice-91
  Scenario: config check refuses hooks.manager, which v6 removed
    Given the config sets "hooks.manager" to "husky"
    When itos checks the config
    Then itos exits with code 2
    And its output says "hooks.manager"

  # Bug 39 (the code review of 07b0e6d, 2026-10-05): itos converted YAML
  # aliases with no record of the anchors it was inside and no limit on how
  # far they expand, so an alias inside its own anchor (a: &x [*x]) ended in a
  # fatal stack overflow, in the commit-msg hook too, and nested aliases
  # expanded exponentially. Every file itos reads as YAML (the config, a
  # ledger, the registry) refuses an alias cycle, and an expansion past a
  # bound, as a problem naming the alias, never a crash: the config's as a
  # config error (exit 2), the registry's as what work check refuses (exit 1).
  @ID-CONFIG-40 @bug-39
  Scenario: config check refuses a config whose alias is inside its own anchor, never crashing
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      loop: &x [*x]
      """
    When itos runs "config check"
    Then itos exits with code 2
    And its output says "alias"

  # Slice 100: proof.code is read before any work done needs it, so a
  # config that names no paths, or a check that cannot be given the item's
  # base, is refused by config check rather than at the first close.
  @ID-CONFIG-41 @slice-100
  Scenario: config check refuses a proof.code whose check does not take {base}
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      proof:
        code:
          paths: ["src/**"]
          check: "itos-cc mutation check --fail-uncovered --json"
      """
    When itos runs "config check"
    Then itos exits with code 2
    And its output says "{base}"

  # Slice 107 (q-25, answered by the person 2026-10-08: refuse it in v7;
  # ADR-0043, a task's checks leave the ledger when work done closes it; ADR-0016
  # is already superseded by it; slice-101 removed the checks at the close and
  # T-125 removed this repository's own step, so nothing reads the step and
  # keeping it is the compatibility p1-drop-compat-code exists to end).
  #
  # The step key stays in the schema only to be refused, so a consumer whose
  # config still carries it is told what happened and what to write instead.
  # Deleting the key refuses it as an unknown key, which is honest and one
  # line less of schema, and says nothing about the rule that removed it.
  @ID-CONFIG-42 @slice-107
  Scenario: config check refuses a nightly step that runs a done task's checks, saying what replaced it
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ci:
        nightly:
          steps: [{ tasks: done }]
      """
    When itos runs "config check"
    Then itos exits with code 2
    And its output says "tasks: done"
    And its output says "work done"

  # One problem, not two: the step's own cost rule (config-step-cost) goes with
  # the step, so cost: static is refused as the same thing rather than as a cost
  # on a step that no longer exists.
  @ID-CONFIG-43 @slice-107
  Scenario: The refused step is one problem with or without a cost
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ci:
        nightly:
          steps: [{ tasks: done, cost: static }]
      """
    When itos runs "config check"
    Then itos exits with code 2
    And its output says "tasks: done"
    And its output does not say "config-step-cost"

  # A consumer who never runs config check must still learn: the plan reads the
  # same config, so it refuses the same step the same way, rather than planning
  # a step no longer has an implementation.
  @ID-CONFIG-44 @slice-107
  Scenario: ci plan refuses the nightly step too, so a plan of a stale config is never a pass
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      ci:
        nightly:
          steps: [{ tasks: done }]
      """
    When itos runs "ci plan --nightly"
    Then itos exits with code 2
    And its output says "tasks: done"

  # Slice 129 (p1-shell-not-empty, with p1-moves-except-types-checked,
  # p1-wip-tag-command-kind, p1-group-label-flag-word and
  # p1-config-finite-numbers, which folds in p1-check-timeout-positive; the v8
  # plan, 2026-10-11): config check accepted values no tool can use, each found
  # only when a command later failed with no word of the key. It now refuses
  # them, exit 1 naming the key: an empty shell (every command would run as
  # sh <command>, a script path); a range check's except_types naming a type
  # commits.types does not list (a misspelt one leaves feat judged); wip_tag on
  # a kind whose adapter is a command (it is read nowhere); a ledger.group.label
  # that cannot be itos task's --<label> flag (a space, or another of its
  # flags such as skip or pending); a number that is .nan or .inf anywhere in
  # the config; and a timeout of 0 or less (ledger.check.timeout, a check's own
  # timeout, ci.watch's interval and timeout), which lifts the hook's cap. The
  # config sets writes its value as a quoted string, which the schema would
  # refuse as the wrong type whatever these rules said, so a value meant as
  # YAML ([] an empty list, .nan and 0 numbers) is set "to the YAML". Refusing
  # what was accepted is breaking, so the feat says so. Slice 130 moved the
  # empty shell, the number not finite and the timeout of 0 to the loader, so
  # @ID-CONFIG-45, 49 and 50 exit 2, as every command does on them.
  @ID-CONFIG-45 @slice-129
  Scenario: config check refuses an empty shell
    Given the config sets "shell" to the YAML "[]"
    When itos checks the config
    Then itos exits with code 2
    And its output says "shell"

  @ID-CONFIG-46 @slice-129
  Scenario: config check refuses an except_types type that commits.types does not list
    Given the config's scenario kind has the range check "scenario-moves", builtin "moves", except_types "fead"
    When itos checks the config
    Then itos exits with code 1
    And its output says "fead"

  @ID-CONFIG-47 @slice-129
  Scenario: config check refuses wip_tag on a kind whose adapter is a command
    Given the config has the test kind "unit" with a command adapter and the wip_tag "@wip"
    When itos checks the config
    Then itos exits with code 1
    And its output says "wip_tag"

  @ID-CONFIG-48 @slice-129
  Scenario: config check refuses a ledger.group.label that cannot be itos task's flag
    Given the config sets "ledger.group.label" to "skip"
    When itos checks the config
    Then itos exits with code 1
    And its output says "ledger.group.label"

  @ID-CONFIG-49 @slice-129
  Scenario: config check refuses a number that is not finite
    Given the config sets "ci.watch.interval" to the YAML ".nan"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ci.watch.interval"

  @ID-CONFIG-50 @slice-129
  Scenario: config check refuses a timeout of 0 or less
    Given the config sets "ledger.check.timeout" to the YAML "0"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.check.timeout"

  # Slice 130 (p1-ledger-layout-checks, with p1-load-refuses-unusable-values,
  # p1-asks-check and p1-ledger-id-default; the v8 plan and the user's calls of
  # 2026-10-11). Four gaps of the same family, each found only when a later
  # command failed with no word of the key:
  #
  # - Values that make a command misbehave when it reads them are refused when
  #   the config loads, exit 2, by every command, as ci.watch.timeout of 0 or
  #   less already is: a .nan or .inf number (a .nan ci.watch.interval panicked
  #   itos push; a .nan hooks.commit_msg.check_timeout left the hook uncapped),
  #   a timeout of 0 or less, an empty shell. Slice 129 made config check name
  #   them with exit 1 (@ID-CONFIG-45, 49 and 50); they move to the loader, so
  #   config check exits 2 for them too, and the feat names those three in
  #   Changes. What is only unread or inconsistent (except_types, wip_tag on a
  #   command kind, a group label that cannot be a flag) stays config check's,
  #   exit 1.
  # - Ledger layouts that read nothing: a ledger.group.pattern written with its
  #   own anchors, which never matches inside the file name; {group} as a
  #   folder in ledger.files, which passes and later fails as a missing folder;
  #   a ledger.files that matches work.registry, naming both keys. And
  #   ledger.group.numeric now sorts the files by number (phase-2 before
  #   phase-10), as its description says; they were sorted byte-wise.
  # - The questions file (work.asks, tasks/asks.yaml) is judged by config check
  #   and by the commit-msg hook's data check, as the ledger and the registry
  #   are: an unknown key, an id given twice, an item naming no item.
  # - ledger.id has one reader (ledger.IDPattern, anchored as ^(?:id)$, so an
  #   alternation anchors every branch; itos task, work show's task items and
  #   minting all use it), and with no ledger.id an ID is whatever the ledger
  #   holds: itos task reads a word as a task ID when the ledger has that task,
  #   not when it matches T-\d+ (@ID-TASK-15).
  #
  # The steps (fixed 2026-10-11 after the implementing agent traced them):
  # "the ledger file … holds the task …" writes a task, where "has the task"
  # asserts one; "the config sets" a key the Background's config already
  # writes (ledger.files, ledger.id) replaces that value rather than adding a
  # second key, which value.Parse refuses; "the config has no ledger.id"
  # writes the config without it.
  @ID-CONFIG-51 @slice-130
  Scenario: Any command refuses a config holding a number that is not finite, exit 2 naming the key
    Given the config sets "ci.watch.interval" to the YAML ".nan"
    When itos runs "work list"
    Then itos exits with code 2
    And its output says "ci.watch.interval"

  @ID-CONFIG-52 @slice-130
  Scenario: config check refuses a ledger.group.pattern written with its own anchors
    Given the config sets "ledger.group.pattern" to "^\d+$"
    When itos checks the config
    Then itos exits with code 1
    And its output says "ledger.group.pattern"

  @ID-CONFIG-53 @slice-130
  Scenario: config check refuses {group} as a folder in ledger.files
    Given the config sets "ledger.files" to "tasks/{group}/tasks.yaml"
    When itos checks the config
    Then itos exits with code 1
    And its output says "ledger.files"

  @ID-CONFIG-54 @slice-130
  Scenario: config check refuses a ledger.files that matches the work registry, naming both keys
    Given the config sets "ledger.files" to "tasks/{group}.yaml"
    And the config sets "ledger.group.pattern" to "[a-z-]+"
    When itos checks the config
    Then itos exits with code 1
    And its output says "ledger.files"
    And its output says "work.registry"

  @ID-CONFIG-55 @slice-130
  Scenario: A numeric ledger group reads phase-2 before phase-10
    Given the config's ledger groups are numeric
    And the ledger file "tasks/phase-10.yaml" holds the task "T-010" with the check "true"
    And the ledger file "tasks/phase-2.yaml" holds the task "T-002" with the check "true"
    When itos runs "task list"
    Then itos exits with code 0
    And its output says "T-002" before "T-010"

  @ID-CONFIG-56 @slice-130
  Scenario: config check refuses a questions file that gives an id twice
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          question: "Which way?"
        - id: q-1
          question: "Which other way?"
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "q-1"

  @ID-CONFIG-57 @slice-130
  Scenario: The commit-msg hook refuses a commit that stages a questions file giving an id twice
    Given itos has installed the hooks
    And the staged file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          question: "Which way?"
        - id: q-1
          question: "Which other way?"
      """
    When itos runs the command line "commit -m 'docs: ask twice'"
    Then itos exits with code 1
    And its output says "q-1"
    And no commit was made

  @ID-CONFIG-58 @slice-130
  Scenario: A ledger.id alternation anchors every branch, so a task id with trailing text is refused
    Given the config sets "ledger.id" to "T-\d+|OPS-\d+"
    And the ledger file "tasks/phase-1.yaml" holds the task "T-1x" with the check "true"
    When itos checks the config
    Then itos exits with code 1
    And its output says "T-1x"

  # Bug 105 (the user's call, 2026-10-11): slice 130 made config check refuse
  # any question naming an item the registry does not have. On itos's own main
  # it found six answered questions naming ideas work promote had since
  # renamed, and the same check run over itos-cc, itos-template and
  # go-template-itos found 31 more, none with a "Was <idea>." marker to repair
  # them from. An answered question is a record: its item gone stale breaks
  # nothing a command does, so config check warns about it (exit 0, naming the
  # question and the item), while an open question, which holds its item up,
  # must still name one the registry has (@ID-CONFIG-60, exit 1). And work
  # promote renames the idea in every question that names it, in the same
  # commit, as it does in every depends_on (@ID-WORK-103), so itos stops
  # writing the stale link itself. A fix: @ID-CONFIG-56 (an id given twice)
  # is unchanged.
  @ID-CONFIG-59 @bug-105
  Scenario: config check warns about an answered question naming an item the registry does not have
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
      """
    When itos checks the config
    Then itos exits with code 0
    And its output says "q-1"
    And its output says "p1-gone"

  @ID-CONFIG-60 @bug-105
  Scenario: config check refuses an open question naming an item the registry does not have
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "q-1"

  # Bug 135 (#49, the user's call, 2026-10-24): the warning of @ID-CONFIG-59
  # and the error of @ID-CONFIG-60 named the question and its stale item but
  # not how to repair them, and no command re-points a question: slice-156's
  # decision relink went to v9, where the typed items' generic link replaces
  # it. So each says the hand edit, the one the asks file takes until then:
  # set that question's item key in the asks file to the item's new id, or
  # remove the key. config check's text prints a warning's or a problem's
  # message alone (its WARN and FAIL lines), never its fix, and itos task,
  # itos init and the commit-msg hook print the same message, so the advice
  # is in the message and again in the JSON's fix. Severity and exit are
  # unchanged: the warning passes, exit 0, among the JSON's warnings. The
  # file is committed past the hooks (the step commits with --no-verify), as
  # an itos before bug 105 left it; no hook is installed.
  @ID-CONFIG-71 @bug-135
  Scenario: config check's warning about an answered question naming a missing item says how to repair it by hand
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
      """
    When itos checks the config
    Then itos exits with code 0
    And its output says "WARN tasks/asks.yaml: q-1, answered, names the item p1-gone"
    And its output says "by hand, set q-1's item in tasks/asks.yaml to p1-gone's new id"
    And its output says "or remove q-1's item key"
    When itos checks the config as JSON
    Then itos exits with code 0
    And its JSON's warning "asks-answered-unknown-item"'s "fix" says "by hand, set q-1's item in tasks/asks.yaml to p1-gone's new id"
    And its JSON's warning "asks-answered-unknown-item"'s "fix" says "or remove q-1's item key"

  # The error's advice is the warning's: an open question's item, renamed or
  # dropped, is repaired by the same edit. q-1, answered, keeps its warning
  # beside q-2's error, each naming another item, so the error's advice must
  # name its own question and item. The error fails the check, exit 1, among
  # the JSON's problems.
  @ID-CONFIG-72 @bug-135
  Scenario: config check's error about an open question naming a missing item says how to repair it by hand
    Given the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - id: q-1
          item: "p1-gone"
          question: "Which way?"
          answer: "This way."
        - id: q-2
          item: "p1-lost"
          question: "Why not?"
      """
    When itos checks the config
    Then itos exits with code 1
    And its output says "FAIL tasks/asks.yaml: q-2 names the item p1-lost"
    And its output says "by hand, set q-2's item in tasks/asks.yaml to p1-lost's new id"
    And its output says "or remove q-2's item key"
    When itos checks the config as JSON
    Then itos exits with code 1
    And its JSON's problem "asks-unknown-item"'s "fix" says "by hand, set q-2's item in tasks/asks.yaml to p1-lost's new id"
    And its JSON's problem "asks-unknown-item"'s "fix" says "or remove q-2's item key"

  # The message for an unknown footer source listed ledger, text and tests but
  # left out registry, which is one and which the fix line already named.
  @ID-CONFIG-69 @bug-126
  Scenario: config check names registry among the footer sources it accepts
    Given the committed file "itos.yaml" holding the lines:
      """
      version: 1
      commits:
        footers:
          Task: { source: nowhere }
      """
    When itos runs "config check"
    Then itos exits with code 2
    And its output says "commits.footers.Task.source"
    And its output says "ledger, registry, text or { tests: <kind> }"
