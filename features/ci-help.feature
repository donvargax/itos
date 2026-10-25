@phase-1
Feature: CI setup is discoverable without reading the implementation
  Issue #57: an agent setting up CI read itos's source to learn the forms a
  ci.steps entry takes and the range a step is given. The help and the
  defaults now say so. They describe what the planner (internal/plan) and the
  driver (internal/ci) already do and change neither: no step gains a
  default, no selection changes, and --print-defaults --json keeps its shape.
  The help texts live in internal/slices/ci/command.go (internal/cli/help.go
  only routes to them), the YAML defaults in internal/slices/config/run.go.
  The whole flag a step accepts is read by nothing (p1-ci-step-whole-inert
  owns that), so the help warns of it and never teaches it as working.

  # What the help gains. The ci help (the "" text, which itos ci --help and
  # itos help ci both print) gains, after its usage lines and a blank line:
  #
  #   The forms of a ci.steps entry, and the range each step is given
  #   (ITOS_FROM, ITOS_TO, {from} and {to}): itos help ci run.
  #
  # The ci run help (itos ci run --help, itos help ci run) gains, before its
  # --json paragraph:
  #
  #   Each entry of ci.steps and ci.nightly.steps is one of:
  #     <command>          a shell command; static when a ci.cost.static
  #                        pattern matches it, else late
  #     { run: <command>, cost: static|late }
  #                        a command whose cost: is its class, whatever the
  #                        patterns say (left out, as for a plain command)
  #     { tests: <kind> }  the run of tests.<kind>: its run.whole when the
  #                        range cannot be read (an empty or all-zeros
  #                        <from>, or no range) and on the nightly, else its
  #                        run.select over the smoke set, the tests the
  #                        range's footers name and the task checks merged
  #                        into it, and no run when that selects nothing; a
  #                        cost: sets its class too
  #   A step's whole: true is accepted and has no effect yet: a tests: step
  #   is narrowed to the range all the same.
  #   Every step and task check the run runs has the range's ends in its
  #   environment as ITOS_FROM and ITOS_TO, full SHAs, which ci.env cannot
  #   change: ITOS_FROM is <from>, or commits.since's commit when <from> is
  #   empty, all zeros or behind it, and is empty when neither gives a start;
  #   --nightly's are commits.since's commit (or empty) and HEAD. {from} and
  #   {to} in a step's text are filled in with the same ends, each as one
  #   quoted shell word (write --from {from}, not '{from}'), and ci plan
  #   prints the step so filled in.
  #
  # Each claim is the code's: plan.stepOf and check.CostOf (a cost: wins,
  # then ci.cost.static, else late, on a tests: step too), plan.Make and
  # tests.CommandFor (no readable range selects tests.Whole, so run.whole;
  # else run.select over run.ids_pattern; nothing selected drops the step),
  # nightlyPlan (a tests: step is run.whole), plan.EndsOf and
  # (*config.Loaded).RangeStart (the start, commits.since, empty), ci.Run (ITOS_FROM
  # and ITOS_TO set after ci.env), tests.FillRange (tests.ShellWord quotes
  # each end). Wording may be rewrapped; the phrases the scenarios quote
  # stay as written.
  #
  # What the help must not promise: that whole: true forces every test (the
  # schema's own about text says so; that is p1-ci-step-whole-inert's to
  # settle), that {from} or {to} is filled into a task check's command (only
  # steps are; a check reads ITOS_FROM), or what ITOS_TO is for ci run with
  # no range (both ends are empty today: an open question, so unsaid).
  #
  # Every scenario here runs itos in the scenario's own scratch folder: help
  # reads no config and needs no Given; --print-defaults runs in the scratch
  # repository the Given writes.

  # Fails at "its output says "itos help ci run"": the ci help is its usage
  # lines alone, which name "itos ci run" and no help topic.
  @wip @ID-CI-62 @slice-151
  Scenario: ci's help points to ci run's help for the step forms and the range a step is given
    When itos prints the help of "ci"
    Then itos exits with code 0
    And its output says "itos help ci run", whatever its line breaks
    And its output says "ITOS_FROM", whatever its line breaks

  # Fails at the first "its output says": ci run's help names run: and
  # tests: steps only in passing and shows no form of an entry.
  @wip @ID-CI-63 @slice-151
  Scenario: ci run's help shows each form a ci.steps entry takes
    When itos prints the help of "ci run"
    Then itos exits with code 0
    And its output says "{ run: <command>, cost: static|late }", whatever its line breaks
    And its output says "{ tests: <kind> }", whatever its line breaks
    And its output says "its run.whole when the range cannot be read", whatever its line breaks

  # Fails at "its output says "ITOS_FROM and ITOS_TO"": ci run's help names
  # neither variable, nor {from} and {to}.
  @wip @ID-CI-64 @slice-151
  Scenario: ci run's help says how a step is given the range, and when its start is empty
    When itos prints the help of "ci run"
    Then itos exits with code 0
    And its output says "ITOS_FROM and ITOS_TO", whatever its line breaks
    And its output says "is empty when neither gives a start", whatever its line breaks
    And its output says "each as one quoted shell word", whatever its line breaks

  # Fails at "its output says "whole: true is accepted and has no effect"":
  # ci run's help does not mention whole. The negative check keeps the
  # schema's promise ("every test of its kind") out of the help, folded as
  # the positive check is (a new step, the mirror of outputSaysFolded), so a
  # line break inside the phrase cannot let it through.
  @wip @ID-CI-65 @slice-151
  Scenario: ci run's help warns that a step's whole flag has no effect
    When itos prints the help of "ci run"
    Then itos exits with code 0
    And its output says "whole: true is accepted and has no effect", whatever its line breaks
    And its output does not say "every test of its kind", whatever its line breaks

  # The YAML --print-defaults prints gains, inside the ci mapping after its
  # last default key and before work:, the same in every repository:
  #
  #   # steps has no default. A step is a command, a command with its cost class,
  #   # or the run of a kind's named tests (itos help ci run). Each is given the
  #   # range's ends as ITOS_FROM and ITOS_TO, and as {from} and {to} in its text:
  #   # steps:
  #   #   - tools/lint-range {from} {to}
  #   #   - run: go vet ./...
  #   #     cost: static
  #   #   - tests: scenario
  #
  # each line indented two spaces, as ci's keys are, so uncommented it is a
  # ci.steps. Block style, since {from} cannot sit in a flow mapping's plain
  # scalar. whole is left out: an example teaches what works. --json is
  # unchanged (the corpus case "the defaults as --json" pins it), so the
  # YAML, read with its comments dropped, is still exactly the JSON's
  # defaults: the two new steps parse stdout as YAML, run config check
  # --print-defaults --json in the same folder, and compare the two as JSON
  # values (600 and 600.0 equal). They hold before the work, and guard that
  # the examples are comments only.
  #
  # Fails at "stdout contains the complete line "  # steps:" exactly once":
  # the YAML has no comment.
  @wip @ID-CI-66 @slice-151
  Scenario: --print-defaults shows ci.steps' forms as comments, the defaults unchanged
    Given a repository whose ledger has the task "T-001"
    When itos prints the defaults of the config
    Then itos exits with code 0
    And stdout contains the complete line "  # steps:" exactly once
    And stdout contains the complete line "  #   - tools/lint-range {from} {to}" exactly once
    And stdout contains the complete line "  #   - tests: scenario" exactly once
    And its output says "ITOS_FROM and ITOS_TO"
    And in stdout the line "ci:" comes before the line "  # steps:"
    And in stdout the line "  # steps:" comes before the line "work:"
    And the YAML on stdout reads as the "defaults" that "config check --print-defaults --json" prints in the same folder
    And the YAML on stdout has no key "ci.steps"
