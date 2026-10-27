@phase-1
Feature: A failed CI run names the step each failed job failed at and what failed in it
  A red run cost the coordinator three to five commands: the run's jobs from
  GitHub's API, each failed job's log through api.github.com, and a grep of
  it for itos ci run's failure lines, go test's FAIL lines, godog's failed
  steps and the code proof's mutation lines (2026-10-11; the user,
  2026-10-23: cut the ceremony of a red run). Once the watched run completes,
  itos push and itos ci watch name, for each job that did not succeed, the
  step it failed at and what its log says failed, each failure with its
  first lines, so the failure is handed to the agent that pushed with
  nothing else read; and a run names the tags its head commit carries on the
  remote, the release it cut.

  The contract is --json and the exit codes (ADR-0035, ADR-0052); neither
  code changes: 0 a run that succeeded, 1 one that did not, its logs read or
  not. Each job of run.jobs that did not succeed (skipped and neutral aside,
  as the end line's failed jobs) gains "step", the name of its first step
  that concluded neither success nor skipped, left out when the jobs API
  lists none; "log", "read" or "unreadable"; "failures", in the log's order,
  each {"kind","name"?,"package"?,"level"?,"lines","more_lines"?}; and
  "more_failures" when some were left out. run gains "tags", the remote's
  tags at the run's head commit, left out when there is none. A job that
  succeeded gains nothing, and a run that succeeded asks for no log. The
  text, after the end line on stderr (under -q and --json too, as the end
  line), says the same in words that may change: scenarios read it by loose
  substrings.

  A failure's kind is go-test, scenario, ci-run or tail, read from the log,
  or annotation, for a log that cannot be read (below). go-test is go
  test's "--- FAIL: <name>" line of a test with no failed subtest. Its lines
  are those indented deeper than that line, up to the first indented no
  deeper (a sibling's "--- FAIL:", "FAIL", the "FAIL", tab, package line,
  "ok", "PASS" or "=== RUN"); a deeper "--- FAIL:" line starts a subtest,
  whose lines are its own by the same rule and never its parent's. A test
  with a failed subtest is never a failure of its own: its own lines (such
  as TestFeatures' "a scenario failed", printed after its subtests) are
  dropped, and its failed leaves are listed, each by its full name. Its
  package is the first "FAIL", tab, package line after it. scenario is an
  entry of godog's summary of failed steps, as its pretty formatter prints
  it: after "--- Failed steps:", "Scenario: <name> # <file>:<line>", its
  failing step and "Error: " with the error, whose further lines godog
  prints as they are, up to the blank line that ends the entry. Its name is
  the scenario's without its location; its lines are the step and the
  Error line, each less godog's indentation, then the error's further lines
  as printed. godog's pretty print of the scenario before the summary is
  not read. go test's "--- FAIL: TestFeatures/<name>" of that scenario, its
  name rewritten as go test rewrites a subtest's (spaces as underscores,
  and its other rules), is the same failure, never a second. ci-run is itos
  ci run's "CI failed at" or "also failed at" line, its name the text after
  the lead less the colon or space that opens it, its lines the ones after
  it up to a blank line, GitHub's next "##[" line or the next failure line:
  the code proof's problems, each its rule and message (mutation.survived,
  mutation.uncovered-statement and the rest) and its fix. tail, which has
  no name, is given when no failure found has lines of its own: the last 20
  lines up to and including GitHub's first "##[error]" line, else the log's
  last 20, so a failed job's log never says nothing.

  A log is split at each LF and a line's trailing CR dropped, so a Windows
  runner's CRLF reads as LF; each line loses the timestamp GitHub stamps it
  with and every terminal escape sequence (CSI, colours among them, and
  OSC), as tools/bin/nightly-report reads the nightly's logs (T-185). A
  go-test or ci-run failure's lines lose the indentation they share, the
  rest kept. The
  limits: 10 lines a failure, 20 failures a job, 20 lines a tail; what is
  left out is counted (more_lines, more_failures) and said in the text,
  never cut silently. A log is asked for once per failed job, after the run
  completed, and never retried: one that cannot be read leaves its job's
  step and says so, and never changes the watch's exit.

  A log that cannot be read is most often not GitHub's refusal but the host
  it redirects to (slice 197): the text is on Azure blob storage
  (productionresultssa<n>.blob.core.windows.net), which a sandboxed agent's
  proxy and the devcontainer's allowlist refuse (2026-10-07, 2026-10-27).
  Such a job then falls back to what api.github.com serves itself: its step
  stays the jobs API's, and its check run's annotations
  (/repos/<repository>/check-runs/<job id>/annotations, an Actions job's id
  being its check run's), asked for once with per_page=100 and never
  retried (GitHub lists 30 a page unless asked for more, at most 100, and
  keeps at most 50 for a job, so the one page holds them all and none is
  dropped uncounted), become its failures, in the order the API lists
  them: those whose annotation_level
  is failure, GitHub's exit-code line among them, for a notice or a warning
  (an action's deprecation, say) is not why a job failed. Each is of kind
  annotation, its name the annotation's path and start line as
  <path>:<line>, its "level" GitHub's annotation_level, its lines the
  message's, split as a log's; its title is left out. The
  limits are the log's: 10 lines an annotation and 20 annotations a job, the
  rest counted in more_lines and more_failures, which counts failure-level
  annotations only. "log" stays "unreadable", and the text still says the
  log could not be read. A check run that lists no failure-level
  annotation, or whose annotations cannot be read either, leaves the job
  as before: its step and the line saying its log could not be read. A log
  that is read is read as before, and no annotation is asked for.

  An annotation is what a run's steps emit: a ::error or ::warning workflow
  command, a problem matcher's match, and GitHub's own "Process completed
  with exit code <n>." for a failed step, at .github line 1. This
  repository's CI emits none of its own (2026-10-27): no step prints
  ::error, and its pinned toolchain action, unlike setup-go, registers no
  problem matcher, so go test's failures, itos ci run's "CI failed at" lines
  and the code proof's mutation lines (an uncovered changed line among them)
  reach the annotations only as that exit-code line. Here the fallback says
  which step failed and with what exit code; what failed in it needs those
  lines emitted as annotations, which this slice does not do.

  The tags are read from the remote the commit was pushed to, where GitHub
  creates a release's tag when the release job publishes it, so no job need
  be named a release job and no config key is added; an annotated tag counts
  by the commit it points to.

  Here GitHub is the fake one of watch.feature. It lists each job with its
  id and its steps, as GitHub's jobs API does, and answers a job's log
  (/repos/<repository>/actions/jobs/<id>/logs) with a redirect to the log's
  text on itself, as GitHub redirects to a short-lived download address,
  each line stamped with a timestamp as GitHub stamps it. A job that fails
  at a step is listed with "Set up job" succeeded, that step failed and
  "Complete job" succeeded. It lists a job's check run's annotations, none
  unless a step lists some, paged as GitHub pages them: 30 unless per_page
  asks for more, at most 100. A step can have it redirect a job's log to a
  host that refuses it, as a sandbox's proxy refuses the blob host. The tags
  are the scenario's remote's.

  The why's command provider went in v5.0.0 (slice 85), so no provider has
  optional keys to grow; what it stood for, a provider giving less, is
  GitHub's own job listed with no steps, which keeps today's lines and
  gains no step. itos status's red Nightly line is left out: since the
  2026-10-11 note, nightly.yml's report job reads each red nightly job's
  log into the Nightly red issue (tools/bin/nightly-report, T-185), and
  status runs on every itos go, where downloading every red nightly job's
  log would cost each session. Every key is new and optional, so no
  scenario of watch.feature, push.feature or status.feature changes;
  status's nightly jobs gain nothing, since status reads no log.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"

  # The step is the jobs API's, the first that concluded neither success nor
  # skipped (a job GitHub timed out ends its step cancelled). The log matches
  # nothing known, so it brings a tail (@ID-WATCH-39), not read here.
  @ID-WATCH-35 @slice-172
  Scenario: A push whose run fails names the step its failed job failed at
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      something went wrong
      ##[error]Process completed with exit code 1.
      """
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's job "ci" has "step" "Run the unit tests"
    And its stderr says "Run the unit tests"

  # go test without -v, as CI runs it: the failed test, its message under it
  # and its package's FAIL line, then itos ci run's line for the step. The
  # test's lines lose go test's indentation. The ci-run failure has no lines
  # of its own, but the test has, so no tail.
  @ID-WATCH-36 @slice-172
  Scenario: A failed Go test is named with its package and its first error lines
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestRunnerLooksUpTheChecksThatOptIn (0.00s)
          evidence_test.go:101: the runner looked up 0 checks, want 2
      FAIL
      FAIL	github.com/donvargax/itos/v8/internal/check	0.012s
      ok  	github.com/donvargax/itos/v8/internal/ci	0.020s
      CI failed at: go test ./cmd/... ./internal/... (it exited 1)
      ##[error]Process completed with exit code 1.
      """
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's job "ci" has "log" "read"
    And its JSON's job "ci" lists these failures:
      | kind    | name                                           |
      | go-test | TestRunnerLooksUpTheChecksThatOptIn            |
      | ci-run  | go test ./cmd/... ./internal/... (it exited 1) |
    And its JSON's job "ci"'s failure "TestRunnerLooksUpTheChecksThatOptIn" has "package" "github.com/donvargax/itos/v8/internal/check"
    And its JSON's job "ci"'s failure "TestRunnerLooksUpTheChecksThatOptIn" has the lines:
      """
      evidence_test.go:101: the runner looked up 0 checks, want 2
      """
    And its stderr says "TestRunnerLooksUpTheChecksThatOptIn"
    And its stderr says "evidence_test.go:101: the runner looked up 0 checks, want 2"

  # A plain Go subtest, no godog: the parent's own line, printed after its
  # subtest as t.Errorf after t.Run prints it, is dropped, and only the leaf
  # is listed, by its full name, with its package and its own lines.
  @ID-WATCH-47 @slice-172
  Scenario: A failed Go subtest is named alone, by its full name, never its parent
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestPlanOrders (0.00s)
          --- FAIL: TestPlanOrders/a_task_before_its_check (0.00s)
              plan_test.go:52: the check ran before its task
          plan_test.go:60: 1 of 3 orders failed
      FAIL
      FAIL	github.com/donvargax/itos/v8/internal/plan	0.011s
      ##[error]Process completed with exit code 1.
      """
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" lists these failures:
      | kind    | name                                   |
      | go-test | TestPlanOrders/a_task_before_its_check |
    And its JSON's job "ci"'s failure "TestPlanOrders/a_task_before_its_check" has "package" "github.com/donvargax/itos/v8/internal/plan"
    And its JSON's job "ci"'s failure "TestPlanOrders/a_task_before_its_check" has the lines:
      """
      plan_test.go:52: the check ran before its task
      """

  # The log is this repository's own red feature run, as go test prints it
  # without -v (taken from a local run on 2026-10-23, the report trimmed):
  # godog's pretty print, its summary of failed steps, then go test's lines.
  # Each \e is an escape character, coloured as godog colours it, part of a
  # line and not the whole; the line that holds only a reset ends the entry
  # once stripped. The error's further lines, the world's report, start at
  # the margin. godog prints the scenario's "Scenario:" line twice; only the
  # summary's counts. go test's subtest for the scenario, with lines of its
  # own (suite.go:640), is the same failure; the parent TestFeatures, whose
  # own line "features_test.go:232: a scenario failed" comes after it, has a
  # failed subtest and so is no failure.
  @ID-WATCH-37 @slice-172
  Scenario: A failed godog scenario is named with its failing step and the step's error
    Given the watched run's job "ci" fails at its step "Run the named tests", its log, each \e in it an escape character:
      """
      \e[1;37mFeature:\e[0m itos push waits for the CI run it started

        \e[1;37mScenario:\e[0m A push waits for its run                     \e[1;30m# watch.feature:27\e[0m
          \e[32mWhen\e[0m \e[32mitos runs "push"\e[0m                            \e[1;30m# release_test.go:190 -> *world\e[0m
          \e[31mAnd\e[0m \e[31mits output says "ci: success"\e[0m              \e[1;30m# steps_test.go:544 -> *world\e[0m
          \e[1;31mafter scenario hook failed: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      , step error: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      \e[0m

      --- \e[31mFailed steps:\e[0m

        \e[31mScenario: A push waits for its run\e[0m \e[1;30m# watch.feature:27\e[0m
          \e[31mAnd its output says "ci: success"\e[0m \e[1;30m# watch.feature:31\e[0m
            \e[31mError: \e[0m\e[1;31mafter scenario hook failed: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      , step error: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      \e[0m


      1 scenarios (\e[31m1 failed\e[0m)
      7 steps (\e[32m6 passed\e[0m, \e[31m1 failed\e[0m)
      474.834233ms
      --- FAIL: TestFeatures (0.50s)
          --- FAIL: TestFeatures/A_push_waits_for_its_run (0.46s)
              suite.go:640: after scenario hook failed: the output does not say "ci: success"
                  exit 1
                  --- stdout
                  --- stderr
                  , step error: the output does not say "ci: success"
                  exit 1
                  --- stdout
                  --- stderr
          features_test.go:232: a scenario failed
      FAIL
      FAIL	github.com/donvargax/itos/v8/features	0.519s
      FAIL
      ##[error]Process completed with exit code 1.
      """
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" lists these failures:
      | kind     | name                     |
      | scenario | A push waits for its run |
    And its JSON's job "ci"'s failure "A push waits for its run" has the lines:
      """
      And its output says "ci: success" # watch.feature:31
      Error: after scenario hook failed: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      , step error: the output does not say "ci: success"
      exit 1
      --- stdout
      --- stderr
      """
    And its stderr says "A push waits for its run"

  # itos ci run prints the code proof's problems on stderr after its "CI
  # failed at the code proof:" line, each rule and message, then its fix:
  # the ci-run failure's lines, up to the blank line after them.
  @ID-WATCH-38 @slice-172
  Scenario: A code proof that fails is named with its mutation lines
    Given the watched run's job "ci" fails at its step "Run itos ci run", its log:
      """
      $ tools/bin/pinned itos-cc mutation run --since 1a2b3c4 --json cmd internal

      CI failed at the code proof: tools/bin/pinned itos-cc mutation run --since 1a2b3c4 --json cmd internal
      mutation.uncovered-statement: internal/vcs/port/vcs.go:34 in port#Within has an uncovered executable Go coverage block
        fix: cover the block with a test, or except it as equivalent

      ##[error]Process completed with exit code 1.
      """
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" lists these failures:
      | kind   | name                                                                                      |
      | ci-run | the code proof: tools/bin/pinned itos-cc mutation run --since 1a2b3c4 --json cmd internal |
    And its JSON's job "ci"'s failure "the code proof: tools/bin/pinned itos-cc mutation run --since 1a2b3c4 --json cmd internal" has the lines:
      """
      mutation.uncovered-statement: internal/vcs/port/vcs.go:34 in port#Within has an uncovered executable Go coverage block
        fix: cover the block with a test, or except it as equivalent
      """
    And its stderr says "mutation.uncovered-statement: internal/vcs/port/vcs.go:34"

  # A log that matches nothing known still says something: the last 20 lines
  # up to GitHub's line for the step's exit, never the cleanup after it, and
  # the 11 before them counted.
  @ID-WATCH-39 @slice-172
  Scenario: A failed job whose log matches nothing known is given its log's bounded tail
    Given the watched run's job "ci" fails at its step "Build", its log the lines "line 1" to "line 30", then GitHub's line for exit code 2 and its cleanup
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" lists these failures:
      | kind | name |
      | tail |      |
    And its JSON's job "ci"'s tail has 20 lines, from "line 12" to "##[error]Process completed with exit code 2."
    And its JSON's job "ci"'s tail has "more_lines" 11
    And its stderr says "line 30"
    And its stderr does not say "Cleaning up orphan processes"

  # Proposal: itos ci run's line for a step names its command and exit code
  # and nothing else, so a ci-run failure with no lines of its own brings the
  # tail too, which holds the step's own output (go vet's here).
  @ID-WATCH-40 @slice-172
  Scenario: A step itos ci run names with no lines of its own brings the log's tail too
    Given the watched run's job "ci" fails at its step "Run itos ci run", its log:
      """
      $ go vet ./...
      # github.com/donvargax/itos/v8/internal/z
      internal/z/z.go:12:2: fmt.Printf format %d has arg s of wrong type string

      CI failed at: go vet ./... (it exited 1)
      ##[error]Process completed with exit code 1.
      """
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" lists these failures:
      | kind   | name                       |
      | ci-run | go vet ./... (it exited 1) |
      | tail   |                            |
    And its JSON's job "ci"'s tail has 6 lines, from "$ go vet ./..." to "##[error]Process completed with exit code 1."
    And its stderr says "internal/z/z.go:12:2"

  # 410 is GitHub's answer for a log past its retention; a redirect that
  # leads nowhere or an answer cut short is the same. The nightly's report
  # once showed a reader refusing a log for its terminal escape sequences:
  # itos strips them (@ID-WATCH-37), so that is no reason here. The run's
  # verdict stands: exit 1, never 3 or 75 for a log. Its check run lists no
  # annotation, so nothing takes the log's place (slice 197).
  @ID-WATCH-41 @slice-172
  Scenario: A failed job whose log cannot be read keeps its step, says so, and the watch still exits 1
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestUnread (0.00s)
      """
    And the fake GitHub answers 410 for the log of the job "ci"
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's "ci" is "failure"
    And its JSON's job "ci" has "step" "Run the unit tests"
    And its JSON's job "ci" has "log" "unreadable"
    And its JSON's job "ci" has no "failures"
    And its stderr says "could not be read"

  # The log is where GitHub redirects it, a host a sandbox refuses; the
  # annotations come from api.github.com. The first is what a problem matcher
  # would make of the Go test's error line. GitHub's own line for the step's
  # exit is one too, at .github line 1, kept for its level, failure. The
  # warning between them, an action's deprecation, is left out: only
  # failure-level annotations, in the API's order. Through itos push's wait,
  # as itos ci watch's.
  @slice-197 @ID-WATCH-48
  Scenario: A failed job whose log is on a host itos cannot reach is named by its check run's annotations
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestRunnerLooksUpTheChecksThatOptIn (0.00s)
          evidence_test.go:101: the runner looked up 0 checks, want 2
      FAIL	github.com/donvargax/itos/v8/internal/check	0.012s
      ##[error]Process completed with exit code 1.
      """
    And the fake GitHub redirects the log of the job "ci" to a host that refuses it
    And the fake GitHub lists these annotations for the check run of the job "ci":
      | path                            | line | level   | message                               |
      | internal/check/evidence_test.go | 101  | failure | the runner looked up 0 checks, want 2 |
      | .github                         | 1    | warning | Node.js 20 actions are deprecated.    |
      | .github                         | 1    | failure | Process completed with exit code 1.   |
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push --json"
    Then itos exits with code 1
    And its JSON's job "ci" has "step" "Run the unit tests"
    And its JSON's job "ci" has "log" "unreadable"
    And its JSON's job "ci" lists these failures:
      | kind       | name                                |
      | annotation | internal/check/evidence_test.go:101 |
      | annotation | .github:1                           |
    And its JSON's job "ci"'s failure "internal/check/evidence_test.go:101" has "level" "failure"
    And its JSON's job "ci"'s failure ".github:1" has the lines:
      """
      Process completed with exit code 1.
      """
    And its JSON's job "ci"'s failure "internal/check/evidence_test.go:101" has the lines:
      """
      the runner looked up 0 checks, want 2
      """
    And its stderr says "could not be read"
    And its stderr says "internal/check/evidence_test.go:101"
    And its stderr says "the runner looked up 0 checks, want 2"
    And its stderr says "Process completed with exit code 1."
    And its stderr does not say "Node.js 20 actions are deprecated."

  # Proposal: an annotation is a failure under the log's limits, 20 a job and
  # 10 lines each, what is left out counted and said. 35 is more than
  # GitHub's page of 30 unless per_page asks for more, so a request that
  # does not ask counts 10 left out, not 15.
  @slice-197 @ID-WATCH-49
  Scenario: A job with more annotations and message lines than the limits names what it left out
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      ##[error]Process completed with exit code 1.
      """
    And the fake GitHub redirects the log of the job "ci" to a host that refuses it
    And the fake GitHub lists 35 annotations for the check run of the job "ci", each a failure of 12 message lines
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" has 20 failures, each a "annotation" with 10 lines and "more_lines" 2
    And its JSON's job "ci" has "more_failures" 15
    And its stderr says "more failures"
    And its stderr says "more lines"

  # A guard, which holds before the work and must after it: a log that is
  # read is read as before, its failures the log's alone, and no annotation
  # is asked for.
  @slice-197 @ID-WATCH-50 @guard
  Scenario: A failed job whose log is read asks for no annotation
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestPlanOrdersTheSteps (0.00s)
          plan_test.go:40: the steps are out of order
      FAIL	github.com/donvargax/itos/v8/internal/plan	0.010s
      """
    And the fake GitHub lists these annotations for the check run of the job "ci":
      | path    | line | level   | message                             |
      | .github | 1    | failure | Process completed with exit code 1. |
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" has "log" "read"
    And its JSON's job "ci" lists these failures:
      | kind    | name                   |
      | go-test | TestPlanOrdersTheSteps |
    And the fake GitHub was never asked for a check run's annotations

  # GitHub creates the tag when the release job publishes the release, while
  # the run is still going, so it is on the remote by the time the run
  # completes. Only the tags at the run's head count: the older release at
  # its parent is not named. The release's tag is annotated, so the remote
  # lists it as a tag object and, peeled, the commit: it counts by the commit
  # it points to, which a match on the unpeeled ref would miss.
  @ID-WATCH-42 @slice-172
  Scenario: A push whose run cut a release names the tag it published
    Given the remote's head is tagged "v1.2.0"
    And the clone has the commit "feat: add the archive" touching "a.txt"
    And the watched run's jobs "ci" and "release" succeed, and once itos has pushed, the remote's head is given the annotated tag "v1.3.0"
    When itos runs "push --json"
    Then itos exits with code 0
    And its JSON's "run.tags" is ["v1.3.0"]
    And its stderr says "v1.3.0"

  # Each failed job is named with its own step and failures, in the jobs'
  # order. The Windows job's log has CRLF line ends, as a Windows runner's
  # test output does, and its lines carry no CR.
  @ID-WATCH-43 @slice-172
  Scenario: Several failed jobs in one run are each named with their step and failures
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log:
      """
      --- FAIL: TestPlanOrdersTheSteps (0.00s)
          plan_test.go:40: the steps are out of order
      FAIL	github.com/donvargax/itos/v8/internal/plan	0.010s
      """
    And the watched run's job "platform (windows-latest)" fails at its step "Run the platform tests", its log:
      """
      --- FAIL: TestShellQuotesThePath (0.00s)
          shell_test.go:77: the path is not quoted
      FAIL	github.com/donvargax/itos/v8/internal/shell	0.015s
      """
    And the fake GitHub serves the log of the job "platform (windows-latest)" with CRLF line ends
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" has "step" "Run the unit tests"
    And its JSON's job "platform (windows-latest)" has "step" "Run the platform tests"
    And its JSON's job "ci" lists these failures:
      | kind    | name                   |
      | go-test | TestPlanOrdersTheSteps |
    And its JSON's job "platform (windows-latest)" lists these failures:
      | kind    | name                   |
      | go-test | TestShellQuotesThePath |
    And its JSON's job "platform (windows-latest)"'s failure "TestShellQuotesThePath" has the lines:
      """
      shell_test.go:77: the path is not quoted
      """
    And its stderr says "TestPlanOrdersTheSteps"
    And its stderr says "TestShellQuotesThePath"

  # A guard, which holds before the work and must after it: a green run asks
  # for no log, and its jobs and run gain no key.
  @ID-WATCH-44 @slice-172 @guard
  Scenario: A run that succeeds asks for no log and names nothing new
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "ci watch --json"
    Then itos exits with code 0
    And the fake GitHub was never asked for a job's log
    And its JSON's job "ci" has no "step"
    And its JSON's job "ci" has no "failures"
    And its JSON's run has no "tags"

  # Proposal: 10 lines a failure and 20 failures a job, enough for a handoff
  # and small enough for an agent's context; what is left out is counted in
  # the JSON and said in the text.
  @ID-WATCH-45 @slice-172
  Scenario: A job with more failures and lines than the limits names what it left out
    Given the watched run's job "ci" fails at its step "Run the unit tests", its log 25 failed Go tests of 14 error lines each
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its JSON's job "ci" has 20 failures, each a "go-test" with 10 lines and "more_lines" 4
    And its JSON's job "ci" has "more_failures" 5
    And its stderr says "more failures"
    And its stderr says "more lines"

  # A job the jobs API lists with no steps (one that failed before its first
  # step, say) has no step to name: it keeps today's lines and gains no
  # "step", while its log is still read.
  @ID-WATCH-46 @slice-172
  Scenario: A failed job listed with no steps names no step and is still given its log
    Given the watched run's job "ci" fails with no steps listed, its log:
      """
      the runner lost its connection
      ##[error]The operation was canceled.
      """
    When itos runs "ci watch --json"
    Then itos exits with code 1
    And its output says "ci: failure"
    And its JSON's job "ci" has no "step"
    And its JSON's job "ci" has "log" "read"
    And its JSON's job "ci" lists these failures:
      | kind | name |
      | tail |      |
