package commit

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// The exit codes commit gives itself (itos --help): a message, a path or a
// commit refused, and git that could not run or set the notes' rewriting.
const (
	refused = 1
	missing = 3
)

// Run is commit over h. check-message exits as the lint and the footer rules
// judge, its report written as it runs. check-paths: 0 when the path rules
// hold, 1 when not, the rejection on stderr or the problems under --json.
// footers: one line each, the commit's short SHA and the text; under --json
// each with the full SHA and the subject. A commit: one lacking a required
// footer is refused as the hook reports a rejection, exit 1, nothing
// committed; else git's own exit code, git's stdout on stderr under --json,
// whose one object says whether it committed and, when it did, HEAD. A line
// itos says instead of going on is on stderr, with its exit. An error is
// the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Quiet: c.Quiet, Tests: c.Tests})
	if err != nil {
		return 0, err
	}
	switch c.Sub {
	case "check-message":
		return a.Code, nil
	case "check-paths":
		return c.checkPaths(a, stdout, stderr)
	case "footers":
		return c.listFooters(a, stdout)
	}
	switch {
	case len(a.Problems) > 0:
		return c.refuseCommit(a, stdout, stderr)
	case a.Said != "":
		fmt.Fprint(stderr, a.Said)
		return a.Code, nil
	case c.JSON:
		fields := []out.Field{{Key: "ok", Value: a.Code == 0}}
		if a.Commit != "" {
			fields = append(fields, out.Field{Key: "commit", Value: a.Commit})
		}
		if err := out.Emit(stdout, fields...); err != nil {
			return 0, err
		}
	}
	return a.Code, nil
}

// checkPaths prints what the path rules reject, as their rejection on
// stderr or, under --json, the type, the paths and the problems on stdout.
func (c CLI) checkPaths(a Answer, stdout, stderr io.Writer) (int, error) {
	files := c.Files
	if c.JSON {
		if files == nil {
			files = []string{}
		}
		if err := out.Emit(stdout,
			out.Field{Key: "type", Value: c.Type},
			out.Field{Key: "files", Value: files},
			out.Field{Key: "ok", Value: len(a.Problems) == 0},
			out.Field{Key: "problems", Value: a.Problems}); err != nil {
			return 0, err
		}
	} else if len(a.Problems) > 0 {
		a.Rules.Reject(stderr, a.Problems)
	}
	if len(a.Problems) > 0 {
		return refused, nil
	}
	return 0, nil
}

// listFooters prints the range's footers.
func (c CLI) listFooters(a Answer, stdout io.Writer) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout,
			out.Field{Key: "footer", Value: c.Footer},
			out.Field{Key: "range", Value: span{c.From, c.To}},
			out.Field{Key: "footers", Value: a.Footers})
	}
	for _, s := range a.Footers {
		fmt.Fprintf(stdout, "%s %s\n", short(s.SHA), s.Text)
	}
	return 0, nil
}

// refuseCommit reports the footers a commit lacks, as the hook reports a
// rejection, and exits 1: no commit is made.
func (c CLI) refuseCommit(a Answer, stdout, stderr io.Writer) (int, error) {
	if c.JSON {
		return refused, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: a.Problems})
	}
	fmt.Fprintln(stderr, a.Reject)
	for _, p := range a.Problems {
		fmt.Fprintf(stderr, "  - %s\n", p.Message)
	}
	return refused, nil
}

// short is a commit's abbreviation as the TypeScript's slice(0, 7) writes it.
func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
