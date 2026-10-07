package ci

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/check"
	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/plan"
)

func load(t *testing.T) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func run(t *testing.T, p *plan.Plan, o Options) (int, string, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	o.Stdout, o.Stderr = &stdout, &stderr
	code, err := Run(load(t), p, o)
	if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// A step with no exit code of its own (stopped by a signal) fails the run
// with 1, as spawnSync's `status ?? 1` reads it.
func TestStepStoppedBySignalExitsOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no signals: Git for Windows' sh reports a killed step as an exit code")
	}
	step := plan.Step{Command: "kill -TERM $$"}
	code, _, stderr := run(t, &plan.Plan{Order: []plan.Item{{Step: &step}}}, Options{})
	if code != 1 || stderr != "\nCI failed at: kill -TERM $$ (it gave no exit code)\n" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// A failing step makes the run exit 1, whatever its own code, which it names
// on stderr and as failed_at's code (slice 86).
func TestAFailingStepExitsOneNamingItsCode(t *testing.T) {
	step := plan.Step{Command: "exit 75"}
	code, stdout, stderr := run(t, &plan.Plan{Order: []plan.Item{{Step: &step}}}, Options{JSON: true})
	if code != 1 || !strings.Contains(stderr, "CI failed at: exit 75 (it exited 75)") || !strings.Contains(stdout, `"code": 75`) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// The nightly runs a check its done tasks share once; a push runs each named
// task's checks as its own.
func TestOnlyTheNightlySharesRuns(t *testing.T) {
	// Slashes, as sh reads a windows path's backslashes as escapes.
	count := filepath.ToSlash(filepath.Join(t.TempDir(), "runs"))
	command := "echo run >> " + count
	shared := func(id string) plan.Item {
		return plan.Item{Check: &plan.Check{
			CostedCheck: check.CostedCheck{Task: ledger.Task{ID: id}, Check: ledger.Check{Run: &command}},
			Action:      plan.Run,
		}}
	}
	p := &plan.Plan{Order: []plan.Item{shared("T-001"), shared("T-002")}}
	for _, c := range []struct {
		nightly bool
		runs    int
	}{{false, 2}, {true, 1}} {
		_ = os.Remove(count)
		code, stdout, _ := run(t, p, Options{Nightly: c.nightly})
		written, err := os.ReadFile(count)
		if err != nil {
			t.Fatal(err)
		}
		if runs := strings.Count(string(written), "run"); code != 0 || runs != c.runs {
			t.Errorf("nightly %v: exit %d, %d runs, want %d\n%s", c.nightly, code, runs, c.runs, stdout)
		}
	}
}
