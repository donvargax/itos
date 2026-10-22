package work

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// The exit codes work gives itself (itos --help): a registry with problems,
// not there or with no such item; and --as not among the people.
const (
	refused  = 1
	notAmong = 3
)

// showWidth is the width an item's why is wrapped at.
const showWidth = 100

// shownCommit is a commit as work show --json gives it: with --patch, its
// whole message and its diff as git show prints them.
type shownCommit struct {
	SHA     string  `json:"sha"`
	Short   string  `json:"short"`
	Header  string  `json:"header"`
	Message *string `json:"message,omitempty"`
	Patch   *string `json:"patch,omitempty"`
}

// Run is work over h, for the subcommands the slice answers: the proposal,
// list, check and show. An error is the command line's to report.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query)
	if err != nil {
		return 0, err
	}
	switch c.Sub {
	case "check":
		return c.check(stdout, stderr, a)
	case "list":
		return c.list(stdout, stderr, a)
	case "show":
		return c.show(stdout, stderr, a)
	}
	return c.proposal(stdout, stderr, a)
}

// check is work check's answer: each problem's message on stderr, exit 1 on
// one; `<file>: sound` on stdout otherwise, unless -q; under --json the
// file, whether it is sound and its problems.
func (c CLI) check(stdout, stderr io.Writer, a Answer) (int, error) {
	if c.JSON {
		found := a.Found
		if found == nil {
			found = []out.Problem{}
		}
		if err := out.Emit(stdout,
			out.Field{Key: "file", Value: a.File},
			out.Field{Key: "sound", Value: len(found) == 0},
			out.Field{Key: "problems", Value: found}); err != nil {
			return 0, err
		}
	} else {
		said(stderr, a.Found)
	}
	if len(a.Found) > 0 {
		return refused, nil
	}
	if !c.JSON && !c.Quiet {
		fmt.Fprintf(stdout, "%s: sound\n", a.File)
	}
	return 0, nil
}

// proposal is what the person can start: exit 1 when the registry is not
// sound, its problems on stderr whatever --json says; the notes of who the
// session is on stderr, and exit 3 when --as is not among the people.
func (c CLI) proposal(stdout, stderr io.Writer, a Answer) (int, error) {
	if len(a.Found) > 0 {
		said(stderr, a.Found)
		return refused, nil
	}
	for _, note := range a.Notes {
		fmt.Fprintln(stderr, note)
	}
	if a.Missing {
		return notAmong, nil
	}
	if c.JSON {
		return 0, out.Emit(stdout, a.Proposal.Fields()...)
	}
	a.Proposal.Print(stdout)
	return 0, nil
}

// list is work list's items, a deferred one marked so. Plain work list
// printed every item until v4.0.0, so a caller that needs every item (the
// Claude Code plugin's titles) asks for --all. A registry that is not there
// is said on stderr whatever --json says, exit 1. Under --json each item is
// written as work --json writes it, so the two describe an item alike, and
// file says where the registry was read.
func (c CLI) list(stdout, stderr io.Writer, a Answer) (int, error) {
	if len(a.Found) > 0 {
		said(stderr, a.Found)
		return refused, nil
	}
	if c.JSON {
		items := make([]any, len(a.Items))
		for i, item := range a.Items {
			items[i] = item
		}
		return 0, out.Emit(stdout,
			out.Field{Key: "file", Value: a.File},
			out.Field{Key: "items", Value: items})
	}
	domain.PrintList(stdout, a.Items)
	return 0, nil
}

