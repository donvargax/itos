// Package refs is the commit refs engine (ADR-0053, step 9 of
// docs/plans/slices-and-ports.md): a ref a command is given, judged before
// the command uses it, which commit, verify, tests list and the ci commands
// share.
//
// Check refuses a ref that names no commit with a usage error (out.Usage),
// in a line for people rather than git's command line (rule 25 of
// docs/CLI.md). An empty or all-zero start (a new
// branch, which ranges read as everything up to its end) and
// vcsport.Unpushed name none to check; outside a repository nothing is
// checked, and the command says what it says there. CheckRange is Check for
// a range a CI run reads: its start may be a full commit name the
// repository does not have, a history rewritten since the provider saw it,
// which the plan reads as a range it cannot read (internal/plan). Both ask
// the VCS port the caller binds.
package refs
