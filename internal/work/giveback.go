package work

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// GiveBack is the registry with the item in progress given back (slice 122):
// its status todo again and nothing else of it changed, its owner kept (an
// owner is who the item is for, in progress or not), and the reason after the
// command's own sentence in the body of the commit that records it, as Drop
// and Defer put theirs. It
// undoes a take, so the item can be taken again or deferred. person and every
// are Take's: an item whose owner, or its group's, is someone else is refused
// unless the session owns every item. Refused, with nothing changed: an id no
// item has, an item that is not doing (naming its status), one someone else
// owns (naming them), and whatever work check would find in the registry with
// the item given back.
func GiveBack(cfg *config.Loaded, r Registry, text, id, person string, every bool, reason string) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, []out.Problem, error) {
		return Change{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	if status := item.At("status"); status != "doing" {
		return refuse("work-give-back-status",
			fmt.Sprintf("%s is %s, not in progress: only an item doing is given back", id, value.String(status)),
			"give back an item in progress (itos work)")
	}
	owner := ownerOf(r, item)
	if !every && !absent(owner) && owner != person {
		return refuse("work-give-back-owned", fmt.Sprintf("%s is %s's, not %s's", id, value.String(owner), person),
			"give back one of yours, or agree with "+value.String(owner)+" that they give it back")
	}
	// A text that cannot be edited in place ends it at the first step that
	// fails: opening it, setting the status, reading the edit back.
	var edited string
	doc, err := value.OpenDoc(text)
	if err == nil {
		err = doc.Set([]any{"items", i, "status"}, "todo")
	}
	if err == nil {
		edited, err = doc.Text()
	}
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(item).(*value.Map)
	after.Set("status", "todo")
	found, err := sound(cfg, r, i, after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	body := fmt.Sprintf("Set %s back to todo, with itos work give-back", titledItem(item))
	if who, ok := item.At("owner").(string); ok && who != "" {
		body += ": it keeps its owner, " + who
	}
	return Change{Text: edited, Item: after, Header: "docs: give back " + id, Body: body + ". " + strings.TrimSpace(reason)}, nil, nil
}

// GiveBackEach is GiveBack made to each id (slice 122), the reason last in
// its body, after the ids.
func GiveBackEach(cfg *config.Loaded, r Registry, text string, ids []string, person string, every bool, reason string) (Several, []string, []out.Problem, error) {
	s, refused, found, err := each(cfg, r, text, ids, func(r Registry, text, id string) (Change, []out.Problem, error) {
		return GiveBack(cfg, r, text, id, person, every, reason)
	})
	if err != nil || len(found) > 0 {
		return Several{}, refused, found, err
	}
	return recorded(s, "give back", "Set ", " back to todo, with itos work give-back: each keeps its owner. "+
		strings.TrimSpace(reason), func(c Change) string { return titledItem(c.Item) }, listed), nil, nil, nil
}
