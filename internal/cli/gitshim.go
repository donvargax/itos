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

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/shim/install"
)

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
			dir = typed(o.Origin, rest[i])
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

// shimPlace is install.Place, of os.Executable.
func shimPlace(dir, goos string) (self, link string, err error) {
	return install.Place(os.Executable, dir, goos)
}

// linksItos is install.LinksItos.
func linksItos(link string) (self, other bool) { return install.LinksItos(link) }

// installShim links the running itos as git in dir, unless a git that is not
// a link to itos is there, and says where dir stands on the PATH.
func installShim(dir string, o Out) (int, error) {
	self, link, err := shimPlace(dir, o.OS)
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
		if err := makeLink(self, link, action == "replaced", o.OS); err != nil {
			fmt.Fprintf(o.Stderr, "itos: cannot link %s to %s: %s\n", link, self, err)
			return ExitMissing, nil
		}
	}
	at, realGit, realAt := pathStanding(filepath.Dir(link), o.OS)
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

// makeLink is install.MakeLink.
func makeLink(self, link string, replace bool, goos string) error {
	return install.MakeLink(self, link, replace, goos)
}

// pathStanding is install.PathStanding.
func pathStanding(dir, goos string) (at int, realGit string, realAt int) {
	return install.PathStanding(dir, goos)
}

// standingLine is install.StandingLine.
func standingLine(dir string, at int, realGit string, realAt int) string {
	return install.StandingLine(dir, at, realGit, realAt)
}

// uninstallShim removes the link to itos named git in dir, and leaves alone
// a git that is not one.
func uninstallShim(dir string, o Out) (int, error) {
	_, link, err := shimPlace(dir, o.OS)
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
