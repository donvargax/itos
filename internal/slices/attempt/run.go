package attempt

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// refusedExit is the exit code of a result refused.
const refusedExit = 1

// Run is the attempt command over h: one line saying the result taken in,
// or as --json's one object what was taken; a refusal is its problems,
// exit 1, on stderr with their rules and fixes, or under --json as {"ok":
// false, "problems": […]} on stdout.
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
			fmt.Fprintf(stderr, "%s: %s\n", p.Rule, p.Message)
			if p.Fix != "" {
				fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
			}
		}
		return refusedExit, nil
	}
	t := a.Taken
	outcome := accepted
	if t.Already {
		outcome = "already"
	}
	if c.JSON {
		return 0, out.Emit(stdout,
			out.Field{Key: "ok", Value: true},
			out.Field{Key: "outcome", Value: outcome},
			out.Field{Key: "item", Value: t.Item},
			out.Field{Key: "role", Value: t.Role},
			out.Field{Key: "attempt", Value: t.Attempt},
			out.Field{Key: "request", Value: t.Request},
			out.Field{Key: "base", Value: t.Base},
			out.Field{Key: "head", Value: t.Head},
		)
	}
	line := fmt.Sprintf("accepted %s's %s attempt %d, %s", t.Item, t.Role, t.Attempt, t.Outcome)
	if t.Head != nil {
		line += " at " + *t.Head
	}
	if t.Already {
		line = "already " + line
	}
	_, err = fmt.Fprintln(stdout, line)
	return 0, err
}
