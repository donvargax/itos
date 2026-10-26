@phase-1
Feature: Under push.mode pr, a commit not yet on main may name a task main's ledger holds
  The ledger footer (Task:) is read at the commit (read_at: commit): a
  commit is judged against the ledger it carries. Under push.mode pr the
  coordinator adds tasks on main (task add goes straight to main, slice-177),
  so a task added while an item's branch is open is on main and not on the
  branch. Seen 2026-10-26 on slice-187: a build commit on the branch naming
  T-313, the item's turn-on task, was refused, "unknown tasks: T-313
  [task-footer]". The way round it, rebasing the branch onto main, then itos
  push (which rebases onto the remote's item branch), left copies of main's
  registry commits on the branch; GitHub's rebase of the pull request then
  failed and it took a force-push by hand. Every turn-on task added mid-item
  meets this.

  The rule (slice-189): under push.mode pr, a ledger footer's ID that the
  commit's own ledger does not have is known when the remote's main, as the
  clone last fetched it, has it in its ledger, and the commit is not on that
  main. Main is the branch the remote's HEAD names as the clone records it
  (refs/remotes/<remote>/HEAD), read at refs/remotes/<remote>/<main>; the
  remote is the one itos push sends to, the branch's upstream's, else
  origin. Nothing is fetched and no remote is asked: the commit-msg hook
  stays offline. Without that ref, or with a main that lacks the ID too, the
  footer is judged as today, with today's words. The rule is the footer
  rule's, so the commit-msg hook, itos commit check-message, the pre-push
  hook's judgement of a pushed range and itos verify all apply it. It
  covers only footers read at the commit (read_at: commit); one read at the
  working tree is judged as today. A clone without refs/remotes/<remote>/HEAD
  has no main for the rule: it does not fall back to <remote>/main.

  Why accept, not refuse with a hint. Under pr a commit reaches main only by
  the pull request's rebase-merge onto main's head (ADR-0039), and main
  never drops a commit, so a task already on the remote's main is in the
  ledger of every commit the merge makes; nothing on main lacks it.
  Refusing has no fix that keeps the branch clean: any copy of main's ledger
  commit on the branch (a rebase onto main, a cherry-pick, the file checked
  out from main) is what GitHub's rebase failed on, and a merge of main into
  the branch is not rebase-mergeable. Not on trunk: there a pull that merges
  puts the commit on main as it was made, its ledger without the task, so
  trunk keeps today's rule (the fifth scenario).

  What verify and CI then do. A commit on the remote's main is judged at its
  own ledger as before, so main's run judges each rebase-merged commit
  against the ledger it carries, which has the task; a commit made on main
  before its task was added is still refused (the fourth scenario). A pull
  request's run that counts is the head GitHub rebased onto main (itos push
  asks for the rebase when main has a commit the head lacks, as it does once
  a task was added there; under push.pr.draft, at --ready), whose commits
  carry the task, so it passes as today. CI's checkout records no remote HEAD, so a run of a head not yet
  rebased still judges by the commit's own ledger and may fail; it is
  superseded by the rebased head's run. This repository's plan of a pull
  request's run reads the ledger of the tree CI checked out, the merge with
  main, which has the task, so its checks run there as for any task. The commit-msg
  hook runs no check of a task its staged ledger lacks, as today: CI owns
  them (ADR-0057).

  Not in this slice. The registry's footer (Item:), read at the commit too:
  an item added on main mid-item would meet the same refusal, but the
  evidence is the ledger's. A refusal that says "fetch" when the clone's
  main is stale: the hook asks no remote, so it cannot know. itos push
  keeping copies of main's commits when a branch rebased onto main is
  rebased onto the remote's item branch: after this slice nothing calls for
  that rebase; refusing or dropping such copies is its own item. A take made
  on an item's branch above its spec reaching main only at the merge
  (p1-pr-take-after-spec): the other direction, a registry commit carried
  from the branch to main by itos push's registryPrefix, with a design
  question of its own (send it to main, or take on main before the switch),
  none of it this footer rule. An item with no ledger task needing a gate
  commit (issue 132): no ledger has its task, so reading main's changes
  nothing for it.

  Two steps are new. "the clone has the commit <subject> naming the task
  <id> and touching <path>" commits a line in <path> with the message
  <subject> and the footer Task: <id>, past the hooks. "the remote has
  gained the commit <subject> naming the task <id> and touching <path>" is
  that commit made in another clone and pushed to the remote's main, past
  the hooks, as the remote's other commits are gained.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And the commit-msg hook is installed
    And the pre-push hook is installed
    And the work registry has the item "T-001" owned by "someone" with the status "doing"

  # The slice's point, as seen on slice-187: the task was added on main after
  # the item's branch was cut, and the clone has fetched main since. Fails
  # before the work at the exit code: today the hook reads only the staged
  # ledger and refuses, "unknown tasks: T-002", exit 1.
  @ID-MAINTASK-01 @slice-189
  Scenario: Under pr, on an item's branch, itos commit names a task added on main since the branch was cut
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And the clone is on the new branch "T-001"
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And the clone has fetched the remote
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-002 -m 'chore: tidy the readme'"
    Then itos exits with code 0
    And the message of HEAD has the footer "Task: T-002"

  # The pre-push hook judges the pushed range with the same rule. The item's
  # branch is already on the remote, the mid-item case of slice-187, so itos
  # push rebases onto origin/T-001, which lacks T-002, not onto main. Fails
  # before the work at the exit code: today the pre-push hook refuses the
  # commit, "unknown tasks: T-002", and the push exits 1.
  @ID-MAINTASK-02 @slice-189
  Scenario: Under pr, itos push sends an item's branch whose commit names a task only the remote's main holds
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "T-001"
    And git pushes HEAD to the remote's new branch "T-001"
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And the clone has fetched the remote
    And the clone has the commit "chore: tidy the readme" naming the task "T-002" and touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote has the branch "T-001" with "chore: tidy the readme"
    And the remote's branch does not have "chore: tidy the readme"

  # The hook reads the remote's main as the clone last fetched it and asks
  # no remote: before the fetch, the task is unknown, in today's words. A
  # guard: it passes before the work and must after it; a hook that fetches
  # or asks the remote fails it.
  @ID-MAINTASK-03 @slice-189 @guard
  Scenario: Under pr, a clone that has not fetched since main gained the task is still told it is unknown
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And the clone is on the new branch "T-001"
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-002 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "unknown tasks: T-002"
    And its output names the rule "task-footer"

  # The rule's bound: a commit already on the remote's main is judged at the
  # ledger it carries, so main's history never holds a commit whose task its
  # own ledger lacks. Here the task was added to main after the commit that
  # names it. A guard: it passes before the work and must after it; a build
  # that reads main's ledger for every commit fails it.
  @ID-MAINTASK-04 @slice-189 @guard
  Scenario: Under pr, verify still refuses a commit on main that names a task only a later commit of main adds
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
        pr:
          draft: true
      """
    And the remote has gained the commit "chore: tidy the readme" naming the task "T-002" and touching "README.md"
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And the clone has fetched the remote
    When itos runs "verify origin/main~2 origin/main"
    Then itos exits with code 1
    And its output says "unknown tasks: T-002"

  # Under trunk a pull that merges puts the commit on main as it was made,
  # its ledger without the task, so trunk keeps today's rule. A guard: it
  # passes before the work and must after it; a build that drops the
  # push.mode pr condition fails it.
  @ID-MAINTASK-05 @slice-189 @guard
  Scenario: Under trunk, a commit naming a task only the remote's main holds is still refused
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: trunk
      """
    And the remote has gained the commit "docs: add T-002" adding the task "T-002" to the ledger
    And the clone has fetched the remote
    And a change to "README.md" is staged
    When itos commits with the arguments "--task T-002 -m 'chore: tidy the readme'"
    Then itos exits with code 1
    And its output says "unknown tasks: T-002"
    And its output names the rule "task-footer"
