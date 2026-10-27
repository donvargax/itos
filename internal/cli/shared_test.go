package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// noMaintenance is the start of a shell script whose git commands start no
// background maintenance, which would make and remove locks in a repository a
// template copy walks. A bare repository a script pushes into needs
// "git -C <it> config maintenance.auto false" too: a push does not hand it the
// client's settings.
const noMaintenance = "export GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=maintenance.auto GIT_CONFIG_VALUE_0=false GIT_CONFIG_KEY_1=gc.auto GIT_CONFIG_VALUE_1=0\n"

// copyTree copies the folder from to a new folder to, files and folders
// alone, a file executable where the original is (a hook). A lock file is not
// copied, and an entry git removes between the walk and its copy is gone, not
// an error: git's background maintenance makes and removes locks in a
// repository a command has just touched.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".lock") {
			return nil
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
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		text, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		return os.WriteFile(dst, text, 0o644|info.Mode().Perm()&0o111)
	})
	if err != nil {
		t.Fatal(err)
	}
}
