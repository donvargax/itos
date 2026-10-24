package cli

import (
	"context"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/shell"
	hookslice "github.com/donvargax/itos/v8/internal/slices/hook"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runHook is `hook`, the hook slice (internal/slices/hook) over its ports
// bound to the real code.
func runHook(args []string, o Out) (int, error) {
	c, err := hookslice.Parse(args, o.JSON, o.Origin, o.Tests)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, hookslice.NewHandler(hookPorts{Git: vcs.Git{Dir: o.Dir}, os: o.OS}))
}

// hookPorts are the hook slice's ports over the real code: git, the hooks
// folder's files, its trees, the config loaded from the working tree, the
// working tree's files, config check's task flags and the config's shell.
type hookPorts struct {
	vcs.Git
	hookFiles
	// os is the OS the evidence store's lock waits by, as Out.OS names it.
	os string
}

func (p hookPorts) CaptureIndex(ctx context.Context) (treeport.Source, error) {
	tree, err := p.Git.IndexTree(ctx)
	if err != nil {
		return nil, err
	}
	return p.Trees(ctx).At(ctx, tree)
}

func (p hookPorts) TaskSnapshot(ctx context.Context, tree string) (hookslice.Snapshot, error) {
	s, err := p.Git.TaskSnapshot(ctx, tree)
	return hookslice.Snapshot{Dir: s.Dir, TempDir: s.TempDir, Env: s.Env, Remove: s.Remove}, err
}

// CheckEnv is checkEnv.
func (hookPorts) CheckEnv(context.Context) []string { return checkEnv() }

// Evidence is newHookEvidence of the tree the hook captured.
func (p hookPorts) Evidence(_ context.Context, cfg *config.Loaded, tree string) *evidence.Engine {
	return newHookEvidence(cfg, tree, p.os)
}

// Trees are the trees git holds.
func (p hookPorts) Trees(context.Context) treeport.Trees { return trees.Trees{VCS: p.Git} }

// LoadConfig is loadConfig.
func (p hookPorts) LoadConfig(context.Context) (*config.Loaded, error) { return loadConfig(p.Dir) }

// Read is the working tree's file.
func (p hookPorts) Read(ctx context.Context, path string) (string, error) {
	return trees.WorktreeIn(p.Dir).Read(ctx, path)
}

// TaskFlags is ungroupedTaskFlags.
func (hookPorts) TaskFlags(context.Context) []string { return ungroupedTaskFlags() }

// Shell is shell.Run.
func (hookPorts) Shell(_ context.Context, cfg *config.Loaded, command string, o shell.Options) shell.Result {
	return shell.Run(cfg, command, o)
}

// checkEnv is the environment a task's check runs in under the hook and work
// done: the hook's, less the index git made the commit from, so that a
// check's own git commands (in a scratch repository too) never read or write
// the commit's index, as under `itos task`, and less the footers and the
// amend itos commit handed the hook, which are this commit's and no commit a
// check makes.
func checkEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_INDEX_FILE=") && !strings.HasPrefix(kv, message.FootersEnv+"=") &&
			!strings.HasPrefix(kv, AmendEnv+"=") {
			env = append(env, kv)
		}
	}
	return env
}
