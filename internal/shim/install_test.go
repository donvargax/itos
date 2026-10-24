package shim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// A folder's place on the PATH, and the real git's, are each the first
// folder that is it: a folder named twice keeps its first place. With no
// git on the PATH there is no real git's place, even for the folder the
// test stands in.
func TestPathStanding(t *testing.T) {
	real, err := git.Real(runtime.GOOS)
	if err != nil {
		t.Skip("no git on the PATH")
	}
	gitDir, link := filepath.Dir(real), t.TempDir()
	t.Setenv("PATH", strings.Join([]string{gitDir, link, gitDir, link}, string(os.PathListSeparator)))
	if at, realGit, realAt := PathStanding(link, runtime.GOOS); at != 1 || realGit != real || realAt != 0 {
		t.Errorf("PathStanding = %d, %q, %d", at, realGit, realAt)
	}
	t.Chdir(link)
	t.Setenv("PATH", link)
	if at, realGit, realAt := PathStanding(link, runtime.GOOS); at != 0 || realGit != "" || realAt != -1 {
		t.Errorf("with no git: PathStanding = %d, %q, %d", at, realGit, realAt)
	}
}

// An empty, a relative or a missing folder on the PATH is never the link's,
// and keeps its place in the count.
func TestPathStandingSkipsWhatIsNoFolder(t *testing.T) {
	link := t.TempDir()
	t.Chdir(link)
	missing := filepath.Join(link, "missing")
	t.Setenv("PATH", strings.Join([]string{"", ".", missing, link}, string(os.PathListSeparator)))
	if at, _, _ := PathStanding(link, runtime.GOOS); at != 3 {
		t.Errorf("PathStanding = %d, want 3", at)
	}
	if sameFolder(link, missing) {
		t.Errorf("sameFolder(%s, %s) = true", link, missing)
	}
}

// The shim is git, and git.exe on windows, in the folder named, made
// absolute, or else the one holding the itos running.
func TestName(t *testing.T) {
	for goos, want := range map[string]string{"linux": "git", "darwin": "git", "windows": "git.exe"} {
		if got := Name(goos); got != want {
			t.Errorf("Name(%s) = %s", goos, got)
		}
		if _, link, err := Place(os.Executable, "bin", goos); err != nil || filepath.Base(link) != want || !filepath.IsAbs(link) {
			t.Errorf("Place(bin, %s) = %s, %v", goos, link, err)
		}
	}
	self, link, err := Place(os.Executable, "", "linux")
	if err != nil || link != filepath.Join(filepath.Dir(self), "git") {
		t.Errorf(`Place("", linux) = %s, %s, %v`, self, link, err)
	}
}

// An itos that cannot tell its own binary has no place for the link.
func TestPlaceWithoutTheBinary(t *testing.T) {
	lost := func() (string, error) { return "", errors.New("gone") }
	if _, _, err := Place(lost, "bin", "linux"); err == nil || err.Error() != "cannot tell the itos binary running: gone" {
		t.Errorf("Place = %v", err)
	}
}

// A folder named from a current folder since removed cannot be made absolute.
func TestPlaceFromARemovedFolder(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		// Where the current folder cannot be removed (windows), there is
		// no removed one to stand in.
		t.Skipf("cannot remove the current folder here: %v", err)
	}
	if _, err := os.Getwd(); err == nil {
		// Where the OS still names a removed current folder, it is made
		// absolute from that name.
		t.Skip("the OS still names the removed current folder")
	}
	if _, _, err := Place(os.Executable, "bin", "linux"); err == nil {
		t.Error("Place from a removed folder made it absolute")
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
		if err := MakeLink(self, link, false, goos); !errors.As(err, &failed) || failed.Op != op {
			t.Errorf("MakeLink on %s: %v; want a %s error", goos, err, op)
		}
	}
}

// Told to replace the git there, MakeLink removes it and links self in its
// folder, which it makes when missing; with nothing there to replace, the
// removal is the error, and a file where the folder would be is one too.
func TestMakeLinkReplaces(t *testing.T) {
	dir := t.TempDir()
	self, link := filepath.Join(dir, "itos"), filepath.Join(dir, "bin", "git")
	writeFile(t, self, "itos")
	if err := MakeLink(self, link, true, runtime.GOOS); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("replacing nothing: %v", err)
	}
	writeFile(t, link, "git")
	if err := MakeLink(self, link, true, runtime.GOOS); err != nil {
		t.Fatal(err)
	}
	if !sameFile(t, self, link) {
		t.Errorf("%s does not link %s", link, self)
	}
	if err := MakeLink(self, filepath.Join(self, "git"), false, runtime.GOOS); err == nil {
		t.Errorf("a link under the file %s was made", self)
	}
}

