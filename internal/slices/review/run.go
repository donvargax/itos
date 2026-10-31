package review

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// Run is review post over h: a refusal's problems on stderr, each message
// and its fix, exit by the problem's code, or under --json one object with
// ok false and the problems; else the line saying what was posted, or that
// the verdict stands already, nothing under -q, or under --json the one
// object. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	if err != nil {
		return 0, err
	}
	if len(a.Problems) > 0 {
		if c.JSON {
			return a.Code, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: a.Problems})
		}
		for _, p := range a.Problems {
			fmt.Fprintln(stderr, p.Message)
			if p.Fix != "" {
				fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
			}
		}
		return a.Code, nil
	}
	if c.JSON {
		fields := []out.Field{{Key: "ok", Value: true}, {Key: "outcome", Value: a.Outcome}, {Key: "item", Value: a.Item},
			{Key: "stage", Value: a.Stage}, {Key: "verdict", Value: a.Verdict}, {Key: "head", Value: a.Head}, {Key: "round", Value: a.Round}}
		if a.Outcome == posted {
			fields = append(fields, out.Field{Key: "comment", Value: a.Comment})
		}
		return 0, out.Emit(stdout, fields...)
	}
	if c.Quiet {
		return 0, nil
	}
	if a.Outcome == already {
		fmt.Fprintf(stdout, "The %s review of %s at %s: %s already, round %d, on %s; nothing was posted.\n",
			a.Stage, a.Item, short(a.Head), a.Verdict, a.Round, a.Address)
		return 0, nil
	}
	fmt.Fprintf(stdout, "Posted the %s review of %s at %s: %s, round %d, on %s.\n", a.Stage, a.Item, short(a.Head), a.Verdict, a.Round, a.Address)
	return 0, nil
}
