@phase-3
Feature: The pre-push hook verifies the commits it pushes
  A commit that skipped the commit-msg hook (git commit --no-verify, a clone
  without the hooks) was first judged by CI, once it was on the remote and no
  longer the person's to amend. itos hook pre-push now verifies the commits
  each pushed ref adds, as itos verify does (their messages, their paths and
  the range checks, commits.since left out): the ones after the remote's
  commit the ref replaces, or, for a new branch, the ones on no
  remote-tracking branch; a push that would replace a remote commit it does
  not build on is refused before that (slice 128). A failure refuses the push,
  prints verify's report and how to fix the commits (git commit --amend for
  the last one, git rebase -i for an earlier one), and runs none of
  hooks.pre_push's commands; only when every commit passes do they run, as
  before. It prints nothing for commits that pass, so a push whose commits
  pass reads as it did before this. CI still verifies every commit since its
  last green run: it is the gate nobody can skip on their own machine.

  A branch's own rewrite is the one exception to slice 128's refusal
  (slice-193): a pushed ref that replaces a remote commit it does not build
  on is accepted when the ref is a branch other than main, the branch the
  remote's HEAD names (asked of the remote, git ls-remote --symref, as itos
  push asks it), and every commit the remote's old head has that main lacks
  has a commit of the same patch-id in the new head's history (git cherry
  <new> <old> <main>, main as the clone's remote-tracking branch of it has
  it; with none, every commit of the old head the new one's history lacks),
  so a rewrite drops no work. itos push --onto-main makes such a rewrite
  (push-mode.feature), and a plain git push --force-with-lease of a branch
  rebased onto main passes too, since the rule is the hook's. Every other
  rewrite is refused as before: main's, a ref that is no branch, one whose
  remote commit the clone lacks, any when the remote's HEAD cannot be asked
  or names no branch it has, and one that would drop a commit, whose
  refusal names each such commit by its short SHA and subject. An accepted
  rewrite is judged as a new branch is: verify over the commits on no
  remote-tracking branch, and hooks.pre_push's per_base once per remote
  commit they grow from, so main's commits the rebased branch now holds,
  on the remote already, are not the pusher's to fix.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the pre-push hook is installed

  # The clone's commits are made with --no-verify, as a commit that skipped
  # the commit-msg hook is; a chore needs a Task footer.
  @ID-PREPUSH-01 @slice-46
  Scenario: A pushed commit that breaks the commit rules refuses the push, saying how to fix it
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "0/1 commits pass the commit rules"
    And its output says "git rebase -i"
    And the remote's branch does not have "chore: tidy the readme"

  # The scratch config has no hooks.pre_push, so the hook has nothing to run
  # but the verify; before this a push there failed on the missing key.
  @ID-PREPUSH-02 @slice-46
  Scenario: Commits that pass are pushed, the hook printing nothing for them
    Given the commit "chore: tidy the readme" naming the task "T-001"
    When itos runs "push"
    Then itos exits with code 0
    And its output does not say "commits pass the commit rules"
    And the remote's branch has "chore: tidy the readme"

  # The remote's commit breaks the rules too, but it is already there: only
  # what this push adds is the pusher's to fix.
  @ID-PREPUSH-03 @slice-46
  Scenario: Only the commits the push adds are judged, not the remote's
    Given the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the commit "chore: tidy the readme" naming the task "T-001"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch ends with "chore: tidy the docs" then "chore: tidy the readme", with no merge commit

  # The remote has no commit the new branch replaces; the commits the clone
  # shares with the remote's main are on origin/main, so only the new one is
  # judged: 0/1, not every commit of the history.
  @ID-PREPUSH-04 @slice-46
  Scenario: A new branch is judged by the commits no remote-tracking branch has
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    When git pushes HEAD to the remote's new branch "topic"
    Then git exits with code 1
    And its output says "0/1 commits pass the commit rules"
    And the remote has no branch "topic"

  @ID-PREPUSH-05 @slice-46
  Scenario: A commit that fails runs none of hooks.pre_push's commands
    Given hooks.pre_push's commands record that they ran
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And none of hooks.pre_push's commands ran

  # slice-193: an item's branch main has moved past is rebased onto main and
  # pushed with a lease, by itos push --onto-main or by hand as here; the
  # rewrite drops none of the branch's commits, so the hook accepts it.
  # Main's "chore: tidy the docs" has no Task footer, which the commit rules
  # want of a chore: the accepted rewrite is judged as a new branch is, by
  # the commits on no remote-tracking branch, and main's is on origin/main
  # once fetched, so only the rebased "docs: add the guide" is judged.
  # Fails before the work at the exit code: today the hook refuses every
  # rewrite (slice 128), exit 1.
  @ID-PREPUSH-07 @slice-193 @wip
  Scenario: A forced push of a branch rebased onto main, dropping none of its commits, is accepted
    Given the clone is on the new branch "topic"
    And the clone has the commit "docs: add the guide" touching "guide.md"
    And git pushes HEAD to the remote's new branch "topic"
    And the remote has gained the commit "chore: tidy the docs" touching "docs.md"
    And the clone has fetched the remote
    And the clone has rebased its branch onto the remote's main
    When the real git runs "push --force-with-lease origin HEAD:topic"
    Then the real git exits with code 0
    And the remote's branch "topic" ends with "chore: tidy the docs" then "docs: add the guide", with no merge commit

  # A rewrite of a branch that would drop a commit of the remote's head that
  # main lacks loses work, lease or not, so it stays refused, naming the
  # commit it would drop. Fails before the work at the commit's subject:
  # today the hook refuses it too, but names only the ref and the SHAs.
  @ID-PREPUSH-08 @slice-193 @wip
  Scenario: A forced push of a branch that would drop one of its remote commits is refused, naming the commit
    Given the clone is on the new branch "topic"
    And the clone has the commit "docs: add the guide" touching "guide.md"
    And git pushes HEAD to the remote's new branch "topic"
    And the clone has reset its branch to the remote's main
    And the clone has the commit "docs: add the index" touching "index.md"
    When the real git runs "push --force-with-lease origin HEAD:topic"
    Then the real git exits with code 1
    And its output says "docs: add the guide"
    And the remote has the branch "topic" with "docs: add the guide"
    And the remote's branch "topic" does not have "docs: add the index"

  # Main is never rewritten, even by a rewrite that drops no change: here a
  # reworded head, whose patch-id is the old one's. The patch-id rule alone
  # would accept it, since main lacks none of its own commits; the branch's
  # being main is what refuses it. A guard: refused today (slice 128) and
  # must stay refused.
  @ID-PREPUSH-09 @slice-193 @guard @wip
  Scenario: A forced push that rewrites main is refused though it drops no change
    Given the remote has gained the commit "docs: someone else's work" touching "OTHER.md"
    And the clone has fetched the remote
    And the clone has reset its branch to the remote's main
    And git amends HEAD with the message "docs: someone else's work, reworded"
    When the real git runs "push --force-with-lease origin HEAD:main"
    Then the real git exits with code 1
    And its output says "itos push"
    And the remote's branch ends with "docs: someone else's work"

  # Slice 128: a push that replaces a remote commit not in the pushed
  # commit's history rewrites what others may have built on. itos push never
  # forces, but git push --force past the shim (or with no shim) still runs
  # this hook, so the hook refuses any pushed ref whose remote commit is not
  # an ancestor of the commit pushed, or that the clone lacks, naming itos
  # push, which rebases instead, and none of hooks.pre_push's commands runs;
  # a branch's own rewrite that drops nothing, never main's, is slice-193's
  # exception (@ID-PREPUSH-07). A new branch has no remote commit and is
  # judged as before (@ID-PREPUSH-04).
  @ID-PREPUSH-06 @slice-128
  Scenario: A forced push that would replace a remote commit is refused, and the remote keeps it
    Given the remote has gained the commit "docs: someone else's work" touching "OTHER.md"
    And the clone has fetched the remote
    And the clone has the commit "docs: mine" touching "README.md"
    When the real git runs "push --force origin HEAD:main"
    Then the real git exits with code 1
    And its output says "itos push"
    And the remote's branch has "docs: someone else's work"
