// Package work is the work registry (tools/itos/work.ts): work.registry,
// work-items.yaml in the ledger's folder by default, which says who owns each
// group and each work item, its status and what it waits on. The port has the
// registry's reading and its problems, which config check and work check
// report, and the proposal `work` prints (propose.go).
package work

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/out"
	"github.com/donvargax/itos/v3/internal/providers"
	"github.com/donvargax/itos/v3/internal/source"
	"github.com/donvargax/itos/v3/internal/value"
)

// kinds are the kinds an item may have.
var kinds = []string{"slice", "task", "idea"}

// Dropped is the status work drop gives an item taken out of the open work
// (slice 78): a status itos knows whatever work.statuses lists, since itos
// alone sets it, as a project lists the statuses its own items move
// through. A dropped item stays in the registry, so its id is never given
// again, and nothing proposes it, takes it or closes it.
const Dropped = "dropped"

// live is whether an item is still to be done: neither done nor dropped.
func live(item *value.Map) bool {
	status := item.At("status")
	return status != "done" && status != Dropped
}

// Registry is the registry as read: the people's logins, whether there are
// people to hold owners to, each group's owner (by the key work.groups_key
// names) and the items, each with its depends_on, a list (none when it gives
// none). An item is the mapping as written, read with JavaScript's leniency,
// since the registry's problems are what is wrong with it.
type Registry struct {
	Logins []string
	// People is whether Logins are who may own work: false under a stealth
	// config, which reads no people file, the person being the only one, and
	// in a project whose people file is missing or cannot be read, which is
	// config check's to report (PeopleProblem) and no other command's. With
	// no people, any owner is one and any session is listed.
	People bool
	Phases *value.Map
	Items  []*value.Map
	// Queue is the registry's queue: as written, a list of item ids when it
	// is sound, the order the work comes in (slice 66); Undefined when it
	// has none.
	Queue any
}

// Load reads the registry at path and the people of the config's source.
func Load(cfg *config.Loaded, path string) (Registry, error) {
	text, err := source.Read(path)
	if err != nil {
		return Registry{}, err
	}
	raw, err := value.Parse(text)
	if err != nil {
		return Registry{}, err
	}
	if raw == nil {
		return Registry{}, fmt.Errorf("Cannot read properties of null (reading '%s')", cfg.Work.GroupsKey)
	}
	phases, err := groups(value.Prop(raw, cfg.Work.GroupsKey))
	if err != nil {
		return Registry{}, err
	}
	var items []*value.Map
	listed := value.Prop(raw, "items")
	if listed != nil && listed != value.Undefined {
		list, ok := listed.([]any)
		if !ok {
			return Registry{}, errors.New("(raw.items ?? []).map is not a function")
		}
		for _, item := range list {
			copied, ok := value.Copy(item).(*value.Map)
			if item == nil {
				return Registry{}, errors.New("Cannot read properties of null (reading 'depends_on')")
			}
			if !ok {
				copied = value.NewMap()
			}
			if deps := copied.At("depends_on"); deps == nil || deps == value.Undefined {
				copied.Set("depends_on", []any{})
			}
			items = append(items, copied)
		}
	}
	r := Registry{Phases: phases, Items: items, Queue: value.Copy(value.Prop(raw, "queue"))}
	if !cfg.Stealth {
		if logins, err := providers.People(cfg.Work.People); err == nil {
			r.Logins, r.People = logins, true
		}
	}
	return r, nil
}

// PeopleProblem is what config check warns of when a project's people file
// is missing or cannot be read, nil when it reads or the config is the
// stealth one, which reads none. It is a warning, never a failure: without
// the people, owners go unchecked and work proposes for whoever the session
// is.
func PeopleProblem(cfg *config.Loaded) *out.Problem {
	if cfg.Stealth {
		return nil
	}
	file := cfg.Work.People.File
	if !source.Has(file) {
		return &out.Problem{
			Rule:    "work-people-missing",
			Message: "no people file at " + file + " (work.people), so the registry's owners go unchecked",
			Fix:     "list the people who may own work in " + file + ", or set work.people.file to where they are",
		}
	}
	if _, err := providers.People(cfg.Work.People); err != nil {
		return &out.Problem{
			Rule:    "work-people-unreadable",
			Message: err.Error() + ", so the registry's owners go unchecked",
			Fix:     "correct " + file,
		}
	}
	return nil
}

// groups are the registry's groups and their owners, a mapping; a list reads
// as one keyed by index.
func groups(v any) (*value.Map, error) {
	if v == nil || v == value.Undefined {
		return value.NewMap(), nil
	}
	switch g := v.(type) {
	case *value.Map:
		return g, nil
	case []any:
		m := value.NewMap()
		for i, owner := range g {
			m.Set(fmt.Sprint(i), owner)
		}
		return m, nil
	}
	return nil, fmt.Errorf("the registry's groups are not a mapping: %s", value.String(v))
}

// key is a value as a JavaScript Map tells keys apart.
func key(v any) string { return value.TypeOf(v) + ":" + value.JSON(v) }

func problem(rule, message, fix string) out.Problem {
	return out.Problem{Rule: rule, Message: message, Fix: fix}
}

