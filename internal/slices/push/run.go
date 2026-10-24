package push

import (
	"context"
	"fmt"
	"io"

	"github.com/donvargax/itos/v8/internal/out"
)

// The exit codes push gives itself (itos --help): a push refused, a
// missing repository, hook, remote or git, a failure of no kind, and a
// remote out of reach.
const (
	policy    = 1
	missing   = 3
	software  = 70
	temporary = 75
)

// Run is push over h. Its end goes to stdout under --json, the outcome with
// the upstream, what --tag and --ref named, the commit pushed and the CI
// run's keys; else its lines go to stdout for a success, nothing under -q,
// and to stderr for anything else. git's own output, and what itos says
// before it stops, is written as it runs. An error is the command line's
// to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Stdout: stdout, Stderr: stderr, JSON: c.JSON, Quiet: c.Quiet})
	if err != nil {
		return 0, err
	}
	if a.Outcome == "" {
		return a.Code, nil
	}
	if c.JSON {
		fields := []out.Field{{Key: "ok", Value: a.Code == 0}, {Key: "outcome", Value: a.Outcome}}
		if a.Remote != "" {
			fields = append(fields, out.Field{Key: "remote", Value: a.Remote})
		}
		if a.Branch != "" {
			fields = append(fields, out.Field{Key: "branch", Value: a.Branch})
		}
		if a.Tags != nil {
			fields = append(fields, out.Field{Key: "tags", Value: a.Tags})
		}
		if a.Refs != nil {
			fields = append(fields, out.Field{Key: "refs", Value: a.Refs})
		}
		if a.Outcome == "pushed" && a.Commit != "" {
			fields = append(fields, out.Field{Key: "commit", Value: a.Commit})
		}
		if a.PullRequest != nil {
			fields = append(fields, out.Field{Key: "pull_request", Value: a.PullRequest})
		}
		if a.Watched != nil {
			fields = append(fields, a.Watched.Fields()...)
		}
		return a.Code, out.Emit(stdout, fields...)
	}
	w := stderr
	if a.Code == 0 {
		if c.Quiet {
			return 0, nil
		}
		w = stdout
	}
	for _, l := range a.Lines {
		fmt.Fprintln(w, l)
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
