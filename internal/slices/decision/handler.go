package decision

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/donvargax/itos/v8/internal/adr"
	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// Query is what the decision command asks: Sub, "" for the list, or add,
// answer, ratify, overturn, record or show; the list's All, under --all;
// the question's ID; add's, answer's and overturn's Text; add's Item and
// Class; answer's Provisional and By, under --provisional --by; record's
// Title, Options, Consequences and Supersedes, or None, under --none.
type Query struct {
	Sub          string
	All          bool
	ID           string
	Text         string
	Item         string
	Class        string
	Provisional  bool
	By           string
	Title        string
	Options      []string
	Consequences string
	Supersedes   int
	None         bool
}

// Ports are what the decision command reaches the world through, which
// internal/cli binds: Config, the config every command finds; Tracker, the
// tracker store the config's documents are in, the questions and the
// decision records, committed through git's hooks unless the config is the
// stealth one; and Now, the clock a record is dated by.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
	Now(ctx context.Context) time.Time
}

// Handler is the decision command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how a change is reported: Quiet, which
// silences the commit's words, and Stderr, where git's words and the
// hooks' go, and the line of a lock that cannot be given back.
type Out struct {
	Quiet  bool
	Stderr io.Writer
}

// Answer is what Handle found or did: Release, nil until the registry's
// lock is held, then for the caller to defer; the Problems that refused
// it, nothing written; the list's Open and Answered questions, its
// Provisional answers awaiting the person, and the Unrecorded questions,
// each as itos decision record would record it; the
// Question shown, or as a change left it; the change's Commit, "" under a
// stealth config, with its Header; and record's Number and Record, the
// file it wrote.
type Answer struct {
	Release                     func()
	Problems                    []out.Problem
	Open, Answered, Provisional []ask.Question
	Unrecorded                  []string
	Question                    ask.Question
	Commit, Header              string
	Number                      int
	Record                      string
}

// Handle answers q.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch q.Sub {
	case "add":
		return h.add(ctx, q, o)
	case "answer":
		return h.answer(ctx, q, o)
	case "ratify", "overturn":
		return h.settle(ctx, q, o)
	case "record":
		return h.record(ctx, q, o)
	case "show":
		return h.show(ctx, q.ID)
	}
	return h.list(ctx, q.All)
}

// held is the config for a subcommand that writes the questions, and its
// tracker: under a stealth config with the registry's lock taken (the
// tracker's Hold), given back by the Answer's Release, so the questions are
// read and written as every stealth writer reads and writes the registry
// and the ledger, one at a time (bug 16). A project's file takes none: its
// writes are commits.
func (h Handler) held(ctx context.Context, o Out) (*config.Loaded, trackerport.Tracker, Answer, error) {
	a := Answer{Release: func() {}}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return nil, nil, a, err
	}
	t := h.ports.Tracker(ctx, cfg)
	if a.Release, err = hold(ctx, t, o.Stderr); err != nil {
		return nil, nil, a, err
	}
	return cfg, t, a, nil
}

// hold is the tracker's Hold, taken for a command that reads and writes its
// documents: release, never nil and deferred by the caller, gives it back,
// saying on stderr when it cannot.
func hold(ctx context.Context, t trackerport.Tracker, stderr io.Writer) (release func(), err error) {
	release = func() {}
	held, err := t.Hold(ctx)
	if err != nil {
		return release, err
	}
	return func() {
		if err := held.Release(ctx); err != nil {
			fmt.Fprintf(stderr, "itos: the lock cannot be given back: %s\n", err)
		}
	}, nil
}

// asks is the config and the questions, for a subcommand that reads.
func (h Handler) asks(ctx context.Context) (ask.File, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return ask.File{}, err
	}
	f, _, _, err := ask.Load(ctx, h.ports.Tracker(ctx, cfg), cfg.Work.Asks)
	return f, err
}

