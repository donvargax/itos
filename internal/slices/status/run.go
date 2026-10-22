package status

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// The status command's exit codes (itos --help): a registry that is not
// there or not sound, and --as not among the people.
const (
	refused  = 1
	notAmong = 3
)

// Run is the status command over h: where things stand on stdout, or as
// --json's one object; a registry's problems on stderr, or under --json on
// stdout, exit 1; --as not among the people exit 3, said on stderr. An
// error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stderr: stderr})
	switch {
	case err != nil:
		return 0, err
	case len(a.Problems) > 0:
		return c.refuse(stdout, stderr, a.Problems)
	case a.Missing:
		return notAmong, nil
	}
	if c.JSON {
		return 0, out.Emit(stdout, a.Standing.Fields()...)
	}
	a.Standing.Print(stdout)
	return 0, nil
}

// refuse reports the problems, exit 1: on stderr, or under --json as
// {"ok": false, "problems": […]} on stdout.
func (c CLI) refuse(stdout, stderr io.Writer, found []out.Problem) (int, error) {
	if c.JSON {
		return refused, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
		}
	}
	return refused, nil
}
