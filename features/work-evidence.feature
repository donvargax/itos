@phase-1
Feature: Final evidence is recorded by the same transaction that closes project work
  # #71, q-70 and q-76. Depends on slice-143's validated atomic batch close
  # (and slice-142 beneath it), not a new proof-reuse engine.
  # work done <id>... --evidence-file <path> captures a nonempty UTF-8
  # narrative once, before checks or CI waiting. Paths are relative to the
  # invocation folder. Normalize CRLF to LF and trim outer blank lines;
  # reject NUL or whitespace-only input. Missing/unreadable files exit 3;
  # invalid text or flag syntax exits 2, before acceptance checks or CI.
  # After ordinary validation, quote the narrative in the single close
  # commit body, explicitly for the IDs actually transitioning to done.
  # Quote it so supplied text cannot create commit trailers or commands.
  # Retain the tool's independent CI/result attribution. The narrative is
  # a recorded claim, never a proof provider, instruction or exemption.
  # No extra associated docs commit, task why edit, registry evidence field,
  # source-file staging, JSON evidence blob or reusable receipt is needed.
  # Existing JSON commit fields locate the durable Git record.
  # All-already-done requests with evidence refuse (exit 1) rather than
  # discard it; ordinary no-evidence idempotency is unchanged. Mixed batches
  # keep already-done results unchanged and make no fresh proof claim for
  # them. --evidence-file with --by is usage 2, not retroactive evidence.
  # Project mode only: stealth refuses the evidence flag before checks,
  # exit 2; ordinary stealth closure still works. No private journal here.
  # Separately committed evidence reuse is p1-reusable-completion-evidence.

  Background:
    Given a repository whose ledger has the task "T-001"

  @wip @ID-WORK-137 @slice-158
  Scenario: Evidence follows validated implementation directly into the close commit
    Given task "T-001" is doing and queued with a current recording acceptance check
    And its implementation commit is pushed and has a verified successful CI run
    And an untracked UTF-8 evidence file records the final observations
    When I close "T-001" with "work done" and that evidence file
    Then itos exits with code 0
    And the current acceptance check ran and the ordinary implementation CI requirement passed
    And exactly one new commit closes and dequeues "T-001" and removes its done_when
    And that commit's body contains the quoted observations and independently attributed CI result
    And there was no intermediate evidence commit requiring another CI result before closure
    And the ledger why is unchanged and no evidence prose was added to the registry
    And the source evidence file is unchanged, untracked and absent from the close commit
    And the evidence can be read from the close commit that work show lists

  @wip @ID-WORK-138 @slice-158
  Scenario Outline: Invalid evidence inputs fail before checks, waiting or writes
    Given "T-001" would otherwise be ready to close
    And <condition>
    When I attempt completion with <arguments>
    Then itos exits with code <code>
    And no acceptance check or CI request has run
    And the registry, ledger, queue, HEAD and index are unchanged
    And its diagnostic identifies the invalid evidence input or unsupported combination

    Examples:
      | condition                                      | arguments                                      | code |
      | the evidence file does not exist                | --evidence-file missing.txt                     | 3    |
      | the evidence file cannot be read                | --evidence-file unreadable.txt                  | 3    |
      | the evidence file is whitespace only            | --evidence-file empty.txt                       | 2    |
      | the evidence file is invalid UTF-8               | --evidence-file invalid.txt                     | 2    |
      | the evidence file contains NUL                  | --evidence-file nul.txt                         | 2    |
      | a valid evidence file exists                    | --evidence-file                                 | 2    |
      | a valid evidence file exists                    | --evidence-file evidence.txt --by T-002          | 2    |
      | the config is stealth and valid evidence exists | --evidence-file evidence.txt                    | 2    |

  @wip @ID-WORK-139 @slice-158
  Scenario Outline: Recorded claims cannot replace current acceptance or CI gates
    Given the evidence file claims that every check and CI run is green
    But "T-001" has <blocking condition>
    When I close "T-001" with "work done" and that evidence file
    Then completion retains its existing refusal or wait outcome for that condition
    And no close or evidence commit was created
    And the registry status, ledger acceptance specification and queue are unchanged
    And the evidence text was not treated as proof or a reason to skip validation

    Examples:
      | blocking condition             |
      | a currently failing check      |
      | an associated unpushed commit  |
      | a still-wip required scenario  |
      | a required failed CI run       |
      | a CI run that times out        |

  @wip @ID-WORK-140 @slice-158
  Scenario: Batch evidence belongs to the items actually closed in the shared transaction
    Given "T-001" and "slice-7" are ready to close with independently satisfied current validation
    And "T-002" is already done
    And a valid evidence file describes the batch
    When itos runs "work done T-002 slice-7 T-001 --evidence-file evidence.txt --json"
    Then itos exits with code 0
    And the one close commit records evidence for "slice-7" and "T-001" in that request order
    And "T-002" remains an unchanged result with no new proof attribution
    And all transitioning items are closed and dequeued in that same commit
    And the existing batch JSON shape is retained and its commit fields locate that close commit
    And no evidence field, body or receipt is added to the JSON result

  @wip @ID-WORK-141 @slice-158
  Scenario: An evidence request cannot silently disappear into an already-done no-op
    Given "T-001" is already done and a valid evidence file exists
    When itos runs "work done T-001 --evidence-file evidence.txt"
    Then itos exits with code 1 and says no item is closing so evidence was not recorded
    And no file, HEAD or index changed
    And it has not appended evidence to an earlier close or created a new record
    When itos runs "work done T-001" without the evidence flag
    Then it retains the ordinary unchanged success

  @wip @ID-WORK-142 @slice-158
  Scenario: Evidence is captured once and remains literal narrative
    Given "T-001" is ready to close and waits for its successful CI result
    And its evidence file contains CRLF lines, a shell substitution, and trailer-looking lines naming an unrelated item
    And the fixture replaces the source file's content while CI is being waited for
    When I close "T-001" with the evidence file from a project subfolder
    Then itos exits with code 0
    And the close commit quotes the originally captured narrative with normalized LF endings
    And it has not included the replacement content in the evidence section
    And no command from the narrative has run
    And the actual parsed commit links name only the ordinary closing items
    And itos has not overwritten the fixture's replacement source-file content

  @wip @ID-WORK-143 @slice-158
  Scenario: Evidence preserves the existing revalidation boundary after a CI wait
    Given "T-001" is ready to close and a valid evidence file exists
    And unrelated registry ownership and queue changes arrive while CI is waited for
    When I close "T-001" with the evidence file after the successful run
    Then completion preserves those unrelated changes in the atomic close
    And evidence is recorded only with the successful close commit
    When a separate attempt instead receives a newly failing acceptance check during its CI wait
    Then the existing close revalidation refuses that attempt
    And no close or evidence record is made for that attempt

  @wip @ID-WORK-144 @slice-158
  Scenario: A rejected close commit rolls back the transaction without staging the evidence source
    Given "T-001" is ready to close and queued with acceptance checks
    And a valid evidence source and another unrelated file have pre-existing staged and unstaged edits
    And the commit hook will reject the close commit
    When I close "T-001" with that evidence source
    Then itos reports the commit rejection
    And the registry, ledger checks and queue are restored together
    And HEAD and all pre-existing staging and unstaged source edits are preserved
    And no additional evidence data file or partial close remains
