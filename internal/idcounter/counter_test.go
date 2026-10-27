package idcounter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func TestMain(m *testing.M) {
	if failing := os.Getenv(failingGitEnv); failing != "" {
		os.Exit(runFailingGit(failing))
	}
	clearGitEnvironment()
	code := m.Run()
	if sharedRoot != "" {
		os.RemoveAll(sharedRoot)
	}
	os.Exit(code)
}

func clearGitEnvironment() {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_") {
			_ = os.Unsetenv(name)
		}
	}
}

func TestClearGitEnvironmentRemovesRepositoryAndConfigOverrides(t *testing.T) {
	for name, value := range map[string]string{
		"GIT_DIR":               "/wrong/repository",
		"GIT_WORK_TREE":         "/wrong/worktree",
		"GIT_COMMON_DIR":        "/wrong/common-dir",
		"GIT_INDEX_FILE":        "/wrong/index",
		"GIT_CONFIG_COUNT":      "1",
		"GIT_CONFIG_KEY_0":      "core.bare",
		"GIT_CONFIG_VALUE_0":    "true",
		"GIT_CONFIG_PARAMETERS": "'core.bare=true'",
	} {
		t.Setenv(name, value)
	}
	clearGitEnvironment()
	for _, name := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
		"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_CONFIG_PARAMETERS",
	} {
		if value, ok := os.LookupEnv(name); ok {
			t.Errorf("%s = %q, still set after clearing inherited Git environment", name, value)
		}
	}
}

func linkedWorktreeStores(t *testing.T) []Store {
	t.Helper()
	common := t.TempDir()
	rootA, rootB := t.TempDir(), t.TempDir()
	return []Store{
		{Root: rootA, CommonDir: common, OS: runtime.GOOS},
		{Root: rootB, CommonDir: common, OS: runtime.GOOS},
	}
}

func TestLinkedWorktreeStoresUseHostOS(t *testing.T) {
	for i, store := range linkedWorktreeStores(t) {
		if store.OS != runtime.GOOS {
			t.Fatalf("store %d OS = %q, want %q", i, store.OS, runtime.GOOS)
		}
	}
}

func TestMintLocalSerializesLinkedWorktrees(t *testing.T) {
	stores := linkedWorktreeStores(t)
	common := stores[0].CommonDir
	for i, store := range stores {
		if store.OS != runtime.GOOS {
			t.Fatalf("store %d OS = %q, want %q", i, store.OS, runtime.GOOS)
		}
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
	data, err := os.ReadFile(filepath.Join(common, "itos", localFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slice: 2") {
		t.Fatalf("counter file = %q, want slice counter 2", data)
	}
}

func TestMintValidatesCategoriesAndKeepsFutureCategoriesOpen(t *testing.T) {
	for _, category := range []string{"slice", "future-2", "X9", "-", "a", "z", "A", "Z", "0", "9"} {
		if !validCategory(category) {
			t.Errorf("validCategory(%q) = false", category)
		}
	}
	for _, category := range []string{"", ".", "..", "../slice", "a/b", `a\b`, "a b", "a_b", "nul\x00byte", "`", "@", "é"} {
		if validCategory(category) {
			t.Errorf("validCategory(%q) = true", category)
		}
	}
	for _, category := range []string{"", ".", "..", "../slice", "a/b", `a\b`, "a b", "a_b", "nul\x00byte", "`", "@", "é"} {
		n, err := Mint(Store{LocalOnly: true, CommonDir: t.TempDir()}, category, 0)
		if err == nil || n != 0 {
			t.Errorf("Mint category %q = (%d, %v), want (0, error)", category, n, err)
		}
	}
}

func TestMintLocalHandlesNegativeRepositoryFloorAndStorageErrors(t *testing.T) {
	t.Run("negative floor", func(t *testing.T) {
		n, err := Mint(Store{CommonDir: t.TempDir(), LocalOnly: true}, "slice", -3)
		if err != nil || n != 1 {
			t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
		}
	})
	t.Run("missing common directory", func(t *testing.T) {
		if n, err := Mint(Store{LocalOnly: true}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("Mint = (%d, %v), want (0, error)", n, err)
		}
	})
	t.Run("unwritable counter folder", func(t *testing.T) {
		common := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(common, []byte("file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if n, err := Mint(Store{CommonDir: common, LocalOnly: true}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("Mint = (%d, %v), want (0, error)", n, err)
		}
	})
	t.Run("invalid counter yaml", func(t *testing.T) {
		common := t.TempDir()
		dir := filepath.Join(common, "itos")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, localFile), []byte("counters: ["), 0o600); err != nil {
			t.Fatal(err)
		}
		if n, err := Mint(Store{CommonDir: common, LocalOnly: true}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("Mint = (%d, %v), want (0, error)", n, err)
		}
	})
}

func TestWriteCountersReturnsAtomicReplacementErrors(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, localFile)
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeCounters(destination, map[string]int{"slice": 1}); err == nil {
		t.Fatal("writeCounters succeeded replacing a directory")
	}
}

