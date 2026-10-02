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
  @ID-CONFIG-05 @slice-3
  Scenario: CI stops at the first failing step by default
    Given the CI steps are "exit 3" then a step that records it ran
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 3
    And its output says "CI failed at: exit 3"
    And the recording step did not run

  @ID-CONFIG-06 @slice-3
  Scenario: CI goes on past a failing step when ci.stop_at_first_failure is false, and still fails
    Given the CI steps are "exit 3" then a step that records it ran
    And ci.stop_at_first_failure is false
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 3
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

  # Detection would pick husky from the .husky folder; the key overrides it.
  @ID-CONFIG-11 @slice-4
  Scenario: hooks install writes the shims of the manager hooks.manager names, over what it detects
    Given a ".husky" folder
    And hooks.manager is "git"
    When itos installs the hooks
    Then itos exits with code 0
    And the file ".git/hooks/commit-msg" calls itos

  # Keys for features not built yet: they come back with the feature (the
  # built-in header lint, a second way to tell a commit is pushed). use has one
  # value, command, and only the built-in lint would give it a second.
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

  @ID-CONFIG-14 @slice-4
  Scenario: config check rejects commits.header_lint.use, which nothing reads yet
    Given the config sets "commits.header_lint.use" to "command"
    When itos checks the config
    Then itos exits with code 2
    And its output says "commits.header_lint.use"

  # config check --print-defaults prints one table, but each tool wrote its own
  # fallback where it read a key, so the two drifted: the table gives
  # tag_prefix "@", while the smoke rule took no prefix and compared
  # "@ID-A-01" with "ID-A-01".
  @ID-CONFIG-15 @slice-19 @wip
  Scenario: Without tests.<kind>.tag_prefix, the smoke rule reads a smoke ID with the default prefix
    Given a feature file "a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And the kind leaves out tag_prefix
    When itos checks the smoke set
    Then itos exits with code 0

  @ID-CONFIG-16 @slice-19 @wip
  Scenario: config check --print-defaults lists hooks.bin's default
    When itos prints the defaults of the config
    Then itos exits with code 0
    And its output says "bin: tools/bin/itos"