// The git at a path is this itos when it is the binary running, another
// itos when it is a symbolic link to a file named itos (itos.exe, in any
// case), even one that is no longer there, and neither when it is any other
// file.
func TestLinksItos(t *testing.T) {
	running, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if self, other := LinksItos(running); !self || other {
		t.Errorf("LinksItos(the binary running) = %v, %v", self, other)
	}
	dir := t.TempDir()
	plain := filepath.Join(dir, "git")
	writeFile(t, plain, "git")
	if self, other := LinksItos(plain); self || other {
		t.Errorf("LinksItos(a file) = %v, %v", self, other)
	}
	for target, want := range map[string]bool{"itos": true, "ITOS.exe": true, "tool": false} {
		writeFile(t, filepath.Join(dir, target), target)
		link := filepath.Join(dir, "git-"+target)
		if err := os.Symlink(filepath.Join(dir, target), link); err != nil {
			// Where symbolic links need a privilege (windows), there is
			// none to read.
			t.Skipf("cannot make a symbolic link here: %v", err)
		}
		if self, other := LinksItos(link); self || other != want {
			t.Errorf("LinksItos(a link to %s) = %v, %v", target, self, other)
		}
	}
	dangling := filepath.Join(dir, "git-dangling")
	if err := os.Symlink(filepath.Join(dir, "moved", "itos"), dangling); err != nil {
		t.Fatal(err)
	}
	if self, other := LinksItos(dangling); self || !other {
		t.Errorf("LinksItos(a link to an itos since moved) = %v, %v", self, other)
	}
}

// A hard link to an itos build or a copy of it is another itos, as the git
// adapter's IsItos reads its build information; a hard link to or a copy of
// a Go program that is not itos, the test binary here, is not.
func TestLinksItosBuilt(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go to build an itos with")
	}
	dir := t.TempDir()
	build := filepath.Join(dir, "itos-build")
	if out, err := exec.Command(gobin, "build", "-o", build, "github.com/donvargax/itos/v8/cmd/itos").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	running, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, from string
		want       bool
	}{
		{"an itos build", build, true},
		{"a Go program that is not itos", running, false},
	} {
		text, err := os.ReadFile(c.from)
		if err != nil {
			t.Fatal(err)
		}
		copied := filepath.Join(dir, "copy-of "+c.name)
		if err := os.WriteFile(copied, text, 0o755); err != nil {
			t.Fatal(err)
		}
		hard := filepath.Join(dir, "hard link to "+c.name)
		if err := os.Link(copied, hard); err != nil {
			t.Fatalf("cannot make a hard link here: %v", err)
		}
		for _, link := range []string{copied, hard} {
			if self, other := LinksItos(link); self || other != c.want {
				t.Errorf("LinksItos(%s) = %v, %v; want other %v", filepath.Base(link), self, other, c.want)
			}
		}
	}
}

// Where the link's folder stands on the PATH, against the real git's.
func TestStandingLine(t *testing.T) {
	dir, realGit := filepath.Join("home", "bin"), filepath.Join("usr", "bin", "git")
	realDir := filepath.Dir(realGit)
	for _, c := range []struct {
		at      int
		realGit string
		realAt  int
		want    string
	}{
		{-1, realGit, 0, dir + " is not on the PATH: put it before " + realDir + ", the real git's folder, for git to run the shim."},
		{-1, "", -1, dir + " is not on the PATH, and the PATH has no other git for the shim to run."},
		{0, "", -1, dir + " is on the PATH, but the PATH has no other git for the shim to run: install git."},
		{0, realGit, 1, dir + " comes before the real git (" + realGit + ") on the PATH: git runs the shim, and in a " +
			"repository itos manages git commit and git push are itos's."},
		{1, realGit, 0, dir + " comes after the real git (" + realGit + ") on the PATH: put it first for git to run the shim."},
	} {
		if got := StandingLine(dir, c.at, c.realGit, c.realAt); got != c.want {
			t.Errorf("StandingLine(%d, %q, %d) = %q, want %q", c.at, c.realGit, c.realAt, got, c.want)
		}
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	x, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(x, y)
}
