package decision

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/out"
)

// refused is the exit code of a refusal (itos --help).
const refused = 1

// Run is the decision command over h: the list, a question, or a change's
// line, on stdout or as --json's one object; a refusal is its problems on
// stderr, or under --json on stdout, exit 1. An error is the command line's
// to report. A lock that cannot be given back is a line on stderr after
// everything else.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Quiet: c.Quiet, Stderr: stderr})
	if a.Release != nil {
		defer a.Release()
	}
	switch {
	case err != nil:
		return 0, err
	case len(a.Problems) > 0:
		return c.refuse(stdout, stderr, a.Problems)
	}
	q := a.Question
	switch c.Sub {
	case "add":
		about := ""
		if q.Item != "" {
			about = " about " + q.Item
		}
		return c.report(stdout, fmt.Sprintf("%s asked%s: %s", q.ID, about, committed(a.Commit, a.Header)), q, a.Commit)
	case "answer":
		if c.Provisional {
			return c.report(stdout, fmt.Sprintf("%s answered provisionally by %s: %s", q.ID, q.By, committed(a.Commit, a.Header)), q, a.Commit)
		}
		return c.report(stdout, fmt.Sprintf("%s answered: %s", q.ID, committed(a.Commit, a.Header)), q, a.Commit)
	case "ratify":
		return c.report(stdout, fmt.Sprintf("%s ratified: %s", q.ID, committed(a.Commit, a.Header)), q, a.Commit)
	case "overturn":
		return c.report(stdout, fmt.Sprintf("%s overturned: %s", q.ID, committed(a.Commit, a.Header)), q, a.Commit)
	case "record":
		if c.None {
			return c.reportRecord(stdout, fmt.Sprintf("%s marked as recorded nowhere: %s", q.ID, committed(a.Commit, a.Header)), q, "", a.Commit)
		}
		return c.reportRecord(stdout, fmt.Sprintf("%s recorded as decision %d, %s: %s", q.ID, a.Number, filepath.ToSlash(a.Record), committed(a.Commit, a.Header)),
			q, a.Record, a.Commit)
	case "show":
		return 0, c.show(stdout, q)
	}
	return 0, c.list(stdout, a)
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

// decisionOf is the question's decision as the list and show say it: "decision
// <n>", "no decision (--none)", or "" while it is neither.
func decisionOf(q ask.Question) string {
	switch q.Decision {
	case "":
		return ""
	case ask.None:
		return "no decision (--none)"
	}
	return "decision " + q.Decision
}

// settledOf is how the question's answer stands with the person, as the
// list and show say it: "provisional, by <who>" while it awaits them,
// "ratified, by <who>" or "overturned, by the person" once settled, "" for
// the person's own answer.
func settledOf(q ask.Question) string {
	switch {
	case q.Provisional:
		return "provisional, by " + q.By
	case q.Overturned != "":
		return "overturned, by the person"
	case q.By != "":
		return "ratified, by " + q.By
	}
	return ""
}

// noteOf is the line under a listed answer: a provisional one's class and
// who gave it, else its decision as decisionOf says it.
func noteOf(q ask.Question) string {
	if !q.Provisional {
		return decisionOf(q)
	}
	if q.Class == "" {
		return "by " + q.By
	}
	return q.Class + ", by " + q.By
}

// committed is how a change's text output ends: the commit made, or why
// there is none.
func committed(sha, header string) string {
	if sha == "" {
		return "the questions are the stealth config's, so nothing is committed"
	}
	return "committed " + sha[:min(7, len(sha))] + " " + header
}

// report prints add's or answer's result: the line, unless -q; under
// --json, ok, the question as it is now and the commit (null when none).
func (c CLI) report(stdout io.Writer, line string, q ask.Question, sha string) (int, error) {
	if c.JSON {
		var commit any
		if sha != "" {
			commit = sha
		}
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: ask.EntryOf(q)},
			out.Field{Key: "commit", Value: commit})
	}
	if !c.Quiet {
		fmt.Fprintln(stdout, line)
	}
	return 0, nil
}

