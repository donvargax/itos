package gocmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/guides"
	"github.com/donvargax/itos/v8/internal/engine/status"
	"github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Ports are what the go command reaches the world through, which
// internal/cli binds: the repository, which locates the config and the
// notes; Trees, the trees itos reads its data from, and Worktree, the
// working tree, where the config and its data are read; and Standing,
// where things stand as itos status computes it for the person the
// identity provider names, the registry's problems when it is not sound.
type Ports interface {
	vcsport.VCS
	Trees(ctx context.Context) treeport.Trees
	Worktree(ctx context.Context) treeport.Source
	Standing(ctx context.Context, cfg *config.Loaded) (*status.Standing, []out.Problem, error)
}

// Handler is the go command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how its answer is printed: Stderr, where
// what cannot be read is a warning and a status left out is a line.
type Out struct {
	Stderr io.Writer
}

// Answer is what Handle found: the coordinator's Guide with the notes
// after it, and Standing, where things stand, nil when it is left out.
type Answer struct {
	Guide    guides.Guide
	Standing *status.Standing
}

// Handle answers the go command: where things stand first, then the
// coordinator's guide, the repository's own notes and this clone's own,
// in the order their warnings are said.
func (h Handler) Handle(ctx context.Context, o Out) Answer {
	c := h.config(ctx)
	st := h.standing(ctx, c, o.Stderr)
	return Answer{Guide: guides.Compose(ctx, h.ports, c, guide.Coordinate, true, o.Stderr), Standing: st}
}

// standing is where things stand, nil when it cannot be computed at all
// (no config, a registry that is not there or not sound), which is one
// line on stderr, never a failure of the guide.
func (h Handler) standing(ctx context.Context, c guides.Config, stderr io.Writer) *status.Standing {
	skip := func(why string) *status.Standing {
		fmt.Fprintf(stderr, "itos: where things stand is not printed: %s\n", why)
		return nil
	}
	switch {
	case c.Err != nil:
		return skip(strings.ReplaceAll(strings.TrimSpace(c.Err.Error()), "\n", "; "))
	case c.Loaded == nil:
		return skip("there is no itos config here (itos init makes one)")
	}
	st, found, err := h.ports.Standing(ctx, c.Loaded)
	switch {
	case err != nil:
		return skip(err.Error())
	case len(found) > 0:
		return skip(found[0].Message + " (itos work check)")
	}
	return st
}

// config is the config at its path as the working tree has it, read from
// the working tree; Loaded and Err are nil when it is not there.
func (h Handler) config(ctx context.Context) guides.Config {
	src := h.ports.Worktree(ctx)
	file := config.Path(h.ports, src)
	if _, err := os.Stat(file); err != nil {
		return guides.Config{File: file}
	}
	cfg, err := config.Load(h.ports, h.ports.Trees(ctx), src, file)
	return guides.Config{File: file, Loaded: cfg, Err: err}
}
