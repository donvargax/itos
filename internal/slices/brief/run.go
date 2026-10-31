package brief

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// refusedExit is the exit code of a brief refused: an unknown item or role, an
// idea, an item with no spec for a role that works from one.
const refusedExit = 1

// commit is --json's spec: its commit, null when nothing is tagged yet.
type commit struct {
	Commit *string `json:"commit"`
}

// Run is the brief command over h: the filled brief on stdout, or as
// --json's one object the request; a refusal is its problems, exit 1, on
// stderr with their fixes or under --json as {"ok": false, "problems":
// […]} on stdout.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	if err != nil {
		return 0, err
	}
	if len(a.Refused) > 0 {
		if c.JSON {
			return refusedExit, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: a.Refused})
		}
		for _, p := range a.Refused {
			fmt.Fprintln(stderr, p.Message)
			if p.Fix != "" {
				fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
			}
		}
		return refusedExit, nil
	}
	b := a.Brief
	if !c.JSON {
		_, err := fmt.Fprint(stdout, b.Text)
		return 0, err
	}
	var spec commit
	if b.Spec != "" {
		spec.Commit = &b.Spec
	}
	return 0, out.Emit(stdout,
		out.Field{Key: "ok", Value: true},
		out.Field{Key: "item", Value: b.Item},
		out.Field{Key: "role", Value: b.Role},
		out.Field{Key: "request", Value: b.Request},
		out.Field{Key: "spec", Value: spec},
		out.Field{Key: "reads", Value: b.Reads},
		out.Field{Key: "omitted", Value: b.Omitted},
		out.Field{Key: "brief", Value: b.Text},
	)
}
