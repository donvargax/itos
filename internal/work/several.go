package work

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

// Several ids (slice 118): grooming a backlog is one change made to many
// items, and a command per item made a commit each. work edit, drop, defer,
// resume and queue take several ids: the same change for each, every id
// judged before anything is written, the whole refused when any one would
// be, and one commit of the registry. EditEach, DropEach, DeferEach and
// ResumeEach apply the one-item writer to each id in turn (each), each on
// the registry as the ids before it left it, so what work check would find
// is judged with the items before it changed; QueueEach places the ids
// together, in the order given.

// headerWidth is the longest header the header lint takes, its
// header-max-length in JavaScript's UTF-16 units (internal/message, which
// imports this package, holds the lint).
const headerWidth = 100

// Several is one command's change of several items: the registry's text
// after, each id's item as it reads after, in the order given, the ids it
// changed, the queue after (QueueEach's), and the header and body of the
// commit that records it. Unchanged is a change with nothing to write: every
// item was already as asked.
type Several struct {
	Text      string
	Items     []*value.Map
	Changed   []string
	Queue     []string
	Header    string
	Body      string
	Unchanged bool
	// Each is each id's own change, in the order given, Unchanged where the
	// item was already as asked (EditEach's Changed per item).
	Each []Change
}

// writer is a one-item writer of the registry, as Drop is.
type writer func(r Registry, text, id string) (Change, []out.Problem, error)

// each applies write to each id in turn, each on the registry and text the
// ids before it left. An id refused is named in refused, its problems in
// found, and the next id judged on the registry as it was before it, so
// every id refused is found in one run; the caller writes nothing then.
func each(cfg *config.Loaded, r Registry, text string, ids []string, write writer) (s Several, refused []string, found []out.Problem, err error) {
	s.Text = text
	for _, id := range ids {
		change, problems, err := write(r, s.Text, id)
		if err != nil {
			return Several{}, nil, nil, err
		}
		if len(problems) > 0 {
			refused = append(refused, id)
			found = append(found, problems...)
			continue
		}
		s.Each = append(s.Each, change)
		s.Items = append(s.Items, change.Item)
		if change.Unchanged {
			continue
		}
		s.Changed = append(s.Changed, id)
		s.Text = change.Text
		next, err := parse(cfg, cfg.Work.Registry, s.Text)
		if err != nil {
			return Several{}, nil, nil, err
		}
		next.Logins, next.People = r.Logins, r.People
		r = next
	}
	return s, refused, found, nil
}

// recorded is s with the header and body of its commit: none when nothing
// changed, the one item's own when one did, else verb's header naming the
// ids it changed (severalHeader) and body, the words before and after the
// changed items, each as titled says it, the list as join makes it (listed,
// or semicolons for descriptions that hold commas).
func recorded(s Several, verb, before, after string, titled func(Change) string, join func([]string) string) Several {
	var changes []Change
	for _, c := range s.Each {
		if !c.Unchanged {
			changes = append(changes, c)
		}
	}
	switch len(changes) {
	case 0:
		s.Unchanged = true
		return s
	case 1:
		s.Header, s.Body = changes[0].Header, changes[0].Body
		return s
	}
	words := make([]string, len(changes))
	for i, c := range changes {
		words[i] = titled(c)
	}
	s.Header = severalHeader(verb, s.Changed)
	s.Body = strings.TrimSpace(before + join(words) + after)
	return s
}

// severalHeader is the header of a commit of several items: "docs: <verb>
// a, b" when that fits the header lint's length, else "docs: <verb> 12
// items"; the body names every one.
func severalHeader(verb string, ids []string) string {
	header := "docs: " + verb + " " + strings.Join(ids, ", ")
	if len(utf16.Encode([]rune(header))) <= headerWidth {
		return header
	}
	return fmt.Sprintf("docs: %s %d items", verb, len(ids))
}

