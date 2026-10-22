package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// promoteRepo is a registryWriterRepo with an idea, p1-idea, and a ledger
// file of phase 1 holding T-001, committed.
func promoteRepo(t *testing.T) string {
	t.Helper()
	dir := registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks", "phase-1.yaml"), []byte("- { id: T-001, type: chore, title: One }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := "version: 1\nledger:\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\n" +
		"commits: { types: [build, chore, docs] }\nwork:\n  registry: registry.yaml\n"
	if err := os.WriteFile(filepath.Join(dir, "itos.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "tasks", "itos.yaml")
	gitIn(t, "commit", "-q", "-m", "docs: a ledger")
	return dir
}

// unpromoted holds that a refused promote left HEAD as it was, the working
// tree clean and, when minted is false, the id counter untouched.
func unpromoted(t *testing.T, head string, minted bool) {
	t.Helper()
	if now := gitIn(t, "rev-parse", "HEAD"); now != head {
		t.Error("the refused promote moved HEAD")
	}
	if status := gitIn(t, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Errorf("the refused promote left git status reporting:\n%s", status)
	}
	_, err := os.Stat(filepath.Join(".git", "itos", "ids.yaml"))
	if reserved := !errors.Is(err, fs.ErrNotExist); reserved != minted {
		t.Errorf("an id was reserved: %t, want %t (%v)", reserved, minted, err)
	}
}

// A ledger that cannot be read is an error, before any id is reserved.
func TestWorkPromoteTaskWithoutALedgerFolderIsAnError(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 1, owner: null, status: todo, kind: idea, why: Specified }\n")
	head := gitIn(t, "rev-parse", "HEAD")
	code, err := workPromote([]string{"p1-idea", "--kind", "task", "--type", "build", "--check", "true"}, Out{})
	if code != 0 || err == nil {
		t.Fatalf("workPromote = (%d, %v), want exit code 0 and the ledger's error", code, err)
	}
	unpromoted(t, head, false)
}

// What ledger.Add refuses of the task, a type commits.types does not list,
// is refused, named, before any id is reserved.
func TestWorkPromoteRefusesTheTasksTypeBeforeMinting(t *testing.T) {
	promoteRepo(t)
	head := gitIn(t, "rev-parse", "HEAD")
	var stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "task", "--type", "undeclared", "--check", "true"}, Out{Stderr: &stderr})
	if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), "undeclared is not a commit type") {
		t.Fatalf("workPromote = (%d, %v), stderr=%q, want the type's refusal", code, err, stderr.String())
	}
	unpromoted(t, head, false)
}

// itos's commit-msg hook not declared stops the promote before any id is
// reserved: a missing environment.
func TestWorkPromoteWithoutHooksReservesNothing(t *testing.T) {
	promoteRepo(t)
	gitIn(t, "config", "--remove-section", "hook.itos-commit-msg")
	head := gitIn(t, "rev-parse", "HEAD")
	code, err := workPromote([]string{"p1-idea", "--kind", "slice"}, Out{})
	if code != 0 || kind.Of(err) != kind.Missing {
		t.Fatalf("workPromote = (%d, %v), want exit code 0 and the missing hook", code, err)
	}
	unpromoted(t, head, false)
}

// What can only be refused once the id is known (a draft's reserved id here)
// is refused after the reservation, nothing written: an id the ledger
// already has, by ledger.Add, and one ledger.id does not match, by
// work.Promote.
func TestWorkPromoteRefusesAReservedIDAfterMinting(t *testing.T) {
	for _, c := range []struct{ id, says string }{
		{"T-001", "T-001 is already a task of the ledger"},
		{"nine", "nine is not a task ID by ledger.id"},
	} {
		t.Run(c.id, func(t *testing.T) {
			promoteRepo(t)
			head := gitIn(t, "rev-parse", "HEAD")
			var stderr strings.Builder
			code, err := workPromote([]string{"p1-idea", "--kind", "task", "--type", "build", "--check", "true"}, Out{Stderr: &stderr, reserved: c.id})
			if code != ExitPolicy || err != nil || !strings.Contains(stderr.String(), c.says) {
				t.Fatalf("workPromote = (%d, %v), stderr=%q, want %q", code, err, stderr.String(), c.says)
			}
			unpromoted(t, head, false)
		})
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

// Under a stealth config nothing is committed, and the line says which files
// are the config's: the ledger's too for a task.
func TestWorkPromoteUnderAStealthConfigSaysWhatIsNotCommitted(t *testing.T) {
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"p1-idea", "--kind", "task", "--type", "build", "--check", "true"},
			"p1-idea is the task T-002 of .git/itos/tasks/phase-1.yaml: the ledger and the registry are the stealth config's, so nothing is committed\n"},
		{[]string{"p1-idea", "--kind", "slice"},
			"p1-idea is the slice slice-1: the registry is the stealth config's, so nothing is committed\n"},
	} {
		t.Run(c.args[2], func(t *testing.T) {
			stealthPromoteRepo(t)
			var stdout strings.Builder
			code, err := workPromote(c.args, Out{Stdout: &stdout, Stderr: &strings.Builder{}})
			if code != 0 || err != nil || filepath.ToSlash(stdout.String()) != c.says {
				t.Fatalf("workPromote = (%d, %v), stdout=%q, want %q", code, err, stdout.String(), c.says)
			}
		})
	}
}

