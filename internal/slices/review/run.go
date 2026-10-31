package review

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
)

// Run is review post or review status over h: a refusal's problems on
// stderr, each message and its fix, exit by the problem's code, or under
// --json one object with ok false and the problems; else the line saying what
// was posted, or that the verdict stands already, nothing under -q, or under
// --json the one object. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	if c.Status {
		return c.runStatus(ctx, stdout, stderr, h)
	}
	a, err := h.Handle(ctx, c.Query)
	if err != nil {
		return 0, err
	}
	if len(a.Problems) > 0 {
		return c.refused(stdout, stderr, a.Code, a.Problems)
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

// refused reports the problems that refused the command, each message and
// its fix on stderr, or under --json one object with ok false and the
// problems, and exits by code.
func (c CLI) refused(stdout, stderr io.Writer, code int, problems []out.Problem) (int, error) {
	if c.JSON {
		return code, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: problems})
	}
	for _, p := range problems {
		fmt.Fprintln(stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
		}
	}
	return code, nil
}

// pullRequest is --json's pull_request, its keys in this order.
type pullRequest struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}

// runStatus is review status over h: a refusal as refused reports it; else
// the line saying what was posted, nothing under -q, or under --json the one
// object, missing and refused [] for none. Exit 0 whenever a status was
// posted, whatever its state.
func (c CLI) runStatus(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	s, err := h.Status(ctx, c.Item)
	if err != nil {
		return 0, err
	}
	if len(s.Problems) > 0 {
		return c.refused(stdout, stderr, s.Code, s.Problems)
	}
	if c.JSON {
		missing := append([]string{}, s.Missing...)
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "outcome", Value: outcomes[s.State]},
			out.Field{Key: "item", Value: s.Item}, out.Field{Key: "state", Value: s.State}, out.Field{Key: "head", Value: s.Head},
			out.Field{Key: "pull_request", Value: pullRequest{s.Address, s.Number}}, out.Field{Key: "description", Value: s.Description},
			out.Field{Key: "missing", Value: missing}, out.Field{Key: "refused", Value: refusedLogins(s.Refused)})
	}
	if c.Quiet {
		return 0, nil
	}
	said := "reviewed by " + strings.Join(s.Logins, ", ") + "."
	switch s.State {
	case statePending:
		said = "waiting on " + strings.Join(s.Missing, ", ") + "."
	case stateFailure:
		each := make([]string, 0, len(s.Refused))
		for _, f := range s.Refused {
			each = append(each, fmt.Sprintf("%s refused to review: \"%s\"", f.Login, f.Line))
		}
		said = strings.Join(each, "; ") + "."
	}
	fmt.Fprintf(stdout, "Posted %s %s on %s of %s: %s\n", statusContext, s.State, short(s.Head), s.Address, said)
	return 0, nil
}
