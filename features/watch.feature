@phase-1
Feature: itos push waits for the CI run it started, and itos ci watch for any commit
  A push was done when git said so, and the CI run it started was watched by
  hand: the run looked up by the commit's full SHA, a watch armed, each job's
  result read, and an agent's report of a green run taken on trust or
  checked again. With ci.watch in the config, itos push waits for the run
  of the commit it pushed, prints each job's result as it finishes and the
  run's address, and exits with the run's result: 0 when it succeeded, 1
  when it failed, naming the failed jobs. The commits are pushed either way;
  only the waiting is new. --no-wait pushes and returns, as before. A run
  still going after ci.watch.timeout seconds exits 75 (slice 86) and names
  itos ci watch <sha>, which waits for any commit's run the same way, HEAD's
  by default.

  ci.watch is opt-in (the user's call, 2026-10-03): its provider is none by
  default, so a config without it pushes exactly as before. github reads the
  run of ci.watch.github.workflow for the commit through GitHub's API, once
  per ci.watch.interval seconds, with a token from ci.range.github.token_env,
  else from gh auth token; with neither it exits 3 naming both, before any
  request. A refusal from the API ends the watch with exit 3. v5.0.0 removed
  the command provider (slice 85), which ran a repository's own command on a
  push. Here GitHub is a fake one, a server of the scenario's that itos is
  pointed at through GITHUB_API_URL, so nothing reaches a network.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"

  @ID-WATCH-01 @slice-51
  Scenario: A push waits for its run, printing each job's result, and exits 0 when it succeeds
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And its output says "ci: success"
    And its output says "platform: success"
    And its output says "https://ci.example/runs/1"
    And the fake GitHub was asked for the run of the clone's HEAD

  # The commits are on the remote whatever the run says: the push is done,
  # and the exit code says what CI made of it.
  @ID-WATCH-02 @slice-51
  Scenario: A run that fails makes the push exit 1, naming the failed job, its commits pushed
    Given the watched run's job "ci" fails and its job "platform" succeeds
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 1
    And its output says "ci: failure"
    And the remote's branch has "chore: tidy the readme"

  # Each poll reports one more job done; a job's line is printed once, when
  # it finishes, so a watcher of the output sees the run as it goes.
  @ID-WATCH-03 @slice-51
  Scenario: Each job's result is printed once, as it finishes
    Given the watched run finishes its job "ci" first and its job "platform" one poll later, both succeeding
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And its output says "ci: success" once, before "platform: success"

  @ID-WATCH-04 @slice-51
  Scenario: push --no-wait pushes and returns without watching
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the fake GitHub was never asked about a run
    And the remote's branch has "chore: tidy the readme"

  @ID-WATCH-05 @slice-51
  Scenario: Without ci.watch a push does not wait, as before
    Given ci.watch's provider is "none"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And the fake GitHub was never asked about a run

  @ID-WATCH-07 @slice-51
  Scenario: itos ci watch waits for HEAD's run with no argument
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "ci watch"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/1"
    And the fake GitHub was asked for the run of the clone's HEAD

  # Slice 85: the command provider's run that was no JSON object is gone with
  # it; GitHub's refusal is the look that ends a github watch.
  @ID-WATCH-08 @slice-51
  Scenario: A watch GitHub's API refuses exits 3
    Given the fake GitHub refuses the token
    When itos runs "ci watch"
    Then itos exits with code 3

  # GH_TOKEN is never in a scenario's environment, GITHUB_TOKEN is taken out
  # of it, and the PATH has no gh, so neither way to sign in is there: it
  # stops before any request.
  @ID-WATCH-09 @slice-51
  Scenario: The github provider with no token and no gh exits 3, naming both ways to sign in
    Given no GitHub token in the environment
    And no gh on the PATH
    When itos runs "ci watch"
    Then itos exits with code 3
    And its output says "GH_TOKEN"
    And its output says "gh auth login"

  # Slice 56 (the user's call, 2026-10-03): an item's registry commits, its
  # take and its close, are written and checked by itos (work check runs
  # before each), and waiting a whole CI run for each made one item cost
  # three waits. A push whose commits touch only the work registry pushes
  # and returns, naming itos ci watch for anyone who wants that run; any
  # other path in the range, prose included, waits as before, since a prose
  # range can fail (vp check did, the day this was written).
  @ID-WATCH-10 @slice-56
  Scenario: A push whose commits touch only the work registry does not wait for CI, and says how to wait for it
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "docs: take slice-9" touching "tasks/work-items.yaml"
    When itos runs "push"
    Then itos exits with code 0
    And the fake GitHub was never asked about a run
    And its output says "itos ci watch"
    And the remote's branch has "docs: take slice-9"

  @ID-WATCH-11 @slice-56
  Scenario: A push whose commits touch the registry and anything else waits as before
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "docs: take slice-9" touching "tasks/work-items.yaml"
    And the clone has the commit "docs: describe the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And its output says "ci: success"

  # Slice 86 (decision 35, the user's calls, 2026-10-06): v6.0.0. Exit 75,
  # sysexits' EX_TEMPFAIL, is a failure that may pass if run again unchanged,
  # so a retry wrapper can act on it without --json: a look at the run that
  # fails for a server error or a rate limit itos gave up retrying, and a run
  # still going at ci.watch.timeout. A refused token, no token and a gh that
  # does not answer stay 3, since running again unchanged does not help.
  # @ID-WATCH-12 replaces @ID-WATCH-06, which the feat removes, marked
  # breaking.
  @ID-WATCH-12 @slice-86
  Scenario: A run still going after ci.watch.timeout exits 75, naming itos ci watch
    Given the watched run never finishes
    And ci.watch.timeout is 1
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 75
    And its output says "itos ci watch"
    And the remote's branch has "chore: tidy the readme"

  # itos gives up retrying after six looks in a row fail so, a minute of them
  # at the default ci.watch.interval: a blip is looked past, and an outage is
  # handed back with 75 well before ci.watch.timeout.
  @ID-WATCH-13 @slice-86
  Scenario: A watch whose every look gets a server error exits 75
    Given the fake GitHub answers every look with a server error
    When itos runs "ci watch"
    Then itos exits with code 75

  # Bug 41 (found 2026-10-03, reported as issue #17; exit 75 the user's call, 2026-10-06): with
  # cancel-in-progress on CI's concurrency group, a newer push to the branch
  # cancels the run still going, and the newer run checks the cancelled run's
  # commits too (ci.range starts at the last green run). ci watch and push
  # read the cancelled run as a failure and exited 1, so an agent chased a red
  # that was not there. A cancelled run is no verdict: itos follows the newest
  # run of ci.watch.github.workflow on the branch whose head has the commit as
  # an ancestor, waits for it as for its own, and exits with its result,
  # saying which run it followed. The newer head need not be in the clone (a
  # push's run is cancelled by someone else's push): GitHub's compare API, or
  # a fetch, answers the ancestry. A run cancelled with no newer run to follow
  # exits 75, decision 35's "may pass if run again unchanged", not 1, which
  # says a check said no. --json: ci gains "cancelled"; a followed run is
  # "run", and the cancelled one's address is "superseded".
  @ID-WATCH-14 @bug-41
  Scenario: A run cancelled by a newer push is judged by the newer run
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    And the clone has the commit "chore: tidy the notes" touching "NOTES.md"
    And the fake GitHub reports the run of the clone's HEAD~1 cancelled
    And the fake GitHub reports the run "https://ci.example/runs/2" of the clone's HEAD, whose jobs "ci" and "platform" succeed
    When itos runs "ci watch HEAD~1"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/2"

  @ID-WATCH-15 @bug-41
  Scenario: The newer run failing fails the watch of the cancelled one
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    And the clone has the commit "chore: tidy the notes" touching "NOTES.md"
    And the fake GitHub reports the run of the clone's HEAD~1 cancelled
    And the fake GitHub reports the run "https://ci.example/runs/2" of the clone's HEAD, whose job "ci" fails
    When itos runs "ci watch HEAD~1"
    Then itos exits with code 1
    And its output says "ci: failure"

  @ID-WATCH-16 @bug-41
  Scenario: A push whose run another push cancelled waits for that push's run
    Given the clone has the commit "chore: tidy the readme" touching "README.md"
    And once itos has pushed, another clone pushes the commit "chore: tidy the notes"
    And the fake GitHub reports the pushed commit's run cancelled, and the run "https://ci.example/runs/2" of the remote's head, whose jobs "ci" and "platform" succeed
    When itos runs "push"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/2"

  @ID-WATCH-17 @bug-41
  Scenario: A cancelled run with no newer run exits 75
    Given the fake GitHub reports the run of the clone's HEAD cancelled
    When itos runs "ci watch"
    Then itos exits with code 75
    And its output says "cancelled"

  # Bug 40 (the code review of 07b0e6d, 2026-10-05): only a 5xx or a 429
  # counted as a look worth retrying, but GitHub answers its primary rate
  # limit, and often its secondary one, with 403 (x-ratelimit-remaining: 0,
  # or a retry-after header, or a message naming the rate limit), so a long
  # watch, or sessions sharing a token, ended ci watch and itos push with
  # exit 3, as if the token were refused. A 403 that is a rate limit is a
  # look worth retrying, waiting as retry-after or x-ratelimit-reset says when
  # either is given; a 403 that is not one stays 3 (@ID-WATCH-12's comment).
  @ID-WATCH-18 @bug-40 @wip
  Scenario: A look GitHub refuses with a rate limit is retried, and the watch ends with the run
    Given the fake GitHub answers the first look with a 403 rate limit
    And the watched run's jobs "ci" and "platform" succeed
    When itos runs "ci watch"
    Then itos exits with code 0
    And its output says "ci: success"

  @ID-WATCH-19 @bug-40 @wip
  Scenario: A watch whose every look is rate limited exits 75, never 3
    Given the fake GitHub answers every look with a 403 rate limit
    When itos runs "ci watch"
    Then itos exits with code 75