// promotedTask refuses an idea with no group, and one with no why when the
// line gives none; the line's --why stands in for the idea's.
func TestPromotedTaskNeedsAGroupAndAWhy(t *testing.T) {
	promoteRepo(t)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		t.Fatal(err)
	}
	p := promotion{id: "p1-idea", kind: "task", taskType: "build"}
	for _, c := range []struct {
		idea      *value.Map
		why, rule string
	}{
		{value.NewMap("id", "p1-idea", "title", "Idea", "why", "Specified"), "", "work-promote-no-group"},
		{value.NewMap("id", "p1-idea", "title", "Idea", "phase", 1.0), "", "work-promote-no-why"},
		{value.NewMap("id", "p1-idea", "title", "Idea", "phase", 1.0, "why", "  "), "", "work-promote-no-why"},
		{value.NewMap("id", "p1-idea", "title", "Idea", "phase", 1.0), "Given.", ""},
	} {
		p.why = c.why
		n, _, found, err := promotedTask(cfg, registry, c.idea, p, tracker(cfg, runtime.GOOS).Load)
		switch {
		case err != nil:
			t.Errorf("promotedTask(%v) error: %v", c.idea, err)
		case c.rule == "" && (len(found) != 0 || n.Why != "Given." || n.Group != "1"):
			t.Errorf("promotedTask(%v) = %+v, %+v, want the line's why in group 1", c.idea, n, found)
		case c.rule != "" && (len(found) != 1 || found[0].Rule != c.rule || !strings.Contains(found[0].Message, "p1-idea")):
			t.Errorf("promotedTask(%v) refused %+v, want %s", c.idea, found, c.rule)
		}
	}
}

// workPromoteLine's usage errors: one idea, each flag once and with a value,
// and a task's type and check; a line it takes gives the flags in order.
func TestWorkPromoteLine(t *testing.T) {
	task := []string{"--kind", "task", "--type", "build", "--check", "true"}
	for _, c := range []struct {
		args []string
		says string
	}{
		{task, "work promote needs one <id>"},
		{append([]string{"a", "b"}, task...), "work promote needs one <id>"},
		{append([]string{"a", "--kind", "task"}, task...), "work promote takes one --kind with a value"},
		{append([]string{"a", "--title", " "}, task...), "work promote takes one --title with a value"},
		{append([]string{"a", "--type", "docs"}, task...), "work promote takes one --type with a value"},
		{[]string{"a", "--kind", "slice", "--why", "Because."}, "work promote --kind slice does not take --why"},
		{[]string{"a", "--kind", "task", "--check", "true"}, "work promote --kind task needs --type <type> and a --check <command>"},
		{[]string{"a", "--bogus", "x"}, "work promote does not take --bogus"},
		{[]string{"a", "--kind"}, "work promote --kind needs a value"},
	} {
		if _, err := workPromoteLine(c.args); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("workPromoteLine(%q) = %v, want %q", c.args, err, c.says)
		}
	}
	p, err := workPromoteLine([]string{"a", "--kind=task", "--type", "build", "--title", "T", "--why", "W",
		"--check", "one", "--timeout", "9", "--check", "two"})
	if err != nil || p.id != "a" || p.kind != "task" || p.taskType != "build" || p.title != "T" || p.why != "W" ||
		len(p.checks) != 2 || p.checks[0].Run != "one" || *p.checks[0].Timeout != 9 || p.checks[1].Run != "two" || p.checks[1].Timeout != nil {
		t.Errorf("workPromoteLine = %+v, %v", p, err)
	}
}

// A ledger that changes between the check and the write, its new group's
// file made a folder once the id is being reserved, is an error after it.
func TestWorkPromoteLedgerErrorAfterMinting(t *testing.T) {
	dir := registryWriterRepo(t, "phases: { 1: null, 2: null }\nitems:\n"+
		"  - { id: p1-idea, title: Idea, phase: 2, owner: null, status: todo, kind: idea, why: Specified }\n")
	if err := os.MkdirAll(filepath.Join(dir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks", "phase-1.yaml"), []byte("- { id: T-001, type: chore, title: One }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "tasks")
	gitIn(t, "commit", "-q", "-m", "docs: a ledger")
	// The counter's store asks git for the remote's address, as nothing the
	// promote checks before does.
	useWrappedGit(t, "", "remote get-url", "mkdir "+filepath.Join(dir, "tasks", "phase-2.yaml"))
	head := gitIn(t, "rev-parse", "HEAD")
	var stderr strings.Builder
	code, err := workPromote([]string{"p1-idea", "--kind", "task", "--type", "build", "--check", "true"}, Out{Stderr: &stderr})
	if code != 0 || err == nil {
		t.Fatalf("workPromote = (%d, %v), stderr=%q, want the ledger's error", code, err, stderr.String())
	}
	unpromoted(t, head, true)
}
