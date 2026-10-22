package adapter

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// abs is filepath.Abs, which a test swaps to reach its failure: it fails
// only when the working folder cannot be read.
var abs = filepath.Abs

// Find walks up from dir as git discovers a repository, to the first
// folder with a .git, or the first that is a git folder itself (no work
// tree then). Symbolic links in dir are resolved first, as git's working
// folder is the physical one.
func (Git) Find(_ context.Context, dir string) (port.Location, bool) {
	if dir == "" {
		dir = "."
	}
	d, err := abs(dir)
	if err != nil {
		return port.Location{}, false
	}
	if real, err := filepath.EvalSymlinks(d); err == nil {
		d = real
	}
	for {
		if gitFolder(d) {
			return port.Location{}, false
		}
		dotgit := filepath.Join(d, ".git")
		if info, err := os.Stat(dotgit); err == nil {
			gitdir := dotgit
			if !info.IsDir() {
				var ok bool
				if gitdir, ok = gitdirOf(d, dotgit); !ok {
					return port.Location{}, false
				}
			}
			return port.Location{Top: d, GitDir: gitdir, Common: commonDir(gitdir)}, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return port.Location{}, false
		}
		d = parent
	}
}

// gitFolder is whether d is a git folder itself, as git tells one: a HEAD
// file beside objects and refs folders.
func gitFolder(d string) bool {
	if _, err := os.Stat(filepath.Join(d, "HEAD")); err != nil {
		return false
	}
	objects, err := os.Stat(filepath.Join(d, "objects"))
	if err != nil || !objects.IsDir() {
		return false
	}
	refs, err := os.Stat(filepath.Join(d, "refs"))
	return err == nil && refs.IsDir()
}

// gitdirOf is the git folder a .git file names ("gitdir: <path>", relative
// to the folder holding it), as a linked worktree or a submodule has one,
// in the system's form: git writes it with forward slashes on Windows too.
func gitdirOf(d, file string) (string, bool) {
	text, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	p, ok := strings.CutPrefix(strings.TrimSpace(string(text)), "gitdir:")
	if p = strings.TrimSpace(p); !ok || p == "" {
		return "", false
	}
	return within(d, p), true
}

// within is the path p names from the folder d, cleaned to the system's
// form.
func within(d, p string) string {
	if !filepath.IsAbs(p) {
		return filepath.Join(d, p)
	}
	return filepath.Clean(p)
}

// commonDir is the git folder's common dir: the folder its commondir file
// names (relative to it), as a linked worktree's has, else itself.
func commonDir(gitdir string) string {
	text, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return gitdir
	}
	p := strings.TrimSpace(string(text))
	if p == "" {
		return gitdir
	}
	return within(gitdir, p)
}
