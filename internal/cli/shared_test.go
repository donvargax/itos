package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	sharedOnce sync.Once
	sharedRoot string
	sharedErr  error
)

// shared is a folder for what the package's tests make once and read, removed
// by TestMain.
func shared(t *testing.T) string {
	t.Helper()
	sharedOnce.Do(func() { sharedRoot, sharedErr = os.MkdirTemp("", "itos-cli-") })
	if sharedErr != nil {
		t.Fatal(sharedErr)
	}
	return sharedRoot
}

// madeOnce makes what the package's tests share under name, with build in a
// folder of its own, once, under the first caller's settings. git starts are
// what the tests spend. It is the folder holding what build wrote.
func madeOnce(t *testing.T, name string, build func(t *testing.T, dir string)) string {
	t.Helper()
	dir := filepath.Join(shared(t), name)
	if _, err := os.Stat(dir + ".made"); err == nil {
		return dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	build(t, dir)
	if err := os.WriteFile(dir+".made", []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// copyTree copies the folder from to a new folder to, files and folders
// alone, a file executable where the original is (a hook).
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, text, 0o644|info.Mode().Perm()&0o111)
	})
	if err != nil {
		t.Fatal(err)
	}
}
