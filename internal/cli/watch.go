package cli

// The ci slice's ci watch, the push slice's end and the work slice's work
// done wait for a commit's CI run through internal/engine/ciwatch, whose doc
// says how, with the provider their Watcher ports bind to watcher.

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/providers"
)

// watcher is ci.watch's provider for the config at the folder itos runs in,
// for a commit pushed to remote: ok is false when the provider is none.
// With no config there is nothing to watch with, as with none.
func watcher(cfg *config.Loaded, remote string, o Out) (providers.Watcher, bool, error) {
	return ciwatch.Watcher(context.Background(), cfg, cfg.VCS, remote, providers.WatchSetup{
		Env:     os.Getenv,
		GhToken: ghToken(cfg.Folder()),
	})
}

// ghToken is providers.GhToken, gh run in the folder dir.
func ghToken(dir string) func() (string, error) {
	return func() (string, error) { return providers.GhToken(dir) }
}
