// Package ci is CI's driver: the plan internal/plan made, carried out item
// by item in the order it gives, so that itos ci run runs what itos ci plan
// prints and never plans again (internal/cli/ci.go makes the plan as ci plan
// does and hands it over).
//
//   - Before the first item comes the preamble: the tasks a footer names that
//     no task file has (which end the run, whatever the settings, named
//     against the ledger's folder), the tasks named but not started, and,
//     for a prose-only range, what it leaves out.
//   - A step runs through shell.Run on itos's own streams, so the command's
//     output keeps its place beside itos's lines; a failing step makes the
//     run exit 1, naming the step's own exit code: passed through, a step's
//     75 or 3 would read as itos's meaning of it.
//   - A task check the plan runs goes through a verbose check.Runner, so its
//     command line and output keep their place in the log; a failure exits 1,
//     naming the task and its title. A check merged into the run of named
//     tests, covered by a step, left out by ci.nightly_only or pending until
//     after the push is only logged, and never fails the run. The nightly
//     shares one Runner's runs across its done tasks, so a check they share
//     runs once; a push runs each named task's checks as its own.
//   - The first failure ends the run, unless ci.stop_at_first_failure is
//     false: then everything runs, each failure says where, and the first is
//     the run's.
//
// Every step and check sees ci.env, and the range's ends (plan.Ends) as
// ITOS_FROM and ITOS_TO, which ci.env cannot override. The log goes to
// stdout, or to stderr under --json, which keeps stdout for its one object;
// --json's failed_at is a Failure.
package ci
