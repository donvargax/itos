package work

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v4/internal/out"
	"github.com/donvargax/itos/v4/internal/providers"
	"github.com/donvargax/itos/v4/internal/value"
)

// Identity is who a session works for: a handle and whether the people list
// it, or why there is none (Problem), which is nobody.
type Identity struct {
	Handle  string
	Listed  bool
	Problem string
}

// Whoami is who a session works for (work.ts's whoami): --as (as, "" when
// not given), which must be among the registry's people, else what the
// identity provider answers, listed or not. With no people (Registry.People:
// a stealth config, or a project's people file missing) there is no one to
// hold a handle to, so --as is taken as given and any handle is listed.
// listedIn is the people's file, as the messages name it.
func Whoami(r Registry, listedIn, as string, identify providers.Identity) Identity {
	listed := func(handle string) bool {
		if !r.People {
			return true
		}
		for _, l := range r.Logins {
			if l == handle {
				return true
			}
		}
		return false
	}
	if as != "" {
		if listed(as) {
			return Identity{Handle: as, Listed: true}
		}
		return Identity{Problem: fmt.Sprintf("\"%s\" is not in %s", as, listedIn)}
	}
	answer := identify()
	if answer.Handle == "" {
		return Identity{Problem: answer.Problem}
	}
	return Identity{Handle: answer.Handle, Listed: listed(answer.Handle)}
}

// Waiting is an item and the dependencies it waits on, not yet done.
type Waiting struct {
	Item *value.Map
	On   []any
}

// Proposal is what a person can take (work.ts's propose): their items in
// progress, theirs that can start, the unowned that can, theirs that wait
// and on what, the ideas they may look at and the deferred ones. Person is
// "" for nobody. Every is a session that owns every item, whatever owner
// it names (a stealth config with no --as), and then Person is "".
type Proposal struct {
	Person               string
	Every                bool
	Doing, Next, Unowned []*value.Map
	Waiting, Ideas       []Waiting
	Deferred             []*value.Map
}

// absent is whether a value is JavaScript's null or undefined.
func absent(v any) bool { return v == nil || v == value.Undefined }

// Propose is the proposal for handle ("" for nobody) over a sound registry:
// `todo` items, every dependency done, theirs first (the item's owner, else
// its group's), then the unowned ones in groups nobody owns. Ideas (kind
// idea) and deferred items (deferred: <reason>) are apart, among the ones
// the person may look at: theirs, or nobody's. Each list is in the queue's
// order, the items it does not name after those it does, in the registry's
// (slice 66); what another person owns is in no list, so each person sees
// their part of the queue.
func Propose(r Registry, handle string) Proposal { return propose(r, handle, false) }

// ProposeEvery is the proposal for a session that owns every item, whoever
// an item or its group names: a stealth config's, the person being the only
// one. Nothing is unowned to it, so every item that can start is its own.
func ProposeEvery(r Registry) Proposal { return propose(r, "", true) }

// propose is Propose for handle, or, with every, for a session that owns
// every item.
func propose(r Registry, handle string, every bool) Proposal {
	done := map[string]bool{}
	for _, item := range r.Items {
		if item.At("status") == "done" {
			done[key(item.At("id"))] = true
		}
	}
	open := func(item *value.Map) []any {
		on := []any{}
		deps, _ := item.At("depends_on").([]any)
		for _, dep := range deps {
			if !done[key(dep)] {
				on = append(on, dep)
			}
		}
		return on
	}
	ownerOf := func(item *value.Map) any {
		if owner := item.At("owner"); !absent(owner) {
			return owner
		}
		if owner := r.Phases.At(value.String(item.At("phase"))); !absent(owner) {
			return owner
		}
		return nil
	}
	mine := func(item *value.Map) bool {
		if every {
			return true
		}
		owner, ok := ownerOf(item).(string)
		return handle != "" && ok && owner == handle
	}
	unowned := func(item *value.Map) bool { return !every && ownerOf(item) == nil }
	idea := func(item *value.Map) bool { return item.At("kind") == "idea" }
	deferred := func(item *value.Map) bool { return item.At("deferred") != value.Undefined }
	p := Proposal{Person: handle, Every: every, Doing: []*value.Map{}, Next: []*value.Map{}, Unowned: []*value.Map{},
		Waiting: []Waiting{}, Ideas: []Waiting{}, Deferred: []*value.Map{}}
	items := inQueueOrder(r)
	for _, item := range items {
		if item.At("status") == "doing" && mine(item) {
			p.Doing = append(p.Doing, item)
		}
	}
	var todo, theirs []*value.Map
	for _, item := range items {
		if item.At("status") != "todo" {
			continue
		}
		if mine(item) || unowned(item) {
			theirs = append(theirs, item)
		}
		if !idea(item) && !deferred(item) {
			todo = append(todo, item)
		}
	}
	for _, item := range todo {
		if mine(item) && len(open(item)) == 0 {
			p.Next = append(p.Next, item)
		}
	}
	for _, item := range todo {
		if unowned(item) && len(open(item)) == 0 {
			p.Unowned = append(p.Unowned, item)
		}
	}
	for _, item := range todo {
		if on := open(item); mine(item) && len(on) > 0 {
			p.Waiting = append(p.Waiting, Waiting{item, on})
		}
	}
	for _, item := range theirs {
		if deferred(item) {
			p.Deferred = append(p.Deferred, item)
		} else if idea(item) {
			p.Ideas = append(p.Ideas, Waiting{item, open(item)})
		}
	}
	return p
}