// reportRecord prints record's result: the line, unless -q; under --json,
// ok, the question as it is now, the record written (null for --none) and
// the commit (null when none).
func (c CLI) reportRecord(stdout io.Writer, line string, q ask.Question, record, sha string) (int, error) {
	if c.JSON {
		var commit, file any
		if sha != "" {
			commit = sha
		}
		if record != "" {
			file = filepath.ToSlash(record)
		}
		return 0, out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: ask.EntryOf(q)},
			out.Field{Key: "record", Value: file}, out.Field{Key: "commit", Value: commit})
	}
	if !c.Quiet {
		fmt.Fprintln(stdout, line)
	}
	return 0, nil
}

// indented is text with every line after its first indented by n spaces.
func indented(text string, n int) string {
	return strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\n"+strings.Repeat(" ", n))
}

// show prints the question, its status, the item it names, its class and
// how its answer stands with the person, then its answer when it has one,
// and the agent's answer the person overturned.
func (c CLI) show(stdout io.Writer, q ask.Question) error {
	if c.JSON {
		return out.Emit(stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: ask.EntryOf(q)})
	}
	state := q.Status()
	if q.Item != "" {
		state += ", about " + q.Item
	}
	if q.Class != "" {
		state += ", class " + q.Class
	}
	if s := settledOf(q); s != "" {
		state += ", " + s
	}
	if d := decisionOf(q); d != "" {
		state += ", " + d
	}
	fmt.Fprintf(stdout, "%s: %s\n%s\n", q.ID, indented(q.Question, len(q.ID)+2), state)
	if q.Answer != "" {
		fmt.Fprintf(stdout, "\n%s\n", strings.TrimRight(q.Answer, "\n"))
	}
	if q.Overturned != "" {
		fmt.Fprintf(stdout, "\noverturned, by %s:\n%s\n", q.By, strings.TrimRight(q.Overturned, "\n"))
	}
	return nil
}

// list prints the open questions, the provisional answers awaiting the
// person, each with its class and who gave it, and under --all the
// answered ones after them, each with its answer, then the nudge naming
// the answered questions recorded nowhere. --json gives the open and
// answered ones as questions, and the provisional ones apart.
func (c CLI) list(stdout io.Writer, a Answer) error {
	open, answered, provisional, unrecorded := a.Open, a.Answered, a.Provisional, a.Unrecorded
	if c.JSON {
		entries, awaiting := []ask.Entry{}, []ask.Entry{}
		for _, q := range append(open, answered...) {
			entries = append(entries, ask.EntryOf(q))
		}
		for _, q := range provisional {
			awaiting = append(awaiting, ask.EntryOf(q))
		}
		return out.Emit(stdout, out.Field{Key: "questions", Value: entries}, out.Field{Key: "provisional", Value: awaiting})
	}
	// The nudge: one line naming the answered questions recorded nowhere,
	// after a blank line when a list is above it.
	nudge := func() {
		if len(unrecorded) == 0 {
			return
		}
		fmt.Fprintf(stdout, "\nAnswered and recorded nowhere: %s (--title <title>, or --none)\n", strings.Join(unrecorded, ", "))
	}
	if len(open) == 0 && len(answered) == 0 && len(provisional) == 0 {
		if c.All {
			fmt.Fprintln(stdout, "No questions.")
		} else {
			fmt.Fprintln(stdout, "No open questions.")
		}
		nudge()
		return nil
	}
	width := 0
	for _, q := range slices.Concat(open, provisional, answered) {
		width = max(width, len(q.ID))
	}
	list := func(heading string, qs []ask.Question) {
		fmt.Fprintln(stdout, heading)
		for _, q := range qs {
			about := ""
			if q.Item != "" {
				about = "  (" + q.Item + ")"
			}
			fmt.Fprintf(stdout, "  %-*s  %s%s\n", width, q.ID, oneLine(q.Question), about)
			if q.Answer != "" {
				fmt.Fprintf(stdout, "  %*s  → %s\n", width, "", oneLine(q.Answer))
			}
			if n := noteOf(q); n != "" {
				fmt.Fprintf(stdout, "  %*s    %s\n", width, "", n)
			}
		}
	}
	if len(open) > 0 {
		list("Open questions:", open)
	} else {
		fmt.Fprintln(stdout, "No open questions.")
	}
	if len(provisional) > 0 {
		fmt.Fprintln(stdout)
		list("Provisional answers, awaiting the person (itos decision ratify <id>, or overturn <id> <text>):", provisional)
	}
	if len(answered) > 0 {
		fmt.Fprintln(stdout)
		list("Answered questions:", answered)
	}
	nudge()
	return nil
}
