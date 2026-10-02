@phase-1 @slice-1
Feature: Verification starts at commits.since
  A repository made from a GitHub template begins with one squashed commit
  that no rule passes, and a project adopting itos has a history written
  before its rules. commits.since in itos.yaml names the commit where
  verification starts: itos verify and every range check skip that commit and
  its ancestors, and itos config check rejects a value that is not the full
  SHA of a commit in the repository. The commit-msg hook, which judges one
  commit at a time, is unaffected.

  Background:
    Given a repository made from a template, its first commit "Initial commit"
    And the commit "docs: write the readme" on top of it

  @ID-SINCE-01
  Scenario: verify skips the commit commits.since names, and its ancestors
    Given commits.since names the first commit
    When itos verifies every commit up to HEAD
    Then itos exits with code 0
    And its output says "1/1 commits pass the commit rules"

  # The first commit's header has no type, which the header lint rejects.
  @ID-SINCE-02
  Scenario: Without commits.since, verify checks the root commit too
    When itos verifies every commit up to HEAD
    Then itos exits with code 1
    And its output says "1/2 commits pass the commit rules"
    And its output names the rule "type-empty"

  @ID-SINCE-03
  Scenario: A range check starts at the commit commits.since names
    Given a range check that records where its range starts
    And commits.since names the first commit
    When itos verifies every commit up to HEAD
    Then the range check started at the first commit

  @ID-SINCE-04
  Scenario: config check accepts a commits.since that names a commit of the repository
    Given commits.since names the first commit
    When itos checks the config
    Then itos exits with code 0

  @ID-SINCE-05
  Scenario: config check rejects a commits.since that is not a full SHA
    Given commits.since is "abc1234"
    When itos checks the config
    Then itos exits with code 2
    And its output says "commits.since is not the full SHA of a commit"

  @ID-SINCE-06
  Scenario: config check rejects a commits.since that names no commit of the repository
    Given commits.since is "0123456789abcdef0123456789abcdef01234567"
    When itos checks the config
    Then itos exits with code 2
    And its output says "which is not a commit of this repository"

  # actions/checkout clones one commit deep by default, so a nightly's
  # checkout lacked the commit commits.since names though the repository has
  # it. The clone is made after the config is written, with the config laid
  # into it: only its history is shorter.
  @ID-SINCE-07 @slice-17 @wip
  Scenario: In a shallow clone, config check says the commits.since it cannot find may be outside the clone
    Given commits.since names the first commit
    And itos runs in a clone of the repository one commit deep
    When itos checks the config
    Then itos exits with code 2
    And its output says "shallow"
    And its output says "git fetch --unshallow"

  @ID-SINCE-08 @slice-17 @wip
  Scenario: In a shallow clone, verify says the commits.since it cannot find may be outside the clone
    Given commits.since names the first commit
    And itos runs in a clone of the repository one commit deep
    When itos verifies every commit up to HEAD
    Then itos exits with code 2
    And its output says "shallow"
    And its output says "git fetch --unshallow"
