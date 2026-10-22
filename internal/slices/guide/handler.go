package guide

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/guides"
	guidetext "github.com/donvargax/itos/v8/internal/guide"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Query is what the guide command asks: the Name of a guide or a
// procedure.
type Query struct {
	Name string
}

// Ports are what the guide command reaches the world through, which
// internal/cli binds: the repository, which locates the config and the
// notes; Trees, the trees itos reads its data from, and Worktree, the
// working tree, where the config and its data are read.
type Ports interface {
	vcsport.VCS
	Trees(ctx context.Context) treeport.Trees
	Worktree(ctx context.Context) treeport.Source
}

// Handler is the guide command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how its answer is printed: Stderr, where
// what cannot be read is a warning.
type Out struct {
	Stderr io.Writer
}

// Answer is what Handle found: the Text printed, and Notes, the file of the
// repository's own notes appended after the coordinator's guide, "" when
// none was.
type Answer struct {
	Text, Notes string
}

// Handle answers q: a procedure as written, the commit procedure followed by
// the commit policy the config sets, or a guide, the coordinator's followed
// by the repository's own notes. The config is read for those two alone.
func (h Handler) Handle(ctx context.Context, q Query, o Out) Answer {
	if guidetext.IsProcedure(q.Name) {
		text, _ := guidetext.Text(q.Name)
		if q.Name == guidetext.Commit {
			if policy := guides.CommitPolicy(h.config(ctx), o.Stderr); policy != "" {
				text = strings.TrimRight(text, "\n") + "\n\n" + policy
			}
		}
		return Answer{Text: text}
	}
	var c guides.Config
	if q.Name == guidetext.Coordinate {
		c = h.config(ctx)
	}
	g := guides.Compose(ctx, h.ports, c, q.Name, false, o.Stderr)
	return Answer{Text: g.Text, Notes: g.Notes}
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
