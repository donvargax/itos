@phase-1
Feature: A push waits for GitHub to register the workflow it just added
  # #56: registration grace is positive evidence from the actual push,
  # not a generic retry of any 404. Compare the captured post-rebase remote
  # base with the exact pushed SHA, never the later working tree or HEAD.
  # A new branch has the empty base tree. A readable existing base without
  # the file and a regular blob at the pushed SHA prove addition. Unknown
  # history proves nothing. Only literal workflow basenames ending .yml or
  # .yaml map to .github/workflows/<name>; other identifiers get no grace.
  # Until the first successful workflow-runs listing, that endpoint's 404
  # means no run yet, not a transport failure. Keep the existing interval
  # and deadline; do not consume the six-error allowance for registration.
  # Timeout stays 75 (slice 86), despite the report's suggested exit 3.
  # Do not extend this grace to standalone watches, range/status readers,
  # jobs endpoints, or covering-run selection. No fetch just for eligibility.
  # Fixtures advance on requests, using short configured waits; no external
  # sleeps or live GitHub. Check cadence and error classification cheaply.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs
    And ci.watch asks a fake GitHub, which reports the run "https://ci.example/runs/1"
    And the watched workflow basename is "ci.yml"

  @wip @ID-WATCH-27 @slice-153
  Scenario Outline: A newly added workflow may take more than six looks to register
    Given the remote base has no ".github/workflows/ci.yml"
    And the clone has committed that workflow followed by a separate README change
    And the fake GitHub returns 404 for the first eight listings of that workflow's runs, then a successful run
    And the configured watch deadline allows all nine looks
    When itos runs "<command>"
    Then itos exits with code 0
    And the remote branch has the exact pushed commit
    And the fake GitHub saw the pushed SHA on every run lookup
    And itos did not abandon registration after six looks
    And normal successful-watch output conventions are preserved for that command

    Examples:
      | command     |
      | push        |
      | push --json |
      | push -q     |

  @wip @ID-WATCH-28 @slice-153
  Scenario: Registration that never completes times out without pretending the push failed
    Given the push range adds the watched workflow
    And every listing of that workflow's runs returns 404
    And ci.watch has a short timeout
    When itos runs "push --json"
    Then itos exits with code 75
    And its JSON records "ci" as "timeout" with no invented run
    And its diagnostic says the workflow may still be registering and names the configured workflow
    And its diagnostic names "itos ci watch" with the exact pushed SHA
    And the remote branch still has the pushed commit
    And polling obeyed the configured interval and original overall deadline
    And itos did not push again or fetch history for registration eligibility

  @wip @ID-WATCH-29 @slice-153
  Scenario Outline: An unproved workflow addition does not turn a permanent 404 into registration grace
    Given the pushed range has <condition>
    And the first workflow-runs listing returns 404
    When itos runs "push"
    Then itos exits with code 3 after that first listing
    And the remote branch still has the pushed commit
    And no additional run lookup or history fetch was made to infer registration

    Examples:
      | condition                                                        |
      | the watched workflow already present at the remote base           |
      | only an uncommitted working-tree copy of the watched workflow      |
      | no watched workflow at the pushed SHA                             |
      | an unreadable existing remote-base tree                           |
      | a numeric workflow identifier rather than a workflow basename     |
      | a workflow identifier outside the supported basename form         |

  @wip @ID-WATCH-30 @slice-153
  Scenario Outline: New-workflow grace does not hide other permanent API failures
    Given the push range adds the watched workflow
    And the fake GitHub gives <failure>
    When itos runs "push"
    Then itos exits with code 3
    And the permanent failure is reported using the existing error convention
    And registration grace has not retried that failure

    Examples:
      | failure                                                            |
      | 401 from the workflow-runs listing                                  |
      | a non-rate-limit 403 from the workflow-runs listing                  |
      | a valid run listing followed by 404 from that run's jobs endpoint    |
      | malformed JSON in a successful workflow-runs response                |

  @wip @ID-WATCH-31 @slice-153
  Scenario: A successful run listing ends registration grace even while its run list is empty
    Given the push range adds the watched workflow
    And the fake GitHub first gives 404, then a successful empty workflow-runs listing, then 404
    When itos runs "push"
    Then itos exits with code 3 on the final 404
    And the empty successful listing was treated as an ordinary run not started yet
    And the original deadline was not reset

  @wip @ID-WATCH-32 @slice-153
  Scenario: The first push of a branch can establish addition without an old remote tip
    Given the clone is pushing a new branch with no previous remote tip
    And its pushed tree contains the watched workflow as a regular file
    And the fake GitHub gives registration 404s followed by a successful run
    When itos runs "push"
    Then itos exits with code 0
    And the new remote branch has the exact pushed commit
    And the workflow addition was evaluated against the empty tree
    And no remote fetch was added just to establish that empty base

  @wip @ID-WATCH-33 @slice-153
  Scenario: A rebase uses the actual remote base rather than the clone's original base
    Given the clone originally based its workflow and README commit on a tree without the workflow
    And another session has since pushed the identical workflow to the remote
    And the clone can rebase its README change without conflict
    And the first workflow-runs listing returns 404
    When itos runs "push"
    Then itos exits with code 3
    And the remote branch contains both sessions' changes
    And no registration grace was granted for a workflow already present at the post-rebase remote base
    And the run lookup used the final pushed SHA

  @wip @ID-WATCH-34 @slice-153
  Scenario: The registration exception belongs only to the waiting push that proved addition
    Given the push range adds the watched workflow
    And the fake GitHub would return registration 404s
    When itos runs "push --no-wait"
    Then itos exits with code 0
    And the fake GitHub was never asked about a run
    When itos runs "ci watch"
    Then itos exits with code 3 on its first workflow-runs 404
    And the earlier push has not left persistent registration-grace state
