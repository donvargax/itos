// Package shell is the one way itos starts a command the config or the
// ledger gives it (tools/itos/shell.ts): a task check, and later a CI step,
// a header-lint delegate, a range check, a provider's or a test adapter's
// command, the pre-push commands and the smoke run. Each goes through the
// shell the config names, `shell`: the argv prefix the command is appended to
// as one argument, [sh, -c] by default. itos's own git calls start directly
// (internal/git).
package shell

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/donvargax/itos/v2/internal/config"
)

// Grace is how long a command past its timeout has, after SIGTERM, before it
// is killed. Node's spawnSync sends SIGTERM alone and waits for the exit
// however long it takes; the port does not hang on a command that ignores
// the signal.
const Grace = 10 * time.Second

// Options are how a command runs: its streams, each nil for /dev/null (an
// *os.File is handed to the command as it is, so its output keeps its place
// beside what itos prints), its timeout (none when 0) and its environment
// (itos's own when nil).
type Options struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Timeout        time.Duration
	Env            []string
}

// Result is how a command ended. Code is its exit code, -1 when it did not
// exit by itself (stopped by a signal, or never started); TimedOut is
// whether it outlasted its timeout; Err is why it could not start, if it
// could not (the shell missing, say), which spawnSync reports as a result
// too, never as a throw.
type Result struct {
	Code     int
	TimedOut bool
	Err      error
	// Signal is the signal that stopped it, 0 when none did.
	Signal syscall.Signal
}

// signals are the names Node gives the signals a command most often dies of.
var signals = map[syscall.Signal]string{
	syscall.SIGHUP: "SIGHUP", syscall.SIGINT: "SIGINT", syscall.SIGQUIT: "SIGQUIT",
	syscall.SIGILL: "SIGILL", syscall.SIGTRAP: "SIGTRAP", syscall.SIGABRT: "SIGABRT",
	syscall.SIGBUS: "SIGBUS", syscall.SIGFPE: "SIGFPE", syscall.SIGKILL: "SIGKILL",
	syscall.SIGSEGV: "SIGSEGV", syscall.SIGPIPE: "SIGPIPE", syscall.SIGALRM: "SIGALRM",
	syscall.SIGTERM: "SIGTERM",
}

// Status is how the command ended as spawnSync's `status ?? signal` writes
// it: its exit code, else the signal's name, else null (it never started).
func (r Result) Status() string {
	switch {
	case r.Code >= 0:
		return strconv.Itoa(r.Code)
	case r.Signal != 0 && signals[r.Signal] != "":
		return signals[r.Signal]
	case r.Signal != 0:
		return r.Signal.String()
	}
	return "null"
}

// OK is whether the command exited 0: spawnSync's status 0, which a command
// that traps the timeout's SIGTERM and exits 0 also has. A caller that fails
// a command past its timeout whatever its code reads TimedOut beside it.
func (r Result) OK() bool { return r.Code == 0 && r.Err == nil }

// Argv is the command as the config's shell runs it: shell's words, then the
// command as one argument. An empty shell is sh with no flags, as shell.ts's
// `[shell = "sh", ...flags]` reads it.
func Argv(cfg *config.Loaded, command string) []string {
	prefix := cfg.Shell
	if len(prefix) == 0 {
		prefix = []string{"sh"}
	}
	return append(append([]string{}, prefix...), command)
}

// Run runs a command through the config's shell and waits for it. Past its
// timeout it gets SIGTERM, as spawnSync's default killSignal, then Grace to
// exit before it is killed.
func Run(cfg *config.Loaded, command string, o Options) Result {
	ctx, cancel := context.Background(), context.CancelFunc(func() {})
	if o.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
	}
	defer cancel()
	argv := Argv(cfg, command)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr, cmd.Env = o.Stdin, o.Stdout, o.Stderr, o.Env
	cmd.Cancel = func() error {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = Grace
	err := cmd.Run()
	timedOut := ctx.Err() == context.DeadlineExceeded
	var exit *exec.ExitError
	switch {
	case err == nil:
		return Result{Code: 0}
	case errors.As(err, &exit):
		r := Result{Code: exit.ExitCode(), TimedOut: timedOut}
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			r.Signal = ws.Signal()
		}
		return r
	case cmd.ProcessState != nil:
		// It ran and exited, and Wait still failed: it trapped the timeout's
		// SIGTERM and exited 0 (Wait gives the context's error), or left a
		// pipe open past Grace.
		return Result{Code: cmd.ProcessState.ExitCode(), TimedOut: timedOut}
	}
	return Result{Code: -1, TimedOut: timedOut, Err: err}
}
