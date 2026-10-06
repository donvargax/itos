package cli

import (
	"fmt"
	"os"

	"github.com/donvargax/itos/v5/internal/ci"
	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/plan"
	"github.com/donvargax/itos/v5/internal/providers"
)

// planOf is the config and the plan a run of it carries out: the nightly's,
// or a pushed range's with its data from the working tree or, with dataAt,
// from that commit.
func planOf(from, to string, nightly bool, dataAt string) (*config.Loaded, *plan.Plan, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return nil, nil, err
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
	return cfg, p, err
}

// ciRun is `ci run [<from> <to>] | --nightly` (ci.ts's ciRun): the plan
// ci plan prints, carried out by internal/ci. A plan that cannot be made
// (a missing ledger folder, say) stops the run before any step, exit 2.
func ciRun(from, to string, nightly bool, o Out) (int, error) {
	cfg, p, err := planOf(from, to, nightly, "")
	if err != nil {
		return 0, err
	}
	return ci.Run(cfg, p, ci.Options{JSON: o.JSON, Nightly: nightly, Stdout: o.Stdout, Stderr: o.Stderr})
}

// ciPlan is `ci plan <from> <to> | --nightly | --whole [--data-at <sha>]`
// (ci.ts's ciPlanCommand): the plan the run would carry out, one item a
// line, or under --json the plan as JSON (internal/plan); it runs nothing.
// --whole is the range with neither end, which cannot be read, so every
// test runs. --data-at reads the ledger, the registry and the smoke set at
// that commit. Under a stealth config `ci plan` with no range plans the
// commits of HEAD on no remote branch (from git.Unpushed, slice 34).
func ciPlan(from, to string, nightly bool, dataAt string, o Out) (int, error) {
	_, p, err := planOf(from, to, nightly, dataAt)
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

// ciRange is `ci range --head <sha> [--base <sha>]` (ci.ts's ciRange): where
// the range starts, `FROM=<sha>`, empty to run everything. The config is
// read first, so a broken ci.range is a config error even for a pull
// request, whose base is kept without asking the provider; a provider that
// fails is not an error.
func ciRange(head, base string, o Out) (int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	from := providers.RangeStart(head, base, providers.RangeProvider(cfg, os.Getenv))
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "from", Value: from})
	}
	fmt.Fprintf(o.Stdout, "FROM=%s\n", from)
	return 0, nil
}
