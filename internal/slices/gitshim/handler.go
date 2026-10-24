package gitshim

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The subcommands, and the two commands run runs.
const (
	install   = "install"
	uninstall = "uninstall"
	run       = "run"
	commit    = "commit"
	push      = "push"
)

// The actions install and uninstall take, as --json names them.
const (
	linked   = "linked"
	kept     = "kept"
	replaced = "replaced"
	removed  = "removed"
	absent   = "absent"
	refused  = "refused"
)

// Query is what git-shim asks: Sub, install, uninstall or run; for install
// and uninstall the folder of the link, Dir, "" for the one holding itos;
// for run the command it runs, Run (commit or push), with git's arguments,
// Args.
type Query struct {
	Sub, Dir string
	Run      string
	Args     []string
}

// Ports are what git-shim reaches the world through, which internal/cli
// binds for the OS itos runs on: Place, the running itos and the link's path
// in dir; LinksItos, whether the git at link is this itos or another itos;
// MakeLink, the link made, replacing one there when told to;
// Standing, the place of the folder dir among the PATH's folders and the
// real git and its folder's place, -1 when not there; Commit and Push, itos
// commit and itos push run with git's arguments, and their exit code.
type Ports interface {
	Place(ctx context.Context, dir string) (self, link string, err error)
	LinksItos(ctx context.Context, link string) (self, other bool)
	MakeLink(ctx context.Context, self, link string, replace bool) error
	Standing(ctx context.Context, dir string) (at int, realGit string, realAt int)
	Commit(ctx context.Context, args []string) (int, error)
	Push(ctx context.Context, args []string) (int, error)
}

// Handler is git-shim's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Answer is what Handle did: the Link and its Target, the running itos
// (install's); the Action taken; Failed, the link that cannot be placed,
// made or removed; where the link's folder stands on the PATH, At, against
// the real Git and its folder's place, GitAt (install's); and the exit Code
// of the command run runs.
type Answer struct {
	Link, Target string
	Action       string
	Failed       error
	At, GitAt    int
	Git          string
	Code         int
}

// Handle does what q asks: install, uninstall, or run's command.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	switch q.Sub {
	case install:
		return h.install(ctx, q.Dir)
	case uninstall:
		return h.uninstall(ctx, q.Dir)
	}
	command := h.ports.Push
	if q.Run == commit {
		command = h.ports.Commit
	}
	code, err := command(ctx, q.Args)
	return Answer{Code: code}, err
}

// install links the running itos as git in dir, unless a git that is not
// itos is there, and says where dir stands on the PATH.
func (h Handler) install(ctx context.Context, dir string) (Answer, error) {
	self, link, err := h.ports.Place(ctx, dir)
	if err != nil {
		return Answer{Failed: err}, nil
	}
	a := Answer{Link: link, Target: self, Action: linked}
	if _, err := os.Lstat(link); err == nil {
		mine, other := h.ports.LinksItos(ctx, link)
		switch {
		case mine:
			a.Action = kept
		case other:
			a.Action = replaced
		default:
			a.Action = refused
			return a, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Answer{}, err
	}
	if a.Action != kept {
		if err := h.ports.MakeLink(ctx, self, link, a.Action == replaced); err != nil {
			a.Failed = fmt.Errorf("cannot link %s to %s: %s", link, self, err)
			return a, nil
		}
	}
	a.At, a.Git, a.GitAt = h.ports.Standing(ctx, filepath.Dir(link))
	return a, nil
}

// uninstall removes the itos named git in dir, and leaves alone a git that
// is not itos.
func (h Handler) uninstall(ctx context.Context, dir string) (Answer, error) {
	_, link, err := h.ports.Place(ctx, dir)
	if err != nil {
		return Answer{Failed: err}, nil
	}
	a := Answer{Link: link, Action: removed}
	if _, err := os.Lstat(link); errors.Is(err, fs.ErrNotExist) {
		a.Action = absent
	} else if err != nil {
		return Answer{}, err
	} else if mine, other := h.ports.LinksItos(ctx, link); !mine && !other {
		a.Action = refused
	} else if err := os.Remove(link); err != nil {
		a.Failed = fmt.Errorf("cannot remove %s: %s", link, err)
	}
	return a, nil
}
