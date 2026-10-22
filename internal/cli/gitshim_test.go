package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// A folder's place on the PATH, and the real git's, are each the first
// folder that is it: a folder named twice keeps its first place. With no
// git on the PATH there is no real git's place, even for the folder the
// test stands in.
func TestPathStanding(t *testing.T) {
	real, err := git.Real()
	if err != nil {
		t.Skip("no git on the PATH")
	}
	gitDir, link := filepath.Dir(real), t.TempDir()
	t.Setenv("PATH", strings.Join([]string{gitDir, link, gitDir, link}, string(os.PathListSeparator)))
	if at, realGit, realAt := pathStanding(link); at != 1 || realGit != real || realAt != 0 {
		t.Errorf("pathStanding = %d, %q, %d", at, realGit, realAt)
	}
	t.Chdir(link)
	t.Setenv("PATH", link)
	if at, realGit, realAt := pathStanding(link); at != 0 || realGit != "" || realAt != -1 {
		t.Errorf("with no git: pathStanding = %d, %q, %d", at, realGit, realAt)
	}
}

// The shim is git, and git.exe on windows.
func TestShimName(t *testing.T) {
	for goos, want := range map[string]string{"linux": "git", "darwin": "git", "windows": "git.exe"} {
		if got := shimName(goos); got != want {
			t.Errorf("shimName(%s) = %s", goos, got)
		}
		if _, link, err := shimPlace("bin", goos); err != nil || filepath.Base(link) != want || !filepath.IsAbs(link) {
			t.Errorf("shimPlace(bin, %s) = %s, %v", goos, link, err)
		}
	}
}

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

// A symbolic link that cannot be made is the error, except on windows, where
// a hard link is made in its place: here, where the link is already there,
// that fails too, with its own error.
func TestMakeLinkFallsBackToAHardLinkOnWindows(t *testing.T) {
	dir := t.TempDir()
	self, link := filepath.Join(dir, "itos"), filepath.Join(dir, "git")
	writeFile(t, self, "itos")
	writeFile(t, link, "git")
	for goos, op := range map[string]string{"linux": "symlink", "darwin": "symlink", "windows": "link"} {
		var failed *os.LinkError
		if err := makeLink(self, link, false, goos); !errors.As(err, &failed) || failed.Op != op {
			t.Errorf("makeLink on %s: %v; want a %s error", goos, err, op)
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
