package config

import (
	"os"
	"path/filepath"
	"testing"
)

// finds is whether itos run in dir reads a config, as Top and Locate find
// it, asking git: the answer Managed must give by file checks alone.
func finds(dir string) bool {
	if Top(dir) != "" {
		return true
	}
	return exists(in(dir, Locate(dir)))
}

// Managed agrees with Top and Locate from the top and from subfolders of a
// repository with an itos.yaml at its top, one with an itos.yaml only in a
// subfolder, a stealth config, a linked worktree sharing it, a repository
// with no config, a folder outside any repository and a link into a
// repository; ITOS_CONFIG naming a file decides alone.
func TestManagedAgreesWithTop(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	repo := func(files ...string) string {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		gitIn(t, dir, "init", "-q")
		for _, f := range files {
			writeFile(t, filepath.Join(dir, f), "version: 1\n")
		}
		if err := os.MkdirAll(filepath.Join(dir, "sub", "deeper"), 0o755); err != nil {
			t.Fatal(err)
		}
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

	gitIn(t, stealth, "commit", "-q", "--allow-empty", "-m", "start")
	worktree := filepath.Join(filepath.Dir(stealth), filepath.Base(stealth)+"-worktree")
	gitIn(t, stealth, "worktree", "add", "-q", "-b", "other", worktree)
	t.Cleanup(func() { os.RemoveAll(worktree) })
	if err := os.MkdirAll(filepath.Join(worktree, "sub"), 0o755); err != nil {
		t.Fatal(err)
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

	link := filepath.Join(outside, "link")
	if err := os.Symlink(filepath.Join(top, "sub"), link); err == nil {
		probes = append(probes, probe{"a link into a repository with a top config", link, true})
	}

	for _, p := range probes {
		if got := Managed(p.dir); got != p.want {
			t.Errorf("%s: Managed = %v, want %v", p.name, got, p.want)
		}
		if got := finds(p.dir); got != p.want {
			t.Errorf("%s: Top and Locate find a config: %v, want %v", p.name, got, p.want)
		}
	}

	t.Setenv("ITOS_CONFIG", filepath.Join(top, "itos.yaml"))
	if !Managed(none) {
		t.Error("ITOS_CONFIG naming a file: not managed")
	}
	t.Setenv("ITOS_CONFIG", filepath.Join(none, "missing.yaml"))
	if Managed(top) {
		t.Error("ITOS_CONFIG naming no file: managed")
	}
}

// Inside the git folder there is no work tree for a commit.
func TestManagedInsideGitFolder(t *testing.T) {
	t.Setenv("ITOS_CONFIG", "")
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "itos.yaml"), "version: 1\n")
	if _, _, ok := WorkTree(filepath.Join(dir, ".git", "refs")); ok {
		t.Error("a work tree found inside the git folder")
	}
	if Managed(filepath.Join(dir, ".git", "refs")) {
		t.Error("managed inside the git folder")
	}
}
