package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A --config the environment refuses (a NUL in it) stops the run before any
// command.
func TestApplyGlobalsRefusesAConfigTheEnvironmentRefuses(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	if _, _, err := applyGlobals(Globals{Config: "a\x00b"}, t.TempDir(), runtime.GOOS); err == nil {
		t.Error("a --config holding a NUL was set")
	}
}

// --root names the folder the command runs in, read from the run's folder
// when relative; one that is missing or no folder is a usage error naming
// it, before any command.
func TestApplyGlobalsRoot(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	dir := t.TempDir()
	writeFiles(t, map[string]string{filepath.Join(dir, "sub", "file"): "a file\n"})
	at, origin, err := applyGlobals(Globals{Root: "sub"}, dir, runtime.GOOS)
	if err != nil || !sameDir(at, filepath.Join(dir, "sub")) || origin != "" {
		t.Errorf("--root sub = %q, %q, %v; want the run's sub, no origin", at, origin, err)
	}
	for _, root := range []string{"gone", filepath.Join("sub", "file"), "a\x00b"} {
		_, _, err := applyGlobals(Globals{Root: root}, dir, runtime.GOOS)
		if ExitCode(err) != ExitUsage || !strings.HasPrefix(err.Error(), "--root "+root+": ") {
			t.Errorf("--root %q: %v, want a usage error naming it", root, err)
		}
	}
}

// From a subfolder of a repository whose config is at its top, the command
// runs at the top, and gives where the person stood; at the top it stays.
func TestApplyGlobalsMovesToTheTop(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	at, origin, err := applyGlobals(Globals{}, sub, runtime.GOOS)
	if err != nil || !sameDir(at, dir) || origin != filepath.Join("a", "b") {
		t.Errorf("from a/b = %q, %q, %v; want the top and a/b", at, origin, err)
	}
	if at, origin, err := applyGlobals(Globals{}, dir, runtime.GOOS); err != nil || at != dir || origin != "" {
		t.Errorf("at the top = %q, %q, %v; want it as given, no origin", at, origin, err)
	}
}

// A folder entered keeps the spelling of the one it was entered from when
// it is the same; another is given with its links resolved, but on windows,
// as it was entered.
func TestEntered(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symbolic link here: %v", err)
	}
	if got := entered("linux", link, real); got != link {
		t.Errorf("entered the same folder = %q, want %q", got, link)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if got := entered("linux", dir, link); got != resolved {
		t.Errorf("entered a link = %q, want %q", got, resolved)
	}
	if got := entered("windows", dir, link); got != link {
		t.Errorf("entered a link on windows = %q, want %q", got, link)
	}
	gone := filepath.Join(dir, "gone")
	if got := entered("linux", dir, gone); got != gone {
		t.Errorf("entered a missing folder = %q, want %q", got, gone)
	}
}