// show is work show's item, its scenarios, the questions naming it and its
// commits, with --patch git show's text of them; under --json one object.
// A refusal is its problems, exit 1. git show's text that cannot be read is
// the error, after the commits are listed.
func (c CLI) show(stdout, stderr io.Writer, a Answer) (int, error) {
	if len(a.Refused) > 0 {
		return c.refuse(stdout, stderr, a.Refused)
	}
	s := a.Shown
	if c.JSON {
		return 0, emitShown(stdout, s, c.Patch)
	}
	s.Item.Print(stdout, showWidth)
	printScenarios(stdout, s.Scenarios, c.ID)
	printQuestions(stdout, s.Questions, c.ID)
	if len(s.Commits) == 0 {
		fmt.Fprintln(stdout, "\nCommits: none yet")
		return 0, nil
	}
	fmt.Fprintln(stdout, "\nCommits, oldest first:")
	for _, commit := range s.Commits {
		fmt.Fprintf(stdout, "  %s %s\n", commit.Short, commit.Header)
	}
	if !c.Patch {
		return 0, nil
	}
	if s.ShowsErr != nil {
		return 0, s.ShowsErr
	}
	fmt.Fprintf(stdout, "\n%s", s.Shows)
	return 0, nil
}

// refuse prints the problems, exit 1: on stderr with their fixes, or under
// --json as {"ok": false, "problems": […]} on stdout.
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

// said writes each problem's message on w, a line each, without its fix.
func said(w io.Writer, found []out.Problem) {
	for _, p := range found {
		fmt.Fprintln(w, p.Message)
	}
}

// printScenarios writes the item's scenarios, a line each: its ID, its file
// and whether it is live or @wip.
func printScenarios(w io.Writer, scenarios []tests.Test, id string) {
	if len(scenarios) == 0 {
		fmt.Fprintf(w, "\nScenarios: none tagged @%s\n", id)
		return
	}
	fmt.Fprintf(w, "\nScenarios tagged @%s:\n", id)
	var width int
	for _, t := range scenarios {
		width = max(width, utf8.RuneCountInString(t.ID))
	}
	for _, t := range scenarios {
		state := "wip "
		if t.Live {
			state = "live"
		}
		fmt.Fprintf(w, "  %s%s  %s  %s\n", t.ID, strings.Repeat(" ", width-utf8.RuneCountInString(t.ID)), state, t.File)
	}
}

// printQuestions writes the questions naming the item, a line each: its id,
// whether it is open or answered and its text; nothing when there are none.
func printQuestions(w io.Writer, questions []ask.Question, id string) {
	if len(questions) == 0 {
		return
	}
	fmt.Fprintf(w, "\nQuestions naming %s:\n", id)
	var width int
	for _, q := range questions {
		width = max(width, len(q.ID))
	}
	for _, q := range questions {
		fmt.Fprintf(w, "  %-*s  %-8s  %s\n", width, q.ID, q.Status(), oneLine(q.Question))
	}
}

// emitShown prints work show --json: the item (less a registry why that
// repeats its ledger_why, as the text prints it once), the ids of the items
// depending on it, its scenarios, the questions naming it and its commits,
// with --patch each one's message and diff.
func emitShown(w io.Writer, s *Shown, patch bool) error {
	scenarios := s.Scenarios
	if scenarios == nil {
		scenarios = []tests.Test{}
	}
	asked := []ask.Entry{}
	for _, q := range s.Questions {
		asked = append(asked, ask.EntryOf(q))
	}
	listed := []shownCommit{}
	for i, c := range s.Commits {
		commit := shownCommit{SHA: c.SHA, Short: c.Short, Header: c.Header}
		if patch {
			diff := strings.TrimLeft(s.Patches[i], "\n")
			commit.Message, commit.Patch = &c.Message, &diff
		}
		listed = append(listed, commit)
	}
	fields := []out.Field{
		{Key: "ok", Value: true},
		{Key: "item", Value: s.Item.Listed()},
	}
	if s.Task {
		var why any
		if s.Item.LedgerWhy != "" {
			why = s.Item.LedgerWhy
		}
		fields = append(fields, out.Field{Key: "ledger_why", Value: why})
	}
	return out.Emit(w, append(fields,
		out.Field{Key: "depended_on_by", Value: s.Item.DependedOnBy},
		out.Field{Key: "scenarios", Value: scenarios},
		out.Field{Key: "questions", Value: asked},
		out.Field{Key: "commits", Value: listed})...)
}

// oneLine is a text on one line, its whitespace each a space, as a list and
// a commit's body give it.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

// itos-cc mutation: 42 killed, 0 survived, 0 uncovered
// end itos-cc mutation
