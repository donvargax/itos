package check

import (
	"math"
	"os"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/ledger"
	"github.com/donvargax/itos/v5/internal/shell"
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
func RunCaptured(cfg *config.Loaded, c ledger.Check, cap float64, env []string) (Captured, error) {
	own := Timeout(cfg, c)
	seconds := math.Min(own, cap)
	dir, err := os.MkdirTemp("", "itos-check-")
	if err != nil {
		return Captured{}, err
	}
	defer os.RemoveAll(dir)
	file, err := os.Create(dir + "/output")
	if err != nil {
		return Captured{}, err
	}
	run := shell.Run(cfg, c.Command(), shell.Options{
		Stdout: file, Stderr: file, Timeout: duration(seconds), Env: env,
	})
	if err := file.Close(); err != nil {
		return Captured{}, err
	}
	output, err := os.ReadFile(file.Name())
	if err != nil {
		return Captured{}, err
	}
	result := Fail
	if !run.TimedOut && run.OK() != c.MustFail() {
		result = Pass
	}
	return Captured{
		Result:   result,
		Output:   string(output),
		Code:     run.Code,
		Seconds:  seconds,
		Capped:   cap < own,
		TimedOut: run.TimedOut,
	}, nil
}
