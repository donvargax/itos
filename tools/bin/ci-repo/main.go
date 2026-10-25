// Command ci-repo applies this repository's metadata shortcut before asking the ordinary itos
// planner and driver to plan or run a range. The shipped CLI's policy stays unchanged. Only a
// readable, nonempty range of prose, the work registry and ledger files that only add or remove
// whole task entries, with no task or test requirements, can borrow ci.prose.steps. The original
// config still validates itself in those steps.
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
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/value"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

type scope struct {
	Full   bool   `json:"full"`
	Reason string `json:"reason"`
	// ledger are the changed ledger files an eligible range adds to ci.prose.paths.
	ledger []string
}

// entries are a ledger file's tasks by ID, each as its parsed JSON, so a task compares by
// what it says, never by its comment or indentation. A file that is not a list of mappings
// with one distinct ID each is not read.
func entries(src port.Source, file string) (map[string]string, bool) {
	text, err := src.Read(context.Background(), file)
	if err != nil {
		return nil, false
	}
	raw, err := value.Parse(text)
	if err != nil {
		return nil, false
	}
	list, ok := raw.([]any)
	if raw != nil && !ok {
		return nil, false
	}
	byID := map[string]string{}
	for _, item := range list {
		id, named := value.Prop(item, "id").(string)
		if _, mapping := item.(*value.Map); !mapping || !named || id == "" {
			return nil, false
		}
		if _, twice := byID[id]; twice {
			return nil, false
		}
		byID[id] = value.JSON(item)
	}
	return byID, true
}

// wholeEntriesOnly is whether the ledger files changed between from and to only gain or lose
// whole tasks: each is there at to, a file new at to has only new tasks, and every task at both
// ends is unchanged. A task that leaves one file and enters another is a move, which changes
// its group, so it is not a whole entry added or removed.
func wholeEntriesOnly(cfg *config.Loaded, from, to string, files []string) bool {
	ctx := context.Background()
	was, err := cfg.Trees.At(ctx, from)
	if err != nil {
		return false
	}
	now, err := cfg.Trees.At(ctx, to)
	if err != nil {
		return false
	}
	added, gone := map[string]bool{}, map[string]bool{}
	for _, file := range files {
		after, ok := entries(now, file)
		if !ok {
			return false
		}
		var before map[string]string
		if was.Has(ctx, file) {
			if before, ok = entries(was, file); !ok {
				return false
			}
		}
		for id, task := range after {
			if old, had := before[id]; !had {
				added[id] = true
			} else if old != task {
				return false
			}
		}
		for id := range before {
			if _, kept := after[id]; !kept {
				gone[id] = true
			}
		}
	}
	for id := range added {
		if gone[id] {
			return false
		}
	}
	return true
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
		if !wholeEntriesOnly(cfg, from, to, files) {
			return full, nil
		}
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
