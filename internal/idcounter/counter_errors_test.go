package idcounter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

const (
	// failingGitEnv names the one git subcommand the copied test binary
	// fails; every other subcommand it passes to the git in realGitEnv.
	failingGitEnv = "ITOS_IDCOUNTER_TEST_FAILING_GIT"
	realGitEnv    = "ITOS_IDCOUNTER_TEST_REAL_GIT"
)

// runFailingGit is the test binary started as git: a copy of the test binary,
// unlike a shell script, starts directly on Windows too.
func runFailingGit(failing string) int {
	if len(os.Args) > 1 && os.Args[1] == failing {
		fmt.Fprintln(os.Stderr, "injected git failure")
		return 1
	}
	cmd := exec.Command(os.Getenv(realGitEnv), os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	var exit *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exit) {
		return exit.ExitCode()
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// failGit makes every git the package runs fail its subcommand and run the
// real git for the rest. The copy of the test binary that is git is made once
// for the package; the subcommand it fails is the environment's.
func failGit(t *testing.T, subcommand string) {
	t.Helper()
	realGit, err := git.Real(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	name := "failing-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(madeOnce(t, "failing-git", func(t *testing.T, dir string) {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		text, err := os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), text, 0o755); err != nil {
			t.Fatal(err)
		}
	}), name)
	t.Setenv(realGitEnv, realGit)
	t.Setenv(failingGitEnv, subcommand)
	t.Setenv("ITOS_GIT", bin)
}

// The mint reads, builds and pushes through git; a failure at any of those
// steps that is not the race a retry answers must stop it with no number.
func TestMintRemoteReturnsNoNumberWhenGitFails(t *testing.T) {
	for _, tc := range []struct {
		subcommand, want string
	}{
		{"fetch", "cannot fetch the id counter from the remote"},
		{"rev-parse", "cannot fetch the id counter from the remote"},
		{"show", "cannot read the id counter on the remote"},
		{"read-tree", "build id counter commit"},
		{"hash-object", "build id counter commit"},
		{"update-index", "build id counter commit"},
		{"write-tree", "build id counter commit"},
		{"commit-tree", "build id counter commit"},
		{"push", "cannot push the id counter to the remote"},
	} {
		t.Run(tc.subcommand, func(t *testing.T) {
			root, remote := counterRemote(t, "3\n")
			failGit(t, tc.subcommand)
			n, err := Mint(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "origin"}, "slice", 0)
			if n != 0 || err == nil || !strings.Contains(err.Error(), tc.want) ||
				!strings.Contains(err.Error(), "injected git failure") {
				t.Fatalf("Mint = (%d, %v), want (0, %q error)", n, err, tc.want)
			}
			t.Setenv("ITOS_GIT", "")
			if got := gitAt(t, remote, "show", "refs/itos/ids:counters/slice"); got != "3\n" {
				t.Fatalf("remote slice counter = %q, want it unchanged", got)
			}
		})
	}
}

// Zero is a counter a remote may hold: the next reservation follows it.
func TestMintRemoteAcceptsAZeroCounter(t *testing.T) {
	root, remote := counterRemote(t, "0\n")
	n, err := Mint(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "origin"}, "slice", 0)
	if err != nil || n != 1 {
		t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "1" {
		t.Fatalf("remote slice counter = %q, want 1", got)
	}
}

// The first reservation on a remote with no counter yet builds its commit
// from an empty tree; git failing there stops it with no number, and the
// remote still has no counter.
func TestMintRemoteReturnsNoNumberWhenTheFirstCounterCommitFails(t *testing.T) {
	root, remote := counterRemote(t, "")
	gitRun(t, remote, "update-ref", "-d", "refs/itos/ids")
	failGit(t, "read-tree")
	n, err := Mint(Store{Root: root, VCS: vcs.Git{Platform: runtime.GOOS, Dir: root}, Remote: "origin"}, "slice", 0)
	if n != 0 || err == nil || !strings.Contains(err.Error(), "build id counter commit") ||
		!strings.Contains(err.Error(), "injected git failure") {
		t.Fatalf("Mint = (%d, %v), want (0, build id counter commit error)", n, err)
	}
	t.Setenv("ITOS_GIT", "")
	if got := gitAt(t, remote, "for-each-ref", "refs/itos/ids"); got != "" {
		t.Fatalf("remote counter ref = %q, want none", got)
	}
}

