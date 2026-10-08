package work

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/value"
)

// The commands that write the registry (slice 52): work take sets an item
// in progress for the person, work promote makes an idea a slice or a task,
// work done (slice 53) marks an item done, and work add and work edit
// (slice 54) make an item and change its title, dependencies, refs, tags or
// why.
// Each judges a sound registry and gives a Change, the registry's text with
// the edit made in place (value.Doc: every comment, quote and line kept), the
// item as it is after and the commit message that records it, or the
// problem that refuses it, nothing written. Committing the registry alone is
// the command line's (internal/cli/workwrite.go), the same for every
// command that writes it.

// Change is an edit of the registry: its text after, the item as it reads
// after (its depends_on a list, as Load gives it), the items whose
// depends_on now name it, the item's keys the edit changed (work edit's),
// the queue after (work queue's and Unqueue's, queue.go), and the header and
// body of the commit that records it. Unchanged is an edit with nothing to
// write: the item was already as asked.
type Change struct {
	Text      string
	Item      *value.Map
	Rewritten []string
	Changed   []string
	Queue     []string
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
		Fix: "itos work list --all names every item"}
}

// Take is the registry with the item set in progress for person (slice 52):
// its status doing and its owner the person. every is a session that owns
// every item (a stealth config with no --as, as ProposeEvery): no owner
// refuses it, and the owner is left as it is. Refused, with nothing changed:
// an id no item has, an idea (work promote makes it a slice or a task
// first), an item done, dropped (slice 78) or deferred, one whose owner, or
// its group's, is someone else, and one with a dependency not done. An item
// already in progress for the person is Unchanged.
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
	case status == Dropped:
		return refuse("work-take-dropped", id+" is dropped, taken out of the open work",
			"take an item that is todo (itos work)")
	case item.At("kind") == "idea":
		return refuse("work-take-idea", id+" is an idea, not yet specified: make it a slice or a task first",
			"itos work promote "+id+" --kind slice|task")
	case status == "done":
		return refuse("work-take-done", id+" is done", "take an item not done (itos work)")
	case item.At("deferred") != value.Undefined:
		return refuse("work-take-deferred", fmt.Sprintf("%s is deferred: %s", id, value.Trim(value.String(item.At("deferred")))),
			"itos work resume "+id+" first, or take another item (itos work)")
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

