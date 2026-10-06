package work

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/value"
)

// Drop is the registry with the item taken out of the open work (slice 78):
// its status dropped, its why dropped as Done drops a closed item's, and the
// item out of the queue when the queue holds it, the reason the body of the
// commit that records it, before what the change is: the commit is the
// record of why, the registry an index. The item stays in the registry, so
// its id is never given again. Refused, with nothing changed: an id no item
// has, an item done, an item that an item still to do (neither done nor
// dropped) depends on, naming those, so the person edits them first, and
// whatever work check would find in the registry with the item dropped. An
// item already dropped is Unchanged.
func Drop(cfg *config.Loaded, r Registry, text, id, reason string) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, []out.Problem, error) {
		return Change{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	switch item.At("status") {
	case Dropped:
		return Change{Item: item, Unchanged: true}, nil, nil
	case "done":
		return refuse("work-drop-done", id+" is done, and a done item stays done",
			"drop an item still to do (itos work list)")
	}
	if on := dependants(r, id); len(on) > 0 {
		return refuse("work-drop-depended-on",
			fmt.Sprintf("%s is depended on by %s, still to do", id, strings.Join(on, ", ")),
			fmt.Sprintf("take %s out of their depends_on first (itos work edit <id> --depends-on …), or drop them first", id))
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.Set([]any{"items", i, "status"}, Dropped); err != nil {
		return Change{}, nil, err
	}
	hasWhy := item.Has("why")
	if hasWhy {
		if err := doc.Drop([]any{"items", i, "why"}); err != nil {
			return Change{}, nil, err
		}
	}
	queued := slices.Contains(Queued(r), id)
	var queue []string
	if queued {
		queue = slices.DeleteFunc(Queued(r), func(q string) bool { return q == id })
		if err := doc.SetBlockList([]any{"queue"}, queue, "items"); err != nil {
			return Change{}, nil, err
		}
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	after.Set("status", Dropped)
	after.Delete("why")
	found, err := sound(cfg, r, i, after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	body := fmt.Sprintf("%s Set %s (%s) to dropped, with itos work drop", strings.TrimSpace(reason), id, value.JSON(value.String(item.At("title"))))
	switch {
	case hasWhy && queued:
		body += ", its why dropped and the item taken out of the queue"
	case hasWhy:
		body += ", its why dropped"
	case queued:
		body += ", the item taken out of the queue"
	}
	body += ": it stays in the registry, so its id is never given again."
	return Change{Text: edited, Item: after, Queue: queue, Header: "docs: drop " + id, Body: body}, nil, nil
}

// dependants are the ids of the items still to do (neither done nor
// dropped) whose depends_on names the id.
func dependants(r Registry, id string) []string {
	var on []string
	for _, other := range r.Items {
		if !live(other) {
			continue
		}
		deps, _ := other.At("depends_on").([]any)
		for _, dep := range deps {
			if !absent(dep) && value.String(dep) == id {
				on = append(on, value.String(other.At("id")))
				break
			}
		}
	}
	return on
}
