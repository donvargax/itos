package config

// From a subfolder (slice 40, features/config.feature): a run with no
// --config, no ITOS_CONFIG and no --root, in a folder with no itos.yaml of
// its own, inside a git repository, looks for the config at the repository's
// top level, the itos.yaml there or the stealth config, and, finding one,
// runs as if started at the top, as --root <top> would, so every path the
// config names means what it means there. A folder's own itos.yaml still
// wins, a run at the top asks git nothing more, and outside a repository
// nothing changes. The command line moves there before any command runs
// (internal/cli), and the launcher reads the pin there (internal/launch).

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/donvargax/itos/v3/internal/git"
)

var (
	topMu sync.Mutex
	tops  = map[string]string{}
)

// Top is the repository's top level when a run in the folder dir ("" for
// the current one) reads its config from there: no ITOS_CONFIG, no itos.yaml
// in dir, dir below the top of a repository that has an itos.yaml at its top
// or a stealth config. "" when the run stays where it is. Asked once per
// folder and run, so the launcher and the command line ask git once.
func Top(dir string) string {
	if os.Getenv("ITOS_CONFIG") != "" {
		return ""
	}
	if _, err := os.Stat(in(dir, fileName)); err == nil {
		return ""
	}
	key := dir
	if cwd, err := os.Getwd(); err == nil {
		key = cwd + "\x00" + dir
	}
	topMu.Lock()
	defer topMu.Unlock()
	if top, ok := tops[key]; ok {
		return top
	}
	top := findTop(dir)
	tops[key] = top
	return top
}

// findTop asks git for the top level and the common dir in one run; inside
// the git folder, where there is no top level, the run stays where it is.
func findTop(dir string) string {
	args := []string{"rev-parse", "--show-toplevel", "--git-common-dir"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	out, err := git.Output(args...)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] == "" {
		return ""
	}
	top, common := lines[0], lines[1]
	if same(top, in(dir, ".")) {
		return ""
	}
	if _, err := os.Stat(filepath.Join(top, fileName)); err == nil {
		return top
	}
	if _, err := os.Stat(in(dir, filepath.Join(common, StealthFolder, fileName))); err == nil {
		return top
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
