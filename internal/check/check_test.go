package check

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/ledger"
)

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func text(s string) *string { return &s }

func seconds(f float64) *float64 { return &f }

// Two checks are one when their commands, whitespace collapsed, and their
// timeouts are the same; run: or fails: is not part of it.
func TestKey(t *testing.T) {
	cfg := load(t, "version: 1\n")
	a := ledger.Check{Run: text("sh  -c 'x'\n")}
	b := ledger.Check{Fails: text(" sh -c 'x'")}
	written := ledger.Check{Run: text("sh -c 'x'"), Timeout: seconds(600)}
	own := ledger.Check{Run: text("sh -c 'x'"), Timeout: seconds(30)}
	if Key(cfg, a) != Key(cfg, b) || Key(cfg, a) != Key(cfg, written) {
		t.Errorf("same check, other keys: %q %q %q", Key(cfg, a), Key(cfg, b), Key(cfg, written))
	}
	if Key(cfg, a) == Key(cfg, own) {
		t.Errorf("another timeout, same key: %q", Key(cfg, own))
	}
}

// A Runner runs a check once, every later one reading its exit status its
// own way, and says so on the verbose command line.
func TestRunnerRunsEachCheckOnce(t *testing.T) {
	cfg := load(t, "version: 1\n")
	// Slashes, as sh reads a windows path's backslashes as escapes.
	count := filepath.ToSlash(filepath.Join(t.TempDir(), "runs"))
	command := "echo ran >> " + count + "; exit 3"
	var stdout, stderr bytes.Buffer
	r := NewRunner(cfg, &stdout, &stderr)
	r.Verbose = true
	if got := r.Run(ledger.Check{Run: text(command)}); got != Fail {
		t.Errorf("run: got %s", got)
	}
	if got := r.Run(ledger.Check{Fails: text(command)}); got != Pass {
		t.Errorf("fails: got %s", got)
	}
	runs, _ := os.ReadFile(count)
	if string(runs) != "ran\n" {
		t.Errorf("ran %q", runs)
	}
	want := "  $ " + command + "\n  $ " + command + "   (must fail)   (ran above; its exit status reused)\n"
	if stdout.String() != want {
		t.Errorf("printed %q, want %q", stdout.String(), want)
	}
}

// A check past its timeout fails, and a command the shell cannot start
// fails rather than stopping the run.
func TestRunnerFailures(t *testing.T) {
	cfg := load(t, "version: 1\n")
	r := NewRunner(cfg, nil, nil)
	if got := r.Run(ledger.Check{Run: text("sleep 5"), Timeout: seconds(0.2)}); got != Fail {
		t.Errorf("timeout: got %s", got)
	}
	missing := load(t, "version: 1\nshell: [no-such-shell-itos, -c]\n")
	if got := NewRunner(missing, nil, nil).Run(ledger.Check{Run: text("true")}); got != Fail {
		t.Errorf("missing shell: got %s", got)
	}
}

// The cost classes: a check's own cost:, else ci.cost.static (hooks.bin read
// as itos), else late; with keep_written_order a static check below a late
// one is late, and the hook's checks stop at the first late one.
func TestCostedChecks(t *testing.T) {
	cfg := load(t, "version: 1\nci: { steps: [\"true\"], cost: { static: [\"^itos config check$\"], keep_written_order: true } }\nhooks: { bin: tools/bin/itos }\n")
	task := ledger.Task{ID: "T-1", DoneWhen: []ledger.Check{
		{Run: text("tools/bin/itos config check")},
		{Run: text("sh -c 'x'"), Cost: "static"},
		{Run: text("go test ./...")},
		{Run: text("itos config check")},
	}}
	want := []Costed{{Static, FromPattern}, {Static, FromExplicit}, {Late, FromDefault}, {Late, FromOrder}}
	for i, c := range CostedChecks(cfg, task) {
		if c.Costed != want[i] || c.Index != i {
			t.Errorf("check %d: got %+v, want %+v", i, c.Costed, want[i])
		}
	}
	if before := ChecksBeforeLate(cfg, task); len(before) != 2 {
		t.Errorf("before the first late one: %d checks", len(before))
	}
}

// A captured run keeps stdout and stderr together in one file, and a check
// past its capped timeout fails, saying the cap set it, even one that must
// fail.
func TestRunCaptured(t *testing.T) {
	cfg := load(t, "version: 1\n")
	run, err := RunCaptured(cfg, ledger.Check{Run: text("echo out; echo err >&2; exit 3")}, NoCap, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != Fail || run.Code != 3 || run.Output != "out\nerr\n" || run.Capped || run.TimedOut {
		t.Errorf("failing check: %+v", run)
	}
	run, err = RunCaptured(cfg, ledger.Check{Fails: text("sleep 5"), Timeout: seconds(30)}, 0.2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != Fail || !run.TimedOut || !run.Capped || run.Seconds != 0.2 {
		t.Errorf("capped check: %+v", run)
	}
	run, err = RunCaptured(cfg, ledger.Check{Run: text(`test "$X" = y`)}, NoCap, []string{"X=y", "PATH=" + os.Getenv("PATH")})
	if err != nil {
		t.Fatal(err)
	}
	if run.Result != Pass || run.Capped {
		t.Errorf("its own environment: %+v", run)
	}
}
