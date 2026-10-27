package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// copyTree leaves out a lock file and treats an entry that is listed but gone
// when it is read (a dangling link stands for one git's maintenance removed
// between the walk and the copy) as gone, copying everything else.
func TestCopyTreeLeavesOutLocksAndGoneEntries(t *testing.T) {
	from := t.TempDir()
	for name, text := range map[string]string{"a.txt": "a", "sub/b.txt": "b", "sub/index.lock": "x", "maintenance.lock": "x"} {
		path := filepath.Join(from, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Where links cannot be made, only the lock files are tried.
	linked := os.Symlink("missing", filepath.Join(from, "gone")) == nil
	to := filepath.Join(t.TempDir(), "copy")
	copyTree(t, from, to)
	for name, want := range map[string]bool{"a.txt": true, "sub/b.txt": true, "sub/index.lock": false, "maintenance.lock": false, "gone": false} {
		if name == "gone" && !linked {
			continue
		}
		_, err := os.Lstat(filepath.Join(to, filepath.FromSlash(name)))
		if (err == nil) != want {
			t.Errorf("%s copied = %v, want %v", name, err == nil, want)
		}
	}
}
