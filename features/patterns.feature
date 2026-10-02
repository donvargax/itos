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

  The TypeScript cannot run RE2: it compiles each pattern with JavaScript's
  RegExp, which it needs to run it, and refuses beside that what RE2 cannot
  compile: a lookaround, a backreference, a letter escape or a class RE2 has
  no reading for, and a repeat count above RE2's 1000. On a
  pattern both compile, matching is RE2's from now on, and the TypeScript's
  differs where the dialects do (\s is JavaScript's whitespace and RE2's
  ASCII one, and . does not match \r in JavaScript): a known gap, closed when
  the TypeScript goes at the switch.

  Background:
    Given a repository whose ledger has the task "T-001"

  # The lookahead compiles in JavaScript, so the TypeScript passed it while
  # the Go binary refused it, with a fix that named JavaScript. The other
  # direction, a pattern RE2 compiles and JavaScript cannot, such as (?i)abc,
  # has no scenario: the TypeScript cannot run it, so it still refuses it, and
  # that is safe, since the TypeScript goes at the switch and the Go binary,
  # which accepts it, stays.
  @ID-PATTERN-01 @slice-24
  Scenario: config check refuses a lookahead, which RE2 does not have
    Given the config sets "ledger.group.pattern" to "(?=\d)\d+"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.group.pattern"
    And its output says "RE2"

  # The backreference compiles in JavaScript too; RE2 has no \1 to match by.
  @ID-PATTERN-02 @slice-24
  Scenario: config check refuses a backreference, which RE2 does not have
    Given the config sets "ledger.group.pattern" to "(\d)\1*"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.group.pattern"
    And its output says "RE2"

  # Slice 24 refused lookarounds and backreferences, but JavaScript compiles
  # other patterns RE2 refuses (p2-pattern-re2-escapes): a repeat count above
  # RE2's 1000, \c, letter escapes, [\b], [] and [^]. The TypeScript let them
  # through, the Go binary refused them; folded into v1.1.0, so consumers are
  # asked for the pattern change once.
  @ID-PATTERN-03 @bug-1
  Scenario: config check refuses a repeat count above RE2's limit
    Given the config sets "ledger.group.pattern" to "\d{1,1001}"
    When itos checks the config
    Then itos exits with code 2
    And its output says "ledger.group.pattern"
    And its output says "RE2"
