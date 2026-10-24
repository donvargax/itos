@phase-1
Feature: Under push.mode pr, the ledger edits itos makes beside the registry go straight to main
  Under push.mode pr (slice 174, push-mode.feature) itos push sends to main
  only the commits that touch the work registry alone (slice 56's rule): a
  take, a queue move, an idea, the close of an item with no checks. Any
  other commit pushed
  from main is refused (outcome work-on-main), and on an item's branch only
  the registry commits directly above main go to main, the rest riding the
  pull request. Three commands commit a ledger file beside the registry, in
  one commit: work done on a task with checks takes them out of its entry
  (ADR-0043, slice 101), task add writes the task's entry with its item, and
  work promote --kind task does as task add. Under pr none of their commits
  can reach main: a task with checks is never closed, and a task added on
  main waits on a pull request before anyone can take it. Most of this
  repository's items are tasks, so this blocks its switch to pr.

  The rule (slice-177): a commit is the registry's, for pr's two judgements
  (whether a push to main adds work, from main or with --ref main; and
  which commits above main an item's branch sends to main), when it touches
  the work registry and every other path it touches is a ledger file: a
  path ledger.files' pattern matches, in its folder, its {group} by
  ledger.group.pattern, as the ledger's readers find the files (config
  check already refuses a ledger.files that matches work.registry). With no
  ledger section in the config, no path is a ledger file and the rule is
  slice 174's. It judges each commit by itself, against its first parent,
  never the push's commits together, so a ledger-only commit is not let
  through by a take beside it. Both judgements use the one rule, so a task
  added on an item's branch directly above main goes to main as a take made
  there does.

  Why paths and the registry, not the ledger's content. The ledger is not
  claims, but these three edits are bookkeeping that must not wait on a pull
  request: the close finishes a claim, and a task must be on main before it
  can be taken there. Each always writes the registry in the same commit, so
  requiring the registry keeps out the usual hand edit of the ledger, a
  check added or fixed or a why reworded, which touches the ledger alone and
  still goes through a pull request, refused on main as today. A hand edit
  bundled into a commit with a registry change slips through; checking that
  the ledger's change is only a task entry added or a done_when removed
  would close that, at the cost of parsing every touched ledger file at two
  commits for each commit judged. Not worth it while itos is one person's
  (guardrails over hardening): such a commit reaches main as it does under
  trunk, past the same commit-msg and pre-push hooks, and main's run judges
  it after.

  Unchanged: trunk, and with no push.mode, push as before. The rule by which
  itos push does not wait for a run (slice 56) and work done passes over a
  commit (slice 93) stays the registry alone (touchesOnlyRegistry): a ledger
  edit changes what a run judges (the ledger's checks, config check's lint
  of it), so the close of a task with checks is waited for as before, under
  both modes. The refusal's words do not change: a commit pr refuses still
  touches more than the registry, a ledger-only one included.

  The implementing commit rewords push-mode.feature's description where it
  names what goes straight to main (a take, a close, a queue move, an idea:
  slice 56's rule) to point here; slice 175 has landed in that file.

  The recording gh is push-mode.feature's. Its steps are reused; two are
  new. "the clone has the commit <subject> closing the task <id> and taking
  out its checks" makes the commit work done makes for a task with checks,
  without running work done (which under pr asks for the item's pull
  request, slice 176): the registry's item <id> given the status done and
  the owner someone, the ledger's entry for <id> without its done_when, both
  files committed together and nothing else, past the hooks. "the clone has
  the commit <subject> adding the idea <id> to the registry and touching
  <path>" is the idea step's registry edit and the touching step's line in
  <path>, in one commit, past the hooks.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the task "T-001" has the committed check "true"
    And a clone of it, where itos runs
    And the work registry has the item "T-001" owned by "someone" with the status "doing"

  # The slice's point: the close of a task with checks touches the ledger
  # beside the registry, and goes from main to main as any close does.
  # Fails before the work at the exit code: today it is refused as
  # work-on-main, exit 1.
  @ID-PUSHLEDGER-01 @slice-177
  Scenario: Under pr, a task's close that takes out its checks goes from main straight to main, opening no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: close T-001" closing the task "T-001" and taking out its checks
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: close T-001"
    And the recording gh was never called

  # task add, run for real: the coordinator adds tasks on main, and a task
  # must be on main before it can be taken there. work promote --kind task
  # commits the same two files and is judged by the same rule. Fails before
  # the work at the exit code: today it is refused as work-on-main, exit 1.
  @ID-PUSHLEDGER-02 @slice-177
  Scenario: Under pr, a task added on main goes straight to main, opening no pull request
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And itos has run the command line "task add --group 1 --type chore --title 'Sweep the docs' --why 'They drifted.' --check 'true'"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: add T-002"
    And the recording gh was never called

  # The second judgement uses the same rule: on an item's branch, a task
  # added directly above main (a follow-up found mid-work) goes to main,
  # the work above it to the item's branch. Fails before the work at the
  # main step: today the task's commit rides the pull request with the work.
  @ID-PUSHLEDGER-03 @slice-177
  Scenario: Under pr, a task added on an item's branch directly above main goes to main and the work to the branch
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone is on the new branch "T-001"
    And itos has run the command line "task add --group 1 --type chore --title 'Sweep the docs' --why 'They drifted.' --check 'true'"
    And the clone has the commit "feat: add the widget" touching "widget.md"
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: add T-002"
    And the remote's branch does not have "feat: add the widget"
    And the remote has the branch "T-001" with "feat: add the widget"

  # A hand edit of a task's checks touches the ledger alone: no registry
  # bookkeeping, so it still goes through a pull request: main still ends
  # with the config's commit. A guard: it passes before the fix and must
  # after it.
  @ID-PUSHLEDGER-04 @slice-177 @guard
  Scenario: Under pr, a push from main of a commit touching only the ledger is still refused, naming git switch -c
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the task "T-001" has the committed check "false"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git switch -c"
    And the remote's branch ends with "chore: add to the config"
    And the recording gh was never called

  # The rule judges commit by commit: an idea added to the registry beside
  # a separate commit touching only the ledger is refused, because the
  # ledger-only commit is not the registry's on its own; a judgement over
  # the union of the push's paths would let it through. A guard: it passes
  # before the fix and must after it.
  @ID-PUSHLEDGER-07 @slice-177 @guard
  Scenario: Under pr, a push from main of an idea and a separate ledger-only commit is still refused
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "docs: add p1-widget" adding the idea "p1-widget" to the registry
    And the task "T-001" has the committed check "false"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git switch -c"
    And the remote's branch ends with "chore: add to the config"
    And the recording gh was never called

  # The registry in a commit is not enough: every other path must be a
  # ledger file, so work beside a take is still work. A guard: it passes
  # before the fix and must after it.
  @ID-PUSHLEDGER-05 @slice-177 @guard
  Scenario: Under pr, a push from main of a commit touching the registry and another file is still refused
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the clone has the commit "feat: add the widget and p1-widget" adding the idea "p1-widget" to the registry and touching "widget.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "git switch -c"
    And the remote's branch does not have "feat: add the widget and p1-widget"
    And the recording gh was never called

  # Trunk and slice 56's wait rule are unchanged: a task's close touches
  # the ledger, which a run judges, so the push waits for its run rather
  # than saying its commits touch only the registry. A guard: it passes
  # before the fix and must after it.
  @ID-PUSHLEDGER-06 @slice-177 @guard
  Scenario: Under trunk with ci.watch, a push of a task's close goes to main and waits for its run
    Given the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: trunk
      """
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "docs: close T-001" closing the task "T-001" and taking out its checks
    When itos runs "push"
    Then itos exits with code 0
    And the remote's branch has "docs: close T-001"
    And the fake GitHub was asked for the run of the clone's HEAD
    And its output does not say "so its CI run is not waited for"
