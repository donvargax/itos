package config

import (
	"os"
	"path/filepath"
	"testing"
)

// From a subfolder with no itos.yaml of its own, the run reads its config at
// the repository's top, the itos.yaml there or else the stealth config; at
// the top, in a folder with its own itos.yaml, with ITOS_CONFIG, in a
// repository with no config at all and outside one, it stays where it is.
func TestTop(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q")
	if real, err := filepath.EvalSymlinks(repo); err == nil {
		repo = real
	}
	for _, d := range []string{"a/b", "own", "bare", "stealthy"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := findTop(filepath.Join(repo, "bare")); got != "" {
		t.Errorf("with no config anywhere, findTop = %q, want \"\"", got)
	}
	writeFile(t, filepath.Join(repo, ".git", "itos", "itos.yaml"), stealthText)
	if got := findTop(filepath.Join(repo, "stealthy")); !same(got, repo) {
		t.Errorf("with the stealth config, findTop = %q, want %q", got, repo)
	}
	writeFile(t, filepath.Join(repo, fileName), "version: 1\n")
	writeFile(t, filepath.Join(repo, "own", fileName), "version: 1\n")

	t.Chdir(filepath.Join(repo, "a", "b"))
	if got := Top(""); !same(got, repo) {
		t.Errorf("from a/b, Top(\"\") = %q, want %q", got, repo)
	}
	if got := Top(filepath.Join(repo, "own")); got != "" {
		t.Errorf("for a folder with its own itos.yaml, Top = %q, want \"\"", got)
	}
	if got := Top(repo); got != "" {
		t.Errorf("at the top, Top = %q, want \"\"", got)
	}
	if got := Top(filepath.Join(repo, ".git")); got != "" {
		t.Errorf("in the git folder, Top = %q, want \"\"", got)
	}
	if got := Top(t.TempDir()); got != "" {
		t.Errorf("outside a repository, Top = %q, want \"\"", got)
	}
	t.Setenv("ITOS_CONFIG", "elsewhere.yaml")
	if got := Top(filepath.Join(repo, "a")); got != "" {
		t.Errorf("with ITOS_CONFIG, Top = %q, want \"\"", got)
	}
}
