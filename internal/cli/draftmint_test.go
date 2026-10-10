package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/draft"
)

// A scratch repository under a stealth config whose registry holds T-002
// and slice-4, the ledger T-002: the repository's top and itos's folder.
func mintingRepo(t *testing.T) (dir, itosDir string) {
	t.Helper()
	dir, gitDir := notesRepo(t)
	itosDir = filepath.Join(gitDir, "itos")
	writeFile(t, filepath.Join(itosDir, "itos.yaml"), "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\" }\nwork: { registry: work-items.yaml }\n")
	if err := os.MkdirAll(filepath.Join(itosDir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(itosDir, "tasks", "phase-1.yaml"), "- { id: T-002, type: docs, title: Two }\n")
	writeFile(t, filepath.Join(itosDir, "work-items.yaml"),
		"phases:\n  1: null\nitems:\n  - { id: T-002, title: Two, phase: 1, status: todo }\n  - { id: slice-4, title: Four, phase: 1, status: todo }\n")
	return dir, itosDir
}

func fileText(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

var sliceLine = []string{"work", "add", "--kind", "slice", "--title", "A thing", "--why", "Because."}

func TestMintedKindIsWhatTheLineMintsWhenItRuns(t *testing.T) {
	mintingRepo(t)
	task := []string{"task", "add", "--group", "1", "--type", "docs", "--title", "A task", "--why", "Because.", "--check", "true"}
	for _, c := range []struct {
		line []string
		want string
	}{
		{sliceLine, "slice"},
		{[]string{"work", "add", "--kind", "task", "--title", "A task", "--why", "Because."}, "task"},
		{[]string{"work", "add", "p1-thing", "--title", "A thing", "--why", "Because."}, ""},
		{[]string{"work", "add", "slice-9", "--kind", "slice", "--title", "A thing", "--why", "Because."}, ""},
		{[]string{"work", "promote", "p1-thing", "--kind", "task"}, "task"},
		{[]string{"work", "promote", "p1-thing", "--kind", "slice", "--id", "slice-9"}, ""},
		{task, "task"},
		{append([]string{"task", "add", "T-009"}, task[2:]...), ""},
		{[]string{"work", "queue", "slice-4", "--top"}, ""},
		{append([]string{"--help"}, sliceLine...), ""},
		{[]string{"work"}, ""},
		{[]string{"work", "add", "--bogus", "x"}, ""},
	} {
		if got := mintedKind(c.line); got != c.want {
			t.Errorf("mintedKind(%q) = %q, want %q", c.line, got, c.want)
		}
	}
	t.Setenv("ITOS_CONFIG", filepath.Join(t.TempDir(), "none.yaml"))
	if got := mintedKind(task); got != "" {
		t.Errorf("mintedKind of a task add whose config is not there = %q, want none", got)
	}
}

func TestDraftMintReservesTheIdAndPutsTheProcessBack(t *testing.T) {
	dir, itosDir := mintingRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	var stderr strings.Builder
	o := Out{Stdout: &stderr, Stderr: &stderr}
	minted, code, err := draftMint(append([]string{"--root", dir, "--no-color"}, sliceLine...), o)
	if minted != "slice-5" || code != 0 || err != nil {
		t.Fatalf("draftMint = %q, %d, %v, want slice-5", minted, code, err)
	}
	if here, _ := os.Getwd(); !sameDir(here, sub) {
		t.Errorf("draftMint left the process in %s, not %s", here, sub)
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		t.Error("draftMint left NO_COLOR set")
	}
	if counters := fileText(t, filepath.Join(itosDir, "ids.yaml")); !strings.Contains(counters, "slice: 5") {
		t.Errorf("the counter holds %q, not slice 5", counters)
	}
	t.Chdir(dir)
	if minted, _, _ := draftMint(sliceLine, o); minted != "slice-6" {
		t.Errorf("a second mint gave %q, not slice-6", minted)
	}
	if minted, code, err := draftMint([]string{"work", "add", "p1-thing", "--title", "A", "--why", "B"}, o); minted != "" || code != 0 || err != nil {
		t.Errorf("an idea's work add minted %q, %d, %v", minted, code, err)
	}
	if minted, code, err := draftMint(append([]string{"--root", filepath.Join(dir, "gone")}, sliceLine...), o); minted != "" || code != 0 || err != nil {
		t.Errorf("a line whose --root is not there minted %q, %d, %v", minted, code, err)
	}
	writeFile(t, filepath.Join(itosDir, "work-items.yaml"),
		"phases:\n  1: null\nitems:\n  - { id: slice-4, title: Four, phase: 1, status: todo }\n  - { id: slice-4, title: Again, phase: 1, status: todo }\n")
	if minted, code, err := draftMint(sliceLine, o); minted != "" || code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "slice-4") {
		t.Errorf("an unsound registry: draftMint = %q, %d, %v, stderr %q", minted, code, err, stderr.String())
	}
}

func TestRunReservedWritesTheItemUnderTheReservedId(t *testing.T) {
	_, itosDir := mintingRepo(t)
	t.Setenv("NO_COLOR", "kept")
	var out strings.Builder
	code := runReserved(draft.Draft{ID: "thing", Command: append([]string{"--no-color"}, sliceLine...), Minted: "slice-9"}, &out)
	if code != 0 || !strings.Contains(out.String(), "slice-9 is a new slice") {
		t.Fatalf("runReserved: exit %d, output %q", code, out.String())
	}
	if registry := fileText(t, filepath.Join(itosDir, "work-items.yaml")); !strings.Contains(registry, "id: slice-9") {
		t.Errorf("the registry holds no slice-9:\n%s", registry)
	}
	if _, err := os.Stat(filepath.Join(itosDir, "ids.yaml")); err == nil {
		t.Error("runReserved minted an id beside the one reserved")
	}
	if os.Getenv("NO_COLOR") != "kept" {
		t.Errorf("NO_COLOR is %q, not put back", os.Getenv("NO_COLOR"))
	}
	if code := runReserved(draft.Draft{ID: "again", Command: sliceLine, Minted: "slice-9"}, &out); code != ExitPolicy {
		t.Errorf("a reserved id an item has: exit %d", code)
	}
}

func TestInPlaceGivesBackWhatFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("removing the current directory is a Unix-specific test")
	}
	gone := t.TempDir()
	t.Chdir(gone)
	if err := inPlace(func() error { return os.Remove(gone) }); err == nil {
		t.Error("inPlace gave no error when the folder it ran in went")
	}
	if err := inPlace(func() error { return nil }); err == nil {
		t.Error("inPlace gave no error outside any folder")
	}
	var out strings.Builder
	if code := runReserved(draft.Draft{ID: "thing", Command: sliceLine, Minted: "slice-9"}, &out); code != ExitSoftware || !strings.HasPrefix(out.String(), "itos: ") {
		t.Errorf("runReserved outside any folder: exit %d, output %q", code, out.String())
	}
	t.Chdir(t.TempDir())
	if err := inPlace(func() error { return os.ErrInvalid }); err != os.ErrInvalid {
		t.Errorf("inPlace gave %v, not f's error", err)
	}
}

