package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/draft"
	"github.com/donvargax/itos/v7/internal/git"
)

// An edit draft of a committed notes.md and a new.md HEAD lacks, made by
// itos draft edit in a scratch repository: the list's file and the draft.
func editDraft(t *testing.T) (string, draft.Draft) {
	t.Helper()
	dir, gitDir := notesRepo(t)
	writeFile(t, filepath.Join(dir, "notes.md"), "Old.\n")
	for _, args := range [][]string{
		{"add", "notes.md"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--no-verify", "-m", "docs: add notes"},
	} {
		if out, err := exec.Command(git.Bin(), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	code, stdout, stderr := run("draft", "edit", "say", "-m", "docs: say", "notes.md", "new.md", "notes.md")
	if code != 0 || strings.Count(stdout, "\n") != 2 {
		t.Fatalf("draft edit: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	file := filepath.Join(gitDir, "itos", draft.Folder, draft.FileName)
	drafts, err := draft.Load(file)
	if err != nil || len(drafts.Drafts) != 1 || !drafts.Drafts[0].IsEdit() {
		t.Fatalf("drafts %+v (%v), not the one edit draft", drafts, err)
	}
	return file, drafts.Drafts[0]
}

func patchOf(t *testing.T, file string, d draft.Draft) (string, string) {
	t.Helper()
	made, why, err := editPatch(file, d)
	if err != nil {
		t.Fatal(err)
	}
	if made == "" {
		return "", why
	}
	defer os.Remove(made)
	text, err := os.ReadFile(made)
	if err != nil {
		t.Fatal(err)
	}
	return string(text), why
}

// Copies left as they were taken change nothing, the empty copy of a file
// HEAD lacks included, and promote is told so.
func TestAnEditDraftLeftAsTakenHasNoPatch(t *testing.T) {
	file, d := editDraft(t)
	if patch, why := patchOf(t, file, d); patch != "" || !strings.Contains(why, "no copy differs") {
		t.Fatalf("patch %q, why %q", patch, why)
	}
}

// A copy removed takes its file out, and a new file's copy written adds it.
func TestAnEditDraftsRemovedCopyDeletesItsFile(t *testing.T) {
	file, d := editDraft(t)
	if err := os.Remove(draft.Copy(file, d.ID, "notes.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, draft.Copy(file, d.ID, "new.md"), "A spec.\n")
	patch, why := patchOf(t, file, d)
	if why != "" || !strings.Contains(patch, "deleted file mode 100644") || !strings.Contains(patch, "new file mode 100644") ||
		!strings.Contains(patch, "+A spec.") {
		t.Fatalf("patch %q, why %q", patch, why)
	}
}

// A scratch repository whose HEAD holds notes.md as "Old.", the file
// changed to "New." in the work tree: the list's file and itos's folder.
func changedNotes(t *testing.T) (file, itosDir string) {
	t.Helper()
	dir, gitDir := notesRepo(t)
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "t")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "t@t")
	}
	writeFile(t, filepath.Join(dir, "notes.md"), "Old.\n")
	for _, args := range [][]string{{"add", "notes.md"}, {"commit", "-q", "--no-verify", "-m", "docs: add notes"}} {
		if out, err := exec.Command(git.Bin(), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	writeFile(t, filepath.Join(dir, "notes.md"), "New.\n")
	itosDir = filepath.Join(gitDir, "itos")
	return filepath.Join(itosDir, draft.Folder, draft.FileName), itosDir
}

// draft add refuses a line it cannot keep, and what it cannot draft.
func TestDraftAddRefusals(t *testing.T) {
	changedNotes(t)
	for _, c := range []struct {
		args []string
		want int
	}{
		{[]string{"x", "-m"}, ExitUsage},
		{nil, ExitUsage},
		{[]string{"a b", "--", "work"}, ExitUsage},
		{[]string{"x", "-m", "a", "-m", "b", "notes.md"}, ExitUsage},
		{[]string{"x", "-m", "a", "--", "work"}, ExitUsage},
		{[]string{"x", "--"}, ExitUsage},
		{[]string{"x", "--", "draft"}, ExitUsage},
		{[]string{"x", "-m", " ", "notes.md"}, ExitUsage},
		{[]string{"x", "-m", "docs: x", "../outside.md"}, ExitUsage},
		{[]string{"x", "-m", "docs: x", "nothing.md"}, ExitPolicy},
	} {
		if code, _, stderr := run(append([]string{"draft", "add"}, c.args...)...); code != c.want {
			t.Errorf("draft add %q: exit %d, not %d; stderr %s", c.args, code, c.want, stderr)
		}
	}
	// The command line's spec refuses these first; draftAdd refuses them
	// too, whoever calls it.
	quiet := Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	for _, args := range [][]string{{"x", "-m"}, {"x", "-m", "a", "-m", "b", "notes.md"}} {
		if _, err := draftAdd(args, quiet); ExitCode(err) != ExitUsage {
			t.Errorf("draftAdd %q: %v, not a usage error", args, err)
		}
	}
	t.Chdir(t.TempDir())
	if code, _, stderr := run("draft", "add", "x", "--", "work"); code != ExitMissing {
		t.Errorf("outside a repository: exit %d, stderr %s", code, stderr)
	}
}

// draft add --json of a change says its kind, header and paths.
func TestDraftAddOfAChangeUnderJSON(t *testing.T) {
	changedNotes(t)
	code, stdout, stderr := run("draft", "add", "say", "--json", "-m", "docs: say", "notes.md")
	if code != 0 || !strings.Contains(stdout, `"kind": "change"`) || !strings.Contains(stdout, `"header": "docs: say"`) {
		t.Errorf("exit %d, stdout %s, stderr %s", code, stdout, stderr)
	}
}

// A list that cannot be saved, or files that cannot be put back, stop
// draft add with what failed.
func TestDraftAddFailures(t *testing.T) {
	t.Run("the list cannot be saved", func(t *testing.T) {
		file, _ := changedNotes(t)
		useWrappedGit(t, "", "diff", "mkdir "+filepath.Join(file, "taken"))
		if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != ExitSoftware {
			t.Errorf("exit %d, stderr %s", code, stderr)
		}
		if _, err := os.Stat(draft.Patch(file, "say")); err == nil {
			t.Error("the patch of a draft not saved is left")
		}
	})
	t.Run("the files cannot be put back", func(t *testing.T) {
		changedNotes(t)
		useWrappedGit(t, "checkout", "", "")
		if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != ExitSoftware ||
			!strings.Contains(stderr, "cannot be put back") {
			t.Errorf("exit %d, stderr %s", code, stderr)
		}
	})
}

// A change draft taken by draft add, and promoteOne's verdict on it.
func promoteChange(t *testing.T, file string) (string, string, error) {
	t.Helper()
	if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != 0 {
		t.Fatalf("draft add: exit %d, stderr %s", code, stderr)
	}
	var log strings.Builder
	return promoteOne(file, draft.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}, Out{Stdout: &log, Stderr: &log})
}

// promoteOne gives why a draft cannot be promoted, or an error when it
// cannot even try.
func TestPromoteOneStops(t *testing.T) {
	quiet := Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	t.Run("an edit draft whose copies changed nothing", func(t *testing.T) {
		file, d := editDraft(t)
		if _, why, err := promoteOne(file, d, quiet); err != nil || !strings.Contains(why, "no copy differs") {
			t.Errorf("why %q, %v", why, err)
		}
	})
	t.Run("a patch gone", func(t *testing.T) {
		file, _ := changedNotes(t)
		if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != 0 {
			t.Fatalf("draft add: exit %d, stderr %s", code, stderr)
		}
		if err := os.Remove(draft.Patch(file, "say")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := promoteOne(file, draft.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}, quiet); err == nil {
			t.Error("no error for a patch gone")
		}
	})
	t.Run("a patch git cannot read", func(t *testing.T) {
		file, _ := changedNotes(t)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, draft.Patch(file, "say"), "no patch\n")
		if _, _, err := promoteOne(file, draft.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}, quiet); err == nil {
			t.Error("no error for a patch git cannot read")
		}
	})
	t.Run("a commit refused", func(t *testing.T) {
		file, _ := changedNotes(t)
		useWrappedGit(t, "commit", "", "")
		if _, why, err := promoteChange(t, file); err != nil || !strings.Contains(why, "its commit failed") {
			t.Errorf("why %q, %v", why, err)
		}
	})
	t.Run("a commit refused whose files cannot be put back", func(t *testing.T) {
		file, _ := changedNotes(t)
		if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != 0 {
			t.Fatalf("draft add: exit %d, stderr %s", code, stderr)
		}
		useWrappedGit(t, "commit|checkout", "", "")
		if _, _, err := promoteOne(file, draft.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}, quiet); err == nil ||
			!strings.Contains(err.Error(), "cannot be put back") {
			t.Errorf("error %v", err)
		}
	})
	t.Run("a commit git cannot start", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a running program cannot remove itself on Windows")
		}
		file, _ := changedNotes(t)
		if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != 0 {
			t.Fatalf("draft add: exit %d, stderr %s", code, stderr)
		}
		useWrappedGit(t, "", "apply --index", "gone")
		if _, _, err := promoteOne(file, draft.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}, quiet); err == nil {
			t.Error("no error for a git that cannot start")
		}
	})
}

// A command draft is run by this itos's binary: one that cannot be found
// or started is an error, one that fails is why the draft stops.
func TestPromoteOneOfACommand(t *testing.T) {
	file, _ := changedNotes(t)
	d := draft.Draft{ID: "check", Command: []string{"work", "check"}}
	quiet := Out{Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	t.Cleanup(func() { executable = os.Executable })
	for name, self := range map[string]func() (string, error){
		"no binary":          func() (string, error) { return "", errors.New("no binary") },
		"a binary not there": func() (string, error) { return filepath.Join(t.TempDir(), "gone"), nil },
	} {
		executable = self
		if _, _, err := promoteOne(file, d, quiet); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	useFailingGit(t)
	failing := os.Getenv("ITOS_GIT")
	executable = func() (string, error) { return failing, nil }
	if _, why, err := promoteOne(file, d, quiet); err != nil || why != "itos work check exited 1" {
		t.Errorf("why %q, %v", why, err)
	}
}
