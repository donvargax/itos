package cli

import (
	"fmt"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/out"
	"github.com/donvargax/itos/internal/plan"
)

// ciPlan is `ci plan <from> <to> | --nightly | --whole [--data-at <sha>]`
// (ci.ts's ciPlanCommand): the plan the run would carry out, one item a
// line, or under --json the plan as JSON (internal/plan); it runs nothing.
// --whole is the range with neither end, which cannot be read, so every
// test runs. --data-at reads the ledger, the registry and the smoke set at
// that commit.
func ciPlan(from, to string, nightly bool, dataAt string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	var p *plan.Plan
	if nightly {
		p, err = plan.ForNightly(cfg)
	} else {
		var d plan.Data
		if d, err = plan.DataAt(cfg, dataAt); err == nil {
			p, err = plan.For(cfg, from, to, d)
		}
	}
	if err != nil {
		return 0, err
	}
	r := plan.RangeOf(from, to, nightly)
	if o.JSON {
		return 0, out.Emit(o.Stdout, p.Fields(r)...)
	}
	p.Print(o.Stdout)
	return 0, nil
}

// ciScope is `ci scope <from> <to>` (ci.ts's ciScope): whether the range is
// prose only, for the workflow.
func ciScope(from, to string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	docs, err := plan.DocsOnly(cfg, plan.Changed(from, to))
	if err != nil {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "docs_only", Value: docs})
	}
	fmt.Fprintf(o.Stdout, "docs_only=%t\n", docs)
	return 0, nil
}
