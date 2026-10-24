package config

import (
	"os"
	"path/filepath"
	"testing"

	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// From a subfolder with no itos.yaml of its own, the run reads its config at
// the repository's top, the itos.yaml there or else the stealth config; at
// the top, in a folder with its own itos.yaml, with ITOS_CONFIG, in a
// repository with no config at all, inside the git folder and outside a
// repository, it stays where it is.
func TestTop(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(repo, ".git")
	located := map[string]vcs.Location{
		repo:                        {Top: repo, GitDir: common, Common: common},
		filepath.Join(repo, ".git"): {GitDir: ".", Common: "."},
		"":                          {Top: repo, GitDir: common, Common: "../../.git"},
		filepath.Join(repo, "a"):    {Top: repo, GitDir: common, Common: common},
		filepath.Join(repo, "own"):  {Top: repo, GitDir: common, Common: common},
		filepath.Join(repo, "bare"): {Top: repo, GitDir: common, Common: common},
		filepath.Join(repo, "deep"): {Top: repo, GitDir: common, Common: "../.git"},
	}
	for _, d := range []string{"a/b", "own", "bare", "deep", ".git"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	v := at(located)
	if got := findTop(v, filepath.Join(repo, "bare")); got != "" {
		t.Errorf("with no config anywhere, findTop = %q, want \"\"", got)
	}
	writeFile(t, filepath.Join(repo, ".git", "itos", "itos.yaml"), stealthText)
	if got := findTop(v, filepath.Join(repo, "deep")); got != repo {
		t.Errorf("with the stealth config, its common dir relative, findTop = %q, want %q", got, repo)
	}
	writeFile(t, filepath.Join(repo, fileName), "version: 1\n")
	writeFile(t, filepath.Join(repo, "own", fileName), "version: 1\n")

	t.Chdir(filepath.Join(repo, "a", "b"))
	if got := Top(v, ""); got != repo {
		t.Errorf("from a/b, Top(\"\") = %q, want %q", got, repo)
	}
	if got := Top(v, filepath.Join(repo, "own")); got != "" {
		t.Errorf("for a folder with its own itos.yaml, Top = %q, want \"\"", got)
	}
	if got := Top(v, repo); got != "" {
		t.Errorf("at the top, Top = %q, want \"\"", got)
	}
	if got := Top(v, filepath.Join(repo, ".git")); got != "" {
		t.Errorf("in the git folder, Top = %q, want \"\"", got)
	}
	if got := Top(v, t.TempDir()); got != "" {
		t.Errorf("outside a repository, Top = %q, want \"\"", got)
	}
	t.Setenv("ITOS_CONFIG", "elsewhere.yaml")
	if got := Top(v, filepath.Join(repo, "a")); got != "" {
		t.Errorf("with ITOS_CONFIG, Top = %q, want \"\"", got)
	}
}

// The config is found from the folder itos runs in, the VCS's, wherever the
// process stands: its itos.yaml, its stealth config, the top a subfolder of
// it moves to, and whether it is managed.
func TestConfigIsFoundFromTheVCSsFolder(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{"sub", filepath.Join(".git", StealthFolder)} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stealth := filepath.Join(".git", StealthFolder, fileName)
	if err := os.WriteFile(filepath.Join(dir, stealth), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	v := &vcstest.VCS{Root: dir, Located: map[string]vcs.Location{
		"":    {Top: dir, GitDir: ".git", Common: ".git"},
		"sub": {Top: dir, GitDir: "../.git", Common: "../.git"},
	}}
	if got := Locate(v, ""); got != stealth || !IsStealth(v, got) || !Managed(v, "") {
		t.Errorf("the stealth config: Locate %q, stealth %v, managed %v", got, IsStealth(v, got), Managed(v, ""))
	}
	if got := Top(v, "sub"); got != dir {
		t.Errorf("Top from sub = %q, want %q", got, dir)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Locate(v, ""); got != fileName || Top(v, "") != "" {
		t.Errorf("the project's config: Locate %q, Top %q", got, Top(v, ""))
	}
}