// Startable is what the proposal's person can start, their own and the
// unowned together, in the queue's order and the unqueued after it in the
// registry's, as itos status lists the next items (slice 67): the queue is
// one order for the whole repository, so an unowned item it puts first comes
// before one of the person's it puts later.
func Startable(r Registry, p Proposal) []*value.Map {
	can := map[*value.Map]bool{}
	for _, item := range append(slices.Clone(p.Next), p.Unowned...) {
		can[item] = true
	}
	all := []*value.Map{}
	for _, item := range inQueueOrder(r) {
		if can[item] {
			all = append(all, item)
		}
	}
	return all
}

// inQueueOrder are the registry's items, those the queue names first, in its
// order, then the rest in the registry's.
func inQueueOrder(r Registry) []*value.Map {
	queued := Queued(r)
	if len(queued) == 0 {
		return r.Items
	}
	place := map[string]int{}
	for i, id := range queued {
		place[id] = i
	}
	ordered := slices.Clone(r.Items)
	slices.SortStableFunc(ordered, func(a, b *value.Map) int {
		at := func(item *value.Map) int {
			if i, ok := place[value.String(item.At("id"))]; ok {
				return i
			}
			return len(queued)
		}
		return at(a) - at(b)
	})
	return ordered
}

// Fields are the proposal as --json prints it, its keys in work.ts's order,
// with every_item: true after person when the session owns every item (a
// key only that proposal has, so a project's is unchanged).
func (p Proposal) Fields() []out.Field {
	var person any
	if p.Person != "" {
		person = p.Person
	}
	items := func(list []*value.Map) []any {
		all := []any{}
		for _, item := range list {
			all = append(all, item)
		}
		return all
	}
	waiting := func(list []Waiting) []any {
		all := []any{}
		for _, w := range list {
			all = append(all, value.NewMap("item", w.Item, "on", w.On))
		}
		return all
	}
	fields := []out.Field{{Key: "person", Value: person}}
	if p.Every {
		fields = append(fields, out.Field{Key: "every_item", Value: true})
	}
	return append(fields, []out.Field{
		{Key: "doing", Value: items(p.Doing)},
		{Key: "next", Value: items(p.Next)},
		{Key: "unowned", Value: items(p.Unowned)},
		{Key: "waiting", Value: waiting(p.Waiting)},
		{Key: "ideas", Value: waiting(p.Ideas)},
		{Key: "deferred", Value: items(p.Deferred)},
	}...)
}

// line is an item as the proposal prints it: its id, its title and, when it
// has one, its issue.
func line(item *value.Map) string {
	s := "  " + value.String(item.At("id")) + "  " + value.String(item.At("title"))
	if issue := item.At("issue"); value.Truthy(issue) {
		s += "  (#" + value.String(issue) + ")"
	}
	return s
}

// join is the values joined as Array.join joins them: null and undefined
// as nothing.
func join(list []any, sep string) string {
	parts := make([]string, len(list))
	for i, v := range list {
		if !absent(v) {
			parts[i] = value.String(v)
		}
	}
	return strings.Join(parts, sep)
}

// Print writes the proposal as text: who it is for, then each section that
// has an item.
func (p Proposal) Print(w io.Writer) {
	switch {
	case p.Every:
		fmt.Fprintln(w, "Working for you: under a stealth config every item is yours.")
	case p.Person != "":
		fmt.Fprintf(w, "Working for %s.\n", p.Person)
	default:
		fmt.Fprintln(w, "Working for nobody.")
	}
	section := func(title string, items []*value.Map) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s\n", title)
		for _, item := range items {
			fmt.Fprintln(w, line(item))
		}
	}
	section("In progress:", p.Doing)
	section("Can start now:", p.Next)
	section("Unowned, can start now (agree an owner first):", p.Unowned)
	if len(p.Waiting) > 0 {
		fmt.Fprintln(w, "\nWaiting:")
		for _, wait := range p.Waiting {
			fmt.Fprintf(w, "%s  ← %s\n", line(wait.Item), join(wait.On, ", "))
		}
	}
	if len(p.Ideas) > 0 {
		fmt.Fprintln(w, "\nIdeas, not yet specified (a coordinator specifies one before it starts):")
		for _, idea := range p.Ideas {
			on := ""
			if len(idea.On) > 0 {
				on = "  ← " + join(idea.On, ", ")
			}
			fmt.Fprintln(w, line(idea.Item)+on)
		}
	}
	if len(p.Deferred) > 0 {
		fmt.Fprintln(w, "\nDeferred:")
		for _, item := range p.Deferred {
			fmt.Fprintf(w, "%s  — %s\n", line(item), value.Trim(value.String(item.At("deferred"))))
		}
	}
}
