package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
)

func taskAddTestConfig() *config.Loaded {
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	return cfg
}

func TestTaskAddArgsParsesChecksAndTimeouts(t *testing.T) {
	n, err := taskAddArgs(taskAddTestConfig(), []string{
		"--phase=1", "--type=chore", "--title=Title", "--why=Reason",
		"--check=true", "--timeout=2", "--check", "echo done",
	})
	if err != nil {
		t.Fatalf("taskAddArgs: %v", err)
	}
	if n.Group != "1" || n.Type != "chore" || n.Title != "Title" || n.Why != "Reason" {
		t.Fatalf("taskAddArgs fields = %#v", n)
	}
	if len(n.Checks) != 2 || n.Checks[0].Run != "true" || n.Checks[1].Run != "echo done" {
		t.Fatalf("taskAddArgs checks = %#v", n.Checks)
	}
	if n.Checks[0].Timeout == nil || *n.Checks[0].Timeout != 2 || n.Checks[1].Timeout != nil {
		t.Fatalf("taskAddArgs timeouts = %#v, want 2 and nil", n.Checks)
	}
}

func TestTaskAddArgsRejectsInvalidCheckAndTimeoutOptions(t *testing.T) {
	base := []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason"}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing check", args: base, want: taskAddUsage},
		{name: "unknown flag", args: append(append([]string{}, base...), "--unknown", "value"), want: "task add does not take --unknown"},
		{name: "timeout before check", args: append(append([]string{}, base...), "--timeout", "2"), want: "task add takes one --timeout"},
		{name: "empty check", args: append(append([]string{}, base...), "--check", " "), want: "task add --check needs a command"},
		{name: "invalid timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "nope"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "duplicate timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "2", "--timeout", "3"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "explicit id", args: append([]string{"T-004"}, append(append([]string{}, base...), "--check", "true")...), want: "task add mints the task id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := taskAddArgs(taskAddTestConfig(), test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("taskAddArgs error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestTaskAddWritesAndReportsMintedTask(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n")
	standInHooks(t)
	gitIn(t, "config", "user.name", "itos test")
	gitIn(t, "config", "user.email", "test@localhost")
	files := map[string]string{
		"tasks/phase-1.yaml":    "- { id: T-001, type: chore, title: Existing }\n",
		"tasks/work-items.yaml": "phases: { 1: null }\nitems:\n  - { id: T-001, title: Existing, phase: 1, owner: null, status: done, depends_on: [] }\n",
	}
	for path, text := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, "add", "--", "itos.yaml", "tasks/phase-1.yaml", "tasks/work-items.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: start")

	var stdout, stderr strings.Builder
	code, err := taskAdd([]string{
		"--group", "1", "--type", "chore", "--title", "New task", "--why", "Because.", "--check", "true",
	}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 {
		t.Fatalf("taskAdd = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "T-002") {
		t.Fatalf("taskAdd stdout = %q, want minted ID T-002", stdout.String())
	}
	ledgerText, err := os.ReadFile(filepath.Join(dir, "tasks/phase-1.yaml"))
	if err != nil || !strings.Contains(string(ledgerText), "id: T-002") {
		t.Fatalf("ledger after taskAdd = %q (%v), want T-002", ledgerText, err)
	}
	registryText, err := os.ReadFile(filepath.Join(dir, "tasks/work-items.yaml"))
	if err != nil || !strings.Contains(string(registryText), "id: T-002") {
		t.Fatalf("registry after taskAdd = %q (%v), want T-002", registryText, err)
	}
}