func TestCounterCommitIsUniqueForConcurrentSameNumberClaims(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
	first, err := counterCommit(t.Context(), vcs.Git{Platform: runtime.GOOS, Dir: root}, "", "slice", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := counterCommit(t.Context(), vcs.Git{Platform: runtime.GOOS, Dir: root}, "", "slice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("identical concurrent reservations share commit %s", first)
	}
}

func TestMintRemoteRejectsMissingRepositoryAndRemoteErrors(t *testing.T) {
	if n, err := mintRemote(Store{Remote: "origin"}, "slice", 0, 1); err == nil || n != 0 {
		t.Fatalf("mintRemote without root = (%d, %v), want (0, error)", n, err)
	}
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	if n, err := mintRemote(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "missing"}, "slice", 0, 1); err == nil || n != 0 {
		t.Fatalf("mintRemote without remote = (%d, %v), want (0, error)", n, err)
	}
}

func TestMintRemoteReadsOnlyNonNegativeIntegerCounters(t *testing.T) {
	for _, data := range []string{"not-an-integer\n", "-1\n"} {
		t.Run(strings.TrimSpace(data), func(t *testing.T) {
			root, remote := counterRemote(t, data)
			n, err := Mint(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "origin"}, "slice", 0)
			if n != 0 || err == nil || !strings.Contains(err.Error(), "not a non-negative integer") {
				t.Fatalf("Mint = (%d, %v), want invalid-counter error", n, err)
			}
			if gitAt(t, remote, "show", "refs/itos/ids:counters/slice") != data {
				t.Fatal("invalid remote counter was changed")
			}
		})
	}
}

func TestMintRemoteTreatsMissingCategoryAsZero(t *testing.T) {
	root, remote := counterRemote(t, "")
	n, err := Mint(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "origin"}, "slice", 0)
	if err != nil || n != 1 {
		t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "1" {
		t.Fatalf("remote slice counter = %q, want 1", got)
	}
}

func TestMintRemoteStopsAfterSixteenPushesLostToTheRemote(t *testing.T) {
	remote := newRemoteFake()
	pushes := 0
	remote.beforePush = func() error {
		pushes++
		return remote.advance(t.Context(), "slice", pushes)
	}
	n, err := Mint(Store{Root: "repository", VCS: remote, Remote: origin}, "slice", 0)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "kept advancing") {
		t.Fatalf("Mint = (%d, %v), want retry-exhaustion error", n, err)
	}
	if pushes != 16 {
		t.Fatalf("counter push attempts = %d, want exactly 16", pushes)
	}
}

// A refusal that leaves the counter where it was is the remote's, whichever
// push it is: after fifteen lost to the remote it is not told as a sixteenth.
func TestMintRemoteReturnsTheRefusalOfAnyPush(t *testing.T) {
	remote := newRemoteFake()
	pushes := 0
	remote.beforePush = func() error {
		pushes++
		if pushes == 16 {
			return refuse()
		}
		return remote.advance(t.Context(), "slice", pushes)
	}
	n, err := Mint(Store{Root: "repository", VCS: remote, Remote: origin}, "slice", 0)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "cannot push the id counter to the remote (nothing was written)") ||
		!strings.Contains(err.Error(), "declined by a hook") {
		t.Fatalf("Mint = (%d, %v), want the push's refusal", n, err)
	}
	if pushes != 16 {
		t.Fatalf("counter push attempts = %d, want 16", pushes)
	}
}

