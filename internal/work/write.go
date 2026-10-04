package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/value"
)

// The commands that write the registry (slice 52): work take sets an item
// in progress for the person, work promote makes an idea a slice or a task.
// Each judges a sound registry and gives a Change, the registry's text with
// the edit made in place (value.Doc: every comment, quote and line kept), the
// item as it is after and the commit message that records it, or the
// problem that refuses it, nothing written. Committing the registry alone is
// the command line's (internal/cli/workwrite.go), the same for every
// command that writes it.

// Change is an edit of the registry: its text after, the item as it reads
// after (its depends_on a list, as Load gives it), the items whose
// depends_on now name it, and the header and body of the commit that
// records it. Unchanged is an edit with nothing to write: the item was
// already as asked.
type Change struct {
	Text      string
	Item      *value.Map
	Rewritten []string
	Header    string
	Body      string
	Unchanged bool
}

// find is the index of the first item whose id is the given one, -1 when
// none is.
func find(r Registry, id string) int {
	for i, item := range r.Items {
		if at := item.At("id"); !absent(at) && value.String(at) == id {
			return i
		}
	}
	return -1
}

// unknown is the problem of an id no item has.
func unknown(id string) *out.Problem {
	return &out.Problem{Rule: "work-unknown-item", Message: fmt.Sprintf("no item %s in the registry", id),
		Fix: "itos work list names every item"}
}

