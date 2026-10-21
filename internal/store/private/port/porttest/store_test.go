package porttest

import (
	"path/filepath"
	"testing"
)

// The fake keeps the store as the adapter keeps it on disk.
func TestStoreKeepsTheContract(t *testing.T) {
	StoreContract(t, func(t *testing.T, repository bool) Fixture {
		root := t.TempDir()
		f := Fixture{
			Kept:    filepath.Join(root, "kept.md"),
			Ignored: filepath.Join(root, "ignored.md"),
			Outside: filepath.Join(t.TempDir(), "outside.md"),
		}
		if !repository {
			f.Store = &Store{}
			return f
		}
		f.Store = &Store{Folder: filepath.Join(root, ".git", "itos"), Exposed: []string{f.Kept}}
		return f
	})
}
