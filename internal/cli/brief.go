package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	briefslice "github.com/donvargax/itos/v8/internal/slices/brief"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runBrief is `brief`, the brief slice (internal/slices/brief) over its
// ports bound to the real code: git, the config read from the working
// tree, and git's tree entries and first-parent log.
func runBrief(args []string, o Out) (int, error) {
	c, err := briefslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, briefslice.NewHandler(briefPorts{Git: vcs.Git{Dir: o.Dir}, o: o}))
}

// briefPorts are the brief slice's ports over the real code: the git
// adapter, and loadConfig in o's folder.
type briefPorts struct {
	vcs.Git
	o Out
}

// Config is loadConfig.
func (p briefPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig(p.o.Dir) }

// TreeEntries is the git adapter's, each entry's kind and object.
func (p briefPorts) TreeEntries(ctx context.Context, commit string, paths []string) (map[string]briefslice.TreeEntry, error) {
	listed, err := p.Git.TreeEntries(ctx, commit, paths)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]briefslice.TreeEntry, len(listed))
	for name, e := range listed {
		entries[name] = briefslice.TreeEntry{Kind: e.Kind, Object: e.Object}
	}
	return entries, nil
}
