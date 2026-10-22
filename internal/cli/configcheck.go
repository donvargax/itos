package cli

import (
	"context"
	"maps"
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/configcheck"
	"github.com/donvargax/itos/v8/internal/out"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// Found is configcheck.Found.
type Found = configcheck.Found

// tagged is configcheck.Tagged.
func tagged(area string, problems []out.Problem) []Found {
	return configcheck.Tagged(area, problems)
}

// judgedSmokeFindings is configcheck.JudgedSmokeFindings.
func judgedSmokeFindings(cfg *config.Loaded, judged func(name, file string) bool, ws *tests.Warnings) ([]Found, []string) {
	return configcheck.JudgedSmokeFindings(cfg, judged, ws)
}

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
