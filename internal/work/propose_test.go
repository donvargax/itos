package work

import (
	"testing"

	"github.com/donvargax/itos/v2/internal/value"
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
