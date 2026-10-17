package work

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/value"
)

// A session that owns every item (a stealth config's) is proposed each item
// that can start as its own, whoever the item or its group names, nothing as
// unowned; what waits still waits, and ideas and deferred items stay apart.
func TestProposeEvery(t *testing.T) {
	item := func(id, owner, status string, more ...any) *value.Map {
		var o any
		if owner != "" {
			o = owner
		}
		return value.NewMap(append([]any{"id", id, "title", id, "phase", 1, "owner", o, "status", status,
			"depends_on", []any{}}, more...)...)
	}
	r := Registry{Phases: value.NewMap("1", "z"), Items: []*value.Map{
		item("theirs", "q", "todo"),
		item("phased", "", "todo"),
		item("busy", "q", "doing"),
		item("later", "q", "todo", "depends_on", []any{"theirs"}),
		item("idea", "q", "todo", "kind", "idea"),
		item("parked", "", "todo", "deferred", "not now"),
	}}
	ids := func(list []*value.Map) []string {
		var all []string
		for _, i := range list {
			all = append(all, value.String(i.At("id")))
		}
		return all
	}
	p := ProposeEvery(r)
	if !p.Every || p.Person != "" {
		t.Errorf("the session is %q, every item %v", p.Person, p.Every)
	}
	if got := ids(p.Next); len(got) != 2 || got[0] != "theirs" || got[1] != "phased" {
		t.Errorf("can start: %v", got)
	}
	if len(p.Unowned) != 0 || len(ids(p.Doing)) != 1 || len(p.Waiting) != 1 || len(p.Ideas) != 1 ||
		len(p.Deferred) != 1 {
		t.Errorf("the proposal is %+v", p)
	}
	if fields := p.Fields(); fields[1].Key != "every_item" || fields[1].Value != true {
		t.Errorf("the fields are %v", fields)
	}
	if fields := Propose(r, "q").Fields(); fields[1].Key != "doing" {
		t.Errorf("a handle's fields are %v", fields)
	}
}

// What a person can start, theirs and the unowned together, comes in the
// queue's one order, the unqueued after in the registry's; another's item and
// one that waits are left out.
func TestStartable(t *testing.T) {
	item := func(id string, owner any, deps ...any) *value.Map {
		return value.NewMap("id", id, "title", id, "phase", 1, "owner", owner, "status", "todo",
			"depends_on", append([]any{}, deps...))
	}
	r := Registry{Phases: value.NewMap("1", nil), Queue: []any{"free-b", "mine-b", "other"}, Items: []*value.Map{
		item("mine-a", "q"),
		item("free-a", nil),
		item("mine-b", "q"),
		item("other", "z"),
		item("free-b", nil),
		item("waits", "q", "free-a"),
	}}
	var got []string
	for _, i := range Startable(r, Propose(r, "q")) {
		got = append(got, value.String(i.At("id")))
	}
	want := []string{"free-b", "mine-b", "mine-a", "free-a"}
	if len(got) != len(want) {
		t.Fatalf("startable: %v, not %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("startable: %v, not %v", got, want)
		}
	}
}

// An item whose dependencies are all done can start; one with a dependency
// not done waits on it.
func TestProposeStartsWhatDoneItemsFree(t *testing.T) {
	item := func(id, status string, deps ...any) *value.Map {
		return value.NewMap("id", id, "title", id, "phase", 1, "owner", "q", "status", status,
			"depends_on", append([]any{}, deps...))
	}
	r := Registry{Phases: value.NewMap("1", nil), Items: []*value.Map{
		item("done", "done"),
		item("open", "todo"),
		item("freed", "todo", "done"),
		item("held", "todo", "done", "open"),
	}}
	p := Propose(r, "q")
	var next []string
	for _, i := range p.Next {
		next = append(next, value.String(i.At("id")))
	}
	if len(next) != 2 || next[0] != "open" || next[1] != "freed" {
		t.Errorf("can start: %v", next)
	}
	if len(p.Waiting) != 1 || value.String(p.Waiting[0].Item.At("id")) != "held" || len(p.Waiting[0].On) != 1 ||
		p.Waiting[0].On[0] != "open" {
		t.Errorf("waiting: %+v", p.Waiting)
	}
}
