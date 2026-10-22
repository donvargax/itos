package cli

// itos's hooks in the git config, internal/engine/hooks bound to the git
// adapter under the names the commands call.

import (
	"context"

	"github.com/donvargax/itos/v8/internal/engine/hooks"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// configHooksSince is the first git that runs the hooks its config declares.
const configHooksSince = hooks.Since

// hookEvents are the two hooks itos declares, in the order it writes them.
var hookEvents = hooks.Events

// hookEntry is the name itos's entry for a hook event has in the git config.
func hookEntry(event string) string { return hooks.Entry(event) }

// hooksReady is hooks.Ready over the git adapter.
func hooksReady(events ...string) error {
	return hooks.Ready(context.Background(), vcs.Git{}, events...)
}

// hooksPathSet is whether git -c sets core.hooksPath (hooks.PathSet).
func hooksPathSet() bool { return hooks.PathSet() }
