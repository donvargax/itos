@phase-2
Feature: The config's patterns are RE2 regular expressions
  Five keys of itos.yaml are compiled as regular expressions: ledger.id,
  ledger.group.pattern, tests.<kind>.id, ci.cost.static and
  ci.covers[].matches. The TypeScript read them as JavaScript's and the Go
  binary as RE2, Go's, so a pattern with a lookaround or a backreference
  passed one and was refused by the other. They are RE2 in both, the user's
  call: the Go binary is what remains after the switch, and RE2 matches in
  linear time, so no pattern in a project's config can hang a commit hook.
  config check refuses what RE2 cannot compile, naming RE2 in its fix.

  Background:
    Given a repository whose ledger has the task "T-001"

  # The lookahead compiles in JavaScript, so the TypeScript passed it while
  # the Go binary refused it, with a fix that named JavaScript.
  @ID-PATTERN-01 @slice-24 @wip
  Scenario: config check refuses a lookahead, which RE2 does not have
    Given the config sets "ledger.group.pattern" to "(?=\d)\d+"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.group.pattern"
    And its output says "RE2"

  @ID-PATTERN-02 @slice-24 @wip
  Scenario: config check refuses a backreference, which RE2 does not have
    Given the config sets "ledger.group.pattern" to "(\d)\1*"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.group.pattern"
    And its output says "RE2"
