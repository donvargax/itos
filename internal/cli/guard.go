package cli

import (
	"context"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/slices/guard"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// GuardSince is the first itos with guard claude-code (v6.0.0, decision 36;
// guard claude-code before it): the launcher answers nothing for the guard
// where it would hand it to an older itos, which has no such command, and
// whose usage error's exit 2 Claude Code would take as a block (slice 44,
// internal/launch).
const GuardSince = "6.0.0"

// guardCommand is `guard <harness>`, the guard slice (internal/slices/guard)
// over its ports bound to the real code; the folder judged when the input
// names none, and the one a relative one is read from, is where itos was
// started, before a run from a subfolder moved to the top.
func guardCommand(args []string, o Out) (int, error) {
	c, err := guard.Parse(args, typed(o.Origin, "."))
	if err != nil {
		return 0, usage("%s", err)
	}
	return c.Run(context.Background(), os.Stdin, o.Stdout, o.Stderr, guard.NewHandler(guardPorts{})), nil
}

// guardPorts are the guard slice's ports over the real code.
type guardPorts struct{}

// Managed is config.Managed.
func (guardPorts) Managed(_ context.Context, dir string) bool {
	return config.Managed(vcs.Git{}, dir)
}
