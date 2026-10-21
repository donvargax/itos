package followup

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/slices/followup/domain"
)

// The exit codes followup gives itself (itos --help): a refusal, and no git
// repository to keep the threads in.
const (
	refused = 1
	missing = 3
)

// entry is a thread as followup --json lists it.
type entry struct {
	ID     string `json:"id"`
	With   string `json:"with"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Opened string `json:"opened"`
	Last   string `json:"last"`
	Notes  int    `json:"notes"`
}

// Run is followup over h: the list, or a subcommand's change, thread or
// document, on stdout or as --json's one object; a refusal is its problem
// on stderr, or under --json the problems on stdout, exit 1; no git
// repository is a line on stderr and exit 3. An error is the command
// line's to report. A lock that cannot be given back is a line on stderr
// after everything else.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	if a.Released != nil {
		defer fmt.Fprintf(stderr, "itos: the lock cannot be given back: %s\n", a.Released)
	}
	if a.Exposed {
		fmt.Fprintf(stderr, "itos: warning: %s is in the work tree and not ignored by git, so a git add can commit this private thread; "+
			"list it in .git/info/exclude or .gitignore, or write it outside the work tree\n", c.Path)
	}
	switch {
	case err != nil:
		return 0, err
	case a.NoRepository:
		fmt.Fprintln(stderr, "itos: followup keeps its threads in the git folder, and this is no git repository")
		return missing, nil
	case a.Refusal != nil:
		return c.refuse(stdout, stderr, *a.Refusal)
	}
	t := a.Thread
	switch c.Sub {
	case "add":
		return c.report(stdout, fmt.Sprintf("%s opened, with %s: %s", t.ID, t.With, t.Title), *t)
	case "note":
		return c.report(stdout, fmt.Sprintf("%s: note %d added", t.ID, len(t.Notes)), *t)
	case "close":
		return c.report(stdout, fmt.Sprintf("%s closed: %s", t.ID, t.Title), *t)
	case "show":
		return c.show(stdout, *t)
	case "doc":
		if c.Path != "-" {
			return c.report(stdout, fmt.Sprintf("%s written to %s", t.ID, c.Path), *t, out.Field{Key: "path", Value: a.Target})
		}
		if c.JSON {
			return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "thread", Value: *t}, out.Field{Key: "markdown", Value: a.Markdown})
		}
		_, err := fmt.Fprint(stdout, a.Markdown)
		return 0, err
	}
	return c.list(stdout, a.Open, a.Closed)
}

// refuse prints a refusal, exit 1.
func (c CLI) refuse(stdout, stderr io.Writer, p out.Problem) (int, error) {
	if c.JSON {
		return refused, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: []out.Problem{p}})
	}
	fmt.Fprintln(stderr, p.Message)
	if p.Fix != "" {
		fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
	}
	return refused, nil
}

// report prints a change's line, or under --json the thread as it is now,
// with the extra fields after it.
func (c CLI) report(stdout io.Writer, line string, t domain.Thread, extra ...out.Field) (int, error) {
	if c.JSON {
		fields := append([]out.Field{{Key: "ok", Value: true}, {Key: "thread", Value: t}}, extra...)
		return 0, out.Emit(stdout, fields...)
	}
	if !c.Quiet {
		fmt.Fprintln(stdout, line)
	}
	return 0, nil
}

// show prints the whole thread, every note in order.
func (c CLI) show(stdout io.Writer, t domain.Thread) (int, error) {
	if c.JSON {
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "thread", Value: t})
	}
	state := domain.Open
	if t.Status == domain.Closed {
		state = "closed " + domain.Show(t.Closed)
	}
	fmt.Fprintf(stdout, "%s: %s\nwith %s, %s\n\n", t.ID, t.Title, t.With, state)
	for _, n := range t.Notes {
		at := domain.Show(n.At)
		indent := strings.Repeat(" ", len(at)+2)
		text := strings.ReplaceAll(strings.TrimRight(n.Text, "\n"), "\n", "\n"+indent)
		fmt.Fprintf(stdout, "%s  %s\n", at, text)
	}
	if len(t.Docs) > 0 {
		fmt.Fprintf(stdout, "\nWritten out to %s\n", strings.Join(t.Docs, ", "))
	}
	return 0, nil
}

// list prints the open threads, then with --all the closed ones.
func (c CLI) list(stdout io.Writer, open, closed []domain.Thread) (int, error) {
	if c.JSON {
		entries := []entry{}
		for _, t := range append(open, closed...) {
			entries = append(entries, entry{t.ID, t.With, t.Title, t.Status, t.Opened(), t.Last(), len(t.Notes)})
		}
		return 0, out.Emit(stdout, out.Field{Key: "threads", Value: entries})
	}
	if len(open) == 0 && len(closed) == 0 {
		if c.All {
			fmt.Fprintln(stdout, "No threads.")
		} else {
			fmt.Fprintln(stdout, "No open threads.")
		}
		return 0, nil
	}
	width := 0
	for _, t := range append(open, closed...) {
		width = max(width, len(t.ID))
	}
	section := func(heading string, ts []domain.Thread) {
		fmt.Fprintln(stdout, heading)
		for _, t := range ts {
			fmt.Fprintf(stdout, "  %-*s  %s  (%s, %s)\n", width, t.ID, t.Title, t.With, domain.Show(t.Last()))
		}
	}
	if len(open) > 0 {
		section("Open threads:", open)
	} else {
		fmt.Fprintln(stdout, "No open threads.")
	}
	if len(closed) > 0 {
		fmt.Fprintln(stdout)
		section("Closed threads:", closed)
	}
	return 0, nil
}
