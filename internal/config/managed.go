package config

// A repository itos manages, for the git shim (slice 41, features/shim.feature):
// the shim asks on every git command whether the folder git runs in has a
// config itos would read, and must answer in a few file checks, with no git
// run, so it finds the work tree with the VCS's Find, which reads files
// alone, instead of Locate. Its answer is Locate's and Top's
// (managed_test.go holds it to them): ITOS_CONFIG when it names a file;
// else an itos.yaml in the folder, which wins as it does there; else, in a
// work tree, the itos.yaml at its top or the stealth config in its common
// dir. Inside a git folder, where there is no work tree for a commit,
// nothing is managed.

import (
	"context"
	"os"
	"path/filepath"

	vcs "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Managed is whether itos run in the folder dir ("" for the one itos runs
// in, v's Folder) reads a config there, told by file checks alone.
func Managed(v vcs.VCS, dir string) bool {
	base := folder(v, dir)
	if p := os.Getenv("ITOS_CONFIG"); p != "" {
		return exists(in(base, p))
	}
	if exists(in(base, fileName)) {
		return true
	}
	l, ok := v.Find(context.Background(), dir)
	if !ok {
		return false
	}
	return exists(filepath.Join(l.Top, fileName)) || exists(filepath.Join(l.Common, StealthFolder, fileName))
}

// exists is whether anything is at the path.
func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
