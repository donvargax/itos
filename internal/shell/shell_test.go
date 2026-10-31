package shell

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// A command runs in the folder its options name, else in the folder itos
// runs in, the config's.
func TestRunInTheFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh is Git for Windows', which names folders its own way")
	}
	folder := func() string {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return dir
	}
	dir, other := folder(), folder()
	t.Chdir(folder())
	cfg := &config.Loaded{VCS: &porttest.VCS{Root: dir}}
	cfg.Shell = []string{"sh", "-c"}
	for given, want := range map[string]string{"": dir, other: other} {
		var out strings.Builder
		if r := Run(cfg, "pwd -P", Options{Stdout: &out, Dir: given}); !r.OK() || out.String() != want+"\n" {
			t.Errorf("Dir %q: %+v, ran in %q, want %q", given, r, out.String(), want)
		}
	}
}

// A command that exits with a code of its own, and is stopped by no signal,
// has that code and no signal.
func TestRunAnExitCodeIsNoSignal(t *testing.T) {
	cfg := &config.Loaded{VCS: &porttest.VCS{Root: t.TempDir()}}
	cfg.Shell = []string{"sh", "-c"}
	if r := Run(cfg, "exit 3", Options{}); r.Code != 3 || r.Signal != 0 || r.TimedOut || r.Err != nil || r.Status() != "3" {
		t.Errorf("exit 3: %+v, status %q, want code 3 and no signal", r, r.Status())
	}
}

// Why a command did not run to completion: it could not start, timed out,
// did not exit by itself, or sh could not run it (126, 127); a command that
// exited, 0 or not, ran. A command sh does not find exits 127, and one with a
// NUL byte cannot start, on every OS.
func TestNotRun(t *testing.T) {
	for want, r := range map[string]Result{
		"could not start: no shell":         {Code: -1, Err: errors.New("no shell")},
		"timed out":                         {Code: 3, TimedOut: true},
		"stopped before it exited":          {Code: -1},
		"command not executable (exit 126)": {Code: 126},
		"command not found (exit 127)":      {Code: 127},
		"":                                  {Code: 1},
	} {
		if got := r.NotRun(); got != want {
			t.Errorf("%+v: got %q, want %q", r, got, want)
		}
	}
	if got := NotRun(0, false); got != "" {
		t.Errorf("exit 0: got %q", got)
	}
	cfg := &config.Loaded{VCS: &porttest.VCS{Root: t.TempDir()}}
	cfg.Shell = []string{"sh", "-c"}
	if r := Run(cfg, "itos-no-such-command-bug-155", Options{}); r.Code != 127 || r.NotRun() != "command not found (exit 127)" {
		t.Errorf("a missing command: %+v", r)
	}
	if r := Run(cfg, "true\x00", Options{}); r.Err == nil || r.Code != -1 || !strings.HasPrefix(r.NotRun(), "could not start: ") {
		t.Errorf("a NUL byte: %+v", r)
	}
}
