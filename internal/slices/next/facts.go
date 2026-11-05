package next

import (
	"context"

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
// Scenarios, @wip ones included, at HEAD, or under push.mode pr at SpecRef,
// origin/<id> when that ref holds some, else HEAD ("" outside pr); and a
// task's Checks, its ledger entry's done_when, none for a task the ledger
// lacks, in the working tree, or under push.mode pr at SpecRef, origin/<id>,
// when the ledger there holds an entry and the task is neither done nor
// dropped, else with SpecRef "". Under push.mode pr, for an item in flight,
// Observed is what the forge and the attempt journal say, and Asked every
// question naming the item, in the questions file's order; outside, both are
// none.
type Facts struct {
	Item         *value.Map
	Kind         string
	Dependencies []Dependency
	Questions    []ask.Question
	Scenarios    []tests.Test
	SpecRef      string
	Checks       []ledger.Check
	Observed     *Observed
	Asked        []ask.Question
}

// Dependency is an item of the item's depends_on: its ID and the Status the
// registry gives it, nil for an item the registry lacks or one with none.
type Dependency struct {
	ID     string
	Status any
}

// Done is whether the dependency's status is done.
func (d Dependency) Done() bool { return d.Status == "done" }

// read is the Facts of the item id, over cfg, the tracker the questions are
// in and, under push.mode pr for an item in flight, the forge; refused is
// the problem of a registry that is not there or an id no item has
// (work.Missing, work.Show), nothing read past it, and failed that of a
// forge that cannot be asked (observe).
func read(ctx context.Context, cfg *config.Loaded, tracker trackerport.Tracker, forge Forge, id string) (f Facts, refused, failed *out.Problem, err error) {
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		missing := work.Missing(file, false)
		return Facts{}, &missing, nil, nil
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return Facts{}, nil, nil, err
	}
	shown, problem := work.Show(registry, id)
	if problem != nil {
		return Facts{}, problem, nil, nil
	}
	f.Item = shown.Item
	f.Kind = ledger.KindOf(shown.Item, cfg)
	f.Dependencies = dependencies(registry, shown.Item)
	asks, _, _, err := ask.Load(ctx, tracker, cfg.Work.Asks)
	if err != nil {
		return Facts{}, nil, nil, err
	}
	asked := asks.About(id)
	for _, q := range asked {
		if q.Status() == ask.Open {
			f.Questions = append(f.Questions, q)
		}
	}
	pr := cfg.Push.Mode == "pr"
	switch f.Kind {
	case "slice", "bug":
		f.Scenarios, f.SpecRef, err = scenarios(ctx, cfg, id, pr)
		if err != nil {
			return Facts{}, nil, nil, err
		}
	case "task":
		f.Checks, f.SpecRef, err = checks(ctx, cfg, shown.Item, id, pr)
		if err != nil {
			return Facts{}, nil, nil, err
		}
	}
	if !pr || Judge(f).State != inFlight {
		return f, nil, nil, nil
	}
	f.Asked = asked
	f.Observed, failed, err = observe(ctx, cfg, forge, registry, id)
	if err != nil || failed != nil {
		return Facts{}, nil, failed, err
	}
	return f, nil, nil, nil
}

// atBranch is an item's spec under push.mode pr and the ref it was read at:
// heldAtBranch's when it finds one, ref origin/<id>, else fallback's. A tree
// that cannot be read is at's error, never fallback's answer.
func atBranch[T any](ctx context.Context, cfg *config.Loaded, id string, at func(tree string) (T, bool, error), fallback func() (T, string, error)) (T, string, error) {
	found, commit, err := heldAtBranch(ctx, cfg, id, at)
	if err != nil || commit != "" {
		return found, "origin/" + id, err
	}
	return fallback()
}

// heldAtBranch is what at finds at origin/<id> and that ref's commit, when the
// clone has the ref, the one the last fetch or push left, nothing fetched, and
// at says its tree holds the item's spec; commit "" otherwise.
func heldAtBranch[T any](ctx context.Context, cfg *config.Loaded, id string, at func(tree string) (T, bool, error)) (T, string, error) {
	tree := "refs/remotes/origin/" + id
	if commit, err := cfg.VCS.Ref(ctx, tree); err == nil {
		found, holds, err := at(tree)
		if err != nil || holds {
			return found, commit, err
		}
	}
	var none T
	return none, "", nil
}

// SpecCommit is the commit of origin/<id> when next's reading finds the spec
// of an item of kind k there (heldAtBranch, scenariosAt, checksAt), under cfg,
// "" when it does not or k has no spec. It is for itos brief, which computes a
// brief at that commit (slice-226) and so decides for itself whether a done or
// dropped item reads it. An error is a tree that cannot be read.
func SpecCommit(ctx context.Context, cfg *config.Loaded, k, id string) (string, error) {
	var commit string
	var err error
	switch k {
	case "slice", "bug":
		_, commit, err = heldAtBranch(ctx, cfg, id, scenariosAt(cfg, id))
	case "task":
		_, commit, err = heldAtBranch(ctx, cfg, id, checksAt(ctx, cfg, id))
	}
	return commit, err
}

// scenarios are the scenarios tagged for the item and the ref they were
// read at: at HEAD outside push.mode pr, ref "" (slice-203); under it at
// origin/<id> when that ref holds some, else at HEAD (atBranch).
func scenarios(ctx context.Context, cfg *config.Loaded, id string, pr bool) ([]tests.Test, string, error) {
	if !pr {
		found, err := tests.Tagged(cfg, "@"+id, "HEAD")
		return found, "", err
	}
	return atBranch(ctx, cfg, id, scenariosAt(cfg, id), func() ([]tests.Test, string, error) {
		found, err := tests.Tagged(cfg, "@"+id, "HEAD")
		return found, "HEAD", err
	})
}

// scenariosAt reads the scenarios tagged for the item at a tree, which holds
// its spec when it tags any.
func scenariosAt(cfg *config.Loaded, id string) func(tree string) ([]tests.Test, bool, error) {
	return func(tree string) ([]tests.Test, bool, error) {
		found, err := tests.Tagged(cfg, "@"+id, tree)
		return found, len(found) > 0, err
	}
}

// checks are the task's ledger checks and the ref they were read at: in the
// working tree, ref "", outside push.mode pr and for a done or dropped task,
// whose branch may be stale; under pr at origin/<id> when the ledger there
// holds an entry for the task, else in the working tree (atBranch).
func checks(ctx context.Context, cfg *config.Loaded, item *value.Map, id string, pr bool) ([]ledger.Check, string, error) {
	inWorkingTree := func() ([]ledger.Check, string, error) {
		found, _, err := ledger.ChecksOf(cfg, id)
		return found, "", err
	}
	status := item.At("status")
	if !pr || status == "done" || status == work.Dropped {
		return inWorkingTree()
	}
	return atBranch(ctx, cfg, id, checksAt(ctx, cfg, id), inWorkingTree)
}

// checksAt reads the task's checks from the ledger at a tree, under cfg as
// attempt submit reads a head's, which holds its spec when it has an entry for
// the task, whatever its checks.
func checksAt(ctx context.Context, cfg *config.Loaded, id string) func(tree string) ([]ledger.Check, bool, error) {
	return func(tree string) ([]ledger.Check, bool, error) {
		source, err := cfg.Trees.At(ctx, tree)
		if err != nil {
			return nil, false, err
		}
		return ledger.ChecksOf(cfg.DataFrom(source), id)
	}
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
