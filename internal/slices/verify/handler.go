package verify

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/engine/refs"
	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is the range verify judges: From and To as typed, from empty or all
// zeros for every commit up to To; or, Bare, no range typed, which a stealth
// config takes as the commits of HEAD on no remote branch.
type Query struct {
	From, To string
	Bare     bool
}

// Ports are what verify reaches the world through, which internal/cli
// binds: the repository, whose refs a typed range must name; Stealth,
// whether the config is the stealth one, asked only when no range is typed;
// and Config, the config, loaded from the working tree.
type Ports interface {
	vcsport.VCS
	Stealth(ctx context.Context) bool
	Config(ctx context.Context) (*config.Loaded, error)
}

// Handler is verify's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// span is a range as --json names it.
type span struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Answer is what Handle found: the range judged; commits.since's problems,
// when it or a footer's since is no commit here, and then nothing judged;
// else the judge's Run.
type Answer struct {
	Range    span
	Problems []out.Problem
	Run      judge.Run
}

// Handle judges q's range through internal/engine/judge (judge.Over over
// the config's VCS), its logs going where o says. With no range typed, a
// config that is not the stealth one is a usage error; under a stealth
// config the range is vcsport.Unpushed to HEAD (slice 34), since others'
// commits in a repository that does not use itos follow no rules of the
// person's. A typed range's refs must name commits, checked before the
// config is read.
func (h Handler) Handle(ctx context.Context, q Query, o judge.Out) (Answer, error) {
	from, to := q.From, q.To
	if q.Bare {
		if !h.ports.Stealth(ctx) {
			return Answer{}, needsRange()
		}
		from, to = vcsport.Unpushed, "HEAD"
	} else if err := refs.Check(ctx, h.ports, "verify", from, to); err != nil {
		return Answer{}, err
	}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	a := Answer{Range: span{from, to}, Problems: cfg.SinceIssues()}
	if len(a.Problems) > 0 {
		return a, nil
	}
	a.Run, err = judge.Over(ctx, cfg, cfg.VCS, from, to, o)
	return a, err
}
