package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/slices/push"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runPush is `push`, the push slice (internal/slices/push) over its ports
// bound to the real code.
func runPush(args []string, o Out) (int, error) {
	c, err := push.Parse(args, o.JSON, o.Quiet, o.Origin)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, push.NewHandler(pushPorts{Git: vcs.Git{Dir: o.Dir}}))
}

// pushPorts are the push slice's ports over the real code: git, the hooks
// folder's files, the config file's presence, the stealth check of its
// path, its load and ci.watch's provider.
type pushPorts struct {
	vcs.Git
	hookFiles
}

// ConfigFound is whether configPath() is there.
func (p pushPorts) ConfigFound(context.Context) bool { return exists(within(p.Dir, configPath(p.Dir))) }

// Stealth is config.IsStealth of configPath().
func (p pushPorts) Stealth(context.Context) bool { return config.IsStealth(p.Git, configPath(p.Dir)) }

// LoadConfig is loadConfig.
func (p pushPorts) LoadConfig(context.Context) (*config.Loaded, error) { return loadConfig(p.Dir) }

// Watcher is watcher.
func (pushPorts) Watcher(_ context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error) {
	return watcher(cfg, remote, Out{})
}

// Gh is providers.Gh in the folder itos runs in.
func (p pushPorts) Gh(ctx context.Context, args ...string) (string, error) {
	return providers.Gh(ctx, p.Dir, args...)
}

// GitHubRepository is providers.Repository, the environment's and the
// remote's URL.
func (p pushPorts) GitHubRepository(ctx context.Context, cfg *config.Loaded, remote string) string {
	url, _ := p.RemoteURL(ctx, remote)
	return providers.Repository(cfg, os.Getenv, url)
}
