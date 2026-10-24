@phase-1
Feature: Optional provider summaries make a successful code proof inspectable
  # Issue #25; q-83 in tasks/asks.yaml. This extends slice-106, not the
  # provider's mutation or coverage policy. No provider-native JSON is guessed.
  # Slice-164 precedes slice-155 in the queue and does not depend on it.
  # Its additive ci run proof member is compatible with slice-155 CI-50:
  # ci plan keeps order[].proof, and ci run keeps its existing schema, ok,
  # failed_at, failures and problem fields. CI-50 does not forbid additive
  # execution evidence. Guide export overrides belong to another slice.
  #
  # Only exit-zero stdout is considered for the optional envelope. It must
  # be one complete JSON object with a top-level itos_proof member to opt in.
  # Empty output, opaque text, invalid JSON, nonobjects and objects without
  # that member still pass, with no available summary. Nested fields and
  # marker strings do not opt in. Surrounding JSON whitespace is allowed.
  #
  # An opted-in member must be an object with version the JSON integer 1.
  # The top-level itos_proof name and all recognized summary keys match
  # exactly, case-sensitively; differently cased names are unknown keys.
  # Recognized keys are version, judged, unit, files, killed, survived,
  # excepted and uncovered. A duplicate top-level itos_proof or recognized
  # summary key is invalid even when the values are identical or null.
  # Duplicate unknown keys, at either level, are ignored along with their
  # values. Unknown fields are never copied into the normalized summary.
  #
  # Each count is absent/null or an unsigned JSON integer token: 0 or
  # [1-9][0-9]*, at most 9007199254740991. Fractions, exponents, negative
  # numbers (including -0), strings, booleans, arrays and objects are invalid.
  # files is a scalar count in this envelope, never a native files array.
  # A nonnull unit must match [A-Za-z][A-Za-z0-9_-]{0,63}, even when judged
  # is absent/null. Numeric judged requires that valid nonnull unit; only
  # absent/null judged allows absent/null unit. Arbitrary unit tokens are
  # preserved exactly, without pluralization or interpreting their scope.
  #
  # Normalized summaries have exactly version, judged, unit, files, killed,
  # survived, excepted and uncovered. Unreported/null metrics and unit stay
  # null, not zero. An invalid opt-in yields cannot-run, summary null and
  # rule ci-code-proof-summary. A valid explicit judged:0 yields refused,
  # rule ci-code-proof-empty and retains the valid normalized zero summary.
  # Each of these new failures has exactly its one named problem, without
  # the generic ci-code-proof wrapper; existing nonzero-provider failures
  # retain their current wrapper/problem behavior as CI-57 specifies.
  # Other counts are descriptive, including survivors and uncovered lines;
  # no sums, consistency checks or new verdict rules are derived from them.
  #
  # After an exit-zero provider with a valid or absent envelope, the log has
  # exactly one summary line before the final CI result or proof refusal.
  # An invalid opt-in (ci-code-proof-summary) prints no summary line.
  # The available summary's prefix is "Code proof summary: ".
  # Nonnull fields appear as label=value, separated by "; ", in this order:
  # judged, unit, files, killed, survived, excepted, uncovered. Omit version
  # and null fields. If none remain, or no envelope was supplied, the exact
  # line is "Code proof summary unavailable: provider did not report a summary."
  # A valid empty envelope still normalizes to an object, not null.
  # Nonzero providers keep their existing verdict/problems and have summary
  # null; their stdout is not parsed for this envelope and no summary line
  # is printed. Every failed CI run exits 1, not the provider's exit code.
  #
  # ci run --json adds proof only after executing the proof command (one
  # whose shell never starts or is stopped by a signal, internal/proof's
  # cannot-run, has it too, outcome cannot-run, summary null). It has
  # exactly command, from, to, outcome (passed/refused/cannot-run), summary.
  # The command is the executed command, the ends resolved full commit SHAs.
  # No per-task attribution is inferred from these range-wide counts.
  # Logs, human summaries and provider stderr go to stderr under --json;
  # stdout remains one report. Planning, --tests and an omitted/unreached
  # proof produce no top-level proof member and no measured-summary line.
  # No extra provider run, receipt, default command or config key is added.
  # work done remains outside this change; it does not rerun the provider.
  # README.md's Proof of done explains this optional provider envelope;
  # docs/rule-ids.md lists ci-code-proof-summary and ci-code-proof-empty.

  Background:
    Given a repository whose ledger has the task "T-001"
    And the CI steps run the named tests of the kind "scenario"
    And the scratch file "tasks/phase-1.yaml" contains exactly:
      """
      - id: T-001
        type: chore
        title: Example task
      """
    And the scratch file "proof-provider.sh" contains exactly:
      """
      printf '%s\n' "$*" >> proof-runs
      printf '%s\n' 'provider diagnostic' >&2
      cat proof.stdout
      exit "$(cat proof.exit)"
      """
    And the scratch file "record-ci.sh" contains exactly:
      """
      printf '%s\n' "$*" >> ci-runs
      if test "$1" = before; then exit "$(cat before.exit)"; fi
      exit 0
      """
    And the scratch files "proof.exit" and "before.exit" each contain "0"
    And the scratch file "proof.stdout" contains exactly:
      """
      {"itos_proof":{"version":1,"judged":1,"unit":"function","killed":6,"excepted":0}}
      """
    And the scratch root's "itos.yaml" is replaced with:
      """
      version: 1
      shell: [sh, -c]
      ledger:
        files: tasks/phase-{group}.yaml
        id: 'T-\d+'
      commits:
        footers:
          Task: {source: ledger, validate_for: all, read_at: commit}
      tests:
        scenario:
          root: features
          id: 'ID-[A-Z]+-\d+'
          tag_prefix: '@'
          run:
            whole: sh record-ci.sh tests whole
            select: sh record-ci.sh tests {pattern}
            ids_pattern: '@(?:{ids})\b'
          smoke: {file: features/smoke.yaml}
      ci:
        steps:
          - sh record-ci.sh before
          - {tests: scenario}
        cost:
          static: ['^sh record-ci\.sh before$']
        keep_step_order: true
        stop_at_first_failure: true
        range: {provider: none}
      proof:
        code:
          paths: ['internal/**']
          check: sh proof-provider.sh --since {base} --json
      """
    And the scratch file "internal/example.go" contains exactly:
      """
      package example
      func Value() int { return 1 }
      """
    And the scratch fixture files are committed with message "chore: proof summary fixture" and tagged "summary-base"
    And the scratch file "internal/example.go" contains exactly:
      """
      package example
      func Value() int { return 2 }
      """
    And only "internal/example.go" is committed with message "chore: change example" and tagged "summary-head"
    And every itos invocation uses the binary under test from the scratch root with "--config itos.yaml", no pin and no inherited ITOS_CONFIG

  @slice-164 @ID-CI-52
  Scenario: A successful opt-in provider prints only the reported range-wide fields
    When I invoke itos with "ci run summary-base summary-head"
    Then stdout contains the complete line "Code proof summary: judged=1; unit=function; killed=6; excepted=0" exactly once
    And itos exits with code 0
    And that summary line follows the proof command and precedes "CI passed"
    And "proof-runs" contains exactly one line with "--since " followed by the full SHA of "summary-base" and " --json"
    And the summary line contains neither "T-001" nor "files=" nor "survived=" nor "uncovered="
    And the recorded file "ci-runs" equals exactly:
      """
      before
      tests @(?:ID-A-02)\b
      """

  @slice-164 @ID-CI-53
  Scenario: Explicit zero judged work retains its measured summary while refusing CI
    Given the scratch file "proof.stdout" contains exactly:
      """
      {"itos_proof":{"version":1,"judged":0,"unit":"function","killed":0}}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "refused" and exactly this normalized summary:
      """
      {"version":1,"judged":0,"unit":"function","files":null,"killed":0,"survived":null,"excepted":null,"uncovered":null}
      """
    And itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And failed_at equals the single failures entry with code 1 and the resolved proof command as step
    And that failure has exactly the problem rules "ci-code-proof-empty" in order
    And that rule's fix names "proof.code.check", "scope" and "base"
    And stderr contains the complete line "Code proof summary: judged=0; unit=function; killed=0" exactly once
    And stderr contains the complete line "CI failed at the code proof: " followed by the resolved proof command, after that summary line and before the line naming rule "ci-code-proof-empty"
    And neither stream contains "CI passed"
    And "proof-runs" has exactly 1 line
    And "ci-runs" contains only "before"

  @slice-164 @ID-CI-54
  Scenario Outline: A legacy successful provider gains an unavailable-summary report
    Given the scratch file "proof.stdout" contains exactly:
      """
      <stdout>
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then stderr contains the complete line "Code proof summary unavailable: provider did not report a summary." exactly once
    And itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true
    And the JSON proof has outcome "passed" and summary null
    And the JSON proof names the resolved proof command and full SHAs of "summary-base" and "summary-head"
    And "proof-runs" has exactly 1 line

    Examples:
      | case                         | stdout                                                                                                      |
      | empty output                 |                                                                                                             |
      | opaque text                  | proof completed                                                                                             |
      | invalid JSON                 | {"itos_proof":                                                                                              |
      | two JSON values              | {"itos_proof":{"version":1,"judged":0,"unit":"function"}} {}                                                   |
      | opaque marker                | log: itos_proof judged=0                                                                                     |
      | JSON null                    | null                                                                                                        |
      | JSON boolean                 | true                                                                                                        |
      | JSON number                  | 42                                                                                                          |
      | JSON string                  | "itos_proof"                                                                                                |
      | JSON array                   | [{"itos_proof":{"version":1,"judged":0,"unit":"function"}}]                                                    |
      | unrelated native JSON        | {"files":[{"judged":0,"killed":6}],"judged":0}                                                                |
      | nested envelope              | {"provider":{"itos_proof":{"version":1,"judged":0,"unit":"function"}}}                                       |
      | duplicate unknown native key | {"files":[],"files":[{"judged":0}]}                                                                          |
      | differently cased envelope   | {"ITOS_PROOF":{"version":1,"judged":0,"unit":"function"}}                                                   |

  @slice-164 @ID-CI-55
  Scenario Outline: Normalization keeps unavailable metrics separate from measured zero
    Given the scratch file "proof.stdout" contains exactly:
      """
      {"itos_proof":<summary>}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "passed" and exactly this normalized summary:
      """
      {"version":1,"judged":null,"unit":<unit>,"files":null,"killed":<killed>,"survived":null,"excepted":null,"uncovered":null}
      """
    And itos exits with code 0
    And stderr contains the complete line "<line>" exactly once
    And stdout is one JSON report with schema 1 and ok true
    And "proof-runs" has exactly 1 line

    Examples:
      | case                           | summary                                                          | unit         | killed | line                                                                   |
      | absent judged and unit         | {"version":1,"killed":6}                                          | null         | 6      | Code proof summary: killed=6                                            |
      | explicit null judged and unit  | {"version":1,"judged":null,"unit":null,"killed":6}                | null         | 6      | Code proof summary: killed=6                                            |
      | absent judged with null unit   | {"version":1,"unit":null,"killed":6}                              | null         | 6      | Code proof summary: killed=6                                            |
      | null judged with absent unit   | {"version":1,"judged":null,"killed":6}                            | null         | 6      | Code proof summary: killed=6                                            |
      | absent judged with valid unit  | {"version":1,"unit":"branch_case","killed":6}                     | "branch_case"| 6      | Code proof summary: unit=branch_case; killed=6                          |
      | null judged with valid unit    | {"version":1,"judged":null,"unit":"Branch-case_2","killed":0}   | "Branch-case_2"| 0    | Code proof summary: unit=Branch-case_2; killed=0                        |
      | no metrics                     | {"version":1}                                                     | null         | null   | Code proof summary unavailable: provider did not report a summary.      |
      | explicit null metrics          | {"version":1,"judged":null,"files":null,"killed":null}            | null         | null   | Code proof summary unavailable: provider did not report a summary.      |
      | unit without metrics           | {"version":1,"unit":"function"}                                  | "function"   | null   | Code proof summary: unit=function                                      |
      | differently cased count        | {"version":1,"Judged":0,"unit":"function"}                       | "function"   | null   | Code proof summary: unit=function                                      |

  @slice-164 @ID-CI-56
  Scenario Outline: Invalid opt-in metadata is a failed proof with a summary-contract problem
    Given the scratch file "proof.stdout" contains exactly:
      """
      <stdout>
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "cannot-run" and summary null
    And itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And failed_at equals the single failures entry with code 1 and the resolved proof command as step
    And that failure has exactly the problem rules "ci-code-proof-summary" in order
    And its "ci-code-proof-summary" problem message names the field "<field>" as a whole token, not followed by "."
    And stderr contains the complete line "CI failed at the code proof: " followed by the resolved proof command, before the line naming rule "ci-code-proof-summary"
    And stderr names rule "ci-code-proof-summary" and the field "<field>" as a whole token, not followed by "."
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:" or "CI passed"
    And "proof-runs" has exactly 1 line
    And "ci-runs" contains only "before"

    Examples:
      | case                         | stdout                                                                                                         | field                |
      | null envelope                | {"itos_proof":null}                                                                                             | itos_proof           |
      | array envelope               | {"itos_proof":[]}                                                                                               | itos_proof           |
      | string envelope              | {"itos_proof":"summary"}                                                                                        | itos_proof           |
      | numeric envelope             | {"itos_proof":1}                                                                                                | itos_proof           |
      | boolean envelope             | {"itos_proof":true}                                                                                             | itos_proof           |
      | missing version              | {"itos_proof":{}}                                                                                               | itos_proof.version   |
      | null version                 | {"itos_proof":{"version":null}}                                                                                 | itos_proof.version   |
      | unsupported version          | {"itos_proof":{"version":2}}                                                                                    | itos_proof.version   |
      | string version               | {"itos_proof":{"version":"1"}}                                                                                  | itos_proof.version   |
      | boolean version              | {"itos_proof":{"version":true}}                                                                                 | itos_proof.version   |
      | array version                | {"itos_proof":{"version":[]}}                                                                                   | itos_proof.version   |
      | object version               | {"itos_proof":{"version":{}}}                                                                                   | itos_proof.version   |
      | fractional version           | {"itos_proof":{"version":1.0}}                                                                                  | itos_proof.version   |
      | exponent version             | {"itos_proof":{"version":1e0}}                                                                                  | itos_proof.version   |
      | fraction count               | {"itos_proof":{"version":1,"judged":1.5,"unit":"function"}}                                                    | itos_proof.judged    |
      | integral fraction count      | {"itos_proof":{"version":1,"judged":1.0,"unit":"function"}}                                                    | itos_proof.judged    |
      | exponent count               | {"itos_proof":{"version":1,"judged":1e0,"unit":"function"}}                                                    | itos_proof.judged    |
      | zero exponent count          | {"itos_proof":{"version":1,"judged":0e0,"unit":"function"}}                                                    | itos_proof.judged    |
      | negative count               | {"itos_proof":{"version":1,"judged":-1,"unit":"function"}}                                                     | itos_proof.judged    |
      | negative zero count          | {"itos_proof":{"version":1,"judged":-0,"unit":"function"}}                                                     | itos_proof.judged    |
      | string count                 | {"itos_proof":{"version":1,"judged":"1","unit":"function"}}                                                    | itos_proof.judged    |
      | true count                   | {"itos_proof":{"version":1,"judged":true,"unit":"function"}}                                                   | itos_proof.judged    |
      | false count                  | {"itos_proof":{"version":1,"judged":false,"unit":"function"}}                                                  | itos_proof.judged    |
      | array count                  | {"itos_proof":{"version":1,"judged":[],"unit":"function"}}                                                     | itos_proof.judged    |
      | object count                 | {"itos_proof":{"version":1,"judged":{},"unit":"function"}}                                                     | itos_proof.judged    |
      | overflowing count            | {"itos_proof":{"version":1,"judged":9007199254740992,"unit":"function"}}                                       | itos_proof.judged    |
      | native files array           | {"itos_proof":{"version":1,"files":[{"killed":6}]}}                                                              | itos_proof.files     |
      | negative files               | {"itos_proof":{"version":1,"files":-1}}                                                                         | itos_proof.files     |
      | fractional killed            | {"itos_proof":{"version":1,"killed":0.5}}                                                                       | itos_proof.killed    |
      | exponent survived            | {"itos_proof":{"version":1,"survived":1e0}}                                                                     | itos_proof.survived  |
      | overflowing excepted         | {"itos_proof":{"version":1,"excepted":9007199254740992}}                                                        | itos_proof.excepted  |
      | boolean uncovered            | {"itos_proof":{"version":1,"uncovered":false}}                                                                  | itos_proof.uncovered |
      | judged without unit          | {"itos_proof":{"version":1,"judged":1}}                                                                        | itos_proof.unit      |
      | zero judged without unit     | {"itos_proof":{"version":1,"judged":0}}                                                                        | itos_proof.unit      |
      | judged with null unit        | {"itos_proof":{"version":1,"judged":1,"unit":null}}                                                             | itos_proof.unit      |
      | empty unit                   | {"itos_proof":{"version":1,"judged":1,"unit":""}}                                                               | itos_proof.unit      |
      | digit starts unit            | {"itos_proof":{"version":1,"judged":1,"unit":"1function"}}                                                      | itos_proof.unit      |
      | underscore starts unit       | {"itos_proof":{"version":1,"judged":1,"unit":"_function"}}                                                      | itos_proof.unit      |
      | non-ASCII unit               | {"itos_proof":{"version":1,"judged":1,"unit":"fünction"}}                                                       | itos_proof.unit      |
      | whitespace unit              | {"itos_proof":{"version":1,"judged":1,"unit":"function count"}}                                                 | itos_proof.unit      |
      | punctuation unit             | {"itos_proof":{"version":1,"judged":1,"unit":"function/count"}}                                                 | itos_proof.unit      |
      | overlong unit                | {"itos_proof":{"version":1,"judged":1,"unit":"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklm"}} | itos_proof.unit      |
      | numeric unit                 | {"itos_proof":{"version":1,"judged":1,"unit":1}}                                                                | itos_proof.unit      |
      | boolean unit                 | {"itos_proof":{"version":1,"judged":1,"unit":true}}                                                             | itos_proof.unit      |
      | array unit                   | {"itos_proof":{"version":1,"judged":1,"unit":[]}}                                                               | itos_proof.unit      |
      | object unit                  | {"itos_proof":{"version":1,"judged":1,"unit":{}}}                                                               | itos_proof.unit      |
      | bad unit with absent judged  | {"itos_proof":{"version":1,"unit":""}}                                                                          | itos_proof.unit      |
      | bad unit with null judged    | {"itos_proof":{"version":1,"judged":null,"unit":false}}                                                         | itos_proof.unit      |
      | identical top envelopes      | {"itos_proof":{"version":1},"itos_proof":{"version":1}}                                                         | itos_proof           |
      | conflicting top envelopes    | {"itos_proof":{"version":1},"itos_proof":{"version":2}}                                                         | itos_proof           |
      | identical versions           | {"itos_proof":{"version":1,"version":1}}                                                                       | itos_proof.version   |
      | conflicting versions         | {"itos_proof":{"version":2,"version":1}}                                                                       | itos_proof.version   |
      | identical judged counts      | {"itos_proof":{"version":1,"judged":1,"judged":1,"unit":"function"}}                                          | itos_proof.judged    |
      | conflicting judged counts    | {"itos_proof":{"version":1,"judged":0,"judged":1,"unit":"function"}}                                          | itos_proof.judged    |
      | duplicate null judged        | {"itos_proof":{"version":1,"judged":null,"judged":null}}                                                       | itos_proof.judged    |
      | identical units              | {"itos_proof":{"version":1,"unit":"function","unit":"function"}}                                             | itos_proof.unit      |
      | identical files              | {"itos_proof":{"version":1,"files":1,"files":1}}                                                               | itos_proof.files     |
      | identical killed             | {"itos_proof":{"version":1,"killed":6,"killed":6}}                                                             | itos_proof.killed    |
      | identical survived           | {"itos_proof":{"version":1,"survived":0,"survived":0}}                                                         | itos_proof.survived  |
      | identical excepted           | {"itos_proof":{"version":1,"excepted":0,"excepted":0}}                                                         | itos_proof.excepted  |
      | identical uncovered          | {"itos_proof":{"version":1,"uncovered":0,"uncovered":0}}                                                       | itos_proof.uncovered |

  @slice-164 @ID-CI-57
  Scenario Outline: A provider failure gains positive JSON execution evidence without accepting its counts
    Given the scratch file "proof.exit" contains "<exit>"
    And the scratch file "proof.stdout" contains exactly:
      """
      {"schema":1,"ok":false,"problems":[{"rule":"mutation.survived","message":"app.Value survived","fix":"test app.Value"}],"itos_proof":{"version":<version>,"judged":1,"unit":"function","killed":6}}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "<outcome>" and summary null
    And the JSON proof names the resolved proof command and full SHAs of "summary-base" and "summary-head"
    And itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false
    And failed_at equals the single failures entry with code 1 and the resolved proof command as step
    And that failure has exactly the problem rules "<rules>" in order
    And its "mutation.survived" problem keeps message "app.Value survived" and fix "test app.Value"
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:" or "CI passed"
    And "proof-runs" has exactly 1 line
    And "ci-runs" contains only "before"

    Examples:
      | exit | version | outcome    | rules                           |
      | 1    | 1       | refused    | mutation.survived               |
      | 2    | 1       | cannot-run | ci-code-proof,mutation.survived |
      | 3    | 1       | cannot-run | ci-code-proof,mutation.survived |
      | 70   | 1       | cannot-run | ci-code-proof,mutation.survived |
      | 75   | 1       | cannot-run | ci-code-proof,mutation.survived |
      | 127  | 1       | cannot-run | ci-code-proof,mutation.survived |
      | 1    | 2       | refused    | mutation.survived               |
      | 3    | 2       | cannot-run | ci-code-proof,mutation.survived |
      | 1    | "1"     | refused    | mutation.survived               |

  @slice-164 @ID-CI-58
  Scenario: Only an executed proof has a measured JSON result
    Given the commit "docs: add a note" touching only "README.md" on top of it is tagged "summary-docs"
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "passed" and exactly this normalized summary:
      """
      {"version":1,"judged":1,"unit":"function","files":null,"killed":6,"survived":null,"excepted":0,"uncovered":null}
      """
    And itos exits with code 0
    And the JSON proof names the resolved proof command and full SHAs of "summary-base" and "summary-head"
    And stderr contains the complete line "Code proof summary: judged=1; unit=function; killed=6; excepted=0" exactly once
    And "proof-runs" has exactly 1 line
    When I invoke itos with "ci plan summary-base summary-head --json"
    Then itos exits with code 0
    And the JSON plan has the resolved proof command once in order with proof true
    And stdout has no top-level proof member
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:"
    And "proof-runs" has exactly 1 line
    When I invoke itos with "ci run summary-base summary-head --tests --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true and no proof member
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:"
    And "proof-runs" has exactly 1 line
    And the latest "ci-runs" entry is exactly "tests @(?:ID-A-02)\b"
    When I invoke itos with "ci run summary-head summary-docs --json"
    Then itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true and no proof member
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:"
    And "proof-runs" has exactly 1 line
    Given the scratch file "before.exit" contains "3"
    When I invoke itos with "ci run summary-base summary-head --json"
    Then itos exits with code 1
    And stdout is one JSON report with schema 1 and ok false and no proof member
    And failed_at names step "sh record-ci.sh before" with code 3
    And neither stream contains "Code proof summary:" or "Code proof summary unavailable:"
    And "proof-runs" still has exactly the executed control's single line

  @slice-164 @ID-CI-59
  Scenario: JSON normalizes only common fields and keeps execution logs on stderr
    Given the scratch file "proof.stdout" contains exactly:
      """
      {"native":1,"native":2,"itos_proof":{"version":1,"judged":2,"unit":"Branch-case_2","files":9007199254740991,"killed":6,"note":"ignored","note":{"judged":0},"provider_native":{"files":[{"killed":999}]}}}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "passed" and exactly this normalized summary:
      """
      {"version":1,"judged":2,"unit":"Branch-case_2","files":9007199254740991,"killed":6,"survived":null,"excepted":null,"uncovered":null}
      """
    And the JSON proof has exactly keys command, from, to, outcome and summary
    And the JSON proof names command "sh proof-provider.sh --since {summary-base-sha} --json", from "{summary-base-sha}" and to "{summary-head-sha}" using the refs' full SHAs
    And itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true and no failed_at or failures members
    And stderr contains the complete line "Code proof summary: judged=2; unit=Branch-case_2; files=9007199254740991; killed=6" exactly once
    And stderr contains the complete line "provider diagnostic" exactly once
    And stdout contains neither "provider diagnostic" nor "Code proof summary:"
    And "proof-runs" contains exactly one line with "--since " followed by the full SHA of "summary-base" and " --json"

  @slice-164 @ID-CI-60
  Scenario: Positive JSON evidence retains descriptive counts without deriving a second verdict
    Given the scratch file "proof.stdout" contains exactly:
      """
      {"itos_proof":{"version":1,"judged":1,"unit":"branch_case","files":0,"killed":0,"survived":9,"excepted":22,"uncovered":4}}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then the JSON proof has outcome "passed" and exactly this normalized summary:
      """
      {"version":1,"judged":1,"unit":"branch_case","files":0,"killed":0,"survived":9,"excepted":22,"uncovered":4}
      """
    And itos exits with code 0
    And stdout is one JSON report with schema 1 and ok true and no failed_at or failures members
    And stderr contains the complete line "Code proof summary: judged=1; unit=branch_case; files=0; killed=0; survived=9; excepted=22; uncovered=4" exactly once
    And "proof-runs" has exactly 1 line
    And the recorded file "ci-runs" equals exactly:
      """
      before
      tests @(?:ID-A-02)\b
      """
