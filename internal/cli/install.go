package cli

// `hook install [--print] [--force]`: itos's commit-msg and pre-push hooks
// declared in the clone's git config (internal/engine/hooks), every
// worktree of it sharing them. Since slice 91 (decision 37) that is the only place itos
// installs them: it knows no hook manager, and never reads or writes a hook
// file, a hook manager's config or core.hooksPath. A project's own hooks are
// its own business; git runs them beside itos's.

import (
	"context"
	"os"

	hookslice "github.com/donvargax/itos/v8/internal/slices/hook"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// hookInstall is `hook install`, the hook slice's, which init runs.
func hookInstall(print, force bool, o Out) (int, error) {
	c := hookslice.CLI{Query: hookslice.Query{Sub: "install", Print: print, Force: force}, JSON: o.JSON, Tests: o.Tests}
	return c.Run(context.Background(), o.Stdout, o.Stderr, hookslice.NewHandler(hookPorts{Git: vcs.Git{Dir: o.Dir}, os: o.OS}))
}
