package next

import (
	"context"
	"regexp"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
)

// Facts are what next reads of an item, each where its existing reader
// reads it: the Item's registry entry as written (work.Load's mapping, its
// depends_on and tags lists added when absent); its Kind, slice, task, idea
// or bug, "" for none; its Dependencies, each with the status the registry
// gives it; the open Questions naming it; a slice's or a bug's tagged
// Scenarios at HEAD, @wip ones included; and a task's Checks, its ledger
// entry's done_when, none for a task the ledger lacks.
type Facts struct {
	Item         *value.Map
	Kind         string
	Dependencies []Dependency
	Questions    []ask.Question
	Scenarios    []tests.Test
	Checks       []ledger.Check
}

// Dependency is an item of the item's depends_on: its ID and the Status the
// registry gives it, nil for an item the registry lacks or one with none.
type Dependency struct {
	ID     string
	Status any
}

// Done is whether the dependency's status is done.
func (d Dependency) Done() bool { return d.Status == "done" }

// read is the Facts of the item id, over cfg and the tracker the questions
// are in; refused is the problem of a registry that is not there or an id
// no item has (work.Missing, work.Show), nothing read past it.
func read(ctx context.Context, cfg *config.Loaded, tracker trackerport.Tracker, id string) (f Facts, refused *out.Problem, err error) {
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		missing := work.Missing(file, false)
		return Facts{}, &missing, nil
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return Facts{}, nil, err
	}
	shown, problem := work.Show(registry, id)
	if problem != nil {
		return Facts{}, problem, nil
	}
	f.Item = shown.Item
	f.Kind = kindOf(shown.Item, cfg)
	f.Dependencies = dependencies(registry, shown.Item)
	asks, _, _, err := ask.Load(ctx, tracker, cfg.Work.Asks)
	if err != nil {
		return Facts{}, nil, err
	}
	for _, q := range asks.About(id) {
		if q.Status() == ask.Open {
			f.Questions = append(f.Questions, q)
		}
	}
	switch f.Kind {
	case "slice", "bug":
		f.Scenarios, err = tests.Tagged(cfg, "@"+id, "HEAD")
		if err != nil {
			return Facts{}, nil, err
		}
	case "task":
		f.Checks, err = checks(cfg, id)
		if err != nil {
			return Facts{}, nil, err
		}
	}
	return f, nil, nil
}

// sliceID and bugID are the ids of a slice and a bug, as the registry's
// header names them.
var (
	sliceID = regexp.MustCompile(`^slice-\d+$`)
	bugID   = regexp.MustCompile(`^bug-\d+$`)
)

// kindOf is the item's kind: its kind key as written when it is text, else
// by its id (next's own rule: work show reads no bug), slice-<n> a slice,
// bug-<n> a bug, an id ledger.id matches a task, where the config has a
// ledger.id; "" for none.
func kindOf(item *value.Map, cfg *config.Loaded) string {
	if kind, ok := item.At("kind").(string); ok {
		return kind
	}
	id, _ := item.At("id").(string)
	switch {
	case sliceID.MatchString(id):
		return "slice"
	case bugID.MatchString(id):
		return "bug"
	case cfg.Ledger.ID != nil && ledger.IDPattern(cfg).MatchString(id):
		return "task"
	}
	return ""
}

// dependencies are the item's depends_on, each with its status in the
// registry, the first item of an id giving it, as work.ItemStatuses reads
// them.
func dependencies(r work.Registry, item *value.Map) []Dependency {
	statuses := map[string]any{}
	for _, other := range r.Items {
		id := value.String(other.At("id"))
		if _, seen := statuses[id]; !seen {
			statuses[id] = other.At("status")
		}
	}
	deps, _ := item.At("depends_on").([]any)
	found := make([]Dependency, 0, len(deps))
	for _, dep := range deps {
		id := value.String(dep)
		status := statuses[id]
		if status == value.Undefined {
			status = nil
		}
		found = append(found, Dependency{ID: id, Status: status})
	}
	return found
}

// checks are the done_when checks of the ledger's task id, none for a task
// the ledger lacks; a ledger that cannot be read is its error.
func checks(cfg *config.Loaded, id string) ([]ledger.Check, error) {
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t.DoneWhen, nil
		}
	}
	return nil, nil
}
