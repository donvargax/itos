package cli

// itos ask (slice 62, features/ask.feature): the questions waiting on the
// person a repository's work is for, in work.asks beside the registry
// (internal/ask). add and answer commit the file alone, as the registry's
// commands commit theirs (writeCommitted), "docs: ask q-<n>" and
// "docs: answer q-<n>"; under a stealth config the file is in the git folder
// beside the stealth registry, written under the registry's lock, which every
// stealth writer holds (bug 16), and nothing is committed. show and the list
// read, never write.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/donvargax/itos/v2/internal/ask"
	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/lock"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/value"
	"github.com/donvargax/itos/v2/internal/work"
)

// askTakes is what ask takes, as a usage error names it.
const askTakes = "it takes add, answer or show, else --all"

// askCommand is `ask [--all]`, `ask add`, `ask answer` and `ask show`.
func askCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "add":
		return askAdd(rest, o)
	case "answer":
		return askAnswer(rest, o)
	case "show":
		return askShow(rest, o)
	}
	pos, _, set, err := subArgs("ask", "", args, nil, []string{"--all"})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usage("ask has no subcommand %s: %s", pos[0], askTakes)
	}
	return askList(set["--all"], o)
}

// askEntry is a question as --json gives it.
type askEntry struct {
	ID       string `json:"id"`
	Item     string `json:"item,omitempty"`
	Question string `json:"question"`
	Status   string `json:"status"`
	Answer   string `json:"answer,omitempty"`
}

func entryOf(q ask.Question) askEntry {
	return askEntry{ID: q.ID, Item: q.Item, Question: q.Question, Status: q.Status(), Answer: q.Answer}
}

// loadAsks is the questions in the config's work.asks, the file's text and
// whether it is there: none, "" and false when it is not.
func loadAsks(cfg *config.Loaded) (ask.File, string, bool, error) {
	raw, err := os.ReadFile(cfg.Work.Asks)
	if errors.Is(err, fs.ErrNotExist) {
		return ask.File{}, "", false, nil
	}
	if err != nil {
		return ask.File{}, "", false, err
	}
	f, err := ask.Parse(cfg.Work.Asks, string(raw))
	return f, string(raw), true, err
}

// heldAsks is the config for a subcommand that writes the questions: under
// a stealth config with the registry's lock taken, given back by release,
// never nil and deferred by the caller, so the questions are read and
// written as every stealth writer reads and writes the registry and the
// ledger, one at a time (bug 16). A project's file takes none: its writes
// are commits.
func heldAsks(o Out) (cfg *config.Loaded, release func(), err error) {
	release = func() {}
	if cfg, err = config.Load(config.Path()); err != nil {
		return nil, release, err
	}
	if cfg.Stealth {
		held, err := lock.Hold(cfg.Work.Registry)
		if err != nil {
			return nil, release, err
		}
		release = func() {
			if err := held.Release(); err != nil {
				fmt.Fprintf(o.Stderr, "itos: the lock cannot be given back: %s\n", err)
			}
		}
	}
	return cfg, release, nil
}

// askID is the question id a subcommand names first, checked; the arguments
// after it are the rest.
func askID(sub string, pos []string, what string) (string, []string, error) {
	if len(pos) == 0 {
		return "", nil, usage("ask %s needs %s", sub, what)
	}
	if !ask.ValidID(pos[0]) {
		return "", nil, usage("ask %s: %q is no question id (q-<n>)", sub, pos[0])
	}
	return pos[0], pos[1:], nil
}

// noQuestion refuses an id no question has, exit 1.
func noQuestion(id string, o Out) (int, error) {
	return refuseWork([]out.Problem{{
		Rule:    "ask-no-question",
		Message: fmt.Sprintf("there is no question %s", id),
		Fix:     "itos ask --all lists the questions",
	}}, ExitPolicy, o)
}

// itemProblem is the problem of an item the registry does not have, or of no
// registry where itos looks; nil when the item is there.
func itemProblem(cfg *config.Loaded, item string) (*out.Problem, error) {
	file := cfg.Work.Registry
	if !source.Has(file) {
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
		Fix:     "itos work list names every item; --item may be left out",
	}, nil
}

// oneLine is a text on one line, its whitespace each a space, as a list and
// a commit's body give it.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }

// askCommitted is how add's and answer's text output ends: the commit made,
// or why there is none.
func askCommitted(sha, header string) string {
	if sha == "" {
		return "the questions are the stealth config's, so nothing is committed"
	}
	return "committed " + sha[:min(7, len(sha))] + " " + header
}