// text is whether a value is text with something in it.
func text(v any) bool {
	s, ok := v.(string)
	return ok && value.Trim(s) != ""
}

// checker holds what every item's problems are judged against.
type checker struct {
	cfg      *config.Loaded
	handles  map[string]bool
	byID     map[string]*value.Map
	order    []string // byID's keys, first seen first
	listedIn string
	// anyone is whether any owner is one: there are no people to hold
	// owners to (Registry.People).
	anyone bool
}

func (c checker) owned(owner any) bool {
	if c.anyone {
		return true
	}
	s, ok := owner.(string)
	return ok && c.handles[s]
}

func (c checker) itemIssues(item *value.Map) ([]out.Problem, error) {
	id := value.String(item.At("id"))
	status := item.At("status")
	kind := item.At("kind")
	var found []out.Problem
	add := func(holds bool, rule, message, fix string) {
		if holds {
			found = append(found, problem(rule, message, fix))
		}
	}
	add(!value.Truthy(item.At("title")), "work-no-title", id+": no title", "give "+id+" a title")
	statuses := c.cfg.Work.Statuses
	add(status != Dropped && !value.Includes(statuses, status), "work-unknown-status",
		fmt.Sprintf("%s: unknown status \"%s\"", id, value.String(status)),
		fmt.Sprintf("set %s's status to one of %s", id, strings.Join(statuses, ", ")))
	add(kind != value.Undefined && !value.Includes(kinds, kind), "work-unknown-kind",
		fmt.Sprintf("%s: unknown kind \"%s\"", id, value.String(kind)),
		fmt.Sprintf("set %s's kind to one of %s", id, strings.Join(kinds, ", ")))
	// An idea is specified (kind slice or task) before anyone takes it; one
	// dropped was never started.
	add(kind == "idea" && status != "todo" && status != Dropped, "work-idea-started",
		fmt.Sprintf("%s: an idea is %s; specify it first (kind slice or task)", id, value.String(status)),
		fmt.Sprintf("specify %s (kind: slice or task, with its scenarios or ledger entry), or set it back to todo", id))
	why, deferred := item.At("why"), item.At("deferred")
	add(why != value.Undefined && !text(why), "work-why-not-text",
		id+": why is not a text", "write "+id+"'s why as text, or remove it")
	add(deferred != value.Undefined && !text(deferred), "work-deferred-no-reason",
		id+": deferred needs its reason", "write why "+id+" is deferred in its deferred:, or remove the key")
	add(deferred != value.Undefined && status != "todo" && status != Dropped, "work-deferred-started",
		fmt.Sprintf("%s: deferred, but %s", id, value.String(status)),
		fmt.Sprintf("remove %s's deferred:, or set it back to todo", id))
	owner := item.At("owner")
	add(owner != nil && !c.owned(owner), "work-unknown-owner",
		fmt.Sprintf("%s: owner \"%s\" is not in %s", id, value.String(owner), c.listedIn),
		fmt.Sprintf("add %s to %s, or set %s's owner to one of its logins", value.String(owner), c.listedIn, id))
	deps, ok := item.At("depends_on").([]any)
	if !ok {
		return nil, errors.New("item.depends_on.flatMap is not a function")
	}
	for _, dep := range deps {
		other, known := c.byID[key(dep)]
		if !known {
			found = append(found, problem("work-unknown-dependency",
				fmt.Sprintf("%s: depends on unknown \"%s\"", id, value.String(dep)),
				fmt.Sprintf("remove \"%s\" from %s's depends_on, or add an item %s", value.String(dep), id, value.String(dep))))
			continue
		}
		add(status == "done" && other.At("status") != "done", "work-done-before-dependency",
			fmt.Sprintf("%s: done, but \"%s\" is %s", id, value.String(dep), value.String(other.At("status"))),
			fmt.Sprintf("finish %s first, or set %s back to doing", value.String(dep), id))
		// An item still to do that waits on a dropped one waits for ever
		// (slice 78).
		add(live(item) && other.At("status") == Dropped, "work-dropped-dependency",
			fmt.Sprintf("%s: depends on \"%s\", which is dropped and will never be done", id, value.String(dep)),
			fmt.Sprintf("remove \"%s\" from %s's depends_on (itos work edit %s --depends-on …), or drop %s too", value.String(dep), id, id, id))
	}
	return found, nil
}

// cycles is every cycle: an item reached again while walking its own
// dependencies.
func (c checker) cycles(file string) []out.Problem {
	var found []out.Problem
	state := map[string]string{}
	var walk func(id any, path []any)
	walk = func(id any, path []any) {
		k := key(id)
		if state[k] == "walking" {
			start := 0
			for i, p := range path {
				if key(p) == k {
					start = i
					break
				}
			}
			names := []string{}
			for _, p := range append(append([]any{}, path[start:]...), id) {
				if p == nil || p == value.Undefined {
					names = append(names, "")
				} else {
					names = append(names, value.String(p))
				}
			}
			loop := strings.Join(names, " → ")
			found = append(found, problem("work-cycle", "cycle: "+loop,
				"remove one of the depends_on links "+loop+" in "+file))
		}
		if _, seen := state[k]; seen {
			return
		}
		state[k] = "walking"
		if item, ok := c.byID[k]; ok {
			deps, _ := item.At("depends_on").([]any)
			for _, dep := range deps {
				walk(dep, append(append([]any{}, path...), id))
			}
		}
		state[k] = "done"
	}
	for _, k := range c.order {
		walk(c.byID[k].At("id"), nil)
	}
	return found
}

