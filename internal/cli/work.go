package cli

import (
	"context"
	"io"
	"os"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	nextslice "github.com/donvargax/itos/v8/internal/slices/next"
	workslice "github.com/donvargax/itos/v8/internal/slices/work"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
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
// given the id a draft reserved, judge.StaticFailure and watcher.
type workPorts struct{ o Out }

// Config is loadConfig.
func (p workPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// Worktree is the working tree's files.
func (p workPorts) Worktree(context.Context) treeport.Source { return trees.WorktreeIn(p.o.Dir) }

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

// TaskChecks is judge.StaticFailure in the environment the commit-msg hook
// runs a task's checks in (checkEnv), the task's opted-in checks looked up
// by HEAD's tree through newDoneEvidence; with none, no engine is made and
// each check runs in the working folder.
func (p workPorts) TaskChecks(_ context.Context, cfg *config.Loaded, task ledger.Task, stderr io.Writer) (workslice.Checked, error) {
	var execution check.Execution
	for _, c := range check.StaticChecks(cfg, task) {
		if c.Check.Evidenced() && !c.Check.Pushed() {
			execution.Evidence = newDoneEvidence(cfg, p.o.OS)
			break
		}
	}
	failure, err := judge.StaticFailure(cfg, task, checkEnv(), stderr, execution)
	checked := workslice.Checked{Failure: failure}
	if execution.Evidence != nil {
		checked.Evidence = execution.Evidence.Entries()
	}
	return checked, err
}

// Watcher is watcher.
func (workPorts) Watcher(_ context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error) {
	return watcher(cfg, remote, Out{})
}

// RemoteHead is the branch the remote's HEAD names, asked of the remote
// itself.
func (p workPorts) RemoteHead(ctx context.Context, remote string, talk vcsport.Talk) (string, error) {
	return vcs.Git{Dir: p.o.Dir}.RemoteHead(ctx, remote, talk)
}

// Gh is providers.Gh in the folder itos runs in.
func (p workPorts) Gh(ctx context.Context, args ...string) (string, error) {
	return providers.Gh(ctx, p.o.Dir, args...)
}

// GitHubRepository is providers.Repository, the environment's and the
// remote's URL.
func (workPorts) GitHubRepository(ctx context.Context, cfg *config.Loaded, remote string) string {
	url, _ := cfg.VCS.RemoteURL(ctx, remote)
	return providers.Repository(cfg, os.Getenv, url)
}

// Revision is the next slice's RevisionOf: the item's revision as itos next
// gives it, over the tracker the questions are in.
func (workPorts) Revision(ctx context.Context, cfg *config.Loaded, tracker trackerport.Tracker, id string) (string, *out.Problem, error) {
	return nextslice.RevisionOf(ctx, cfg, tracker, id)
}

// workAddLine is workslice.AddLine, which draft reads a work add line with.
func workAddLine(args []string) (work.New, error) { return workslice.AddLine(args) }

// workPromoteLine is workslice.PromoteLine, which draft reads a work
// promote line with.
func workPromoteLine(args []string) (workslice.Promotion, error) { return workslice.PromoteLine(args) }
