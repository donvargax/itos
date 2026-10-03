@phase-3
Feature: The pre-push hook verifies the commits it pushes
  A commit that skipped the commit-msg hook (git commit --no-verify, a clone
  without the hooks) was first judged by CI, once it was on the remote and no
  longer the person's to amend. itos hook pre-push now verifies the commits
  each pushed ref adds, as itos verify does (their messages, their paths and
  the range checks, commits.since left out): the ones after the remote's
  commit the ref replaces, or, for a new branch or a remote commit the clone
  lacks, the ones on no remote-tracking branch. A failure refuses the push,
  prints verify's report and how to fix the commits (git commit --amend for
  the last one, git rebase -i for an earlier one), and runs none of
  hooks.pre_push's commands; only when every commit passes do they run, as
  before. It prints nothing for commits that pass, so a push whose commits
  pass reads as it did before this. CI still verifies every commit since its
  last green run: it is the gate nobody can skip on their own machine.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the pre-push hook is installed

  # The clone's commits are made with --no-verify, as a commit that skipped
  # the commit-msg hook is; a chore needs a Task footer.
  @ID-PREPUSH-01 @slice-46 @wip
  Scenario: A pushed commit that breaks the commit rules refuses the push, saying how to fix it
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "0/1 commits pass the commit rules"
    And its output says "git rebase -i"
    And the remote's branch does not have "chore: tidy the readme"

  # The scratch config has no hooks.pre_push, so the hook has nothing to run
  # but the verify; before this a push there failed on the missing key.
  @ID-PREPUSH-02 @slice-46 @wip
  Scenario: Commits that pass are pushed, the hook printing nothing for them
    Given the commit "chore: tidy the readme" naming the task "T-001"
    When itos runs "push"
    Then itos exits with code 0
    And its output does not say "commits pass the commit rules"
    And the remote's branch has "chore: tidy the readme"

  # The remote's commit breaks the rules too, but it is already there: only
  # what this push adds is the pusher's to fix.
  @ID-PREPUSH-03 @slice-46 @wip
  Scenario: Only the commits the push adds are judged, not the remote's
    Given the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the commit "chore: tidy the readme" naming the task "T-001"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the docs" then "chore: tidy the readme", with no merge commit

  # The remote has no commit the new branch replaces; the commits the clone
  # shares with the remote's main are on origin/main, so only the new one is
  # judged: 0/1, not every commit of the history.
  @ID-PREPUSH-04 @slice-46 @wip
  Scenario: A new branch is judged by the commits no remote-tracking branch has
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    When git pushes HEAD to the remote's new branch "topic"
    Then git exits with code 1
    And its output says "0/1 commits pass the commit rules"
    And the remote has no branch "topic"

  @ID-PREPUSH-05 @slice-46 @wip
  Scenario: A commit that fails runs none of hooks.pre_push's commands
    Given hooks.pre_push's commands record that they ran
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And none of hooks.pre_push's commands ran
