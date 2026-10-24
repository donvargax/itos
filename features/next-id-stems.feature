@phase-1
Feature: An unambiguous bare scenario area gets a correction before any ID is reserved
  #59: with the id 'ID-[A-Z]+-\d+' and the scenario ID-SEC-01 present,
  itos tests next-id scenario SEC printed SEC-1, an id the pattern refuses,
  and exited 0. q-75 chose refusal with a concrete correction over silent
  normalization, keeping the auxiliary previews (slice, bug) and every full
  stem as they are.

  The rule. tests next-id <kind> <stem> refuses the stem with a correction
  when all of these hold, and behaves as it does today otherwise:
  1. the text of the kind's id pattern, a leading ^ dropped, begins with
  ID-: the test that namedtests.Area and the ID- padding already make;
  2. the stem is no area (Area of the stem is false) and ID-<stem> is one
  (Area of ID-<stem> is true);
  3. the stem is neither slice nor bug, and no id the kind holds (its
  files' tags under a gherkin adapter, its listing's IDs under a command
  adapter) and no id of the work registry is <stem>-<digits>: a raw series
  already in use stays a read-only preview.
  Why the text of the pattern, and not "every string the pattern matches
  begins with ID-": the text test is the one Area makes, so a correction is
  offered exactly when the corrected stem is claimed as an area, and the
  corrected command always claims, never only reads; it needs no analysis
  of the regex, which q-75 rules out (no prefixes inferred from arbitrary
  regexes). So 'ID-[A-Z]+-\d+|T-\d+-[a-z]+', whose ID-SEC is an area today,
  refuses SEC and corrects it to ID-SEC, where the every-match reading
  would keep printing SEC-1; and 'T-\d+-[a-z]+|ID-[A-Z]+-\d+', whose
  ID-SEC is no area either, offers no correction (ID-NEXTID-13).

  The refusal is a usage error: exit 2, nothing on stdout, and on stderr
  the stem, the configured id pattern and the corrected command, itos tests
  next-id <kind> ID-<stem>, followed by --count <n> when --count was given.
  The correction does not repeat --json or -q: they choose how itos
  reports, not what it claims. Under --json the refusal keeps the existing
  JSON usage-error contract, today's for a --count of a stem that only
  reads: exit 2, stdout empty, the message on stderr. The exact wording is
  not judged (ADR-0052), only its parts.

  The correction wins over today's --count refusal: handler.go refuses
  --count of a non-area stem before the registry is read, and for a stem
  the rule corrects that refusal is not given, the correction is; a stem the
  rule does not correct (slice, bug, a raw series) keeps that usage error,
  word for word. Deciding rule 3 reads the registry and the kind's ids
  once, the same read NextTag makes (a command adapter is called once to
  list the working tree); no test runs, and the id counter is never
  contacted or written.

  How the counter is observed: the Background's repository is a clone of a
  remote, so the counter is the remote's ref refs/itos/ids (ids.feature).
  The scenario's repository has never minted, so "the remote has no ref
  refs/itos/ids" after a refusal shows nothing was reserved, and "the
  remote's id counter for ID-SEC is 3" after the corrected claim shows the
  claim alone moved it. ID-NEXTID-12 makes the remote unreachable: a run
  that contacted the counter would exit 3, and the refusal exits 2.

  Existing tests that must keep passing: next-id.feature's live scenarios;
  internal/slices/tests/tests_test.go:474 (TestNextIDRefusesWhatItCannotRead)
  and internal/cli/tests_test.go:90
  (TestTestsNextIDRefusesAWrongLineAndClaimsNothing), which pin the exact
  --count refusal for slice, an auxiliary stem the rule never corrects.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And a feature file "a.feature" with the live scenario "@ID-SEC-01"

  # Red before the work: the first row exits 0 printing @SEC-1; the second
  # exits 2 with today's --count refusal, which names no correction.
  @ID-NEXTID-08 @slice-157
  Scenario Outline: A mistaken bare area is refused with the actual usable stem
    When itos runs "<command>"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "ID-[A-Z]+-\d+"
    And its stderr says "<correction>"
    And the remote has no ref "refs/itos/ids"

    Examples:
      | command                              | correction                                   |
      | tests next-id scenario SEC           | itos tests next-id scenario ID-SEC           |
      | tests next-id scenario SEC --count 2 | itos tests next-id scenario ID-SEC --count 2 |

  # Red before the work at the first exit code: SEC exits 0 printing @SEC-1.
  # The corrected command's first ID, @ID-SEC-02, shows the refusal reserved
  # nothing for ID-SEC, and the counter at 3 that the claim alone moved it.
  @ID-NEXTID-09 @slice-157
  Scenario: The corrected command claims as a full stem always has, the refusal having taken no number
    When itos runs "tests next-id scenario SEC"
    Then itos exits with code 2
    And its stderr says "itos tests next-id scenario ID-SEC"
    And the remote has no ref "refs/itos/ids"
    When itos runs "tests next-id scenario ID-SEC --count 2 --json"
    Then itos exits with code 0
    And its JSON's "stem" is "ID-SEC"
    And its JSON's "ids" is ["@ID-SEC-02", "@ID-SEC-03"]
    And its JSON's "claimed" is true
    And the remote's id counter for "ID-SEC" is 3
    When itos runs "tests next-id scenario ID-SEC"
    Then itos exits with code 0
    And its output says "@ID-SEC-04"

  # Under a pattern that takes lowercase areas, ID-slice, ID-bug and
  # ID-batch are areas, so rule 3 alone keeps slice, bug and batch (a raw
  # series, @batch-7) previews. The steps up to sec pass today and must
  # after; sec, which no series uses, is red before the work: it exits 0
  # printing @sec-1.
  @ID-NEXTID-10 @slice-157
  Scenario: Auxiliary series stay previews while an unused lowercase stem is corrected
    Given the tests kind "scenario" has the id pattern "ID-[A-Za-z]+-\d+"
    And a feature file with the scenario "@ID-SEC-02" tagged "@batch-7 @bug-4"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs "tests next-id scenario slice --json"
    Then itos exits with code 0
    And its JSON's "ids" is ["@slice-10"]
    And its JSON's "claimed" is false
    When itos runs "tests next-id scenario bug --json"
    Then itos exits with code 0
    And its JSON's "ids" is ["@bug-5"]
    And its JSON's "claimed" is false
    When itos runs "tests next-id scenario batch --json"
    Then itos exits with code 0
    And its JSON's "ids" is ["@batch-8"]
    And its JSON's "claimed" is false
    When itos runs "tests next-id scenario batch --count 2"
    Then itos exits with code 2
    And stdout is empty
    And its stderr does not say "ID-batch"
    When itos runs "tests next-id scenario sec"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "itos tests next-id scenario ID-sec"
    And the remote has no ref "refs/itos/ids"

  # The fixture kind is behind a command adapter that logs each call; its
  # listing is ID-SEC-01, a non-live F-002 and OPS-2. Red before the work:
  # SEC exits 0 printing SEC-1 (the kind has no tag prefix). OPS, a raw
  # series of the listing, stays a preview, as today.
  @ID-NEXTID-11 @slice-157
  Scenario: A command-adapter kind gets the same correction, its listing read once
    Given a command adapter lists live IDs "ID-SEC-01" and "OPS-2" in that order, with an intervening non-live test
    And the tests kind "fixture" has the id pattern "ID-[A-Z]+-\d+"
    When itos runs "tests next-id fixture SEC"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "itos tests next-id fixture ID-SEC"
    And the adapter was called once for "worktree"
    And the remote has no ref "refs/itos/ids"
    When itos runs "tests next-id fixture OPS"
    Then itos exits with code 0
    And its output says "OPS-3"

  # Red before the work: the first run exits 2 today with the --count
  # refusal, which names no correction; the second exits 0 printing @SEC-1.
  # Neither contacts the counter today, since SEC is no area. The
  # unreachable remote proves the refusal never contacts it: a run that did
  # would exit 3.
  @ID-NEXTID-12 @slice-157
  Scenario: The refusal keeps the JSON usage-error contract and never contacts the counter
    Given the remote cannot be reached
    When itos runs "tests next-id scenario SEC --count 2 --json"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "itos tests next-id scenario ID-SEC --count 2"
    And its stderr does not say "--json"
    When itos runs "tests next-id scenario SEC -q"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "itos tests next-id scenario ID-SEC"
    And its stderr does not say " -q"

  # The two readings of rule 1 differ on the first pattern: its text begins
  # with ID-, so SEC is corrected (red before the work: it exits 0 printing
  # @SEC-1). The second pattern's text begins with T-, so its ID-SEC is no
  # area and SEC keeps today's preview: that part passes today and must
  # after. Today's help names neither SEC nor the corrected command.
  @ID-NEXTID-13 @slice-157
  Scenario: Only a pattern whose text begins with ID- gets a correction, and help says so
    Given the tests kind "scenario" has the id pattern "ID-[A-Z]+-\d+|T-\d+-[a-z]+"
    When itos runs "tests next-id scenario SEC"
    Then itos exits with code 2
    And stdout is empty
    And its stderr says "itos tests next-id scenario ID-SEC"
    Given the tests kind "scenario" has the id pattern "T-\d+-[a-z]+|ID-[A-Z]+-\d+"
    When itos runs "tests next-id scenario SEC --json"
    Then itos exits with code 0
    And its JSON's "ids" is ["@SEC-1"]
    And its JSON's "claimed" is false
    And the remote has no ref "refs/itos/ids"
    When itos runs "help tests next-id"
    Then itos exits with code 0
    And its output says "itos tests next-id scenario SEC", whatever its line breaks
    And its output says "itos tests next-id scenario ID-SEC", whatever its line breaks
