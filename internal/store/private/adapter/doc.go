// Package adapter is the private store's adapter (internal/store/private/port)
// over the disk and git: each file in config.StealthFolder of the absolute
// git common dir of the folder itos runs in (git rev-parse
// --path-format=absolute --git-common-dir, read on every call, so a command
// that moves to the top finds the same store), where the stealth mode keeps
// its data. Write writes a file beside the target, mode 0600, syncs it and
// moves it over, so a reader in another worktree never sees half of it and
// a crash leaves the old file or the new; Hold is internal/lock's lock file;
// NotIgnored asks git check-ignore. internal/cli binds it to the slices'
// ports; its tests run porttest.StoreContract, as the fake's do.
package adapter
