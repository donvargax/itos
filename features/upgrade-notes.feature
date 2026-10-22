@phase-1
Feature: Upgrade reports a commit heading once for all of its entries
  # #50: grouping is presentation only. It does not decide whether a human
  # instruction is complete; p1-upgrade-notes-tidy retains that concern.
  # Within each release and category, group by the full nonempty commit
  # identity. Groups follow first occurrence; entries retain their order
  # within each group, including repeated text. The first entry supplies
  # the displayed header. An entry without an identity remains separate.
  # Keep each entry's multiline content with the existing indentation and
  # trailing-newline convention. Never group by header or displayed SHA.
  # JSON release and manual arrays remain the original ordered entries.

  Background:
    Given a release server offering the versions "9.1.0", "9.2.0" and "9.3.0", each with an upgrading.json naming the one before it
    And a repository whose ledger has the task "T-001"
    And the itos that runs is the itos under test, as the version "9.3.0"
    And the config pins the version "9.1.0" of the release server

  # #86: one commit's consecutive Upgrading footers are often one wrapped
  # note (371596cb in v7.0.0 wrote four lines, and its upgrading.json holds
  # four entries); under its commit heading print them as one paragraph in
  # text output, keeping the entries in JSON. Say which wins over keeping
  # every text entry before this slice is built.
  @wip @ID-UPGRADECMD-26 @slice-152
  Scenario: Many changed cases share one commit heading
    Given release "9.2.0" declares 38 distinct Changes entries from one full commit identity and header
    When itos runs "upgrade 9.2.0"
    Then itos exits with code 0
    And the report contains one "Changes on purpose" heading for that commit
    And all 38 entry texts appear below it in their release-data order
    And no entry text was omitted or combined with another text
    And the ordinary upgrade summary and final checks are still reported

  @wip @ID-UPGRADECMD-27 @slice-152
  Scenario: Distinct commits sharing a header and abbreviated SHA remain distinct
    Given release "9.2.0" declares two Changes entries with the same header
    And their full commit identities differ but share their first seven characters
    When itos runs "upgrade 9.2.0"
    Then itos exits with code 0
    And the report contains two "Changes on purpose" commit headings
    And each heading contains only its own commit's entry text
    And the headings follow the entries' first-occurrence order

  @wip @ID-UPGRADECMD-28 @slice-152
  Scenario: Grouping is stable within a category and never crosses release or category boundaries
    Given release "9.2.0" has Changes entries from commits A, B and A, with distinguishable texts in that order
    And release "9.2.0" also has Breaking and Upgrading entries from commit A
    And release "9.3.0" has two more Changes entries from commit A
    When itos runs "upgrade 9.3.0"
    Then itos exits with code 0
    And release "9.2.0" reports one Changes group for A with both A texts in order, followed by the B group
    And release "9.3.0" reports its own Changes group for A
    And the releases are still reported oldest first
    And the By hand section retains separate Breaking and Upgrading groups for A
    And no entry text has moved into a different category or release

  @wip @ID-UPGRADECMD-29 @slice-152
  Scenario: Grouped entries preserve duplicate and multiline instructions without guessing identity
    Given release "9.2.0" has two identical Upgrading entries from one commit
    And that commit also has a multiline Upgrading entry containing a command and manual cleanup
    And release "9.2.0" has two more Upgrading entries with no commit identity
    When itos runs "upgrade 9.2.0"
    Then itos exits with code 0
    And the identified commit has one Upgrading heading
    And the repeated instruction appears twice below that heading
    And every line of the multiline instruction remains readable below that heading
    And the two unidentified entries retain separate headings
    And the report has not suppressed an instruction because upgrade edited an agent-rules block
    And no command from a release instruction has run

  @wip @ID-UPGRADECMD-30 @slice-152
  Scenario: Compact text does not restructure machine output or change the upgrade transaction
    Given release "9.2.0" has nonadjacent Changes entries from the same commit and multiple manual entries
    And an unrelated staged edit and an untracked file exist in the repository
    When itos runs "upgrade 9.2.0 --json"
    Then itos exits with code 0
    And the release's Changes, Breaking and Upgrading arrays retain every original entry in its original order
    And every entry retains its existing commit, header and text fields
    And the manual array retains the existing release/text entries in its existing order
    And JSON has no grouped replacement arrays
    And upgrade reports only the files its normal upgrade operations edited
    And the unrelated staged edit and untracked file are unchanged
    And HEAD is unchanged and no commit was created