// write is the record engine's Write (record.Write) through the tracker:
// the files written and, in a project, committed alone, through the hooks,
// git's words and the hooks' on stderr. Its result is the new commit's
// SHA, "" under a stealth config, which commits nothing. A file with
// changes no commit holds, or a commit refused, is a problem, every file
// as it was; the rest are errors.
func write(ctx context.Context, t trackerport.Tracker, cfg *config.Loaded, files []record.File, header, body string, o Out) (string, []out.Problem, error) {
	return record.Write(ctx, t, cfg, files, record.Commit{
		Header: header,
		Body:   body,
		Quiet:  o.Quiet,
		// git's words and the hooks' go to stderr: stdout is the command's
		// own result.
		Stdin: os.Stdin,
		Words: o.Stderr,
		Env:   append(message.WithoutFooters(os.Environ()), message.AmendEnv+"=0"),
	})
}

// noQuestion refuses an id no question has.
func noQuestion(id string) []out.Problem {
	return []out.Problem{{
		Rule:    "ask-no-question",
		Message: fmt.Sprintf("there is no question %s", id),
		Fix:     "itos decision --all lists the questions",
	}}
}

// itemProblem is the problem of an item the registry does not have, or of no
// registry where itos looks; nil when the item is there.
func itemProblem(ctx context.Context, cfg *config.Loaded, item string) (*out.Problem, error) {
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		p := work.Missing(file, false)
		return &p, nil
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return nil, err
	}
	for _, it := range registry.Items {
		if value.String(it.At("id")) == item {
			return nil, nil
		}
	}
	return &out.Problem{
		Rule:    "ask-unknown-item",
		Message: fmt.Sprintf("no item %s in %s, so no question can name it", item, file),
		Fix:     "itos work list --all names every item; --item may be left out",
	}, nil
}

// add is `decision add <text>… [--item <id>] [--class <class>]`: a
// question, open, with the next free id, naming the item when --item gives
// one, which the registry must have, of the class --class gives; the file
// committed alone, "docs: ask q-<n>".
func (h Handler) add(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, t, a, err := h.held(ctx, o)
	if err != nil {
		return a, err
	}
	if q.Item != "" {
		problem, err := itemProblem(ctx, cfg, q.Item)
		if err != nil {
			return a, err
		}
		if problem != nil {
			a.Problems = []out.Problem{*problem}
			return a, nil
		}
	}
	f, old, there, err := ask.Load(ctx, t, cfg.Work.Asks)
	if err != nil {
		return a, err
	}
	question := *f.Add(q.Text, q.Item, q.Class)
	about := ""
	if q.Item != "" {
		about = " about " + q.Item
	}
	class := ""
	if q.Class != "" {
		class = ", of the class " + q.Class
	}
	header := "docs: ask " + question.ID
	body := fmt.Sprintf("Ask %s%s (\"%s\")%s, with itos decision add.", question.ID, about, oneLine(q.Text), class)
	file := record.File{Path: cfg.Work.Asks, Old: old, Text: ask.Text(f), Created: !there}
	return h.committed(ctx, t, cfg, a, []record.File{file}, header, body, question, o)
}

// committed is a's change written and committed (write), the question as
// it left it; a refusal its problems.
func (h Handler) committed(ctx context.Context, t trackerport.Tracker, cfg *config.Loaded, a Answer, files []record.File, header, body string,
	question ask.Question, o Out,
) (Answer, error) {
	sha, found, err := write(ctx, t, cfg, files, header, body, o)
	if err != nil {
		return a, err
	}
	if len(found) > 0 {
		a.Problems = found
		return a, nil
	}
	a.Question, a.Commit, a.Header = question, sha, header
	return a, nil
}

// answer is `decision answer <id> <text>… [--provisional --by <handle>]`:
// the answer kept beside the question, which is then answered; the file
// committed alone, "docs: answer q-<n>". A question already answered is
// refused, provisionally or not. A provisional answer (slice 205) is an
// answering agent's, kept with who gave it, which the person ratifies or
// overturns; it is refused for a question no agent may settle: one with no
// class, or of a class work.delegated does not list.
func (h Handler) answer(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, t, a, err := h.held(ctx, o)
	if err != nil {
		return a, err
	}
	f, old, _, err := ask.Load(ctx, t, cfg.Work.Asks)
	if err != nil {
		return a, err
	}
	question := f.Find(q.ID)
	if question == nil {
		a.Problems = noQuestion(q.ID)
		return a, nil
	}
	if question.Answer != "" {
		a.Problems = []out.Problem{answered(*question)}
		return a, nil
	}
	if q.Provisional {
		if p := notDelegated(cfg, *question); p != nil {
			a.Problems = []out.Problem{*p}
			return a, nil
		}
		question.Provisional, question.By = true, q.By
	}
	question.Answer = q.Text
	header := "docs: answer " + q.ID
	body := fmt.Sprintf("Answer %s (\"%s\") with \"%s\", by itos decision answer.", q.ID, oneLine(question.Question), oneLine(q.Text))
	if q.Provisional {
		body = fmt.Sprintf("Answer %s (\"%s\", of the class %s) with \"%s\", provisionally, by %s, through itos decision answer --provisional; "+
			"work proceeds on it until the person ratifies or overturns it.", q.ID, oneLine(question.Question), question.Class, oneLine(q.Text), q.By)
	}
	return h.committed(ctx, t, cfg, a, []record.File{{Path: cfg.Work.Asks, Old: old, Text: ask.Text(f)}}, header, body, *question, o)
}

