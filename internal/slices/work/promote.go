package work

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// Promoted is what work promote made of an idea: the id it Was, the Kind it
// became under ID, the registry's Change, a task's Ledger entry (nil for a
// slice) and the questions Renamed to the new id.
type Promoted struct {
	Was, Kind, ID string
	Change        domain.Change
	Ledger        *ledger.Added
	Renamed       []string
}

// promote is `work promote <idea> --kind slice [--title <title>]` and `work
// promote <idea> --kind task --type <type> --check <command> [--timeout
// <seconds>]… [--title <title>] [--why <why>]` (slices 52, 65 and 138): the
// idea renamed, given a minted ID and the kind (and the title, when given),
// every depends_on and question (work.asks, bug 105) naming it renamed too,
// and, for a task, its ledger entry written, so the id works in a Task
// footer from the commit that makes it (#54): the idea's group, its title
// and why unless the line gives others, and the line's type and checks.
// Everything the command can refuse is judged before ItemID reserves the
// id, since a reserved id is never given back (ADR-0045): the line, the
// registry, the idea, the questions, the task's ledger entry and the files
// the commit would sweep in (record.Ready). The ledger file, the registry
// and the questions renamed are then committed together, or put back when
// the commit fails.
func (h Handler) promote(ctx context.Context, p Promotion, o Out) (Answer, error) {
	a, s, err := h.read(ctx, o)
	if s == nil {
		return a, err
	}
	idea, problem := domain.Promotable(s.registry, p.ID)
	if problem != nil {
		return stopped(a, asList(problem), nil)
	}
	asks, old, problem, err := promotionAsks(ctx, s, p.ID)
	if err != nil || problem != nil {
		return stopped(a, asList(problem), err)
	}
	files := []record.File{{Path: s.cfg.Work.Registry, Old: s.text}}
	if named := asks.Rename(p.ID, p.ID); len(named) > 0 {
		files = append(files, record.File{Path: s.cfg.Work.Asks, Old: old})
	}
	var task ledger.NewTask
	if p.Kind == "task" {
		var added ledger.Added
		var found []out.Problem
		if task, added, found, err = promotedTask(s.cfg, s.registry, idea, p, s.tracker.Load); err != nil || len(found) > 0 {
			return stopped(a, found, err)
		}
		files = append([]record.File{{Path: added.Path, Old: added.Old, Created: added.Created}}, files...)
	}
	if problem, err := record.Ready(ctx, s.tracker, s.cfg, files); err != nil || problem != nil {
		return stopped(a, asList(problem), err)
	}

	newID, err := h.ports.ItemID(ctx, s.cfg, p.Kind, ids.Items(s.registry))
	if err != nil {
		return a, err
	}
	change, problem, err := domain.Promote(s.registry, s.text, p.ID, newID, p.Kind, p.Title, ledger.IDPattern(s.cfg))
	if err != nil {
		return a, uneditable(s.cfg.Work.Registry, err)
	}
	if problem != nil {
		return stopped(a, asList(problem), nil)
	}
	body := strings.TrimSuffix(change.Body, ".")
	promoted := &Promoted{Was: p.ID, Kind: p.Kind, ID: newID, Change: change}
	if p.Kind == "task" {
		task.ID = newID
		added, found, err := ledger.Add(s.cfg, task, s.tracker.Load)
		if err != nil || len(found) > 0 {
			return stopped(a, found, err)
		}
		files[0].Text = added.Text
		body += ", and write its ledger entry, with its why and checks, in " + added.Path
		promoted.Ledger = &added
	}
	registryAt := slices.IndexFunc(files, func(f record.File) bool { return f.Path == s.cfg.Work.Registry })
	files[registryAt].Text = change.Text
	if promoted.Renamed = asks.Rename(p.ID, newID); len(promoted.Renamed) > 0 {
		files[len(files)-1].Text = ask.Text(asks)
		body += ", and rename it in " + and(promoted.Renamed) + " of " + s.cfg.Work.Asks
	}
	commit, found, err := writeAll(ctx, s, files, change.Header, body+".", o)
	if err != nil || len(found) > 0 {
		return stopped(a, found, err)
	}
	a.Commit, a.Promoted = commit, promoted
	return a, nil
}

// stopped is how a step that gives an error or problems ends work promote:
// the error as it is, else the problems refused (exit 1). The task's ledger
// entry, judged before the id is reserved and written after it, ends both
// ways through it (T-189).
func stopped(a Answer, found []out.Problem, err error) (Answer, error) {
	if err != nil {
		return a, err
	}
	a.Refused, a.Code = found, refused
	return a, nil
}

// promotionAsks is the questions' file, work.asks, and its text, read before
// an id is minted; an empty file when it is not there. The problem, which
// refuses the promote, is a file that cannot be read as itos decision writes
// it.
func promotionAsks(ctx context.Context, s *writing, id string) (ask.File, string, *out.Problem, error) {
	f, old, there, err := ask.Load(ctx, s.tracker, s.cfg.Work.Asks)
	if err != nil && there {
		return ask.File{}, "", &out.Problem{
			Rule:    "asks-invalid",
			Message: err.Error(),
			Fix:     "correct " + s.cfg.Work.Asks + " to what itos decision writes, then promote " + id + " again",
		}, nil
	}
	return f, old, nil, err
}

// promotedTask is the ledger entry a task's promotion writes, judged by
// ledger.Add under the id the ledger would give next (the counter not yet
// asked), so what it refuses is refused before an id is reserved: a type
// commits.types does not list, a ledger with problems, a group ledger.files
// cannot name. The task is the idea's group, its title and why unless the
// line gives others, and the line's type and checks; an idea with no group
// or no why, and no --why, is refused too.
func promotedTask(cfg *config.Loaded, registry domain.Registry, idea *value.Map, p Promotion, load ledger.Load) (ledger.NewTask, ledger.Added, []out.Problem, error) {
	refuse := func(rule, message, fix string) (ledger.NewTask, ledger.Added, []out.Problem, error) {
		return ledger.NewTask{}, ledger.Added{}, []out.Problem{{Rule: rule, Message: message, Fix: fix}}, nil
	}
	n := ledger.NewTask{Type: p.Type, Title: p.Title, Why: p.Why, Checks: p.Checks}
	switch phase := idea.At("phase").(type) {
	case string:
		n.Group = phase
	case float64:
		n.Group = value.Number(phase)
	default:
		label := cfg.Ledger.Group.Label
		return refuse("work-promote-no-group", fmt.Sprintf("%s names no %s, so its task has no ledger file", p.ID, label),
			"give "+p.ID+" a "+label+" in "+cfg.Work.Registry+", then promote it again")
	}
	if n.Title == "" {
		n.Title = value.String(idea.At("title"))
	}
	if n.Why == "" {
		why, _ := idea.At("why").(string)
		n.Why = value.Trim(why)
	}
	if n.Why == "" {
		return refuse("work-promote-no-why", p.ID+" has no why for its task's ledger entry",
			"pass --why <why>")
	}
	var err error
	if n.ID, err = ledger.NextID(cfg, ids.Items(registry)); err != nil {
		return ledger.NewTask{}, ledger.Added{}, nil, err
	}
	added, found, err := ledger.Add(cfg, n, load)
	return n, added, found, err
}
