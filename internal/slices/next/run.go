package next

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
)

// refused is the policy failure's exit code (itos --help): a registry that
// is not there, or an id no item has.
const refused = 1

// Run is the next command over h: the item's line on stdout, or as --json's
// one object; a refusal's problem on stderr, or under --json on stdout,
// exit 1, and a forge that cannot be asked so, exit 3. An error is the
// command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	switch {
	case err != nil:
		return 0, err
	case a.Refused != nil:
		return c.refuse(stdout, stderr, *a.Refused, refused)
	case a.Failed != nil:
		return c.refuse(stdout, stderr, *a.Failed, forgeFailed)
	case c.JSON:
		return 0, out.Emit(stdout, a.Next.Fields()...)
	}
	fmt.Fprintln(stdout, a.Next.Line())
	return 0, nil
}

// refuse reports the problem, exit code: on stderr, its rule and message
// then its fix, or under --json as {"ok": false, "problems": [p]} on
// stdout.
func (c CLI) refuse(stdout, stderr io.Writer, p out.Problem, code int) (int, error) {
	if c.JSON {
		return code, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: []out.Problem{p}})
	}
	fmt.Fprintf(stderr, "%s: %s\n", p.Rule, p.Message)
	if p.Fix != "" {
		fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
	}
	return code, nil
}

// Line is the item's line: "<id>: <state>; next: <actor> <action>", then
// "; run: <command>" when the step is a deterministic itos command.
func (n Next) Line() string {
	line := fmt.Sprintf("%s: %s; next: %s %s", n.Item, n.State, n.Actor, n.Action)
	if n.Argv != nil {
		line += "; run: " + strings.Join(n.Argv, " ")
	}
	return line
}

// step is the next step as --json prints it, its argv only when the step
// is a deterministic itos command.
type step struct {
	Actor  string   `json:"actor"`
	Action string   `json:"action"`
	Argv   []string `json:"argv,omitempty"`
}

// observed is what the forge and the journal said, as --json prints it:
// the pull request (null for none), the verdict (null for none), the
// checks, the open threads, the journal's latest implementer record (null
// for none), and the reviewers not requested for the head ([] for none).
type observed struct {
	Pull        *Pull    `json:"pr"`
	Verdict     *string  `json:"verdict"`
	Checks      string   `json:"checks"`
	ThreadsOpen int      `json:"threads_open"`
	Attempt     *Record  `json:"attempt"`
	Unrequested []string `json:"unrequested"`
}

// Fields are the item as --json prints it, after "schema": ok, item, kind
// (null for none), state, owner, next (actor, action, argv when there is
// one), waits_on and blocked_reason when the state has them, observed when
// the forge was asked, and revision.
func (n Next) Fields() []out.Field {
	var kind any
	if n.Kind != "" {
		kind = n.Kind
	}
	fields := []out.Field{
		{Key: "ok", Value: true},
		{Key: "item", Value: n.Item},
		{Key: "kind", Value: kind},
		{Key: "state", Value: n.State},
		{Key: "owner", Value: n.Owner},
		{Key: "next", Value: step{Actor: n.Actor, Action: n.Action, Argv: n.Argv}},
	}
	if n.WaitsOn != nil {
		fields = append(fields, out.Field{Key: "waits_on", Value: n.WaitsOn})
	}
	if n.BlockedReason != "" {
		fields = append(fields, out.Field{Key: "blocked_reason", Value: n.BlockedReason})
	}
	if o := n.Observed; o != nil {
		seen := observed{Pull: o.Pull, Checks: o.Checks, ThreadsOpen: o.ThreadsOpen, Unrequested: o.Unrequested}
		if o.Verdict != "" {
			seen.Verdict = &o.Verdict
		}
		if at := o.latest("implementer"); at >= 0 {
			seen.Attempt = &o.Records[at]
		}
		fields = append(fields, out.Field{Key: "observed", Value: seen})
	}
	return append(fields, out.Field{Key: "revision", Value: n.Revision})
}
