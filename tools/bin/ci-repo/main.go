// Command ci-repo applies this repository's metadata shortcut before asking the ordinary itos
// planner and driver to plan or run a range. The shipped CLI's policy stays unchanged. Only a
// readable, nonempty range of prose, the work registry and ledger files, with no task or test
// requirements, can borrow ci.prose.steps. The full plan runs a task's checks only when a commit
// names the task, so a ledger edit in a range naming none is judged by what the prose steps run,
// itos config check. The original config still validates itself in those steps.
//
// Run from the repository root: go run ./tools/bin/ci-repo scope|plan|run|tests FROM TO.
// Verification remains the workflow's preceding `tools/bin/itos verify FROM TO` command.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/donvargax/itos/v8/internal/ci"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

type scope struct {
	Full   bool   `json:"full"`
	Reason string `json:"reason"`
	// ledger are the changed ledger files an eligible range adds to ci.prose.paths.
	ledger []string
}

func classify(cfg *config.Loaded, from, to string) (scope, error) {
	full := scope{true, "code, requirements or unknown range", nil}
	if !plan.Readable(cfg.VCS, from, to) || !cfg.VCS.IsAncestor(context.Background(), from, to) {
		return full, nil
	}
	paths := plan.Changed(cfg.VCS, from, to)
	if len(paths) == 0 || cfg.CI.Prose == nil || len(cfg.CI.Prose.Steps) == 0 {
		return full, nil
	}
	if len(plan.TasksIn(cfg, from, to)) != 0 {
		return full, nil
	}
	for _, kind := range cfg.Tests.Keys {
		if len(plan.TestsNamedIn(cfg, from, to, kind)) != 0 {
			return full, nil
		}
	}
	layout, err := ledger.LayoutOf(cfg)
	if err != nil {
		return full, nil
	}
	registry := false
	var prose, files []string
	for _, p := range paths {
		switch {
		case p == "tasks/work-items.yaml":
			registry = true
		case path.Dir(p) == layout.Dir && layout.File.MatchString(path.Base(p)):
			files = append(files, p)
		default:
			prose = append(prose, p)
		}
	}
	if len(prose) != 0 {
		ok, err := plan.DocsOnly(cfg, prose)
		if err != nil || !ok {
			return full, err
		}
	}
	if len(files) != 0 {
		return scope{false, "registry, ledger and prose without requirements", files}, nil
	}
	if registry {
		return scope{false, "registry and prose without requirements", nil}, nil
	}
	return scope{false, "prose without requirements", nil}, nil
}

func run(args []string) (int, error) {
	if len(args) != 3 || (args[0] != "scope" && args[0] != "plan" && args[0] != "run" && args[0] != "tests") {
		return 2, fmt.Errorf("usage: ci-repo scope|plan|run|tests FROM TO")
	}
	// git runs in the folder the program stands in, named: itos's adapters
	// read every relative path from the folder their git runs in.
	here, _ := os.Getwd()
	git := vcs.Git{Dir: here}
	cfg, err := config.Load(git, adapter.Trees{VCS: git}, adapter.Worktree, config.Path(git, adapter.Worktree))
	if err != nil {
		return 2, err
	}
	from, to := args[1], args[2]
	s, err := classify(cfg, from, to)
	if err != nil {
		return 2, err
	}
	if args[0] == "scope" {
		return 0, json.NewEncoder(os.Stdout).Encode(s)
	}
	if s.Full {
		// Even prose naming a late task or scenario keeps its promised checks.
		cfg.CI.Prose = nil
	} else {
		cfg.CI.Prose.Paths = append(append(cfg.CI.Prose.Paths, "tasks/work-items.yaml"), s.ledger...)
	}
	d, err := plan.DataAt(cfg, "")
	if err != nil {
		return 2, err
	}
	p, err := plan.For(cfg, from, to, d)
	if err != nil {
		return 2, err
	}
	if args[0] == "plan" {
		return 0, out.Emit(os.Stdout, p.Fields(plan.RangeOf(cfg.VCS, from, to, false))...)
	}
	if args[0] == "tests" {
		p = p.NamedTests()
	}
	// The existing release/plugin checks read FROM, including in local reproductions.
	if err := os.Setenv("FROM", from); err != nil {
		return 2, err
	}
	return ci.Run(cfg, p, ci.Options{Stdout: os.Stdout, Stderr: os.Stderr})
}

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ci-repo:", err)
		if code == 0 {
			code = 2
		}
	}
	os.Exit(code)
}
