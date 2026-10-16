@phase-1
Feature: Each project commit is verified under its tracked policy
  # Issue #55, q-81. First delivery changes per-commit verification: header
  # and footer policy, types, path sets/scopes, and builtin moves use the
  # selected project config's blob at that commit. Rules added/removed in a
  # commit apply to that same commit. ID sources still obey each historical
  # footer's read_at and since, with read_at:commit reading that commit.
  # Resolve the invocation's effective config path to one repository-relative
  # path once, then read that exact path at each selected SHA. No current-
  # tree, parent-config, branch-tip or default-config fallback. Historical
  # pin/requires data does not launch/download another itos: the invoked
  # runtime parses the historical schema, refusing unsupported config.
  # Current effective commits.since still selects the overall verification
  # window as today; older configs do not independently truncate that list.
  # A selected commit lacking the fixed config path is config usage exit 2,
  # naming SHA/path. Adoption must supply a truthful verification window;
  # do not silently skip unconfigured commits. Config deletion/rename inside
  # the window is explicitly unsupported here and refuses, rather than
  # guessing provenance or moving since to conceal rule failures. Unavailable
  # Git objects are environment exit 3 with full-history/fetch guidance.
  # Invalid historical YAML/schema is 2; actual rule rejection stays 1.
  # Merge commits use their own config and existing own-change merge rules;
  # side commits use their own blobs, not the first parent's policy.
  # Current range enumeration, empty/unpushed starts and footer since remain.
  # A historical delegated header lint uses that config's command and shell
  # in the existing current execution directory, with the same message-input
  # protocol: this is not a historical script checkout or policy sandbox.
  # Missing delegates fail under their existing contract, never use a newer
  # command silently. No checkouts, worktrees, stashes, refs or index writes.
  # Arbitrary whole-range commands retain their current once-per-range,
  # effective-config behavior and are labeled/documented as such. Historical
  # range-command epochs are p1-historical-range-commands, not claimed solved
  # by per-commit config. Builtin moves is per commit and is solved here.
  # Stealth has no tracked private config history: explicitly retain current
  # private policy and private source/note behavior, without claiming the
  # project-mode historical guarantee. No private config enters Git.
  # Keep existing JSON result keys and meanings; add config {path,at} to
  # project per-commit results, with at the judged SHA. Diagnostics identify
  # failing historical SHA/path. Do not report an unreadable policy as pass.

  @wip @slice-163 @ID-VERIFY-01
  Scenario: A later builtin moves rule does not reject an earlier adopted live scenario
    Given a project with a declared Gherkin kind but no builtin moves rule
    And a test commit valid under that config adds an existing-behavior live scenario
    And a later valid config commit adds the builtin moves rule
    When verify checks the complete configured commit range
    Then the older test commit is judged with its own config and passes
    And the rule-adding commit is judged under the rule it adds
    And no commit is hidden by moving commits.since

  @wip @slice-163 @ID-VERIFY-02
  Scenario: A rule takes effect in the commit that adds it
    Given a commit that adds a path restriction and violates that restriction itself
    When verify checks that commit
    Then itos exits with code 1 and identifies that commit's rule failure
    And it does not borrow the permissive parent config
    And a later commit removing the rule does not make that failure pass

  @wip @slice-163 @ID-VERIFY-03
  Scenario: Removing an obsolete path rule and its file in one valid commit works
    Given an older build commit permitted to edit a gate path by its config
    And a later build commit removes that path and its now-obsolete path-set entry together
    When verify checks both against their own tracked configs
    Then the older commit retains its historical permission
    And the later commit is judged under its updated path rules
    And no two-push retain-the-obsolete-rule workaround is needed

  @wip @slice-163 @ID-VERIFY-04
  Scenario: Types footer definitions and ID sources are historical together
    Given commits with different type lists footer names substitution rules and ledger layouts
    And their linked IDs exist at the commits their historical footer policies read
    When verify checks them in one range with --json
    Then each message is judged under its own complete footer and type policy
    And each project result includes the selected config path and its own SHA as config.at
    And a later footer rename or ID removal does not rewrite an older commit's policy

  @wip @slice-163 @ID-VERIFY-05
  Scenario: Working-tree config edits cannot change historical verdicts
    Given two committed config versions that disagree on a commit's path permission
    And unrelated unstaged config edits that would allow every path
    When verify checks the committed range twice in differing traversal orders
    Then each commit receives the verdict of its own blob both times
    And cached policy is not reused merely because the config path is the same
    And the working-tree config cannot waive a historical rejection

  @wip @slice-163 @ID-VERIFY-06
  Scenario: Adoption and missing historical config have an explicit boundary
    Given a truthful current commits.since excluding history before config adoption
    When verify checks the configured range
    Then the adoption commit uses the config it introduces
    And excluded pre-adoption commits remain outside the range as before
    When a requested in-window commit lacks the selected config path
    Then itos exits with code 2 and identifies that SHA and path
    And config deletion or rename is not treated as an automatically exempt commit

  @wip @slice-163 @ID-VERIFY-07
  Scenario: Unreadable or unsupported historical policy never falls back to today's config
    Given commits with malformed or unsupported historical config blobs
    When verify reaches each
    Then itos exits with code 2 and names the historical config problem
    And no newer config default or pinned runtime substitutes for it
    Given missing history objects needed for a selected commit
    When verify needs those objects
    Then itos exits with code 3 with history-fetch guidance and no invented pass

  @wip @slice-163 @ID-VERIFY-08
  Scenario: Merge own-changes and side commits retain their own policy attribution
    Given a merged history whose side branch and merge commit carry different configs
    When verify checks the merged range
    Then side commits use their own config blobs
    And the merge uses its own blob and the existing merge own-change comparison
    And an empty own-change merge retains its existing verification exemption

  @wip @slice-163 @ID-VERIFY-09
  Scenario: Delegated lint and arbitrary range commands retain an honest execution context
    Given historical configs with a recording delegated header-lint command
    And current config with an arbitrary whole-range command
    When verify checks the range
    Then each header delegate receives its historical command shell and message protocol
    And it executes in the existing current project directory without a checkout swap
    And the arbitrary whole-range command runs once under current effective config as before
    And no historical epoch or deleted-script replay guarantee is claimed

  @wip @slice-163 @ID-VERIFY-10
  Scenario: Private stealth verification keeps current private policy explicit
    Given a stealth repository with private config ledger registry and commit notes
    When verify checks its normal unpushed or explicit range
    Then existing private policy and source rules apply
    And no historical tracked-config guarantee is claimed for stealth
    And no private data ref index or checkout file is written by policy loading