// answered refuses an answer to a question answered already: its fix names
// a new question, or, for a provisional answer, ratify and overturn, which
// settle it.
func answered(q ask.Question) out.Problem {
	p := out.Problem{
		Rule:    "ask-answered",
		Message: fmt.Sprintf("%s is already answered: %s", q.ID, oneLine(q.Answer)),
		Fix:     "itos decision add asks a new question",
	}
	if q.Provisional {
		p.Message = fmt.Sprintf("%s is already answered, provisionally, by %s: %s", q.ID, q.By, oneLine(q.Answer))
		p.Fix = fmt.Sprintf("itos decision ratify %s makes that the decision, itos decision overturn %s <text> replaces it with the person's", q.ID, q.ID)
	}
	return p
}

// notDelegated refuses a provisional answer to a question no answering
// agent may settle: one with no class, which no class delegates implicitly,
// or of a class work.delegated does not list; nil when it may.
func notDelegated(cfg *config.Loaded, q ask.Question) *out.Problem {
	switch {
	case q.Class == "":
		return &out.Problem{
			Rule:    "ask-not-delegated",
			Message: fmt.Sprintf("%s has no class, so it is the person's to answer", q.ID),
			Fix:     "a question an answering agent may settle is asked with itos decision add --class <class>, a class work.delegated lists",
		}
	case !slices.Contains(cfg.Work.Delegated, q.Class):
		return &out.Problem{
			Rule:    "ask-not-delegated",
			Message: fmt.Sprintf("%s is of the class %s, which work.delegated does not list, so it is the person's to answer", q.ID, q.Class),
			Fix:     fmt.Sprintf("list %s in work.delegated for an answering agent to settle it, or leave it to the person", q.Class),
		}
	}
	return nil
}

// settle is `decision ratify <id>` and `decision overturn <id> <text>…`:
// the person's word on a provisional answer. ratify clears provisional,
// keeping the class and who answered, so the answer is the decision;
// overturn replaces the answer with the person's text, keeping the agent's
// beside it as overturned. The file is committed alone, "docs: ratify
// q-<n>" or "docs: overturn q-<n>". A question with no provisional answer
// is refused, the person's own answer included: nothing replaces what the
// person settled.
func (h Handler) settle(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, t, a, err := h.held(ctx, o)
	if err != nil {
		return a, err
	}
	f, old, _, err := ask.Load(ctx, t, cfg.Work.Asks)
	if err != nil {
		return a, err
	}
	question := f.Find(q.ID)
	switch {
	case question == nil:
		a.Problems = noQuestion(q.ID)
		return a, nil
	case !question.Provisional:
		a.Problems = []out.Problem{{
			Rule:    "ask-not-provisional",
			Message: fmt.Sprintf("%s has no provisional answer, so there is nothing to %s", q.ID, q.Sub),
			Fix:     "itos decision lists the provisional answers awaiting the person",
		}}
		return a, nil
	}
	question.Provisional = false
	header := fmt.Sprintf("docs: %s %s", q.Sub, q.ID)
	body := fmt.Sprintf("Ratify %s (\"%s\"): %s's answer \"%s\" is the decision, by itos decision ratify.",
		q.ID, oneLine(question.Question), question.By, oneLine(question.Answer))
	if q.Sub == "overturn" {
		question.Overturned, question.Answer = question.Answer, q.Text
		body = fmt.Sprintf("Overturn %s (\"%s\"): %s's answer \"%s\" is replaced by the person's, \"%s\", by itos decision overturn.",
			q.ID, oneLine(question.Question), question.By, oneLine(question.Overturned), oneLine(q.Text))
	}
	return h.committed(ctx, t, cfg, a, []record.File{{Path: cfg.Work.Asks, Old: old, Text: ask.Text(f)}}, header, body, *question, o)
}

