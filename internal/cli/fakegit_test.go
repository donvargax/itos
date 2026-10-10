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

	"github.com/donvargax/itos/v7/internal/git"
)

const fakeGitFailureEnv = "ITOS_CLI_TEST_FAKE_GIT_FAILURE"

// The environment of a git that runs the real one (wrappedGitEnv, its path)
// except where a test says: a command line starting with one of the
// "|"-separated prefixes of wrappedGitFails fails; after one starting with
// wrappedGitAfter runs, wrappedGitThen is done ("mkdir <dir>", or "gone",
// the wrapper removing itself, so the next git cannot start).
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
		}
	}
	return code
}

// useWrappedGit makes git a copy of this test binary that runs the real
// git except as fails, after and then say (wrappedGitFails, wrappedGitAfter,
// wrappedGitThen).
func useWrappedGit(t *testing.T, fails, after, then string) {
	t.Helper()
	real := git.Bin()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	name := "wrapped-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin = filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(bin, text, 0o755); err != nil {
		t.Fatal(err)
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
	os.Exit(m.Run())
}

func useFailingGit(t *testing.T) {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	name := "fake-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin = filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(bin, text, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_GIT", bin)
	t.Setenv(fakeGitFailureEnv, "1")
}
