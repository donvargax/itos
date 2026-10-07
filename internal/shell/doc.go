// Package shell is the one way itos starts a command the config or the
// ledger gives it: a task check, a CI step, a header-lint delegate, a range
// check, a test adapter's command, the pre-push commands and the smoke run.
// Each goes through the shell the config names, shell: the argv prefix the
// command is appended to as one argument, [sh, -c] by default (Argv).
// itos's own git calls start directly (internal/git).
//
// Run starts a command with its streams (an *os.File handed over as it is,
// so a check's output keeps its place beside what itos prints; nil is the
// null device), its timeout and its environment, and says how it ended in a
// Result: the code, whether it timed out, why it could not start. Past its
// timeout a command gets SIGTERM, then Grace (10 seconds) before it is
// killed. A command that traps the signal and exits 0 has code 0, so a
// caller that fails a timeout whatever its code reads TimedOut. Status is
// the exit written as Node's spawnSync wrote it, status ?? signal, which the
// messages quote.
package shell
