package cli

import (
	"context"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/status"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/slices/gocmd"
	"github.com/donvargax/itos/v8/internal/slices/guide"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runGuide is `guide <name>`, the guide slice (internal/slices/guide) over
// its ports bound to the real code: git, the trees over it and the working
// tree.
func runGuide(args []string, o Out) (int, error) {
	c, err := guide.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, guide.NewHandler(guidePorts{}))
}

// guidePorts are the guide slice's ports over the real code: git, the trees
// over it and the working tree.
type guidePorts struct{ vcs.Git }

// Trees is the tree adapter over git.
func (guidePorts) Trees(context.Context) port.Trees { return trees.Trees{VCS: vcs.Git{}} }

// Worktree is the working tree's files.
func (guidePorts) Worktree(context.Context) port.Source { return trees.Worktree }

// runGo is `go`, the go slice (internal/slices/gocmd) over its ports bound
// to the real code: the guide slice's, and statusOf for where things stand.
func runGo(args []string, o Out) (int, error) {
	c, err := gocmd.Parse(args, o.JSON)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, gocmd.NewHandler(goPorts{o: o}))
}

// goPorts are the go slice's ports over the real code: the guide slice's,
// and statusOf on o for the person the identity provider names.
type goPorts struct {
	guidePorts
	o Out
}

// Standing is statusOf, for the person the identity provider names.
func (p goPorts) Standing(_ context.Context, cfg *config.Loaded) (*status.Standing, []out.Problem, error) {
	st, found, _, err := statusOf(cfg, "", p.o)
	if st == nil {
		return nil, found, err
	}
	return st.Standing, found, err
}
