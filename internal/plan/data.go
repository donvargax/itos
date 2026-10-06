package plan

import (
	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/ledger"
	"github.com/donvargax/itos/v6/internal/source"
	"github.com/donvargax/itos/v6/internal/value"
	"github.com/donvargax/itos/v6/internal/work"
)

// Data is what a push's plan reads besides the range's commits: the ledger's
// tasks, the registry's statuses and the smoke set.
type Data struct {
	Tasks    []ledger.Task
	Statuses work.Statuses
	Smoke    []string
}

// DataAt is a plan's data from the working tree, or with at from that
// commit's tree (`ci plan --data-at`): the ledger (ledger.files), the
// registry (work.registry) and the smoke set. The config is always the
// working tree's. Every work key has a default, so a config without a work
// section reads the registry at its default path, as itos work does (bug
// 10); a registry that is not there has no items. The ledger's folder
// missing is one problem naming it.
func DataAt(cfg *config.Loaded, at string) (Data, error) {
	var d Data
	read := func() error {
		tasks, err := ledger.Tasks(cfg)
		d.Tasks = tasks
		d.Statuses = work.ItemStatuses(cfg.Work.Registry)
		return err
	}
	var err error
	if at == "" {
		err = read()
	} else {
		var tree source.Source
		if tree, err = source.At(at); err == nil {
			err = source.ReadingFrom(tree, read)
		}
	}
	if err != nil {
		return Data{}, err
	}
	d.Smoke, err = Smoke(cfg, at)
	return d, err
}

// waiting is whether a task's work item is in ci.wait_on_status: nobody has
// started it, so its checks cannot pass yet.
func waiting(cfg *config.Loaded, d Data, id string) bool {
	status, ok := d.Statuses.Of(id)
	return ok && value.Includes(cfg.CI.WaitOnStatus, status)
}

// For is the plan for a pushed range (ci-plan.ts's planWith): its commits
// read from git, the rest from d. The tasks its ledger footers name run in
// ledger order, less the ones not started; a named ID no task has is
// Unknown, and a task not started is NotStarted, both in the footers' order.
// The range's ends are filled into its steps (EndsOf).
func For(cfg *config.Loaded, from, to string, d Data) (*Plan, error) {
	ids := TasksIn(cfg, from, to)
	prose, err := DocsOnly(cfg, Changed(from, to))
	if err != nil {
		return nil, err
	}
	known := Readable(from, to)
	kind, err := TestsKind(cfg)
	if err != nil {
		return nil, err
	}
	in := Input{Prose: prose, Known: known, Scenarios: TestsNamedIn(cfg, from, to, kind), Smoke: d.Smoke}
	has := map[string]bool{}
	for _, t := range d.Tasks {
		has[t.ID] = true
		if value.Includes(ids, t.ID) && !waiting(cfg, d, t.ID) {
			in.Tasks = append(in.Tasks, t)
		}
	}
	p, err := Make(cfg, in)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if !has[id] {
			p.Unknown = append(p.Unknown, id)
		}
		if waiting(cfg, d, id) {
			p.NotStarted = append(p.NotStarted, id)
		}
	}
	p.fill(EndsOf(cfg, from, to))
	return p, nil
}