// listed is the words joined as a list is said: "a", "a and b", "a, b and
// c".
func listed(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// semicolons is the descriptions joined by "; ", each of which may hold
// commas.
func semicolons(words []string) string { return strings.Join(words, "; ") }

// titledItem is "<id> (<title>)", as the writers' bodies name an item.
func titledItem(item *value.Map) string {
	return fmt.Sprintf("%s (%s)", value.String(item.At("id")), value.JSON(value.String(item.At("title"))))
}

// EditEach is Edit made to each id (slice 118), its body naming the keys
// it changed of each item (Change.Changed, why for a note).
func EditEach(cfg *config.Loaded, r Registry, text string, ids []string, e Edits, taskID *regexp.Regexp) (Several, []string, []out.Problem, error) {
	s, refused, found, err := each(cfg, r, text, ids, func(r Registry, text, id string) (Change, []out.Problem, error) {
		return Edit(cfg, r, text, id, e, taskID)
	})
	if err != nil || len(found) > 0 {
		return Several{}, refused, found, err
	}
	return recorded(s, "edit", "Change, with itos work edit, ", ".", func(c Change) string {
		return titledItem(c.Item) + ": its " + listed(c.Changed)
	}, semicolons), nil, nil, nil
}

// DropEach is Drop made to each id (slice 118), the reason last in its
// body, after the ids.
func DropEach(cfg *config.Loaded, r Registry, text string, ids []string, reason string) (Several, []string, []out.Problem, error) {
	s, refused, found, err := each(cfg, r, text, ids, func(r Registry, text, id string) (Change, []out.Problem, error) {
		return Drop(cfg, r, text, id, reason)
	})
	if err != nil || len(found) > 0 {
		return Several{}, refused, found, err
	}
	return recorded(s, "drop", "Set ", " to dropped, with itos work drop: they stay in the registry, so their ids are never given again. "+
		strings.TrimSpace(reason), func(c Change) string { return titledItem(c.Item) }, listed), nil, nil, nil
}

// DeferEach is Defer made to each id (slice 118), the reason last in its
// body, after the ids.
func DeferEach(cfg *config.Loaded, r Registry, text string, ids []string, reason string) (Several, []string, []out.Problem, error) {
	s, refused, found, err := each(cfg, r, text, ids, func(r Registry, text, id string) (Change, []out.Problem, error) {
		return Defer(cfg, r, text, id, reason)
	})
	if err != nil || len(found) > 0 {
		return Several{}, refused, found, err
	}
	return recorded(s, "defer", "Set ", " deferred, with itos work defer: each keeps its status, its owner and its place in the queue. "+
		strings.TrimSpace(reason), func(c Change) string { return titledItem(c.Item) }, listed), nil, nil, nil
}

// ResumeEach is Resume made to each id (slice 118), its body naming the
// reason each was deferred with.
func ResumeEach(cfg *config.Loaded, r Registry, text string, ids []string) (Several, []string, []out.Problem, error) {
	reasons := map[string]string{}
	for _, item := range r.Items {
		if deferred(item) {
			reasons[value.String(item.At("id"))] = value.Trim(value.String(item.At("deferred")))
		}
	}
	s, refused, found, err := each(cfg, r, text, ids, func(r Registry, text, id string) (Change, []out.Problem, error) {
		return Resume(cfg, r, text, id)
	})
	if err != nil || len(found) > 0 {
		return Several{}, refused, found, err
	}
	return recorded(s, "resume", "Lift the deferral, with itos work resume, of ", ".", func(c Change) string {
		return titledItem(c.Item) + ", deferred with the reason " + value.JSON(reasons[value.String(c.Item.At("id"))])
	}, semicolons), nil, nil, nil
}

// QueueEach is the registry with the ids put together where at says, in
// the order given (slice 118): first, just before or after an item the queue
// holds and the ids do not name, or each out of the queue. Each id is judged
// as Queue judges it alone, a problem two ids share given once. The header
// names every id given.
func QueueEach(r Registry, text string, ids []string, at Place) (Several, []string, []out.Problem, error) {
	var refused []string
	var found []out.Problem
	items := make([]*value.Map, len(ids))
	for i, id := range ids {
		// Queue judges the id alone; its text is not written, so a text it
		// cannot edit is the change's below, when there is one to make.
		if _, p, _ := Queue(r, text, id, at); p != nil {
			refused = append(refused, id)
			if !slices.Contains(found, *p) {
				found = append(found, *p)
			}
			continue
		}
		items[i] = r.Items[find(r, id)]
	}
	if len(found) > 0 {
		return Several{}, refused, found, nil
	}
	words := make([]string, len(items))
	for i, item := range items {
		words[i] = titledItem(item)
	}
	rest := slices.DeleteFunc(Queued(r), func(q string) bool { return slices.Contains(ids, q) })
	var after []string
	var body string
	switch other, where, offset := at.Before, "before", 0; {
	case at.Remove:
		after = rest
		body = fmt.Sprintf("Take %s out of the queue, with itos work queue.", listed(words))
	case at.Top:
		after = append(slices.Clone(ids), rest...)
		body = fmt.Sprintf("Put %s first in the queue, in that order, with itos work queue.", listed(words))
	default:
		if other == "" {
			other, where, offset = at.After, "after", 1
		}
		after = slices.Insert(rest, slices.Index(rest, other)+offset, ids...)
		body = fmt.Sprintf("Put %s %s %s in the queue, in that order, with itos work queue.", listed(words), where, other)
	}
	verb := "queue"
	if at.Remove {
		verb = "dequeue"
	}
	change, _, err := queueChange(r, text, nil, after, severalHeader(verb, ids), body)
	if err != nil {
		return Several{}, nil, nil, err
	}
	s := Several{Text: change.Text, Items: items, Queue: after, Header: change.Header, Body: change.Body, Unchanged: change.Unchanged}
	if !s.Unchanged {
		s.Changed = ids
	}
	return s, nil, nil, nil
}
