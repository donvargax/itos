package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	nextslice "github.com/donvargax/itos/v8/internal/slices/next"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
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

// nextPorts are the next slice's ports over the real code: loadConfig, the
// tracker on o's OS, and the forge as review post and attempt submit reach
// it: the repository push names, gh in the folder itos runs in, and the
// attempt journal in the git common dir.
type nextPorts struct{ o Out }

// Config is loadConfig.
func (p nextPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// Tracker is tracker.
func (p nextPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}

// GitHubRepository is review post's: the repository named by the
// environment, else by the URL of the current branch's upstream's remote,
// else origin's.
func (p nextPorts) GitHubRepository(ctx context.Context, cfg *config.Loaded) string {
	r := p.review()
	return r.GitHubRepository(ctx, cfg, r.Remote(ctx))
}

// Gh is providers.Gh in the folder itos runs in.
func (p nextPorts) Gh(ctx context.Context, args ...string) (string, error) {
	return p.review().Gh(ctx, args...)
}

// Journal is attempt submit's: the item's journal's bytes, none when it
// has none.
func (p nextPorts) Journal(ctx context.Context, id string) ([]byte, error) {
	return attemptPorts{o: p.o, git: vcs.Git{Dir: p.o.Dir}}.Journal(ctx, id)
}

func (p nextPorts) review() reviewPorts { return reviewPorts{o: p.o, git: vcs.Git{Dir: p.o.Dir}} }
