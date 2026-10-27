package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeGitSays is the line the test binary prints, exiting 0, when it runs
// as git: a git whose every answer is that line.
const fakeGitSays = "ITOS_VCS_TEST_FAKE_GIT_SAYS"

// TestMain also runs as a git that answers fakeGitSays's line, a program
// Windows starts as directly as any other.
func TestMain(m *testing.M) {
	if fault := os.Getenv(snapshotGitFault); fault != "" {
		os.Exit(runSnapshotGit(fault))
	}
	if line := os.Getenv(fakeGitSays); line != "" {
		fmt.Println(line)
		os.Exit(0)
	}
	code := m.Run()
	if templateDir != "" {
		os.RemoveAll(templateDir)
	}
	os.Exit(code)
}

// useFakeGit makes git a copy of this test binary answering line to every
// command.
func useFakeGit(t *testing.T, line string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	name := "fake-git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(bin, text, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_GIT", bin)
	t.Setenv(fakeGitSays, line)
}

// A git whose answer to rev-parse is neither the three folders nor the
// git folder's two locates nothing.
func TestLocateRefusesAnAnswerOfNoFolders(t *testing.T) {
	useFakeGit(t, "one line")
	if l, err := (Git{Platform: runtime.GOOS}).locate("sub"); err == nil {
		t.Errorf("located %+v", l)
	}
}
