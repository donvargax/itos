package adapter

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/private/port/porttest"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// clone is a new folder, a git repository whose work tree ignores
// ignored.md when repository, else in no repository, with no git setting
// from outside and a short lock wait; the test stands in another folder.
func clone(t *testing.T, repository bool) string {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	was := lock.Wait
	lock.Wait = 50 * time.Millisecond
	t.Cleanup(func() { lock.Wait = was })
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if repository {
		if out, err := exec.Command(git.Bin(runtime.GOOS), "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %s", out)
		}
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.md\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(t.TempDir())
	return dir
}

// The adapter keeps the store on disk as the fake keeps it in memory.
func TestStoreKeepsTheContract(t *testing.T) {
	porttest.StoreContract(t, func(t *testing.T, repository bool) porttest.Fixture {
		dir := clone(t, repository)
		return porttest.Fixture{
			Store:   Store{OS: runtime.GOOS, VCS: vcs.Git{Dir: dir}},
			Kept:    filepath.Join(dir, "kept.md"),
			Ignored: filepath.Join(dir, "ignored.md"),
			Outside: filepath.Join(t.TempDir(), "outside.md"),
		}
	})
}

// The store is itos's folder of the git common dir, which every linked
// worktree shares; its files are the owner's alone, in a folder made 0700,
// with nothing left beside them.
func TestStoreIsTheCommonDirsItosFolder(t *testing.T) {
	dir := clone(t, true)
	path, err := Store{VCS: vcs.Git{Dir: dir}}.Path(t.Context(), "threads.yaml")
	if want := filepath.Join(dir, ".git", "itos", "threads.yaml"); err != nil || path != want {
		t.Fatalf("Path: %q, %v; want %q", path, err, want)
	}
	if err := (Store{VCS: vcs.Git{Dir: dir}}).Write(t.Context(), "threads.yaml", []byte("threads: []\n")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("the folder holds %v, %v; want threads.yaml alone", entries, err)
	}
	// Windows keeps no Unix permission bits.
	if runtime.GOOS == "windows" {
		return
	}
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		if info, err := os.Stat(p); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: %v, %v; want %v", p, info.Mode().Perm(), err, want)
		}
	}
}

// A file the store cannot read, or a folder it cannot write in, is the
// disk's error.
func TestStoreSaysTheDisksErrors(t *testing.T) {
	dir := clone(t, true)
	folder := filepath.Join(dir, ".git", "itos")
	if err := os.MkdirAll(filepath.Join(folder, "threads.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := (Store{VCS: vcs.Git{Dir: dir}}).Read(t.Context(), "threads.yaml"); ok || err == nil {
		t.Errorf("a folder read as a file: %v, %v", ok, err)
	}
	if err := (Store{VCS: vcs.Git{Dir: dir}}).Write(t.Context(), "threads.yaml", []byte("x")); err == nil {
		t.Error("a file written over a folder")
	}
	if err := os.RemoveAll(folder); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(folder, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{VCS: vcs.Git{Dir: dir}}).Write(t.Context(), "threads.yaml", []byte("x")); err == nil {
		t.Error("a file written where the folder is a file")
	}
	if _, err := (Store{VCS: vcs.Git{Dir: dir}}).Hold(t.Context(), "threads.yaml"); err == nil {
		t.Error("a lock taken where the folder is a file")
	}
	if err := os.Remove(folder); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}
	if err := (Store{VCS: vcs.Git{Dir: dir}}).Write(t.Context(), "threads.yaml", []byte("x")); err == nil {
		t.Error("a file written in a folder no one may write in")
	}
}
