package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ci range reads the config before anything else, so a broken ci.range is a
// config error, exit 2, with nothing printed and no exit code of its own.
func TestCIRangeRefusesABrokenConfig(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci: { steps: [vp check], range: { provider: command } }\n")
	var stdout, stderr strings.Builder
	code, err := ciRange("HEAD", "", Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err == nil || ExitCode(err) != ExitUsage {
		t.Fatalf("exit %d, %v", code, err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout %q", stdout.String())
	}
}

// ci scope reads the config first, so a broken one is a config error, exit
// 2, with nothing printed.
func TestCIScopeRefusesABrokenConfig(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci: { steps: [vp check], range: { provider: command } }\n")
	var stdout strings.Builder
	code, err := ciScope("HEAD", "HEAD", Out{Stdout: &stdout})
	if code != 0 || err == nil || ExitCode(err) != ExitUsage || stdout.Len() != 0 {
		t.Errorf("exit %d, %v, stdout %q", code, err, stdout.String())
	}
}

// With --json, ci range's start, commits.since's commit here with provider
// none (slice 109), is the from of one object, and the exit is 0.
func TestCIRangeJSONGivesTheStart(t *testing.T) {
	gitConfigRepo(t, "version: 1\nci: { steps: [vp check], range: { provider: none } }\n")
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	gitIn(t, "commit", "-q", "--allow-empty", "-m", "chore: start")
	since := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD"))
	gitIn(t, "commit", "-q", "--allow-empty", "-m", "chore: next")
	head := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD"))
	config := "version: 1\ncommits: { since: \"" + since + "\" }\nci: { steps: [vp check], range: { provider: none } }\n"
	if err := os.WriteFile("itos.yaml", []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	code, err := ciRange(head, "", Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	if code != 0 || err != nil {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr.String())
	}
	var got struct {
		Schema int    `json:"schema"`
		From   string `json:"from"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil || got.Schema != 1 || got.From != since {
		t.Errorf("stdout %q (%v), want from %s", stdout.String(), err, since)
	}
}

// The nightly's plan and run are its steps alone, in the order written: no
// task's checks, so no ledger or registry is read (slice 107).
func TestCINightlyIsItsStepsAlone(t *testing.T) {
	t.Setenv("ITOS_FROM", "")
	t.Setenv("ITOS_TO", "")
	gitConfigRepo(t, "version: 1\nci: { steps: [], nightly: { steps: [echo one, { run: echo two, cost: static }] } }\n")
	var stdout, stderr strings.Builder
	code, err := ciPlan("", "", true, "", Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil || stdout.String() != "late  echo one\nstatic  echo two\n" {
		t.Fatalf("plan: exit %d, %v, stdout %q", code, err, stdout.String())
	}
	stdout.Reset()
	code, err = ciRun("", "", true, false, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil || !strings.Contains(stdout.String(), "one\n") || !strings.Contains(stdout.String(), "CI passed") {
		t.Errorf("run: exit %d, %v, stdout %q", code, err, stdout.String())
	}
}

// ci run --tests with no run of named tests runs nothing and says so, on
// stderr under --json, whose stdout keeps its one object (slice 132).
func TestCIRunTestsWithNoRunOfNamedTests(t *testing.T) {
	t.Setenv("ITOS_FROM", "")
	t.Setenv("ITOS_TO", "")
	gitConfigRepo(t, "version: 1\nci: { steps: [], nightly: { steps: [echo one] } }\n")
	var stdout, stderr strings.Builder
	code, err := ciRun("", "", true, true, Out{JSON: true, Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil || strings.Contains(stderr.String(), "one\n") ||
		!strings.Contains(stderr.String(), "no named tests") || strings.Contains(stdout.String(), "no named tests") ||
		!strings.Contains(stdout.String(), `"ok": true`) {
		t.Errorf("run: exit %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
}
