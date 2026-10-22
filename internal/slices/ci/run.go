package ci

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// Run is the ci command over h: plan's plan one item a line, or under
// --json its object; scope's docs_only and range's FROM for the workflow,
// or under --json in one object; run's exit the run's; watch's the
// watch's, under --json its object on stdout. An error is the command
// line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Quiet: c.Quiet})
	if err != nil {
		return a.Code, err
	}
	switch c.Sub {
	case "plan":
		if c.JSON {
			return 0, out.Emit(stdout, a.Plan.Fields(a.Range)...)
		}
		a.Plan.Print(stdout)
		return 0, nil
	case "scope":
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "docs_only", Value: a.DocsOnly})
		}
		fmt.Fprintf(stdout, "docs_only=%t\n", a.DocsOnly)
		return 0, nil
	case "range":
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "from", Value: a.From})
		}
		fmt.Fprintf(stdout, "FROM=%s\n", a.From)
		return 0, nil
	case "watch":
		return c.watched(stdout, a)
	}
	return a.Code, nil
}

// watched ends a ci watch: its code, under --json its object on stdout.
func (c CLI) watched(stdout io.Writer, a Answer) (int, error) {
	if a.Watched == nil {
		return a.Code, nil
	}
	w := *a.Watched
	if !c.JSON {
		return w.Code, nil
	}
	fields := append([]out.Field{{Key: "ok", Value: w.Code == 0}, {Key: "commit", Value: a.SHA}}, w.Fields()...)
	return w.Code, out.Emit(stdout, fields...)
}