// A counter commit whose scratch index cannot be made, the temporary folder
// gone, is no commit.
func TestCounterCommitStopsWhereItsIndexCannotBeMade(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	missing := filepath.Join(t.TempDir(), "missing")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	if commit, err := counterCommit(t.Context(), vcs.Git{Platform: runtime.GOOS, Dir: root}, "", "slice", 1); commit != "" || err == nil {
		t.Fatalf("counterCommit = (%q, %v), want an error", commit, err)
	}
}

// A local counter file that cannot be read is no reservation; one that holds
// no counters (empty, or counters: with nothing under it) is a counter at 0.
func TestMintLocalReadsTheCounterFile(t *testing.T) {
	counterFile := func(t *testing.T) (common, file string) {
		t.Helper()
		common = t.TempDir()
		if err := os.MkdirAll(filepath.Join(common, "itos"), 0o700); err != nil {
			t.Fatal(err)
		}
		return common, filepath.Join(common, "itos", localFile)
	}
	t.Run("a folder in its place", func(t *testing.T) {
		common, file := counterFile(t)
		if err := os.Mkdir(file, 0o700); err != nil {
			t.Fatal(err)
		}
		if n, err := Mint(Store{CommonDir: common, LocalOnly: true}, "slice", 0); err == nil || n != 0 {
			t.Fatalf("Mint = (%d, %v), want (0, error)", n, err)
		}
	})
	for name, text := range map[string]string{"empty": "", "no counters": "counters:\n"} {
		t.Run(name, func(t *testing.T) {
			common, file := counterFile(t)
			if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			n, err := Mint(Store{CommonDir: common, LocalOnly: true}, "slice", 0)
			if err != nil || n != 1 {
				t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
			}
			if got, err := os.ReadFile(file); err != nil || string(got) != "counters:\n    slice: 1\n" {
				t.Fatalf("counter file = %q (%v)", got, err)
			}
		})
	}
}

// writeCounters makes the counter's folder; one it cannot make, a file in the
// way, stops it.
func TestWriteCountersStopsWhereItsFolderCannotBeMade(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCounters(filepath.Join(blocker, "itos", localFile), map[string]int{"slice": 1}); err == nil {
		t.Fatal("writeCounters made a folder under a file")
	}
}

var errDisk = errors.New("injected disk failure")

// failingFile is a real temporary file that fails at one step, as a full or
// failing disk would. Its Close still closes the real file, so it can be
// removed, on Windows too.
type failingFile struct {
	*os.File
	at string
}

func (f failingFile) Chmod(mode os.FileMode) error {
	if f.at == "chmod" {
		return errDisk
	}
	return f.File.Chmod(mode)
}

func (f failingFile) Write(b []byte) (int, error) {
	if f.at == "write" {
		return 0, errDisk
	}
	return f.File.Write(b)
}

func (f failingFile) Sync() error {
	if f.at == "sync" {
		return errDisk
	}
	return f.File.Sync()
}

func (f failingFile) Close() error {
	err := f.File.Close()
	if f.at == "close" {
		return errDisk
	}
	return err
}

// A temporary file that cannot be made, set, written, synced or closed stops
// writeCounters with its error: the counter keeps what it held, and no
// temporary file is left beside it.
func TestWriteCountersLeavesTheCounterWhereItsFileFails(t *testing.T) {
	for _, at := range []string{"create", "chmod", "write", "sync", "close"} {
		t.Run(at, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, localFile)
			const held = "counters:\n    slice: 4\n"
			if err := os.WriteFile(file, []byte(held), 0o600); err != nil {
				t.Fatal(err)
			}
			err := writeCountersThrough(file, map[string]int{"slice": 5}, func(dir string) (tempFile, error) {
				if at == "create" {
					return nil, errDisk
				}
				f, err := os.CreateTemp(dir, ".ids-*.tmp")
				if err != nil {
					return nil, err
				}
				return failingFile{f, at}, nil
			})
			if !errors.Is(err, errDisk) {
				t.Fatalf("writeCountersThrough = %v, want the injected failure", err)
			}
			if got, err := os.ReadFile(file); err != nil || string(got) != held {
				t.Fatalf("counter file = %q (%v), want it unchanged", got, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("the counter's folder holds %v (%v), want the counter alone", entries, err)
			}
		})
	}
}
