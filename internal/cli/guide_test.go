package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A scratch repository with one commit and no config, as the current
// folder, away from the user's git config; its git folder is returned.
func notesRepo(t *testing.T) (dir, gitDir string) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("ITOS_CONFIG", "")
	dir = t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "docs: start"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	gitDir = filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "itos"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return dir, gitDir
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A linked worktree reads the clone's own notes from the git common dir.
func TestLocalNotesInALinkedWorktree(t *testing.T) {
	dir, gitDir := notesRepo(t)
	writeFile(t, filepath.Join(gitDir, "itos", "notes.md"), "Shared by every worktree.\n")
	linked := filepath.Join(t.TempDir(), "linked")
	if out, err := exec.Command("git", "-C", dir, "worktree", "add", "-q", "--detach", linked).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s", out)
	}
	t.Chdir(linked)
	code, stdout, stderr := run("go")
	if code != 0 || !strings.Contains(stdout, localNotesHeading) || !strings.Contains(stdout, "Shared by every worktree.") {
		t.Errorf("exit %d, stdout ends %q, stderr %s", code, stdout[max(0, len(stdout)-300):], stderr)
	}
}

// A stealth config whose guide.orchestrating names the clone's own notes
// prints them once, as the repository's notes, with no heading of their own.
func TestLocalNotesNamedByTheStealthConfigPrintOnce(t *testing.T) {
	_, gitDir := notesRepo(t)
	writeFile(t, filepath.Join(gitDir, "itos", "itos.yaml"), "version: 1\nguide:\n  orchestrating: notes.md\n")
	writeFile(t, filepath.Join(gitDir, "itos", "notes.md"), "Printed once.\n")
	code, stdout, stderr := run("go", "--json")
	if code != 0 || strings.Count(stdout, "Printed once.") != 1 || strings.Contains(stdout, localNotesHeading) ||
		strings.Contains(stdout, `"local_notes"`) || !strings.Contains(stdout, `"notes"`) {
		t.Errorf("exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
}

// itos guide coordinate prints the guide alone: never the clone's own notes.
func TestGuideCoordinateLeavesTheLocalNotesOut(t *testing.T) {
	_, gitDir := notesRepo(t)
	writeFile(t, filepath.Join(gitDir, "itos", "notes.md"), "Only itos go prints these.\n")
	code, stdout, _ := run("guide", "coordinate")
	if code != 0 || strings.Contains(stdout, "Only itos go prints these.") {
		t.Errorf("exit %d, stdout ends %q", code, stdout[max(0, len(stdout)-300):])
	}
}
