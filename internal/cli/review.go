package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/slices/review"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runReview is `review`, the review slice (internal/slices/review) over its
// ports bound to the real code.
func runReview(args []string, o Out) (int, error) {
	c, err := review.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, review.NewHandler(reviewPorts{o: o, git: vcs.Git{Dir: o.Dir}}))
}

// reviewPorts are the review slice's ports over the real code: git, the
// config file's presence and load, gh and the files of the folder itos runs
// in.
type reviewPorts struct {
	o   Out
	git vcs.Git
}

// ConfigFound is whether configPath() is there.
func (p reviewPorts) ConfigFound(context.Context) bool {
	return exists(within(p.o.Dir, configPath(p.o.Dir)))
}

// LoadConfig is loadConfig.
func (p reviewPorts) LoadConfig(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// Resolve is git's.
func (p reviewPorts) Resolve(ctx context.Context, rev string) (string, error) {
	return p.git.Resolve(ctx, rev)
}

// Remote is the current branch's upstream's remote, else origin, as push
// names it for --tag and --ref.
func (p reviewPorts) Remote(ctx context.Context) string {
	if branch := p.git.Branch(ctx); branch != "" {
		remote, _, _ := p.git.Upstream(ctx, branch)
		return remote
	}
	return "origin"
}

// GitHubRepository is providers.Repository, the environment's and the
// remote's URL.
func (p reviewPorts) GitHubRepository(ctx context.Context, cfg *config.Loaded, remote string) string {
	url, _ := p.git.RemoteURL(ctx, remote)
	return providers.Repository(cfg, os.Getenv, url)
}

// Gh is providers.Gh in the folder itos runs in.
func (p reviewPorts) Gh(ctx context.Context, args ...string) (string, error) {
	return providers.Gh(ctx, p.o.Dir, args...)
}

// ReadFile reads the path from the folder itos runs in, an error naming the
// path as given.
func (p reviewPorts) ReadFile(_ context.Context, path string) ([]byte, error) {
	text, err := os.ReadFile(within(p.o.Dir, path))
	return text, named(err, path)
}
