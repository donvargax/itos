package ci

import (
	"context"
	"fmt"
	"io"

	cirun "github.com/donvargax/itos/v8/internal/ci"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/prtrust"
	"github.com/donvargax/itos/v8/internal/engine/refs"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// missing is the exit code of a missing environment (itos --help): ci
// watch outside a git repository, or a provider that cannot look.
const missing = 3

// Query is what the ci command asks: Sub, plan, run, scope, watch or range;
// a range From and To as typed, Bare when plan was given none, neither
// --nightly nor --whole; the Nightly's and the Whole range's switches;
// plan's DataAt, "" without --data-at; run's Tests; watch's Rev, HEAD when
// none was typed; range's Head and Base.
type Query struct {
	Sub                         string
	From, To                    string
	Bare, Nightly, Whole, Tests bool
	DataAt                      string
	Rev                         string
	Head, Base                  string
}

// Ports are what the ci command reaches the world through, which
// internal/cli binds: the repository, whose refs a typed range must name
// and whose commit a watch waits for; Stealth, whether the config is the
// stealth one, asked only when plan is given no range; Config, the config,
// loaded from the working tree; Env, the environment, which ci.range's
// provider reads; RunPlan, a plan carried out (internal/ci); and Watcher,
// ci.watch's provider for a commit pushed to remote, ok false when it is
// none.
type Ports interface {
	vcsport.VCS
	Stealth(ctx context.Context) bool
	Config(ctx context.Context) (*config.Loaded, error)
	Env(ctx context.Context, name string) string
	RunPlan(ctx context.Context, cfg *config.Loaded, p *plan.Plan, o cirun.Options) (int, error)
	Watcher(ctx context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error)
}

// Handler is the ci command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is where ci run and ci watch write while they work: their logs and
// progress to Stdout, or Stderr under JSON, a watch's nowhere under Quiet.
type Out struct {
	Stdout, Stderr io.Writer
	JSON, Quiet    bool
}

// Answer is what Handle found or did: plan's Plan and its Range; scope's
// DocsOnly; range's From; run's Code, the run's exit; watch's Watched, how
// the watch of the commit SHA ended, nil with Code missing outside a git
// repository.
type Answer struct {
	Plan     *plan.Plan
	Range    plan.Range
	DocsOnly bool
	From     string
	Code     int
	SHA      string
	Watched  *ciwatch.Watched
}

// Handle answers q, ci run and ci watch writing where o says as they work.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch q.Sub {
	case "plan":
		return h.plan(ctx, q)
	case "run":
		return h.run(ctx, q, o)
	case "scope":
		return h.scope(ctx, q)
	case "watch":
		return h.watch(ctx, q, o)
	}
	return h.start(ctx, q)
}

// planOf is the config and the plan a run of it carries out: the nightly's,
// or a pushed range's with its data from the working tree or, with dataAt,
// from that commit. A range that is a merged pull request's commits, whose
// head's CI run passed, is planned as a prose-only range (internal/engine/prtrust,
// slice 188), and a merged pull request's head not trusted says why in the plan.
func (h Handler) planOf(ctx context.Context, from, to string, nightly bool, dataAt string) (*config.Loaded, *plan.Plan, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return nil, nil, err
	}
	var p *plan.Plan
	if nightly {
		p = plan.ForNightly(cfg)
	} else {
		var d plan.Data
		if d, err = plan.DataAt(cfg, dataAt); err == nil {
			p, err = h.planRange(ctx, cfg, from, to, d)
		}
	}
	return cfg, p, err
}

// planRange is the plan of a pushed range, trusting a merged pull request's
// green head for it when GitHub lists one that merged the range's end and the
// rule of prtrust.Judge holds.
func (h Handler) planRange(ctx context.Context, cfg *config.Loaded, from, to string, d plan.Data) (*plan.Plan, error) {
	env := func(name string) string { return h.ports.Env(ctx, name) }
	var verdict prtrust.Result
	if look, ok := providers.MergedPullsProvider(cfg, env); ok {
		verdict = prtrust.Judge(ctx, cfg, look, from, to)
	}
	if verdict.Trust != nil {
		return plan.ForTrusted(cfg, from, to, d, verdict.Trust)
	}
	p, err := plan.For(cfg, from, to, d)
	if p != nil {
		p.Note = verdict.Note
	}
	return p, err
}

