package cli

import (
	"os"
	"path/filepath"
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
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
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
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
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
