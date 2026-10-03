package work

import (
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/providers"
	"github.com/donvargax/itos/v2/internal/value"
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
// "" for nobody.
type Proposal struct {
	Person               string
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
// the person may look at: theirs, or nobody's.
func Propose(r Registry, handle string) Proposal {
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
		owner, ok := ownerOf(item).(string)
		return handle != "" && ok && owner == handle
	}
	unowned := func(item *value.Map) bool { return ownerOf(item) == nil }
	idea := func(item *value.Map) bool { return item.At("kind") == "idea" }
	deferred := func(item *value.Map) bool { return item.At("deferred") != value.Undefined }
	p := Proposal{Person: handle, Doing: []*value.Map{}, Next: []*value.Map{}, Unowned: []*value.Map{},
		Waiting: []Waiting{}, Ideas: []Waiting{}, Deferred: []*value.Map{}}
	for _, item := range r.Items {
		if item.At("status") == "doing" && mine(item) {
			p.Doing = append(p.Doing, item)
		}
	}
	var todo, theirs []*value.Map
	for _, item := range r.Items {
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

// Fields are the proposal as --json prints it, its keys in work.ts's order.
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
	return []out.Field{
		{Key: "person", Value: person},
		{Key: "doing", Value: items(p.Doing)},
		{Key: "next", Value: items(p.Next)},
		{Key: "unowned", Value: items(p.Unowned)},
		{Key: "waiting", Value: waiting(p.Waiting)},
		{Key: "ideas", Value: waiting(p.Ideas)},
		{Key: "deferred", Value: items(p.Deferred)},
	}
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
	if p.Person != "" {
		fmt.Fprintf(w, "Working for %s.\n", p.Person)
	} else {
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
