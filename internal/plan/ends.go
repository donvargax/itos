package plan

import (
	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/tests"
)

// The range a run is for, as its steps and checks are given it (slice 83,
// issue #12): a step that depends on the range reads it from itos, so that
// `itos ci run <from> <to>` on a machine runs what CI runs, rather than from
// variables the workflow sets. The driver sets ITOS_FROM and ITOS_TO in its
// environment, which every step and check it runs inherits, and the plan
// fills {from} and {to} into each step's command with the range checks'
// filler (tests.FillRange), so `ci plan` prints the command the run runs.
// The cost class and ci.covers read a step as ci.steps writes it.

// Ends are a range's ends as a run gives them: full SHAs where git resolves
// them, From empty for a range that runs everything (an empty or all-zeros
// start with no commits.since to start after).
type Ends struct {
	From, To string
}

// EndsOf are the ends of the range from..to: From is the start range checks
// are given (tests.RangeFrom: the unpushed commits' base, commits.since),
// To the end; the nightly's are those of every commit up to HEAD.
func EndsOf(cfg *config.Loaded, from, to string) Ends {
	start := tests.RangeFrom(cfg, from, to)
	if config.NewBranch(start) {
		start = ""
	}
	return Ends{fullSHA(start), fullSHA(to)}
}

// fill fills the ends into the plan's steps, and into the step a covered
// check names, so that what `ci plan` prints is what `ci run` runs.
func (p *Plan) fill(e Ends) {
	p.Ends = e
	in := func(command string) string { return tests.FillRange(command, e.From, e.To) }
	for i := range p.Steps {
		p.Steps[i] = in(p.Steps[i])
	}
	for _, item := range p.Order {
		if item.Step != nil {
			item.Step.Command = in(item.Step.Command)
		}
	}
	for _, c := range p.Checks {
		if c.CoveredBy != "" {
			c.CoveredBy = in(c.CoveredBy)
		}
	}
}
