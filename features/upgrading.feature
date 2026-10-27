@phase-3
Feature: A footer that says what a consumer changes
  A release's Upgrading section was written by reading the commits after
  the fact, and nothing could tell when it missed one: the notes check sees
  changed defaults, not a behaviour change. A footer whose source is free
  text records it where the change is made: commits.footers.<name> with
  source: text, required for the types the config names, carrying either
  what a consumer must do or the word none. It is required only of commits
  after the footer's since, so a history written before the rule still
  verifies; the release then gathers the range's footers into its notes.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a change to "README.md" is staged
    And the config requires an "Upgrading" footer of free text for "chore"

  @ID-UPGRADE-01 @slice-26
  Scenario: A commit without the free-text footer its type needs is rejected, naming the footer
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "Upgrading"

  @ID-UPGRADE-02 @slice-26
  Scenario: The word none is a free-text footer that says a consumer changes nothing
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      Upgrading: none
      """
    Then itos exits with code 0

  @ID-UPGRADE-03 @slice-26
  Scenario: A free-text footer is accepted with whatever it says
    When the commit-msg hook checks the message:
      """
      chore: tidy the readme

      Task: T-001
      Upgrading: rename commits.lint to commits.header_lint
      """
    Then itos exits with code 0

  # Without a since of its own, adding a required footer would turn verify
  # red on every commit written before the rule existed.
  @ID-UPGRADE-04 @slice-26
  Scenario: verify does not require the footer of a commit before the footer's since
    Given the commit "chore: tidy the readme" naming the task "T-001"
    And the "Upgrading" footer is required only after HEAD
    When itos verifies every commit up to HEAD
    Then itos exits with code 0

  # The release gathers what each commit asked of a consumer: the footers of
  # the range, one per line with the commit it came from; a none asks nothing.
  @ID-UPGRADE-05 @slice-26
  Scenario: itos lists a range's free-text footers with their commits, skipping those that say none
    Given the commit "chore: rename the key" naming the task "T-001" with the footer "Upgrading: rename commits.lint to commits.header_lint"
    And the commit "chore: tidy the readme" naming the task "T-001" with the footer "Upgrading: none"
    When itos lists the "Upgrading" footers of the commits after the first
    Then itos exits with code 0
    And its output says "rename commits.lint to commits.header_lint"
    And its output does not say "none"

  # Bug 151 (found 2026-10-27 by T-328's FuzzTrailersMatchGit, in
  # internal/message/trailers_fuzz_test.go): the message "0\n\nChanges:\t\f\n"
  # with the comment character # is [{Changes "\f"}] to git interpret-trailers
  # and [{Changes ""}] to itos. trailers.go's contract is the footers as git
  # reads a message's trailers (bug 110), so the fix trims as git trims.
  #
  # The trim set is git's. trailer.c trims a trailer's key and value with
  # strbuf_trim (parse_trailer), and the unfolded value again (unfold_value,
  # which also drops the isspace run after each line break); strbuf_trim
  # trims git's isspace, sane_istest(x, GIT_SPACE), and git's sane_ctype
  # table (ctype.c) marks only bytes 9, 10 and 13 (Z, GIT_CNTRL | GIT_SPACE)
  # and 32 (S) as space: tab, line feed, carriage return and the space. Form
  # feed and vertical tab (12, 11) are X, control only, and no byte above
  # 127 is space, so String.trim's other characters (form feed, vertical
  # tab, no-break space U+00A0, U+1680, U+2000 to U+200A, U+2028, U+2029,
  # U+202F, U+205F, U+3000 and the BOM U+FEFF) are kept where they stand. A
  # value made only of them is a value, not empty, and says what it says.
  #
  # Where: unfolded (trailers.go) ends with value.Trim, String.trim's set,
  # where parseTrailers already trims git's (gitSpace's " \t\n\r"); the fix
  # trims the unfolded value with git's set. value.Trim stays as it is for
  # its other callers (the header lint's, which is commitlint's JavaScript,
  # the config's, the ledger's, the registry's); only the footers' trim
  # changes. Every reader of a free-text footer's value goes through Texts,
  # so each changes with it:
  # - Gathered (itos commit footers, a release's notes): a footer of only a
  #   form feed or a no-break space was empty and left out, and is listed;
  #   one of none and a no-break space was none and left out, and is listed,
  #   since git does not read it as the word none; a text's trailing no-break
  #   space is printed, as git keeps it (ID-UPGRADE-06, ID-UPGRADE-07).
  # - checkText (the footer rules of the commit-msg hook, check-message and
  #   verify): such a footer satisfies required_for, and validate_for does
  #   not call it empty (ID-UPGRADE-08). Missing, itos commit's required-
  #   footer check before git runs, reads it the same way.
  # - Written.Against, itos commit's comparison of a flag's footer with the
  #   message's (bug 138): a message's text with a trailing no-break space
  #   is no longer the flag's text without it, so the commit is refused,
  #   naming both (ID-COMMITCMD-40, commit-command.feature).
  # - readCommitFlags (internal/slices/commit/git.go), the flag's side:
  #   it trims each --breaking and --<text footer> value with
  #   strings.TrimSpace, Go's Unicode set (U+0085 and U+00A0 among it).
  #   A flag's value becomes a trailer git reads, so it is trimmed with
  #   git's set too (space, tab, line feed, carriage return), and both sides
  #   of Written.Against trim alike: the same text ending in a no-break
  #   space, in the flag and in the message, is one footer (ID-COMMITCMD-41).
  #   A --upgrading or --breaking of only a no-break space is a value, not
  #   refused as empty, and is written as git would read it; a value empty
  #   after git's trim stays the usage error it is.
  # - typedFooter, the stealth mode's refusal, only counts the footers, and
  #   does not change. The footers of IDs are split by rawIDs on
  #   value.SpaceChars and commas, never through unfolded, and do not change.
  #
  # The last release's promises: go run ./tools/bin/previous-release keeps
  # every promise of v8.25.2 (1197 scenarios, 921 corpus cases), and none of
  # its feature files or corpus cases holds a vertical tab, a form feed or
  # one of String.trim's characters above 127, raw or escaped, so no case
  # changes: the fix carries no Changes: footer.
  #
  # The fix commits the fuzzer's input as a seed, so the push tier guards it
  # (ADR-0051): internal/message/testdata/fuzz/FuzzTrailersMatchGit/
  # value-of-a-form-feed, in go test fuzz v1's format, the arguments that
  # rebuild the message as fuzzMessage builds it: string("0"), string(""),
  # string("\f"), []byte("\x041") (kind 4, Changes; argument 0x31, the
  # separator ":\t" and one byte of text) and uint8(0) (comment character #).
  #
  # Steps. The new Given "the commit <header> naming the task <id> with the
  # footer <footer>, its backslash escapes read as Go's" is the existing one
  # (commitOnTop: the header, a blank line, Task: <id>, the footer) with the
  # footer read as the body of a Go string literal (strconv.Unquote of it in
  # double quotes: \t, \f and \u00a0 are those characters, \" a quote). git
  # commit's cleanup trims a line's end of git's isspace alone, so the form
  # feed and the no-break space reach the commit. The new Then "its output
  # says <text>, its backslash escapes read as Go's" is "its output says"
  # with the text read the same way.
  #
  # The fuzzer's value, its key Upgrading. Before the fix Texts reads it as
  # "" and Gathered leaves the footer out, so itos prints nothing: it fails
  # at its output says "\f".
  @ID-UPGRADE-06 @bug-151 @wip
  Scenario: A free-text footer whose value is only a form feed is listed, as git reads one
    Given the commit "chore: keep the feed" naming the task "T-001" with the footer "Upgrading:\t\f", its backslash escapes read as Go's
    When itos lists the "Upgrading" footers of the commits after the first
    Then itos exits with code 0
    And its output says "\f", its backslash escapes read as Go's

  # itos commit footers prints a line "<short sha> <text>" a footer. Before
  # the fix the text loses its no-break space and the line ends "lint\n": it
  # fails at its output says "rename commits.lint\u00a0\n".
  @ID-UPGRADE-07 @bug-151 @wip
  Scenario: A free-text footer keeps a trailing no-break space, as git keeps it
    Given the commit "chore: rename the key" naming the task "T-001" with the footer "Upgrading: rename commits.lint\u00a0", its backslash escapes read as Go's
    When itos lists the "Upgrading" footers of the commits after the first
    Then itos exits with code 0
    And its output says "rename commits.lint\u00a0\n", its backslash escapes read as Go's

  # verify reads each commit after the first through the footer rules,
  # Upgrading required for chore. Before the fix the footer's text is "", so
  # checkText says "chore commits need an Upgrading footer": it fails at
  # itos exits with code 0 (it exits 1).
  @ID-UPGRADE-08 @bug-151 @wip
  Scenario: A free-text footer whose value is only a no-break space satisfies the type that requires it
    Given the commit "chore: tidy the readme" naming the task "T-001" with the footer "Upgrading: \u00a0", its backslash escapes read as Go's
    When itos verifies the commits after the first
    Then itos exits with code 0
