package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	statusslice "github.com/donvargax/itos/v8/internal/slices/status"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runStatus is `status [--as <handle>]`, the status slice
// (internal/slices/status) over its ports bound to the real code.
func runStatus(args []string, o Out) (int, error) {
	c, err := statusslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, statusslice.NewHandler(statusPorts{o: o}))
}

// statusPorts are the status slice's ports over the real code: git,
// loadConfig, work.identity's provider, os.Getenv and gh for the providers,
// and the tracker on o's OS.
type statusPorts struct {
	vcs.Git
	o Out
}

// Config is loadConfig.
func (statusPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Identity is work.identity's provider.
func (statusPorts) Identity(_ context.Context, cfg *config.Loaded) providers.Identity {
	return providers.IdentityProvider(cfg)
}

// Setup is the environment and gh's token.
func (statusPorts) Setup(context.Context) providers.WatchSetup {
	return providers.WatchSetup{Env: os.Getenv, GhToken: providers.GhToken}
}

// Tracker is tracker.
func (p statusPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}
