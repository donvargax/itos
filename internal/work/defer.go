package work

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/value"
)

// Defer is the registry with the item put off (slice 98): its deferred key
// the reason, a folded text as the registry's other texts are (value.Doc's
// Note), and the reason first in the body of the commit that records it, as
// Drop puts it. The item keeps its status, owner, tags and place in the
// queue, as a deferred key written by hand does; work and status list it
// apart, and work take and work done refuse it. Refused, with nothing
// changed: an id no item has, an item already deferred (naming its reason:
// Resume lifts it first), an item done or dropped, one of any other status
// but todo, since work check holds a deferred item to todo, and whatever work
// check would find in the registry with the item deferred.
func Defer(cfg *config.Loaded, r Registry, text, id, reason string) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, []out.Problem, error) {
		return Change{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	switch status := item.At("status"); {
	case deferred(item):
		return refuse("work-defer-deferred",
			fmt.Sprintf("%s is already deferred: %s", id, value.Trim(value.String(item.At("deferred")))),
			"itos work resume "+id+" first, then defer it with the new reason")
	case status == "done":
		return refuse("work-defer-done", id+" is done, and a done item stays done",
			"defer an item still to do (itos work list)")
	case status == Dropped:
		return refuse("work-defer-dropped", id+" is dropped, already out of the open work",
			"defer an item still to do (itos work list)")
	case status != "todo":
		return refuse("work-defer-status", fmt.Sprintf("%s is %s: only an item not started, todo, is deferred", id, value.String(status)),
			"defer an item that is todo (itos work list)")
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.Note([]any{"items", i, "deferred"}, reason); err != nil {
		return Change{}, nil, err
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	after.Set("deferred", value.Prop(value.Prop(doc.Want, "items").([]any)[i], "deferred"))
	found, err := sound(cfg, r, i, after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	body := fmt.Sprintf("%s Set %s (%s) deferred, with itos work defer: it keeps its status, its owner and its place in the queue.",
		strings.TrimSpace(reason), id, value.JSON(value.String(item.At("title"))))
	return Change{Text: edited, Item: after, Header: "docs: defer " + id, Body: body}, nil, nil
}

// Resume is the registry with the item's deferral lifted (slice 98): its
// deferred key removed, and nothing else of it changed. Refused, with nothing
// changed: an id no item has and an item that is not deferred.
func Resume(cfg *config.Loaded, r Registry, text, id string) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	if !deferred(item) {
		return Change{}, []out.Problem{{Rule: "work-resume-not-deferred", Message: id + " is not deferred",
			Fix: "itos work list marks a deferred item (deferred)"}}, nil
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.Drop([]any{"items", i, "deferred"}); err != nil {
		return Change{}, nil, err
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	after.Delete("deferred")
	found, err := sound(cfg, r, i, after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	body := fmt.Sprintf("Lift the deferral of %s (%s), with itos work resume: it was deferred with the reason %s.",
		id, value.JSON(value.String(item.At("title"))), value.JSON(value.Trim(value.String(item.At("deferred")))))
	return Change{Text: edited, Item: after, Header: "docs: resume " + id, Body: body}, nil, nil
}

// deferred is an item with a deferred key, whatever it holds: one work check
// reports when it holds no reason.
func deferred(item *value.Map) bool { return item.At("deferred") != value.Undefined }
