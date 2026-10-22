package cli

import (
	"context"
	"maps"
	"slices"

	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// Found is configcheck.Found.
type Found = configcheck.Found

// configFindings is configcheck.Findings over git.
func configFindings(src port.Source, ledgerFile string, ws *tests.Warnings) (int, []Found, []Found, []string, error) {
	return configcheck.Findings(context.Background(), vcs.Git{}, trees.Trees{VCS: vcs.Git{}}, src, ledgerFile, ungroupedTaskFlags(), ws)
}

// ungroupedTaskFlags are the taskFlags that are none of groupNames: the
// flags ledger.group.label may not name.
func ungroupedTaskFlags() []string {
	return slices.DeleteFunc(taskFlags(), func(f string) bool { return slices.Contains(groupNames, f) })
}

// taskFlags are the flags itos task and its subcommands take, the global
// ones with them, by name.
func taskFlags() []string {
	names := []string{"--config", "--root"}
	for name := range switches {
		names = append(names, name)
	}
	task := specs["task"]
	for _, s := range append([]spec{task}, slices.Collect(maps.Values(task.Subs))...) {
		for _, f := range s.Flags {
			names = append(names, f.Name)
		}
	}
	return names
}
