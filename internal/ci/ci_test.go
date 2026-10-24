package ci

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/plan"
	privport "github.com/donvargax/itos/v8/internal/store/private/port"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func load(t *testing.T) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, file)
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

// failingWriter refuses every write, as a closed stdout does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout is closed") }

// A run stops, before anything runs, at what it cannot set or read: a ci.env
// key the environment cannot take, a range end it cannot hand on, the
// ledger's folder to name an unknown task against in a config with no
// ledger; and under --json at a verdict it cannot write.
func TestRunStopsAtWhatItCannotSetOrRead(t *testing.T) {
	t.Setenv("ITOS_FROM", "")
	t.Setenv("ITOS_TO", "")
	var stdout, stderr strings.Builder
	cfg := load(t)
	cfg.CI.Env = map[string]string{"A\x00B": "x"}
	if _, err := Run(cfg, &plan.Plan{}, Options{Stdout: &stdout, Stderr: &stderr}); err == nil {
		t.Error("a ci.env key with a NUL in it gave no error")
	}
	if _, err := Run(load(t), &plan.Plan{Ends: plan.Ends{From: "a\x00b"}}, Options{Stdout: &stdout, Stderr: &stderr}); err == nil {
		t.Error("a range end with a NUL in it gave no error")
	}
	// The error is the run's answer, so its code is 0, never a failing
	// step's 1.
	if code, err := Run(load(t), &plan.Plan{Unknown: []string{"T-9"}}, Options{Stdout: &stdout, Stderr: &stderr}); code != 0 || err == nil ||
		!strings.Contains(err.Error(), "ledger is missing") {
		t.Errorf("an unknown task with no ledger: exit %d, %v", code, err)
	}
	if _, err := Run(load(t), &plan.Plan{}, Options{JSON: true, Stdout: failingWriter{}, Stderr: &stderr}); err == nil {
		t.Error("a verdict it cannot write gave no error")
	}
}

// A run runs each named task's checks as its own, so a check two tasks share
// runs once for each.
func TestEachTaskRunsASharedCheck(t *testing.T) {
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
	code, stdout, _ := run(t, p, Options{})
	written, err := os.ReadFile(count)
	if err != nil {
		t.Fatal(err)
	}
	if runs := strings.Count(string(written), "run"); code != 0 || runs != 2 {
		t.Errorf("exit %d, %d runs, want 2\n%s", code, runs, stdout)
	}
}

// The nightly goes on past a failing step whatever ci.stop_at_first_failure
// says (its default, true, here), naming the first failure as "CI failed at"
// and each later one, a step's, the code proof's or a task check's, as "also
// failed at"; --json's failures lists each in run order, failed_at the
// first. A push's run, under the same setting, stops at the first (slice
// 133).
func TestTheNightlyGoesOnPastAFailingStep(t *testing.T) {
	failing := "exit 5"
	items := func() []plan.Item {
		first, passing, proof := plan.Step{Command: "exit 3"}, plan.Step{Command: "exit 0"}, plan.Step{Command: "exit 4", Proof: true}
		return []plan.Item{{Step: &first}, {Step: &passing}, {Step: &proof}, {Check: &plan.Check{
			CostedCheck: check.CostedCheck{Task: ledger.Task{ID: "T-002"}, Check: ledger.Check{Run: &failing}},
			Action:      plan.Run,
		}}}
	}
	failuresOf := func(stdout string) []Failure {
		var verdict struct {
			FailedAt Failure   `json:"failed_at"`
			Failures []Failure `json:"failures"`
		}
		if err := json.Unmarshal([]byte(stdout), &verdict); err != nil {
			t.Fatalf("%v: %q", err, stdout)
		}
		if verdict.FailedAt.Step == nil || *verdict.FailedAt.Step != "exit 3" {
			t.Errorf("failed_at is not the first failure: %q", stdout)
		}
		return verdict.Failures
	}
	code, stdout, stderr := run(t, &plan.Plan{Order: items()}, Options{JSON: true, Nightly: true})
	for _, want := range []string{
		"\nCI failed at: exit 3 (it exited 3)\n",
		"\nalso failed at the code proof: exit 4\n",
		"\nalso failed at T-002, its check: exit 5\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the nightly's stderr does not say %q: %q", want, stderr)
		}
	}
	failures := failuresOf(stdout)
	if code != 1 || len(failures) != 3 || failures[1].Problems == nil || failures[2].Task != "T-002" {
		t.Errorf("the nightly: exit %d, failures %+v", code, failures)
	}
	code, stdout, stderr = run(t, &plan.Plan{Order: items()}, Options{JSON: true})
	if failures := failuresOf(stdout); code != 1 || len(failures) != 1 || strings.Contains(stderr, "also failed at") {
		t.Errorf("a push's run: exit %d, failures %+v, stderr %q", code, failures, stderr)
	}
}

// fakeGit is a repository whose index is one tree, with an edit not staged.
type fakeGit struct{ unstaged []string }

