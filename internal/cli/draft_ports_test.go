package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/git"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
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
		if out, err := exec.Command(git.Bin(runtime.GOOS), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
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

// committer sets who git says commits, and the hooks a promote's commit
// needs, in the scratch repository itos runs in.
func committer(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "t")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "t@t")
	}
	standInHooks(t)
}

// Copies left as they were taken change nothing, the empty copy of a file
// HEAD lacks included, and promote is told so.
func TestAnEditDraftLeftAsTakenHasNoPatch(t *testing.T) {
	editDraft(t)
	committer(t)
	if code, _, stderr := run("draft", "promote"); code != ExitPolicy || !strings.Contains(stderr, "no copy differs") {
		t.Fatalf("draft promote: exit %d, stderr %q", code, stderr)
	}
}

// A copy removed takes its file out, and a new file's copy written adds it.
func TestAnEditDraftsRemovedCopyDeletesItsFile(t *testing.T) {
	file, d := editDraft(t)
	committer(t)
	if err := os.Remove(draft.Copy(file, d.ID, "notes.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, draft.Copy(file, d.ID, "new.md"), "A spec.\n")
	if code, stdout, stderr := run("draft", "promote"); code != 0 || !strings.Contains(stdout, "say promoted: committed") {
		t.Fatalf("draft promote: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, err := os.Stat("notes.md"); err == nil {
		t.Error("notes.md is still there")
	}
	if text := fileText(t, "new.md"); text != "A spec.\n" {
		t.Errorf("new.md holds %q", text)
	}
}

// A scratch repository whose HEAD holds notes.md as "Old.", the file
// changed to "New." in the work tree: the list's file and itos's folder.
func changedNotes(t *testing.T) (file, itosDir string) {
	t.Helper()
	scratchEnv(t)
	// The repository with notes.md committed is made once for the package.
	template := madeOnce(t, "committed-notes", func(t *testing.T, dir string) {
		copyTree(t, notesTemplate(t), dir)
		t.Chdir(dir)
		committer(t)
		writeFile(t, filepath.Join(dir, "notes.md"), "Old.\n")
		for _, args := range [][]string{{"add", "notes.md"}, {"commit", "-q", "--no-verify", "-m", "docs: add notes"}} {
			if out, err := exec.Command(git.Bin(runtime.GOOS), append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %s: %s", strings.Join(args, " "), out)
			}
		}
	})
	dir, gitDir := notesRepoCopy(t, template)
	committer(t)
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
	t.Chdir(t.TempDir())
	if code, _, stderr := run("draft", "add", "x", "--", "work"); code != ExitMissing {
		t.Errorf("outside a repository: exit %d, stderr %s", code, stderr)
	}
}

// The drafts' file is in the folder git names as the common one; where git
// cannot name it, there is none, and draft and draft promote say they need
// a git repository, exit 3.
func TestDraftsFileIsWhereGitSays(t *testing.T) {
	changedNotes(t)
	if got := draftsFile(); !strings.HasSuffix(got, filepath.Join("itos", draft.Folder, draft.FileName)) {
		t.Errorf("draftsFile = %q", got)
	}
	useWrappedGit(t, "rev-parse", "", "")
	if got := draftsFile(); got != "" {
		t.Errorf("draftsFile of a git that failed = %q", got)
	}
	for _, args := range [][]string{{"draft"}, {"draft", "promote"}} {
		if code, stdout, stderr := run(args...); code != ExitMissing || stdout != "" || !strings.Contains(stderr, "no git repository") {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
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

// A change draft taken by draft add, the work tree then as HEAD has it.
func addChange(t *testing.T) {
	t.Helper()
	if code, _, stderr := run("draft", "add", "say", "-m", "docs: say", "notes.md"); code != 0 {
		t.Fatalf("draft add: exit %d, stderr %s", code, stderr)
	}
}

// promote stops at a draft it cannot promote, exit 1, or with an error
// when it cannot even try, exit 70.
func TestDraftPromoteStops(t *testing.T) {
	t.Run("a patch gone", func(t *testing.T) {
		file, _ := changedNotes(t)
		addChange(t)
		if err := os.Remove(draft.Patch(file, "say")); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := run("draft", "promote"); code != ExitSoftware || !strings.Contains(stderr, "say's patch cannot be read") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})
	t.Run("a patch git cannot read", func(t *testing.T) {
		file, _ := changedNotes(t)
		addChange(t)
		writeFile(t, draft.Patch(file, "say"), "no patch\n")
		if code, _, stderr := run("draft", "promote"); code != ExitSoftware || !strings.Contains(stderr, "say's patch cannot be read") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})
	t.Run("a commit refused", func(t *testing.T) {
		changedNotes(t)
		addChange(t)
		useWrappedGit(t, "commit", "", "")
		if code, _, stderr := run("draft", "promote"); code != ExitPolicy || !strings.Contains(stderr, "its commit failed") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
		if text := fileText(t, "notes.md"); text != "Old.\n" {
			t.Errorf("notes.md holds %q, not put back", text)
		}
	})
	t.Run("a commit refused whose files cannot be put back", func(t *testing.T) {
		changedNotes(t)
		addChange(t)
		useWrappedGit(t, "commit|checkout", "", "")
		if code, _, stderr := run("draft", "promote"); code != ExitSoftware || !strings.Contains(stderr, "cannot be put back") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})
	t.Run("a commit git cannot start", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a running program cannot remove itself on Windows")
		}
		changedNotes(t)
		addChange(t)
		useWrappedGit(t, "", "apply --index", "gone")
		if code, _, stderr := run("draft", "promote"); code != ExitSoftware {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})
}

// A command draft is run by this itos's binary: one that cannot be found
// or started is an error, one that fails is why the draft stops.
func TestDraftPromoteOfACommand(t *testing.T) {
	file, _ := changedNotes(t)
	writeFile(t, "notes.md", "Old.\n")
	keep := func() {
		t.Helper()
		if err := draft.Save(file, draft.File{Drafts: []draft.Draft{{ID: "check", Command: []string{"work", "check"}}}}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { executable = os.Executable })
	for name, c := range map[string]struct {
		self func() (string, error)
		want string
	}{
		"no binary":          {func() (string, error) { return "", errors.New("no binary") }, "cannot find its own binary to run check: no binary"},
		"a binary not there": {func() (string, error) { return filepath.Join(t.TempDir(), "gone"), nil }, "gone"},
	} {
		keep()
		executable = c.self
		if code, _, stderr := run("draft", "promote"); code != ExitSoftware || !strings.Contains(stderr, c.want) {
			t.Errorf("%s: exit %d, stderr %q", name, code, stderr)
		}
	}
	if err := draft.Save(file, draft.File{Drafts: []draft.Draft{{ID: "version", Command: []string{"--version"}}}}); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return git.Bin(runtime.GOOS), nil }
	if code, stdout, stderr := run("draft", "promote"); code != 0 || stdout != "version promoted: itos --version (no commit)\n" {
		t.Errorf("a command that passes: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	keep()
	useFailingGit(t)
	failing := os.Getenv("ITOS_GIT")
	t.Setenv("ITOS_GIT", "")
	executable = func() (string, error) { return failing, nil }
	if code, _, stderr := run("draft", "promote"); code != ExitPolicy || !strings.Contains(stderr, "itos work check exited 1") {
		t.Errorf("a command that fails: exit %d, stderr %q", code, stderr)
	}
}

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

// draft add mints the id of what the line makes when it runs, read by the
// commands' own code: a slice, a task, a task promoted, a task added; none
// for an idea, a caller's id, a line the command refuses or one that asks
// for help.
func TestDraftAddMintsWhatTheLineMints(t *testing.T) {
	mintingRepo(t)
	task := []string{"task", "add", "--group", "1", "--type", "docs", "--title", "A task", "--why", "Because.", "--check", "true"}
	for i, c := range []struct {
		line  []string
		mints bool
	}{
		{sliceLine, true},
		{[]string{"work", "add", "--kind", "task", "--title", "A task", "--why", "Because."}, true},
		{[]string{"work", "add", "p1-thing", "--title", "A thing", "--why", "Because."}, false},
		{[]string{"work", "add", "slice-9", "--kind", "slice", "--title", "A thing", "--why", "Because."}, false},
		{[]string{"work", "promote", "p1-thing", "--kind", "task", "--type", "docs", "--check", "true"}, true},
		{[]string{"work", "promote", "p1-thing", "--kind", "task"}, false},
		{[]string{"work", "promote", "p1-thing", "--kind", "slice", "--id", "slice-9"}, false},
		{task, true},
		{append([]string{"task", "add", "T-009"}, task[2:]...), false},
		{[]string{"work", "queue", "slice-4", "--top"}, false},
		{append([]string{"--help"}, sliceLine...), false},
		{[]string{"work"}, false},
		{[]string{"work", "add", "--bogus", "x"}, false},
	} {
		id := "line-" + string(rune('a'+i))
		code, stdout, stderr := run(append([]string{"draft", "add", id, "--json", "--"}, c.line...)...)
		if code != 0 || strings.Contains(stdout, `"minted"`) != c.mints {
			t.Errorf("draft add -- %q: exit %d, stdout %s, want minted %v; stderr %s", c.line, code, stdout, c.mints, stderr)
		}
	}
	t.Setenv("ITOS_CONFIG", filepath.Join(t.TempDir(), "none.yaml"))
	if code, stdout, _ := run(append([]string{"draft", "add", "no-config", "--json", "--"}, task...)...); code != 0 || strings.Contains(stdout, "minted") {
		t.Errorf("a task add whose config is not there: exit %d, stdout %s", code, stdout)
	}
}

// The id is minted under the line's own global flags, and the process is
// put back as it was; a line whose --root is not there mints nothing, and
// an unsound registry is refused, nothing drafted.
func TestDraftAddMintsUnderTheLinesGlobalFlags(t *testing.T) {
	dir, itosDir := mintingRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	code, stdout, stderr := run(append([]string{"draft", "add", "thing", "--", "--root", dir, "--no-color"}, sliceLine...)...)
	if code != 0 || !strings.HasSuffix(stdout, "(makes slice-5)\n") {
		t.Fatalf("draft add: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		t.Error("draft add left NO_COLOR set")
	}
	if counters := fileText(t, filepath.Join(itosDir, "ids.yaml")); !strings.Contains(counters, "slice: 5") {
		t.Errorf("the counter holds %q, not slice 5", counters)
	}
	code, stdout, stderr = run(append([]string{"draft", "add", "gone", "--", "--root", filepath.Join(dir, "gone")}, sliceLine...)...)
	if code != 0 || strings.Contains(stdout, "makes") {
		t.Errorf("a line whose --root is not there: exit %d, stdout %q, stderr %s", code, stdout, stderr)
	}
	writeFile(t, filepath.Join(itosDir, "work-items.yaml"),
		"phases:\n  1: null\nitems:\n  - { id: slice-4, title: Four, phase: 1, status: todo }\n  - { id: slice-4, title: Again, phase: 1, status: todo }\n")
	code, _, stderr = run(append([]string{"draft", "add", "unsound", "--"}, sliceLine...)...)
	if drafts, _ := draft.Load(filepath.Join(itosDir, draft.Folder, draft.FileName)); code != ExitPolicy || drafts.Find("unsound") != nil ||
		!strings.Contains(stderr, "slice-4") {
		t.Errorf("an unsound registry: draft add exit %d, stderr %q, drafts %+v", code, stderr, drafts)
	}
}

// promote runs a minting draft's line in this process, which writes the
// item under the id reserved, minting none, and puts back what its global
// flags changed; a reserved id an item has stops it.
func TestDraftPromoteWritesTheItemUnderTheReservedId(t *testing.T) {
	_, itosDir := mintingRepo(t)
	t.Setenv("NO_COLOR", "kept")
	file := filepath.Join(itosDir, draft.Folder, draft.FileName)
	if err := draft.Save(file, draft.File{Drafts: []draft.Draft{
		{ID: "thing", Command: append([]string{"--no-color"}, sliceLine...), Minted: "slice-9"},
		{ID: "again", Command: sliceLine, Minted: "slice-9"},
	}}); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run("draft", "promote")
	if code != ExitPolicy || !strings.Contains(stderr, "slice-9 is a new slice") || !strings.Contains(stderr, "again cannot be promoted") {
		t.Fatalf("draft promote: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if registry := fileText(t, filepath.Join(itosDir, "work-items.yaml")); !strings.Contains(registry, "id: slice-9") {
		t.Errorf("the registry holds no slice-9:\n%s", registry)
	}
	if _, err := os.Stat(filepath.Join(itosDir, "ids.yaml")); err == nil {
		t.Error("promote minted an id beside the one reserved")
	}
	if os.Getenv("NO_COLOR") != "kept" {
		t.Errorf("NO_COLOR is %q, not put back", os.Getenv("NO_COLOR"))
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
}

// A tree git cannot list stops draft edit, nothing kept.
func TestDraftEditOfATreeGitCannotList(t *testing.T) {
	changedNotes(t)
	useWrappedGit(t, "ls-tree", "", "")
	if code, _, stderr := run("draft", "edit", "say", "-m", "docs: say", "notes.md"); code != ExitSoftware || !strings.Contains(stderr, "git ls-tree") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// wordsOf is git's words of a command that failed, else the error's own.
func TestWordsOf(t *testing.T) {
	if got := wordsOf(errors.New("no git")); got != "no git" {
		t.Errorf("wordsOf = %q", got)
	}
	if got := wordsOf(fmt.Errorf("wrapped: %w", &vcsport.Failed{Words: " refused \n"})); got != "refused" {
		t.Errorf("wordsOf of a command that failed = %q", got)
	}
}