// draft add of a line that mints prints the id and keeps it; promote
// writes the item under it, and stops at one whose item is there.
func TestDraftAddAndPromoteOfALineThatMints(t *testing.T) {
	_, itosDir := mintingRepo(t)
	code, stdout, stderr := run(append([]string{"draft", "add", "thing", "--"}, sliceLine...)...)
	if code != 0 || !strings.HasSuffix(stdout, "(makes slice-5)\n") {
		t.Fatalf("draft add: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	code, stdout, stderr = run(append([]string{"draft", "add", "again", "--json", "--"}, sliceLine...)...)
	if code != 0 || !strings.Contains(stdout, `"minted": "slice-6"`) {
		t.Fatalf("draft add --json: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	code, stdout, stderr = run("draft", "promote")
	if code != 0 || !strings.Contains(stdout, "thing promoted") || !strings.Contains(stdout, "again promoted") {
		t.Fatalf("draft promote: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	if registry := fileText(t, filepath.Join(itosDir, "work-items.yaml")); !strings.Contains(registry, "id: slice-5") || !strings.Contains(registry, "id: slice-6") {
		t.Errorf("the registry holds not slice-5 and slice-6:\n%s", registry)
	}
	file := filepath.Join(itosDir, draft.Folder, draft.FileName)
	if err := draft.Save(file, draft.File{Drafts: []draft.Draft{{ID: "twice", Command: sliceLine, Minted: "slice-5"}}}); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = run("draft", "promote")
	if code != ExitPolicy || !strings.Contains(stderr, "twice cannot be promoted") || !strings.Contains(stderr, "exited 1") {
		t.Errorf("a reserved id an item has: exit %d, stderr %q", code, stderr)
	}
	writeFile(t, filepath.Join(itosDir, "work-items.yaml"),
		"phases:\n  1: null\nitems:\n  - { id: slice-4, title: Four, phase: 1, status: todo }\n  - { id: slice-4, title: Again, phase: 1, status: todo }\n")
	code, _, _ = run(append([]string{"draft", "add", "unsound", "--"}, sliceLine...)...)
	if drafts, _ := draft.Load(file); code != ExitPolicy || drafts.Find("unsound") != nil {
		t.Errorf("an unsound registry: draft add exit %d, drafts %+v", code, drafts)
	}
}
