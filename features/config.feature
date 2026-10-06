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
  # Weak as written (the coordinator's spec): the default, tools/bin/itos,
  # contains "bin/itos" too, so printing the default would pass. The corpus
  # case "config get prints the hooks.bin the config sets, not its default"
  # pins the exact output; p3-config-21-own-value tightens this one.
  @ID-CONFIG-21 @slice-45
  Scenario: config get prints a key's value as the config sets it
    Given hooks.bin is "bin/itos"
    When itos runs "config get hooks.bin"
    Then itos exits with code 0
    And its output says "bin/itos"

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
