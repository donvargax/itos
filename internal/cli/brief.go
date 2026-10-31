package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	briefslice "github.com/donvargax/itos/v8/internal/slices/brief"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runBrief is `brief`, the brief slice (internal/slices/brief) over its
// ports bound to the real code: git, the config and its data read from
// the commit's tree, and git's tree entries and first-parent log.
func runBrief(args []string, o Out) (int, error) {
	c, err := briefslice.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, briefslice.NewHandler(briefPorts{Git: vcs.Git{Dir: o.Dir}, o: o}))
}

// briefPorts are the brief slice's ports over the real code: the git
// adapter, and the config read from a commit's tree in o's folder.
type briefPorts struct {
	vcs.Git
	o Out
}

// Config is the config at its path as the commit's tree has it, it and its
// data read from that tree (a stealth config, beside the git folder, is
// read from there as ever): the brief is a function of the commit, not of
// an uncommitted change to the policy.
func (p briefPorts) Config(ctx context.Context, commit string) (*config.Loaded, error) {
	t := trees.Trees{VCS: p.Git}
	src, err := t.At(ctx, commit)
	if err != nil {
		return nil, kind.Else(kind.Missing, err)
	}
	return config.Load(p.Git, t, src, config.Path(p.Git, src))
}

// TreeEntries are the git adapter's FolderEntries, each entry's kind, mode
// and object: a path is the folder's, as the config's paths are.
func (p briefPorts) TreeEntries(ctx context.Context, commit string, paths []string) (map[string]briefslice.TreeEntry, error) {
	listed, err := p.Git.FolderEntries(ctx, commit, paths)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]briefslice.TreeEntry, len(listed))
	for name, e := range listed {
		entries[name] = briefslice.TreeEntry{Kind: e.Kind, Mode: e.Mode, Object: e.Object}
	}
	return entries, nil
}
