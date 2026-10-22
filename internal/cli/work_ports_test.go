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
	if code, err := runWork([]string{"take"}, Out{}); code != 0 || err == nil || err.Error() != "work take needs one <id>" {
		t.Errorf("runWork take = %d, %v, want the usage error alone", code, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code, err := runWork([]string{"take", "nope"}, Out{Stdout: &stdout, Stderr: &stderr, OS: runtime.GOOS}); code != ExitPolicy || err != nil ||
		!strings.Contains(stderr.String(), "nope") {
		t.Errorf("runWork take nope = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
}

// Every subcommand not yet in the slice is its function here; the ones the
// slice answers are none.
func TestUnmovedNamesTheSubcommandsStillHere(t *testing.T) {
	for sub, want := range map[string]command{
		"take": workTake, "promote": workPromote, "done": workDone, "add": workAdd, "edit": workEdit,
		"queue": workQueue, "drop": workDrop, "defer": workDefer, "resume": workResume, "give-back": workGiveBack,
	} {
		if got := unmoved(sub); got == nil || reflect.ValueOf(got).Pointer() != reflect.ValueOf(want).Pointer() {
			t.Errorf("unmoved(%q) is not its function", sub)
		}
	}
	for _, sub := range []string{"", "list", "check", "show", "nope"} {
		if unmoved(sub) != nil {
			t.Errorf("unmoved(%q) is a function", sub)
		}
	}
}
