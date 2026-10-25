package shell

import (
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