// Done is the registry with the item done (slice 53): its status done, its
// owner left as it is, and its why dropped (slice 76): the registry is an
// index, a done item holding its index fields alone, and the reasons live
// in its spec (a slice's feature file, a task's ledger entry) and its
// commits, the dropped text in git's history. Whether the work landed (its scenarios live, its
// commits pushed, its CI run green, a task's checks passing) is the command
// line's to judge (internal/cli/workdone.go), as is the ledger's edit that
// takes a closed task's checks out (slice 101); Done judges the registry
// alone. Refused, with nothing changed: an id no item has, an idea (not yet
// specified, so nothing of it can be done: work promote makes it a slice or
// a task), an item dropped or deferred, one that is not doing (work take
// starts it, so a todo item is never closed unstarted), and whatever work
// check would find in the registry with the item done, a dependency not
// done among it (bug 37: a registry work check refuses would stop every
// later write under a stealth config). An item already done is Unchanged.
func Done(cfg *config.Loaded, r Registry, text, id string) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, []out.Problem, error) {
		return Change{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	status := item.At("status")
	switch {
	case status == "done":
		return Change{Item: item, Unchanged: true}, nil, nil
	case status == Dropped:
		return refuse("work-done-dropped", id+" is dropped, taken out of the open work, so it is never done",
			"close an item that is in progress")
	case item.At("kind") == "idea":
		return refuse("work-done-idea", id+" is an idea, not yet specified, so there is nothing of it to be done",
			"itos work promote "+id+" --kind slice|task, then build it")
	case item.At("deferred") != value.Undefined:
		return refuse("work-done-deferred", fmt.Sprintf("%s is deferred: %s", id, value.Trim(value.String(item.At("deferred")))),
			"itos work resume "+id+" first")
	case status != "doing":
		return refuse("work-done-status", fmt.Sprintf("%s is %s, not doing", id, value.String(status)),
			"take it first (itos work take "+id+"), then land its work")
	}
	after := value.Copy(item).(*value.Map)
	after.Set("status", "done")
	after.Delete("why")
	if found, err := sound(cfg, r, i, after); err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.Set([]any{"items", i, "status"}, "done"); err != nil {
		return Change{}, nil, err
	}
	hasWhy := item.Has("why")
	if hasWhy {
		if err := doc.Drop([]any{"items", i, "why"}); err != nil {
			return Change{}, nil, err
		}
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	body := fmt.Sprintf("Set %s (%s) to done, with itos work done.", id, value.JSON(value.String(item.At("title"))))
	if hasWhy {
		body += " Its why is dropped, the registry being an index: the spec and the commits keep the reasons."
	}
	return Change{Text: edited, Item: after, Header: "docs: close " + id, Body: body}, nil, nil
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
// its why, and every depends_on that named the old id naming the new one,
// as the queue does when it holds the idea (slice 66).
// A title given ("" for none) replaces the idea's (slice 65): an idea
// promoted has usually become something more specific than its title says,
// and the commit's body names the new title. Refused, with nothing changed: an id no item has, an item that is not an
// idea, a new id an item already has, and for a task a new id the ledger's
// pattern (taskID) does not match, and an idea dropped (slice 78).
func Promote(r Registry, text, id, newID, kind, title string, taskID *regexp.Regexp) (Change, *out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, unknown(id), nil
	}
	item := r.Items[i]
	refuse := func(rule, message, fix string) (Change, *out.Problem, error) {
		return Change{}, &out.Problem{Rule: rule, Message: message, Fix: fix}, nil
	}
	if item.At("status") == Dropped {
		return refuse("work-promote-dropped", id+" is dropped, taken out of the open work",
			"add the work anew with itos work add, under an id no item has")
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
	retitled := title != "" && item.At("title") != title
	if retitled {
		if err := doc.Set([]any{"items", i, "title"}, title); err != nil {
			return Change{}, nil, err
		}
		after.Set("title", title)
	}
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
	queued := false
	for k, q := range Queued(r) {
		if q != id {
			continue
		}
		if err := doc.Set([]any{"queue", k}, newID); err != nil {
			return Change{}, nil, err
		}
		queued = true
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	body := fmt.Sprintf("Make the idea %s (%s) the %s %s", id, value.JSON(value.String(item.At("title"))), kind, newID)
	if retitled {
		body += ", its title now " + value.JSON(title)
	}
	body += ", with itos work promote"
	if len(rewritten) > 0 {
		body += ", and rename it where " + strings.Join(rewritten, ", ") + " depend"
		if len(rewritten) == 1 {
			body += "s"
		}
		body += " on it"
	}
	if queued {
		body += ", keeping its place in the queue"
	}
	body += "."
	return Change{Text: edited, Item: after, Rewritten: rewritten, Header: "docs: promote " + id + " to " + newID, Body: body}, nil, nil
}

// New is the item work add makes (slice 54): its id, title and why, its
// kind (an idea unless said), its group (Phase, "" for the default Add
// picks), its owner ("" for nobody) and its dependencies, refs and tags
// (slice 97).
type New struct {
	ID, Title, Why, Kind, Phase, Owner string
	DependsOn, Refs, Tags              []string
}

// phaseOf is the group a new item goes in: the one given, else the one its
// id names (p<n>-…) when the registry lists it, else the registry's only
// one; "" when none of those is.
func phaseOf(r Registry, n New) string {
	if n.Phase != "" {
		return n.Phase
	}
	if m := ideaPhase.FindStringSubmatch(n.ID); m != nil && r.Phases.Has(m[1]) {
		return m[1]
	}
	if keys := r.Phases.Keys(); len(keys) == 1 {
		return keys[0]
	}
	return ""
}

// ideaPhase is an id not yet numbered, p<phase>-<name>, and its phase.
var ideaPhase = regexp.MustCompile(`^p(\d+)-`)

// listOf is a list of text as the registry holds it.
func listOf(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// sound is the problems work check finds in the registry with after in
// place of its item i, or added when i is past its end: the registry was
// sound, so every one is the edit's.
func sound(cfg *config.Loaded, r Registry, i int, after *value.Map) ([]out.Problem, error) {
	items := append([]*value.Map{}, r.Items...)
	if i < len(items) {
		items[i] = after
	} else {
		items = append(items, after)
	}
	r.Items = items
	return Issues(cfg, r, cfg.Work.Registry)
}

// Add is the registry with a new item at the end of its items (slice 54):
// todo, of its kind, in its group, owned by its owner or nobody, its why a
// folded text. Refused, with nothing changed: an id an item has, a task's id
// that ledger.id (taskID) does not match, no group given where the default
// is not plain (phaseOf), and whatever work check would find in the
// registry with it (a group not listed, an owner not among the people, a
// dependency on no item, a tag work.tags does not declare).
func Add(cfg *config.Loaded, r Registry, text string, n New, taskID *regexp.Regexp) (Change, []out.Problem, error) {
	refuse := func(rule, message, fix string) (Change, []out.Problem, error) {
		return Change{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	if j := find(r, n.ID); j >= 0 {
		return refuse("work-add-taken", fmt.Sprintf("%s is already an item: %s", n.ID, value.String(r.Items[j].At("title"))),
			"give the new item an id no item has, or change "+n.ID+" with itos work edit")
	}
	if n.Kind == "task" && taskID != nil && !taskID.MatchString(n.ID) {
		return refuse("work-add-not-task-id", fmt.Sprintf("%s is not a task ID by ledger.id (%s)", n.ID, taskID.String()),
			"give the task an ID ledger.id matches, so a Task footer can name it")
	}
	label := cfg.Ledger.Group.Label
	phase := phaseOf(r, n)
	if phase == "" {
		return refuse("work-add-no-phase", fmt.Sprintf("which %s %s is in is not said: the registry lists %s",
			label, n.ID, strings.Join(r.Phases.Keys(), ", ")), "pass --phase <"+label+">")
	}
	var owner any
	if n.Owner != "" {
		owner = n.Owner
	}
	group := value.Resolve(phase)
	if _, isNumber := group.(float64); !isNumber {
		group = phase
	}
	item := value.NewMap("id", n.ID, "title", n.Title, "phase", group, "owner", owner, "status", "todo",
		"depends_on", listOf(n.DependsOn), "kind", n.Kind, "why", n.Why)
	if len(n.Refs) > 0 {
		item.Set("refs", listOf(n.Refs))
	}
	if len(n.Tags) > 0 {
		item.Set("tags", listOf(n.Tags))
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	if err := doc.Append([]any{"items"}, item, "why"); err != nil {
		return Change{}, nil, err
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	list := value.Prop(doc.Want, "items").([]any)
	after := value.Copy(list[len(list)-1]).(*value.Map)
	defaultLists(after)
	found, err := sound(cfg, r, len(r.Items), after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	body := fmt.Sprintf("Add the %s %s (%s) to %s %s, with itos work add.", n.Kind, n.ID, value.JSON(n.Title), label, phase)
	return Change{Text: edited, Item: after, Header: "docs: add " + n.ID, Body: body}, nil, nil
}

// Edits are what work edit changes of an item (slice 54): its title, its
// depends_on, its refs, its tags (slice 97; each nil when not given), and a paragraph to add to
// its why ("" for none).
type Edits struct {
	Title                 *string
	DependsOn, Refs, Tags *[]string
	Note                  string
}

// WhereWhy is where a slice's or a task's why lives (slice 76), the
// registry being an index whose whys are an idea's: SpecKind's kinds.
var WhereWhy = map[string]string{
	"slice": "its feature file (the description, and the comments above its scenarios)",
	"task":  "its ledger entry",
}

// Edit is the registry with the item's title, depends_on, refs or tags replaced
// and a paragraph added to its why (value.Doc's Note: a blank line, then the
// paragraph, in a folded why). A list given is written whole, one flow list
// on its key's line, whatever shape the old one had (value.Doc's SetList,
// bug 14). Change.Changed names the keys it changed.
// Refused, with nothing changed: an id no item has, a note on a slice or a
// task (SpecKind, taskID read as Add reads it), whose why lives in its spec
// (slice 79), and whatever work check would find in the registry with the
// item so (a dependency on no item, a cycle, an item done waiting on one not
// done). An item already as asked, with no note, is Unchanged; a list the
// item lacks is as asked when the list given is empty.
func Edit(cfg *config.Loaded, r Registry, text, id string, e Edits, taskID *regexp.Regexp) (Change, []out.Problem, error) {
	i := find(r, id)
	if i < 0 {
		return Change{}, []out.Problem{*unknown(id)}, nil
	}
	item := r.Items[i]
	if kind := SpecKind(item, taskID); e.Note != "" && WhereWhy[kind] != "" {
		return Change{}, []out.Problem{{
			Rule:    "work-note-spec",
			Message: fmt.Sprintf("%s is a %s, whose why belongs in %s, not the registry, which keeps an idea's why alone", id, kind, WhereWhy[kind]),
			Fix:     fmt.Sprintf("write the note in %s's %s", id, strings.TrimPrefix(WhereWhy[kind], "its ")),
		}}, nil
	}
	doc, err := value.OpenDoc(text)
	if err != nil {
		return Change{}, nil, err
	}
	// The note first: a key added after the why (refs, when the item has
	// none) is then written after it, as an edit at the same place comes
	// after the one made before it.
	if e.Note != "" {
		if err := doc.Note([]any{"items", i, "why"}, e.Note); err != nil {
			return Change{}, nil, err
		}
	}
	var changed []string
	if e.Title != nil && item.At("title") != *e.Title {
		if err := doc.Set([]any{"items", i, "title"}, *e.Title); err != nil {
			return Change{}, nil, err
		}
		changed = append(changed, "title")
	}
	for _, l := range []struct {
		key  string
		list *[]string
	}{{"depends_on", e.DependsOn}, {"refs", e.Refs}, {"tags", e.Tags}} {
		if l.list == nil {
			continue
		}
		// A list the item lacks, or one of nothing, is an empty one (bug 14):
		// --refs '' on an item with no refs is already so.
		old := item.At(l.key)
		if absent(old) {
			old = []any{}
		}
		if value.JSON(old) == value.JSON(listOf(*l.list)) {
			continue
		}
		if err := doc.SetList([]any{"items", i, l.key}, *l.list); err != nil {
			return Change{}, nil, err
		}
		changed = append(changed, l.key)
	}
	if e.Note != "" {
		changed = append(changed, "why")
	}
	if len(changed) == 0 {
		return Change{Item: item, Changed: []string{}, Unchanged: true}, nil, nil
	}
	edited, err := doc.Text()
	if err != nil {
		return Change{}, nil, err
	}
	after := value.Copy(value.Prop(doc.Want, "items").([]any)[i]).(*value.Map)
	defaultLists(after)
	found, err := sound(cfg, r, i, after)
	if err != nil || len(found) > 0 {
		return Change{}, found, err
	}
	what := make([]string, len(changed))
	for j, key := range changed {
		what[j] = "its " + key
		if key == "why" {
			what[j] = "a note on its why"
		}
	}
	listed := strings.Join(what, ", ")
	if len(what) > 1 {
		listed = strings.Join(what[:len(what)-1], ", ") + " and " + what[len(what)-1]
	}
	body := fmt.Sprintf("Change %s (%s): %s, with itos work edit.", id, value.JSON(value.String(after.At("title"))), listed)
	return Change{Text: edited, Item: after, Changed: changed, Header: "docs: edit " + id, Body: body}, nil, nil
}