// Issues are every problem with the registry, each with its rule; none means
// it is sound.
func Issues(cfg *config.Loaded, r Registry, file string) ([]out.Problem, error) {
	c := checker{cfg: cfg, handles: map[string]bool{}, byID: map[string]*value.Map{}, listedIn: cfg.Work.People.File, anyone: !r.People}
	for _, login := range r.Logins {
		c.handles[login] = true
	}
	for _, item := range r.Items {
		k := key(item.At("id"))
		if _, ok := c.byID[k]; !ok {
			c.order = append(c.order, k)
		}
		c.byID[k] = item
	}
	// A group, as the messages name it: by ledger.group.label.
	group := func(g any) string { return cfg.Ledger.Group.Label + " " + value.String(g) }
	var found []out.Problem
	firstOf := map[string]int{}
	for i, item := range r.Items {
		k := key(item.At("id"))
		if _, ok := firstOf[k]; !ok {
			firstOf[k] = i
		}
	}
	for i, item := range r.Items {
		if firstOf[key(item.At("id"))] != i {
			id := value.String(item.At("id"))
			found = append(found, problem("work-duplicate-id", id+": listed twice", "rename or remove one of the two "+id+" items"))
		}
	}
	for _, item := range r.Items {
		phase := item.At("phase")
		if !r.Phases.Has(value.String(phase)) {
			id := value.String(item.At("id"))
			found = append(found, problem("work-unknown-phase",
				id+": "+group(phase)+" is not listed",
				"add "+group(phase)+" to "+cfg.Work.GroupsKey+": in "+file+", or move "+id+" to a listed one"))
		}
	}
	for _, item := range r.Items {
		own, err := c.itemIssues(item)
		if err != nil {
			return nil, err
		}
		found = append(found, own...)
	}
	for _, phase := range r.Phases.Keys() {
		owner := r.Phases.At(phase)
		if owner != nil && !c.owned(owner) {
			found = append(found, problem("work-unknown-phase-owner",
				fmt.Sprintf("%s: owner \"%s\" is not in %s", group(phase), value.String(owner), c.listedIn),
				fmt.Sprintf("add %s to %s, or set %s's owner to one of its logins", value.String(owner), c.listedIn, group(phase))))
		}
	}
	found = append(found, c.queueIssues(r.Queue, file)...)
	return append(found, c.cycles(file)...), nil
}

// queueIssues are the queue's problems (slice 66): a queue that is not a
// list, and each id in it that no item has or that it names a second time.
// No queue, or a null one, is an empty one.
func (c checker) queueIssues(queue any, file string) []out.Problem {
	if absent(queue) {
		return nil
	}
	list, ok := queue.([]any)
	if !ok {
		return []out.Problem{problem("work-queue-not-list", "queue: is not a list of item ids",
			"write queue: in "+file+" as a list of item ids, or remove it")}
	}
	var found []out.Problem
	seen := map[string]bool{}
	for _, entry := range list {
		k, id := key(entry), value.String(entry)
		if _, known := c.byID[k]; !known {
			found = append(found, problem("work-queue-unknown-item",
				fmt.Sprintf("queue: names unknown \"%s\"", id),
				fmt.Sprintf("remove \"%s\" from queue: in %s, or add an item %s", id, file, id)))
		} else if seen[k] {
			found = append(found, problem("work-queue-twice",
				fmt.Sprintf("queue: names \"%s\" twice", id),
				fmt.Sprintf("remove one of the two \"%s\" from queue: in %s", id, file)))
		}
		seen[k] = true
	}
	return found
}

// Problems are the registry's problems, or that there is none where itos
// looks: a project whose registry is at the old default (docs/work-items.yaml)
// learns where itos reads it now, and how to say otherwise.
func Problems(cfg *config.Loaded) ([]out.Problem, error) {
	return ProblemsAt(cfg, cfg.Work.Registry, false)
}

// ProblemsAt are the problems of the registry at file, the config's
// work.registry or, named, a file given on the command line, whose fix when
// it is not there says so.
func ProblemsAt(cfg *config.Loaded, file string, named bool) ([]out.Problem, error) {
	if !source.Has(file) {
		return []out.Problem{Missing(file, named)}, nil
	}
	r, err := Load(cfg, file)
	if err != nil {
		return nil, err
	}
	return Issues(cfg, r, file)
}

// Missing is the problem of no registry at file: the config's work.registry,
// or a file named on the command line.
func Missing(file string, named bool) out.Problem {
	fix := "move the registry to " + file + ", or set work.registry to where it is"
	if named {
		fix = "name the registry's file, or write one at " + file
	}
	return problem("work-registry-missing", "no work registry at "+file, fix)
}
