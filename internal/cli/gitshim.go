package cli

// itos git-shim (slice 41, features/shim.feature): install links the itos
// binary running as git in a folder, by default the one holding it, and says
// where that folder stands on the PATH against the real git, since the shim
// only acts before it; uninstall takes the link away. Neither replaces nor
// removes a git that is not a link to itos. run is what the link runs in a
// repository itos manages (internal/shim): git commit or git push as itos
// commit or itos push, with git's arguments, which the shim writes after a
// "--", so none of them is read as an itos global flag.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donvargax/itos/v5/internal/git"
	"github.com/donvargax/itos/v5/internal/out"
)

// GitShimSince is the first itos with the git-shim command: the shim hands
// git commit and git push to an older pinned itos as nothing, running the
// real git instead (bug 7, internal/shim).
const GitShimSince = "2.2.0"

// gitShim is `git-shim install [--dir <folder>]`, `git-shim uninstall [--dir
// <folder>]` and `git-shim run <commit|push> [<git args>…]`.
func gitShim(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "install", "uninstall":
		dir := ""
		for i := 0; i < len(rest); i++ {
			if rest[i] != "--dir" {
				return 0, usage("git-shim %s takes only --dir <folder> (%s)", sub, rest[i])
			}
			if i+1 >= len(rest) || rest[i+1] == "" {
				return 0, usage("git-shim %s --dir needs <folder>", sub)
			}
			i++
			dir = typed(rest[i])
		}
		if sub == "install" {
			return installShim(dir, o)
		}
		return uninstallShim(dir, o)
	case "run":
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		name, gitArgs := split(rest)
		switch name {
		case "commit":
			return gitCommit(gitArgs, o)
		case "push":
			return push(gitArgs, o)
		}
		return 0, usage("git-shim run needs commit or push, then git's arguments")
	}
	return 0, usage("git-shim needs install, uninstall or run")
}

// shimName is the link's name: git, git.exe on windows.
func shimName() string {
	if runtime.GOOS == "windows" {
		return "git.exe"
	}
	return "git"
}

// shimPlace is the running itos and the link's path in dir, by default the
// folder holding that itos, made absolute.
func shimPlace(dir string) (self, link string, err error) {
	self, err = os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("cannot tell the itos binary running: %w", err)
	}
	if dir == "" {
		dir = filepath.Dir(self)
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return "", "", err
	}
	return self, filepath.Join(dir, shimName()), nil
}

// linksItos is what the git at link is: this itos (a link to it, followed),
// or a symbolic link to another binary named itos, as an older install's.
func linksItos(link string) (self, other bool) {
	if git.IsSelf(link) {
		return true, false
	}
	target, err := os.Readlink(link)
	if err != nil {
		return false, false
	}
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(target)), ".exe")
	return false, name == "itos"
}

// installShim links the running itos as git in dir, unless a git that is not
// a link to itos is there, and says where dir stands on the PATH.
func installShim(dir string, o Out) (int, error) {
	self, link, err := shimPlace(dir)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
		return ExitMissing, nil
	}
	action := "linked"
	if _, err := os.Lstat(link); err == nil {
		mine, other := linksItos(link)
		switch {
		case mine:
			action = "kept"
		case other:
			action = "replaced"
		default:
			return shimRefused(link, "is a git that is not a link to itos; git-shim install never replaces one", o)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	if action != "kept" {
		if err := makeLink(self, link, action == "replaced"); err != nil {
			fmt.Fprintf(o.Stderr, "itos: cannot link %s to %s: %s\n", link, self, err)
			return ExitMissing, nil
		}
	}
	at, realGit, realAt := pathStanding(filepath.Dir(link))
	if o.JSON {
		fields := []out.Field{{Key: "link", Value: link}, {Key: "target", Value: self}, {Key: "action", Value: action},
			{Key: "on_path", Value: at >= 0}, {Key: "before_git", Value: at >= 0 && (realAt < 0 || at < realAt)}}
		if realGit != "" {
			fields = append(fields, out.Field{Key: "git", Value: realGit})
		}
		return 0, out.Emit(o.Stdout, fields...)
	}
	switch action {
	case "linked":
		fmt.Fprintf(o.Stdout, "Linked %s to %s.\n", link, self)
	case "replaced":
		fmt.Fprintf(o.Stdout, "Replaced %s, a link to another itos, with a link to %s.\n", link, self)
	default:
		fmt.Fprintf(o.Stdout, "%s already links to %s.\n", link, self)
	}
	fmt.Fprintln(o.Stdout, standingLine(filepath.Dir(link), at, realGit, realAt))
	return 0, nil
}

// makeLink makes link a symbolic link to self, replacing the one there when
// told to; where symbolic links need a privilege (windows), a hard link.
func makeLink(self, link string, replace bool) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if replace {
		if err := os.Remove(link); err != nil {
			return err
		}
	}
	err := os.Symlink(self, link)
	if err != nil && runtime.GOOS == "windows" {
		err = os.Link(self, link)
	}
	return err
}

// pathStanding is the place of the folder dir among the PATH's folders and
// the real git's, git.Real's, -1 when it is not there or there is none.
func pathStanding(dir string) (at int, realGit string, realAt int) {
	at, realAt = -1, -1
	realGit, _ = git.Real()
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

// standingLine says where the link's folder stands on the PATH, and what to
// do when git does not run the shim yet.
func standingLine(dir string, at int, realGit string, realAt int) string {
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

// uninstallShim removes the link to itos named git in dir, and leaves alone
// a git that is not one.
func uninstallShim(dir string, o Out) (int, error) {
	_, link, err := shimPlace(dir)
	if err != nil {
		fmt.Fprintf(o.Stderr, "itos: %s\n", err)
		return ExitMissing, nil
	}
	action := "removed"
	if _, err := os.Lstat(link); errors.Is(err, fs.ErrNotExist) {
		action = "absent"
	} else if err != nil {
		return 0, err
	} else if mine, other := linksItos(link); !mine && !other {
		return shimRefused(link, "is a git that is not a link to itos; git-shim uninstall never removes one", o)
	} else if err := os.Remove(link); err != nil {
		fmt.Fprintf(o.Stderr, "itos: cannot remove %s: %s\n", link, err)
		return ExitMissing, nil
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "link", Value: link}, out.Field{Key: "action", Value: action})
	}
	if action == "absent" {
		fmt.Fprintf(o.Stdout, "No git shim at %s: nothing to remove.\n", link)
	} else {
		fmt.Fprintf(o.Stdout, "Removed %s.\n", link)
	}
	return 0, nil
}

// shimRefused reports a git left alone, exit 1.
func shimRefused(link, why string, o Out) (int, error) {
	if o.JSON {
		return ExitPolicy, out.Emit(o.Stdout, out.Field{Key: "link", Value: link}, out.Field{Key: "action", Value: "refused"})
	}
	fmt.Fprintf(o.Stderr, "itos: %s %s; left alone\n", link, why)
	return ExitPolicy, nil
}
