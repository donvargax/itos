package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const fakeGitFailureEnv = "ITOS_CLI_TEST_FAKE_GIT_FAILURE"

// TestMain also acts as a portable fake git executable for tests that need a
// child process to fail. Unlike an extensionless shell script, the test binary
// can be started directly on Windows.
func TestMain(m *testing.M) {
	if os.Getenv(fakeGitFailureEnv) == "1" {
		fmt.Fprintln(os.Stderr, "injected git failure")
		os.Exit(1)
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
