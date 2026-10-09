package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
)

func taskAddTestConfig() *config.Loaded {
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	return cfg
}

func taskAddRepo(t *testing.T, configText, ledgerText, registryText string) string {
	t.Helper()
	dir := gitConfigRepo(t, configText)
	standInHooks(t)
	gitIn(t, "config", "user.name", "itos test")
	gitIn(t, "config", "user.email", "test@localhost")
	for path, text := range map[string]string{"tasks/phase-1.yaml": ledgerText, "tasks/work-items.yaml": registryText} {
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
	return dir
}

func TestTaskAddArgsParsesChecksAndTimeouts(t *testing.T) {
	n, err := taskAddArgs(taskAddTestConfig(), []string{
		"--phase=1", "--type=chore", "--title=Title", "--why=Reason",
		"--check=true", "--timeout=2", "--check", "echo done", "--timeout=0.5",
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
	if n.Checks[0].Timeout == nil || *n.Checks[0].Timeout != 2 || n.Checks[1].Timeout == nil || *n.Checks[1].Timeout != 0.5 {
		t.Fatalf("taskAddArgs timeouts = %#v, want 2 and 0.5", n.Checks)
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
		{name: "missing group", args: []string{"--type", "chore", "--title", "Title", "--why", "Reason", "--check", "true"}, want: taskAddUsage},
		{name: "missing type", args: []string{"--group", "1", "--title", "Title", "--why", "Reason", "--check", "true"}, want: taskAddUsage},
		{name: "missing title", args: []string{"--group", "1", "--type", "chore", "--why", "Reason", "--check", "true"}, want: taskAddUsage},
		{name: "missing why", args: []string{"--group", "1", "--type", "chore", "--title", "Title", "--check", "true"}, want: taskAddUsage},
		{name: "unknown flag", args: append(append([]string{}, base...), "--unknown", "value"), want: "task add does not take --unknown"},
		{name: "timeout before check", args: append(append([]string{}, base...), "--timeout", "2"), want: "task add takes one --timeout"},
		{name: "empty check", args: append(append([]string{}, base...), "--check", " "), want: "task add --check needs a command"},
		{name: "invalid timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "nope"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "duplicate timeout", args: append(append(append([]string{}, base...), "--check", "true", "--timeout", "2", "--timeout", "3"), "--check", "echo done"), want: "task add takes one --timeout"},
		{name: "missing final option value", args: []string{"--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason", "--check"}, want: "task add --check needs a value"},
		{name: "duplicate field", args: append(append([]string{}, base...), "--title", "Another title", "--check", "true"), want: "task add takes one --title"},
		{name: "blank field", args: []string{"--group", "1", "--type", "chore", "--title", " ", "--why", "Reason", "--check", "true"}, want: "task add takes one --title"},
		{name: "zero timeout", args: append(append([]string{}, base...), "--check", "true", "--timeout", "0"), want: "task add takes one --timeout"},
		{name: "negative timeout", args: append(append([]string{}, base...), "--check", "true", "--timeout", "-1"), want: "task add takes one --timeout"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := taskAddArgs(taskAddTestConfig(), test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("taskAddArgs error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestTaskAddArgsRetainsCallerIDForCommandValidation(t *testing.T) {
	args := []string{"T-004", "--group", "1", "--type", "chore", "--title", "Title", "--why", "Reason", "--check", "true"}
	n, err := taskAddArgs(taskAddTestConfig(), args)
	if err != nil || n.ID != "T-004" {
		t.Fatalf("taskAddArgs ID = %q, %v, want T-004 for later command validation", n.ID, err)
	}
}

func TestTaskAddPreservesLedgerPolicyBeforeRefusingCallerID(t *testing.T) {
	for _, test := range []struct {
		name, config, registry, group, kind, want string
		callerID                                  bool
	}{
		{
			name: "type", config: "commits: { types: [docs] }\n", registry: "phases: { 1: null }\nitems: []\n",
			group: "1", kind: "chore", want: "is not a commit type", callerID: true,
		},
		{
			name: "registry group", registry: "phases: { 1: null }\nitems: []\n",
			group: "4", kind: "docs", want: "is not listed", callerID: true,
		},
		{
			name: "ledger group pattern", registry: "phases: { 1: null }\nitems: []\n",
			group: "two", kind: "docs", want: "ledger.group.pattern", callerID: true,
		},
		{
			name: "registry group after automatic mint", registry: "phases: { 1: null }\nitems: []\n",
			group: "4", kind: "docs", want: "is not listed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfgText := "version: 1\n" + test.config + "ledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n  group: { pattern: \"\\\\d+\" }\nwork: { registry: tasks/work-items.yaml }\n"
			dir := gitConfigRepo(t, cfgText)
			standInHooks(t)
			gitIn(t, "config", "user.name", "itos test")
			gitIn(t, "config", "user.email", "test@localhost")
			for path, text := range map[string]string{
				"tasks/phase-1.yaml":    "- { id: T-001, type: docs, title: Existing }\n",
				"tasks/work-items.yaml": test.registry,
			} {
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
			args := []string{"--group", test.group, "--type", test.kind, "--title", "New task", "--why", "Because.", "--check", "true"}
			if test.callerID {
				args = append([]string{"T-002"}, args...)
			}
			code, err := taskAdd(args, Out{Stdout: &stdout, Stderr: &stderr})
			if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("taskAdd with invalid %s and caller ID = (%d, %v), stdout=%q stderr=%q, want policy refusal containing %q", test.name, code, err, stdout.String(), stderr.String(), test.want)
			}
		})
	}
}

func TestTaskAddReturnsConfigAndArgumentErrors(t *testing.T) {
	t.Run("invalid config", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := os.WriteFile("itos.yaml", []byte("version: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := taskAdd(nil, Out{}); err == nil {
			t.Fatal("taskAdd succeeded with invalid config")
		}
	})
	t.Run("invalid arguments", func(t *testing.T) {
		gitConfigRepo(t, "version: 1\n")
		if _, err := taskAdd([]string{"--unknown"}, Out{}); err == nil || !strings.Contains(err.Error(), "does not take --unknown") {
			t.Fatalf("taskAdd error = %v, want unknown-flag usage error", err)
		}
	})
}

func TestTaskAddReturnsLedgerErrorsBeforeRefusingCallerID(t *testing.T) {
	t.Run("missing ledger section", func(t *testing.T) {
		taskAddRepo(t, "version: 1\n", "[]\n", "phases: { 1: null }\nitems: []\n")
		var stdout, stderr strings.Builder
		_, err := taskAdd([]string{"T-002", "--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr})
		if err == nil || !strings.Contains(err.Error(), "ledger is missing") {
			t.Fatalf("taskAdd with no ledger section = %v, stderr=%q, want missing-ledger error", err, stderr.String())
		}
	})
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	taskAddRepo(t, configText, "- { id: T-001, type: chore\n", "phases: { 1: null }\nitems: []\n")
	var stdout, stderr strings.Builder
	code, err := taskAdd([]string{"T-002", "--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "tasks/phase-1.yaml") {
		t.Fatalf("taskAdd with a malformed ledger and caller ID = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	t.Run("missing ledger folder", func(t *testing.T) {
		configText := "version: 1\nledger:\n  files: \"missing/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
		taskAddRepo(t, configText, "[]\n", "phases: { 1: null }\nitems: []\n")
		var stdout, stderr strings.Builder
		code, err := taskAdd([]string{"T-002", "--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr})
		problem := stderr.String()
		if err != nil {
			problem = err.Error()
		}
		if (err == nil && code != ExitPolicy) || !strings.Contains(problem, "missing") {
			t.Fatalf("taskAdd with a missing ledger folder = (%d, %v), stderr=%q", code, err, stderr.String())
		}
	})
}

func TestTaskAddPropagatesLedgerEditErrors(t *testing.T) {
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	for _, test := range []struct {
		name, ledger string
		callerID     bool
	}{
		{name: "before reservation", ledger: "[{id: T-001, type: chore, title: Existing}]\n", callerID: true},
		{name: "after reservation", ledger: "[{id: T-001, type: chore, title: Existing}]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			taskAddRepo(t, configText, test.ledger, "phases: { 1: null }\nitems: []\n")
			args := []string{"--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}
			if test.callerID {
				args = append([]string{"T-002"}, args...)
			}
			_, err := taskAdd(args, Out{})
			if err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
				t.Fatalf("taskAdd error = %v, want ledger edit error", err)
			}
		})
	}
}

func TestTaskAddPropagatesRegistryEditErrors(t *testing.T) {
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	registry := "{phases: {1: null}, items: []}\n"
	for _, test := range []struct {
		name     string
		callerID bool
	}{
		{name: "before reservation", callerID: true},
		{name: "after reservation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			taskAddRepo(t, configText, "- { id: T-001, type: chore, title: Existing }\n", registry)
			args := []string{"--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}
			if test.callerID {
				args = append([]string{"T-002"}, args...)
			}
			_, err := taskAdd(args, Out{})
			if err == nil || !strings.Contains(err.Error(), "cannot be edited in place") {
				t.Fatalf("taskAdd error = %v, want registry edit error", err)
			}
		})
	}
}

func TestTaskAddCreatesANewGroupLedgerFile(t *testing.T) {
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n  group: { pattern: \"\\\\d+\" }\n"
	dir := taskAddRepo(t, configText, "- { id: T-001, type: docs, title: Existing }\n", "phases: { 1: null, 2: null }\nitems:\n  - { id: T-001, title: Existing, phase: 1, owner: null, status: done, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	code, err := taskAdd([]string{"--group", "2", "--type", "docs", "--title", "Second group", "--why", "A new file.", "--check", "true"}, Out{Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 || !strings.Contains(stdout.String(), "a new tasks/phase-2.yaml") {
		t.Fatalf("taskAdd new group = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	text, err := os.ReadFile(filepath.Join(dir, "tasks/phase-2.yaml"))
	if err != nil || !strings.Contains(string(text), "id: T-002") {
		t.Fatalf("new ledger file = %q (%v), want task T-002", text, err)
	}
}

func TestTaskAddPropagatesReservationFailures(t *testing.T) {
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	taskAddRepo(t, configText, "- { id: T-001, type: chore, title: Existing }\n", "phases: { 1: null }\nitems:\n  - { id: T-001, title: Existing, phase: 1, owner: null, status: done, depends_on: [] }\n")
	fakeGit := filepath.Join(t.TempDir(), "fake-git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\necho injected git failure >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ITOS_GIT", fakeGit)
	_, err := taskAdd([]string{"--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}, Out{})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") {
		t.Fatalf("taskAdd reservation error = %v, want injected Git command failure", err)
	}
}

func TestTaskAddPropagatesLedgerErrorsAfterIDReservation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the deterministic Git wrapper is a POSIX shell script")
	}
	configText := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n"
	dir := taskAddRepo(t, configText, "- { id: T-001, type: chore, title: Existing }\n", "phases: { 1: null }\nitems:\n  - { id: T-001, title: Existing, phase: 1, owner: null, status: done, depends_on: [] }\n")
	ledgerPath := filepath.Join(dir, "tasks/phase-1.yaml")
	fakeGit := filepath.Join(t.TempDir(), "git-wrapper")
	script := "#!/bin/sh\nout=$(\"$T129_NATIVE_GIT\" \"$@\") || exit $?\nif [ \"$1\" = rev-parse ] && [ \"$2\" = --git-common-dir ]; then rm \"$T129_LEDGER\" && mkdir \"$T129_LEDGER\"; fi\nprintf '%s\\n' \"$out\"\n"
	if err := os.WriteFile(fakeGit, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("T129_NATIVE_GIT", git.Bin())
	t.Setenv("T129_LEDGER", ledgerPath)
	t.Setenv("ITOS_GIT", fakeGit)
	_, err := taskAdd([]string{"--group", "1", "--type", "chore", "--title", "Task", "--why", "Reason", "--check", "true"}, Out{})
	info, statErr := os.Stat(ledgerPath)
	if err == nil || statErr != nil || !info.IsDir() {
		t.Fatalf("taskAdd after its ledger path became a directory = %v; path info=(%v,%v)", err, info, statErr)
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
	if !strings.Contains(stdout.String(), "docs: add T-002") {
		t.Fatalf("taskAdd stdout = %q, want the committed header", stdout.String())
	}
	ledgerText, err := os.ReadFile(filepath.Join(dir, "tasks/phase-1.yaml"))
	if err != nil || !strings.Contains(string(ledgerText), "id: T-002") {
		t.Fatalf("ledger after taskAdd = %q (%v), want T-002", ledgerText, err)
	}
	registryText, err := os.ReadFile(filepath.Join(dir, "tasks/work-items.yaml"))
	if err != nil || !strings.Contains(string(registryText), "id: T-002") {
		t.Fatalf("registry after taskAdd = %q (%v), want T-002", registryText, err)
	}
	stdout.Reset()
	stderr.Reset()
	code, err = taskAdd([]string{
		"--group", "1", "--type", "chore", "--title", "JSON task", "--why", "Because.", "--check", "true",
	}, Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	var report struct {
		OK     bool           `json:"ok"`
		Task   map[string]any `json:"task"`
		Commit any            `json:"commit"`
	}
	jsonErr := json.Unmarshal([]byte(stdout.String()), &report)
	if code != 0 || err != nil || jsonErr != nil || !report.OK || report.Task["id"] != "T-003" || report.Commit == nil {
		t.Fatalf("JSON taskAdd = (%d, %v), parse error=%v, report=%v stderr=%q, want committed task T-003", code, err, jsonErr, report, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code, err = taskAdd([]string{
		"--group", "1", "--type", "chore", "--title", "Quiet task", "--why", "Because.", "--check", "true",
	}, Out{Stdout: &stdout, Stderr: &stderr, Quiet: true})
	if code != 0 || err != nil || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("quiet taskAdd = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	beforeLedger, beforeRegistry := string(ledgerText), string(registryText)
	ledgerText, err = os.ReadFile(filepath.Join(dir, "tasks/phase-1.yaml"))
	if err != nil || !strings.Contains(string(ledgerText), "id: T-004") {
		t.Fatalf("ledger after quiet taskAdd = %q (%v), want T-004", ledgerText, err)
	}
	registryText, err = os.ReadFile(filepath.Join(dir, "tasks/work-items.yaml"))
	if err != nil || !strings.Contains(string(registryText), "id: T-004") {
		t.Fatalf("registry after quiet taskAdd = %q (%v), want T-004", registryText, err)
	}
	beforeLedger, beforeRegistry = string(ledgerText), string(registryText)
	head := gitIn(t, "rev-parse", "HEAD")
	stdout.Reset()
	stderr.Reset()
	refusingHook(t)
	code, err = taskAdd([]string{
		"--group", "1", "--type", "chore", "--title", "Refused task", "--why", "Because.", "--check", "true",
	}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != ExitPolicy || err != nil || stdout.Len() != 0 {
		t.Fatalf("taskAdd with a refusing hook = (%d, %v), stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	ledgerText, _ = os.ReadFile(filepath.Join(dir, "tasks/phase-1.yaml"))
	registryText, _ = os.ReadFile(filepath.Join(dir, "tasks/work-items.yaml"))
	if string(ledgerText) != beforeLedger || string(registryText) != beforeRegistry || gitIn(t, "rev-parse", "HEAD") != head {
		t.Fatal("refused taskAdd changed the ledger, registry, or HEAD")
	}
	stdout.Reset()
	stderr.Reset()
	code, err = taskAdd([]string{
		"T-006", "--group", "1", "--type", "chore", "--title", "Supplied ID", "--why", "Because.", "--check", "true",
	}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "mints the task id") {
		t.Fatalf("taskAdd with a valid caller ID = (%d, %v), stdout=%q stderr=%q, want usage refusal", code, err, stdout.String(), stderr.String())
	}
	ledgerText, _ = os.ReadFile(filepath.Join(dir, "tasks/phase-1.yaml"))
	registryText, _ = os.ReadFile(filepath.Join(dir, "tasks/work-items.yaml"))
	if string(ledgerText) != beforeLedger || string(registryText) != beforeRegistry || gitIn(t, "rev-parse", "HEAD") != head {
		t.Fatal("taskAdd with a caller ID changed the ledger, registry, or HEAD")
	}
}
