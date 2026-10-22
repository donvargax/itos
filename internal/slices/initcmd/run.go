package initcmd

import (
	"context"
	"io"
)

// exitPolicy is the exit code init gives itself (itos --help): something
// missing where a config is there, an offer asked for that failed, or a
// policy refused.
const exitPolicy = 1

// Run is init over h: what it writes and does, said as it goes on stdout,
// or under --json on stderr with the one object on stdout; its exit is hook
// install's, else 1 when anything is missing, an offer asked for failed or
// a policy is refused. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Tests: c.Tests})
	return a.Code, err
}
