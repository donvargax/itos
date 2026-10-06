package cli

// `hook install [--print] [--force]`: itos's commit-msg and pre-push hooks
// declared in the clone's git config (gitconfig.go), every worktree of it
// sharing them. Since slice 91 (decision 37) that is the only place itos
// installs them: it knows no hook manager, and never reads or writes a hook
// file, a hook manager's config or core.hooksPath. A project's own hooks are
// its own business; git runs them beside itos's.

import (
	"fmt"
	"os"

	"github.com/donvargax/itos/v5/internal/config"
)

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readIf is a file's text, and whether it is there.
func readIf(p string) (string, bool) {
	text, err := os.ReadFile(p)
	return string(text), err == nil
}

// hookInstall is `hook install`: 0 when both hooks are declared (or
// printed), 1 when an entry under itos's name that does not call itos stood
// in the way and --force was not given, 3 when this git runs no hook its
// config declares.
func hookInstall(print, force bool, o Out) (int, error) {
	const root = "."
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	log := o.Stdout
	if o.JSON {
		log = o.Stderr
	}
	return declareHooks(root, cfg.Hooks.Bin, print, force, o, func(line string) { fmt.Fprintln(log, line) })
}
