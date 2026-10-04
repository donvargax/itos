package plan

import (
	"strings"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/git"
	"github.com/donvargax/itos/v3/internal/glob"
	"github.com/donvargax/itos/v3/internal/message"
)

// What a pushed range is (ci-scope.ts): the files it touched, whether it can
// be read, whether it is prose only, and the tasks and named tests its
// commits' footers name. A range that cannot be read (a first push, an empty
// start, a rewritten history) touched nothing and names nothing, and is not
// prose, so the shortcut is never taken on a guess and every test runs.

// Changed are the files a pushed range touched (`git diff --name-only from
// to`), the paths its commits touch for the unpushed commits (git.Unpushed);
// none when it cannot be read or has no start or end.
func Changed(from, to string) []string {
	if from == "" || to == "" {
		return nil
	}
	if from == git.Unpushed {
		paths, _ := git.UnpushedPaths(to)
		return paths
	}
	diff, err := git.Output("diff", "--name-only", from, to)
	if err != nil {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(diff, "\n") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// Readable is whether git can read a range at all (`git rev-list --quiet
// from..to`, or to --not --remotes for the unpushed commits); not with no
// start or end.
func Readable(from, to string) bool {
	return from != "" && to != "" && git.Succeeds(append([]string{"rev-list", "--quiet"}, git.Revs(from, to)...)...)
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
