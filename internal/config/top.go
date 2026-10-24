package config

// From a subfolder (slice 40, features/config.feature): a run with no
// --config, no ITOS_CONFIG and no --root, in a folder with no itos.yaml of
// its own, inside a git repository, looks for the config at the repository's
// top level, the itos.yaml there or the stealth config, and, finding one,
// runs as if started at the top, as --root <top> would, so every path the
// config names means what it means there. A folder's own itos.yaml still
// wins, a run at the top asks git nothing more, and outside a repository
// nothing changes. The command line runs there, the folder it hands each
// command, the process's own never changed (internal/cli), and the launcher
// reads the pin there (internal/launch).

import (
	"context"
	"os"
	"path/filepath"

	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Top is the repository's top level when a run in the folder dir ("" for
// the one itos runs in, v's Folder) reads its config from there: no ITOS_CONFIG, no itos.yaml
// in dir, dir below the top of a repository that has an itos.yaml at its top
// or a stealth config. "" when the run stays where it is. v locates the
// repository, once per folder and run, so the launcher and the command line
// ask git once.
func Top(v vcs.VCS, dir string) string {
	if os.Getenv("ITOS_CONFIG") != "" {
		return ""
	}
	if _, err := os.Stat(in(folder(v, dir), fileName)); err == nil {
		return ""
	}
	return findTop(v, dir)
}

// findTop is the top v locates with the common dir; inside the git folder,
// where there is no top level, the run stays where it is.
func findTop(v vcs.VCS, dir string) string {
	l, err := v.Locate(context.Background(), dir)
	if err != nil || l.Top == "" {
		return ""
	}
	base := folder(v, dir)
	if same(l.Top, in(base, ".")) {
		return ""
	}
	if _, err := os.Stat(filepath.Join(l.Top, fileName)); err == nil {
		return l.Top
	}
	if _, err := os.Stat(in(base, filepath.Join(l.Common, StealthFolder, fileName))); err == nil {
		return l.Top
	}
	return ""
}

// same is whether two paths name one folder.
func same(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}
