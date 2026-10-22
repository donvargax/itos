package verify

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/out"
)

// The exit codes verify gives itself (itos --help): a commit or a range
// check that fails, and a since that is no commit here.
const (
	refused  = 1
	unusable = 2
)

// Run is `verify <from> <to>` over h: 0 when every commit of the range
// passes and its range checks hold, else 1; 2 when commits.since or a
// footer's since is no commit here, each problem a FAIL line on stderr, or
// under --json the range and the problems on stdout. Under --json the
// judge's logs go to stderr and stdout holds one object: the range,
// commits.since when the config names one, each commit's result, the
// count and the range checks. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, judge.Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Tests: c.Tests})
	if err != nil {
		return 0, err
	}
	if len(a.Problems) > 0 {
		if c.JSON {
			return unusable, out.Emit(stdout,
				out.Field{Key: "range", Value: a.Range},
				out.Field{Key: "problems", Value: a.Problems})
		}
		printSinceIssues(a.Problems, stderr)
		return unusable, nil
	}
	run := a.Run
	if c.JSON {
		fields := []out.Field{{Key: "range", Value: a.Range}}
		if run.Since != "" {
			fields = append(fields, out.Field{Key: "since", Value: run.Since})
		}
		fields = append(fields,
			out.Field{Key: "commits", Value: run.Results},
			out.Field{Key: "passed", Value: run.Passed},
			out.Field{Key: "total", Value: len(run.Results)},
			out.Field{Key: "range_checks", Value: run.Ranged})
		if err := out.Emit(stdout, fields...); err != nil {
			return 0, err
		}
	}
	if !run.OK() {
		return refused, nil
	}
	return 0, nil
}

// printSinceIssues says commits.since is no commit here, one FAIL line a
// problem.
func printSinceIssues(missing []out.Problem, stderr io.Writer) {
	for _, p := range missing {
		fmt.Fprintf(stderr, "FAIL %s\n", p.Message)
	}
}
