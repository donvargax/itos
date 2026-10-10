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

	"github.com/donvargax/itos/v7/internal/git"
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
// real git for the rest.
func failGit(t *testing.T, subcommand string) {
	t.Helper()
	realGit, err := git.Real()
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	name := "failing-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(bin, text, 0o755); err != nil {
		t.Fatal(err)
	}
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
		{"show", "cannot reading id counter on remote"},
		{"read-tree", "build id counter commit"},
		{"push", "cannot pushing id counter on remote"},
	} {
		t.Run(tc.subcommand, func(t *testing.T) {
			root, remote := counterRemote(t, "3\n")
			failGit(t, tc.subcommand)
			n, err := Mint(Store{Root: root, Remote: "origin"}, "slice", 0)
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
	n, err := Mint(Store{Root: root, Remote: "origin"}, "slice", 0)
	if err != nil || n != 1 {
		t.Fatalf("Mint = (%d, %v), want (1, nil)", n, err)
	}
	if got := strings.TrimSpace(gitAt(t, remote, "show", "refs/itos/ids:counters/slice")); got != "1" {
		t.Fatalf("remote slice counter = %q, want 1", got)
	}
}
