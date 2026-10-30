package judge

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/value"
)

// checkFailure is what went wrong with a failed check, in a few words.
func checkFailure(c ledger.Check, run check.Captured) string {
	command := "`" + c.Command() + "`"
	switch {
	case run.TimedOut && run.Capped:
		return fmt.Sprintf("%s ran past hooks.commit_msg.check_timeout (%ss)", command, value.Number(run.Seconds))
	case run.TimedOut:
		return fmt.Sprintf("%s ran past its timeout (%ss)", command, value.Number(run.Seconds))
	case c.MustFail() && run.Code == 0:
		return command + " passed, and must fail"
	case c.MustFail() && run.NotRun != "":
		return command + " did not run: " + run.NotRun
	case run.Code < 0:
		return command + " was stopped"
	}
	return fmt.Sprintf("%s exited %d", command, run.Code)
}

// Failure is the first check of a task that failed: the task, the command,
// what went wrong in a few words, the tail of what it printed (as long as a
// record of its evidence keeps), its exit code (-1 when it did not exit by
// itself) and whether its timeout stopped it.
type Failure struct {
	Task, Command, Why, Output string
	Code                       int
	TimedOut                   bool
}

// FirstFailure runs one task's checks before its first late one, an after:
// push check pending since the commit is not pushed, and gives the first
// failure, if any, with its output printed under the task's title and the
// check, as `itos task` prints them: the commit-msg hook's run.
func FirstFailure(cfg *config.Loaded, task ledger.Task, cap float64, env []string, stderr io.Writer, execution ...check.Execution) (string, error) {
	if task.Err != nil {
		return "", task.Err
	}
	failure, err := firstFailure(cfg, task, check.ChecksBeforeLate(cfg, task), cap, env, stderr, execution)
	if failure == nil {
		return "", err
	}
	return failure.Why, err
}

// StaticFailure is work done's run of one task's checks: every static one in
// written order, a static check written below a late one included, an after:
// push check pending, each under its own timeout, stopping at the first
// failure, printed as FirstFailure prints it. Late checks are CI's and never
// run.
func StaticFailure(cfg *config.Loaded, task ledger.Task, env []string, stderr io.Writer, execution ...check.Execution) (*Failure, error) {
	if task.Err != nil {
		return nil, task.Err
	}
	return firstFailure(cfg, task, check.StaticChecks(cfg, task), check.NoCap, env, stderr, execution)
}

// firstFailure runs the checks in order, an after: push check passed over,
// and describes the first that fails, with its output printed under the
// task's title and the check.
func firstFailure(cfg *config.Loaded, task ledger.Task, checks []check.CostedCheck, cap float64, env []string, stderr io.Writer, execution []check.Execution) (*Failure, error) {
	if len(execution) > 0 {
		at := execution[0]
		at.Log, at.Task = stderr, task.ID
		execution = []check.Execution{at}
	}
	for _, costed := range checks {
		c := costed.Check
		if c.Pushed() {
			continue
		}
		run, err := check.RunCaptured(cfg, c, cap, env, execution...)
		if err != nil {
			return nil, err
		}
		if run.Result == check.Pass {
			continue
		}
		mode := ""
		if c.MustFail() {
			mode = "   (must fail)"
		}
		fmt.Fprintf(stderr, "%s %s\n  $ %s%s\n", task.ID, task.Title, c.Command(), mode)
		if run.Output != "" {
			fmt.Fprint(stderr, run.Output)
			if !strings.HasSuffix(run.Output, "\n") {
				fmt.Fprintln(stderr)
			}
		}
		return &Failure{
			Task: task.ID, Command: c.Command(), Why: checkFailure(c, run), Output: tail(run.Output),
			Code: run.Code, TimedOut: run.TimedOut,
		}, nil
	}
	return nil, nil
}

// tail is the last bytes of a check's output that a record of its evidence
// would keep.
func tail(output string) string {
	return output[max(0, len(output)-evidence.OutputTail):]
}
