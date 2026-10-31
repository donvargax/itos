@phase-1
Feature: itos next walks each item kind's path from work.paths: a kind whose spec_review is skip needs no verdict on its spec
  The v9 plan's "A path per kind" (docs/plans/v9-coordinator.md): the steps
  an item walks are config, one row per kind, read by itos next, so a
  typo-level task does not walk every state. slice-209 gives every in-flight
  item under push.mode pr the same states; this slice lets a kind skip the
  spec review, the first step piloted, with a default that is slice-209's
  flow.

  The config. work.paths maps an item kind, slice, bug or task, to its
  steps; this slice has one step, spec_review, whose value is required or
  skip. required is the default for every kind, whether the kind or its step
  is left out, and gives slice-209's states unchanged. The default is in the
  one table of defaults, so config get work.paths.<kind>.spec_review prints
  required for a kind the config leaves out. config check refuses what the
  schema does not know, as it refuses any key or value, exit 2 with the
  problem's rule and fix: a kind other than slice, bug and task
  (config-unknown-key, its fix naming the three), a step other than
  spec_review (config-unknown-key) and a value other than required or skip
  (config-enum). An item's kind is the one itos next reads (slice-203): its
  kind key, else its id.

  The rule. Under push.mode pr, for an item whose kind's spec_review is
  skip, every state slice-209 derives from the spec's verdict is
  spec-accepted instead, driver take, itos work take <id>: a todo item whose
  open pull request has no counted verdict at its head (slice-209's
  spec-review) and one whose verdict there is changes (slice-209's
  spec-changes). So next never asks a reviewer for that item. Everything
  else is as slice-209 has it: the forge is asked the same calls, observed
  is filled the same, the verdict included (a marker posted anyway is
  reported and decides nothing), and every other state keeps its order and
  its step: spec-proposed (a spec with no open or merged pull request, the
  drafter's to push, which opens it) and, past the spec, implementing,
  handed-back, merging, repair, merged and blocked. spec-changes from the
  journal (an implementer's spec_defect, which no verdict decides) stays the
  drafter's. A kind's path changes nothing for another kind. Under push.mode
  trunk nothing changes: in-flight, nothing asked.

  The revision digests the path of the item's kind as loaded, the default
  included, beside slice-203's and slice-209's facts, for an item whose kind
  has a path (slice, bug, task) whatever its state: a change to work.paths
  that changes that kind's path gives a new revision, and a change to
  another kind's path, or the default written out, leaves it as it was.

  Not in this slice, each an idea when wanted: the C value (spec review
  required only for the classes the config names, ADR-0061's high blast
  radius as data); red_first (slice-207's check that a test commit comes
  first); change_review (push.pr.review_by per kind); a path for the idea
  kind; a key of next --json naming the path.

  The steps. Those slice-209 uses, as its spec writes them: its own "the
  recording gh answers that the pull request's head's checks are <state>"
  and slice-207's "the file <path> holds the lines, the saved values filled
  in", both in ID-NEXTPATHS-04, and slice-206's verdict markers, on main.
  One is new: "the config sets <key> to the YAML <value>, in place of its
  earlier value" drops the scenario's earlier setting of the key and writes
  the config again, not committed, since a second setting of the same key
  under work would write it twice in the work mapping and make the config
  unreadable.

  Background:
    Given a repository whose ledger has the task "T-001"
    And a clone of it, where itos runs

  # The slice's point: a task whose kind skips the review needs no verdict.
  # With its spec, the ledger check, on its pushed branch and no pull
  # request yet it is spec-proposed, the drafter's to push, as slice-209
  # has it. With a draft pull request and no marker, which slice-209 calls
  # spec-review, it is spec-accepted: the driver takes it. A marker asking
  # for changes at the head is reported and decides nothing.
  @ID-NEXTPATHS-01 @wip @slice-211
  Scenario: Under push.mode pr, a task whose kind skips spec review is spec-accepted on its draft pull request with no verdict, and a changes verdict does not send it back
    Given the config sets "work.paths" to the YAML "{ task: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the clone is on the new branch "T-001"
    And the task "T-001" has the committed check "true"
    And git pushes HEAD to the remote's new branch "T-001"
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-proposed"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "T-001", "--role", "drafter"]}
    Given the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "T-001"
    When itos runs "next T-001"
    Then itos exits with code 0
    And stdout contains the complete line "T-001: spec-accepted; next: driver take; run: itos work take T-001" exactly once
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "task"
    And its JSON's "state" is "spec-accepted"
    And its JSON's "next" is {"actor": "driver", "action": "take", "argv": ["itos", "work", "take", "T-001"]}
    And its JSON's "observed.pr.number" is 7
    And its JSON's "observed.pr.draft" is true
    And its JSON's "observed.pr.head" holds the full SHA of the head of the remote's branch "T-001"
    And its JSON's "observed.verdict" is null
    Given the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "T-001" at the head of the remote's branch "T-001" as round 1: "changes"
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-accepted"
    And its JSON's "next" is {"actor": "driver", "action": "take", "argv": ["itos", "work", "take", "T-001"]}
    And its JSON's "observed.verdict" is "changes"

  # The same task, its path the default: the config's work.paths names the
  # slices alone, so a build that read any path in the config as skip, or
  # took skip for the default, says spec-accepted. The verdict decides, as
  # slice-209 has it: changes at the head sends it back to the drafter.
  @ID-NEXTPATHS-02 @wip @slice-211
  Scenario: Under push.mode pr, a task whose kind's path is the default is in spec review on its draft pull request, and a changes verdict sends it to the drafter
    Given the config sets "work.paths" to the YAML "{ slice: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the clone is on the new branch "T-001"
    And the task "T-001" has the committed check "true"
    And git pushes HEAD to the remote's new branch "T-001"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "T-001"
    When itos runs "next T-001"
    Then itos exits with code 0
    And stdout contains the complete line "T-001: spec-review; next: reviewer review; run: itos brief T-001 --role reviewer" exactly once
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-review"
    And its JSON's "next" is {"actor": "reviewer", "action": "review", "argv": ["itos", "brief", "T-001", "--role", "reviewer"]}
    Given the recording gh answers that the pull request has a comment by "someone" recording the "spec" review of "T-001" at the head of the remote's branch "T-001" as round 1: "changes"
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-changes"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "T-001", "--role", "drafter"]}
    And its JSON's "observed.verdict" is "changes"

  # The other direction: the config skips the tasks' review, so a build
  # that skipped every kind's once any kind's is skipped says spec-accepted
  # for the slice.
  @ID-NEXTPATHS-03 @wip @slice-211
  Scenario: Under push.mode pr, a slice under a config that skips the tasks' spec review is still in spec review
    Given the config sets "work.paths" to the YAML "{ task: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by nobody with the status "todo"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "kind" is "slice"
    And its JSON's "state" is "spec-review"
    And its JSON's "next" is {"actor": "reviewer", "action": "review", "argv": ["itos", "brief", "slice-9", "--role", "reviewer"]}
    And its JSON's "observed.verdict" is null

  # Past the spec, skip changes nothing. slice-9 is doing on its draft with
  # no journal: implementing. slice-8's pull request is ready, its checks
  # passed, no thread: merging, the forge's. Then the implementer finds
  # slice-9's spec defective: spec-changes, the drafter's, since the journal
  # decides it and no verdict does; a build that read every spec-changes as
  # a verdict's would say spec-accepted, and one that let skip reach a doing
  # item would say it for the first run too. The two branches are pushed one
  # after the other, slice-9's from slice-8's: each spec is read at its own
  # origin/<item>.
  @ID-NEXTPATHS-04 @wip @slice-211
  Scenario: Under push.mode pr, an item whose kind skips spec review is implementing, merging or sent back on a spec defect as slice-209 has it
    Given the config sets "work.paths" to the YAML "{ slice: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-8" owned by "ana" with the status "doing"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-8"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-8"
    And git pushes HEAD to the remote's new branch "slice-8"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-B-01" tagged "@wip @slice-9"
    And the last commit SHA is saved as "spec"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the pull request "https://github.example/o/r/pull/8" open for the branch "slice-8"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/9" open for the branch "slice-9"
    And the recording gh answers that the pull request's head's checks are "SUCCESS"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's "next" is {"actor": "implementer", "action": "implement", "argv": ["itos", "brief", "slice-9", "--role", "implementer"]}
    And its JSON's "observed.pr.number" is 9
    When itos runs "next slice-8 --json"
    Then itos exits with code 0
    And its JSON's "state" is "merging"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON's "observed.pr.number" is 8
    And its JSON's "observed.checks" is "success"
    Given the file ".git/itos/attempts/slice-9.jsonl" holds the lines, the saved values filled in:
      """
      {"schema":1,"role":"implementer","attempt":1,"request":"sha256:0000000000000000000000000000000000000000000000000000000000000000","base":"<spec>","outcome":"spec_defect","head":null,"notes":"The scenario asks the impossible.","verdict":"accepted","sha256":"sha256:1111111111111111111111111111111111111111111111111111111111111111"}
      """
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "spec-changes"
    And its JSON's "next" is {"actor": "drafter", "action": "specify", "argv": ["itos", "brief", "slice-9", "--role", "drafter"]}
    And its JSON's "observed.attempt.outcome" is "spec_defect"

  # No push.mode: trunk, where slice-209 keeps in-flight and asks nothing.
  # A build that applied skip there would say spec-accepted, or ask the gh.
  @ID-NEXTPATHS-05 @wip @slice-211
  Scenario: Under push.mode trunk, a task whose kind skips spec review stays in flight and the forge is not asked
    Given the config sets "work.paths" to the YAML "{ task: { spec_review: skip } }"
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the task "T-001" owned by nobody with the status "todo" and no why
    And the task "T-001" has the committed check "true"
    When itos runs "next T-001 --json"
    Then itos exits with code 0
    And its JSON's "state" is "in-flight"
    And its JSON's "next" is {"actor": "forge", "action": "observe"}
    And its JSON has no "observed"
    And the recording gh was never called

  # slice-9 is doing, so its state is implementing whatever its path says:
  # a revision of the state alone, or of slice-203's and slice-209's facts
  # alone, gives "first" again. Only the slices' path changes, not
  # committed, since the config is read from the working tree.
  @ID-NEXTPATHS-06 @wip @slice-211
  Scenario: Under push.mode pr, a change to the path of the item's kind gives a new revision
    Given the config sets "work.paths" to the YAML "{ bug: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's revision is saved as "first"
    Given the config sets "work.paths" to the YAML "{ bug: { spec_review: skip }, slice: { spec_review: skip } }", in place of its earlier value
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's revision differs from the one saved as "first"

  # The same item: first the bugs' path changes and the tasks' is added,
  # then the slices' default is written out, which leaves the slices' path
  # as loaded what it was. A revision of the whole config, or of all of
  # work.paths as written, differs.
  @ID-NEXTPATHS-07 @wip @slice-211
  Scenario: Under push.mode pr, a change to another kind's path, or the default written out, leaves the revision as it was
    Given the config sets "work.paths" to the YAML "{ bug: { spec_review: skip } }"
    And the config, committed and pushed to the remote's main, adds the YAML:
      """
      push:
        mode: pr
      """
    And a recording gh on the PATH, which opens pull requests as "https://github.example/o/r/pull/1"
    And the work registry has the item "slice-9" owned by "ana" with the status "doing"
    And the clone is on the new branch "slice-9"
    And a feature file with the scenario "@ID-A-01" tagged "@wip @slice-9"
    And git pushes HEAD to the remote's new branch "slice-9"
    And the recording gh has the draft pull request "https://github.example/o/r/pull/7" open for the branch "slice-9"
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's revision is saved as "first"
    Given the config sets "work.paths" to the YAML "{ bug: { spec_review: required }, task: { spec_review: skip } }", in place of its earlier value
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's "state" is "implementing"
    And its JSON's revision is the one saved as "first"
    Given the config sets "work.paths" to the YAML "{ bug: { spec_review: required }, task: { spec_review: skip }, slice: { spec_review: required } }", in place of its earlier value
    When itos runs "next slice-9 --json"
    Then itos exits with code 0
    And its JSON's revision is the one saved as "first"

  # Refused as any key or value the schema does not know: idea is not a
  # kind with a path, red_first is a step this slice does not have, and
  # optional is no value. Today the whole of work.paths is unknown, so the
  # message names work.paths alone and no problem is config-enum.
  @ID-NEXTPATHS-08 @wip @slice-211
  Scenario Outline: config check refuses an unknown kind or step under work.paths, and a value other than required or skip
    Given the config sets "work.paths" to the YAML "<paths>"
    When itos checks the config as JSON
    Then itos exits with code 2
    And its JSON's "valid" is false
    And its JSON's problem "<rule>"'s "message" says "<message>"
    And its JSON's problem "<rule>"'s "fix" says "<fix>"

    Examples:
      | paths                               | rule               | message                                                     | fix                                                        |
      | { idea: { spec_review: skip } }     | config-unknown-key | unknown key work.paths.idea                                 | remove work.paths.idea; the keys here are slice, bug, task |
      | { task: { red_first: required } }   | config-unknown-key | unknown key work.paths.task.red_first                       | the keys here are spec_review                              |
      | { task: { spec_review: optional } } | config-enum        | work.paths.task.spec_review should be one of required, skip | set work.paths.task.spec_review to one of required, skip   |

  # The default is the one table's, so config get prints it for the slices,
  # which the config leaves out, as the tools read it.
  @ID-NEXTPATHS-09 @wip @slice-211
  Scenario: config check accepts work.paths, and config get prints required for a kind the config leaves out
    Given the config sets "work.paths" to the YAML "{ task: { spec_review: skip }, bug: { spec_review: required } }"
    When itos checks the config
    Then itos exits with code 0
    When itos runs "config get work.paths.task.spec_review"
    Then itos exits with code 0
    And its output says "skip"
    When itos runs "config get work.paths.slice.spec_review"
    Then itos exits with code 0
    And its output says "required"
