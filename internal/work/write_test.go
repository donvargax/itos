package work

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/value"
)

// registryOf is the registry as Load gives it, from its text, with people
// to hold owners to when logins are given.
func registryOf(t *testing.T, text string, logins ...string) Registry {
	t.Helper()
	raw, err := value.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	r := Registry{Phases: value.Prop(raw, "phases").(*value.Map), Logins: logins, People: len(logins) > 0,
		Queue: value.Prop(raw, "queue")}
	for _, item := range value.Prop(raw, "items").([]any) {
		m := value.Copy(item).(*value.Map)
		if m.At("depends_on") == value.Undefined {
			m.Set("depends_on", []any{})
		}
		r.Items = append(r.Items, m)
	}
	return r
}

func TestTake(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: todo }\n" +
		"  - { id: b, title: B, phase: 1, owner: q, status: todo, depends_on: [a] }\n"
	r := registryOf(t, text)
	change, problem, err := Take(r, text, "a", "p", false)
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	if !strings.Contains(change.Text, "{ id: a, title: A, phase: 1, owner: p, status: doing }") ||
		change.Header != "docs: take a" || change.Body != `Set a ("A") to doing for p, with itos work take.` {
		t.Errorf("take a: %+v", change)
	}
	if _, problem, _ := Take(r, text, "b", "p", false); problem == nil || problem.Rule != "work-take-owned" {
		t.Errorf("b is q's: %+v", problem)
	}
	// A session that owns every item takes q's, its owner kept, and waits as
	// anyone does.
	if _, problem, _ := Take(r, text, "b", "", true); problem == nil || problem.Rule != "work-take-waiting" {
		t.Errorf("b waits on a: %+v", problem)
	}
	done := strings.Replace(text, "owner: null, status: todo", "owner: null, status: done", 1)
	change, problem, err = Take(registryOf(t, done), done, "b", "", true)
	if err != nil || problem != nil || change.Item.At("owner") != "q" || change.Item.At("status") != "doing" {
		t.Errorf("every item's: %+v, %v, %v", change, problem, err)
	}
}

func TestPromote(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: i, title: I, phase: 1, status: todo, kind: idea, why: 'a gap' }\n" +
		"  - { id: s, title: S, phase: 1, status: todo, depends_on: [i] }\n"
	r := registryOf(t, text)
	change, problem, err := Promote(r, text, "i", "T-9", "task", "", regexp.MustCompile(`^T-\d+$`))
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	want := "  - { id: T-9, title: I, phase: 1, status: todo, kind: task, why: 'Was i. a gap' }\n" +
		"  - { id: s, title: S, phase: 1, status: todo, depends_on: [T-9] }\n"
	if !strings.HasSuffix(change.Text, want) || strings.Join(change.Rewritten, " ") != "s" ||
		change.Header != "docs: promote i to T-9" {
		t.Errorf("promote i: %q %v %q", change.Text, change.Rewritten, change.Header)
	}
	// A title given replaces the idea's, and the body says so (slice 65).
	change, problem, err = Promote(r, text, "i", "slice-2", "slice", "I, specified", nil)
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	if !strings.Contains(change.Text, "{ id: slice-2, title: \"I, specified\", phase: 1, status: todo, kind: slice,") ||
		change.Item.At("title") != "I, specified" ||
		change.Body != `Make the idea i ("I") the slice slice-2, its title now "I, specified", with itos work promote, and rename it where s depends on it.` {
		t.Errorf("promote i --title: %q %q", change.Text, change.Body)
	}
	if _, problem, _ := Promote(r, text, "i", "s", "slice", "", nil); problem == nil || problem.Rule != "work-promote-taken" {
		t.Errorf("s is taken: %+v", problem)
	}
	if _, problem, _ := Promote(r, text, "i", "nine", "task", "", regexp.MustCompile(`^T-\d+$`)); problem == nil || problem.Rule != "work-promote-not-task-id" {
		t.Errorf("nine is no task ID: %+v", problem)
	}
}

func TestDone(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: s, title: S, phase: 1, owner: q, status: doing } # landing\n" +
		"  - { id: d, title: D, phase: 1, owner: q, status: done }\n" +
		"  - { id: i, title: I, phase: 1, status: todo, kind: idea }\n" +
		"  - { id: p, title: P, phase: 1, status: todo, deferred: later }\n"
	r := registryOf(t, text)
	change, problem, err := Done(r, text, "s")
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	if !strings.Contains(change.Text, "{ id: s, title: S, phase: 1, owner: q, status: done } # landing") ||
		change.Header != "docs: close s" || change.Body != `Set s ("S") to done, with itos work done.` ||
		change.Item.At("owner") != "q" {
		t.Errorf("done s: %+v", change)
	}
	if change, problem, _ := Done(r, text, "d"); problem != nil || !change.Unchanged {
		t.Errorf("d is done already: %+v %+v", change, problem)
	}
	for id, rule := range map[string]string{"i": "work-done-idea", "p": "work-done-deferred", "x": "work-unknown-item"} {
		if _, problem, _ := Done(r, text, id); problem == nil || problem.Rule != rule {
			t.Errorf("%s: %+v, not %s", id, problem, rule)
		}
	}
}

