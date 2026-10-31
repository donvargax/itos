package next

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// Ports are what the next command reaches the world through, which
// internal/cli binds: Config, the config every command finds, whose source,
// trees and VCS the registry, the ledger and the scenarios are read through;
// Tracker, the tracker store the questions are in; and the Forge, asked
// under push.mode pr for an item in flight.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
	Forge
}

// Handler is the next command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Answer is what Handle found: the item's Next, or the Refused problem of
// a registry that is not there or an id no item has, or the Failed one of a
// forge that cannot be asked.
type Answer struct {
	Next    *Next
	Refused *out.Problem
	Failed  *out.Problem
}

// Handle answers q over the config every command finds: the item's facts,
// read, and judged.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	facts, refused, failed, err := read(ctx, cfg, h.ports.Tracker(ctx, cfg), h.ports, q.ID)
	if err != nil || refused != nil || failed != nil {
		return Answer{Refused: refused, Failed: failed}, err
	}
	n := Judge(facts)
	return Answer{Next: &n}, nil
}
