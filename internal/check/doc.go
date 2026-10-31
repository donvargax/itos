// Package check runs a task's checks and gives each its cost class, so a
// check means one thing wherever it runs: itos task, CI's plan and driver,
// and the commit-msg hook.
//
// A Runner is one invocation's runs, keyed by Key: the command, its
// whitespace collapsed (config.Normal), its timeout and its evidence
// declaration, or none, with snapshot: false when a check declares evidence and
// runs in the working folder, so that it is never taken for the one that runs
// in the snapshot. Its Run prints the
// verbose command line and runs each distinct check once, every later task
// that lists it reading the kept exit status by its own run: or fails:. The
// runs live for the invocation alone, and CI's driver keeps none, so a push
// runs each named task's checks as its own.
//
// A run: check passes when its command exits 0. A fails: check passes only
// when its command ran to completion and exited non-zero: one that timed
// out, could not start, or that sh could not find (127) or execute (126)
// fails, whatever its code (shell.Result.NotRun), on every path, an answer
// from evidence included; a verbose run prints "did not run:" and why, and
// Captured.NotRun gives why to the hook's message.
//
// A Runner given an evidence engine (internal/engine/evidence) looks a check that
// opts in up by the index tree and runs a miss in a checkout of it, recording
// the result; RunInfo gives the check's evidence beside its Result, and a
// later check that reads the exit status of a run the invocation already made
// reports that run's evidence, not recorded again. A check that does not opt
// in, or a Runner with no engine (itos task --working-tree), runs in the
// working folder, nothing looked up or written. So does a check with snapshot:
// false (ledger.Check.NoSnapshot) wherever it runs, in itos task, work done,
// ci run and the commit-msg hook, evidence or not; Execution.WorkEnv is the
// environment the hook gives it.
//
// CostOf, CostedChecks, ChecksBeforeLate, StaticChecks and LateChecks are the
// cost classes and the written order: a check is static by its own cost:
// static, else by a pattern of ci.cost.static, else late, and a task's checks
// keep their written order, so the commit-msg hook runs a task's checks only
// up to its first late one, while work done runs every static one (a static
// check written below a late one included, unless ci.cost.keep_written_order
// makes it late) and names the late ones as left to CI. (config check's written-order rule stays in internal/ledger,
// since it reads the ledger before it is typed.)
//
// RunCaptured is how the commit-msg hook runs a check: quiet, stdout and
// stderr in one temporary file rather than a pipe, so a command it leaves
// running cannot hold the commit past its timeout, which
// hooks.commit_msg.check_timeout caps (Captured.Capped says the cap set it);
// a timeout is a failure whatever the code, and the check runs in the hook's
// environment less GIT_INDEX_FILE. An after: push check is Pending until a
// remote branch holds HEAD (Pushed).
// Execution can place the command in a private checkout and its captures in
// a sibling temporary directory, leaving the caller's CWD and policy unchanged.
// It can also carry an evidence engine: RunCaptured then answers a check that
// opts in from a record, or runs a miss in a checkout of the engine's own,
// holding the run to the cap while the key binds the check's own timeout, and
// Answers says, without running, whether a record would answer.
package check
