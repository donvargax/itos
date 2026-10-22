package cli

import (
	"context"

	configslice "github.com/donvargax/itos/v8/internal/slices/config"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runConfig is `config`, the config slice (internal/slices/config) over its
// ports bound to the real code; check's --ledger is read from where itos
// was started, before a run from a subfolder moved to the top.
func runConfig(args []string, o Out) (int, error) {
	c, err := configslice.Parse(args, o.Origin, o.JSON, o.Quiet, o.Tests)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, configslice.NewHandler(configPorts{}))
}

// configPorts are the config slice's ports over the real code: git, the
// trees over it, the working tree, and the flags of itos task that name no
// group.
type configPorts struct{ vcs.Git }

// Trees is the tree adapter over git.
func (configPorts) Trees(context.Context) port.Trees { return trees.Trees{VCS: vcs.Git{}} }

// Worktree is the working tree's files.
func (configPorts) Worktree(context.Context) port.Source { return trees.Worktree }

// TaskFlags is ungroupedTaskFlags.
func (configPorts) TaskFlags(context.Context) []string { return ungroupedTaskFlags() }
