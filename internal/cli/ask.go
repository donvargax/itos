package cli

// itos question (slice 62, features/ask.feature): the questions waiting on the
// person a repository's work is for, in work.asks beside the registry
// (internal/ask). add and answer commit the file alone, as the registry's
// commands commit theirs (writeCommitted), "docs: ask q-<n>" and
// "docs: answer q-<n>"; under a stealth config the file is in the git folder
// beside the stealth registry, written under the registry's lock, which every
// stealth writer holds (bug 16), and nothing is committed. show and the list
// read, never write; the list ends naming the answered questions recorded
// nowhere, each as itos question record would record it.
//
// record (slice 69) writes an answered question as the next architecture
// decision record, in MADR 4's format since slice 71 (internal/adr), in the
// folder work.decisions names, notes the record's number on the question and
// commits the questions, the record, the one it supersedes and the folder's
// index together, "docs: record q-<n> as decision <m>"; --none notes no
// record and commits the questions alone, "docs: mark q-<n> as recorded
// nowhere". Under a stealth config the records are in the git folder too,
// decisions/ beside the stealth config (work.decisions resolved there), and
// nothing is committed.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/donvargax/itos/v5/internal/adr"
	"github.com/donvargax/itos/v5/internal/ask"
	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/lock"
	"github.com/donvargax/itos/v5/internal/out"
	"github.com/donvargax/itos/v5/internal/source"
	"github.com/donvargax/itos/v5/internal/value"
	"github.com/donvargax/itos/v5/internal/work"
)

// askTakes is what ask takes, as a usage error names it.
const askTakes = "it takes add, answer, record or show, else --all"

// askCommand is `question [--all]`, `question add`, `question answer` and `question show`.
func askCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "add":
		return askAdd(rest, o)
	case "answer":
		return askAnswer(rest, o)
	case "record":
		return askRecord(rest, o)
	case "show":
		return askShow(rest, o)
	}
	pos, _, set, err := subArgs("question", "", args, nil, []string{"--all"})
	if err != nil {
		return 0, err
	}
	if len(pos) > 0 {
		return 0, usage("question has no subcommand %s: %s", pos[0], askTakes)
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
	// Decision is the record's number, or "none".
	Decision any `json:"decision,omitempty"`
}

func entryOf(q ask.Question) askEntry {
	e := askEntry{ID: q.ID, Item: q.Item, Question: q.Question, Status: q.Status(), Answer: q.Answer}
	if n, err := strconv.Atoi(q.Decision); err == nil {
		e.Decision = n
	} else if q.Decision != "" {
		e.Decision = q.Decision
	}
	return e
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
		return "", nil, usage("question %s needs %s", sub, what)
	}
	if !ask.ValidID(pos[0]) {
		return "", nil, usage("question %s: %q is no question id (q-<n>)", sub, pos[0])
	}
	return pos[0], pos[1:], nil
}

