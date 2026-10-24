package check

import (
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
)

// The cost rule (cost.ts), one for CI's plan and the commit-msg hook's run of
// the named tasks' checks: a step's or a check's cost class is its own cost:,
// else static when one of ci.cost.static's patterns matches its command
// (config.MatchesStatic, which also reads a first word that is hooks.bin as
// itos), else late; with ci.cost.keep_written_order, a check below a late
// check of its task is late too. config check's written-order rule reads the
// ledger as written, before it is typed, so it keeps its own reading of the
// same rule (ledger.orderFindings).

// Cost is a cost class.
type Cost string

const (
	Static Cost = "static"
	Late   Cost = "late"
)

// CostFrom is where a cost came from: the item's own cost:, a pattern, the
// default (late), or the order of the task's checks.
type CostFrom string

const (
	FromExplicit CostFrom = "explicit"
	FromPattern  CostFrom = "pattern"
	FromDefault  CostFrom = "default"
	FromOrder    CostFrom = "order"
)

// Costed is a cost class and where it came from.
type Costed struct {
	Cost Cost
	From CostFrom
}

// CostOf is a step's or check's cost: its own (own, "" for none), else the
// patterns', else late. The static commands need nothing built and take
// seconds; in doubt a command is late, so it runs after everything it might
// need.
func CostOf(cfg *config.Loaded, command, own string) Costed {
	switch {
	case own != "":
		return Costed{Cost(own), FromExplicit}
	case cfg.MatchesStatic(command):
		return Costed{Static, FromPattern}
	}
	return Costed{Late, FromDefault}
}

// CostedCheck is one of a task's checks with its cost, its task and its
// place in the task's done_when, from 0.
type CostedCheck struct {
	Costed
	Task  ledger.Task
	Check ledger.Check
	Index int
}

// CostedChecks are a task's checks with their costs. With
// keep_written_order, a check below a late one is late, whatever its own
// class: authors write a task's checks in the order they depend on (a check
// that reads a file runs after the one above it that writes the file).
func CostedChecks(cfg *config.Loaded, task ledger.Task) []CostedCheck {
	keep := cfg.CI.Cost.KeepWrittenOrder
	late := false
	costed := make([]CostedCheck, len(task.DoneWhen))
	for i, c := range task.DoneWhen {
		cost := CostOf(cfg, c.Command(), c.Cost)
		if keep && late && cost.Cost == Static {
			cost = Costed{Late, FromOrder}
		}
		if cost.Cost == Late {
			late = true
		}
		costed[i] = CostedCheck{cost, task, c, i}
	}
	return costed
}

// ChecksBeforeLate are a task's checks that CI runs before any late one: its
// checks in written order, up to its first late one. The commit-msg hook runs
// these, so a commit is held only by what takes seconds.
func ChecksBeforeLate(cfg *config.Loaded, task ledger.Task) []CostedCheck {
	costed := CostedChecks(cfg, task)
	for i, c := range costed {
		if c.Cost == Late {
			return costed[:i]
		}
	}
	return costed
}

// StaticChecks are a task's checks of the static class wherever they are
// written, a static one below a late one included (a check below a late one
// is late itself under keep_written_order): work done runs these, in written
// order, where the commit-msg hook runs ChecksBeforeLate.
func StaticChecks(cfg *config.Loaded, task ledger.Task) []CostedCheck {
	return ofCost(cfg, task, Static)
}

// LateChecks are a task's checks of the late class, which CI runs and work
// done names as left to it.
func LateChecks(cfg *config.Loaded, task ledger.Task) []CostedCheck {
	return ofCost(cfg, task, Late)
}

func ofCost(cfg *config.Loaded, task ledger.Task, cost Cost) []CostedCheck {
	var found []CostedCheck
	for _, c := range CostedChecks(cfg, task) {
		if c.Cost == cost {
			found = append(found, c)
		}
	}
	return found
}
