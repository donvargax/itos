@phase-1
Feature: itos go and itos guide, the guides a session starts from
  A session that coordinates a repository's work followed a guide kept in
  that repository, written for it and copied nowhere else, so what one
  repository learnt about running agents never reached another (the user's
  calls, 2026-10-04, p3-coordinator-skill). The guides now ship in the
  binary, generic and versioned with itos: itos go prints the coordinator's
  guide, the command a person starts every session with (! itos go), and
  itos guide work prints the implementer's, which a coordinator asks each
  agent it starts to run first. What a repository learns that holds for no
  other stays its own: itos go appends docs/ORCHESTRATING.md after the
  generic guide when the repository has one (guide.orchestrating in the
  config names another file, under the git folder for a stealth config).
  They read and print, never write, and need no config: any git repository
  will do. The plugin's skills wrapping them is a later step.

  @ID-GUIDE-01 @slice-64 @wip
  Scenario: itos go prints the coordinator's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos"

  @ID-GUIDE-02 @slice-64 @wip
  Scenario: itos go appends the repository's own orchestrating notes after the guide
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "docs/ORCHESTRATING.md" holding "Our own lesson: the inbox is read first."
    When itos runs "go"
    Then itos exits with code 0
    And its output says "# Coordinating with itos" before "Our own lesson: the inbox is read first."

  @ID-GUIDE-03 @slice-64 @wip
  Scenario: guide.orchestrating names the file itos go appends
    Given a repository whose ledger has the task "T-001"
    And the committed file "notes/agents.md" holding "Kept elsewhere."
    And guide.orchestrating is "notes/agents.md"
    When itos runs "go"
    Then itos exits with code 0
    And its output says "Kept elsewhere."

  @ID-GUIDE-04 @slice-64 @wip
  Scenario: itos guide work prints the implementer's guide
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide work"
    Then itos exits with code 0
    And its output says "# Working with itos"

  @ID-GUIDE-05 @slice-64 @wip
  Scenario: itos guide refuses a guide it does not have, naming those it has
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "guide review"
    Then itos exits with code 2
    And its output says "review"
    And its output says "coordinate, work"
