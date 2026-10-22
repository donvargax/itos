package cli

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/store/tracker/adapter"
)

// workPorts bind the work slice to the real code: the config the working
// tree has, the working tree, work.identity's provider and the tracker of
// the config on the OS itos runs on; runWork reads the line, runs the slice
// over them for what it answers and hands the rest to the subcommands still
// here.
func TestWorkPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork:\n  registry: work-items.yaml\n  identity: { provider: none, hint: say --as }\n")
	p := workPorts{Out{OS: runtime.GOOS}}
	cfg, err := p.Config(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("Config = %+v, %v, want the working tree's config", cfg, err)
	}
	if tree := p.Worktree(t.Context()); !tree.Has(t.Context(), "itos.yaml") || tree.Tree(t.Context()) != "worktree" {
		t.Errorf("Worktree = %#v, want the working tree", tree)
	}
	if answer := p.Identity(t.Context(), cfg)(); !strings.Contains(answer.Problem, "say --as") {
		t.Errorf("Identity() = %+v, want work.identity none's", answer)
	}
	files, ok := p.Tracker(t.Context(), cfg).(adapter.Files)
	if !ok || files.OS != runtime.GOOS || files.Lock != "work-items.yaml" || files.Stealth {
		t.Errorf("Tracker = %#v, want the files adapter of the config on this OS", files)
	}
	if err := os.WriteFile("work-items.yaml", []byte("phases: { 1: null }\nitems:\n  - { id: one, title: One, phase: 1, owner: null, status: todo, depends_on: [] }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code, err := runWork([]string{"list"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.Contains(stdout.String(), "one") || !strings.Contains(stdout.String(), "One") {
		t.Errorf("runWork list = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if code, err := runWork([]string{"show", "a", "b"}, Out{}); code != 0 || err == nil || err.Error() != "work show needs one <id>" {
		t.Errorf("runWork show a b = %d, %v, want the usage error alone", code, err)
	}
	// A subcommand still here is the slice's line first, then its own.
	if code, err := runWork([]string{"promote"}, Out{}); code != 0 || err == nil || err.Error() != "work promote needs one <id>" {
		t.Errorf("runWork promote = %d, %v, want the usage error alone", code, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code, err := runWork([]string{"promote", "nope", "--kind", "slice"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != ExitPolicy || err != nil ||
		!strings.Contains(stderr.String(), "nope") {
		t.Errorf("runWork promote nope = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
}

// A write the slice answers holds, reads and commits the registry through
// the tracker of the config, and ItemID is newItemID: the id a draft
// reserved, else one minted from the repository's counter.
func TestWorkPortsWriteThroughTheRealCode(t *testing.T) {
	registryWriterRepo(t, "phases: { 1: null }\nitems:\n  - { id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n")
	var stdout, stderr strings.Builder
	if code, err := runWork([]string{"take", "slice-9"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != 0 || err != nil ||
		!strings.HasPrefix(stdout.String(), "slice-9 is in progress: committed ") {
		t.Errorf("runWork take slice-9 = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
	if header := strings.TrimSpace(gitIn(t, "log", "-1", "--format=%s")); header != "docs: take slice-9" {
		t.Errorf("the take committed %q", header)
	}
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if id, err := (workPorts{Out{OS: runtime.GOOS, reserved: "slice-77"}}).ItemID(t.Context(), cfg, "slice", []string{"slice-9"}); id != "slice-77" || err != nil {
		t.Errorf("ItemID with a reserved id = %q, %v", id, err)
	}
	if id, err := (workPorts{Out{OS: runtime.GOOS}}).ItemID(t.Context(), cfg, "slice", []string{"slice-9"}); id != "slice-10" || err != nil {
		t.Errorf("ItemID minted = %q, %v", id, err)
	}
}

// draft reads a work add line with the slice's AddLine.
func TestWorkAddLineIsTheSlices(t *testing.T) {
	if n, err := workAddLine([]string{"--kind", "bug", "--title", "T", "--why", "W"}); err != nil || n.Kind != "bug" || n.ID != "" {
		t.Errorf("workAddLine = %+v, %v", n, err)
	}
	if _, err := workAddLine([]string{"a", "b"}); err == nil || err.Error() != "work add takes at most one <id>" {
		t.Errorf("workAddLine a b = %v", err)
	}
}

// Every subcommand not yet in the slice is its function here; the ones the
// slice answers are none.
func TestUnmovedNamesTheSubcommandsStillHere(t *testing.T) {
	for sub, want := range map[string]command{"promote": workPromote, "done": workDone} {
		if got := unmoved(sub); got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			t.Errorf("unmoved(%q) is not its function", sub)
		}
	}
	for _, sub := range []string{"", "list", "check", "show", "take", "add", "edit", "queue", "drop", "defer", "resume", "give-back", "nope"} {
		if unmoved(sub) != nil {
			t.Errorf("unmoved(%q) is a function", sub)
		}
	}
}
