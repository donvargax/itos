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
// and Tracker, the tracker store the questions are in.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
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
// a registry that is not there or an id no item has.
type Answer struct {
	Next    *Next
	Refused *out.Problem
}

// Handle answers q over the config every command finds: the item's facts,
// read, and judged.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	facts, refused, err := read(ctx, cfg, h.ports.Tracker(ctx, cfg), q.ID)
	if err != nil || refused != nil {
		return Answer{Refused: refused}, err
	}
	n := Judge(facts)
	return Answer{Next: &n}, nil
}
