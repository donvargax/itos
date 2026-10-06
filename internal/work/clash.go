package work

import "github.com/donvargax/itos/v6/internal/value"

// Clash is an item two commits changed from the same registry (slice 66):
// its id, and as the other side has it, its owner ("" for none) and whether
// it is in progress there, taken.
type Clash struct {
	ID    string
	Owner string
	Taken bool
}

// Clashes are the items both theirs and mine changed from base, three texts
// of the registry, in mine's order: what itos push names when its rebase
// stops on a conflict in the registry, which is how two takes of one item
// meet. None when a text does not read as a registry.
func Clashes(base, theirs, mine string) []Clash {
	was, _, ok1 := itemsByID(base)
	now, _, ok2 := itemsByID(theirs)
	ours, order, ok3 := itemsByID(mine)
	if !ok1 || !ok2 || !ok3 {
		return nil
	}
	changed := func(a, b *value.Map) bool {
		if a == nil || b == nil {
			return a != b
		}
		return value.JSON(a) != value.JSON(b)
	}
	var found []Clash
	for _, id := range order {
		if !changed(was[id], ours[id]) || !changed(was[id], now[id]) || !changed(now[id], ours[id]) {
			continue
		}
		c := Clash{ID: id}
		if item := now[id]; item != nil {
			if owner, ok := item.At("owner").(string); ok {
				c.Owner = owner
			}
			c.Taken = item.At("status") == "doing"
		}
		found = append(found, c)
	}
	return found
}

// itemsByID are a registry text's items by their ids, the ids in its order,
// and whether the text read as YAML.
func itemsByID(text string) (map[string]*value.Map, []string, bool) {
	raw, err := value.Parse(text)
	if err != nil {
		return nil, nil, false
	}
	list, _ := value.Prop(raw, "items").([]any)
	byID := map[string]*value.Map{}
	var order []string
	for _, entry := range list {
		if item, ok := entry.(*value.Map); ok && !absent(item.At("id")) {
			id := value.String(item.At("id"))
			byID[id] = item
			order = append(order, id)
		}
	}
	return byID, order, true
}
