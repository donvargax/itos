package status

import (
	"context"
	"io"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	statusengine "github.com/donvargax/itos/v8/internal/engine/status"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what the status command asks: As, the person --as names, "" when
// not given.
type Query struct {
	As string
}

// Ports are what the status command reaches the world through, which
// internal/cli binds: the repository, where the newest release, the rules
// for agents and the drafts' file are read (the head is read through the
// config's VCS); Config, the config every command finds; Identity,
// work.identity's provider, asked who the session works for; Setup, what
// ci.watch's and ci.range's providers read; Gh, gh within providers.GhTimeout
// (providers.ErrGhNoAnswer past it), which the pull requests of the items in
// progress are asked through; and Tracker, the tracker store the questions
// are in.
type Ports interface {
	vcsport.VCS
	Config(ctx context.Context) (*config.Loaded, error)
	Identity(ctx context.Context, cfg *config.Loaded) providers.Identity
	Setup(ctx context.Context) providers.WatchSetup
	Gh(ctx context.Context, args ...string) (string, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
}

// Handler is the status command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how its answer is printed: Stderr, where it
// is said when the person is not among the people.
type Out struct {
	Stderr io.Writer
}

// Answer is what Handle found: Standing, where things stand; else the
// Problems of a registry that is not there or not sound, or Missing, --as
// not among the people.
type Answer struct {
	Standing *statusengine.Standing
	Problems []out.Problem
	Missing  bool
}

// Handle answers q, over the config every command finds.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	st, found, missing, err := h.Standing(ctx, cfg, q.As, o)
	return Answer{Standing: st, Problems: found, Missing: missing}, err
}

// Standing is internal/engine/status's Of over the ports: where things stand
// for the person as names ("" when not given), the questions read through
// the config's tracker and the drafts from the git common dir, none outside
// a repository. A registry that is not there or not sound is its problems;
// missing is true when as is not among the people, said on o.Stderr. itos
// go prints it after its guide.
func (h Handler) Standing(ctx context.Context, cfg *config.Loaded, as string, o Out) (st *statusengine.Standing, found []out.Problem, missing bool, err error) {
	return statusengine.Of(ctx, cfg, as, statusengine.In{
		Identity: h.ports.Identity(ctx, cfg),
		Stderr:   o.Stderr,
		Git:      h.ports,
		Setup:    h.ports.Setup(ctx),
		Gh:       h.ports.Gh,
		Questions: func() (ask.File, error) {
			asks, _, _, err := ask.Load(ctx, h.ports.Tracker(ctx, cfg), cfg.Work.Asks)
			return asks, err
		},
		Drafts: func() (draft.File, bool, error) {
			file := draft.Path(ctx, h.ports)
			if file == "" {
				return draft.File{}, false, nil
			}
			drafts, err := draft.Load(file)
			return drafts, true, err
		},
	})
}
