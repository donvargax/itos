package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

// GhBounded is Gh within GhTimeout: it gives gh's stdout and its failures as
// Gh does, and a gh given up on, killed for taking too long, is
// ErrGhNoAnswer, not a failure of gh's.
func TestGhBounded(t *testing.T) {
	onPath(t, "printf '%s' \"$*\"\nif [ \"$1\" = fail ]; then echo refused >&2; exit 4; fi\n")
	if out, err := GhBounded(t.Context(), t.TempDir(), "pr", "list"); out != "pr list" || err != nil {
		t.Errorf("GhBounded = %q, %v", out, err)
	}
	var failed *GhFailed
	if out, err := GhBounded(t.Context(), t.TempDir(), "fail"); out != "fail" || !errors.As(err, &failed) || failed.TimedOut || failed.Words != "refused\n" {
		t.Errorf("GhBounded of a gh that fails = %q, %v", out, err)
	}
	late, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if out, err := GhBounded(late, t.TempDir(), "pr", "list"); out != "" || !errors.Is(err, ErrGhNoAnswer) {
		t.Errorf("GhBounded out of time = %q, %v, want ErrGhNoAnswer", out, err)
	}
	if _, err := Gh(late, t.TempDir(), "pr", "list"); !errors.As(err, &failed) || !failed.TimedOut || errors.Is(err, ErrGhNoAnswer) {
		t.Errorf("Gh out of time = %v, want a GhFailed that timed out", err)
	}
	t.Setenv("PATH", t.TempDir())
	if out, err := GhBounded(t.Context(), t.TempDir(), "pr", "list"); out != "" || !errors.Is(err, ErrNoGh) {
		t.Errorf("GhBounded with no gh = %q, %v", out, err)
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

// A failed gh call is told as a person reads it: gh not there, what gh said
// trimmed, else the error itself, as a gh that said nothing leaves it.
func TestGhReason(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{ErrNoGh, "gh is not on the PATH"},
		{fmt.Errorf("wrapped: %w", ErrNoGh), "gh is not on the PATH"},
		{&GhFailed{Words: "HTTP 401: Bad credentials\n", Err: errors.New("exit status 1")}, "HTTP 401: Bad credentials"},
		{&GhFailed{Words: " \n", Err: errors.New("exit status 1")}, "exit status 1"},
		{errors.New("something else"), "something else"},
	} {
		if got := GhReason(c.err); got != c.want {
			t.Errorf("GhReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
