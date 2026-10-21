package version

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/donvargax/itos/v8/internal/out"
)

// refused is the policy failure's exit code (itos --help): a version that
// does not satisfy the config's requires, under --check.
const refused = 1

// CLI is the version command's line: whether --check holds the version to
// the config's requires, and whether it prints the one object --json asks
// for.
type CLI struct {
	Check, JSON bool
}

// Parse reads the arguments internal/cli's flag spec hands the version
// command, with the global --json.
func Parse(args []string, json bool) CLI {
	return CLI{Check: slices.Contains(args, "--check"), JSON: json}
}

// Query is what the version command asks: whether the config's requires
// must be read, which only --check needs.
type Query struct {
	Check bool
}

// Ports are what the version command reaches the world through, which
// internal/cli binds: BuildVersion, the version this binary says it is;
// Requires, the config's requires, nil when it has none or cannot be
// loaded, with the config's failure to load; Satisfies, whether version v satisfies the range r.
type Ports interface {
	BuildVersion(ctx context.Context) string
	Requires(ctx context.Context) (*string, error)
	Satisfies(ctx context.Context, v, r string) bool
}

// Handler is the version command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Found is what Handle found: the version, the config's requires (nil when
// there is none or, without --check, no config could be read) and whether
// the version satisfies it.
type Found struct {
	Version   string
	Requires  *string
	Satisfied bool
}

// Handle is the version and the config's requires. Only q.Check needs the
// config: a global itos is run outside any project first of all (issue #2),
// so without it a config that is missing or cannot be read leaves requires
// out rather than failing.
func (h Handler) Handle(ctx context.Context, q Query) (Found, error) {
	requires, err := h.ports.Requires(ctx)
	if err != nil && q.Check {
		return Found{}, err
	}
	v := h.ports.BuildVersion(ctx)
	return Found{v, requires, requires == nil || h.ports.Satisfies(ctx, v, *requires)}, nil
}

// Run prints the version, or under --json the version with the config's
// requires and whether it holds; with --check, a version that does not
// satisfy requires exits 1.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	found, err := h.Handle(ctx, Query{Check: c.Check})
	if err != nil {
		return 0, err
	}
	if c.JSON {
		fields := []out.Field{{Key: "version", Value: found.Version}}
		if found.Requires != nil {
			fields = append(fields,
				out.Field{Key: "requires", Value: *found.Requires},
				out.Field{Key: "satisfied", Value: found.Satisfied})
		}
		if err := out.Emit(stdout, fields...); err != nil {
			return 0, err
		}
	} else {
		fmt.Fprintf(stdout, "itos %s\n", found.Version)
	}
	if !c.Check || found.Satisfied {
		return 0, nil
	}
	fmt.Fprintf(stderr, "itos %s does not satisfy %s (the config's requires)\n", found.Version, *found.Requires)
	return refused, nil
}
