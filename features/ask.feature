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

  @ID-ASK-01 @slice-62 @wip
  Scenario: ask add records a question beside the registry, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask add 'Should the starter require a Task footer of docs commits?' --item slice-9"
    Then itos exits with code 0
    And its output says "q-1"
    And the last commit's header is "docs: ask q-1"
    And the last commit touches only "tasks/asks.yaml"

  @ID-ASK-02 @slice-62 @wip
  Scenario: ask lists the open questions, and an answered one is not among them
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "ask add 'Labels or Projects?'"
    And itos has run the command line "ask add 'Is the archive worth it?'"
    And itos has run the command line "ask answer q-1 'Labels.'"
    When itos runs "ask"
    Then itos exits with code 0
    And its output says "Is the archive worth it?"
    And its output does not say "Labels or Projects?"

  @ID-ASK-03 @slice-62 @wip
  Scenario: ask answer keeps the answer beside the question, and commits it alone
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    And itos has run the command line "ask add 'Labels or Projects?'"
    When itos runs the command line "ask answer q-1 'Labels, with trust by who acted.'"
    Then itos exits with code 0
    And the last commit's header is "docs: answer q-1"
    When itos runs "ask show q-1"
    Then its output says "Labels, with trust by who acted."

  @ID-ASK-04 @slice-62 @wip
  Scenario: ask answer refuses a question that does not exist
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask answer q-7 'Yes.'"
    Then itos exits with code 1
    And its output says "q-7"

  @ID-ASK-05 @slice-62 @wip
  Scenario: ask add refuses an item the registry does not have
    Given the work registry has the item "slice-9" owned by nobody with the status "todo"
    When itos runs the command line "ask add 'Which way?' --item slice-404"
    Then itos exits with code 1
    And its output says "slice-404"
    And the file "tasks/asks.yaml" does not exist
