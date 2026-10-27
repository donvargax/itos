package cli

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
)

const fakeGitFailureEnv = "ITOS_CLI_TEST_FAKE_GIT_FAILURE"

// The environment of a git that runs the real one (wrappedGitEnv, its path)
// except where a test says: a command line starting with one of the
// "|"-separated prefixes of wrappedGitFails fails; after one starting with
// wrappedGitAfter runs, wrappedGitThen is done ("mkdir <dir>"; "gone",
// the wrapper removing itself, so the next git cannot start; or "fail", the
// command exiting 1 once it has written what it writes).
const (
	wrappedGitEnv   = "ITOS_CLI_TEST_WRAPPED_GIT"
	wrappedGitFails = "ITOS_CLI_TEST_WRAPPED_GIT_FAILS"
	wrappedGitAfter = "ITOS_CLI_TEST_WRAPPED_GIT_AFTER"
	wrappedGitThen  = "ITOS_CLI_TEST_WRAPPED_GIT_THEN"
)

// wrappedGit is the wrapper's run: the real git's exit code, or 1 for a
// command line the test fails.
func wrappedGit(args []string) int {
	line := strings.Join(args, " ")
	for _, prefix := range strings.Split(os.Getenv(wrappedGitFails), "|") {
		if prefix != "" && strings.HasPrefix(line, prefix) {
			fmt.Fprintln(os.Stderr, "injected git failure")
			return 1
		}
	}
	cmd := exec.Command(os.Getenv(wrappedGitEnv), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		code = exit.ExitCode()
	}
	if after := os.Getenv(wrappedGitAfter); after != "" && strings.HasPrefix(line, after) {
		then := os.Getenv(wrappedGitThen)
		if dir, ok := strings.CutPrefix(then, "mkdir "); ok {
			_ = os.MkdirAll(dir, 0o755)
		} else if then == "gone" {
			if self, err := os.Executable(); err == nil {
				_ = os.Remove(self)
			}
		} else if then == "fail" {
			return 1
		}
	}
	return code
}

// testBinaryAsGit is a copy of this test binary, made once for the package,
// that TestMain runs as git; the environment says what it does. It is the
// path of the copy.
func testBinaryAsGit(t *testing.T) string {
	t.Helper()
	name := "test-binary-as-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(madeOnce(t, "test-binary-as-git", func(t *testing.T, dir string) {
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
}

// useWrappedGit makes git a copy of this test binary that runs the real
// git except as fails, after and then say (wrappedGitFails, wrappedGitAfter,
// wrappedGitThen). The copy is the package's, except where it removes itself
// ("gone"): that test has a copy of its own.
func useWrappedGit(t *testing.T, fails, after, then string) {
	t.Helper()
	real := git.Bin(runtime.GOOS)
	bin := testBinaryAsGit(t)
	if then == "gone" {
		text, err := os.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		bin = filepath.Join(t.TempDir(), filepath.Base(bin))
		if err := os.WriteFile(bin, text, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(wrappedGitEnv, real)
	t.Setenv(wrappedGitFails, fails)
	t.Setenv(wrappedGitAfter, after)
	t.Setenv(wrappedGitThen, then)
	t.Setenv("ITOS_GIT", bin)
}

// TestMain also acts as a portable fake git executable for tests that need a
// child process to fail. Unlike an extensionless shell script, the test binary
// can be started directly on Windows.
func TestMain(m *testing.M) {
	if os.Getenv(fakeGitFailureEnv) == "1" {
		fmt.Fprintln(os.Stderr, "injected git failure")
		os.Exit(1)
	}
	if os.Getenv(wrappedGitEnv) != "" {
		os.Exit(wrappedGit(os.Args[1:]))
	}
	dropInheritedGit()
	code := m.Run()
	if sharedRoot != "" {
		os.RemoveAll(sharedRoot)
	}
	os.Exit(code)
}

// A hook hands its children the git environment of the repository it runs in, and go-unit-tests
// -hook-env hands every test binary one with failing hooks: every scratch repository a test makes,
// and each fixture the package builds once before a test isolates itself, would inherit it
// (T-326). A test that needs one of these variables sets it itself.
func dropInheritedGit() {
	for _, name := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_PREFIX", "GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS",
	} {
		os.Unsetenv(name)
	}
}

func useFailingGit(t *testing.T) {
	t.Helper()
	bin := testBinaryAsGit(t)
	t.Setenv("ITOS_GIT", bin)
	t.Setenv(fakeGitFailureEnv, "1")
}
