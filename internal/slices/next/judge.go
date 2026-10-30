package next

import (
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// Next is an item's state and next step, as next prints it: the Item's id,
// its Kind ("" for none), its State, its Owner as written (nil for null or
// none), the Actor and Action of the step, the Argv of the itos command the
// step runs when it is a deterministic one, WaitsOn, what the state waits
// on, BlockedReason, "decision" or "status" on a blocked item, and the
// Revision of the facts it was judged from.
type Next struct {
	Item          string
	Kind          string
	State         string
	Owner         any
	Actor, Action string
	Argv          []string
	WaitsOn       []string
	BlockedReason string
	Revision      string
}

// Judge is the item's state and next step from its facts: the first state
// that holds wins, in the order done, parked, blocked (an open question
// before the registry's status), waiting, idea, needs-spec, in-flight.
func Judge(f Facts) Next {
	n := Next{Item: value.String(f.Item.At("id")), Kind: f.Kind, Revision: Revision(f)}
	if owner := f.Item.At("owner"); owner != value.Undefined {
		n.Owner = owner
	}
	status := f.Item.At("status")
	waits := undone(f.Dependencies)
	switch {
	case status == "done":
		n.step("done", "none", "none")
	case status == work.Dropped || f.Item.At("deferred") != value.Undefined:
		n.step("parked", "none", "none")
	case len(f.Questions) > 0:
		n.step("blocked", "person", "answer")
		n.BlockedReason = "decision"
		for _, q := range f.Questions {
			n.WaitsOn = append(n.WaitsOn, q.ID)
		}
	case status == "blocked":
		n.step("blocked", "person", "answer")
		n.BlockedReason = "status"
		n.WaitsOn = blockedBy(f.Item)
	case waits != nil:
		n.step("waiting", "none", "wait")
		n.WaitsOn = waits
	case f.Kind != "slice" && f.Kind != "bug" && f.Kind != "task":
		n.step("idea", "person", "promote")
	case len(f.Scenarios) == 0 && len(f.Checks) == 0:
		n.step("needs-spec", "drafter", "specify")
		n.Argv = []string{"itos", "brief", n.Item, "--role", "drafter"}
	default:
		n.step("in-flight", "forge", "observe")
	}
	return n
}

func (n *Next) step(state, actor, action string) {
	n.State, n.Actor, n.Action = state, actor, action
}

// blockedBy are the ids the item's blocked_by lists, none when it is prose
// (why, in words, as the registry's header describes it) or absent.
func blockedBy(item *value.Map) []string {
	listed, _ := item.At("blocked_by").([]any)
	var ids []string
	for _, id := range listed {
		ids = append(ids, value.String(id))
	}
	return ids
}

// undone are the ids of the dependencies not done, in the depends_on's
// order; nil for none.
func undone(deps []Dependency) []string {
	var ids []string
	for _, d := range deps {
		if !d.Done() {
			ids = append(ids, d.ID)
		}
	}
	return ids
}
