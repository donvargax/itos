package cli

// itos's hooks in the git config or the hooks folder, internal/engine/hooks
// bound to the git adapter and the OS under the names the commands call.

import (
	"context"
	"os"
	"path/filepath"

	"github.com/donvargax/itos/v8/internal/engine/hooks"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// hookEvents are the two hooks itos declares, in the order it writes them.
var hookEvents = hooks.Events

// hookEntry is the name itos's entry for a hook event has in the git config.
func hookEntry(event string) string { return hooks.Entry(event) }

// hooksReady is hooks.Ready over the git adapter and the OS.
func hooksReady(events ...string) error {
	return hooks.Ready(context.Background(), hooksPorts{}, events...)
}

// hooksPorts are hooks.Ports over the git adapter, which says where the
// hooks folder is and what git tracks, and the OS, which holds its files.
type hooksPorts struct {
	vcs.Git
	hookFiles
}

// hookFiles are the hooks folder's files on the OS, hooks.Folder's file
// operations.
type hookFiles struct{}

// HookFile is the file's text.
func (hookFiles) HookFile(_ context.Context, path string) (string, error) {
	text, err := os.ReadFile(path)
	return string(text), err
}

// WriteHookFile writes the file executable, making its folder.
func (hookFiles) WriteHookFile(_ context.Context, path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

// MoveHookFile renames the file.
func (hookFiles) MoveHookFile(_ context.Context, from, to string) error { return os.Rename(from, to) }

// hooksPathSet is whether git -c sets core.hooksPath (hooks.PathSet).
func hooksPathSet() bool { return hooks.PathSet() }
