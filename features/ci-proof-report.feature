@phase-1
Feature: CI reports whether its configured ranged code proof is scheduled
  # #36 and the explicit-warning alternative in #35. Reporting only: which
  # proof is selected, its command, order and execution, provider parsing,
  # exit codes and JSON fields stay as they are, and no proof runs, and no
  # base is made up, to explain an omission.
  #
  # The texts, exactly. A scheduled proof's ci plan row is the step row
  # with a check row's action suffix: "<cost>  <command>   (code proof)".
  # ci run prints the line "Code proof" right before the proof's unchanged
  # "$ <command>" line (slice-164 reads that line). Both say the proof is
  # scheduled, not that it passed; "Code proof summary" stays slice-164's,
  # after a proof that ran. With proof.code configured and no proof step in
  # a push's plan, one line says why:
  #   Code proof not scheduled: the range has no start.
  #   Code proof not scheduled: the range's changes could not be read.
  #   Code proof not scheduled: the range changes no path in proof.code.paths.
  # ci plan prints it last, after the rows; ci run prints it in its log
  # before the first step's "$" line, beside the not-started and prose lines.
  #
  # Streams. ci plan: the row and the line on stdout, like every row.
  # ci plan --json: the plan object as before, order[].proof marking the
  # proof, and nothing added to either stream. ci run: the label and the
  # line in its log, on stdout. ci run --json: in its log on stderr; stdout
  # stays one report (slice-164's proof member only after a proof ran).
  #
  # Which line, decided in this order on the range as typed, which is what
  # plan.Changed compares:
  # 1. no start: the typed start is empty or all zeros (config.NewBranch:
  #    ci plan --whole, ci run with no range, a new branch's push), even when
  #    commits.since gives range checks a {from} (config.RangeStart), since
  #    such a range's paths are never compared and no read failed; or the
  #    resolved start (plan.Ends.From) is empty. Scheduling the proof from
  #    commits.since instead would change selection, which is not this slice.
  # 2. could not be read: plan.Changed's read (git diff <from> <to>, git log
  #    for the unpushed commits) returns an error, which it drops today.
  #    plan.Readable (git rev-list --quiet) is not the signal: it walks
  #    commits only, so it passes where the diff fails, as in @ID-CI-49.
  # 3. no path: the read succeeded and no changed path is in
  #    proof.code.paths (proof.Touches false).
  # No line without proof.code, for ci run --tests, or for the nightly (ci
  # plan --nightly, ci run --nightly): those keep their contracts. No new
  # JSON field and no generated CI list in AGENTS.md (q-45).
  #
  # The fixture is slice-164's (ci-proof-summary.feature), its tag names
  # kept so its steps serve here too, and a commit touching only README.md
  # tagged summary-docs. A step's {<tag>-sha} is that tag's full SHA.

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
    And the commit "docs: add a note" touching only "README.md" on top of it is tagged "summary-docs"
    And every itos invocation uses the binary under test from the scratch root with "--config itos.yaml", no pin and no inherited ITOS_CONFIG

  # Fails before the work at the plan's row: today it is the bare step row.
  # The JSON plan's steps are a guard folded in: order[].proof stays the
  # marker and plan --json gains no label.
  @wip @ID-CI-46 @slice-155
  Scenario: A scheduled proof is labelled in the plan and where the run reaches it
    When I invoke itos with "ci plan summary-base summary-head"
    Then itos exits with code 0
    And stdout contains the complete line "static  sh proof-provider.sh --since {summary-base-sha} --json   (code proof)" exactly once, the tags' full SHAs filled in
    And neither stream contains "Code proof not scheduled"
    And "proof-runs" has exactly 0 lines
    When I invoke itos with "ci plan summary-base summary-head --json"
    Then itos exits with code 0
    And the JSON plan has the resolved proof command once in order with proof true
    And neither stream contains "code proof" or "Code proof"
    When I invoke itos with "ci run summary-base summary-head"
    Then itos exits with code 0
    And stdout contains the complete line "Code proof" exactly once
    And in stdout the line "Code proof" comes right before the line "$ sh proof-provider.sh --since {summary-base-sha} --json", the tags' full SHAs filled in
    And neither stream contains "Code proof not scheduled"
    And "proof-runs" has exactly 1 line

  # Fails before the work at the plan's last line. The plan --json steps
  # are a guard folded in: the explanation is text only.
  @wip @ID-CI-47 @slice-155
  Scenario: A readable range that changes no proof path says why the proof is not scheduled
    When I invoke itos with "ci plan summary-head summary-docs"
    Then itos exits with code 0
    And the last line of stdout is "Code proof not scheduled: the range changes no path in proof.code.paths."
    And stdout contains the complete line "Code proof not scheduled: the range changes no path in proof.code.paths." exactly once
    And stdout contains neither "(code proof)" nor "proof-provider.sh"
    When I invoke itos with "ci plan summary-head summary-docs --json"
    Then itos exits with code 0
    And neither stream contains "Code proof not scheduled" or "code proof"
    When I invoke itos with "ci run summary-head summary-docs"
    Then itos exits with code 0
    And stdout contains the complete line "Code proof not scheduled: the range changes no path in proof.code.paths." exactly once
    And in stdout the line "Code proof not scheduled: the range changes no path in proof.code.paths." comes before the line "$ sh record-ci.sh before"
    And stdout contains the complete line "CI passed" exactly once
    And "proof-runs" has exactly 0 lines

  # Fails before the work at the plan's last line. No commits.since here:
  # @ID-CI-61 is the start commits.since supplies.
  @wip @ID-CI-48 @slice-155
  Scenario: A whole run says the proof has no range start
    When I invoke itos with "ci plan --whole"
    Then itos exits with code 0
    And the last line of stdout is "Code proof not scheduled: the range has no start."
    And stdout contains neither "(code proof)" nor "proof-provider.sh"
    When I invoke itos with "ci run"
    Then itos exits with code 0
    And stdout contains the complete line "Code proof not scheduled: the range has no start." exactly once
    And in stdout the line "Code proof not scheduled: the range has no start." comes before the line "$ sh record-ci.sh before"
    And "proof-runs" has exactly 0 lines

  # commits.since gives range checks a start for an all-zeros or empty one,
  # but plan.Changed compares the range as typed, so its paths are never
  # compared and the proof is left out: the missing-start line, not the
  # unreadable one, though git diff of an all-zeros start fails. Fails
  # before the work at the plan's last line.
  @wip @ID-CI-61 @slice-155
  Scenario: A range whose only start is commits.since says the proof has no range start
    Given the scratch itos.yaml's commits.since is the full SHA of the tag "summary-base"
    When I invoke itos with "ci plan 0000000000000000000000000000000000000000 summary-head"
    Then itos exits with code 0
    And the last line of stdout is "Code proof not scheduled: the range has no start."
    And stdout contains neither "could not be read" nor "changes no path"
    When I invoke itos with "ci plan --whole"
    Then itos exits with code 0
    And the last line of stdout is "Code proof not scheduled: the range has no start."
    When I invoke itos with "ci run 0000000000000000000000000000000000000000 summary-head"
    Then itos exits with code 0
    And stdout contains the complete line "Code proof not scheduled: the range has no start." exactly once
    And stdout contains neither "could not be read" nor "changes no path"
    And "proof-runs" has exactly 0 lines

  # Unreadable is plan.Changed's read failing. Without the start commit's
  # tree, git diff fails while git rev-list --quiet, plan.Readable, still
  # walks the range and the refs still name commits, so ci plan and ci run
  # keep exit 0. Fails before the work at the plan's last line.
  @wip @ID-CI-49 @slice-155
  Scenario: A range whose changes cannot be read is not reported as changing no proof path
    Given the tree of the commit tagged "summary-base" is missing from the repository's objects
    When I invoke itos with "ci plan summary-base summary-head"
    Then itos exits with code 0
    And the last line of stdout is "Code proof not scheduled: the range's changes could not be read."
    And stdout contains neither "changes no path" nor "(code proof)"
    When I invoke itos with "ci run summary-base summary-head"
    Then itos exits with code 0
    And stdout contains the complete line "Code proof not scheduled: the range's changes could not be read." exactly once
    And stdout contains neither "changes no path" nor "proof-provider.sh"
    And "proof-runs" has exactly 0 lines

  # Fails before the work at the label on stderr.
  @wip @ID-CI-50 @slice-155
  Scenario: Under --json the label and the explanation go to stderr and stdout stays one report
    Given the scratch file "proof.exit" contains "1"
    And the scratch file "proof.stdout" contains exactly:
      """
      {"schema":1,"ok":false,"problems":[{"rule":"mutation.survived","message":"app.Value survived","fix":"test app.Value"}]}
      """
    When I invoke itos with "ci run summary-base summary-head --json"
    Then itos exits with code 1
    And stderr contains the complete line "Code proof" exactly once
    And in stderr the line "Code proof" comes right before the line "$ sh proof-provider.sh --since {summary-base-sha} --json", the tags' full SHAs filled in
    And stdout is one JSON report with schema 1 and ok false
    And failed_at equals the single failures entry with code 1 and the resolved proof command as step
    When I invoke itos with "ci run summary-head summary-docs --json"
    Then itos exits with code 0
    And stderr contains the complete line "Code proof not scheduled: the range changes no path in proof.code.paths." exactly once
    And stdout is one JSON report with schema 1 and ok true and no proof member

  # A guard: it passes before the fix and must after it. The label and the
  # line go through the plan's printer and the run's preamble, which these
  # modes share; none of them selects a ranged proof.
  @wip @ID-CI-51 @slice-155 @guard
  Scenario: Modes that select no ranged proof gain neither the label nor an explanation
    When I invoke itos with "ci run summary-base summary-head --tests"
    Then itos exits with code 0
    And neither stream contains "Code proof" or "code proof"
    And "proof-runs" has exactly 0 lines
    When I invoke itos with "ci plan --nightly"
    Then itos exits with code 0
    And neither stream contains "Code proof" or "code proof"
    When I invoke itos with "ci run --nightly"
    Then itos exits with code 0
    And neither stream contains "Code proof" or "code proof"
    And "proof-runs" has exactly 0 lines
    Given the scratch itos.yaml's proof section is removed
    When I invoke itos with "ci plan summary-head summary-docs"
    Then itos exits with code 0
    And neither stream contains "Code proof" or "code proof"
    And stdout contains the complete line "static  sh record-ci.sh before" exactly once
