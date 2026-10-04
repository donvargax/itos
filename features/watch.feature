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
  still going after ci.watch.timeout seconds exits 3 and names itos ci watch
  <sha>, which waits for any commit's run the same way, HEAD's by default.

  ci.watch is opt-in (the user's call, 2026-10-03): its provider is none by
  default, so a config without it pushes exactly as before. github reads the
  run of ci.watch.github.workflow for the commit through GitHub's API, with
  a token from ci.range.github.token_env, else from gh auth token; with
  neither it exits 3 naming both, before any request. command runs
  ci.watch.command with {sha} filled in, once per ci.watch.interval seconds,
  and reads one JSON object from its stdout: {"url", "status" (queued,
  in_progress or completed), "conclusion" (when completed), "jobs":
  [{"name", "status", "conclusion"}]}. A command that fails, or prints what
  is not that object, exits 3. Here the command is a script of the
  scenario's that reports a run it was given, so nothing reaches a network.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And ci.watch runs a command that reports the run "https://ci.example/runs/1"

  @ID-WATCH-01 @slice-51
  Scenario: A push waits for its run, printing each job's result, and exits 0 when it succeeds
    Given the watched run's jobs "ci" and "platform" succeed
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And its output says "ci: success"
    And its output says "platform: success"
    And its output says "https://ci.example/runs/1"
    And the watch command was given the full SHA of the clone's HEAD

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
    And the watch command was never run
    And the remote's branch has "chore: tidy the readme"

  @ID-WATCH-05 @slice-51
  Scenario: Without ci.watch a push does not wait, as before
    Given ci.watch's provider is "none"
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 0
    And the watch command was never run

  @ID-WATCH-06 @slice-51
  Scenario: A run still going after ci.watch.timeout exits 3, naming itos ci watch
    Given the watched run never finishes
    And ci.watch.timeout is 1
    And the clone has the commit "chore: tidy the readme" touching "README.md"
    When itos runs "push"
    Then itos exits with code 3
    And its output says "itos ci watch"
    And the remote's branch has "chore: tidy the readme"

  @ID-WATCH-07 @slice-51
  Scenario: itos ci watch waits for HEAD's run with no argument
    Given the watched run's jobs "ci" and "platform" succeed
    When itos runs "ci watch"
    Then itos exits with code 0
    And its output says "https://ci.example/runs/1"
    And the watch command was given the full SHA of the clone's HEAD

  @ID-WATCH-08 @slice-51
  Scenario: A watch command that prints no run object exits 3
    Given the watch command prints "not json"
    When itos runs "ci watch"
    Then itos exits with code 3

  # GITHUB_TOKEN and GH_TOKEN are never in a scenario's environment, and the
  # PATH has no gh, so neither way to sign in is there: it stops before any
  # request.
  @ID-WATCH-09 @slice-51
  Scenario: The github provider with no token and no gh exits 3, naming both ways to sign in
    Given ci.watch's provider is "github"
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
    And the watch command was never run
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
