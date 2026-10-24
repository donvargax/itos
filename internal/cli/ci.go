package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/ci"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/providers"
	cislice "github.com/donvargax/itos/v8/internal/slices/ci"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runCi is `ci`, the ci slice (internal/slices/ci) over its ports bound to
// the real code.
func runCi(args []string, o Out) (int, error) {
	c, err := cislice.Parse(args, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, cislice.NewHandler(ciPorts{goos: o.OS}))
}

// ciPorts are the ci slice's ports over the real code: git, the stealth
// check of the config's path, the config's load, the process's
// environment, internal/ci's run and ci.watch's provider as push's end
// asks it.
type ciPorts struct {
	vcs.Git
	goos string
}

// Stealth is config.IsStealth of configPath().
func (ciPorts) Stealth(context.Context) bool { return config.IsStealth(vcs.Git{}, configPath()) }

// Config is loadConfig.
func (ciPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Env is os.Getenv.
func (ciPorts) Env(_ context.Context, name string) string { return os.Getenv(name) }

// RunPlan is ci.Run, its checks and steps that opt in to evidence records looked up through newEvidence.
func (c ciPorts) RunPlan(_ context.Context, cfg *config.Loaded, p *plan.Plan, o ci.Options) (int, error) {
	o.Evidence = newEvidence(cfg, "ci", c.goos)
	return ci.Run(cfg, p, o)
}

// Watcher is watcher.
func (ciPorts) Watcher(_ context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error) {
	return watcher(cfg, remote, Out{})
}
