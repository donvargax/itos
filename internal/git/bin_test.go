package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The real git is the first git on the PATH that is not this binary: a link
// to it is skipped, a relative folder is not searched, and with no other git
// there is none. ITOS_GIT names it over the PATH, unless it names this
// binary.
func TestReal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the gits here are shell scripts and symbolic links")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shim, other := t.TempDir(), t.TempDir()
	link := filepath.Join(shim, "git")
	if err := os.Symlink(self, link); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(other, "git")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv(EnvGit, "")

	t.Setenv("PATH", "relative"+sep+shim+sep+other)
	if got, err := Real(); err != nil || got != real {
		t.Errorf("Real = %q, %v; want %q", got, err, real)
	}
	if !IsSelf(link) || IsSelf(real) {
		t.Errorf("IsSelf(link) = %v, IsSelf(real) = %v", IsSelf(link), IsSelf(real))
	}
	if got := Bin(); got != real {
		t.Errorf("Bin = %q, want %q", got, real)
	}

	t.Setenv("PATH", shim)
	if _, err := Real(); !errors.Is(err, ErrNoGit) {
		t.Errorf("with only the link, Real's error is %v", err)
	}
	if got := Bin(); got != "git" {
		t.Errorf("with only the link, Bin = %q, want git", got)
	}

	t.Setenv(EnvGit, real)
	if got := Bin(); got != real {
		t.Errorf("with ITOS_GIT, Bin = %q, want %q", got, real)
	}
	t.Setenv(EnvGit, link)
	if got := Inherited(); got != "" {
		t.Errorf("ITOS_GIT naming this binary is inherited: %q", got)
	}
}

// exe is a program's file name: name, name.exe on windows.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// The real git passes over any itos, not only this binary (bug 45): a
// symbolic link to a file named itos, whatever it holds, and a hard link to
// or a copy of an itos build, recognised by its build information; a Go
// program that is not itos is a git like any other. ITOS_GIT naming another
// itos is not inherited.
func TestRealPassesOverAnotherItos(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go to build an itos with")
	}
	root := t.TempDir()
	dir := func(name string) string {
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}
	build := filepath.Join(dir("build"), exe("itos"))
	if out, err := exec.Command(gobin, "build", "-o", build, "github.com/donvargax/itos/v5/cmd/itos").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	write := func(p string, text []byte) {
		if err := os.WriteFile(p, text, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	real := filepath.Join(dir("real"), exe("git"))
	write(real, []byte("#!/bin/sh\n"))
	text, err := os.ReadFile(build)
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(dir("copy"), exe("git"))
	write(copied, text)
	t.Setenv(EnvGit, "")
	t.Setenv("PATHEXT", ".exe")
	sep := string(os.PathListSeparator)
	real1 := func(t *testing.T, path string, want string) {
		t.Helper()
		t.Setenv("PATH", path)
		if got, err := Real(); err != nil || got != want {
			t.Errorf("Real = %q, %v; want %q", got, err, want)
		}
	}

	t.Run("a copy of an itos build", func(t *testing.T) {
		if !IsItos(copied) {
			t.Errorf("IsItos(%s) is false", copied)
		}
		real1(t, filepath.Dir(copied)+sep+filepath.Dir(real), real)
	})
	t.Run("a hard link to an itos build", func(t *testing.T) {
		link := filepath.Join(dir("hard"), exe("git"))
		if err := os.Link(build, link); err != nil {
			t.Skipf("no hard links here: %v", err)
		}
		real1(t, filepath.Dir(link)+sep+filepath.Dir(real), real)
	})
	t.Run("symbolic links to a file named itos", func(t *testing.T) {
		script := filepath.Join(dir("script"), exe("itos"))
		write(script, []byte("#!/bin/sh\n"))
		direct := filepath.Join(dir("direct"), exe("git"))
		if err := os.Symlink(script, direct); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
		// git -> itos-latest -> itos, as an install that links its own name.
		latest := filepath.Join(filepath.Dir(script), exe("itos-latest"))
		if err := os.Symlink(exe("itos"), latest); err != nil {
			t.Fatal(err)
		}
		chained := filepath.Join(dir("chained"), exe("git"))
		if err := os.Symlink(latest, chained); err != nil {
			t.Fatal(err)
		}
		real1(t, filepath.Dir(direct)+sep+filepath.Dir(chained)+sep+filepath.Dir(real), real)
		if IsItos(real) || IsItos(script+".none") {
			t.Errorf("IsItos(%s) or a missing file's is true", real)
		}

		t.Setenv(EnvGit, direct)
		if got := Inherited(); got != "" {
			t.Errorf("ITOS_GIT naming another itos is inherited: %q", got)
		}
		t.Setenv(EnvGit, real)
		if got := Inherited(); got != real {
			t.Errorf("Inherited = %q, want %q", got, real)
		}
	})
	t.Run("a Go program that is not itos", func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		text, err := os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
		other := filepath.Join(dir("other"), exe("git"))
		write(other, text)
		real1(t, filepath.Dir(other)+sep+filepath.Dir(real), other)
	})
}
