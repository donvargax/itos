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

// configHooksRun is whether this git runs the hooks its config declares
// (the git adapter's ConfigHooksRun).
func configHooksRun() bool { return (vcs.Git{}).ConfigHooksRun(context.Background()) }

// gitVersion is the version the git itos runs gives, as "git version"
// prints it after those words.
func gitVersion() string { return (vcs.Git{}).Version(context.Background()) }

// entryDeclared is hooks.Declared over the git adapter.
func entryDeclared(event string) bool { return hooks.Declared(context.Background(), vcs.Git{}, event) }

// hooksReady is hooks.Ready over the git adapter.
func hooksReady(events ...string) error {
	return hooks.Ready(context.Background(), vcs.Git{}, events...)
}

// declareHooks is hooks.Declare over the git adapter.
func declareHooks(bin string, print, force bool, o Out, say func(string)) (int, error) {
	return hooks.Declare(context.Background(), vcs.Git{}, bin, print, force,
		hooks.Out{Stdout: o.Stdout, Stderr: o.Stderr, JSON: o.JSON}, say)
}

// hooksPathSet is whether git -c sets core.hooksPath (hooks.PathSet).
func hooksPathSet() bool { return hooks.PathSet() }