func (fakeGit) IndexTree(context.Context) (string, error)         { return "tree", nil }
func (fakeGit) Head(context.Context) (string, error)              { return "head", nil }
func (fakeGit) IndexPaths(context.Context) ([]string, error)      { return nil, nil }
func (g fakeGit) UnstagedPaths(context.Context) ([]string, error) { return g.unstaged, nil }
func (fakeGit) Snapshot(context.Context, string) (evidence.Checkout, error) {
	return evidence.Checkout{Dir: "", Remove: func() error { return nil }}, nil
}

type fakeStore struct{ root string }

func (s fakeStore) Path(_ context.Context, name string) (string, error) {
	return filepath.Join(s.root, name), nil
}
func (fakeStore) Read(context.Context, string) ([]byte, bool, error)  { return nil, false, nil }
func (fakeStore) Write(context.Context, string, []byte) error         { return nil }
func (fakeStore) NotIgnored(context.Context, string) bool             { return false }
func (fakeStore) Hold(context.Context, string) (privport.Held, error) { return fakeHeld{}, nil }

type fakeHeld struct{}

func (fakeHeld) Release(context.Context) error { return nil }

// A step that opts in is looked up and recorded, and answers from its record
// the next run, which names its exit code as a failing step does; the run's
// JSON lists the evidence of each command looked up, and the paths not
// staged; a step that did not opt in, or a run with no engine, is as it was.
func TestAStepThatOptsInIsLookedUp(t *testing.T) {
	count := filepath.ToSlash(filepath.Join(t.TempDir(), "runs"))
	command := "echo ran >> " + count + "; exit 5"
	opted := plan.Step{Command: command, Evidence: &config.Evidence{}}
	plain := plan.Step{Command: "exit 0"}
	engine := func(store string) *evidence.Engine {
		return &evidence.Engine{Cfg: load(t), Git: fakeGit{unstaged: []string{"a"}}, Store: fakeStore{store}, Binding: evidence.ThisBinary(), Mode: "ci", CIEnv: map[string]string{}}
	}
	store := t.TempDir()
	p := &plan.Plan{Order: []plan.Item{{Step: &plain}, {Step: &opted}}}
	code, stdout, stderr := run(t, p, Options{JSON: true, Evidence: engine(store)})
	var report struct {
		Evidence []map[string]any `json:"evidence"`
		Unstaged *int             `json:"unstaged"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("%v: %s", err, stdout)
	}
	if code != 1 || len(report.Evidence) != 1 || report.Evidence[0]["command"] != command ||
		report.Evidence[0]["hit"] != false || report.Evidence[0]["recorded"] != true || report.Unstaged == nil || *report.Unstaged != 1 {
		t.Fatalf("first run: exit %d, evidence %v, unstaged %v\nstdout %s\nstderr %s", code, report.Evidence, report.Unstaged, stdout, stderr)
	}
	if !strings.Contains(stderr, "CI failed at: "+command+" (it exited 5)") || !strings.Contains(stderr, "1 changed path not staged") {
		t.Errorf("stderr %q", stderr)
	}
	code, stdout, stderr = run(t, p, Options{JSON: true, Evidence: engine(store)})
	report.Evidence = nil
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if code != 1 || report.Evidence[0]["hit"] != true || !strings.Contains(stderr, "(it exited 5)") || !strings.Contains(stderr, "cached evidence") {
		t.Errorf("second run: exit %d, evidence %v, stderr %q", code, report.Evidence, stderr)
	}
	runs, _ := os.ReadFile(count)
	if string(runs) != "ran\n" {
		t.Errorf("the step ran %q", runs)
	}
	// No engine: the step runs as it always did, and the JSON has no evidence.
	code, stdout, _ = run(t, &plan.Plan{Order: []plan.Item{{Step: &opted}}}, Options{JSON: true})
	if code != 1 || strings.Contains(stdout, `"evidence"`) || strings.Contains(stdout, `"unstaged"`) {
		t.Errorf("without an engine: exit %d, %s", code, stdout)
	}
	if runs, _ = os.ReadFile(count); string(runs) != "ran\nran\n" {
		t.Errorf("the step ran %q", runs)
	}
	// An engine that no step used leaves the JSON as it was.
	_, stdout, _ = run(t, &plan.Plan{Order: []plan.Item{{Step: &plain}}}, Options{JSON: true, Evidence: engine(store)})
	if strings.Contains(stdout, `"evidence"`) {
		t.Errorf("an engine nothing used: %s", stdout)
	}
	// An engine that cannot count the unstaged paths gives no count.
	broken := engine(store)
	broken.Git = noCount{fakeGit{}}
	_, stdout, _ = run(t, &plan.Plan{Order: []plan.Item{{Step: &opted}}}, Options{JSON: true, Evidence: broken})
	if !strings.Contains(stdout, `"evidence"`) || strings.Contains(stdout, `"unstaged"`) {
		t.Errorf("without a count: %s", stdout)
	}
}

// noCount cannot say what is unstaged.
type noCount struct{ fakeGit }

func (noCount) UnstagedPaths(context.Context) ([]string, error) { return nil, errors.New("git") }
