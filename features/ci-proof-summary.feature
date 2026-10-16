@phase-1
Feature: Optional provider summaries make a successful code proof inspectable
  # Issue #25, q-83, after slice-106. Existing providers remain valid.
  # An exit-zero provider may opt in by emitting one JSON object whose
  # top-level itos_proof member is an object with version:1. Recognized
  # fields are judged, unit, files, killed, survived, excepted and uncovered.
  # Counts may be absent/null (unavailable) or integers 0..9007199254740991.
  # Numeric judged requires a nonempty ASCII unit identifier, letters,
  # digits, underscore/hyphen, starting with a letter, at most 64 characters.
  # Example: {"itos_proof":{"version":1,"judged":1,"unit":"function",
  # "killed":6,"excepted":0}}. No provider-native files aggregation is
  # guessed; providers/wrappers explicitly supply the common envelope.
  # Unknown extra summary fields are ignored, not summed or executed.
  # Absent itos_proof, opaque text, invalid JSON or non-object stdout keep
  # exit-zero acceptance and summary unavailable as before. A parsed top-level
  # member explicitly opting in must be valid: null/nonobject, duplicate
  # recognized summary keys, unsupported version, invalid counts/unit or
  # conflicting duplicate top-level itos_proof is CannotRun, not a pass.
  # No marker-string detection inside opaque output or nested provider data.
  # Missing/null counts never become zero. A valid summary reporting judged:0
  # refuses an actually scheduled ranged code proof with ci-code-proof-empty,
  # even when exit is zero. A positive judged count is not independent proof
  # of accuracy. Other counts are reported, without new sum-consistency,
  # survivor/exception or coverage gates beyond the provider's own verdict.
  # Preserve all nonzero provider exit/problem behavior. Parse the optional
  # summary for successful answers only; failure cannot be converted to pass
  # by positive counts. CI's outer exit remains 1 for any failed proof step,
  # including CannotRun, as today; do not leak provider codes as CI exits.
  # Text after the executed step reports only supplied counts and their unit,
  # or says the provider did not report a summary. Never invent task-specific
  # attribution from a whole range, zero counts or execution at planning time.
  # JSON ci run adds proof only when the step executed: command, from, to,
  # outcome (passed/refused/cannot-run), summary (normalized object|null).
  # Unknown normalized metrics are null. Keep existing ok/failed_at/failures
  # and problem behavior. Logs/provider stderr go to stderr under --json.
  # ci plan remains execution-free, with no measured summary. If a prior
  # failure stops before proof, --tests omits it, or planning schedules none,
  # no proof result or success summary is fabricated. No new provider run,
  # saved receipt, scope inference, default check command or config setting.
  # Keep work done's required-CI behavior: it does not rerun this provider.
  # Document the envelope and absence semantics for third-party providers.

  @wip @slice-164 @ID-CI-52
  Scenario: A successful opt-in provider reports its scoped counts
    Given a ranged code proof whose provider exits zero with judged 1 function killed 6 and excepted 0
    And its stdout supplies those values in a version-one itos_proof summary
    When the full CI run reaches the code proof
    Then the text reports 1 function judged 6 killed and 0 excepted
    And the provider executes exactly once
    And itos does not attribute the whole range's counts to one task

  @wip @slice-164 @ID-CI-53
  Scenario: Explicit zero judged work refuses a required proof despite provider success
    Given a scheduled ranged code proof whose provider exits zero with valid judged 0 metadata
    When CI executes it
    Then CI exits with code 1
    And its problem rule is "ci-code-proof-empty"
    And its fix asks for the provider's actual scope and base to be corrected
    And no successful proof or CI pass is claimed

  @wip @slice-164 @ID-CI-54
  Scenario: Existing providers without the envelope keep exit-code acceptance
    Given successful providers returning no output opaque text invalid JSON or unrelated native JSON
    When each is the configured code proof of a ranged CI run
    Then each retains exit-zero acceptance
    And the log says the provider did not report a summary
    And no native files array or similarly named nested field is guessed into counts

  @wip @slice-164 @ID-CI-55
  Scenario: Unavailable metrics are distinct from measured zero
    Given a successful version-one summary with killed 6 and judged absent or null
    When ranged CI executes the proof
    Then the proof does not fail for zero judged work
    And only the reported metric is counted in text
    And normalized judged files and other unreported counts are null rather than zero

  @wip @slice-164 @ID-CI-56
  Scenario: An invalid opt-in summary cannot silently become a successful proof
    Given successful providers explicitly supplying malformed or unsupported itos_proof metadata
    And invalid cases include fractional negative string overflowing and boolean counts
    And invalid cases include duplicate recognized keys and judged without a valid unit
    When ranged CI executes each provider
    Then each proof is CannotRun with a precise summary-contract diagnostic
    And CI exits with code 1 using its existing failed-proof reporting
    And itos does not coerce truncate or fall back to an unavailable successful summary

  @wip @slice-164 @ID-CI-57
  Scenario: Positive summary counts never override a provider failure
    Given providers returning structured exit-one refusals or other failing exits
    And their stdout also contains positive optional summary counts
    When ranged CI executes each
    Then the existing refused or CannotRun outcome and provider problems are preserved
    And CI exits with code 1
    And no successful-count message overrides that outcome

  @wip @slice-164 @ID-CI-58
  Scenario: An unexecuted proof has no measured result
    Given CI plans with no scheduled proof or with --tests selecting named tests only
    And a plan that stops at an earlier failing step before its scheduled proof
    When each plan is shown or executed as appropriate
    Then no provider summary success or measured count is emitted
    And no provider is run merely to fill a summary
    And JSON contains no fabricated proof execution result

  @wip @slice-164 @ID-CI-59
  Scenario: JSON exposes execution evidence without mixing log text into stdout
    Given a ranged code proof with a valid successful common summary
    When CI runs with --json
    Then stdout has one report retaining its existing ok and failure contract
    And its proof identifies the executed command exact range outcome and normalized summary
    And provider stderr and human count text are on stderr
    And missing metrics remain null and unknown extra metrics are not invented

  @wip @slice-164 @ID-CI-60
  Scenario: Counts are descriptive apart from the explicit zero-work refusal
    Given a successful summary with positive judged count and other reported counts
    When CI executes the proof
    Then it does not derive an additional verdict by summing or reinterpreting the counts
    And it retains the provider's own acceptance except for the specified zero-work rule
    And subsequent work done uses required hosted CI without rerunning or replaying proof counts
