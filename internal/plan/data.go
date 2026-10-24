package plan

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/proof"
	"github.com/donvargax/itos/v8/internal/value"
	"github.com/donvargax/itos/v8/internal/work"
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
	data := cfg
	if at != "" {
		tree, err := cfg.Trees.At(context.Background(), at)
		if err != nil {
			return Data{}, err
		}
		data = cfg.DataFrom(tree)
	}
	tasks, err := ledger.Tasks(data)
	d.Tasks = tasks
	d.Statuses = work.ItemStatuses(data.Source, data.Work.Registry)
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
// The range's ends are filled into its steps (EndsOf), and its code proof,
// when the config has one and the range touches its paths (codeProof), runs
// with {base} that start.
func For(cfg *config.Loaded, from, to string, d Data) (*Plan, error) {
	ids := TasksIn(cfg, from, to)
	changed, readErr := ChangedErr(cfg.VCS, from, to)
	prose, err := DocsOnly(cfg, changed)
	if err != nil {
		return nil, err
	}
	known := Readable(cfg.VCS, from, to)
	kind, err := TestsKind(cfg)
	if err != nil {
		return nil, err
	}
	in := Input{Prose: prose, Known: known, Scenarios: TestsNamedIn(cfg, from, to, kind), Smoke: d.Smoke}
	ends := EndsOf(cfg, from, to)
	var omitted string
	if in.Proof, omitted, err = codeProof(cfg, from, changed, readErr, ends); err != nil {
		return nil, err
	}
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
	p.ProofOmitted = omitted
	p.fill(ends)
	return p, nil
}

// The lines that say why a configured code proof is not a step of a push's
// plan (slice 155, issues #35 and #36), one for each reason.
const (
	proofNoStart    = "Code proof not scheduled: the range has no start."
	proofUnreadable = "Code proof not scheduled: the range's changes could not be read."
	proofNoPath     = "Code proof not scheduled: the range changes no path in proof.code.paths."
)

// codeProof is the range's code proof as a step of its plan: with the
// config's proof.code and the range touching proof.code.paths, its check
// with {base} the range's start (ends.From), so every commit since the last
// green run is judged, whoever's item it is (slice 106, issue #28, which
// settles #26). None with no proof.code. None too, with the line that says
// why, when it is left out, for the first reason that holds: the range as
// typed (from) has no start, even when commits.since gives range checks one,
// since its paths are never compared and no read failed; its changes could
// not be read (readErr), so no touched path is guessed; or it touches none
// of the paths.
func codeProof(cfg *config.Loaded, from string, changed []string, readErr error, ends Ends) (*Step, string, error) {
	if cfg.Proof == nil || cfg.Proof.Code == nil {
		return nil, "", nil
	}
	switch {
	case config.NewBranch(from) || ends.From == "":
		return nil, proofNoStart, nil
	case readErr != nil:
		return nil, proofUnreadable, nil
	}
	touched, err := proof.Touches(cfg.Proof.Code.Paths, changed)
	if err != nil {
		return nil, "", err
	}
	if !touched {
		return nil, proofNoPath, nil
	}
	return &Step{Command: proof.Command(cfg.Proof.Code.Check, ends.From), Proof: true}, "", nil
}