// Slice 76: the item closed loses its why, its index fields kept, and the
// commit's body says so.
func TestDoneDropsWhy(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n  - id: s\n    title: S\n    phase: 1\n    status: doing\n    kind: slice\n" +
		"    why: >\n      Because it\n      was missing.\n    refs: [features/s.feature]\n\n  - { id: t, title: T, phase: 1, status: todo }\n"
	change, problem, err := Done(registryOf(t, text), text, "s")
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	want := "phases: { 1: null }\nitems:\n  - id: s\n    title: S\n    phase: 1\n    status: done\n    kind: slice\n" +
		"    refs: [features/s.feature]\n\n  - { id: t, title: T, phase: 1, status: todo }\n"
	if change.Text != want || change.Item.Has("why") || !strings.Contains(change.Body, "Its why is dropped") {
		t.Errorf("done s:\n got %q\nwant %q\n%+v", change.Text, want, change)
	}
}

// A config of the registry at work-items.yaml, with no people, in a folder
// of its own.
func addConfig(t *testing.T) *config.Loaded {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("ITOS_CONFIG", "itos.yaml")
	if err := os.WriteFile("itos.yaml", []byte("version: 1\nwork: { registry: work-items.yaml }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("itos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestAdd(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 0: null, 1: null }\nitems:\n" +
		"  - id: a\n    title: A\n    phase: 0\n    owner: null\n    status: done\n    depends_on: []\n\n" +
		"  - id: b\n    title: B\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: [a]\n"
	r := registryOf(t, text)
	n := New{ID: "p1-c", Title: "C: the third", Why: "Because.", Kind: "idea", DependsOn: []string{"b"}, Refs: []string{"x.md"}}
	change, found, err := Add(cfg, r, text, n, nil)
	if err != nil || found != nil {
		t.Fatal(err, found)
	}
	want := text + "\n  - id: p1-c\n    title: \"C: the third\"\n    phase: 1\n    owner: null\n    status: todo\n" +
		"    depends_on: [b]\n    kind: idea\n    why: >\n      Because.\n    refs: [x.md]\n"
	if change.Text != want || change.Header != "docs: add p1-c" ||
		change.Body != `Add the idea p1-c ("C: the third") to phase 1, with itos work add.` {
		t.Errorf("add p1-c:\n got %q\nwant %q\n%s", change.Text, want, change.Body)
	}
	for name, c := range map[string]struct {
		n    New
		rule string
	}{
		"taken":        {New{ID: "b", Title: "B", Why: "w", Kind: "idea"}, "work-add-taken"},
		"no phase":     {New{ID: "slice-3", Title: "S", Why: "w", Kind: "slice"}, "work-add-no-phase"},
		"no such dep":  {New{ID: "p1-d", Title: "D", Why: "w", Kind: "idea", DependsOn: []string{"z"}}, "work-unknown-dependency"},
		"no such kind": {New{ID: "p1-d", Title: "D", Why: "w", Kind: "epic"}, "work-unknown-kind"},
		"not a task":   {New{ID: "task-1", Title: "T", Why: "w", Kind: "task", Phase: "1"}, "work-add-not-task-id"},
	} {
		_, found, err := Add(cfg, r, text, c.n, regexp.MustCompile(`^T-\d+$`))
		if err != nil || len(found) == 0 || found[0].Rule != c.rule {
			t.Errorf("%s: %+v, %v, not %s", name, found, err, c.rule)
		}
	}
}

func TestEdit(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: done }\n" +
		"  - id: b\n    title: B\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: []\n    why: >\n      A reason.\n"
	r := registryOf(t, text)
	title, deps := "Bee", []string{"a"}
	change, found, err := Edit(cfg, r, text, "b", Edits{Title: &title, DependsOn: &deps, Note: "And more."})
	if err != nil || found != nil {
		t.Fatal(err, found)
	}
	want := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: done }\n" +
		"  - id: b\n    title: Bee\n    phase: 1\n    owner: null\n    status: todo\n    depends_on: [a]\n    why: >\n      A reason.\n\n      And more.\n"
	if change.Text != want || change.Header != "docs: edit b" ||
		change.Body != `Change b ("Bee"): its title, its depends_on and a note on its why, with itos work edit.` {
		t.Errorf("edit b:\n got %q\nwant %q\n%s", change.Text, want, change.Body)
	}
	if change, _, _ := Edit(cfg, r, text, "b", Edits{Title: new("B")}); !change.Unchanged {
		t.Errorf("b is already titled B: %+v", change)
	}
	// b has no refs: an empty list given is already so (bug 14).
	if change, _, _ := Edit(cfg, r, text, "b", Edits{Refs: &[]string{}}); !change.Unchanged {
		t.Errorf("b already has no refs: %+v", change)
	}
	// a is done, so it cannot wait on b, which is not.
	if _, found, _ := Edit(cfg, r, text, "a", Edits{DependsOn: &[]string{"b"}}); len(found) == 0 || found[0].Rule != "work-done-before-dependency" {
		t.Errorf("a done before b: %+v", found)
	}
	if _, found, _ := Edit(cfg, r, text, "z", Edits{Note: "x"}); len(found) == 0 || found[0].Rule != "work-unknown-item" {
		t.Errorf("no z: %+v", found)
	}
}
