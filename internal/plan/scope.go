package plan

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/glob"
	"github.com/donvargax/itos/v8/internal/message"
	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// What a pushed range is (ci-scope.ts): the files it touched, whether it can
// be read, whether it is prose only, and the tasks and named tests its
// commits' footers name. A range that cannot be read (a first push, an empty
// start, a rewritten history) touched nothing and is not prose, so the
// shortcut is never taken on a guess and every test runs. An empty start
// still names what every commit up to the end names (bug 33), so the tasks
// those commits name run their checks; a start git cannot read names nothing.

// Changed are the files a pushed range touched (the paths that differ
// between its ends, each named as it is, bug 31), the paths its commits
// touch for the unpushed commits (vcs.Unpushed), a merge's none; none when
// it cannot be read or has no start or end. A rename is both its paths (bug
// 32), so a range that moves code into prose paths is not prose-only.
func Changed(v vcs.VCS, from, to string) []string {
	paths, _ := ChangedErr(v, from, to)
	return paths
}

// ChangedErr is Changed with the error of the read that failed, which
// Changed drops; none for a range with no start or end, which has nothing
// to read.
func ChangedErr(v vcs.VCS, from, to string) ([]string, error) {
	if from == "" || to == "" {
		return nil, nil
	}
	ctx := context.Background()
	var paths []string
	var err error
	if from == vcs.Unpushed {
		paths, err = v.Touched(ctx, vcs.Range{From: from, To: to}, false)
	} else {
		paths, err = v.Diff(ctx, from, to)
	}
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// Readable is whether v can read a range at all (from..to, or the unpushed
// commits of to); not with no start or end.
func Readable(v vcs.VCS, from, to string) bool {
	return from != "" && to != "" && v.Readable(context.Background(), vcs.Range{From: from, To: to})
}

// DocsOnly is whether every path is prose (ci.prose.paths), and there is at
// least one: without prose paths nothing is prose. A config error without a
// ci section; an error for a glob V8 would refuse, when a path reaches it.
func DocsOnly(cfg *config.Loaded, paths []string) (bool, error) {
	if err := cfg.Section("ci"); err != nil {
		return false, err
	}
	if cfg.CI.Prose == nil || cfg.CI.Prose.Paths == nil || len(paths) == 0 {
		return false, nil
	}
	for _, p := range paths {
		ok, err := glob.MatchesAny(p, cfg.CI.Prose.Paths)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// namedIn are the IDs a range's commits give in every footer that of picks,
// each once, footer by footer in the config's order.
func namedIn(cfg *config.Loaded, from, to string, of func(config.Footer) bool) []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, key := range cfg.Commits.Footers.Keys {
		f, _ := cfg.Commits.Footers.Get(key)
		if !of(f) {
			continue
		}
		for _, id := range message.IDsIn(cfg, from, to, key) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// TasksIn are the tasks a range's commits name in their ledger footers: each
// footer whose source is the ledger, whatever commits.footers calls it.
func TasksIn(cfg *config.Loaded, from, to string) []string {
	return namedIn(cfg, from, to, func(f config.Footer) bool {
		return f.Source.IsName && f.Source.Name == "ledger"
	})
}

// TestsNamedIn are the tests of a kind a range's footers name (`Scenarios:`
// for the scenario kind); none for the kind "".
func TestsNamedIn(cfg *config.Loaded, from, to, kind string) []string {
	return namedIn(cfg, from, to, func(f config.Footer) bool {
		return kind != "" && !f.Source.IsName && f.Source.Tests == kind
	})
}