// show is `decision show <id>`: the question, or the problem of none with
// the id.
func (h Handler) show(ctx context.Context, id string) (Answer, error) {
	f, err := h.asks(ctx)
	if err != nil {
		return Answer{}, err
	}
	q := f.Find(id)
	if q == nil {
		return Answer{Problems: noQuestion(id)}, nil
	}
	return Answer{Question: *q}, nil
}

// list is `decision [--all]`: the open questions, the provisional answers
// awaiting the person, or with all every one, the answered ones after
// them; and the answered questions recorded nowhere.
func (h Handler) list(ctx context.Context, all bool) (Answer, error) {
	f, err := h.asks(ctx)
	if err != nil {
		return Answer{}, err
	}
	var a Answer
	for _, q := range f.Questions {
		if q.Unrecorded() {
			a.Unrecorded = append(a.Unrecorded, "itos decision record "+q.ID)
		}
		switch {
		case q.Provisional:
			a.Provisional = append(a.Provisional, q)
		case q.Answer == "":
			a.Open = append(a.Open, q)
		case all:
			a.Answered = append(a.Answered, q)
		}
	}
	return a, nil
}

// record is `decision record <id> --title <title> [--option <text>]…
// [--consequences <text>] [--supersedes <n>]`, or `decision record <id>
// --none`: the answered question written as the next decision record, or
// marked as recorded nowhere.
func (h Handler) record(ctx context.Context, q Query, o Out) (Answer, error) {
	cfg, t, a, err := h.held(ctx, o)
	if err != nil {
		return a, err
	}
	f, old, _, err := ask.Load(ctx, t, cfg.Work.Asks)
	if err != nil {
		return a, err
	}
	id := q.ID
	question := f.Find(id)
	switch {
	case question == nil:
		a.Problems = noQuestion(id)
		return a, nil
	case question.Answer == "":
		a.Problems = []out.Problem{{
			Rule:    "ask-unanswered",
			Message: fmt.Sprintf("%s is not answered, so it holds no decision to record", id),
			Fix:     "itos decision answer " + id + " <text> answers it first",
		}}
		return a, nil
	case question.Provisional:
		a.Problems = []out.Problem{{
			Rule:    "ask-provisional",
			Message: fmt.Sprintf("%s's answer is provisional, %s's, so it holds no decision to record yet", id, question.By),
			Fix:     fmt.Sprintf("itos decision ratify %s makes it the decision, itos decision overturn %s <text> replaces it", id, id),
		}}
		return a, nil
	case question.Decision != "":
		a.Problems = []out.Problem{{
			Rule:    "ask-recorded",
			Message: fmt.Sprintf("%s is already recorded, as %s", id, decisionOf(*question)),
			Fix:     "itos decision show " + id + " shows it",
		}}
		return a, nil
	}
	asks := record.File{Path: cfg.Work.Asks, Old: old}
	if q.None {
		question.Decision = ask.None
		asks.Text = ask.Text(f)
		header := "docs: mark " + id + " as recorded nowhere"
		body := fmt.Sprintf("Mark %s (\"%s\") as recorded nowhere, its answer having concerned its item alone, so itos decision stops naming it. By itos decision record --none.", id, oneLine(question.Question))
		return h.committed(ctx, t, cfg, a, []record.File{asks}, header, body, *question, o)
	}
	title, supersedes := q.Title, q.Supersedes
	dir := filepath.FromSlash(cfg.Work.Decisions)
	records, err := adr.List(cfg.Source, dir)
	if err != nil {
		return a, err
	}
	names, err := t.List(ctx, dir)
	if err != nil {
		return a, err
	}
	n := adr.Next(names)
	asked := strings.TrimSpace(question.Question) + "\n\nAsked as " + id
	if question.Item != "" {
		asked += ", about " + question.Item
	}
	written := adr.Record{Number: n, File: adr.FileName(n, title)}
	written.Text = adr.Text(adr.New{Title: title, Date: h.ports.Now(ctx).Format(time.DateOnly), Context: asked + ".",
		Options: q.Options, Decision: question.Answer, Consequences: q.Consequences, Supersedes: supersedes})
	files := []record.File{asks, {Path: filepath.Join(dir, written.File), Created: true, Rule: "decision-file-uncommitted"}}
	if supersedes != 0 {
		at := -1
		for i, r := range records {
			if r.Number == supersedes {
				at = i
				break
			}
		}
		if at < 0 {
			a.Problems = []out.Problem{{
				Rule:    "ask-no-decision",
				Message: fmt.Sprintf("there is no decision record %d in %s, so none can be superseded", supersedes, dir),
				Fix:     "--supersedes takes the number of a record there",
			}}
			return a, nil
		}
		older := records[at]
		if older.Superseded() {
			a.Problems = []out.Problem{alreadySuperseded(supersedes, older)}
			return a, nil
		}
		records[at].Text = adr.SetStatus(older.Text, adr.SupersededBy(n))
		files = append(files, record.File{Path: filepath.Join(dir, older.File), Old: older.Text, Text: records[at].Text,
			Rule: "decision-file-uncommitted"})
	}
	files[1].Text = written.Text
	index := filepath.Join(dir, adr.IndexName)
	docs, err := t.Load(ctx, []string{index})
	if err != nil {
		return a, err
	}
	before := docs[0]
	files = append(files, record.File{Path: index, Old: before.Text, Text: adr.Index(before.Text, append(records, written)),
		Created: !before.There, Rule: "decision-index-uncommitted"})
	question.Decision = strconv.Itoa(n)
	files[0].Text = ask.Text(f)
	made, err := madeDirs(cfg.Folder(), dir)
	if err != nil {
		return a, err
	}
	header := fmt.Sprintf("docs: record %s as decision %d", id, n)
	// The record's path is one word, which the body's wrap leaves whole: one
	// longer than a body line, a long title's slug, would put a line over
	// the lint's limit and the commit would be refused, so the body names
	// its folder instead, the file being the commit's to show (bug 22).
	in := filepath.ToSlash(files[1].Path)
	if utf8.RuneCountInString(in+",") > record.BodyWidth {
		in = filepath.ToSlash(dir)
	}
	body := fmt.Sprintf("Record %s (\"%s\") as decision %d, %s, in %s, by itos decision record.", id, oneLine(question.Question), n, title, in)
	if supersedes != 0 {
		body += fmt.Sprintf(" It supersedes decision %d, which leaves the index.", supersedes)
	}
	a, err = h.committed(ctx, t, cfg, a, files, header, body, *question, o)
	if err != nil || len(a.Problems) > 0 {
		unmake(made)
		return a, err
	}
	a.Number, a.Record = n, files[1].Path
	return a, nil
}