// plan is `ci plan <from> <to> | --nightly | --whole [--data-at <sha>]`
// (ci.ts's ciPlanCommand): the plan the run would carry out, and its range
// as the JSON gives it; it runs nothing. --whole is the range with neither
// end, which cannot be read, so every test runs. --data-at reads the
// ledger, the registry and the smoke set at that commit. Under a stealth
// config `ci plan` with no range plans the commits of HEAD on no remote
// branch (from vcsport.Unpushed, slice 34); with no range, any other config
// is a usage error. The range's refs and --data-at must name commits,
// checked before the config is read.
func (h Handler) plan(ctx context.Context, q Query) (Answer, error) {
	from, to := q.From, q.To
	if q.Bare {
		if !h.ports.Stealth(ctx) {
			return Answer{}, needsRange()
		}
		from, to = vcsport.Unpushed, "HEAD"
	}
	if err := refs.CheckRange(ctx, h.ports, "ci plan", from, to, q.DataAt); err != nil {
		return Answer{}, err
	}
	cfg, p, err := h.planOf(ctx, from, to, q.Nightly, q.DataAt)
	if err != nil {
		return Answer{}, err
	}
	return Answer{Plan: p, Range: plan.RangeOf(cfg.VCS, from, to, q.Nightly)}, nil
}

// run is `ci run [<from> <to>] | --nightly [--tests]` (ci.ts's ciRun): the
// plan ci plan prints, carried out through the RunPlan port. A plan that
// cannot be made (a missing ledger folder, say) stops the run before any
// step, exit 2. With Tests it carries out the plan's run of named tests
// alone (plan.NamedTests, slice 132), what a platform job runs per push;
// with no such run it says so and passes, having run nothing.
func (h Handler) run(ctx context.Context, q Query, o Out) (Answer, error) {
	if err := refs.CheckRange(ctx, h.ports, "ci run", q.From, q.To); err != nil {
		return Answer{}, err
	}
	cfg, p, err := h.planOf(ctx, q.From, q.To, q.Nightly, "")
	if err != nil {
		return Answer{}, err
	}
	if q.Tests {
		if p = p.NamedTests(); len(p.Order) == 0 {
			log := o.Stdout
			if o.JSON {
				log = o.Stderr
			}
			fmt.Fprintln(log, "ci run --tests: no named tests to run (no tests: step, or nothing selected), so nothing ran.")
		}
	}
	code, err := h.ports.RunPlan(ctx, cfg, p, cirun.Options{
		JSON: o.JSON, Nightly: q.Nightly, Stdout: o.Stdout, Stderr: o.Stderr,
		Annotate: h.ports.Env(ctx, "GITHUB_ACTIONS") == "true",
	})
	return Answer{Code: code}, err
}

// scope is `ci scope <from> <to>` (ci.ts's ciScope): whether the range is
// prose only, for the workflow, its refs checked before the config is read.
func (h Handler) scope(ctx context.Context, q Query) (Answer, error) {
	if err := refs.CheckRange(ctx, h.ports, "ci scope", q.From, q.To); err != nil {
		return Answer{}, err
	}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	docs, err := plan.DocsOnly(cfg, plan.Changed(cfg.VCS, q.From, q.To))
	return Answer{DocsOnly: docs}, err
}

// start is `ci range --head <sha> [--base <sha>]` (ci.ts's ciRange): where
// the range starts, commits.since's commit when the provider gives no start
// (slice 109), empty to run everything. The config is read first, so a
// broken ci.range is a config error even for a pull request, whose base is
// kept without asking the provider; a provider that fails is not an error.
func (h Handler) start(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	env := func(name string) string { return h.ports.Env(ctx, name) }
	return Answer{From: providers.RangeStart(cfg.VCS, q.Head, q.Base, providers.RangeProvider(cfg, env), cfg.Since())}, nil
}

// watch is `ci watch [<sha>]`: the run of the commit, HEAD's by default,
// waited for as itos push waits for its own (internal/engine/ciwatch), the
// remote the current branch's upstream, else origin. Outside a git
// repository it says so, exit 3; a rev that names no commit is a usage
// error; a provider that cannot look is said on stderr and ends the watch
// as an error, exit 3; provider none is a config error.
func (h Handler) watch(ctx context.Context, q Query, o Out) (Answer, error) {
	if _, err := h.ports.Locate(ctx, ""); err != nil {
		fmt.Fprintln(o.Stderr, "itos: ci watch runs in a git repository, and this is none")
		return Answer{Code: missing}, nil
	}
	sha, err := h.ports.Resolve(ctx, q.Rev)
	if err != nil {
		return Answer{}, out.Usage("ci watch: %s is not a commit of this repository", q.Rev)
	}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	remote := "origin"
	if branch := h.ports.Branch(ctx); branch != "" {
		remote, _, _ = h.ports.Upstream(ctx, branch)
	}
	wr, ok, err := h.ports.Watcher(ctx, cfg, remote)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
		return Answer{SHA: sha, Watched: &ciwatch.Watched{Code: missing, Outcome: "error"}}, nil
	}
	if !ok {
		return Answer{}, config.Invalid(cfg.Path, "ci.watch.provider is none, so there is no CI run to watch: set ci.watch.provider to github")
	}
	w := ciwatch.Watch(ctx, cfg, wr, h.ports, sha, remote, "", ciwatch.Out(o))
	return Answer{SHA: sha, Watched: &w}, nil
}
