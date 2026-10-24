package work

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// The exit codes work gives itself (itos --help): a registry with problems,
// not there or with no such item; --as not among the people; and a CI run
// work done could not look at.
const (
	refused  = 1
	notAmong = 3
	missing  = 3
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

// Run is work over h: the proposal, list, check, show and the writes. An
// error is the command line's to report. A lock that cannot be given back
// is a line on stderr after everything else.
func (c CLI) Run(ctx context.Context, stdout, stderr io.Writer, h Handler) (int, error) {
	a, err := h.Handle(ctx, c.Query, Out{Quiet: c.Quiet, JSON: c.JSON, Stdout: stdout, Stderr: stderr})
	if a.Release != nil {
		defer a.Release()
	}
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
	case "done":
		return c.closed(stdout, stderr, a)
	case "take", "add", "promote", "edit", "queue", "drop", "defer", "resume", "give-back":
		return c.wrote(stdout, stderr, a)
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
		return c.refuse(stdout, stderr, a.Refused, refused)
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

// refuse prints the problems and exits with code: on stderr with their
// fixes, or under --json as {"ok": false, "problems": […]} on stdout.
func (c CLI) refuse(stdout, stderr io.Writer, found []out.Problem, code int) (int, error) {
	if c.JSON {
		return code, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(stderr, "  fix: %s\n", p.Fix)
		}
	}
	return code, nil
}

// wrote is a write's answer: its refusal, a change of several ids refused
// whole naming every id refused, else the change made or the idea promoted.
func (c CLI) wrote(stdout, stderr io.Writer, a Answer) (int, error) {
	switch {
	case len(a.Each) > 0:
		return c.refuseEach(stdout, stderr, a.Each, a.Refused)
	case len(a.Refused) > 0:
		return c.refuse(stdout, stderr, a.Refused, a.Code)
	case a.Several != nil:
		return c.reportEach(stdout, a.Several, a.Commit)
	case a.Promoted != nil:
		return c.promoted(stdout, a.Promoted, a.Commit)
	}
	return c.report(stdout, a)
}

// report prints a write of one id: its line, unless -q; under --json, ok,
// the item, the fields the subcommand adds (edit's changed, queue's queue)
// and the commit (null when none).
func (c CLI) report(stdout io.Writer, a Answer) (int, error) {
	change := a.Change
	if c.JSON {
		var commit any
		if a.Commit != "" {
			commit = a.Commit
		}
		fields := []out.Field{{Key: "ok", Value: true}, {Key: "item", Value: change.Item}}
		switch c.Sub {
		case "edit":
			changed := make([]any, len(change.Changed))
			for i, key := range change.Changed {
				changed[i] = key
			}
			fields = append(fields, out.Field{Key: "changed", Value: changed})
		case "queue":
			fields = append(fields, queued(change.Queue))
		}
		return 0, out.Emit(stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if !c.Quiet {
		fmt.Fprintln(stdout, c.line(a.ID, change, a.Commit))
	}
	return 0, nil
}

// line is a write of one id's line: what became of the item and the commit
// made, or that there was nothing to change.
func (c CLI) line(id string, change *domain.Change, sha string) string {
	done := committed(sha, change.Header)
	switch c.Sub {
	case "take":
		if change.Unchanged {
			return id + " is already in progress" + forWhom(change) + "; nothing to change"
		}
		return fmt.Sprintf("%s is in progress%s: %s", id, forWhom(change), done)
	case "add":
		return fmt.Sprintf("%s is a new %s: %s", id, c.New.Kind, done)
	case "edit":
		if change.Unchanged {
			return id + " is already so; nothing to change"
		}
		return fmt.Sprintf("%s's %s changed: %s", id, strings.Join(change.Changed, ", "), done)
	case "queue":
		if change.Unchanged {
			return id + " is already " + c.where() + "; nothing to change"
		}
		return fmt.Sprintf("%s is %s: %s", id, c.where(), done)
	case "drop":
		if change.Unchanged {
			return id + " is already dropped; nothing to change"
		}
		return fmt.Sprintf("%s is dropped: %s", id, done)
	case "defer":
		return fmt.Sprintf("%s is deferred: %s", id, done)
	case "resume":
		return fmt.Sprintf("%s is resumed: %s", id, done)
	}
	return fmt.Sprintf("%s is given back, todo again%s: %s", id, forWhom(change), done)
}

// closed is work done's answer: its refusal, under --json with the watch's
// ci, run and superseded keys when its CI run refused it; else the close
// made, or the item already done, its line unless -q, under --json ok, the
// item, the watch's keys and queue_commit (null when none, and only when
// the queue held the item), then the commit (null when none).
func (c CLI) closed(stdout, stderr io.Writer, a Answer) (int, error) {
	switch {
	case a.CI != nil && c.JSON:
		found := a.Refused
		if found == nil {
			found = []out.Problem{}
		}
		fields := append([]out.Field{{Key: "ok", Value: false}, {Key: "problems", Value: found}}, a.CI.Fields()...)
		return a.Code, out.Emit(stdout, withEvidence(fields, a.Evidence)...)
	case a.Code != 0 && c.JSON:
		fields := []out.Field{{Key: "ok", Value: false}, {Key: "problems", Value: a.Refused}}
		return a.Code, out.Emit(stdout, withEvidence(fields, a.Evidence)...)
	case a.Code != 0:
		return c.refuse(stdout, stderr, a.Refused, a.Code)
	}
	d := a.Done
	if c.JSON {
		var commit any
		if a.Commit != "" {
			commit = a.Commit
		}
		fields := append([]out.Field{{Key: "ok", Value: true}, {Key: "item", Value: d.Change.Item}}, d.Fields...)
		return 0, out.Emit(stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if !c.Quiet {
		fmt.Fprintln(stdout, d.Line)
	}
	return 0, nil
}

// withEvidence is the fields with the evidence of the task checks looked up,
// when there is any.
func withEvidence(fields []out.Field, entries []evidence.Entry) []out.Field {
	if len(entries) == 0 {
		return fields
	}
	return append(fields, out.Field{Key: "evidence", Value: entries})
}

// promoted prints work promote's change: the idea, what it became and the
// items and questions naming it now, then the commit, unless -q; under
// --json, ok, the item, the id it was, the items rewritten, a task's entry
// and ledger file, the questions renamed (only when one named the idea, so
// a promote without one reports as it did) and the commit (null when none).
func (c CLI) promoted(stdout io.Writer, p *Promoted, sha string) (int, error) {
	if c.JSON {
		fields := []out.Field{{Key: "ok", Value: true}, {Key: "item", Value: p.Change.Item},
			{Key: "was", Value: p.Was}, {Key: "rewritten", Value: anys(p.Change.Rewritten)}}
		if p.Ledger != nil {
			fields = append(fields, out.Field{Key: "task", Value: p.Ledger.Task}, out.Field{Key: "file", Value: p.Ledger.Path})
		}
		if len(p.Renamed) > 0 {
			fields = append(fields, out.Field{Key: "questions", Value: anys(p.Renamed)})
		}
		var commit any
		if sha != "" {
			commit = sha
		}
		return 0, out.Emit(stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if c.Quiet {
		return 0, nil
	}
	line := fmt.Sprintf("%s is the %s %s", p.Was, p.Kind, p.ID)
	if p.Ledger != nil {
		line += " of " + p.Ledger.Path
	}
	if namedBy := append(slices.Clone(p.Change.Rewritten), p.Renamed...); len(namedBy) > 0 {
		line += ", named now by " + strings.Join(namedBy, ", ")
	}
	result := committed(sha, p.Change.Header)
	if sha == "" && p.Ledger != nil {
		result = "the ledger and the registry are the stealth config's, so nothing is committed"
	}
	fmt.Fprintln(stdout, line+": "+result)
	return 0, nil
}

// anys is the strings as a list out.Emit writes.
func anys(names []string) []any {
	list := make([]any, len(names))
	for i, n := range names {
		list[i] = n
	}
	return list
}

// forWhom is " for <owner>" of the item a change left owned, "" when it has
// no owner.
func forWhom(change *domain.Change) string {
	if owner, ok := change.Item.At("owner").(string); ok && owner != "" {
		return " for " + owner
	}
	return ""
}

// where is where work queue puts the items, as its lines say it.
func (c CLI) where() string {
	switch at := c.Place; {
	case at.Remove:
		return "out of the queue"
	case at.Before != "":
		return "before " + at.Before + " in the queue"
	case at.After != "":
		return "after " + at.After + " in the queue"
	}
	return "first in the queue"
}

// queued is work queue's --json field: the queue after.
func queued(list []string) out.Field {
	queue := make([]any, len(list))
	for i, q := range list {
		queue[i] = q
	}
	return out.Field{Key: "queue", Value: queue}
}

// reportEach prints a write of several ids: a line for each item, the last
// the commit, unless -q; under --json, ok, the items in the order given,
// work queue's queue and the commit (null when none).
func (c CLI) reportEach(stdout io.Writer, s *domain.Several, sha string) (int, error) {
	if c.JSON {
		items := make([]any, len(s.Items))
		for i, item := range s.Items {
			items[i] = item
		}
		var commit any
		if sha != "" {
			commit = sha
		}
		fields := []out.Field{{Key: "ok", Value: true}, {Key: "items", Value: items}}
		if c.Sub == "queue" {
			fields = append(fields, queued(s.Queue))
		}
		return 0, out.Emit(stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if c.Quiet {
		return 0, nil
	}
	lines := c.eachSays(s)
	if s.Unchanged {
		lines = append(lines, "nothing to change")
	} else {
		lines = append(lines, committed(sha, s.Header))
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return 0, nil
}

// eachSays is the lines of a write of several ids before its commit's:
// work queue's one for them all, else one for each item.
func (c CLI) eachSays(s *domain.Several) []string {
	switch c.Sub {
	case "queue":
		placed := c.where()
		if !c.Place.Remove {
			placed += ", in that order"
		}
		if s.Unchanged {
			return []string{and(c.IDs) + " are already " + placed}
		}
		return []string{and(c.IDs) + " are " + placed}
	case "edit":
		return eachLine(s, func(id string, ch domain.Change) string {
			return fmt.Sprintf("%s's %s changed", id, strings.Join(ch.Changed, ", "))
		}, "is already so")
	case "drop":
		return eachLine(s, itemIs("dropped"), "is already dropped")
	case "defer":
		return eachLine(s, itemIs("deferred"), "")
	case "resume":
		return eachLine(s, itemIs("resumed"), "")
	}
	return eachLine(s, itemIs("given back"), "")
}

// eachLine is a line for each item of a change of several ids: done, when
// its change wrote something, else unchanged.
func eachLine(s *domain.Several, done func(id string, c domain.Change) string, unchanged string) []string {
	lines := make([]string, len(s.Each))
	for i, c := range s.Each {
		id := value.String(c.Item.At("id"))
		lines[i] = id + " " + unchanged
		if !c.Unchanged {
			lines[i] = done(id, c)
		}
	}
	return lines
}

// itemIs is the line of an item a change of several ids changed: "<id> is
// <what>".
func itemIs(what string) func(string, domain.Change) string {
	return func(id string, _ domain.Change) string { return id + " is " + what }
}

// refuseEach reports a change of several ids refused, exit 1: a line naming
// every id refused, then each problem, as refuse reports them; under
// --json, {"ok": false, "refused": […], "problems": […]}.
func (c CLI) refuseEach(stdout, stderr io.Writer, ids []string, found []out.Problem) (int, error) {
	if c.JSON {
		listed := make([]any, len(ids))
		for i, id := range ids {
			listed[i] = id
		}
		return refused, out.Emit(stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "refused", Value: listed},
			out.Field{Key: "problems", Value: found})
	}
	fmt.Fprintf(stderr, "work %s refuses %s, so nothing is written:\n", c.Sub, and(ids))
	return c.refuse(stdout, stderr, found, refused)
}

// committed is how a registry command's text output ends: the commit it
// made, or why there is none.
func committed(sha, header string) string {
	if sha == "" {
		return "the registry is the stealth config's, so nothing is committed"
	}
	return "committed " + sha[:min(7, len(sha))] + " " + header
}

// and is the names joined as a list is said: "a", "a and b", "a, b and c".
func and(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
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
