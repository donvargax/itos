@phase-1
Feature: Fresh initialization writes local agent rules by default
  # Issue #30, q-78. Fresh means there was no effective config before init
  # began, including init --stealth and fresh init --policy. Decide this
  # before writing the config; a newly written config is not a rerun.
  # The default has exactly the existing explicit --agent-rules write
  # semantics and failure reporting, including existing text/marker/CRLF
  # preservation. It does not create another generated block or policy.
  # Remove only the agent-rules question on a fresh run. Plugin and shim
  # offers retain their current opt-in behavior and share input correctly.
  # Existing config means report-only rules handling unless --agent-rules
  # is explicit, even when --policy updates that config. --no-agent-rules
  # declines without inspecting or writing rules files. No new config key.
  # Existing JSON shape/actions remain; report the actual write rather than
  # offered. Changes to released default-init cases must be named by the
  # implementing feat, with its compatibility and opt-out guidance.

  @wip @slice-159 @ID-INIT-61
  Scenario: A noninteractive fresh project init writes rules without an opt-in flag
    Given a repository with no effective itos config or local agent rules
    And init has its normal controlled release-server fixture
    When noninteractive itos runs "init"
    Then itos exits with code 0
    And AGENTS.md has exactly the current marked itos rules block
    And CLAUDE.md imports AGENTS.md exactly once
    And no plugin installation or global Git shim installation ran
    And the agent-rules result reports the files actually written

  @wip @slice-159 @ID-INIT-62
  Scenario: Fresh terminal init no longer asks whether to write local rules
    Given a repository with no effective itos config or local agent rules
    And a terminal declines the existing plugin and Git shim offers
    When terminal itos runs "init"
    Then itos exits with code 0
    And no agent-rules question consumed terminal input
    And the plugin and Git shim offers each received their intended answer
    And AGENTS.md has exactly the current marked itos rules block

  @wip @slice-159 @ID-INIT-63
  Scenario: Fresh stealth init writes only the existing private rules setup
    Given a repository with no effective itos config
    And the project has a tracked AGENTS.md with project instructions
    When noninteractive itos runs "init --stealth --json"
    Then itos exits with code 0
    And the JSON agent_rules action is "written" with the existing files result shape
    And the shared Git-private AGENTS.md contains the current itos rules
    And CLAUDE.local.md and AGENTS.override.md use the existing stealth rules format
    And the existing stealth exclusions hide those two local files
    And no tracked project file was changed

  @wip @slice-159 @ID-INIT-64
  Scenario: The opt-out skips rules handling in either fresh mode
    Given fresh project and fresh stealth init fixtures with malformed existing rules markers
    When each runs init with "--no-agent-rules --json"
    Then their rules results are "declined" with no rules files written
    And no rules-marker error or agent-rules question occurs
    And their remaining initialization follows its existing result

  @wip @slice-159 @ID-INIT-65
  Scenario: Ordinary reruns remain report-only for missing stale or incomplete rules
    Given initialized project and stealth fixtures with missing or stale rules setup
    When each runs init without either agent-rules flag
    Then no rules file or rules exclusion is changed
    And their existing missing or stale rules guidance is reported
    And the existing config and hook findings determine their rerun outcome

  @wip @slice-159 @ID-INIT-66
  Scenario: Explicit rewrite still repairs a rerun while an opt-out leaves it alone
    Given initialized project and stealth fixtures with stale rules setup
    When each runs init with "--agent-rules --json"
    Then their rules are rewritten through the existing explicit-write path
    And their JSON rules result accurately reports changed and kept files
    When each runs init with "--no-agent-rules --json"
    Then their rules result is "declined" and the rules bytes are unchanged

  @wip @slice-159 @ID-INIT-67
  Scenario: Default writing preserves existing project prose line endings and current imports
    Given a fresh project config fixture with CRLF agent files and prose around a stale itos block
    And CLAUDE.md already imports AGENTS.md once
    When noninteractive itos runs "init --json"
    Then itos exits with code 0
    And only the marked block is refreshed in AGENTS.md with its CRLF line endings kept
    And CLAUDE.md retains its prose and exactly one AGENTS.md import
    And agent_rules reports "written" using only the existing JSON fields

  @wip @slice-159 @ID-INIT-68
  Scenario: A failed default rules write is an initialization failure with truthful partial results
    Given a fresh init fixture whose rules target has malformed markers or cannot be written
    When noninteractive itos runs "init --json"
    Then itos exits with code 1
    And agent_rules reports "failed" with the existing problem field
    And no successful rules write is invented
    And earlier starter files and hook outcomes follow existing partial-init behavior
    And there is no rollback of unrelated project edits

  @wip @slice-159 @ID-INIT-69
  Scenario: Policy initialization uses preexisting configuration to distinguish fresh from rerun
    Given fresh and already-configured policy-init fixtures in project and stealth modes
    When each runs init with a valid "--policy" source and no agent-rules flag
    Then only the fresh fixtures write the default local rules setup
    And the already-configured fixtures keep report-only rules handling
    And policy validation refusals still occur before any initialization write
