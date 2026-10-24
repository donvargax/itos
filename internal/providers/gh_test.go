package providers

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// onPath puts a gh that runs the script first on the PATH.
func onPath(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the gh is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Gh gives gh's stdout, run in the folder asked; a gh that fails is a
// GhFailed holding what it said on stderr; one that is not on the PATH is
// ErrNoGh.
func TestGh(t *testing.T) {
	onPath(t, "printf '%s|%s' \"$*\" \"$(pwd)\"\nif [ \"$1\" = fail ]; then echo refused >&2; exit 4; fi\n")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := Gh(t.Context(), dir, "pr", "list"); out != "pr list|"+dir || err != nil {
		t.Errorf("Gh = %q, %v", out, err)
	}
	out, err := Gh(t.Context(), dir, "fail")
	var failed *GhFailed
	if !errors.As(err, &failed) || failed.Words != "refused\n" || out != "fail|"+dir || err.Error() != "exit status 4" ||
		!errors.Is(err, failed.Err) {
		t.Errorf("Gh = %q, %v, want a GhFailed", out, err)
	}
	t.Setenv("PATH", t.TempDir())
	if out, err := Gh(t.Context(), dir, "pr", "list"); out != "" || !errors.Is(err, ErrNoGh) {
		t.Errorf("Gh with no gh = %q, %v", out, err)
	}
}

// The repository every gh call names is the variable the config names for
// the watch's repository, else the remote's URL as GitHub's, else none.
func TestRepository(t *testing.T) {
	cfg := &config.Loaded{}
	cfg.CI.Range.GitHub.RepositoryEnv = "MY_REPOSITORY"
	env := func(set string) Env {
		return func(name string) string {
			if name == "MY_REPOSITORY" {
				return set
			}
			return "not " + name
		}
	}
	for _, c := range []struct{ env, url, want string }{
		{"o/env", "git@github.com:o/remote.git", "o/env"},
		{"", "git@github.com:o/remote.git", "o/remote"},
		{"", "/srv/git/itos.git", ""},
	} {
		if got := Repository(cfg, env(c.env), c.url); got != c.want {
			t.Errorf("Repository(%q, %q) = %q, want %q", c.env, c.url, got, c.want)
		}
	}
}
