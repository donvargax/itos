package work

import (
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/value"
)

// Statuses are each item's status by its id (work.ts's itemStatuses), read
// from the registry alone: no people, and nothing judged, so `task list`
// works over a registry with problems, and later the commit-msg hook reads a
// task's status through it. An item that gives no status has none.
type Statuses struct{ byID map[string]any }

// ItemStatuses reads the registry at path. A registry that is not there or
// cannot be read, whose items are not a list, or one of whose items is null,
// gives none, as the TypeScript's read of it throws and is caught. The first
// item of an id wins, since a duplicate is a registry problem.
func ItemStatuses(path string) Statuses {
	none := Statuses{map[string]any{}}
	text, err := source.Read(path)
	if err != nil {
		return none
	}
	raw, err := value.Parse(text)
	if err != nil {
		return none
	}
	items := value.Prop(raw, "items")
	if items == nil || items == value.Undefined {
		return none
	}
	list, ok := items.([]any)
	if !ok {
		return none
	}
	found := map[string]any{}
	for _, item := range list {
		if item == nil {
			return none
		}
		id := key(value.Prop(item, "id"))
		if _, seen := found[id]; !seen {
			found[id] = value.Prop(item, "status")
		}
	}
	return Statuses{found}
}

// Of is the status of the item whose id is the given one, and whether it has
// one: none when the registry has no such item or the item gives null or no
// status (`?? "no item"`).
func (s Statuses) Of(id string) (any, bool) {
	status, ok := s.byID[key(id)]
	if !ok || status == nil || status == value.Undefined {
		return nil, false
	}
	return status, true
}
