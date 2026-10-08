package idcounter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestMintLocalSerializesLinkedWorktrees(t *testing.T) {
	common := t.TempDir()
	rootA, rootB := t.TempDir(), t.TempDir()
	stores := []Store{{Root: rootA, CommonDir: common}, {Root: rootB, CommonDir: common}}
	var wg sync.WaitGroup
	got := make(chan int, len(stores))
	errs := make(chan error, len(stores))
	for _, store := range stores {
		wg.Add(1)
		go func(store Store) {
			defer wg.Done()
			n, err := Mint(store, "slice", 0)
			got <- n
			errs <- err
		}(store)
	}
	wg.Wait()
	close(got)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int]bool{}
	for n := range got {
		seen[n] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("concurrent reservations = %v, want 1 and 2", seen)
	}
	data, err := os.ReadFile(filepath.Join(common, "itos", localFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slice: 2") {
		t.Fatalf("counter file = %q, want slice counter 2", data)
	}
}

func TestMintRemoteRetriesConcurrentFastForward(t *testing.T) {
	base := t.TempDir()
	remote := filepath.Join(base, "origin.git")
	gitRun(t, base, "init", "-q", "--bare", remote)
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	seed := filepath.Join(base, "seed")
	gitRun(t, base, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "add", "README.md")
	gitRun(t, seed, "commit", "-q", "-m", "base")
	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "-q", "origin", "HEAD:refs/heads/main")
	stores := make([]Store, 2)
	for i := range stores {
		root := filepath.Join(base, "clone-"+strconv.Itoa(i))
		gitRun(t, base, "clone", "-q", remote, root)
		stores[i] = Store{Root: root, CommonDir: filepath.Join(root, ".git"), Remote: "origin"}
	}
	var wg sync.WaitGroup
	got := make(chan int, len(stores))
	errs := make(chan error, len(stores))
	for _, store := range stores {
		wg.Add(1)
		go func(store Store) {
			defer wg.Done()
			n, err := Mint(store, "slice", 0)
			got <- n
			errs <- err
		}(store)
	}
	wg.Wait()
	close(got)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int]bool{}
	for n := range got {
		seen[n] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("concurrent reservations = %v, want 1 and 2", seen)
	}
	cmd := exec.Command("git", "show", "refs/itos/ids:counters/slice")
	cmd.Dir = remote
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "2" {
		t.Fatalf("remote counter = %q, want 2", data)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=itos test", "GIT_AUTHOR_EMAIL=test@localhost",
		"GIT_COMMITTER_NAME=itos test", "GIT_COMMITTER_EMAIL=test@localhost")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
