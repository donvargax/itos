@phase-1
Feature: itos question, the questions waiting on the person a repository's work is for
  A coordinator held its questions for the user by hand, in the handoff's
  open questions and its review list, rewritten after every landing. itos
  ask keeps them as data beside the work registry (the user's calls,
  2026-10-04, p1-follow-ups): public and committed, unlike itos followup's
  private threads, since a question about the work belongs with it. A
  question gets the next free id (q-1, q-2, and so on), may name the item it
  holds up (--item, an id the registry has), and stays open until it is
  answered; its answer is kept beside it. Each command commits the questions
  file alone, as the registry's commands do (docs: ask q-1, docs: answer
  q-1), and under a stealth config writes it and commits nothing. itos question
  with nothing after it lists the open questions.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-ASK-01 @slice-62
  Scenario: question add records a question beside the registry, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "question add 'Should the starter require a Task footer of docs commits?' --item slice-9"
    Then itos exits with code 0
    And its output says "q-1"
    And the last commit's header is "docs: ask q-1"
    And the last commit touches only "tasks/asks.yaml"

  @ID-ASK-02 @slice-62
  Scenario: ask lists the open questions, and an answered one is not among them
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question add 'Is the archive worth it?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs "question"
    Then itos exits with code 0
    And its output says "Is the archive worth it?"
    And its output does not say "Labels or Projects?"

  @ID-ASK-03 @slice-62
  Scenario: question answer keeps the answer beside the question, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "question add 'Labels or Projects?'"
    When itos runs the command line "question answer q-1 'Labels, with trust by who acted.'"
    Then itos exits with code 0
    And the last commit's header is "docs: answer q-1"
    When itos runs "question show q-1"
    Then its output says "Labels, with trust by who acted."

  @ID-ASK-04 @slice-62
  Scenario: question answer refuses a question that does not exist
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "question answer q-7 'Yes.'"
    Then itos exits with code 1
    And its output says "q-7"

  @ID-ASK-05 @slice-62
  Scenario: question add refuses an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "question add 'Which way?' --item slice-404"
    Then itos exits with code 1
    And its output says "slice-404"
    And the file "tasks/asks.yaml" does not exist

  # An answered question is a decision, but read where it was asked it is lost
  # among the questions, and a person cannot read a registry's whys. itos question
  # record writes one, on demand, as an architecture decision record in
  # docs/decisions or the folder work.decisions names (its format is MADR's,
  # below); the next number is past the highest file there. Its README.md
  # holds, between itos's markers, the index of the decisions that stand, so
  # a reader never wades through superseded ones. itos question nudges toward it:
  # an answered question recorded nowhere else is named, until it is recorded
  # or marked --none, an answer that concerned its item alone (the user's
  # calls, 2026-10-04, p1-ask-record-decisions).
  @ID-ASK-08 @slice-69
  Scenario: ask names the answered questions recorded nowhere else
    Given itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs "question"
    Then itos exits with code 0
    And its output says "itos question record q-1"

  @ID-ASK-09 @slice-69
  Scenario: An answer marked --none writes no record and stops the nudge
    Given itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    And itos has run the command line "question record q-1 --none"
    When itos runs "question"
    Then itos exits with code 0
    And its output does not say "itos question record"
    And the file "docs/decisions/README.md" does not exist

  @ID-ASK-10 @slice-69
  Scenario: question record refuses a question not yet answered
    Given itos has run the command line "question add 'Labels or Projects?'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 1
    And its output says "q-1"
    And the file "docs/decisions/README.md" does not exist

  # A project's pre-commit hook may format what it commits, as this
  # repository's vp staged does. itos ask record's index block was not
  # formatter-stable (the user's call of 2026-10-03: what itos writes is),
  # so the formatter rewrote it at every record; and since itos commits its
  # own files with git commit --only, the formatted file went into the commit
  # and the working tree while the index kept itos's copy, so the next itos
  # push refused the uncommitted change (found 2026-10-04, recording
  # decision 1). Every command that commits itos's files alone shares that
  # commit, so the second scenario holds for each of them.
  @ID-ASK-12 @bug-17
  Scenario: question record writes the index as a Markdown formatter leaves it, a blank line inside each marker
    Given itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And in the file "docs/decisions/README.md" the line after "<!-- itos:decisions:begin -->" is blank
    And in the file "docs/decisions/README.md" the line before "<!-- itos:decisions:end -->" is blank

  @ID-ASK-13 @bug-17
  Scenario: When a pre-commit hook rewrites the files itos commits, the index is left as the commit has them
    Given a pre-commit hook that appends a line to each staged Markdown file and stages it again
    And itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And git reports no change to the working tree or the index

  # Decision records are MADR 4's (adr/madr), not adr-tools' Nygard format
  # (the user's calls, 2026-10-04, q-9 and q-10, p1-adr-madr): the maintained
  # template, and the closer fit to a question and its answer. A record is
  # NNNN-slug.md in docs/decisions, MADR's own folder, or the folder
  # work.decisions names; optional YAML frontmatter holds its status and
  # date, which no section repeats; the bare-minimal sections hold the
  # question (Context and Problem Statement), the options (Considered
  # Options), the answer (Decision Outcome) and its consequences. A record
  # superseded says so in its status, and the index lists the accepted ones
  # only. This replaced the Nygard records of v2.28.0, which no one used, in
  # v3.0.0.
  @ID-ASK-14 @slice-71
  Scenario: question record writes an answered question as the next MADR record in docs/decisions
    Given itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels, with trust by who acted.'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And the file "docs/decisions/0001-triage-issues-with-labels.md" has the line "status: accepted"
    And the file "docs/decisions/0001-triage-issues-with-labels.md" has the line "# Triage issues with labels"
    And the file "docs/decisions/0001-triage-issues-with-labels.md" has the line "## Decision Outcome" after the line "## Context and Problem Statement"
    And the file "docs/decisions/0001-triage-issues-with-labels.md" says "Labels or Projects?"
    And the file "docs/decisions/0001-triage-issues-with-labels.md" says "Labels, with trust by who acted."
    And the file "docs/decisions/README.md" says "Triage issues with labels"
    And the last commit's header is "docs: record q-1 as decision 1"

  @ID-ASK-15 @slice-71
  Scenario: A superseded record says so in its status and leaves the index
    Given itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    And itos has run the command line "question record q-1 --title 'Triage issues with labels'"
    And itos has run the command line "question add 'Labels still?'"
    And itos has run the command line "question answer q-2 'Projects now.'"
    When itos runs the command line "question record q-2 --title 'Triage issues with Projects' --supersedes 1"
    Then itos exits with code 0
    And the file "docs/decisions/0001-triage-issues-with-labels.md" has the line "status: superseded by ADR-0002"
    And the file "docs/decisions/README.md" says "Triage issues with Projects"
    And the file "docs/decisions/README.md" does not say "Triage issues with labels"

  @ID-ASK-16 @slice-71
  Scenario: work.decisions names the folder, and the records already in it set the next number
    Given work.decisions is "notes/decisions"
    And the committed file "notes/decisions/0007-use-go.md" holding "# Use Go"
    And itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And the file "notes/decisions/0008-triage-issues-with-labels.md" has the line "# Triage issues with labels"

  # The commits itos makes of its own files (the registry's, ask's, task
  # add's) wrapped their bodies with a plain wrap, so a question or a why
  # could start a line with a word the header lint reads as a footer, and
  # the commit drew its footer-leading-blank warning (docs: answer q-10,
  # 2026-10-04; work done's run address before it). They wrap as itos commit
  # does since bug 15, which never starts a line with a footer token
  # (p1-done-body-url-footer).
  @ID-ASK-17 @bug-18
  Scenario: question add's commit body never starts a line with a word the lint reads as a footer
    When itos runs question add with a question whose commit body would wrap to start a line with "recommendation: hold it."
    Then itos exits with code 0
    And no line of HEAD's message starts with "recommendation:"
    And the message of HEAD says "recommendation: hold it."

  # MADR makes a record's frontmatter optional, but the index listed only
  # records whose status says accepted, so a repository whose records carry
  # no frontmatter got an empty index (found by slice 71's agent). A record
  # is live unless its status says otherwise (superseded, deprecated or
  # rejected), by decision 1 (p1-decisions-no-frontmatter).
  @ID-ASK-18 @bug-19
  Scenario: A record with no frontmatter is listed in the index as live
    Given the committed file "docs/decisions/0001-use-go.md" holding "# Use Go"
    And itos has run the command line "question add 'Labels or Projects?'"
    And itos has run the command line "question answer q-1 'Labels.'"
    When itos runs the command line "question record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And the file "docs/decisions/README.md" says "Use Go"
    And the file "docs/decisions/README.md" says "Triage issues with labels"

  # Bug 22 (found 2026-10-04 recording q-12): ask record's commit body names
  # the record's file, docs/decisions/<nnnn>-<slug>.md, on one line, so a
  # title of about 120 characters made that line longer than the header
  # lint's 100, the commit-msg hook refused it, and ask record put
  # everything back. The body keeps every line within the limit, as itos
  # commit wraps a body (slice 58); the record's file name and title are
  # unchanged.
  @ID-ASK-19 @bug-22
  Scenario: question record of a long title commits a body whose every line is within the limit
    Given itos has run the command line "question add 'Where does how to work with itos live?'"
    And itos has run the command line "question answer q-1 'In its own guides.'"
    When itos runs the command line "question record q-1 --title 'itos init generates the config rules into a marked block of AGENTS.md, and the guides of itos say how to work with it'"
    Then itos exits with code 0
    And the last commit's body has no line longer than 100 characters

  # Slice 89 (decision of q-16): "I need to ask someone this" was logged
  # with itos ask, whose questions are for the person the work is for, when
  # it meant itos follow: an agent picks a command by its name, and the
  # everyday verb pointed the wrong way. The questions are itos question;
  # asks.yaml, work.asks, the q-<n> ids and the docs: ask q-<n> headers stay.
  # The feat rewrites the live scenarios that run ask, marked breaking.
  @ID-ASK-20 @slice-89
  Scenario: question add records a question beside the registry, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "question add 'Should the starter require a Task footer of docs commits?' --item slice-9"
    Then itos exits with code 0
    And its output says "q-1"
    And the last commit touches only "tasks/asks.yaml"

  @ID-ASK-21 @slice-89
  Scenario: itos ask exits 2, naming itos question and itos followup
    When itos runs "ask"
    Then itos exits with code 2
    And its output says "itos question"
    And its output says "itos followup"
