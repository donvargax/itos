package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
	vcstest "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// at is the VCS fake locating each folder as located says, in no
// repository when it lacks the folder.
func at(located map[string]vcs.Location) vcstest.VCS {
	return vcstest.VCS{Located: located}
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
	t.Chdir(dir)
	t.Setenv("ITOS_CONFIG", "")
	repo := at(map[string]vcs.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}})
	stealth := filepath.Join(".git", "itos", "itos.yaml")
	if got := Path(repo, adapter.Worktree); got != "itos.yaml" {
		t.Fatalf("with no config, Path = %q, want itos.yaml", got)
	}
	writeFile(t, stealth, stealthText)
	if got := Path(repo, adapter.Worktree); got != stealth {
		t.Fatalf("Path = %q, want %q", got, stealth)
	}
	beside := filepath.Join(".git", "itos")
	c, err := Load(repo, adapter.Trees{}, adapter.Worktree, Path(repo, adapter.Worktree))
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
	if c, err := Load(repo, adapter.Trees{}, adapter.Worktree, Path(repo, adapter.Worktree)); err != nil || !c.Stealth || c.Ledger.Files != filepath.Join(filepath.Dir(abs), "tasks/phase-{group}.yaml") {
		t.Errorf("ITOS_CONFIG naming the stealth config: %+v, %v", c, err)
	}
	writeFile(t, "other.yaml", stealthText)
	t.Setenv("ITOS_CONFIG", "other.yaml")
	if c, err := Load(repo, adapter.Trees{}, adapter.Worktree, Path(repo, adapter.Worktree)); err != nil || c.Stealth || c.Ledger.Files != "tasks/phase-{group}.yaml" ||
		c.Work.People.File != "people.yaml" || c.Hooks.Bin != "tools/bin/itos" {
		t.Errorf("ITOS_CONFIG naming another config: %+v, %v", c, err)
	}
	t.Setenv("ITOS_CONFIG", "")
	writeFile(t, "itos.yaml", stealthText)
	if got := Path(repo, adapter.Worktree); got != "itos.yaml" || IsStealth(repo, got) {
		t.Errorf("with an itos.yaml in the root, Path = %q", got)
	}
}

// A linked worktree reads the config in the common git folder, which the
// VCS names absolute there, and Locate finds it for another folder than the
// current one.
func TestStealthConfigInALinkedWorktree(t *testing.T) {
	parent := t.TempDir()
	common := filepath.Join(parent, "repo", ".git")
	writeFile(t, filepath.Join(common, "itos", "itos.yaml"), stealthText)
	t.Setenv("ITOS_CONFIG", "")
	t.Chdir(parent)
	repo := at(map[string]vcs.Location{
		"wt":   {Top: filepath.Join(parent, "wt"), GitDir: filepath.Join(common, "worktrees", "wt"), Common: common},
		"repo": {Top: filepath.Join(parent, "repo"), GitDir: ".git", Common: ".git"},
	})
	got := Locate(repo, "wt")
	if !filepath.IsAbs(got) || !IsStealthIn(repo, "wt", got) {
		t.Errorf("Locate(wt) = %q, want the stealth config, absolute", got)
	}
	if got := Locate(repo, "repo"); got != filepath.Join(".git", "itos", "itos.yaml") || !IsStealthIn(repo, "repo", got) {
		t.Errorf("Locate(repo) = %q", got)
	}
	if IsStealthIn(repo, "elsewhere", got) {
		t.Errorf("IsStealthIn outside a repository is true")
	}
}

// While itos reads a git tree, as the commit-msg hook reads the index, an
// itos.yaml in the root wins over the stealth config whether the tree or
// only the working tree holds it.
func TestARootConfigInEitherTreeWinsOverTheStealthOne(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("ITOS_CONFIG", "")
	writeFile(t, filepath.Join(".git", "itos", "itos.yaml"), stealthText)
	repo := at(map[string]vcs.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}})
	index := func(files map[string]string) string {
		src, err := porttest.Trees{Common: ".git", Files: map[string]map[string]string{"index": files}}.At(context.Background(), "index")
		if err != nil {
			t.Fatal(err)
		}
		return Path(repo, src)
	}
	writeFile(t, "itos.yaml", stealthText)
	if got := index(nil); got != "itos.yaml" {
		t.Errorf("an itos.yaml only in the working tree: Locate = %q", got)
	}
	if err := os.Remove("itos.yaml"); err != nil {
		t.Fatal(err)
	}
	if got := index(map[string]string{"itos.yaml": stealthText}); got != "itos.yaml" {
		t.Errorf("an itos.yaml only in the index: Locate = %q", got)
	}
	if got := index(nil); got != filepath.Join(".git", "itos", "itos.yaml") {
		t.Errorf("an itos.yaml in neither tree: Locate = %q", got)
	}
}
