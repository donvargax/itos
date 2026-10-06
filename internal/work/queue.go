package work

import (
	"fmt"
	"slices"

	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/value"
)

// The queue (slice 66): a top-level queue: list of item ids in the registry,
// one for the whole repository, ideas included, the order the work comes in.
// itos work proposes in its order (propose.go), each person seeing the part
// they can take; work queue puts an item in it or takes one out, and work
// done takes a closed item out (Unqueue), each a registry commit as the other
// writers make (write.go). It is written as a block list, before items:,
// which no formatter rewraps however long it grows (value.Doc's
// SetBlockList).

// Place is where work queue puts an item: first (Top), just before or just
// after an item the queue holds (Before, After), or out of the queue (Drop).
type Place struct {
	Top, Drop     bool
	Before, After string
}

// queueHeader is the header of every commit that changes the queue.
func queueHeader(id string) string { return "docs: queue " + id }

// Queued are the ids of a sound registry's queue, in its order; none when it
// has no queue.
func Queued(r Registry) []string {
	list, _ := r.Queue.([]any)
	ids := make([]string, len(list))
	for i, id := range list {
		ids[i] = value.String(id)
	}
	return ids
}

// Queue is the registry with the item put where at says, the queue written
// whole (slice 66). An item the queue holds already is moved. Refused, with
// nothing changed: an id no item has, an item done or dropped (the queue
// holds work to do), and for --before or --after the item itself or one the queue does not
// hold. An item already where it is put, or out of a queue that does not hold
// it, is Unchanged.
func Queue(r Registry, text, id string, at Place) (Change, *out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, unknown(id), nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, *out.Problem, error) {
		return Change{}, &out.Problem{Rule: rule, Message: message, Fix: fix}, nil
	}
	now := Queued(r)
	rest := slices.DeleteFunc(slices.Clone(now), func(q string) bool { return q == id })
	title := value.JSON(value.String(item.At("title")))
	var after []string
	var body string
	switch {
	case at.Drop:
		after = rest
		body = fmt.Sprintf("Take %s (%s) out of the queue, with itos work queue.", id, title)
	case item.At("status") == "done":
		return refuse("work-queue-done", id+" is done, and the queue holds work still to do",
			"queue an item not done (itos work list)")
	case item.At("status") == Dropped:
		return refuse("work-queue-dropped", id+" is dropped, and the queue holds work still to do",
			"queue an item still to do (itos work list)")
	case at.Top:
		after = append([]string{id}, rest...)
		body = fmt.Sprintf("Put %s (%s) first in the queue, with itos work queue.", id, title)
	default:
		other, where, offset := at.Before, "before", 0
		if other == "" {
			other, where, offset = at.After, "after", 1
		}
		if other == id {
			return refuse("work-queue-itself", fmt.Sprintf("%s cannot be queued %s itself", id, where),
				"name another item the queue holds, or --top")
		}
		j := slices.Index(rest, other)
		if j < 0 {
			return refuse("work-queue-not-queued", other+" is not in the queue",
				"queue "+other+" first (itos work queue "+other+" --top), or name an item the queue holds")
		}
		after = slices.Insert(rest, j+offset, id)
		body = fmt.Sprintf("Put %s (%s) %s %s in the queue, with itos work queue.", id, title, where, other)
	}
	return queueChange(r, text, item, after, queueHeader(id), body)
}

// Unqueue is the registry with the item out of the queue (slice 66), as work
// done leaves it once the item is closed; ok is false when the queue does
// not hold it, nothing to change.
func Unqueue(r Registry, text, id string) (change Change, ok bool, err error) {
	i := find(r, id)
	if i < 0 || !slices.Contains(Queued(r), id) {
		return Change{}, false, nil
	}
	item := r.Items[i]
	rest := slices.DeleteFunc(Queued(r), func(q string) bool { return q == id })
	body := fmt.Sprintf("Take %s (%s) out of the queue: it is done, closed by itos work done.", id, value.JSON(value.String(item.At("title"))))
	change, _, err = queueChange(r, text, item, rest, queueHeader(id), body)
	return change, err == nil, err
}

// queueChange is the change that writes the queue as after, Unchanged when
// it is already so.
func queueChange(r Registry, text string, item *value.Map, after []string, header, body string) (Change, *out.Problem, error) {
	if slices.Equal(after, Queued(r)) {
		return Change{Item: item, Queue: after, Unchanged: true}, nil, nil
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.SetBlockList([]any{"queue"}, after, "items"); err != nil {
		return Change{}, nil, err
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	return Change{Text: edited, Item: item, Queue: after, Header: header, Body: body}, nil, nil
}
