package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// Uninstalling where there is no shim removes nothing, naming the link by
// Out's OS.
func TestUninstallShimWithNoneThere(t *testing.T) {
	dir := t.TempDir()
	for goos, name := range map[string]string{"linux": "git", "windows": "git.exe"} {
		var stdout strings.Builder
		code, err := uninstallShim(dir, Out{Stdout: &stdout, OS: goos})
		if want := "No git shim at " + filepath.Join(dir, name) + ": nothing to remove.\n"; code != 0 || err != nil || stdout.String() != want {
			t.Errorf("on %s: exit %d, %v, %q", goos, code, err, stdout.String())
		}
	}
}

// An extension's command is the file's name after itos-, without its
// extension on windows alone; a file without the prefix names none.
func TestExtensionName(t *testing.T) {
	for _, c := range []struct {
		file, goos, want string
		ok               bool
	}{
		{"itos-hello", "linux", "hello", true},
		{"itos-hello.sh", "darwin", "hello.sh", true},
		{"itos-hello.exe", "linux", "hello.exe", true},
		{"itos-hello.exe", "windows", "hello", true},
		{"itos-hello", "windows", "hello", true},
		{"hello.exe", "windows", "hello", false},
		{"hello", "linux", "hello", false},
	} {
		if got, ok := extensionName(c.file, c.goos); got != c.want || ok != c.ok {
			t.Errorf("extensionName(%s, %s) = %q, %v", c.file, c.goos, got, ok)
		}
	}
}
