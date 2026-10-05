package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v4/internal/value"
)

// gitIn runs git in dir, away from the repository a hook runs the tests in.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
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

const stealthText = "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\n" +
	"work: { registry: tasks/work-items.yaml, people: { source: yaml, file: people.yaml } }\n" +
	"hooks: { bin: tools/bin/itos }\n"

// With no itos.yaml in the root, the config is the one in the git folder,
// and the files it names are read beside it, but for the people, which it
// has none of, and its hooks.bin is itos, whatever it says; a config
// anywhere else that ITOS_CONFIG names reads them from the root, as it says,
// and an itos.yaml in the root wins.
func TestStealthConfig(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	t.Chdir(dir)
	t.Setenv("ITOS_CONFIG", "")
	stealth := filepath.Join(".git", "itos", "itos.yaml")
	if got := Path(); got != "itos.yaml" {
		t.Fatalf("with no config, Path() = %q, want itos.yaml", got)
	}
	writeFile(t, stealth, stealthText)
	if got := Path(); got != stealth {
		t.Fatalf("Path() = %q, want %q", got, stealth)
	}
	beside := filepath.Join(".git", "itos")
	c, err := Load(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Stealth || c.Ledger.Files != filepath.Join(beside, "tasks/phase-{group}.yaml") ||
		c.Work.Registry != filepath.Join(beside, "tasks/work-items.yaml") || c.Work.Asks != filepath.Join(beside, "tasks/asks.yaml") ||
		c.Work.People.File != "" || c.Hooks.Bin != GlobalBin {
		t.Errorf("the stealth config reads %+v, %q, %q, %q, %q", c.Stealth, c.Ledger.Files, c.Work.Registry, c.Work.People.File, c.Hooks.Bin)
	}
	if table := DefaultsFor(nil, true); table.At("hooks").(*value.Map).At("bin") != GlobalBin || table.At("work").(*value.Map).Has("people") {
		t.Errorf("a stealth config's defaults are %s", value.JSON(table))
	}
	abs, _ := filepath.Abs(stealth)
	t.Setenv("ITOS_CONFIG", abs)
	if c, err := Load(Path()); err != nil || !c.Stealth || c.Ledger.Files != filepath.Join(filepath.Dir(abs), "tasks/phase-{group}.yaml") {
		t.Errorf("ITOS_CONFIG naming the stealth config: %+v, %v", c, err)
	}
	writeFile(t, "other.yaml", stealthText)
	t.Setenv("ITOS_CONFIG", "other.yaml")
	if c, err := Load(Path()); err != nil || c.Stealth || c.Ledger.Files != "tasks/phase-{group}.yaml" ||
		c.Work.People.File != "people.yaml" || c.Hooks.Bin != "tools/bin/itos" {
		t.Errorf("ITOS_CONFIG naming another config: %+v, %v", c, err)
	}
	t.Setenv("ITOS_CONFIG", "")
	writeFile(t, "itos.yaml", stealthText)
	if got := Path(); got != "itos.yaml" || IsStealth(got) {
		t.Errorf("with an itos.yaml in the root, Path() = %q", got)
	}
}

// A linked worktree reads the config in the common git folder, and Locate
// finds it for another folder than the current one.
func TestStealthConfigInALinkedWorktree(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	gitIn(t, parent, "init", "-q", "repo")
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "start")
	gitIn(t, repo, "worktree", "add", "-q", "../wt")
	writeFile(t, filepath.Join(repo, ".git", "itos", "itos.yaml"), stealthText)
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(parent)
	got := Locate("wt")
	if !filepath.IsAbs(got) || !IsStealthIn("wt", got) {
		t.Errorf("Locate(wt) = %q, want the stealth config, absolute", got)
	}
	if got := Locate("repo"); got != filepath.Join(".git", "itos", "itos.yaml") || !IsStealthIn("repo", got) {
		t.Errorf("Locate(repo) = %q", got)
	}
}
