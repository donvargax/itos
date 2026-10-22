package hook

import (
	"context"
	"fmt"
	"io"
)

// The exit codes hook gives itself (itos --help): a commit or a push
// refused, and commits.since naming no commit here, as verify's.
const (
	policy = 1
	usage  = 2
)

// Run is hook over h, its exit the hook's: 0 when the commit or the push
// may go on, 1 when a rule refuses it, its rejection on stderr, else the
// header lint's own code or 2 as verify's; hook install's, as
// hooks.Declare's. What the rules run is written as it runs. An error is
// the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Tests: c.Tests})
	if err != nil {
		return 0, err
	}
	switch {
	case a.Rules != nil:
		a.Rules.Reject(stderr, a.Problems)
	case len(a.Lines) > 0:
		fmt.Fprintln(stderr, a.Header)
		for _, line := range a.Lines {
			fmt.Fprintf(stderr, "  - %s\n", line)
		}
	default:
		fmt.Fprint(stderr, a.Said)
	}
	return a.Code, nil
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
