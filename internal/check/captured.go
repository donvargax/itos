package check

import (
	"context"
	"io"
	"math"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/shell"
)

// Captured is a check run quietly (checks.ts's Captured): whether it passed,
// what it printed, stdout and stderr together, and how it ended.
type Captured struct {
	Result Result
	Output string
	// Code is its exit code, -1 when it did not exit by itself (stopped by a
	// signal, or never started): spawnSync's status null.
	Code int
	// Seconds are the seconds it was given, Capped whether the cap set them,
	// and TimedOut whether it outlasted them.
	Seconds  float64
	Capped   bool
	TimedOut bool
	// NotRun is why it did not run to completion, "" when it did
	// (shell.Result.NotRun): what fails a fails: check that exited non-zero.
	NotRun string
}

// Execution relocates a check and its captured output without changing policy.
// Empty paths retain the caller's working and temporary directories.
// Evidence, when set, answers a check that opts in from a record and runs a
// miss in a checkout of its own, rather than in Dir (RunCaptured); Log is
// where a hit says so and Task the task the check is for, both only for that.
//
// A check with snapshot: false runs in the working folder whatever Dir says,
// its output captured beside nothing of the snapshot's, in WorkEnv when that
// is set and else in the environment the caller gives RunCaptured.
type Execution struct {
	Dir, TempDir string
	WorkEnv      []string
	Evidence     *evidence.Engine
	Log          io.Writer
	Task         string
}

// evidenceRequest is the check as the evidence engine looks it up: keyed by
// its own timeout, run for at most seconds.
func evidenceRequest(cfg *config.Loaded, c ledger.Check, seconds float64, at Execution) evidence.Request {
	return evidence.Request{
		Command: c.Command(), Seconds: Timeout(cfg, c), RunSeconds: &seconds, Decl: *c.Evidence, Task: at.Task, Log: at.Log,
	}
}

// Answers is whether the engine would answer the check from a record: it
// opts in, is not an after: push check, and a record of what it would run on
// is there.
func Answers(e *evidence.Engine, cfg *config.Loaded, c ledger.Check) bool {
	if e == nil || !c.Evidenced() || c.Pushed() {
		return false
	}
	return e.Hit(context.Background(), evidenceRequest(cfg, c, Timeout(cfg, c), Execution{}))
}

// NoCap is a cap that leaves a check's own timeout as it is.
var NoCap = math.Inf(1)

// RunCaptured runs one check quietly, its output kept to print when it fails
// (checks.ts's runCheckCaptured), with its timeout capped at cap seconds:
// the commit-msg hook's run of a named task's checks. The output goes to a
// file rather than a pipe, so a command it leaves running cannot hold the run
// past its timeout; a check past its timeout fails whatever its code. env is
// the environment it runs in (itos's own when nil). It runs every check it
// is given: an after: push check is its caller's to leave pending.
func RunCaptured(cfg *config.Loaded, c ledger.Check, cap float64, env []string, execution ...Execution) (Captured, error) {
	var at Execution
	if len(execution) > 0 {
		at = execution[0]
	}
	own := Timeout(cfg, c)
	seconds := math.Min(own, cap)
	if c.NoSnapshot {
		at.Dir, at.TempDir = "", ""
		if at.WorkEnv != nil {
			env = at.WorkEnv
		}
	}
	if at.Evidence != nil && c.Evidenced() && !c.Pushed() {
		o := at.Evidence.Run(context.Background(), evidenceRequest(cfg, c, seconds, at))
		if o.Err != nil {
			return Captured{}, o.Err
		}
		notRun := shell.NotRun(o.Code, o.TimedOut)
		return Captured{
			Result: captured(c, o.OK, o.TimedOut, notRun), Output: o.Output, Code: o.Code,
			Seconds: seconds, Capped: cap < own, TimedOut: o.TimedOut, NotRun: notRun,
		}, nil
	}
	dir, err := os.MkdirTemp(at.TempDir, "itos-check-")
	if err != nil {
		return Captured{}, err
	}
	defer os.RemoveAll(dir)
	file, err := os.Create(dir + "/output")
	if err != nil {
		return Captured{}, err
	}
	run := shell.Run(cfg, c.Command(), shell.Options{
		Stdout: file, Stderr: file, Timeout: duration(seconds), Env: env, Dir: at.Dir,
	})
	if err := file.Close(); err != nil {
		return Captured{}, err
	}
	output, err := os.ReadFile(file.Name())
	if err != nil {
		return Captured{}, err
	}
	notRun := run.NotRun()
	return Captured{
		Result:   captured(c, run.OK(), run.TimedOut, notRun),
		Output:   string(output),
		Code:     run.Code,
		Seconds:  seconds,
		Capped:   cap < own,
		TimedOut: run.TimedOut,
		NotRun:   notRun,
	}, nil
}

// captured is a captured run's verdict: a check past its timeout fails
// whatever its code, else it is the check's verdict.
func captured(c ledger.Check, ok, timedOut bool, notRun string) Result {
	if timedOut {
		return Fail
	}
	return verdict(c, ok, notRun)
}