// reportAsk prints add's or answer's result: the line, unless -q; under
// --json, ok, the question as it is now and the commit (null when none).
func reportAsk(line string, q ask.Question, sha string, o Out) (int, error) {
	if o.JSON {
		var commit any
		if sha != "" {
			commit = sha
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: entryOf(q)},
			out.Field{Key: "commit", Value: commit})
	}
	if !o.Quiet {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// askAdd is `ask add <text>… [--item <id>]`: a question, open, with the next
// free id, naming the item when --item gives one, which the registry must
// have; the file committed alone, "docs: ask q-<n>".
func askAdd(args []string, o Out) (int, error) {
	pos, flags, _, err := subArgs("ask", "add", args, []string{"--item"}, nil)
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(strings.Join(pos, " "))
	if text == "" {
		return 0, usage("ask add needs <text>, the question")
	}
	cfg, release, err := heldAsks(o)
	defer release()
	if err != nil {
		return 0, err
	}
	item := flags["--item"]
	if item != "" {
		problem, err := itemProblem(cfg, item)
		if err != nil {
			return 0, err
		}
		if problem != nil {
			return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
		}
	}
	f, old, there, err := loadAsks(cfg)
	if err != nil {
		return 0, err
	}
	q := *f.Add(text, item)
	about := ""
	if item != "" {
		about = " about " + item
	}
	header := "docs: ask " + q.ID
	body := fmt.Sprintf("Ask %s%s (\"%s\"), with itos ask add.", q.ID, about, oneLine(text))
	file := written{path: cfg.Work.Asks, old: old, text: ask.Text(f), created: !there}
	sha, code, err := writeCommitted(cfg, []written{file}, header, body, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportAsk(fmt.Sprintf("%s asked%s: %s", q.ID, about, askCommitted(sha, header)), q, sha, o)
}

// askAnswer is `ask answer <id> <text>…`: the answer kept beside the
// question, which is then answered; the file committed alone,
// "docs: answer q-<n>". A question already answered is refused.
func askAnswer(args []string, o Out) (int, error) {
	pos, _, _, err := subArgs("ask", "answer", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := askID("answer", pos, "<id> <text>")
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	if text == "" {
		return 0, usage("ask answer needs <id> <text>, the answer")
	}
	cfg, release, err := heldAsks(o)
	defer release()
	if err != nil {
		return 0, err
	}
	f, old, _, err := loadAsks(cfg)
	if err != nil {
		return 0, err
	}
	q := f.Find(id)
	if q == nil {
		return noQuestion(id, o)
	}
	if q.Answer != "" {
		return refuseWork([]out.Problem{{
			Rule:    "ask-answered",
			Message: fmt.Sprintf("%s is already answered: %s", id, oneLine(q.Answer)),
			Fix:     "itos ask add asks a new question",
		}}, ExitPolicy, o)
	}
	q.Answer = text
	header := "docs: answer " + id
	body := fmt.Sprintf("Answer %s (\"%s\") with \"%s\", by itos ask answer.", id, oneLine(q.Question), oneLine(text))
	sha, code, err := writeCommitted(cfg, []written{{path: cfg.Work.Asks, old: old, text: ask.Text(f)}}, header, body, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportAsk(fmt.Sprintf("%s answered: %s", id, askCommitted(sha, header)), *q, sha, o)
}

// readAsks is the config and the questions, for a subcommand that reads.
func readAsks() (ask.File, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return ask.File{}, err
	}
	f, _, _, err := loadAsks(cfg)
	return f, err
}

// indented is text with every line after its first indented by n spaces.
func indented(text string, n int) string {
	return strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\n"+strings.Repeat(" ", n))
}

// askShow is `ask show <id>`: the question, its status and the item it
// names, then its answer when it has one.
func askShow(args []string, o Out) (int, error) {
	pos, _, _, err := subArgs("ask", "show", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := askID("show", pos, "<id>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("ask show takes one <id>, not %s", strings.Join(rest, " "))
	}
	f, err := readAsks()
	if err != nil {
		return 0, err
	}
	q := f.Find(id)
	if q == nil {
		return noQuestion(id, o)
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: entryOf(*q)})
	}
	state := q.Status()
	if q.Item != "" {
		state += ", about " + q.Item
	}
	fmt.Fprintf(o.Stdout, "%s: %s\n%s\n", q.ID, indented(q.Question, len(q.ID)+2), state)
	if q.Answer != "" {
		fmt.Fprintf(o.Stdout, "\n%s\n", strings.TrimRight(q.Answer, "\n"))
	}
	return 0, nil
}

// askList is `ask [--all]`: the open questions, or with --all every one, the
// answered ones after them, each with its answer.
func askList(all bool, o Out) (int, error) {
	f, err := readAsks()
	if err != nil {
		return 0, err
	}
	var open, answered []ask.Question
	for _, q := range f.Questions {
		if q.Answer == "" {
			open = append(open, q)
		} else if all {
			answered = append(answered, q)
		}
	}
	if o.JSON {
		entries := []askEntry{}
		for _, q := range append(open, answered...) {
			entries = append(entries, entryOf(q))
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "questions", Value: entries})
	}
	if len(open) == 0 && len(answered) == 0 {
		if all {
			fmt.Fprintln(o.Stdout, "No questions.")
		} else {
			fmt.Fprintln(o.Stdout, "No open questions.")
		}
		return 0, nil
	}
	width := 0
	for _, q := range append(open, answered...) {
		width = max(width, len(q.ID))
	}
	list := func(heading string, qs []ask.Question) {
		fmt.Fprintln(o.Stdout, heading)
		for _, q := range qs {
			about := ""
			if q.Item != "" {
				about = "  (" + q.Item + ")"
			}
			fmt.Fprintf(o.Stdout, "  %-*s  %s%s\n", width, q.ID, oneLine(q.Question), about)
			if q.Answer != "" {
				fmt.Fprintf(o.Stdout, "  %*s  → %s\n", width, "", oneLine(q.Answer))
			}
		}
	}
	if len(open) > 0 {
		list("Open questions:", open)
	} else {
		fmt.Fprintln(o.Stdout, "No open questions.")
	}
	if len(answered) > 0 {
		fmt.Fprintln(o.Stdout)
		list("Answered questions:", answered)
	}
	return 0, nil
}
