package judge

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
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
	case run.Code < 0:
		return command + " was stopped"
	}
	return fmt.Sprintf("%s exited %d", command, run.Code)
}

// FirstFailure runs one task's checks before its first late one, an after:
// push check pending since the commit is not pushed, and gives the first
// failure, if any, with its output printed under the task's title and the
// check, as `itos task` prints them.
func FirstFailure(cfg *config.Loaded, task ledger.Task, cap float64, env []string, stderr io.Writer, execution ...check.Execution) (string, error) {
	if task.Err != nil {
		return "", task.Err
	}
	if len(execution) > 0 {
		at := execution[0]
		at.Log, at.Task = stderr, task.ID
		execution = []check.Execution{at}
	}
	for _, costed := range check.ChecksBeforeLate(cfg, task) {
		c := costed.Check
		if c.Pushed() {
			continue
		}
		run, err := check.RunCaptured(cfg, c, cap, env, execution...)
		if err != nil {
			return "", err
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
		return checkFailure(c, run), nil
	}
	return "", nil
}
