package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
)

// holdTracker's release gives the tracker back, and says on stderr when it
// cannot; a tracker another writer holds is the Hold's error, its release
// a no-op.
func TestHoldTrackerGivesTheTrackerBack(t *testing.T) {
	tracker := &porttest.Tracker{Stealth: true}
	var stderr strings.Builder
	o := Out{Stderr: &stderr}
	release, err := holdTracker(tracker, o)
	if err != nil {
		t.Fatal(err)
	}
	again, err := holdTracker(tracker, o)
	if err == nil {
		t.Error("a tracker held twice")
	}
	again()
	release()
	if stderr.Len() > 0 {
		t.Errorf("a release said %q", stderr.String())
	}
	release()
	if stderr.String() != "itos: the lock cannot be given back: the tracker is not held\n" {
		t.Errorf("a second release said %q", stderr.String())
	}
}

// stealthPromoteRepo is a stealth config's folder, in the git folder: its
// config, a ledger file of phase 1 and a registry with an idea, p1-idea.
func stealthPromoteRepo(t *testing.T) string {
	t.Helper()
	dir := gitConfigRepo(t, "")
	if err := os.Remove("itos.yaml"); err != nil {
		t.Fatal(err)
	}
	itos := filepath.Join(dir, ".git", "itos")
	for name, text := range map[string]string{
		"itos.yaml":          "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\nwork: { registry: work-items.yaml }\n",
		"tasks/phase-1.yaml": "- { id: T-001, type: chore, title: One }\n",
		"work-items.yaml":    "phases: { 1: null }\nitems:\n  - { id: p1-idea, title: Idea, phase: 1, status: todo, kind: idea, why: Specified }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(itos, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(itos, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return itos
}

// Under a stealth config, a writer of the registry or the questions whose
// registry's lock cannot be taken is the lock's error, naming the lock
// file, and writes nothing.
func TestStealthWritersStopWhenTheRegistrysLockFails(t *testing.T) {
	itos := stealthPromoteRepo(t)
	readOnly(t, itos)
	for _, args := range [][]string{{"work", "take", "p1-idea"}, {"decision", "add", "Why?"},
		{"task", "add", "--group", "1", "--type", "chore", "--title", "Two", "--why", "Why.", "--check", "true"}} {
		code, stdout, stderr := run(args...)
		if code != ExitSoftware || stdout != "" || !strings.Contains(stderr, "work-items.yaml.lock") {
			t.Errorf("itos %s: exit %d, stdout %q, stderr %q", strings.Join(args, " "), code, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(itos, "tasks", "asks.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the questions were written: %v", err)
	}
}

// work done of a queued item takes it out of the queue as the tracker loads
// the registry after the close, in a commit of its own; work show reads
// the questions about it through the tracker, none here.
func TestWorkDoneUnqueuesThroughTheTracker(t *testing.T) {
	closeRepo(t)
	putFile(t, filepath.Join("tasks", "work-items.yaml"), "queue: [T-1]\n"+closeRegistry)
	gitIn(t, "commit", "-q", "-am", "docs: queue T-1")
	var stdout, stderr strings.Builder
	code, err := runWork([]string{"done", "T-1"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS})
	if code != 0 || err != nil || !strings.Contains(stdout.String(), "; out of the queue: committed ") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if log := gitIn(t, "log", "-2", "--format=%s"); log != "docs: dequeue T-1\ndocs: close T-1\n" {
		t.Errorf("the log reads:\n%s", log)
	}
	if code, stdout, stderr := run("work", "show", "T-1"); code != 0 || !strings.Contains(stdout, "T-1") {
		t.Errorf("work show: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// A writer of the questions whose config cannot be read is the config's
// error, exit 2.
func TestDecisionAddOfABrokenConfig(t *testing.T) {
	gitConfigRepo(t, "version: 1\nnot_a_key: 1\n")
	if code, stdout, _ := run("decision", "add", "Why?"); code != ExitUsage || stdout != "" {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
}

// itos decision reads and writes the questions, the records' folder and
// their index through the tracker: a question asked, answered and shown,
// then recorded as the next record past the folder's highest number, the
// index made beside it.
func TestDecisionReadsAndWritesThroughTheTracker(t *testing.T) {
	dir := gitConfigRepo(t, "version: 1\n")
	standInHooks(t)
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	putFile(t, filepath.Join("docs", "decisions", "0004-older.md"), "# Older\n")
	if err := os.Mkdir("tasks", 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "itos.yaml", "docs/decisions/0004-older.md")
	gitIn(t, "commit", "-q", "-m", "chore: start")
	for _, step := range []struct {
		args []string
		says string
	}{
		{[]string{"decision", "add", "Which?"}, "q-1 asked: committed "},
		{[]string{"decision", "answer", "q-1", "This."}, "q-1 answered: committed "},
		{[]string{"decision", "show", "q-1"}, "q-1: Which?\nanswered\n\nThis.\n"},
		{[]string{"decision", "record", "q-1", "--title", "Take this"}, "q-1 recorded as decision 5, docs/decisions/0005-take-this.md: committed "},
	} {
		code, stdout, stderr := run(step.args...)
		if code != 0 || !strings.HasPrefix(stdout, step.says) {
			t.Fatalf("itos %s: exit %d, stdout %q, stderr %q", strings.Join(step.args, " "), code, stdout, stderr)
		}
	}
	index, err := os.ReadFile(filepath.Join(dir, "docs", "decisions", "README.md"))
	if err != nil || !strings.Contains(string(index), "- [ADR-0005: Take this](0005-take-this.md)") {
		t.Errorf("the index reads %q, %v", index, err)
	}
	if status := gitIn(t, "status", "--porcelain"); status != "" {
		t.Errorf("git status reports:\n%s", status)
	}
}
