package work

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v6/internal/value"
)

// RegistryHeader reads the headers the registry's writers commit with, so
// work show finds an item's take, promotion and close by them.
func TestRegistryHeaderReadsTheWritersHeaders(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: todo }\n" +
		"  - { id: i, title: I, phase: 1, owner: null, status: todo, kind: idea }\n"
	r := registryOf(t, text)
	take, _, err := Take(r, text, "a", "p", false)
	if err != nil {
		t.Fatal(err)
	}
	done, _, err := Done(r, text, "a")
	if err != nil {
		t.Fatal(err)
	}
	promote, _, err := Promote(r, text, "i", "slice-1", "slice", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	queue, _, err := Queue(r, text, "a", Place{Top: true})
	if err != nil {
		t.Fatal(err)
	}
	for header, want := range map[string]string{
		take.Header: "a", done.Header: "a", promote.Header: "slice-1", queue.Header: "a",
		"docs: add T-1": "T-1", "docs: edit p1-x": "p1-x", "docs: defer p1-x": "p1-x", "docs: resume p1-x": "p1-x",
	} {
		if id, ok := RegistryHeader(header); !ok || id != want {
			t.Errorf("%q names %q, %v; want %q", header, id, ok, want)
		}
	}
	for _, header := range []string{"docs: take", "docs: take a b", "chore: take a", "docs: promote i slice-1", "docs: describe a"} {
		if id, ok := RegistryHeader(header); ok {
			t.Errorf("%q names %q; want none", header, id)
		}
	}
}

func TestShowNamesTheItemsDependingOnIt(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: todo }\n" +
		"  - { id: b, title: B, phase: 1, owner: q, status: todo, depends_on: [a] }\n" +
		"  - { id: c, title: C, phase: 1, owner: q, status: todo, depends_on: [b, a], why: \"Because.\" }\n"
	r := registryOf(t, text)
	shown, problem := Show(r, "a")
	if problem != nil || !slices.Equal(shown.DependedOnBy, []string{"b", "c"}) {
		t.Errorf("a: %+v %+v", shown, problem)
	}
	if _, problem := Show(r, "z"); problem == nil || problem.Rule != "work-unknown-item" {
		t.Errorf("z: %+v", problem)
	}
	var b strings.Builder
	shown, _ = Show(r, "c")
	shown.Print(&b, 100)
	want := "c  C\n  kind: -, status: todo, owner: q, phase: 1\n  depends on: b, a\n\n  Because.\n"
	if b.String() != want {
		t.Errorf("c prints\n%s\nwant\n%s", b.String(), want)
	}
}

// Where an item's why lives (slice 76): its kind says, else its id, a task's
// only where ledger.id is given.
func TestSpecKind(t *testing.T) {
	taskID := regexp.MustCompile(`^T-\d+$`)
	for _, c := range []struct {
		item   *value.Map
		taskID *regexp.Regexp
		want   string
	}{
		{value.NewMap("id", "x", "kind", "slice"), taskID, "slice"},
		{value.NewMap("id", "T-1", "kind", "task"), nil, "task"},
		{value.NewMap("id", "slice-3", "kind", "idea"), taskID, ""},
		{value.NewMap("id", "slice-3"), nil, "slice"},
		{value.NewMap("id", "T-1"), taskID, "task"},
		{value.NewMap("id", "T-1"), nil, ""},
		{value.NewMap("id", "p1-thing"), taskID, ""},
	} {
		if got := SpecKind(c.item, c.taskID); got != c.want {
			t.Errorf("%s: %q, not %q", value.JSON(c.item), got, c.want)
		}
	}
}
