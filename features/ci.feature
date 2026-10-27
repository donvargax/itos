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

  # Slice 109 (2026-10-09, the user agreeing). Main's last green run fell past
  # the 100 first parents ci range asks about, so every run's start was empty:
  # verify judged all history by today's rules, and plugin-version,
  # previous-release, deps-check, schema-contract and the mutation sample,
  # which need a start, failed (run 38014916168 stopped at plugin-version).
  # With no green run there was no start, and with no start no run could be
  # green. T-136 moved commits.since to the last green commit to get out. The
  # call: when the provider gives no start (none green within reach, a provider
  # that fails, or provider none), the range starts at the commit commits.since
  # names, when that commit is in the head's history; verification starts
  # there already. With no commits.since, or one not in the head's history, the
  # start stays empty and everything runs, as @ID-CI-11 says. A pull request
  # keeps its base. Raising the 100 was turned down: it only postpones this.
  @ID-CI-24 @slice-109
  Scenario: ci range starts at commits.since when no ancestor has a green run
    Given ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And three commits on top of the first
    And commits.since names the first commit
    And the fake GitHub has a failed run of the head's parent
    When itos prints where the range of the head starts
    Then itos exits with code 0
    And the range starts at the first commit

  # Bug 28 (issue #14; was p1-ci-after-push-checks; the user's call,
  # 2026-10-05). Push CI ran a named task's after: push checks, though such
  # a check means something only once the push has landed: one waiting on a
  # release or a tag is red on every range that names its task until then
  # (T-021 here), and itos ci watch, run inside push CI, waits on the run it
  # is part of. A consumer listed it in ci.nightly_only, whose label then
  # said it runs in the nightly, which runs no named task's checks. Push CI
  # now lists a named task's after: push checks as pending, after the push,
  # and neither runs them nor counts them against the run; itos task still
  # runs them, and itos work done leaves them pending too. ci.nightly_only's
  # label for a task's check says only that push CI leaves it out, not that
  # the nightly runs it.
  @ID-CI-12 @bug-28
  Scenario: Push CI lists a named task's after: push check as pending and does not run it
    Given the task "T-001" has a check that records it ran, after push
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the recording check did not run
    And its output says "after the push"

  # Bug 29 (issue #15; the user's call, 2026-10-05). Since bug 23 the range
  # walk asks GitHub for each first parent's own runs, still filtered by
  # ci.range.github.branch, so a push to another branch found only main's
  # green ancestor and re-checked commits that branch had already proved. A
  # green run of the workflow on that exact commit proves it whatever
  # branch it ran on, so the walk (ci range's, and itos status's last green
  # commit) no longer filters by branch. v5 removes ci.range.github.branch
  # itself (slice 85, the user's call), so no warning is needed here.
  @ID-CI-13 @bug-29
  Scenario: ci range starts at the head's parent whose green run was on another branch
    Given ci.range asks a fake GitHub for the runs of "ci.yml" on "main"
    And three commits on top of the first
    And the fake GitHub has a green run of the head's parent on the branch "feature"
    When itos prints where the range of the head starts
    Then itos exits with code 0
    And the range starts at the head's parent

  # Slice 83 (issue #12; the user's call, 2026-10-05). A CI step was not
  # given the range it runs for, so a step that depends on it read the
  # workflow's own variables, which itos ci run <from> <to> on a machine does
  # not set, and the plan there differed from CI's. Every step, and every
  # named task's check that ci run runs, gets ITOS_FROM and ITOS_TO in its
  # environment, the range's ends as full SHAs (ITOS_FROM empty for a range
  # that runs everything), and {from} and {to} in a step's command are
  # filled in as range checks' are. A step only for some paths stays
  # p3-ci-step-paths.
  @ID-CI-14 @slice-83
  Scenario: A CI step is given the range's start in its environment
    Given the CI steps are "echo from=$ITOS_FROM"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "from=" with the first commit's full SHA

  @ID-CI-15 @slice-83
  Scenario: A CI step's {from} is filled in with the range's start
    Given the CI steps are "echo since {from}"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "since " with the first commit's full SHA

  # Slice 84 (issue #13; the user's call, 2026-10-05). ci run sorts its
  # steps by cost class, static before late, so a repository whose previous
  # CI ran its steps in a chosen order could keep it only by calling every
  # command static, which empties the cost class of meaning for the hook and
  # the nightly. ci.keep_step_order: true runs ci.steps as written, then the
  # named tasks' checks in their ledger order, instead of sorting by cost;
  # the cost class still decides what the hook and the nightly's static step
  # run. A named task's check that should run on a prose-only range says
  # prose: true, as it can already.
  @ID-CI-16 @slice-84
  Scenario: With ci.keep_step_order, ci plan lists the steps as written, a late one before a static one
    Given ci.cost.static is "^echo static$"
    And ci.keep_step_order is true
    And the CI steps are "echo late", then "echo static"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos plans CI over the commits after the first
    Then itos exits with code 0
    And its output says "echo late" before "echo static"

  # Bug 32 (found by the code review of 07b0e6d). CI's plan read the
  # range's changed paths with rename detection on, so a commit moving code
  # into docs/ listed only its new path, made the range prose-only, and the
  # build and the tests were skipped. The plan reads them with --no-renames
  # and every change type, as the commit-msg hook does.
  @ID-CI-17 @bug-32
  Scenario: A range that renames code into docs/ is not prose-only
    Given "src/app.js" is in the first commit
    And the CI steps are "echo full"
    And the prose paths are "docs/**" and the prose steps are "echo prose"
    And the commit "docs: move the app" renaming "src/app.js" to "docs/app.md" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And its output says "echo full"
    And its output does not say "echo prose"

  # Bug 33 (found by the code review of 07b0e6d). An empty range start
  # means every commit up to the head, as on a first run, an API blip or no
  # green parent within the bound; yet the footer reader returned early on
  # an empty start, so the plan named no task and no test: a red late check
  # went green and never ran again. An empty start reads the footers of
  # every commit up to the head, for the named tasks and the named tests
  # alike.
  @ID-CI-18 @bug-33
  Scenario: With an empty range start, CI runs the checks of the tasks every commit names
    Given the task "T-001" has a static check that records it ran
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI with an empty range start
    Then itos exits with code 0
    And the recording check ran

  # Slice 86 (decision 35): ci run exited with a failing step's own code, so
  # a step exiting 75 (or 2, or 3) read as itos's own meaning of it. A failing
  # step makes ci run exit 1, its output naming the step's code. The step
  # writes 75 as $((70+5)), so the command printed does not hold the code.
  # Slice 86 also takes every exit code from the kind of the error, never from
  # a default (decision of q-16): failure() gave every error that was not a
  # usage or config error exit 2, so a network error not wrapped by hand
  # could never be 75. A usage error and a config error exit 2, a missing
  # environment 3, a failure that may pass when run again 75, and an error
  # itos cannot classify 70 (sysexits' EX_SOFTWARE: report it), unit-tested
  # since no scenario can provoke one on purpose. v6.0.0 ships slice-86, 87,
  # 88, 89 and T-101 in one push: each agent commits and does not push, and
  # the coordinator pushes the stack.
  @ID-CI-19 @slice-86
  Scenario: A step that exits 75 makes ci run exit 1, naming the step's code
    Given the CI steps are "exit $((70+5))"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "75"

  # Slice 106 (issue #28; the user's calls, 2026-10-08): the code proof
  # gates code before it ships, so it runs in CI, not at work done, where it
  # came after the push, the green run and a feat's release (itos-template's
  # slice 8 released v0.10.0 before any proof ran). When the config has
  # proof.code and the range touches its paths, itos ci run runs
  # proof.code.check as a step of the plan, added as a named task's checks
  # are, with {base} the range's start (<from>): every commit since the last
  # green run is code no green run has proven, whoever's item it is, so the
  # whole range is judged. It runs among the static steps, before the named
  # tests, since it reads recorded results and runs none; itos ci plan
  # lists it. A survivor, an uncovered or missing result fails the run,
  # naming each problem's rule and message from the check's --json, so the
  # release job that needs the run never runs; a check that cannot run fails
  # it too, as any step does (@ID-CI-19), never a pass. Recording results
  # stays the author's, before the push. With no proof.code, or a range that
  # touches none of its paths, the plan is as before. work done no longer
  # runs the proof (@ID-WORK-87): the green run it requires already judged it.
  @ID-CI-20 @slice-106
  Scenario: A range that touches proof.code's paths runs the code proof over it, its base the range's start
    Given the config's proof.code covers "src/**" and checks with a provider that passes
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the provider was run with the base the range starts at

  @ID-CI-21 @slice-106
  Scenario: A code proof that finds a surviving mutant fails the CI run, naming it
    Given the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "app.Total"

  @ID-CI-22 @slice-106
  Scenario: A range that touches none of proof.code's paths does not run the code proof
    Given the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the commit "chore: tidy the readme" touching only "README.md" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 0
    And the provider was not run

  @ID-CI-23 @slice-106
  Scenario: A code proof that cannot run fails the CI run
    Given the config's proof.code covers "src/**" and checks with a command that does not exist
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1

  # Slice 132 (the user's call, 2026-10-12, rapid feedback): a push waits on
  # its slowest job, and that was a platform job running every feature, where
  # the ci job runs only the smoke set and the scenarios the range names. ci
  # plan's steps are plain commands with nothing marking the run of named
  # tests, so a platform job could not ask for it. ci run <from> <to> --tests
  # runs that run alone, as ci run would merge it (the smoke set, the range's
  # named tests, the task checks recognized as them), and nothing else: no
  # other step, no other task check, no code proof. With no run of named tests
  # for the range, it runs nothing and says so, exit 0.
  @ID-CI-25 @slice-132
  Scenario: ci run --tests runs the range's named tests and no other step
    Given the CI steps are "exit 3"
    And the CI steps run the named tests of the kind "scenario"
    And the task "T-001" has a check that runs the scenario "@ID-A-01"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI's named tests over the commits after the first
    Then itos exits with code 0
    And its output says "ID-A-02"
    And its output says "@ID-A-01"
    And its output does not say "exit 3"

  @ID-CI-26 @slice-132
  Scenario: ci run --tests runs no task check that is not a run of named tests
    Given the CI steps run the named tests of the kind "scenario"
    And the task "T-001" has the check "exit 4"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI's named tests over the commits after the first
    Then itos exits with code 0
    And its output does not say "exit 4"

  @ID-CI-27 @slice-132
  Scenario: ci run --tests does not run the code proof
    Given the CI steps run the named tests of the kind "scenario"
    And the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI's named tests over the commits after the first
    Then itos exits with code 0
    And the provider was not run

  @ID-CI-28 @slice-132
  Scenario: ci run --tests with no run of named tests for the range runs nothing and says so
    Given the CI steps are "exit 3"
    And the CI steps run the named tests of the kind "scenario"
    And the smoke set is empty
    And the commit "chore: tidy the readme" touching only "README.md" on top of it
    When itos runs CI's named tests over the commits after the first
    Then itos exits with code 0
    And its output says "no named tests"
    And its output does not say "exit 3"

  # Slice 133 (p1-nightly-past-first-failure; the user's call, 2026-10-12,
  # rapid feedback). ci.stop_at_first_failure was one setting for pushes and
  # the nightly, so the nightly stopped at its first failure: on 2026-10-12 a
  # red self-test hid both mutation steps behind it, a day per hidden failure.
  # The nightly runs once a day to report, so it always goes on past a
  # failing step and fails at the end; a push still stops early when the
  # setting says so (@ID-CONFIG-05). A run that goes on names the first
  # failure as before and each later one after it, and --json lists them all
  # in failures, failed_at staying the first.
  @ID-CI-29 @slice-133
  Scenario: The nightly goes on past a failing step when ci.stop_at_first_failure is true, and still fails
    Given the nightly steps are "exit 3" then a step that records it ran
    And ci.stop_at_first_failure is true
    When itos runs the nightly CI
    Then itos exits with code 1
    And its output says "CI failed at: exit 3"
    And the recording step ran

  @ID-CI-30 @slice-133
  Scenario: The nightly names every failing step after the first, and its --json lists each one
    Given the nightly steps are "exit 3", then "exit 4"
    When itos runs the nightly CI with --json
    Then itos exits with code 1
    And its output says "also failed at: exit 4"
    And its JSON's "failures" has one entry whose "step" is "exit 4"

  @ID-CI-31 @slice-133
  Scenario: A push's CI that goes on past its first failure names every failing step
    Given the CI steps are "exit 3", then "exit 4"
    And ci.stop_at_first_failure is false
    When itos runs CI over every commit up to HEAD
    Then itos exits with code 1
    And its output says "CI failed at: exit 3"
    And its output says "also failed at: exit 4"

  # Slice 198 (was p1-ci-steps-annotate-failures, found specifying slice-197,
  # #212). Slice 197's itos ci watch and itos push read a failed job's
  # failure-level check-run annotations when its log's host is refused, but
  # no ci run emitted any, so the fallback could only show GitHub's "Process
  # completed with exit code N." Under GITHUB_ACTIONS=true, exactly as Actions
  # sets it, ci run (push, --tests and --nightly alike) prints one workflow
  # command per failure it reports, on stderr beside that failure's lines:
  # GitHub reads commands from both streams, and stderr keeps --json's stdout
  # one object.
  #   ::error file=<path>,line=<n>::<what> failed: <reason>
  # when the failure names a file and line, else
  #   ::error::<what> failed: <the first line of its reason>
  # <what> is a step's command, a task check's "<ID> (<title>)" ("<ID>"
  # without a title), "the code proof", or "the tasks named". A failing
  # step's or task check's file and line are those of the first line of its
  # output, stdout or stderr, that begins with <path>:<line>: (a <column>:
  # may follow), <path> relative, holding no white space and naming a file
  # in the folder ci run runs in; the reason is the rest of that line,
  # trimmed. Without one, a step's reason is "it exited <n>" ("it gave no
  # exit code"), a task check's "its check <command>". The code proof's
  # failure is its problems, one command each: at the file and line the
  # provider's problem names as its subject (itos-cc's file and line keys, a
  # string and a positive whole number), taken as given, its reason
  # "<rule>: <message>", the whole message; a problem without both, itos's
  # own among them, is plain. A footer naming a task no file has is plain,
  # its reason itos's "No task <ID> in <dir>/, though a footer names it".
  # The message escapes % as %25, CR as %0D and LF as %0A; a property those
  # and : as %3A and , as %2C; the path is written with forward slashes.
  # At most 10 a run, the first in run order: one ci run is one workflow
  # step, and GitHub keeps 10 error annotations a step (50 a job) and drops
  # the rest; the log still names every failure. Outside GitHub Actions
  # nothing changes. Not in this slice: go test's failures inside a step's
  # output, which it prints indented and relative to their package, so no
  # line of them begins with a path from the root; a problem matcher for
  # them is an idea of its own.
  @ID-CI-67 @slice-198 @wip
  Scenario: Under GitHub Actions a failing step whose output names a file and line is annotated at that line
    Given GITHUB_ACTIONS is "true"
    And "src/app.go" is in the first commit
    And the scratch file "lint.sh" contains exactly:
      """
      printf '%s\n' 'checking src/app.go'
      printf '%s\n' '    src/app.go:1: indented, as go test prints its failures'
      printf '%s\n' 'nowhere.go:2:1: no such file in the working folder'
      printf '%s\n' 'src/app.go:3:9: Total is never negative (SA4003)' >&2
      printf '%s\n' 'src/app.go:5:1: a second finding' >&2
      exit 1
      """
    And the CI steps are "sh lint.sh"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And stderr contains the complete line "::error file=src/app.go,line=3::sh lint.sh failed: Total is never negative (SA4003)" exactly once
    And stderr has exactly 1 line starting with "::error"

  @ID-CI-68 @slice-198 @wip
  Scenario: Under GitHub Actions a failing step that names no file is a plain error naming it, on stderr under --json too
    Given GITHUB_ACTIONS is "true"
    And the CI steps are "exit 3"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And stderr contains the complete line "::error::exit 3 failed: it exited 3" exactly once
    When itos runs CI over the commits after the first with --json
    Then itos exits with code 1
    And stderr contains the complete line "::error::exit 3 failed: it exited 3" exactly once
    And stdout contains neither "::error" nor "CI failed at"

  @ID-CI-69 @slice-198 @wip
  Scenario: Under GitHub Actions a failing task check that names no file is a plain error naming its task
    Given GITHUB_ACTIONS is "true"
    And the task "T-001" has the check "exit 4"
    And the commit "chore: tidy the readme" naming the task "T-001" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And stderr contains the complete line "::error::T-001 (Tidy) failed: its check exit 4" exactly once

  # The Given's provider answers as itos-cc's mutation check --fail-uncovered
  # does for an uncovered line: rule mutation.uncovered-statement, message
  # "src/app.go:7 in app.Total has an uncovered executable Go coverage
  # block", subject keys file, function and line.
  @ID-CI-70 @slice-198 @wip
  Scenario: Under GitHub Actions the code proof's uncovered line is annotated at that line
    Given GITHUB_ACTIONS is "true"
    And the config's proof.code covers "src/**" and checks with a provider that finds line 7 of "src/app.go" uncovered in "app.Total"
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And stderr contains the complete line "::error file=src/app.go,line=7::the code proof failed: mutation.uncovered-statement: src/app.go:7 in app.Total has an uncovered executable Go coverage block" exactly once
    And stderr has exactly 1 line starting with "::error"

  # The provider's file is taken as given, so it need not exist; the comma
  # and the colon are escaped in the property, the percent sign, the CR and
  # the LF in the message.
  @ID-CI-71 @slice-198 @wip
  Scenario: Under GitHub Actions an annotation's message and file are escaped as workflow commands require
    Given GITHUB_ACTIONS is "true"
    And the config's proof.code covers "src/**" and checks with a provider that refuses with the problems:
      """
      [{"rule": "mutation.survived", "message": "100% of app.Total\r\nsurvived", "fix": "test app.Total", "file": "src/odd,name:1.go", "line": 4}]
      """
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And stderr contains the complete line "::error file=src/odd%2Cname%3A1.go,line=4::the code proof failed: mutation.survived: 100%25 of app.Total%0D%0Asurvived" exactly once
    And stderr has exactly 1 line starting with "::error"

  @ID-CI-72 @slice-198 @wip
  Scenario: Under GitHub Actions a run annotates its first 10 failures and no more
    Given GITHUB_ACTIONS is "true"
    And the config's proof.code covers "src/**" and checks with a provider that refuses with the problems:
      """
      [{"rule": "mutation.survived", "message": "one", "fix": "test it", "file": "src/app.go", "line": 1},
       {"rule": "mutation.survived", "message": "two", "fix": "test it", "file": "src/app.go", "line": 2},
       {"rule": "mutation.survived", "message": "three", "fix": "test it", "file": "src/app.go", "line": 3},
       {"rule": "mutation.survived", "message": "four", "fix": "test it", "file": "src/app.go", "line": 4},
       {"rule": "mutation.survived", "message": "five", "fix": "test it", "file": "src/app.go", "line": 5},
       {"rule": "mutation.survived", "message": "six", "fix": "test it", "file": "src/app.go", "line": 6},
       {"rule": "mutation.survived", "message": "seven", "fix": "test it", "file": "src/app.go", "line": 7},
       {"rule": "mutation.survived", "message": "eight", "fix": "test it", "file": "src/app.go", "line": 8},
       {"rule": "mutation.survived", "message": "nine", "fix": "test it", "file": "src/app.go", "line": 9},
       {"rule": "mutation.survived", "message": "ten", "fix": "test it", "file": "src/app.go", "line": 10},
       {"rule": "mutation.survived", "message": "eleven", "fix": "test it", "file": "src/app.go", "line": 11}]
      """
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "mutation.survived: eleven"
    And stderr has exactly 10 lines starting with "::error"
    And stderr has exactly 1 line starting with "::error file=src/app.go,line=10::"
    And stderr has exactly 0 lines starting with "::error file=src/app.go,line=11::"

  # A guard: it passes before the work and must after it. The scenario
  # harness drops every GITHUB_* variable from itos's environment.
  @ID-CI-73 @slice-198 @guard @wip
  Scenario: Outside GitHub Actions a failing run prints no workflow command
    Given the CI steps are "exit 3"
    And ci.stop_at_first_failure is false
    And the config's proof.code covers "src/**" and checks with a provider that finds a survivor in "app.Total"
    And the commit "chore: change the app" touching only "src/app.go" on top of it
    When itos runs CI over the commits after the first
    Then itos exits with code 1
    And its output says "failed at: exit 3"
    And its output says "app.Total"
    And neither stream contains "::error"

  # Slice 137 (#72, q-65): classification and a workflow recipe only;
  # ci run keeps its policy. Keep schema/docs_only and add classification
  # (prose, metadata, full) and reasons (stable string codes).
  # Every scope fixture commits valid policy, ledger, registry and any named
  # tests before recording the range base. Subsequent steps make real commits.
  # The When passes those exact base/head SHAs to ci scope --json; fixture
  # setup is outside the range. Classification is never a validation verdict.
  @ID-CI-32 @slice-137 @wip
  Scenario Outline: bookkeeping uses the configured registry path and an explicit field allow-list
    Given a committed CI scope fixture with registry path "acceptance/work.yaml"
    And a committed range whose only registry change is "<change>"
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "metadata"
    And its JSON's "docs_only" is false
    And its JSON's "reasons" equals:
      """
      ["registry-bookkeeping"]
      """

    Examples:
      | change          |
      | item owner      |
      | item status     |
      | top-level queue |
      | YAML formatting |

  @ID-CI-33 @slice-137 @wip
  Scenario: additive classification preserves the existing prose-only result
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a committed range changing only prose allowed by the committed policy
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "prose"
    And its JSON's "docs_only" is true
    And its JSON's "reasons" equals:
      """
      ["prose-only"]
      """

  @ID-CI-34 @slice-137 @wip
  Scenario Outline: non-bookkeeping changes keep full validation even beside an owner change
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a committed range changing "<subject>" alongside an item owner
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "full"
    And its JSON's "reasons" includes "<reason>"

    Examples:
      | subject                | reason                     |
      | registry title         | registry-definition-change |
      | registry why           | registry-definition-change |
      | registry dependency    | registry-definition-change |
      | registry ref           | registry-definition-change |
      | registry item addition | registry-definition-change |
      | registry item removal  | registry-definition-change |
      | registry item rename   | registry-definition-change |
      | registry phase owner   | registry-definition-change |
      | task check definition  | non-metadata-path          |
      | scenario text          | non-metadata-path          |
      | test fixture           | non-metadata-path          |
      | implementation source  | non-metadata-path          |
      | workflow configuration | non-metadata-path          |

  # The fixture declares each referenced task or live scenario at base.
  @ID-CI-35 @slice-137 @wip
  Scenario Outline: a named requirement prevents a reduced classification
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a committed range changing only an item owner and naming a "<requirement>" requirement
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "full"
    And its JSON's "reasons" includes "named-requirement"

    Examples:
      | requirement |
      | task        |
      | scenario    |

  # Endpoints differ only in an owner: temporary changes are real commits
  # that must remain visible to the classifier after they are reverted.
  @ID-CI-36 @slice-137 @wip
  Scenario Outline: endpoint-only diffs cannot hide requirement-affecting intermediate changes
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a committed range temporarily changing "<subject>" and restoring it before an owner-only final commit
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "full"
    And its JSON's "reasons" includes "intermediate-non-metadata-change"

    Examples:
      | subject               |
      | implementation source |
      | registry dependency   |
      | task check definition |
      | execution policy      |

  # Existing config/usage errors keep their exit behavior. These fixtures
  # have valid config but cannot prove the range or policy grants an exemption.
  @ID-CI-37 @slice-137 @wip
  Scenario Outline: unknown history or unproven policy cannot grant a metadata exemption
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a CI scope fixture with "<problem>"
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "full"
    And its JSON's "reasons" includes "<reason>"

    Examples:
      | problem                             | reason          |
      | an empty range                      | unproven-range  |
      | an unavailable base commit          | unproven-range  |
      | a shallow clone lacking the base    | unproven-range  |
      | a base divergent from the target    | unproven-range  |
      | a zero start SHA                    | unproven-range  |
      | an uncommitted classification policy | unproven-policy |

  # The recording command is configured before the base. Classification
  # invokes no CI step, task check, hook or provider to choose the route.
  @ID-CI-38 @slice-137 @wip
  Scenario: classification executes no checks and changes no repository files
    Given a committed CI scope fixture with a recording CI step
    And a committed range whose only registry change is "item owner"
    And the repository files and index are saved for comparison
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "metadata"
    And the recording step did not run
    And the repository files and index match the saved state

  # Invalid registry structure cannot prove bookkeeping. The classifier
  # reports the full route and this reason; the recipe's separate metadata
  # validation must still run and reject the malformed data.
  @ID-CI-39 @slice-137 @wip
  Scenario Outline: malformed registry data is diagnosed without granting a reduced route
    Given a committed CI scope fixture with registry path "tasks/work-items.yaml"
    And a committed range with "<problem>" in the registry
    When itos classifies this committed range with --json
    Then itos exits with code 0
    And its JSON's "classification" is "full"
    And its JSON's "reasons" includes "invalid-registry"

    Examples:
      | problem               |
      | unparseable YAML      |
      | an unknown field      |
      | an invalid item shape |
