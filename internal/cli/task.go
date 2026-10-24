package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/ledger"
	taskslice "github.com/donvargax/itos/v8/internal/slices/task"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// runTask is `task`, the task slice (internal/slices/task) over its ports
// bound to the real code: task add writes through work's soundRegistry,
// newItemID, tracker and writeCommitted.
func runTask(args []string, o Out) (int, error) {
	c, err := taskslice.Parse(args, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, taskslice.NewHandler(taskPorts{o}))
}

// taskPorts are the task slice's ports over the real code, reporting as o
// says.
type taskPorts struct{ o Out }

// Config is loadConfig.
func (taskPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Evidence is newEvidence of a hand-run itos task.
func (p taskPorts) Evidence(_ context.Context, cfg *config.Loaded) *evidence.Engine {
	return newEvidence(cfg, "task", p.o.OS)
}

// Sound is soundRegistry.
func (p taskPorts) Sound(context.Context) (taskslice.Sound, error) {
	cfg, registry, text, release, code, err := soundRegistry(p.o)
	return taskslice.Sound{Cfg: cfg, Registry: registry, Text: text, Release: release, Code: code}, err
}

// ItemID is newItemID of a task.
func (p taskPorts) ItemID(_ context.Context, cfg *config.Loaded, registryIDs []string) (string, error) {
	return newItemID(cfg, "task", registryIDs, p.o.reserved, p.o.OS)
}

// Tracker is tracker.
func (p taskPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}

// Write is writeCommitted.
func (p taskPorts) Write(_ context.Context, cfg *config.Loaded, files []record.File, header, body string) (string, int, error) {
	list := make([]written, len(files))
	for i, f := range files {
		list[i] = written{path: f.Path, old: f.Old, text: f.Text, created: f.Created, rule: f.Rule}
	}
	return writeCommitted(cfg, list, header, body, p.o)
}

// taskAddArgs is taskslice.AddArgs, which draft reads a task add line with.
func taskAddArgs(cfg *config.Loaded, args []string) (ledger.NewTask, error) {
	return taskslice.AddArgs(cfg, args)
}

// groupNames is ledger.GroupNames.
var groupNames = ledger.GroupNames

// groupFlags is ledger.GroupFlags.
func groupFlags(cfg *config.Loaded) []string { return ledger.GroupFlags(cfg) }
