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

// The code proof of a run (slice 106, issue #28): exit 0 passes, and the
// check's --json is what a refusal is read from, so a survivor fails the run
// naming its rule and message, on stderr and as failed_at's problems; a
// check that cannot run fails it too, never a pass.
func TestTheCodeProofFailsTheRunByItsProblems(t *testing.T) {
	refused := `{"schema":1,"ok":false,"problems":[{"rule":"mutation.survived","message":"a mutant of app.Total survived","fix":"test it"}]}`
	for _, c := range []struct {
		name    string
		command string
		want    []string
		json    []string
	}{
		{"a survivor", `printf '%s\n' '` + refused + `'; exit 1`,
			[]string{"mutation.survived: a mutant of app.Total survived", "fix: test it"},
			[]string{`"rule": "mutation.survived"`, `"message": "a mutant of app.Total survived"`, `"fix": "test it"`}},
		{"a check that cannot start", "itos-no-such-provider mutation check",
			[]string{"ci-code-proof: the code proof could not run", "itos-no-such-provider mutation check"},
			[]string{`"rule": "ci-code-proof"`}},
		{"a check that cannot run", "exit 3", []string{"ci-code-proof: the code proof could not run", "exited 3"},
			[]string{`"rule": "ci-code-proof"`}},
		{"a refusal naming no problem", `printf '%s\n' '{"schema":1,"ok":false}'; exit 1`,
			[]string{"ci-code-proof: the code proof refused and named no problem"},
			[]string{`"rule": "ci-code-proof"`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			step := plan.Step{Command: c.command, Proof: true}
			code, stdout, stderr := run(t, &plan.Plan{Order: []plan.Item{{Step: &step}}}, Options{JSON: true})
			if code != 1 || !strings.Contains(stderr, "CI failed at the code proof: "+c.command) {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			for _, want := range c.want {
				// On stderr as the line a person reads, in failed_at's
				// problems as the machine contract.
				if !strings.Contains(stderr, want) {
					t.Errorf("%q is not on stderr: %q", want, stderr)
				}
			}
			for _, want := range c.json {
				if !strings.Contains(stdout, want) {
					t.Errorf("%q is not in stdout: %q", want, stdout)
				}
			}
		})
	}
	step := plan.Step{Command: "exit 0", Proof: true}
	if code, stdout, stderr := run(t, &plan.Plan{Order: []plan.Item{{Step: &step}}}, Options{}); code != 0 ||
		!strings.Contains(stdout, "CI passed") || strings.Contains(stderr, "code proof") {
		t.Errorf("a passing proof: exit %d, stdout %q, stderr %q", code, stdout, stderr)
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
