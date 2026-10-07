package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v6/internal/draft"
	"github.com/donvargax/itos/v6/internal/git"
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
