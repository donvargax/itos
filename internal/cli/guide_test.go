package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/guides"
	"github.com/donvargax/itos/v8/internal/git"
)

// A scratch repository with one commit and no config, as the current
// folder, away from the user's git config; its git folder is returned. The
// repository is made once for the package and copied.
func notesRepo(t *testing.T) (dir, gitDir string) {
	t.Helper()
	scratchEnv(t)
	return notesRepoCopy(t, notesTemplate(t))
}

// notesTemplate is the folder holding the repository notesRepo copies, made
// once under the scratch environment.
func notesTemplate(t *testing.T) string {
	t.Helper()
	return madeOnce(t, "notes", func(t *testing.T, dir string) {
		for _, args := range [][]string{
			{"init", "-q", dir},
			{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-q", "--allow-empty", "-m", "docs: start"},
		} {
			if out, err := exec.Command(git.Bin(runtime.GOOS), args...).CombinedOutput(); err != nil {
				t.Fatalf("git %s: %s", strings.Join(args, " "), out)
			}
		}
	})
}

// notesRepoCopy is a copy of the repository template holds, with itos's
// folder in its git folder, as the current folder.
func notesRepoCopy(t *testing.T, template string) (dir, gitDir string) {
	t.Helper()
	dir = t.TempDir()
	copyTree(t, template, dir)
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
	if out, err := exec.Command(git.Bin(runtime.GOOS), "-C", dir, "worktree", "add", "-q", "--detach", linked).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s", out)
	}
	t.Chdir(linked)
	code, stdout, stderr := run("go")
	if code != 0 || !strings.Contains(stdout, guides.LocalNotesHeading) || !strings.Contains(stdout, "Shared by every worktree.") {
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
	if code != 0 || strings.Count(stdout, "Printed once.") != 1 || strings.Contains(stdout, guides.LocalNotesHeading) ||
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

// itos guide <procedure> prints the procedure alone, under --json as its
// guide and text; an argument after it, a guide's name with one, an unknown
// name and no name at all are usage errors.
func TestGuideProcedures(t *testing.T) {
	notesRepo(t)
	code, stdout, stderr := run("guide", "land")
	if code != 0 || !strings.HasPrefix(stdout, "# Landing a result\n") || stderr != "" {
		t.Errorf("guide land: exit %d, stdout %.60q, stderr %s", code, stdout, stderr)
	}
	code, stdout, _ = run("guide", "push", "--json")
	if code != 0 || !strings.Contains(stdout, `"guide": "push"`) || !strings.Contains(stdout, `"text": "# Pushing with itos\n`) {
		t.Errorf("guide push --json: exit %d, stdout %.200s", code, stdout)
	}
	for args, want := range map[string]string{
		"guide brief slice-1": "guide brief takes no argument: slice-1",
		"guide work slice-1":  "guide takes one name: work slice-1",
		"guide review":        "no guide or procedure review: the guides are coordinate, work; the procedures are brief, commit, decide, land, push, repair",
		"guide":               "guide needs a name: the guides are coordinate, work; the procedures are",
	} {
		code, _, stderr := run(strings.Fields(args)...)
		if code != 2 || !strings.Contains(stderr, want) {
			t.Errorf("%s: exit %d, stderr %s", args, code, stderr)
		}
	}
}

// itos guide commit prints the commit policy the config sets after the
// procedure; with no config the procedure
// prints alone, and with one that cannot be read, alone after a warning.
func TestGuideCommitPrintsTheConfigsPolicy(t *testing.T) {
	dir, _ := notesRepo(t)
	code, stdout, stderr := run("guide", "commit")
	if code != 0 || strings.Contains(stdout, "## The rules the config sets") || stderr != "" {
		t.Errorf("no config: exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
	writeFile(t, filepath.Join(dir, "itos.yaml"), "version: 1\ncommits:\n  types: [feat, deps]\n")
	code, stdout, stderr = run("guide", "commit")
	policy := strings.Index(stdout, "## The rules the config sets\n\nAs `itos.yaml` sets them; the hooks and CI hold")
	if code != 0 || policy < strings.Index(stdout, "itos commit check-paths") ||
		!strings.Contains(stdout[max(policy, 0):], "A commit's type is one of `feat` or `deps`.") || stderr != "" {
		t.Errorf("a config: exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
	writeFile(t, filepath.Join(dir, "itos.yaml"), "version: [\n")
	code, stdout, stderr = run("guide", "commit")
	if code != 0 || strings.Contains(stdout, "## The rules the config sets") || !strings.HasPrefix(stdout, "# Committing with itos\n") ||
		!strings.Contains(stderr, "itos: itos.yaml cannot be read, so the commit rules it sets are not printed: ") {
		t.Errorf("a broken config: exit %d, stdout %.60q, stderr %s", code, stdout, stderr)
	}
}
