package check

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// HEAD is pushed when a remote branch holds it, and not with no remote
// branch, or one behind it.
func TestPushed(t *testing.T) {
	history := []porttest.Commit{{Name: "c-a"}, {Name: "c-b", Parents: []string{"c-a"}}}
	for _, c := range []struct {
		remote string
		want   bool
	}{{"c-b", true}, {"c-a", false}, {"", false}} {
		named := map[string]string{"HEAD": "c-b"}
		if c.remote != "" {
			named["refs/remotes/origin/main"] = c.remote
		}
		if got := Pushed(&porttest.VCS{History: history, Named: named}); got != c.want {
			t.Errorf("Pushed with origin/main at %q = %v", c.remote, got)
		}
	}
}

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, file)
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

// work done runs every static check of a task, wherever it is written, and
// leaves the late ones to CI: without keep_written_order a static check below
// a late one is static, with it the check is late.
func TestStaticAndLateChecks(t *testing.T) {
	task := ledger.Task{ID: "T-1", DoneWhen: []ledger.Check{
		{Run: text("a"), Cost: "static"},
		{Run: text("b")},
		{Run: text("c"), Cost: "static"},
	}}
	names := func(checks []CostedCheck) string {
		var found []string
		for _, c := range checks {
			found = append(found, c.Check.Command())
		}
		return strings.Join(found, " ")
	}
	cfg := load(t, "version: 1\n")
	if static, late := names(StaticChecks(cfg, task)), names(LateChecks(cfg, task)); static != "a c" || late != "b" {
		t.Errorf("static %q, late %q", static, late)
	}
	cfg = load(t, "version: 1\nci: { steps: [\"true\"], cost: { keep_written_order: true } }\n")
	if static, late := names(StaticChecks(cfg, task)), names(LateChecks(cfg, task)); static != "a" || late != "b c" {
		t.Errorf("kept in order: static %q, late %q", static, late)
	}
}

// missing is a command sh does not find, exit 127.
const missing = "itos-no-such-command-bug-155"

// stalled outlasts its timeout and exits 3 on the SIGTERM that stops it, so
// only its timeout says it did not run to completion (on Windows, where the
// timeout kills it, its code is Kill's).
const stalled = "trap 'exit 3' TERM; sleep 2 & wait"

// A fails: check passes only on a command that ran to completion and exited
// non-zero: one sh does not find, one past its timeout and one that cannot
// start (a NUL byte in its argument) fail, a verbose run saying why, and a
// later fails: check reading the kept run reads why too; a run: check is
// told nothing.
func TestRunnerAFailsCheckThatDidNotRunFails(t *testing.T) {
	cfg := load(t, "version: 1\n")
	var log bytes.Buffer
	r := NewRunner(cfg, &log, &log)
	r.Verbose = true
	for name, c := range map[string]ledger.Check{
		"missing":    {Fails: text(missing)},
		"reused":     {Fails: text(missing)},
		"run:":       {Run: text(missing)},
		"timed out":  {Fails: text(stalled), Timeout: seconds(0.2)},
		"no start":   {Fails: text("exit 3\x00")},
		"ran, run:":  {Run: text("exit 3")},
		"ran, fails": {Fails: text("exit 4")},
	} {
		want := Fail
		if name == "ran, fails" {
			want = Pass
		}
		if got := r.Run(c); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
	for line, n := range map[string]int{
		"  did not run: command not found (exit 127)\n": 2,
		"  did not run: timed out\n":                    1,
		"  did not run: could not start: ":              1,
		"  did not run: ":                               4,
	} {
		if got := strings.Count(log.String(), line); got != n {
			t.Errorf("%q printed %d times, want %d:\n%s", line, got, n, log.String())
		}
	}
	var quiet bytes.Buffer
	if got := NewRunner(cfg, &quiet, &quiet).Run(ledger.Check{Fails: text(missing)}); got != Fail || quiet.Len() != 0 {
		t.Errorf("a quiet run: %s, printed %q", got, quiet.String())
	}
}

// A run: check passes on exit 0 alone; a fails: check on a non-zero exit of
// a command that ran to completion.
func TestVerdict(t *testing.T) {
	run, fails := ledger.Check{Run: text("x")}, ledger.Check{Fails: text("x")}
	for _, c := range []struct {
		check  ledger.Check
		ok     bool
		notRun string
		want   Result
	}{
		{run, true, "", Pass},
		{run, false, "", Fail},
		{run, false, "timed out", Fail},
		{run, true, "timed out", Pass},
		{fails, false, "", Pass},
		{fails, true, "", Fail},
		{fails, false, "command not found (exit 127)", Fail},
	} {
		if got := verdict(c.check, c.ok, c.notRun); got != c.want {
			t.Errorf("%s, ok %v, not run %q: got %s, want %s", c.check.Command(), c.ok, c.notRun, got, c.want)
		}
	}
}

// A captured fails: check that did not run to completion fails and says
// why; one that exited non-zero passes and says nothing.
func TestRunCapturedAFailsCheckThatDidNotRun(t *testing.T) {
	cfg := load(t, "version: 1\n")
	for command, want := range map[string]string{
		missing:      "command not found (exit 127)",
		"exit 3":     "",
		"exit 3\x00": "could not start: ",
	} {
		run, err := RunCaptured(cfg, ledger.Check{Fails: text(command)}, NoCap, nil)
		if err != nil {
			t.Fatal(err)
		}
		if (run.Result == Pass) != (want == "") || !strings.HasPrefix(run.NotRun, want) || (want == "") != (run.NotRun == "") {
			t.Errorf("%q: %+v, want not run %q", command, run, want)
		}
	}
}
