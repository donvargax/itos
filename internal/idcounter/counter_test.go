package idcounter

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/donvargax/itos/v7/internal/git"
)

func TestMain(m *testing.M) {
	if failing := os.Getenv(failingGitEnv); failing != "" {
		os.Exit(runFailingGit(failing))
	}
	clearGitEnvironment()
	os.Exit(m.Run())
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
	first, err := counterCommit(root, "", "slice", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := counterCommit(root, "", "slice", 1)
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
	if n, err := mintRemote(Store{Root: root, Remote: "missing"}, "slice", 0, 1); err == nil || n != 0 {
		t.Fatalf("mintRemote without remote = (%d, %v), want (0, error)", n, err)
	}
}

func TestMintRemoteReadsOnlyNonNegativeIntegerCounters(t *testing.T) {
	for _, data := range []string{"not-an-integer\n", "-1\n"} {
		t.Run(strings.TrimSpace(data), func(t *testing.T) {
			root, remote := counterRemote(t, data)
			n, err := Mint(Store{Root: root, Remote: "origin"}, "slice", 0)
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
	n, err := Mint(Store{Root: root, Remote: "origin"}, "slice", 0)
	if err != nil || n != 1 {
		t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "1" {
		t.Fatalf("remote slice counter = %q, want 1", got)
	}
}

func TestMintRemoteStopsAfterSixteenNonFastForwardPushes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Git command wrapper is a POSIX shell script")
	}
	root, remote := counterRemote(t, "")
	base := filepath.Dir(remote)
	pushes := filepath.Join(base, "pushes")
	native, err := git.Real()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(base, "git-wrapper")
	script := "#!/bin/sh\nif [ \"$1\" = push ]; then printf x >> \"$ITOS_COUNTER_TEST_PUSHES\"; echo 'stale info' >&2; exit 1; fi\nexec \"" + native + "\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_GIT", wrapper)
	t.Setenv("ITOS_COUNTER_TEST_PUSHES", pushes)
	n, err := Mint(Store{Root: root, Remote: "origin"}, "slice", 0)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "kept advancing") {
		t.Fatalf("Mint = (%d, %v), want retry-exhaustion error", n, err)
	}
	data, err := os.ReadFile(pushes)
	if err != nil || len(data) != 16 {
		t.Fatalf("counter push attempts = %d (%v), want exactly 16", len(data), err)
	}
}

func counterRemote(t *testing.T, sliceValue string) (root, remote string) {
	t.Helper()
	base := t.TempDir()
	remote = filepath.Join(base, "remote.git")
	root = filepath.Join(base, "repo")
	gitRun(t, base, "init", "-q", "--bare", remote)
	gitRun(t, base, "init", "-q", "-b", "main", root)
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
	gitRun(t, root, "remote", "add", "origin", remote)
	gitRun(t, root, "push", "-q", "origin", "HEAD:refs/itos/ids")
	return root, remote
}

func gitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git.Bin(), args...)
	cmd.Dir = dir
	cmd.Env = withoutGitIdentity(os.Environ())
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func TestCounterErrorClassifiers(t *testing.T) {
	for _, message := range []string{"object does not exist in tree", "path 'counters/slice' is missing"} {
		if !missingPath(errors.New(message)) {
			t.Errorf("missingPath(%q) = false", message)
		}
	}
	if missingPath(errors.New("permission denied")) {
		t.Error("missingPath classified permission denied as a missing path")
	}
	for _, message := range []string{"non-fast-forward", "fetch first", "stale info", "reference already exists"} {
		if !nonFastForward(errors.New(message)) {
			t.Errorf("nonFastForward(%q) = false", message)
		}
	}
	if nonFastForward(errors.New("permission denied")) {
		t.Error("nonFastForward classified permission denied as a race")
	}
}

func TestMintRemoteRetriesConcurrentFastForward(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
	base := t.TempDir()
	remote := filepath.Join(base, "origin.git")
	gitRun(t, base, "init", "-q", "--bare", remote)
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	seed := filepath.Join(base, "seed")
	gitRun(t, base, "init", "-q", "-b", "main", seed)
	gitIdentity(t, seed)
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
		gitIdentity(t, root)
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
	cmd := exec.Command(git.Bin(), "show", "refs/itos/ids:counters/slice")
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
	cmd := exec.Command(git.Bin(), args...)
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
