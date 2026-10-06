@phase-1
Feature: ci run's log names a merged check by the kind of named tests it ran in
  A push's CI merges every task check that runs named tests into one run of
  their kind (the `tests:` step), and its log lists each merged check under
  its task. That line said "(in the E2E run above)", the template's word for
  its Playwright suite, whatever the kind was called. itos knows the kind by
  the name tests.<kind> gives it, so the line names that kind: the log tells
  an agent which run to read for the check's result.

  Background:
    Given a repository whose ledger has the task "T-001"

  @ID-CI-01 @slice-11
  Scenario: A task's check that runs named tests is logged as part of its kind's run
    Given the CI steps run the named tests of the kind "scenario"
    And the task "T-001" has a check that runs the scenario "@ID-A-01"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "(in the scenario run above)"
    And its output does not say "E2E"

  # An ids selection renders through ids_pattern, so an empty one rendered
  # "@(?:)\b", which matches every tag: the run meant for one scenario ran
  # them all.
  @ID-CI-02 @slice-12
  Scenario: An empty smoke set adds no pattern to the run of named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And the task "T-001" has a check that runs the scenario "@ID-A-01"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "run @ID-A-01"
    And its output does not say "@(?:)"

  @ID-CI-03 @slice-12
  Scenario: A range that names no test, with an empty smoke set, runs no named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output does not say "printf"
    And its output does not say "run every scenario"

  # A check recognized as the kind's smoke run merges into the run of named
  # tests. With an empty smoke set and a range that names no test there is no
  # such run, so the check runs as the task wrote it. The recording script is
  # the check, so whether it ran is read from the tree.
  @ID-CI-04 @slice-15
  Scenario: A check recognized as the smoke run runs as itself when there is no run of named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And "run-smoke" is a script that records it ran
    And the kind "scenario" recognizes "./run-smoke" as its smoke run
    And the task "T-001" has the check "./run-smoke"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check ran
    And its output does not say "(in the scenario run above)"

  # The ledger's footer is whatever commits.footers calls the footer whose
  # source is the ledger; Task: is only this repository's name for it.
  @ID-CI-05 @slice-18
  Scenario: A push's CI runs the checks of a task named in the ledger footer, whatever the config calls it
    Given the ledger footer is called "Work"
    And the task "T-001" has a static check that records it ran
    And the commit "chore: tidy the readme" naming the task "T-001" in the footer "Work" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check ran

  @ID-CI-06 @slice-18
  Scenario: A footer naming a task the ledger lacks is reported against the ledger's own folder
    Given the ledger's files are "work/phase-{group}.yaml"
    And the CI steps are "exit 0"
    And the commit "chore: tidy the readme" naming the task "T-009" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "No task T-009 in work/"
    And its output does not say "tasks/"

  @ID-CI-07 @slice-18
  Scenario: A prose-only range names the prose steps the config gives
    Given the CI steps are "exit 0"
    And the prose paths are "**/*.md" and the prose steps are "echo prose"
    And the commit "docs: write the readme" touching only "README.md" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "echo prose"
    And its output does not say "vp check"

  # Bug 10, found by slice 48's agent on the starter itos init writes: ci plan
  # and ci run required a work: section though every work key has a default,
  # so a config without one, the starter's among them, could not plan CI
  # ("itos.yaml: work is missing"), while itos work read the defaults.
  @ID-CI-08 @bug-10
  Scenario: A config without a work section plans and runs CI, reading work's defaults
    Given the config has no work section
    And the CI steps are "exit 0"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos plans CI over the commits after the first
    Then itos exits with code 0
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output does not say "work is missing"

  # Bug 23 (issue #8). ci range's github provider took the newest green run
  # from GitHub's list of the workflow's runs on the branch. On 2026-10-05,
  # near 13:30Z, that list answered stale in two repositories at once: here
  # run 37316223828 began its range at 8e3acf8, a day old, past the green
  # 612bf3b, and character-editor's run 37318371439 began 371 commits back,
  # past its green parent. The wide range ran old task checks (T-092, T-113),
  # and the next run's right range no longer reached them, so a red check was
  # dropped. The coordinator's calls (2026-10-05, the user agreeing to the
  # fix): the provider asks each commit's own runs instead, walking the
  # head's first parents from its parent (the head's own run is the one
  # asking), and the range starts at the first commit with a green run of
  # the workflow; a run failed, cancelled or still going is passed over. Past
  # 100 first parents with none green the start is empty, which runs
  # everything, as a provider that fails does. The provider asks
  # GITHUB_API_URL when it is set, as Actions sets it and GitHub Enterprise
  # needs it, else https://api.github.com; that is also how a scenario points
  # itos at a fake GitHub. ci.watch's provider asks the same address.
  @ID-CI-09 @bug-23
  Scenario: ci range starts at the head's parent when its run is green, whatever the list of runs says
    Given ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And three commits on top of the first
    And the fake GitHub's list of runs names a green run of the first commit alone
    And the fake GitHub has a green run of the head's parent
    When itos prints where the range of the head starts
    Then itos exits with code 0
    And the range starts at the head's parent

  @ID-CI-10 @bug-23
  Scenario: ci range passes over an ancestor whose run failed or is still going
    Given ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And three commits on top of the first
    And the fake GitHub has a failed run of the head's parent
    And the fake GitHub has a run still going of the commit before the head's parent
    And the fake GitHub has a green run of the first commit
    When itos prints where the range of the head starts
    Then itos exits with code 0
    And the range starts at the first commit

  @ID-CI-11 @bug-23
  Scenario: ci range starts nowhere, running everything, when no ancestor has a green run
    Given ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And three commits on top of the first
    And the fake GitHub has a failed run of the head's parent
    When itos prints where the range of the head starts
    Then itos exits with code 0
    And the range is empty

  # Bug 28 (issue #14; was p1-ci-after-push-checks; the user's call,
  # 2026-10-05). Push CI ran a named task's after: push checks, though such
  # a check means something only once the push has landed: one waiting on a
  # release or a tag is red on every range that names its task until then
  # (T-021 here), and itos ci watch, run inside push CI, waits on the run it
  # is part of. A consumer listed it in ci.nightly_only, whose label then
  # said it runs in the nightly, which runs no named task's checks. Push CI
  # now lists a named task's after: push checks as pending, after the push,
  # and neither runs them nor counts them against the run; itos task and
  # itos work done still run them. ci.nightly_only's label for a task's
  # check says only that push CI leaves it out, not that the nightly runs it.
  @ID-CI-12 @bug-28 @wip
  Scenario: Push CI lists a named task's after: push check as pending and does not run it
    Given the task "T-001" has a check that records it ran, after push
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check did not run
    And its output says "after the push"
