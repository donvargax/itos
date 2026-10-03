package git

import (
	"errors"
	"os"
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