// The push that failed is judged by the remote's counter, so a remote that
// cannot be asked is an error of its own, and no number.
func TestMintRemoteReturnsNoNumberWhenTheRemoteCannotBeAskedAfterAFailedPush(t *testing.T) {
	remote := newRemoteFake()
	remote.beforePush = func() error {
		remote.Remote[origin].Down = true
		return nil
	}
	n, err := Mint(Store{Root: "repository", VCS: remote, Remote: origin}, "slice", 0)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "cannot fetch the id counter from the remote (nothing was written)") {
		t.Fatalf("Mint = (%d, %v), want a fetch error", n, err)
	}
}

// counterRemote is a repository whose origin, a bare repository beside it,
// holds the counter of the slice category at sliceValue (none for ""). Each
// value is made once, in a folder of the package's, and copied: the
// repository's remote is the relative ../remote.git, so the copy needs no git.
func counterRemote(t *testing.T, sliceValue string) (root, remote string) {
	t.Helper()
	template := madeOnce(t, fmt.Sprintf("counter-%x", sliceValue), func(t *testing.T, dir string) {
		remote := filepath.Join(dir, "remote.git")
		root := filepath.Join(dir, "repo")
		gitRun(t, dir, "init", "-q", "--bare", remote)
		gitRun(t, dir, "init", "-q", "-b", "main", root)
		gitIdentity(t, root)
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, root, "add", "--", "README.md")
		if sliceValue != "" {
			if err := os.MkdirAll(filepath.Join(root, "counters"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "counters", "slice"), []byte(sliceValue), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, root, "add", "--", "counters/slice")
		}
		gitRun(t, root, "commit", "-q", "-m", "base")
		gitRun(t, root, "remote", "add", "origin", "../remote.git")
		gitRun(t, root, "push", "-q", "origin", "HEAD:refs/itos/ids")
	})
	base := t.TempDir()
	copyTree(t, template, base)
	return filepath.Join(base, "repo"), filepath.Join(base, "remote.git")
}

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = dir
	cmd.Env = withoutGitIdentity(os.Environ())
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// clonedStores are n clones (at most clonesMade) of one bare remote that holds
// a main branch and no counter, each a Store on its own repository, and the
// remote's folder. The clones are made once and copied: their remote is the
// relative ../origin.git, so a copy needs no git.
func clonedStores(t *testing.T, n int) (stores []Store, remote string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
	template := madeOnce(t, "clones", func(t *testing.T, dir string) {
		remote := filepath.Join(dir, "origin.git")
		gitRun(t, dir, "init", "-q", "--bare", remote)
		gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
		seed := filepath.Join(dir, "seed")
		gitRun(t, dir, "init", "-q", "-b", "main", seed)
		gitIdentity(t, seed)
		if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, seed, "add", "README.md")
		gitRun(t, seed, "commit", "-q", "-m", "base")
		gitRun(t, seed, "remote", "add", "origin", remote)
		gitRun(t, seed, "push", "-q", "origin", "HEAD:refs/heads/main")
		for i := range clonesMade {
			root := filepath.Join(dir, "clone-"+strconv.Itoa(i))
			gitRun(t, dir, "clone", "-q", remote, root)
			gitRun(t, root, "remote", "set-url", "origin", "../origin.git")
			gitIdentity(t, root)
		}
	})
	base := t.TempDir()
	copyTree(t, template, base)
	stores = make([]Store, n)
	for i := range stores {
		root := filepath.Join(base, "clone-"+strconv.Itoa(i))
		stores[i] = Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, CommonDir: filepath.Join(root, ".git"), Remote: "origin"}
	}
	return stores, filepath.Join(base, "origin.git")
}

// clonesMade is how many clones the template holds: the most a test asks for.
const clonesMade = 3

func TestMintRemoteRetriesConcurrentFastForward(t *testing.T) {
	stores, remote := clonedStores(t, 2)
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
	cmd := exec.Command(git.Bin(runtime.GOOS), "show", "refs/itos/ids:counters/slice")
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
	cmd := exec.Command(git.Bin(runtime.GOOS), args...)
	cmd.Dir = dir
	cmd.Env = withoutGitIdentity(os.Environ())
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitIdentity(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "config", "--local", "user.name", "itos test")
	gitRun(t, dir, "config", "--local", "user.email", "test@localhost")
}

func withoutGitIdentity(env []string) []string {
	keys := map[string]bool{
		"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true,
		"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true,
	}
	filtered := make([]string, 0, len(env))
	for _, pair := range env {
		key, _, _ := strings.Cut(pair, "=")
		if !keys[key] {
			filtered = append(filtered, pair)
		}
	}
	return filtered
}
