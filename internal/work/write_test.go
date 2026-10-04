package work

import (
	"regexp"
	"strings"
	"testing"

	"github.com/donvargax/itos/v2/internal/value"
)

// registryOf is the registry as Load gives it, from its text, with people
// to hold owners to when logins are given.
func registryOf(t *testing.T, text string, logins ...string) Registry {
	t.Helper()
	raw, err := value.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	r := Registry{Phases: value.Prop(raw, "phases").(*value.Map), Logins: logins, People: len(logins) > 0}
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
	change, problem, err := Promote(r, text, "i", "T-9", "task", regexp.MustCompile(`^T-\d+$`))
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	want := "  - { id: T-9, title: I, phase: 1, status: todo, kind: task, why: 'Was i. a gap' }\n" +
		"  - { id: s, title: S, phase: 1, status: todo, depends_on: [T-9] }\n"
	if !strings.HasSuffix(change.Text, want) || strings.Join(change.Rewritten, " ") != "s" ||
		change.Header != "docs: promote i to T-9" {
		t.Errorf("promote i: %q %v %q", change.Text, change.Rewritten, change.Header)
	}
	if _, problem, _ := Promote(r, text, "i", "s", "slice", nil); problem == nil || problem.Rule != "work-promote-taken" {
		t.Errorf("s is taken: %+v", problem)
	}
	if _, problem, _ := Promote(r, text, "i", "nine", "task", regexp.MustCompile(`^T-\d+$`)); problem == nil || problem.Rule != "work-promote-not-task-id" {
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

func TestWrap(t *testing.T) {
	if got := Wrap("one two three four", 9); got != "one two\nthree\nfour" {
		t.Errorf("%q", got)
	}
}
