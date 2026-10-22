package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The ledger and registry of closeRepo, before work done.
const (
	closeLedger   = "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n  done_when:\n    - run: \"true\"\n"
	closeRegistry = "phases: { 1: null }\nitems:\n  - { id: T-1, title: One, phase: 1, owner: null, status: doing, depends_on: [] }\n"
)

// A scratch repository, as rollbackRepo makes one, whose ledger has the task
// T-1 with a check that passes, its item doing, and no remote and no CI to
// ask: work done of T-1 closes it.
func closeRepo(t *testing.T) {
	t.Helper()
	config := "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\nci: { watch: { provider: none } }\n"
	dir := gitConfigRepo(t, config)
	standInHooks(t)
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"tasks/phase-1.yaml": closeLedger, "tasks/work-items.yaml": closeRegistry} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, "add", "--", "itos.yaml", "tasks/phase-1.yaml", "tasks/work-items.yaml")
	gitIn(t, "commit", "-q", "-m", "chore: start")
}

// The close commit takes the task's checks out of its ledger entry, its why
// kept, and its body names the commit before it, which still holds them
// (slice 101).
func TestWorkDoneTakesTheChecksOut(t *testing.T) {
	closeRepo(t)
	before := strings.TrimSpace(gitIn(t, "rev-parse", "--short", "HEAD"))
	var stdout, stderr strings.Builder
	code, err := runWork([]string{"done", "T-1"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if text, _ := os.ReadFile("tasks/phase-1.yaml"); string(text) != "- id: T-1\n  type: docs\n  title: One\n  why: Because.\n" {
		t.Errorf("the ledger reads %q", text)
	}
	if files := gitIn(t, "show", "--format=", "--name-only", "HEAD"); files != "tasks/phase-1.yaml\ntasks/work-items.yaml\n" {
		t.Errorf("the close commit touches:\n%s", files)
	}
	body := gitIn(t, "log", "-1", "--format=%B")
	for _, want := range []string{"docs: close T-1", "Its checks are taken out of tasks/phase-1.yaml", "The commit before this one, " + before + ", still holds them."} {
		if !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
			t.Errorf("the body lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(stdout.String(), "; its checks are out of tasks/phase-1.yaml") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

// A close commit a hook refuses leaves the ledger and the registry as they
// were, and nothing staged.
func TestWorkDoneRefusedKeepsTheChecks(t *testing.T) {
	closeRepo(t)
	refusingHook(t)
	var stdout, stderr strings.Builder
	code, err := runWork([]string{"done", "T-1"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	if code != ExitPolicy || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	for name, want := range map[string]string{"tasks/phase-1.yaml": closeLedger, "tasks/work-items.yaml": closeRegistry} {
		if text, _ := os.ReadFile(name); string(text) != want {
			t.Errorf("%s reads %q", name, text)
		}
	}
	if status := gitIn(t, "status", "--porcelain"); status != "" {
		t.Errorf("git status reports:\n%s", status)
	}
}

// closeRepo with a Task: footer linking commits to the ledger's tasks,
// committed.
func closeRepoLinked(t *testing.T) {
	t.Helper()
	closeRepo(t)
	text, err := os.ReadFile("itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	footers := "commits:\n  footers:\n    Task:\n      source: ledger\n      read_at: commit\n"
	if err := os.WriteFile("itos.yaml", append(text, footers...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "commit", "-q", "-m", "chore: link tasks", "--", "itos.yaml")
}

// closeRepoLinked with a remote, origin, a bare repository holding its
// commits.
func closeRepoWithRemote(t *testing.T) {
	t.Helper()
	closeRepoLinked(t)
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, "init", "-q", "--bare", remote)
	gitIn(t, "remote", "add", "origin", remote)
	gitIn(t, "push", "-q", "origin", "HEAD")
}

// An empty commit with the message.
func commitEmpty(t *testing.T, message string) string {
	t.Helper()
	gitIn(t, "commit", "-q", "--allow-empty", "-m", message)
	return gitLine(t, "rev-parse", "HEAD")
}

// work done passes over a commit no remote has that is not the item's own,
// and refuses while one of the item's own is on no remote (slice 127).
func TestWorkDoneJudgesTheItemsOwnCommits(t *testing.T) {
	closeRepoWithRemote(t)
	commitEmpty(t, "chore: build T-1\n\nTask: T-1\n")
	gitIn(t, "push", "-q", "origin", "HEAD")
	commitEmpty(t, "chore: something else")
	var stdout, stderr strings.Builder
	code, err := runWork([]string{"done", "T-1"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}

	closeRepoWithRemote(t)
	own := commitEmpty(t, "chore: build T-1\n\nTask: T-1\n")
	stdout.Reset()
	stderr.Reset()
	code, err = runWork([]string{"done", "T-1"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	want := "The history up to " + gitLine(t, "rev-parse", "--short", own) + ", the newest commit of T-1, has a commit no remote has"
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), want) {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
}
