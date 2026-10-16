@phase-1
Feature: ADR acceptance and validation are separate facts
  # Issue #32, q-80. An accepted direction is not automatically implemented.
  # Record YAML frontmatter gains a validation mapping: status is unknown,
  # untried, supported or challenged; evidence is literal narrative text or
  # absent; date is the explicit validation update's UTC YYYY-MM-DD or absent.
  # Unknown means no reliable validation claim is recorded, untried explicitly
  # says no attempt is claimed, supported/challenged report scoped evidence
  # for/against the direction. These are recorded claims, not itos-certified
  # experiments, universal correctness or a lifecycle status transition.
  # New decision record flags: --validation <status> and --evidence <text>.
  # Default unknown, no evidence/date. Nondefault status or explicit evidence
  # records today's date. Supported/challenged require nonblank evidence.
  # Evidence without --validation is a usage error, as is combining either
  # flag with --none. Never infer validation from question text, linked item
  # completion, CI, source existence or the fact the user accepted an answer.
  # decision validation <record-number> reads the effective metadata. With
  # --status <status> [--evidence <text>] it explicitly replaces that metadata
  # in one transaction and refreshes the marked ADR index. Unknown/untried
  # permit omitted evidence; replacement never accidentally retains stale
  # evidence. Supported/challenged require explicit new evidence. No command
  # executes evidence, fetches links, closes work or records an ADR implicitly.
  # Missing metadata on an old record reads unknown without editing it.
  # Annotate validation in the generated live-record index so a reader sees
  # it before following an ADR; keep lifecycle-based inclusion unchanged.
  # decision show for a recorded question displays its validation separately;
  # existing JSON gains validation without changing question decision/status.
  # The dedicated read JSON includes record number, file and validation
  # {status,evidence:null|string,date:null|string}; writes add commit:null|SHA.
  # Dirty target/index, duplicate record numbers, malformed frontmatter or
  # invalid existing validation refuse with exit 1 before mutation; bad CLI
  # or missing required evidence is 2. Unknown record is 1, I/O failure 3.
  # Update only managed validation metadata and index span, retaining body,
  # lifecycle/date fields, unrelated frontmatter, line endings and outside
  # index prose. If metadata cannot be edited unambiguously, refuse rather
  # than reserialize the whole document or discard comments. Existing commit
  # isolation/rollback and private stealth writes apply. Same status/evidence
  # is a semantic no-op: no new date, write, hook or commit.

  @wip @slice-161 @ID-ASK-32
  Scenario: A newly recorded accepted direction has unknown validation by default
    Given an answered unrecorded question naming a done item
    When the user explicitly records it with a title and no validation flags
    Then its ADR lifecycle status is "accepted"
    And its validation status is "unknown" with no evidence or validation date
    And the generated live index visibly labels its validation "unknown"
    And the normal question-to-record association is retained

  @wip @slice-161 @ID-ASK-33
  Scenario: An explicitly untried accepted ADR tells readers that before they build
    Given an answered question whose direction has not been attempted
    When the user records it with "--validation untried"
    Then the ADR remains "accepted" with validation "untried" and today's UTC validation date
    And its live index entry and decision show expose "untried" separately from acceptance
    And no experiment task or source change was fabricated

  @wip @slice-161 @ID-ASK-34
  Scenario: Evidence can support or challenge a direction without changing its lifecycle
    Given an accepted ADR with validation "untried"
    When the user sets its validation to "supported" with a literal scoped experiment description
    Then one metadata-and-index transaction records that status evidence and UTC date
    And its lifecycle title question link and original record date are unchanged
    When the user sets validation to "challenged" with a new failed experiment description
    Then the new evidence replaces the previous validation evidence
    And the ADR remains accepted until an explicit lifecycle decision supersedes it

  @wip @slice-161 @ID-ASK-35
  Scenario: Unknown legacy validation is readable without a migration write
    Given a legacy ADR with no validation metadata and a recorded question linking it
    When decision validation reads its number as text and JSON
    Then validation is "unknown" with null evidence and date
    And decision show for the question exposes the same effective validation
    And every file index ref and staged entry is unchanged
    And no inference is made from linked done work or existing CI evidence

  @wip @slice-161 @ID-ASK-36
  Scenario: Missing evidence and invalid combinations fail before writes
    Given answered questions and existing readable ADRs
    When supported or challenged status is requested with missing or whitespace-only evidence
    Then itos exits with code 2 before changing questions records or index
    And unknown status spelling evidence without status or validation flags with --none also exit 2
    And an unknown record number exits 1 without creating an ADR

  @wip @slice-161 @ID-ASK-37
  Scenario: Updating validation preserves document prose and unrelated staging
    Given a CRLF ADR with comments custom frontmatter and handwritten body sections
    And an ADR index with prose outside its managed span
    And an unrelated staged project change
    When validation is explicitly updated with multiline evidence that resembles YAML and commit footers
    Then the evidence is stored and read back as literal data
    And only its validation metadata and the managed index span change
    And all unrelated bytes line endings and staged changes are preserved
    And the commit names only the changed ADR metadata and index files

  @wip @slice-161 @ID-ASK-38
  Scenario: A refused or failed validation update leaves no partial record
    Given ADR fixtures with dirty targets dirty index ambiguous record numbers or invalid validation metadata
    When validation is updated
    Then each refuses with code 1 before writes
    Given clean readable ADR and index files and a rejecting commit hook
    When validation is updated
    Then the normal write transaction rolls back its own files and staging
    And the old effective validation remains visible

  @wip @slice-161 @ID-ASK-39
  Scenario: Repeating the same validation does not manufacture fresh evidence
    Given an ADR with supported validation evidence recorded on an earlier date
    When that same status and evidence are submitted again
    Then its old validation date remains unchanged
    And no files hooks or commits are created
    When status becomes "unknown" with no evidence
    Then prior evidence is cleared and a real explicit update is recorded

  @wip @slice-161 @ID-ASK-40
  Scenario: Stealth storage and lifecycle indexing remain independent of validation
    Given a custom private stealth ADR directory with live and superseded records
    When a superseded record receives explicit challenged validation evidence
    Then private record metadata is updated without a Git commit or tracked project write
    And the superseded record does not reenter the live index
    And ordinary decision superseding preserves existing validation metadata
