package cli

import (
	"context"
	"io"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/providers"
	workslice "github.com/donvargax/itos/v8/internal/slices/work"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// runWork is `work`, the work slice (internal/slices/work) over its ports
// bound to the real code.
func runWork(args []string, o Out) (int, error) {
	c, err := workslice.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, workslice.NewHandler(workPorts{o}))
}

// workPorts are the work slice's ports over the real code: loadConfig, the
// working tree, work.identity's provider, the tracker on o's OS, newItemID,
// given the id a draft reserved, judge.FirstFailure and watcher.
type workPorts struct{ o Out }

// Config is loadConfig.
func (workPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Worktree is the working tree's files.
func (workPorts) Worktree(context.Context) treeport.Source { return trees.Worktree }

// Identity is work.identity's provider.
func (workPorts) Identity(_ context.Context, cfg *config.Loaded) providers.Identity {
	return providers.IdentityProvider(cfg)
}

// Tracker is tracker.
func (p workPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}

// ItemID is newItemID.
func (p workPorts) ItemID(_ context.Context, cfg *config.Loaded, kind string, registryIDs []string) (string, error) {
	return newItemID(cfg, kind, registryIDs, p.o.reserved, p.o.OS)
}

// FirstFailure is judge.FirstFailure in the environment the commit-msg
// hook runs a task's checks in (checkEnv).
func (workPorts) FirstFailure(_ context.Context, cfg *config.Loaded, task ledger.Task, limit float64, stderr io.Writer) (string, error) {
	return judge.FirstFailure(cfg, task, limit, checkEnv(), stderr)
}

// Watcher is watcher.
func (workPorts) Watcher(_ context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error) {
	return watcher(cfg, remote, Out{})
}

// workAddLine is workslice.AddLine, which draft reads a work add line with.
func workAddLine(args []string) (work.New, error) { return workslice.AddLine(args) }

// workPromoteLine is workslice.PromoteLine, which draft reads a work
// promote line with.
func workPromoteLine(args []string) (workslice.Promotion, error) { return workslice.PromoteLine(args) }
