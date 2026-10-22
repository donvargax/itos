package config

import (
	"os"
	"path/filepath"
	"testing"

	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// finds is whether itos run in dir reads a config, as Top and Locate find
// it, asking the VCS: the answer Managed must give by file checks alone.
func finds(v vcs.VCS, dir string) bool {
	if Top(v, dir) != "" {
		return true
	}
	return exists(in(dir, Locate(v, dir)))
}

// Managed agrees with Top and Locate from the top and from subfolders of a
// repository with an itos.yaml at its top, one with an itos.yaml only in a
// subfolder, a stealth config, a linked worktree sharing it, a repository
// with no config and a folder outside any repository;
// ITOS_CONFIG naming a file decides alone.
func TestManagedAgreesWithTop(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	located := map[string]vcs.Location{}
	repo := func(files ...string) string {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			writeFile(t, filepath.Join(dir, f), "version: 1\n")
		}
		if err := os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755); err != nil {
			t.Fatal(err)
		}
		common := filepath.Join(dir, ".git")
		for _, d := range []string{dir, filepath.Join(dir, "sub"), filepath.Join(dir, "sub", "deeper")} {
			located[d] = vcs.Location{Top: dir, GitDir: common, Common: common}
		}
		located[common] = vcs.Location{GitDir: common, Common: common}
		return dir
	}
	type probe struct {
		name, dir string
		want      bool
	}
	var probes []probe

	top := repo("itos.yaml")
	probes = append(probes, probe{"top config, at the top", top, true},
		probe{"top config, from a subfolder", filepath.Join(top, "sub", "deeper"), true})

	own := repo(filepath.Join("sub", "itos.yaml"))
	probes = append(probes, probe{"a subfolder's own config, there", filepath.Join(own, "sub"), true},
		probe{"a subfolder's own config, from the top", own, false},
		probe{"a subfolder's own config, from below it", filepath.Join(own, "sub", "deeper"), false})

	stealth := repo(filepath.Join(".git", "itos", "itos.yaml"))
	probes = append(probes, probe{"stealth, at the top", stealth, true},
		probe{"stealth, from a subfolder", filepath.Join(stealth, "sub"), true})

	worktree := repo()
	common := filepath.Join(stealth, ".git")
	for _, d := range []string{worktree, filepath.Join(worktree, "sub")} {
		located[d] = vcs.Location{Top: worktree, GitDir: filepath.Join(common, "worktrees", "wt"), Common: common}
	}
	probes = append(probes, probe{"stealth, from a linked worktree", worktree, true},
		probe{"stealth, from a linked worktree's subfolder", filepath.Join(worktree, "sub"), true})

	none := repo()
	probes = append(probes, probe{"no config, at the top", none, false},
		probe{"no config, from a subfolder", filepath.Join(none, "sub"), false})

	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	probes = append(probes, probe{"outside a repository", outside, false})

	v := at(located)
	for _, p := range probes {
		if got := Managed(v, p.dir); got != p.want {
			t.Errorf("%s: Managed = %v, want %v", p.name, got, p.want)
		}
		if got := finds(v, p.dir); got != p.want {
			t.Errorf("%s: Top and Locate find a config: %v, want %v", p.name, got, p.want)
		}
	}

	t.Setenv("ITOS_CONFIG", filepath.Join(top, "itos.yaml"))
	if !Managed(v, none) {
		t.Error("ITOS_CONFIG naming a file: not managed")
	}
	t.Setenv("ITOS_CONFIG", filepath.Join(none, "missing.yaml"))
	if Managed(v, top) {
		t.Error("ITOS_CONFIG naming no file: managed")
	}
}

// Inside the git folder there is no work tree for a commit.
func TestManagedInsideGitFolder(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "itos.yaml"), "version: 1\n")
	common := filepath.Join(dir, ".git")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	if Managed(at(map[string]vcs.Location{common: {GitDir: ".", Common: "."}}), common) {
		t.Error("managed inside the git folder")
	}
}
