package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	nextslice "github.com/donvargax/itos/v8/internal/slices/next"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// runNext is `next <id>`, the next slice (internal/slices/next) over its
// ports bound to the real code.
func runNext(args []string, o Out) (int, error) {
	c, err := nextslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, nextslice.NewHandler(nextPorts{o}))
}

// nextPorts are the next slice's ports over the real code: loadConfig and
// the tracker on o's OS.
type nextPorts struct{ o Out }

// Config is loadConfig.
func (p nextPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// Tracker is tracker.
func (p nextPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}
