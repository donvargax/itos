@phase-1
Feature: ADR acceptance and validation are separate facts
  # Issue #32, q-80; user's NONBREAKING FIRST choice, 2026-10-23.
  # This delivery is a feat with Upgrading: none, not an authorised major.
  # Preserve default ADR bytes and the legacy index contract. Universal index
  # labels wait for v9. ASK34 and ASK37..40 are retired from this file to the
  # future p1-adr-validation-updates re-specification; never reuse those IDs.
  # No update flags, edit-existing command, replacement transaction or no-op
  # update contract belongs here. Existing --supersedes behaviour stays.
  # Full review of this draft is required before implementation.
  #
  # Acceptance chooses a direction. Validation records a scoped claim:
  # unknown = no reliable validation claim; untried = no attempt claimed;
  # supported/challenged = stated evidence for/against that direction.
  # Neither acceptance, an answered question, done work nor green CI proves
  # validation. A claim does not certify an experiment or universal correctness,
  # change lifecycle status, execute evidence, fetch a link or close work.
  #
  # Only decision record gains --validation <status> and --evidence <text>.
  # With neither flag, omit the validation key entirely: the exact existing
  # MADR rendering, local record date, index link and commit message remain.
  # Effective metadata is {status:"unknown",evidence:null,date:null}.
  # Explicit --validation unknown IS opt-in: write status unknown and date,
  # even without evidence. Index links keep their existing form. It differs
  # observably from omitting both flags. All four exact lowercase statuses
  # allow evidence; supported/challenged require fresh nonblank evidence in
  # this invocation. Unknown/untried allow its omission. Any supplied evidence
  # must be nonblank after whitespace detection, but its stored/read string
  # retains every space, tab, newline and trailing newline from the argument.
  # --evidence without --validation, either flag with --none, a missing flag
  # value, an empty/unknown status or blank evidence is a usage error (2).
  #
  # An opted-in new record adds a YAML validation mapping after the existing
  # status/date keys, before the closing frontmatter fence. Its logical fields
  # are status: string, optional evidence: string, date: YYYY-MM-DD string.
  # Omit evidence when absent. Use the existing renderer's scalar conventions
  # (as in internal/ask's quoted strings) so evidence is data, including
  # YAML-looking lines and footers;
  # tests compare decoded evidence, not one particular quote/block style.
  # Both dates use the SAME existing record-clock observation in its existing
  # local zone. Do not migrate the original record date to UTC or add a test
  # clock flag. Scenario helpers bound the command with before/after local
  # clock observations in the command's zone and accept dates within those
  # bounds, including midnight; they never assert an unbounded fixed today.
  #
  # Read a missing mapping, including a frontmatter-free ADR, as unknown with
  # no evidence/date and without writing. A present mapping requires one valid
  # status; optional evidence must be a nonblank string and is required for
  # supported/challenged; optional date must be a valid YYYY-MM-DD string.
  # Omitted evidence/date read as null. Null/nonmapping validation, duplicate
  # managed keys, unknown validation keys, wrong scalar types and invalid
  # dates/status/evidence are malformed data, not inferred unknown claims.
  # Unrelated frontmatter keys are allowed. There is no in-place YAML editor.
  #
  # Index inclusion, order, markers, blank lines and outside prose still use
  # the existing lifecycle rules. ALL index links keep exactly their legacy
  # form, including explicitly opted-in new records and hand-authored matching
  # metadata. First-delivery inspection is decision show/validation, not an
  # index suffix. This avoids inferring author consent from a previously
  # unreserved custom frontmatter key. Universal index labels wait for v9.
  # - [ADR-0001: Try labels](0001-try-labels.md)
  # Index regeneration does not inspect validation metadata at all: retain the
  # existing status/title reader and leave other ADR bytes alone. Existing
  # tolerance of malformed frontmatter
  # and existing duplicate-number behaviour on record are not tightened.
  # Config check, question add/answer, decision and decision --all gain no
  # validation checks, reads, fields or output changes in this delivery.
  #
  # decision validation <positive-record-number> is read-only, in the effective
  # work.decisions directory, including custom/project/private stealth paths.
  # It takes only that number and existing global flags, never --status,
  # --validation or --evidence. No question link or clean Git index is needed.
  # Success text is four LF-terminated lines, with a slash-separated path:
  # ADR-0007: notes/decisions/0007-legacy.md
  # Validation: unknown
  # Evidence: null
  # Validation date: null
  # Evidence uses a JSON string literal when present, preserving whitespace
  # through escapes; date uses YYYY-MM-DD or literal null. Success JSON is
  # {"schema":1,"ok":true,"number":7,"record":"notes/decisions/0007-legacy.md",
  #  "validation":{"status":"unknown","evidence":null,"date":null}}.
  # There is no commit key. Read any lifecycle, including superseded records.
  # A missing number is refusal 1 (ask-no-decision); multiple matching files
  # are refusal 1 (decisions-number-twice, naming both). A bad number/extra
  # argument/unsupported flag is usage 2. Malformed TARGET frontmatter or
  # validation is data error 2 naming its path and offending field, following
  # existing config/data diagnostics. Unrelated records are not validated.
  # Other failures keep the existing classified error path: 3 means missing
  # environment, 75 temporary, 70 unclassified. Do not turn every I/O error
  # into 3 or silently downgrade a failed explicit read to unknown.
  #
  # decision show retains its existing question/status/decision/answer and
  # adds a TOP-LEVEL validation key to success JSON, never inside question:
  # {"schema":1,"ok":true,"question":{...},"validation":{...}|null}.
  # Unrecorded questions and decision: none give null. For a uniquely resolved
  # readable ADR, use its effective metadata, even when it has no mapping.
  # Append a blank line and the same three Validation/Evidence/Validation date
  # lines to existing show text only when that metadata is available.
  # Compatibility: the ask corpus permits a dangling decision number. Show
  # therefore remains best-effort: a missing/ambiguous/unreadable ADR or bad
  # optional metadata yields validation:null, no new text block or failure;
  # use the explicit validation read for diagnostics. Existing errors reading
  # config/questions are NOT swallowed. List/all and record JSON stay as-is.
  #
  # Reading valid hand-authored metadata reports its literal recorded claim;
  # it is not consent to change the index or proof that itos created/verified
  # the claim. No migration or rewriting of that legacy metadata is authorised.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the header lint is itos's built-in one
    And work.decisions is "notes/decisions"
    And itos runs in a clone of the repository one commit deep

  @wip @slice-161 @ID-ASK-32
  Scenario: Default recording preserves the legacy bytes but exposes effective unknown validation
    Given the work registry has the item "slice-9" owned by nobody with the status "done"
    And itos has run the command line "decision add 'Labels or Projects?' --item slice-9"
    And itos has run the command line "decision answer q-1 'Labels.'"
    When itos runs the command line "decision record q-1 --title 'Try labels'" within observed local-clock bounds
    Then itos exits with code 0
    And the file "notes/decisions/0001-try-labels.md" equals these bytes with only <record-date> replaced by a date within those bounds and a final LF:
      """
      ---
      status: accepted
      date: <record-date>
      ---

      # Try labels

      ## Context and Problem Statement

      Labels or Projects?

      Asked as q-1, about slice-9.

      ## Considered Options

      The options are those the question names.

      ## Decision Outcome

      Labels.

      ### Consequences

      None recorded.
      """
    And the file "notes/decisions/README.md" equals these bytes with a final LF:
      """
      # Decisions

      The decisions that still stand, one record each. A record superseded keeps its
      file and leaves this list, which itos decision record writes.

      <!-- itos:decisions:begin -->

      - [ADR-0001: Try labels](0001-try-labels.md)

      <!-- itos:decisions:end -->
      """
    And the last commit's header is "docs: record q-1 as decision 1"
    When itos runs "decision show q-1 --json"
    Then itos exits with code 0
    And stdout is this JSON object with no additional keys:
      """json
      {"schema":1,"ok":true,"question":{"id":"q-1","item":"slice-9","question":"Labels or Projects?","status":"answered","answer":"Labels.","decision":1},"validation":{"status":"unknown","evidence":null,"date":null}}
      """

  @wip @slice-161 @ID-ASK-33
  Scenario Outline: Opted-in new records expose scoped claims without changing acceptance
    Given the committed file "notes/decisions/0007-legacy.md" holding "# Legacy"
    And itos has run the command line "decision add 'Labels or Projects?'"
    And itos has run the command line "decision answer q-1 'Labels.'"
    # Decode JSON evidence to ONE literal argv value; null means omit the flag.
    # This helper invokes decision record q-1 --title 'Try labels' --validation
    # <status> [--evidence <decoded-string>] through the normal command runner.
    When itos records "q-1" as "Try labels" with validation "<status>" and JSON evidence <evidence> within observed local-clock bounds
    Then itos exits with code 0
    And the file "notes/decisions/0008-try-labels.md" has the line "status: accepted"
    And its validation mapping has status "<status>" and literal JSON evidence <evidence>
    And its record date and validation date are the same local date within those bounds
    And its body equals the legacy MADR body for this question and title
    And the file "notes/decisions/README.md" has the line "- [ADR-0007: Legacy](0007-legacy.md)"
    And the file "notes/decisions/README.md" has the line "- [ADR-0008: Try labels](0008-try-labels.md)"
    And the file "notes/decisions/README.md" does not name "validation:"
    And the file "notes/decisions/0007-legacy.md" has the same contents as its committed version
    And the last commit's header is "docs: record q-1 as decision 8"
    When itos runs "decision show q-1 --json"
    Then itos exits with code 0
    And JSON's top-level validation has exactly status "<status>", evidence <evidence> and the recorded validation date
    And JSON's question has status "answered" and decision 8 with no validation key
    When itos runs "decision validation 8"
    Then itos exits with code 0
    And stdout is exactly the four-line validation report for "ADR-0008", "notes/decisions/0008-try-labels.md" and that literal metadata
    When itos runs "decision validation 8 --json"
    Then itos exits with code 0
    And stdout is exactly the validation-read JSON for number 8, record "notes/decisions/0008-try-labels.md" and that metadata, without a commit key
    When itos runs "decision show q-1"
    Then itos exits with code 0
    And stdout is the unchanged question report followed by a blank line and the same three metadata lines

    Examples:
      | status     | evidence                                                                                                    |
      | unknown    | null                                                                                                        |
      | untried    | null                                                                                                        |
      | unknown    | "  Existing notes do not establish a result.  "                                                              |
      | untried    | "  Planned only; no attempt claimed.  "                                                                      |
      | supported  | "  Local fixture A retained its labels in one trial.  "                                                      |
      | challenged | "  Local fixture B lost its labels in one trial.  "                                                          |
      | supported  | "  Fixture A only:\\nstatus: rejected\\nTask: T-001\\nhttps://example.invalid/evidence\\n\\tkept literal\\n" |

  @wip @slice-161 @ID-ASK-35
  Scenario: Legacy reads stay unknown despite done work and a successful CI run, without migrating files
    Given the work registry has the item "slice-9" owned by nobody with the status "done"
    And the committed file "notes/decisions/0007-legacy.md" holding the lines:
      """
      ---
      status: accepted
      date: 2020-01-02
      ---

      # Legacy

      Handwritten body, kept byte for byte.
      """
    And the committed file "notes/decisions/0008-bare.md" holding "# Bare"
    And the committed file "notes/decisions/README.md" holding "Handwritten index, not regenerated by reading."
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - { id: q-1, item: slice-9, question: "Labels?", answer: "Yes.", decision: 7 }
        - { id: q-2, question: "Unrecorded?", answer: "Yes." }
        - { id: q-3, question: "Item only?", answer: "Yes.", decision: none }
        - { id: q-4, question: "Dangling?", answer: "Yes.", decision: 99 }
      """
    And the nightly steps are "printf green > ci-result.txt", then "true"
    And itos has run the command line "ci run --nightly"
    And the file "ci-result.txt" says "green"
    And a change to "unrelated.txt" is staged
    And the repository's files, HEAD, refs and staged entries are snapshotted for these reads
    When itos runs "decision validation 7"
    Then itos exits with code 0
    And stdout equals these lines with a final LF:
      """
      ADR-0007: notes/decisions/0007-legacy.md
      Validation: unknown
      Evidence: null
      Validation date: null
      """
    When itos runs "decision validation 7 --json"
    Then itos exits with code 0
    And stdout is this JSON object with no additional keys:
      """json
      {"schema":1,"ok":true,"number":7,"record":"notes/decisions/0007-legacy.md","validation":{"status":"unknown","evidence":null,"date":null}}
      """
    When itos runs "decision show q-1 --json"
    Then itos exits with code 0
    And stdout is this JSON object with no additional keys:
      """json
      {"schema":1,"ok":true,"question":{"id":"q-1","item":"slice-9","question":"Labels?","status":"answered","answer":"Yes.","decision":7},"validation":{"status":"unknown","evidence":null,"date":null}}
      """
    When itos runs "decision validation 8 --json"
    Then itos exits with code 0
    And stdout is exactly the validation-read JSON for number 8, record "notes/decisions/0008-bare.md" and unknown validation with null evidence and date
    When each of "decision show q-2 --json", "decision show q-3 --json" and "decision show q-4 --json" runs
    Then each exits 0 with a top-level validation key equal to JSON null and its unchanged question object
    When itos runs "decision --all --json"
    Then itos exits with code 0
    And its questions retain the existing list schema with no validation keys anywhere
    And every snapshotted file, HEAD, ref and staged entry is unchanged after all these reads

  @wip @slice-161 @ID-ASK-36
  Scenario: Invalid opt-ins and strict reads fail without widening default-workflow refusals
    Given the committed file "notes/decisions/0007-legacy.md" holding "# Legacy"
    And the committed file "tasks/asks.yaml" holding the lines:
      """
      questions:
        - { id: q-1, question: "Labels or Projects?", answer: "Labels." }
        - { id: q-2, question: "Legacy?", answer: "Yes.", decision: 7 }
      """
    When itos runs "decision validation 7 --json"
    Then itos exits with code 0
    And stdout is exactly the validation-read JSON for number 7, record "notes/decisions/0007-legacy.md" and unknown validation with null evidence and date
    # Each row runs independently against the same fixture. JSON argv arrays
    # permit exact empty/whitespace arguments without shell normalization.
    # Double backslashes in Gherkin tables preserve JSON escapes for decoding.
    When each argument vector below runs with its files, HEAD, refs and staging snapshotted first:
      | argv                                                                                                  | exit |
      | ["decision","record","q-1","--title","Try labels","--validation","supported"]                        | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","challenged"]                       | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","supported","--evidence",""]        | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","challenged","--evidence"," \\t\\n"] | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","unknown","--evidence","  "]      | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","untried","--evidence","  "]      | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","Supported","--evidence","trial"]  | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","proven"]                           | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation",""]                                | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation"]                                   | 2    |
      | ["decision","record","q-1","--title","Try labels","--evidence","trial"]                              | 2    |
      | ["decision","record","q-1","--title","Try labels","--validation","untried","--evidence"]            | 2    |
      | ["decision","record","q-1","--none","--validation","unknown"]                                       | 2    |
      | ["decision","record","q-1","--none","--evidence","trial"]                                           | 2    |
      | ["decision","record","q-1","--none","--validation","supported","--evidence","trial"]                | 2    |
      | ["decision","validation","99","--json"]                                                              | 1    |
      | ["decision","validation","0"]                                                                        | 2    |
      | ["decision","validation","q-1"]                                                                      | 2    |
      | ["decision","validation","7","8"]                                                                    | 2    |
      | ["decision","validation","7","--status","supported","--evidence","trial"]                           | 2    |
      | ["decision","validation","7","--validation","untried"]                                               | 2    |
      | ["decision","validation","7","--evidence","trial"]                                                   | 2    |
    Then every command has the listed exit code and no snapshotted file, HEAD, ref or staged entry changes
    And the missing record refusal uses "ask-no-decision" while usage diagnostics name the bad flag, value or combination
    # The helper makes/commits each fixture in a separate scratch copy before
    # invoking the real binary, and compares state only across that invocation.
    When "decision validation 7 --json" reads each independent committed target fixture below:
      | target frontmatter as JSON string                                                   | exit |
      | "status: [accepted\\n"                                                             | 2    |
      | "status: accepted\\nvalidation: null\\n"                                             | 2    |
      | "status: accepted\\nvalidation: supported\\n"                                        | 2    |
      | "status: accepted\\nvalidation: {status: proven}\\n"                                 | 2    |
      | "status: accepted\\nvalidation: {status: supported}\\n"                              | 2    |
      | "status: accepted\\nvalidation: {status: challenged, evidence: '  '}\\n"              | 2    |
      | "status: accepted\\nvalidation: {status: unknown, evidence: 42}\\n"                   | 2    |
      | "status: accepted\\nvalidation: {status: untried, date: '2026-02-30'}\\n"              | 2    |
      | "status: accepted\\nvalidation: {status: unknown, extra: yes}\\n"                     | 2    |
      | "status: accepted\\nvalidation: {status: unknown, status: untried}\\n"                | 2    |
    Then every read exits 2 naming "notes/decisions/0007-legacy.md" and its malformed field or YAML, without writes
    And "decision show q-2 --json" against each such fixture still exits 0 with top-level validation null
    And "decision record q-1 --title 'Try labels'" against a fresh copy of each such fixture exits 0, preserving that target's bytes and its unsuffixed index link
    And "decision record q-1 --title 'Try labels' --validation untried" against another fresh copy exits 0, labelling only the new record and preserving the malformed target's bytes
    When two independent committed files "notes/decisions/0007-legacy.md" and "notes/decisions/0007-other.md" share target number 7
    And itos runs "decision validation 7 --json"
    Then itos exits with code 1 with "decisions-number-twice" naming both files and without writes
