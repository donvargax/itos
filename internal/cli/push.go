package cli

import (
	"context"
	"path"
	"path/filepath"

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
	return c.Run(context.Background(), o.Stdout, o.Stderr, push.NewHandler(pushPorts{}))
}

// pushPorts are the push slice's ports over the real code: git, the config
// file's presence, the stealth check of its path, its load and ci.watch's
// provider.
type pushPorts struct{ vcs.Git }

// ConfigFound is whether configPath() is there.
func (pushPorts) ConfigFound(context.Context) bool { return exists(configPath()) }

// Stealth is config.IsStealth of configPath().
func (pushPorts) Stealth(context.Context) bool { return config.IsStealth(vcs.Git{}, configPath()) }

// LoadConfig is loadConfig.
func (pushPorts) LoadConfig(context.Context) (*config.Loaded, error) { return loadConfig() }

// Watcher is watcher.
func (pushPorts) Watcher(_ context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error) {
	return watcher(cfg, remote, Out{})
}

// touchesOnlyRegistry is whether the paths some commits touch, each against
// its first parent, are the work registry and no other path: the one rule by
// which itos push does not wait for a run (slice 56) and work done passes
// over a commit (slice 93), since such a commit changes nothing a run
// judges. A merge counts what it brought in against its first parent, and a
// commit that touches no path adds nothing to the answer, so commits that
// touch nothing at all are not registry-only; nor are commits whose paths
// cannot be read.
func touchesOnlyRegistry(registry string, files []string, err error) bool {
	if err != nil {
		return false
	}
	registry = path.Clean(filepath.ToSlash(registry))
	touched := false
	for _, f := range files {
		if path.Clean(f) != registry {
			return false
		}
		touched = true
	}
	return touched
}