// Take is the registry with the item set in progress for person (slice 52):
// its status doing and its owner the person. every is a session that owns
// every item (a stealth config with no --as, as ProposeEvery): no owner
// refuses it, and the owner is left as it is. Refused, with nothing changed:
// an id no item has, an idea (work promote makes it a slice or a task
// first), an item done or deferred, one whose owner, or its group's, is
// someone else, and one with a dependency not done. An item already in
// progress for the person is Unchanged.
func Take(r Registry, text, id, person string, every bool) (Change, *out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, unknown(id), nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, *out.Problem, error) {
		return Change{}, &out.Problem{Rule: rule, Message: message, Fix: fix}, nil
	}
	status := item.At("status")
	switch {
	case item.At("kind") == "idea":
		return refuse("work-take-idea", id+" is an idea, not yet specified: make it a slice or a task first",
			"itos work promote "+id+" --as <id> --kind slice|task")
	case status == "done":
		return refuse("work-take-done", id+" is done", "take an item not done (itos work)")
	case item.At("deferred") != value.Undefined:
		return refuse("work-take-deferred", fmt.Sprintf("%s is deferred: %s", id, value.Trim(value.String(item.At("deferred")))),
			"remove "+id+"'s deferred: first, or take another item (itos work)")
	case status != "todo" && status != "doing":
		return refuse("work-take-status", fmt.Sprintf("%s is %s, neither todo nor doing", id, value.String(status)),
			"take an item that is todo (itos work)")
	}
	owner := ownerOf(r, item)
	if !every && !absent(owner) && owner != person {
		return refuse("work-take-owned", fmt.Sprintf("%s is %s's, not %s's", id, value.String(owner), person),
			"take one of yours or nobody's (itos work), or agree with "+value.String(owner)+" to give it to you")
	}
	if status == "doing" && (every || owner == person) {
		return Change{Item: item, Unchanged: true}, nil, nil
	}
	if on := waitsOn(r, item); len(on) > 0 {
		return refuse("work-take-waiting", id+" waits on "+strings.Join(on, ", "),
			"finish what it waits on first, or take an item that can start (itos work)")
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	if err := doc.Set([]any{"items", i, "status"}, "doing"); err != nil {
		return Change{}, nil, err
	}
	after.Set("status", "doing")
	if !every && item.At("owner") != person {
		if err := doc.Set([]any{"items", i, "owner"}, person); err != nil {
			return Change{}, nil, err
		}
		after.Set("owner", person)
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	body := fmt.Sprintf("Set %s (%s) to doing", id, value.JSON(value.String(item.At("title"))))
	if who, ok := after.At("owner").(string); ok && who != "" {
		body += " for " + who
	}
	body += ", with itos work take."
	return Change{Text: edited, Item: after, Header: "docs: take " + id, Body: body}, nil, nil
}

// ownerOf is an item's owner, else its group's, nil when neither has one.
func ownerOf(r Registry, item *value.Map) any {
	if owner := item.At("owner"); !absent(owner) {
		return owner
	}
	if owner := r.Phases.At(value.String(item.At("phase"))); !absent(owner) {
		return owner
	}
	return nil
}

// waitsOn are the item's dependencies not done, each with its status.
func waitsOn(r Registry, item *value.Map) []string {
	var on []string
	deps, _ := item.At("depends_on").([]any)
	for _, dep := range deps {
		status := "not an item"
		if j := find(r, value.String(dep)); j >= 0 {
			if r.Items[j].At("status") == "done" {
				continue
			}
			status = value.String(r.Items[j].At("status"))
		}
		on = append(on, fmt.Sprintf("%s (%s)", value.String(dep), status))
	}
	return on
}

// Promote is the registry with the idea made a slice or a task (kind) under
// a new id (slice 52): its id and kind changed, "Was <old id>." put before
// its why, and every depends_on that named the old id naming the new one.
// Refused, with nothing changed: an id no item has, an item that is not an
// idea, a new id an item already has, and for a task a new id the ledger's
// pattern (taskID) does not match.
func Promote(r Registry, text, id, newID, kind string, taskID *regexp.Regexp) (Change, *out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, unknown(id), nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, *out.Problem, error) {
		return Change{}, &out.Problem{Rule: rule, Message: message, Fix: fix}, nil
	}
	if item.At("kind") != "idea" {
		what := "has no kind"
		if kind := item.At("kind"); !absent(kind) {
			what = "is a " + value.String(kind)
		}
		return refuse("work-promote-not-idea", fmt.Sprintf("%s %s, not an idea: only an idea is promoted", id, what),
			"promote an item of kind idea")
	}
	if j := find(r, newID); j >= 0 {
		return refuse("work-promote-taken", fmt.Sprintf("%s is already an item: %s", newID, value.String(r.Items[j].At("title"))),
			"give the "+kind+" an id no item has")
	}
	if kind == "task" && taskID != nil && !taskID.MatchString(newID) {
		return refuse("work-promote-not-task-id", fmt.Sprintf("%s is not a task ID by ledger.id (%s)", newID, taskID.String()),
			"give the task an ID ledger.id matches, so a Task footer can name it")
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	lead := "Was " + id + "."
	for _, edit := range []func() error{
		func() error { return doc.Set([]any{"items", i, "id"}, newID) },
		func() error { return doc.Set([]any{"items", i, "kind"}, kind) },
		func() error { return doc.Lead([]any{"items", i, "why"}, lead) },
	} {
		if err := edit(); err != nil {
			return Change{}, nil, err
		}
	}
	after.Set("id", newID)
	after.Set("kind", kind)
	if why := value.Prop(value.Prop(doc.Want, "items").([]any)[i], "why"); why != value.Undefined {
		after.Set("why", why)
	}
	rewritten := []string{}
	for j, other := range r.Items {
		deps, _ := other.At("depends_on").([]any)
		named := false
		for k, dep := range deps {
			if dep != id {
				continue
			}
			if err := doc.Set([]any{"items", j, "depends_on", k}, newID); err != nil {
				return Change{}, nil, err
			}
			named = true
		}
		if named {
			rewritten = append(rewritten, value.String(other.At("id")))
		}
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	body := fmt.Sprintf("Make the idea %s (%s) the %s %s, with itos work promote", id, value.JSON(value.String(item.At("title"))), kind, newID)
	if len(rewritten) > 0 {
		body += ", and rename it where " + strings.Join(rewritten, ", ") + " depend"
		if len(rewritten) == 1 {
			body += "s"
		}
		body += " on it"
	}
	body += "."
	return Change{Text: edited, Item: after, Rewritten: rewritten, Header: "docs: promote " + id + " to " + newID, Body: body}, nil, nil
}

// Wrap is text broken into lines of at most width characters, at spaces, as
// a commit body is written; a word longer than width has a line to itself.
func Wrap(text string, width int) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len([]rune(word)) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