// alreadySuperseded is the refusal to supersede the record again: it names
// the record that superseded it as its status does, and the fix supersedes
// that one instead; a status naming no record leaves the fix without a number.
func alreadySuperseded(n int, older adr.Record) out.Problem {
	status := older.Status()
	p := out.Problem{
		Rule:    "ask-superseded",
		Message: fmt.Sprintf("decision record %d is already %s, so it cannot be superseded again", n, status),
		Fix:     "supersede the record that replaced it instead; this one's status does not name it",
	}
	if m, ok := older.SupersededByNumber(); ok {
		p.Fix = fmt.Sprintf("--supersedes %d supersedes the record that replaced it", m)
	}
	return p
}

// madeDirs makes the folder, read from the folder base, and those above it
// that are not there, and says which it made, read from there, the deepest
// first, for unmake.
func madeDirs(base, dir string) ([]string, error) {
	var made []string
	for at := dir; at != "." && at != string(filepath.Separator) && at != ""; at = filepath.Dir(at) {
		if _, err := os.Stat(vcsport.In(base, at)); err == nil {
			break
		}
		made = append(made, vcsport.In(base, at))
	}
	return made, os.MkdirAll(vcsport.In(base, dir), 0o755)
}

// unmake removes the folders madeDirs made, when they are empty again.
func unmake(made []string) {
	for _, dir := range made {
		// A folder something else wrote into stays.
		_ = os.Remove(dir)
	}
}
