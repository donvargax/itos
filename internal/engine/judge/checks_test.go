package judge

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// command is a check's command text.
func command(text string) *string { return &text }

// What went wrong with a failed check, in a few words: past the hook's cap,
// past its own timeout, passing when it must fail, a fails: check that did
// not run to completion, stopped, or its exit code.
func TestCheckFailure(t *testing.T) {
	run, fails := ledger.Check{Run: command("go test")}, ledger.Check{Fails: command("go vet")}
	for want, c := range map[string]struct {
		check ledger.Check
		run   check.Captured
	}{
		"`go test` ran past hooks.commit_msg.check_timeout (1.5s)": {run, check.Captured{TimedOut: true, Capped: true, Seconds: 1.5}},
		"`go test` ran past its timeout (30s)":                     {run, check.Captured{TimedOut: true, Seconds: 30}},
		"`go vet` passed, and must fail":                           {fails, check.Captured{}},
		"`go test` was stopped":                                    {run, check.Captured{Code: -1}},
		"`go vet` exited 2":                                        {fails, check.Captured{Code: 2}},
		"`go vet` did not run: command not found (exit 127)":       {fails, check.Captured{Code: 127, NotRun: "command not found (exit 127)"}},
		"`go test` exited 127":                                     {run, check.Captured{Code: 127, NotRun: "command not found (exit 127)"}},
		"`go test` exited 0":                                       {run, check.Captured{}},
	} {
		if got := checkFailure(c.check, c.run); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// A task's checks run up to its first late one, an after: push check passed
// over, and the first that fails is printed under the task's title, its
// output ended with a newline, and named in a few words.
func TestFirstFailure(t *testing.T) {
	cfg := configOver(t, &vcsfake.VCS{}, rules)
	task := ledger.Task{ID: "T-1", Title: "Do it", DoneWhen: []ledger.Check{
		{Run: command("exit 9"), After: "push", Cost: "static"},
		{Run: command("exit 0"), Cost: "static"},
		{Run: command("printf out; exit 3"), Cost: "static"},
		{Run: command("exit 4"), Cost: "static"},
	}}
	var stderr strings.Builder
	why, err := FirstFailure(cfg, task, check.NoCap, nil, &stderr)
	if err != nil || why != "`printf out; exit 3` exited 3" {
		t.Errorf("got %q, %v", why, err)
	}
	if want := "T-1 Do it\n  $ printf out; exit 3\nout\n"; stderr.String() != want {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}
	task.DoneWhen = []ledger.Check{{Fails: command("echo yes"), Cost: "static"}, {Run: command("exit 5")}}
	stderr.Reset()
	why, err = FirstFailure(cfg, task, check.NoCap, nil, &stderr)
	if err != nil || why != "`echo yes` passed, and must fail" {
		t.Errorf("got %q, %v", why, err)
	}
	if want := "T-1 Do it\n  $ echo yes   (must fail)\nyes\n"; stderr.String() != want {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}
	task.DoneWhen = []ledger.Check{{Run: command("exit 0"), Cost: "static"}, {Run: command("exit 5")}}
	if why, err := FirstFailure(cfg, task, check.NoCap, nil, &stderr); why != "" || err != nil {
		t.Errorf("a late check ran: %q, %v", why, err)
	}
	// A check's output goes to a file in a temporary folder: with the folder
	// temporary files go to missing, as every OS reports alike, it cannot run.
	// TMPDIR names it on unix, TMP and TEMP on Windows.
	missing := filepath.Join(t.TempDir(), "missing")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	task.DoneWhen = []ledger.Check{{Run: command("exit 0"), Cost: "static"}}
	if _, err := FirstFailure(cfg, task, check.NoCap, nil, &stderr); err == nil {
		t.Error("a check whose output has nowhere to go gave no error")
	}
	task.Err = errors.New("a check has no command")
	if _, err := FirstFailure(cfg, task, check.NoCap, nil, &stderr); !errors.Is(err, task.Err) {
		t.Errorf("a task whose checks cannot run: %v", err)
	}
}

// work done runs every static check of a task in written order, one below a
// late check included, the late one never, an after: push check passed over,
// and describes the first that fails: its task, command, words, output tail,
// exit code and whether its timeout stopped it.
func TestStaticFailure(t *testing.T) {
	cfg := configOver(t, &vcsfake.VCS{}, rules)
	task := ledger.Task{ID: "T-1", Title: "Do it", DoneWhen: []ledger.Check{
		{Run: command("exit 9"), After: "push", Cost: "static"},
		{Run: command("exit 8"), Cost: "late"},
		{Run: command("exit 0"), Cost: "static"},
		{Run: command("printf out; exit 3"), Cost: "static"},
		{Run: command("exit 4"), Cost: "static"},
	}}
	var stderr strings.Builder
	got, err := StaticFailure(cfg, task, nil, &stderr)
	want := &Failure{Task: "T-1", Command: "printf out; exit 3", Why: "`printf out; exit 3` exited 3", Output: "out", Code: 3}
	if err != nil || got == nil || *got != *want {
		t.Errorf("got %+v, %v, want %+v", got, err, want)
	}
	if want := "T-1 Do it\n  $ printf out; exit 3\nout\n"; stderr.String() != want {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}

	// Under no cap: a check runs to its own timeout, and one past it is a
	// timeout whatever its code.
	one := 1.0
	task.DoneWhen = []ledger.Check{{Run: command("sleep 5"), Cost: "static", Timeout: &one}}
	got, err = StaticFailure(cfg, task, nil, &stderr)
	if err != nil || got == nil || !got.TimedOut || got.Why != "`sleep 5` ran past its timeout (1s)" {
		t.Errorf("a timeout: %+v, %v", got, err)
	}

	// The output kept is the tail a record of evidence keeps.
	long := `i=0; while [ $i -lt 4000 ]; do printf 0123456789; i=$((i+1)); done; echo end; exit 1`
	task.DoneWhen = []ledger.Check{{Run: command(long), Cost: "static"}}
	got, err = StaticFailure(cfg, task, nil, &stderr)
	if err != nil || got == nil || len(got.Output) != evidence.OutputTail || !strings.HasSuffix(got.Output, "789end\n") {
		t.Errorf("a long output: %v, %v", got, err)
	}

	task.DoneWhen = []ledger.Check{{Run: command("exit 0"), Cost: "static"}, {Run: command("exit 5")}}
	if got, err := StaticFailure(cfg, task, nil, &stderr); got != nil || err != nil {
		t.Errorf("passing static checks and a late one: %+v, %v", got, err)
	}
	task.Err = errors.New("a check has no command")
	if _, err := StaticFailure(cfg, task, nil, &stderr); !errors.Is(err, task.Err) {
		t.Errorf("a task whose checks cannot run: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	task.Err = nil
	task.DoneWhen = []ledger.Check{{Run: command("exit 0"), Cost: "static"}}
	if _, err := StaticFailure(cfg, task, nil, &stderr); err == nil {
		t.Error("a check whose output has nowhere to go gave no error")
	}
}
