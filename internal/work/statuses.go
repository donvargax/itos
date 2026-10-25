package work

import (
	"context"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/value"
)

// Statuses are each item's status by its id (work.ts's itemStatuses), read
// from the registry alone: no people, and nothing judged, so `task list`
// works over a registry with problems, and later the commit-msg hook reads a
// task's status through it. An item that gives no status has none.
type Statuses struct{ byID map[string]any }

// ItemStatuses reads the registry at path in src. A registry that is not there or
// cannot be read, whose items are not a list, or one of whose items is null,
// gives none, as the TypeScript's read of it throws and is caught. The first
// item of an id wins, since a duplicate is a registry problem.
func ItemStatuses(src port.Source, path string) Statuses {
	none := Statuses{map[string]any{}}
	text, err := src.Read(context.Background(), path)
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

// IDsAt are the ids of the registry's items at path in a tree: a commit,
// "index" for the staged tree, or "" for the working tree (slice 63, the
// footer whose source is the registry). A tree without the registry, or one
// that does not read as a list of items, has none; an id that is not text is
// no item's.
func IDsAt(trees port.Trees, path, tree string) (map[string]bool, error) {
	list, err := itemsAt(trees, path, tree)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, item := range list {
		if id, ok := value.Prop(item, "id").(string); ok {
			ids[id] = true
		}
	}
	return ids, nil
}

// TitleAt is the title of the registry's item id at path in a tree (IDsAt
// names the trees), each run of space in it one space and none at either end
// (slice 184): "" for a tree that cannot be opened, one without the
// registry, one that does not read as a list of items, an item it lacks, or a
// title that is no text or is empty once trimmed.
func TitleAt(trees port.Trees, path, tree, id string) string {
	list, _ := itemsAt(trees, path, tree)
	for _, item := range list {
		if value.Prop(item, "id") == id {
			title, _ := value.Prop(item, "title").(string)
			return value.Trim(value.Collapse(title))
		}
	}
	return ""
}

// itemsAt are the entries of the registry's items list at path in a tree,
// none for a tree without the registry or one that does not read as a list of
// items; an error when the tree cannot be opened.
func itemsAt(trees port.Trees, path, tree string) ([]any, error) {
	ctx := context.Background()
	if tree == "" {
		tree = "worktree"
	}
	from, err := trees.At(ctx, tree)
	if err != nil {
		return nil, err
	}
	if !from.Has(ctx, path) {
		return nil, nil
	}
	text, err := from.Read(ctx, path)
	if err != nil {
		return nil, nil
	}
	raw, err := value.Parse(text)
	if err != nil {
		return nil, nil
	}
	list, _ := value.Prop(raw, "items").([]any)
	return list, nil
}
