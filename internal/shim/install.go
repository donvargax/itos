package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

// Name is the link's name on the OS goos: git, git.exe on windows.
func Name(goos string) string {
	if goos == "windows" {
		return "git.exe"
	}
	return "git"
}

// Place is the running itos, as executable (os.Executable) tells it, and the
// link's path in dir, by default the folder holding that itos, made absolute.
func Place(executable func() (string, error), dir, goos string) (self, link string, err error) {
	self, err = executable()
	if err != nil {
		return "", "", fmt.Errorf("cannot tell the itos binary running: %w", err)
	}
	if dir == "" {
		dir = filepath.Dir(self)
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return "", "", err
	}
	return self, filepath.Join(dir, Name(goos)), nil
}

// LinksItos is what the git at link is: this itos (a link to it, followed),
// or another itos: a symbolic link to a binary named itos, even one since
// moved or deleted, as an older install's, or any program the git adapter's
// IsItos tells is itos, a hard link to one or a copy of it among them.
func LinksItos(link string) (self, other bool) {
	if (vcs.Git{}).IsSelf(link) {
		return true, false
	}
	if target, err := os.Readlink(link); err == nil {
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(target)), ".exe")
		if name == "itos" {
			return false, true
		}
	}
	return false, (vcs.Git{}).IsItos(link)
}

// MakeLink makes link a symbolic link to self, replacing the one there when
// told to; where symbolic links need a privilege (goos windows), a hard
// link.
func MakeLink(self, link string, replace bool, goos string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if replace {
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	err := os.Symlink(self, link)
	if err != nil && goos == "windows" {
		err = os.Link(self, link)
	}
	return err
}

// PathStanding is the place of the folder dir among the PATH's folders and
// the real git's on goos (the git adapter's Real), -1 when it is not there or
// there is none.
func PathStanding(dir, goos string) (at int, realGit string, realAt int) {
	at, realAt = -1, -1
	realGit, _ = vcs.Git{Platform: goos}.Real()
	for i, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" || !filepath.IsAbs(entry) {
			continue
		}
		if at < 0 && sameFolder(entry, dir) {
			at = i
		}
		if realAt < 0 && realGit != "" && sameFolder(entry, filepath.Dir(realGit)) {
			realAt = i
		}
	}
	return at, realGit, realAt
}

// sameFolder is whether two paths name one folder.
func sameFolder(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

// StandingLine says where the link's folder stands on the PATH, and what to
// do when git does not run the shim yet.
func StandingLine(dir string, at int, realGit string, realAt int) string {
	switch {
	case at < 0 && realGit != "":
		return fmt.Sprintf("%s is not on the PATH: put it before %s, the real git's folder, for git to run the shim.",
			dir, filepath.Dir(realGit))
	case at < 0:
		return fmt.Sprintf("%s is not on the PATH, and the PATH has no other git for the shim to run.", dir)
	case realGit == "":
		return fmt.Sprintf("%s is on the PATH, but the PATH has no other git for the shim to run: install git.", dir)
	case at < realAt:
		return fmt.Sprintf("%s comes before the real git (%s) on the PATH: git runs the shim, and in a repository "+
			"itos manages git commit and git push are itos's.", dir, realGit)
	}
	return fmt.Sprintf("%s comes after the real git (%s) on the PATH: put it first for git to run the shim.", dir, realGit)
}