// noQuestion refuses an id no question has, exit 1.
func noQuestion(id string, o Out) (int, error) {
	return refuseWork([]out.Problem{{
		Rule:    "ask-no-question",
		Message: fmt.Sprintf("there is no question %s", id),
		Fix:     "itos question --all lists the questions",
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
		Fix:     "itos work list --all names every item; --item may be left out",
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

// askAdd is `question add <text>… [--item <id>]`: a question, open, with the next
// free id, naming the item when --item gives one, which the registry must
// have; the file committed alone, "docs: ask q-<n>".
func askAdd(args []string, o Out) (int, error) {
	pos, flags, _, err := subArgs("question", "add", args, []string{"--item"}, nil)
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(strings.Join(pos, " "))
	if text == "" {
		return 0, usage("question add needs <text>, the question")
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
	body := fmt.Sprintf("Ask %s%s (\"%s\"), with itos question add.", q.ID, about, oneLine(text))
	file := written{path: cfg.Work.Asks, old: old, text: ask.Text(f), created: !there}
	sha, code, err := writeCommitted(cfg, []written{file}, header, body, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportAsk(fmt.Sprintf("%s asked%s: %s", q.ID, about, askCommitted(sha, header)), q, sha, o)
}

// askAnswer is `question answer <id> <text>…`: the answer kept beside the
// question, which is then answered; the file committed alone,
// "docs: answer q-<n>". A question already answered is refused.
func askAnswer(args []string, o Out) (int, error) {
	pos, _, _, err := subArgs("question", "answer", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := askID("answer", pos, "<id> <text>")
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(strings.Join(rest, " "))
	if text == "" {
		return 0, usage("question answer needs <id> <text>, the answer")
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
			Fix:     "itos question add asks a new question",
		}}, ExitPolicy, o)
	}
	q.Answer = text
	header := "docs: answer " + id
	body := fmt.Sprintf("Answer %s (\"%s\") with \"%s\", by itos question answer.", id, oneLine(q.Question), oneLine(text))
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

// askShow is `question show <id>`: the question, its status and the item it
// names, then its answer when it has one.
func askShow(args []string, o Out) (int, error) {
	pos, _, _, err := subArgs("question", "show", args, nil, nil)
	if err != nil {
		return 0, err
	}
	id, rest, err := askID("show", pos, "<id>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("question show takes one <id>, not %s", strings.Join(rest, " "))
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
	if d := decisionOf(*q); d != "" {
		state += ", " + d
	}
	fmt.Fprintf(o.Stdout, "%s: %s\n%s\n", q.ID, indented(q.Question, len(q.ID)+2), state)
	if q.Answer != "" {
		fmt.Fprintf(o.Stdout, "\n%s\n", strings.TrimRight(q.Answer, "\n"))
	}
	return 0, nil
}

// askList is `question [--all]`: the open questions, or with --all every one, the
// answered ones after them, each with its answer.
func askList(all bool, o Out) (int, error) {
	f, err := readAsks()
	if err != nil {
		return 0, err
	}
	var open, answered []ask.Question
	var unrecorded []string
	for _, q := range f.Questions {
		if q.Unrecorded() {
			unrecorded = append(unrecorded, "itos question record "+q.ID)
		}
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
	// The nudge: one line naming the answered questions recorded nowhere,
	// after a blank line when a list is above it.
	nudge := func() {
		if len(unrecorded) == 0 {
			return
		}
		fmt.Fprintf(o.Stdout, "\nAnswered and recorded nowhere: %s (--title <title>, or --none)\n", strings.Join(unrecorded, ", "))
	}
	if len(open) == 0 && len(answered) == 0 {
		if all {
			fmt.Fprintln(o.Stdout, "No questions.")
		} else {
			fmt.Fprintln(o.Stdout, "No open questions.")
		}
		nudge()
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
			if d := decisionOf(q); d != "" {
				fmt.Fprintf(o.Stdout, "  %*s    %s\n", width, "", d)
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
	nudge()
	return 0, nil
}

// askRecord is `question record <id> --title <title> [--option <text>]…
// [--consequences <text>] [--supersedes <n>]`, or `question record <id> --none`:
// the answered question written as the next decision record, or marked as
// recorded nowhere.
func askRecord(args []string, o Out) (int, error) {
	options, args, err := repeated("question record", "--option", args)
	if err != nil {
		return 0, err
	}
	pos, flags, set, err := subArgs("question", "record", args, []string{"--title", "--consequences", "--supersedes"}, []string{"--none"})
	if err != nil {
		return 0, err
	}
	id, rest, err := askID("record", pos, "<id>")
	if err != nil {
		return 0, err
	}
	if len(rest) > 0 {
		return 0, usage("question record takes one <id>, not %s", strings.Join(rest, " "))
	}
	title, consequences := oneLine(flags["--title"]), strings.TrimSpace(flags["--consequences"])
	supersedes := 0
	if text, given := flags["--supersedes"]; given {
		if supersedes, err = strconv.Atoi(text); err != nil || supersedes < 1 {
			return 0, usage("question record --supersedes takes a decision record's number, not %q", text)
		}
	}
	none := set["--none"]
	switch {
	case none && (title != "" || consequences != "" || supersedes != 0 || len(options) > 0):
		return 0, usage("question record --none writes no record, so it takes no --title, --option, --consequences or --supersedes")
	case !none && title == "":
		return 0, usage("question record needs --title <title>, the decision's, or --none")
	case !none && adr.Slug(title) == "":
		return 0, usage("question record --title needs a letter or a digit, which the record's file is named by")
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
	switch {
	case q == nil:
		return noQuestion(id, o)
	case q.Answer == "":
		return refuseWork([]out.Problem{{
			Rule:    "ask-unanswered",
			Message: fmt.Sprintf("%s is not answered, so it holds no decision to record", id),
			Fix:     "itos question answer " + id + " <text> answers it first",
		}}, ExitPolicy, o)
	case q.Decision != "":
		return refuseWork([]out.Problem{{
			Rule:    "ask-recorded",
			Message: fmt.Sprintf("%s is already recorded, as %s", id, decisionOf(*q)),
			Fix:     "itos question show " + id + " shows it",
		}}, ExitPolicy, o)
	}
	asks := written{path: cfg.Work.Asks, old: old}
	if none {
		q.Decision = ask.None
		asks.text = ask.Text(f)
		header := "docs: mark " + id + " as recorded nowhere"
		body := fmt.Sprintf("Mark %s (\"%s\") as recorded nowhere, its answer having concerned its item alone, so itos question stops naming it. By itos question record --none.", id, oneLine(q.Question))
		sha, code, err := writeCommitted(cfg, []written{asks}, header, body, o)
		if err != nil || code != 0 {
			return code, err
		}
		return reportRecord(fmt.Sprintf("%s marked as recorded nowhere: %s", id, askCommitted(sha, header)), *q, "", sha, o)
	}
	dir := filepath.FromSlash(cfg.Work.Decisions)
	records, err := adr.List(dir)
	if err != nil {
		return 0, err
	}
	n, err := adr.Next(dir)
	if err != nil {
		return 0, err
	}
	context := strings.TrimSpace(q.Question) + "\n\nAsked as " + id
	if q.Item != "" {
		context += ", about " + q.Item
	}
	record := adr.Record{Number: n, File: adr.FileName(n, title)}
	record.Text = adr.Text(adr.New{Title: title, Date: time.Now().Format(time.DateOnly), Context: context + ".",
		Options: options, Decision: q.Answer, Consequences: consequences, Supersedes: supersedes})
	files := []written{asks, {path: filepath.Join(dir, record.File), created: true, rule: "decision-file-uncommitted"}}
	if supersedes != 0 {
		at := -1
		for i, r := range records {
			if r.Number == supersedes {
				at = i
				break
			}
		}
		if at < 0 {
			return refuseWork([]out.Problem{{
				Rule:    "ask-no-decision",
				Message: fmt.Sprintf("there is no decision record %d in %s, so none can be superseded", supersedes, dir),
				Fix:     "--supersedes takes the number of a record there",
			}}, ExitPolicy, o)
		}
		older := records[at]
		records[at].Text = adr.SetStatus(older.Text, adr.SupersededBy(n))
		files = append(files, written{path: filepath.Join(dir, older.File), old: older.Text, text: records[at].Text,
			rule: "decision-file-uncommitted"})
	}
	files[1].text = record.Text
	index := filepath.Join(dir, adr.IndexName)
	before, err := os.ReadFile(index)
	there := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	files = append(files, written{path: index, old: string(before), text: adr.Index(string(before), append(records, record)),
		created: !there, rule: "decision-index-uncommitted"})
	q.Decision = strconv.Itoa(n)
	files[0].text = ask.Text(f)
	made, err := madeDirs(dir)
	if err != nil {
		return 0, err
	}
	header := fmt.Sprintf("docs: record %s as decision %d", id, n)
	// The record's path is one word, which the body's wrap leaves whole: one
	// longer than a body line, a long title's slug, would put a line over
	// the lint's limit and the commit would be refused, so the body names
	// its folder instead, the file being the commit's to show (bug 22).
	in := filepath.ToSlash(files[1].path)
	if utf8.RuneCountInString(in+",") > bodyWidth {
		in = filepath.ToSlash(dir)
	}
	body := fmt.Sprintf("Record %s (\"%s\") as decision %d, %s, in %s, by itos question record.", id, oneLine(q.Question), n, title, in)
	if supersedes != 0 {
		body += fmt.Sprintf(" It supersedes decision %d, which leaves the index.", supersedes)
	}
	sha, code, err := writeCommitted(cfg, files, header, body, o)
	if err != nil || code != 0 {
		unmake(made)
		return code, err
	}
	return reportRecord(fmt.Sprintf("%s recorded as decision %d, %s: %s", id, n, filepath.ToSlash(files[1].path), askCommitted(sha, header)),
		*q, files[1].path, sha, o)
}

// madeDirs makes the folder and those above it that are not there, and
// says which it made, the deepest first, for unmake.
func madeDirs(dir string) ([]string, error) {
	var made []string
	for at := dir; at != "." && at != string(filepath.Separator) && at != ""; at = filepath.Dir(at) {
		if _, err := os.Stat(at); err == nil {
			break
		}
		made = append(made, at)
	}
	return made, os.MkdirAll(dir, 0o755)
}

// unmake removes the folders madeDirs made, when they are empty again.
func unmake(made []string) {
	for _, dir := range made {
		// A folder something else wrote into stays.
		_ = os.Remove(dir)
	}
}

// reportRecord prints record's result: the line, unless -q; under --json,
// ok, the question as it is now, the record written (null for --none) and
// the commit (null when none).
func reportRecord(line string, q ask.Question, record, sha string, o Out) (int, error) {
	if o.JSON {
		var commit, file any
		if sha != "" {
			commit = sha
		}
		if record != "" {
			file = filepath.ToSlash(record)
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "question", Value: entryOf(q)},
			out.Field{Key: "record", Value: file}, out.Field{Key: "commit", Value: commit})
	}
	if !o.Quiet {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// repeated takes every occurrence of the flag out of args, as `--flag
// <value>` or `--flag=<value>`, before the arguments' "--": the values in
// order and the arguments left. A flag with no value, or an empty one, is a
// usage error naming the command.
func repeated(command, flag string, args []string) (values, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		name, value, joined := strings.Cut(arg, "=")
		if name != flag {
			rest = append(rest, arg)
			continue
		}
		if !joined {
			if i+1 >= len(args) {
				return nil, nil, usage("%s %s needs a value", command, flag)
			}
			i++
			value = args[i]
		}
		if value = oneLine(value); value == "" {
			return nil, nil, usage("%s %s needs a value", command, flag)
		}
		values = append(values, value)
	}
	return values, rest, nil
}
