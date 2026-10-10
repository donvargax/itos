@phase-1
Feature: The commit-msg hook validates itos's own data
  itos.yaml, the ledger, the work registry and the smoke set are itos's data,
  and a project's pre-commit hook passes them as prose. So itos hook
  commit-msg validates them itself, from the staged tree as it reads the
  footers: the config check when any of them is staged, the registry's check
  when the registry is, and the commit is rejected with the problems they
  print. A commit that stages none of them runs neither check.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-CDATA-01 @slice-6
  Scenario: A staged registry with a problem is rejected, with the problem
    Given the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "lost" is staged
    When the commit-msg hook checks the message "docs: plan the work"
    Then itos exits with code 1
    And its output says "unknown status"

  @ID-CDATA-02 @slice-6
  Scenario: A staged itos.yaml with an unknown key is rejected, naming the key
    Given an itos.yaml with the unknown key "colour" is staged
    When the commit-msg hook checks the message:
      """
      build: colour the config

      Task: T-001
      """
    Then itos exits with code 1
    And its output says "unknown key colour"

  @ID-CDATA-03 @slice-6
  Scenario: A staged ledger with an unknown key is rejected, naming the key
    Given a ledger whose task "T-001" has the unknown key "titel" is staged
    When the commit-msg hook checks the message "docs: retitle the task"
    Then itos exits with code 1
    And its output says "titel"

  # The hook judges what is committed: the working tree's copy, broken after
  # staging, is never part of the commit.
  @ID-CDATA-04 @slice-6
  Scenario: The registry is judged as staged, not as it is on disk
    Given the work registry at "tasks/work-items.yaml" with the item "T-001" with the status "doing" is staged
    And the working tree's "tasks/work-items.yaml" sets the item "T-001" to the status "lost"
    When the commit-msg hook checks the message "docs: plan the work"
    Then itos exits with code 0

  # A broken registry already committed shows that no check ran: either check
  # would have rejected it.
  @ID-CDATA-05 @slice-6
  Scenario: A commit that stages none of itos's data runs neither check
    Given the commit of a registry at "tasks/work-items.yaml" with the item "T-001" with the status "lost"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message "docs: write the readme"
    Then itos exits with code 0

  # Bug 9: on windows the registry beside a ledger was named with a backslash
  # (work\work-items.yaml), which no staged path matched, git naming them with
  # slashes, so a staged registry was never checked. T-072's windows run found
  # it.
  @ID-CDATA-06 @bug-9
  Scenario: A staged registry beside a ledger kept in another folder is rejected, with the problem
    Given the ledger's files are "work/phase-{group}.yaml"
    And the commit "chore: keep the ledger in work" on top of it
    And the work registry at "work/work-items.yaml" with the item "T-001" with the status "lost" is staged
    When the commit-msg hook checks the message "docs: plan the work"
    Then itos exits with code 1
    And its output says "unknown status"

  # Slice 117 (p1-smoke-rule-at-commit, queued by the backlog review of
  # 2026-10-10). The smoke rule reads a kind's test files as well as its
  # smoke set: every file with a live test has a smoke test (unless
  # smoke.every_file is false), and every smoke ID is live. A feat that makes
  # a new file's scenarios live without a smoke entry stages no smoke file, so
  # the hook's data check never ran and CI's smoke step was the first to say
  # so: slice 82's feat went red that way (e5fce6ae), a red round trip on the
  # correct path. Now, when a commit stages a file a kind's test files match
  # (tests.<kind>), the hook also runs that kind's smoke check, as itos tests
  # smoke check judges it, over the staged tree, and rejects the commit with
  # its problems. A commit staging no test file of a kind runs no smoke check
  # for it.
  @ID-CDATA-07 @slice-117 @wip
  Scenario: A staged feature file whose live scenarios have no smoke entry is rejected, naming the file
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And a feature file "features/b.feature" with the live scenario "@ID-B-01" is staged
    When the commit-msg hook checks the message:
      """
      feat: add b

      Scenarios: @ID-B-01
      Upgrading: none
      """
    Then itos exits with code 1
    And its output says "features/b.feature"

  @ID-CDATA-08 @slice-117 @wip
  Scenario: The same commit staging the smoke entry for the new file passes
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And a feature file "features/b.feature" with the live scenario "@ID-B-01" is staged
    And the smoke set listing "@ID-A-01" and "@ID-B-01" is staged
    When the commit-msg hook checks the message:
      """
      feat: add b

      Scenarios: @ID-B-01
      Upgrading: none
      """
    Then itos exits with code 0

  # As CDATA-04: what is committed is judged, and a smoke entry left unstaged
  # is not part of the commit.
  @ID-CDATA-09 @slice-117 @wip
  Scenario: The smoke rule is judged on the staged tree, not on disk
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01"
    And the smoke set lists only "@ID-A-01"
    And a feature file "features/b.feature" with the live scenario "@ID-B-01" is staged
    And the working tree's smoke set lists "@ID-A-01" and "@ID-B-01", unstaged
    When the commit-msg hook checks the message:
      """
      feat: add b

      Scenarios: @ID-B-01
      Upgrading: none
      """
    Then itos exits with code 1
    And its output says "features/b.feature"

  # A committed break shows no smoke check ran: it would have rejected the
  # commit.
  @ID-CDATA-10 @slice-117 @wip
  Scenario: A commit that stages no test file of a kind runs no smoke check for it
    Given the committed feature file "features/a.feature" with the live scenario "@ID-A-01"
    And the committed feature file "features/b.feature" with the live scenario "@ID-B-01"
    And the smoke set lists only "@ID-A-01"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message "docs: write the readme"
    Then itos exits with code 0
