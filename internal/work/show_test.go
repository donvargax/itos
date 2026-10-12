package work

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// RegistryHeader reads the headers the registry's writers commit with, so
// work show finds an item's take, promotion and close by them.
func TestRegistryHeaderReadsTheWritersHeaders(t *testing.T) {
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: null, status: todo }\n" +
		"  - { id: b, title: B, phase: 1, owner: null, status: doing }\n" +
		"  - { id: i, title: I, phase: 1, owner: null, status: todo, kind: idea }\n"
	r := registryOf(t, text)
	take, _, err := Take(r, text, "a", "p", false)
	if err != nil {
		t.Fatal(err)
	}
	done, _, err := Done(addConfig(t), r, text, "b")
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
		take.Header: "a", done.Header: "b", promote.Header: "slice-1", queue.Header: "a",
		"docs: add T-1": "T-1", "docs: edit p1-x": "p1-x", "docs: defer p1-x": "p1-x", "docs: resume p1-x": "p1-x",
		"docs: give back slice-2": "slice-2",
	} {
		if id, ok := RegistryHeader(header); !ok || id != want {
			t.Errorf("%q names %q, %v; want %q", header, id, ok, want)
		}
	}
	for _, header := range []string{"docs: take", "docs: take a b", "chore: take a", "docs: promote i slice-1", "docs: describe a", "docs: give back", "docs: give back a b", "docs: give away a"} {
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

// A registry why that reads as the ledger's, whitespace and blank lines
// aside, is the copy task add wrote until bug-107: work show gives it once.
// One that says something else, or beside no ledger why, stays.
func TestListedLeavesOutARegistryWhyRepeatingTheLedgers(t *testing.T) {
	item := func(why any) *value.Map { return value.NewMap("id", "T-1", "kind", "task", "why", why) }
	for _, c := range []struct {
		ledger   string
		registry any
		kept     bool
	}{
		{"The ledger\nsays why.\n", "The ledger says why.", true},
		{"One  para.\n\nTwo.\n", "  One para.\nTwo.", false},
		{"The ledger says why.\n", "The ledger says why.\n", false},
		{"The ledger says why.\n", "An idea's reasons.", true},
		{"", "The registry says why.", true},
		{"The ledger says why.\n", nil, true},
	} {
		shown := Shown{Item: item(c.registry), LedgerWhy: c.ledger}
		listed := shown.Listed()
		if listed.Has("why") != c.kept {
			t.Errorf("ledger %q, registry %#v: why kept %v, want %v", c.ledger, c.registry, listed.Has("why"), c.kept)
		}
		if !shown.Item.Has("why") {
			t.Errorf("ledger %q, registry %#v: Listed changed the registry's item", c.ledger, c.registry)
		}
	}
	var b strings.Builder
	Shown{Item: item("The  ledger says why."), LedgerWhy: "The ledger says why.\n"}.Print(&b, 100)
	if got := strings.Count(b.String(), "says why."); got != 1 {
		t.Errorf("prints the why %d times:\n%s", got, b.String())
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
