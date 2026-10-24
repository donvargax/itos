package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/slices/guard"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// runGuard is `guard <harness>`, the guard slice (internal/slices/guard)
// over its ports bound to the real code; the folder judged when the input
// names none, and the one a relative one is read from, is where itos was
// started, before a run from a subfolder moved to the top.
func runGuard(args []string, o Out) (int, error) {
	c, err := guard.Parse(args, within(o.Dir, typed(o.Origin, ".")))
	if err != nil {
		return 0, usage("%s", err)
	}
	return c.Run(context.Background(), os.Stdin, o.Stdout, o.Stderr, guard.NewHandler(guardPorts{vcs.Git{Dir: o.Dir}})), nil
}

// guardPorts are the guard slice's ports over the real code: git in the
// folder itos runs in.
type guardPorts struct{ git vcs.Git }

// Managed is config.Managed.
func (p guardPorts) Managed(_ context.Context, dir string) bool {
	return config.Managed(p.git, dir)
}
