package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/providers"
	workslice "github.com/donvargax/itos/v8/internal/slices/work"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// runWork is `work`, the work slice (internal/slices/work) over its ports
// bound to the real code, which reads every subcommand's line and answers
// all but work promote and work done. Those two are still internal/cli's
// (unmoved), each called with the arguments after its name once the slice
// has read them.
func runWork(args []string, o Out) (int, error) {
	c, err := workslice.Parse(args, o.Origin, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	if write := unmoved(c.Sub); write != nil {
		_, rest := split(args)
		return write(rest, o)
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, workslice.NewHandler(workPorts{o}))
}

// unmoved is the subcommand of work not yet in the slice that sub names,
// nil for one the slice answers.
func unmoved(sub string) command {
	switch sub {
	case "promote":
		return workPromote
	case "done":
		return workDone
	}
	return nil
}

// workPorts are the work slice's ports over the real code: loadConfig, the
// working tree, work.identity's provider, the tracker on o's OS and
// newItemID, given the id a draft reserved.
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

// workAddLine is workslice.AddLine, which draft reads a work add line with.
func workAddLine(args []string) (work.New, error) { return workslice.AddLine(args) }
