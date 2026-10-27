// Package ci is CI's driver: the plan internal/plan made, carried out item
// by item in the order it gives, so that itos ci run runs what itos ci plan
// prints and never plans again (internal/cli/ci.go makes the plan as ci plan
// does and hands it over).
//
//   - Before the first item comes the preamble: the tasks a footer names that
//     no task file has (which end the run, whatever the settings, named
//     against the ledger's folder), the tasks named but not started, and,
//     for a prose-only range, what it leaves out, and the plan's line that
//     says why a configured code proof is not scheduled (ProofOmitted,
//     slice 155).
//   - A step runs through shell.Run on itos's own streams, so the command's
//     output keeps its place beside itos's lines; a failing step makes the
//     run exit 1, naming the step's own exit code: passed through, a step's
//     75 or 3 would read as itos's meaning of it.
//   - The range's code proof, the plan's proof step, is announced by the log
//     line "Code proof" right before its "$" line (scheduled, not passed),
//     and runs through internal/proof, not shell.Run: the config's proof.code.check answers by
//     its --json, so a survivor, an uncovered or a missing result fails the
//     run naming each problem's rule and message, and a check that cannot run
//     fails it too, never a pass (slice 106, issue #28). The run's exit code
//     is 1, as for any other failing step. A check that passes may report
//     what it judged in an itos_proof envelope (slice 164, issue #25): the
//     log gets one line of it before the result, a broken envelope or a
//     judged of 0 fails the run with a problem of its own, and --json gains
//     proof, the executed proof's command, ends, outcome and summary.
//   - A task check the plan runs goes through a verbose check.Runner, so its
//     command line and output keep their place in the log; a failure exits 1,
//     naming the task and its title. A check merged into the run of named
//     tests, covered by a step, left out by ci.nightly_only or pending until
//     after the push is only logged, and never fails the run. Each named
//     task's checks run as its own, a check two tasks share once for each.
//   - The first failure ends a push's run, unless ci.stop_at_first_failure
//     is false: then everything runs, the first failure says "CI failed at"
//     and each later one "also failed at", and the first is the run's. The
//     nightly (Options.Nightly) always goes on and fails at the end, whatever
//     the setting says: it runs once a day to report, and a failure that
//     stops it hides every step after it for a day (slice 133).
//
// With Options.Annotate (itos ci run sets it when GITHUB_ACTIONS is "true",
// slice 198) each failure the run reports is also an error annotation, a
// workflow command on stderr beside its lines, at most 10 a run (the first
// in run order; GitHub keeps no more in a step): "::error file=<path>,line=<n>::"
// when the failure names a file and line, else "::error::", then "<what>
// failed: <reason>". A step's or task check's file and line is the first line
// of its stdout, else of its stderr (an evidence run's merged output is
// stdout), that begins with a relative <path>:<line>: naming a file of the
// folder the commands run in, the path written from the repository's top
// level; a code proof problem's are its subject's file and line, as given.
// Annotating reads those streams as the command writes them, so the command
// no longer gets itos's own files.
//
// A step that is a mapping with run: and an evidence declaration, and a task
// check with one, are looked up by the index tree through Options.Evidence
// (internal/engine/evidence) and run, on a miss, in a checkout of it; --json then
// lists the evidence of each command looked up, in run order, and the
// tracked paths the index leaves out (unstaged). Without an engine, or
// without a declaration, they run as before.
//
// Every step and check sees ci.env, and the range's ends (plan.Ends) as
// ITOS_FROM and ITOS_TO, which ci.env cannot override. The log goes to
// stdout, or to stderr under --json, which keeps stdout for its one object;
// --json's failed_at is the first Failure, and its failures every one, in
// run order.
package ci
