package draft

import (
	"context"
	"fmt"
	"io"

	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
)

// The exit codes draft gives itself (itos --help): a refusal, and no git
// repository to keep the drafts in.
const (
	refused = 1
	missing = 3
)

// Run is draft over h: the list, or a subcommand's draft, copies or drafts
// promoted, on stdout or as --json's one object; promote says each draft
// promoted as it goes, its commands' and commits' words on stderr. A
// refusal is its problems on stderr, or under --json on stdout, exit 1; no
// git repository is a line on stderr and exit 3. An error is the command
// line's to report. A lock that cannot be given back is a line on stderr
// after everything else.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Progress{Stderr: stderr, Promoted: func(id, header string) {
		if !c.JSON && !c.Quiet {
			fmt.Fprintf(stdout, "%s promoted: %s\n", id, header)
		}
	}})
	if a.Released != nil {
		defer fmt.Fprintf(stderr, "itos: the lock cannot be given back: %s\n", a.Released)
	}
	switch {
	case err != nil:
		return 0, err
	case a.NoRepository:
		fmt.Fprintln(stderr, "itos: draft keeps its drafts in the git folder, and this is no git repository")
		return missing, nil
	case a.Code != 0:
		return a.Code, nil
	}
	switch c.Sub {
	case "add", "drop":
		if len(a.Problems) > 0 {
			return c.refuse(stdout, stderr, a.Problems)
		}
		return c.report(stdout, a.Draft)
	case "edit":
		if len(a.Problems) > 0 {
			return c.refuse(stdout, stderr, a.Problems)
		}
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "draft", Value: domain.EntryOf(a.Draft)},
				out.Field{Key: "copies", Value: a.Copies})
		}
		for _, w := range a.Copies {
			fmt.Fprintln(stdout, w)
		}
		return 0, nil
	case "promote":
		return c.promoted(stdout, stderr, a)
	}
	return c.list(stdout, a.Drafts)
}

// refuse prints the problems, exit 1: on stderr, or under --json as
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

// report prints the draft added or dropped: its line, unless --quiet, or
// under --json the draft.
func (c CLI) report(stdout io.Writer, d domain.Draft) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "draft", Value: domain.EntryOf(d)})
	}
	if c.Quiet {
		return 0, nil
	}
	verb := "dropped"
	if c.Sub == "add" {
		verb = "drafted"
	}
	fmt.Fprintf(stdout, "%s %s: %s\n", d.ID, verb, domain.Summary(d))
	return 0, nil
}

// promoted reports a promote, each draft promoted being said as it was:
// no drafts at all said so, unless --quiet; under --json ok, the drafts
// promoted and any problem; a refusal its problems, exit 1.
func (c CLI) promoted(stdout, stderr io.Writer, a Answer) (int, error) {
	if len(a.Promoted) == 0 && len(a.Problems) == 0 && !c.JSON && !c.Quiet {
		fmt.Fprintln(stdout, "No drafts to promote.")
	}
	if c.JSON {
		ok := len(a.Problems) == 0
		fields := []out.Field{{Key: "ok", Value: ok}, {Key: "promoted", Value: a.Promoted}}
		if !ok {
			fields = append(fields, out.Field{Key: "problems", Value: a.Problems})
		}
		if err := out.Emit(stdout, fields...); err != nil || ok {
			return 0, err
		}
		return refused, nil
	}
	if len(a.Problems) == 0 {
		return 0, nil
	}
	return c.refuse(stdout, stderr, a.Problems)
}

// list prints the drafts in the order they were added, which is the order
// promote applies them in.
func (c CLI) list(stdout io.Writer, drafts []domain.Draft) (int, error) {
	if c.JSON {
		entries := []domain.Entry{}
		for _, d := range drafts {
			entries = append(entries, domain.EntryOf(d))
		}
		return 0, out.Emit(stdout, out.Field{Key: "drafts", Value: entries})
	}
	if len(drafts) == 0 {
		fmt.Fprintln(stdout, "No drafts.")
		return 0, nil
	}
	fmt.Fprintln(stdout, "Drafts, in the order itos draft promote applies them:")
	for _, line := range domain.Lines(drafts) {
		fmt.Fprintln(stdout, line)
	}
	return 0, nil
}
