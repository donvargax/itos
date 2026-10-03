package config

// A repository itos manages, for the git shim (slice 41, features/shim.feature):
// the shim asks on every git command whether the folder git runs in has a
// config itos would read, and must answer in a few file checks, with no git
// run, so it walks up to the .git as git does instead of asking git. Its
// answer is Locate's and Top's (managed_test.go holds it to them): ITOS_CONFIG
// when it names a file; else an itos.yaml in the folder, which wins as it
// does there; else, in a work tree, the itos.yaml at its top or the stealth
// config in its common dir, the folder a .git file's gitdir and that
// gitdir's commondir name in a linked worktree. Inside a git folder, where
// there is no work tree for a commit, nothing is managed.

import (
	"os"
	"path/filepath"
	"strings"
)

// Managed is whether itos run in the folder dir ("" for the current one)
// reads a config there, told by file checks alone.
func Managed(dir string) bool {
	if p := os.Getenv("ITOS_CONFIG"); p != "" {
		return exists(in(dir, p))
	}
	if exists(in(dir, fileName)) {
		return true
	}
	top, common, ok := WorkTree(dir)
	if !ok {
		return false
	}
	return exists(filepath.Join(top, fileName)) || exists(filepath.Join(common, StealthFolder, fileName))
}

// WorkTree is the top of the git work tree holding the folder dir and its
// common git dir, found as git discovers a repository, by walking up to the
// first folder with a .git, or the first that is a git folder itself (no work
// tree then). Symbolic links in dir are resolved first, as git's working
// folder is the physical one.
func WorkTree(dir string) (top, common string, ok bool) {
	if dir == "" {
		dir = "."
	}
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", "", false
	}
	if real, err := filepath.EvalSymlinks(d); err == nil {
		d = real
	}
	for {
		if gitFolder(d) {
			return "", "", false
		}
		dotgit := filepath.Join(d, ".git")
		if info, err := os.Stat(dotgit); err == nil {
			gitdir := dotgit
			if !info.IsDir() {
				if gitdir, ok = gitdirOf(d, dotgit); !ok {
					return "", "", false
				}
			}
			return d, commonDir(gitdir), true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", "", false
		}
		d = parent
	}
}

// gitFolder is whether d is a git folder itself, as git tells one: a HEAD
// file beside objects and refs folders.
func gitFolder(d string) bool {
	if !exists(filepath.Join(d, "HEAD")) {
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
// to the folder holding it), as a linked worktree or a submodule has one.
func gitdirOf(d, file string) (string, bool) {
	text, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	p, ok := strings.CutPrefix(strings.TrimSpace(string(text)), "gitdir:")
	if p = strings.TrimSpace(p); !ok || p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(d, p)
	}
	return p, true
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
	if !filepath.IsAbs(p) {
		p = filepath.Join(gitdir, p)
	}
	return p
}

// exists is whether anything is at the path.
func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
