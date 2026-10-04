@phase-1
Feature: itos ask, the questions waiting on the person a repository's work is for
  A coordinator held its questions for the user by hand, in the handoff's
  open questions and its review list, rewritten after every landing. itos
  ask keeps them as data beside the work registry (the user's calls,
  2026-10-04, p1-follow-ups): public and committed, unlike itos follow's
  private threads, since a question about the work belongs with it. A
  question gets the next free id (q-1, q-2, and so on), may name the item it
  holds up (--item, an id the registry has), and stays open until it is
  answered; its answer is kept beside it. Each command commits the questions
  file alone, as the registry's commands do (docs: ask q-1, docs: answer
  q-1), and under a stealth config writes it and commits nothing. itos ask
  with nothing after it lists the open questions.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-ASK-01 @slice-62
  Scenario: ask add records a question beside the registry, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask add 'Should the starter require a Task footer of docs commits?' --item slice-9"
    Then itos exits with code 0
    And its output says "q-1"
    And the last commit's header is "docs: ask q-1"
    And the last commit touches only "tasks/asks.yaml"

  @ID-ASK-02 @slice-62
  Scenario: ask lists the open questions, and an answered one is not among them
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask add 'Is the archive worth it?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    When itos runs "ask"
    Then itos exits with code 0
    And its output says "Is the archive worth it?"
    And its output does not say "Labels or Projects?"

  @ID-ASK-03 @slice-62
  Scenario: ask answer keeps the answer beside the question, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "ask add 'Labels or Projects?'"
    When itos runs the command line "ask answer q-1 'Labels, with trust by who acted.'"
    Then itos exits with code 0
    And the last commit's header is "docs: answer q-1"
    When itos runs "ask show q-1"
    Then its output says "Labels, with trust by who acted."

  @ID-ASK-04 @slice-62
  Scenario: ask answer refuses a question that does not exist
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask answer q-7 'Yes.'"
    Then itos exits with code 1
    And its output says "q-7"

  @ID-ASK-05 @slice-62
  Scenario: ask add refuses an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask add 'Which way?' --item slice-404"
    Then itos exits with code 1
    And its output says "slice-404"
    And the file "tasks/asks.yaml" does not exist

  # An answered question is a decision, but read where it was asked it is lost
  # among the questions, and a person cannot read a registry's whys. itos ask
  # record writes one, on demand, as an architecture decision record in
  # adr-tools' format (NNNN-slug.md; Date, Status, Context, Decision,
  # Consequences), in docs/adr/ or the folder a .adr-dir file names, as
  # adr-tools reads it, so a repository that already keeps ADRs keeps its
  # own; the next number is past the highest file there. Its README.md holds,
  # between itos's markers, the index of the decisions still live, so a
  # reader never wades through superseded ones. itos ask nudges toward it:
  # an answered question recorded nowhere else is named, until it is recorded
  # or marked --none, an answer that concerned its item alone (the user's
  # calls, 2026-10-04, p1-ask-record-decisions).
  @ID-ASK-06 @slice-69 @wip
  Scenario: ask record writes an answered question as the next decision record, and commits it with the question
    Given itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask answer q-1 'Labels, with trust by who acted.'"
    When itos runs the command line "ask record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And the file "docs/adr/0001-triage-issues-with-labels.md" says "# 1. Triage issues with labels"
    And the file "docs/adr/0001-triage-issues-with-labels.md" says "Labels or Projects?"
    And the file "docs/adr/0001-triage-issues-with-labels.md" says "Labels, with trust by who acted."
    And the file "docs/adr/0001-triage-issues-with-labels.md" says "Accepted"
    And the file "docs/adr/README.md" says "Triage issues with labels"
    And the last commit's header is "docs: record q-1 as decision 1"

  @ID-ASK-07 @slice-69 @wip
  Scenario: A decision that supersedes another leaves the index, and each file links the other
    Given itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    And itos has run the command line "ask record q-1 --title 'Triage issues with labels'"
    And itos has run the command line "ask add 'Labels still?'"
    And itos has run the command line "ask answer q-2 'Projects now.'"
    When itos runs the command line "ask record q-2 --title 'Triage issues with Projects' --supersedes 1"
    Then itos exits with code 0
    And the file "docs/adr/0001-triage-issues-with-labels.md" says "Superseded by"
    And the file "docs/adr/0002-triage-issues-with-projects.md" says "Supersedes"
    And the file "docs/adr/README.md" says "Triage issues with Projects"
    And the file "docs/adr/README.md" does not say "Triage issues with labels"

  @ID-ASK-08 @slice-69 @wip
  Scenario: ask names the answered questions recorded nowhere else
    Given itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    When itos runs "ask"
    Then itos exits with code 0
    And its output says "itos ask record q-1"

  @ID-ASK-09 @slice-69 @wip
  Scenario: An answer marked --none writes no record and stops the nudge
    Given itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    And itos has run the command line "ask record q-1 --none"
    When itos runs "ask"
    Then itos exits with code 0
    And its output does not say "itos ask record"
    And the file "docs/adr/README.md" does not exist

  @ID-ASK-10 @slice-69 @wip
  Scenario: ask record refuses a question not yet answered
    Given itos has run the command line "ask add 'Labels or Projects?'"
    When itos runs the command line "ask record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 1
    And its output says "q-1"
    And the file "docs/adr/README.md" does not exist

  @ID-ASK-11 @slice-69 @wip
  Scenario: The folder a .adr-dir names, and the records already in it, set where the next one goes
    Given the committed file ".adr-dir" holding "doc/decisions"
    And the committed file "doc/decisions/0007-use-go.md" holding "# 7. Use Go"
    And itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    When itos runs the command line "ask record q-1 --title 'Triage issues with labels'"
    Then itos exits with code 0
    And the file "doc/decisions/0008-triage-issues-with-labels.md" says "# 8. Triage issues with labels"
