package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/lock"
	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
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

// trackedText is a document's text; one not there is an error naming it,
// fs.ErrNotExist, and a document that cannot be read the disk's error.
func TestTrackedTextOfADocumentGone(t *testing.T) {
	tracker := &porttest.Tracker{}
	tracker.Edit("a.yaml", "a\n")
	if text, err := trackedText(tracker, "a.yaml"); text != "a\n" || err != nil {
		t.Errorf("trackedText = %q, %v", text, err)
	}
	if text, err := trackedText(tracker, "gone.yaml"); text != "" || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "gone.yaml") {
		t.Errorf("a document gone: %q, %v", text, err)
	}
	if text, err := trackedText(adapter.Files{}, "a\x00b"); text != "" || err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a name no file can have: %q, %v", text, err)
	}
}

// Under a stealth config, a writer of the registry or the questions that
// finds the registry's lock held past the wait exits 75, naming the lock
// file, and writes nothing.
func TestStealthWritersWaitForTheRegistrysLock(t *testing.T) {
	itos := stealthPromoteRepo(t)
	was := lock.Wait
	lock.Wait = 20 * time.Millisecond
	t.Cleanup(func() { lock.Wait = was })
	held := filepath.Join(itos, "work-items.yaml") + lock.Suffix
	if err := os.WriteFile(held, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"work", "take", "p1-idea"}, {"decision", "add", "Why?"}} {
		code, stdout, stderr := run(args...)
		if code != ExitTemporary || stdout != "" || !strings.Contains(stderr, "work-items.yaml"+lock.Suffix) {
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
	code, err := workDone([]string{"T-1"}, Out{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || err != nil || !strings.Contains(stdout.String(), "; out of the queue: committed ") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	if log := gitIn(t, "log", "-2", "--format=%s"); log != "docs: queue T-1\ndocs: close T-1\n" {
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
