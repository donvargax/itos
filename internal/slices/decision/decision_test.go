package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

var at = time.Date(2026, 10, 22, 9, 30, 0, 0, time.UTC)

// ports are the decision command's ports over the fakes: a config, or the
// error loading it, the tracker's fake and a stopped clock.
type ports struct {
	cfg     *config.Loaded
	err     error
	tracker trackerport.Tracker
}

func (p ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

func (ports) Now(context.Context) time.Time { return at }

// The default config's files: the registry, the questions and the records'
// folder.
const (
	registry  = "tasks/work-items.yaml"
	asks      = "tasks/asks.yaml"
	decisions = "docs/decisions"
)

// A registry holding the item p1-a.
const registryText = "phases: { 1: null }\nitems:\n  - { id: p1-a, title: A, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }\n"

// A record numbered 1, accepted.
const oldRecord = "---\nstatus: accepted\ndate: 2026-01-01\n---\n\n# Old\n\n## Context and Problem Statement\n\nOld.\n"

// over is the ports of a project from an empty folder: the working tree's
// files, read through the config's source, and the tracker, its documents
// committed (none when committed is nil).
func over(t *testing.T, files map[string]string, committed map[string]string) (ports, *trackerfake.Tracker) {
	t.Helper()
	t.Chdir(t.TempDir())
	src, err := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	return withSource(t, src, committed)
}

// withSource is over with the config's source src.
func withSource(t *testing.T, src treeport.Source, committed map[string]string) (ports, *trackerfake.Tracker) {
	t.Helper()
	cfg, err := config.FromText(&vcsfake.VCS{}, treefake.Trees{}, src, "itos.yaml", "version: 1\n", false)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &trackerfake.Tracker{}
	commit(t, tracker, committed)
	return ports{cfg: cfg, tracker: tracker}, tracker
}

// commit commits the documents in the tracker.
func commit(t *testing.T, tracker trackerport.Tracker, docs map[string]string) {
	t.Helper()
	if len(docs) == 0 {
		return
	}
	var changes []trackerport.Change
	for name, text := range docs {
		changes = append(changes, trackerport.Change{Name: name, Text: text, Created: true})
	}
	if _, err := tracker.Transact(t.Context(), trackerport.Transaction{Changes: changes, Paragraphs: []string{"chore: start"}, Quiet: true}); err != nil {
		t.Fatal(err)
	}
}

// call is the command's line run over p: its exit, stdout, stderr and
// error; a --json or --quiet first sets the global flag.
func call(t *testing.T, p Ports, args ...string) (int, string, string, error) {
	t.Helper()
	json, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "--quiet") {
		json, quiet = json || args[0] == "--json", quiet || args[0] == "--quiet"
		args = args[1:]
	}
	c, err := Parse(args, json, quiet)
	if err != nil {
		t.Fatalf("decision %s: %v", strings.Join(args, " "), err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// doc is the tracker's text of name, "" when it is not there.
func doc(t *testing.T, tracker trackerport.Tracker, name string) string {
	t.Helper()
	docs, err := tracker.Load(t.Context(), []string{name})
	if err != nil {
		t.Fatal(err)
	}
	return docs[0].Text
}

// questions is an asks file of the questions, each a YAML mapping's lines.
func questions(qs ...string) string {
	text := "questions:\n"
	for _, q := range qs {
		text += "  - " + strings.ReplaceAll(strings.TrimSuffix(q, "\n"), "\n", "\n    ") + "\n"
	}
	return text
}

func TestParseRefuses(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"nope"}, "decision has no subcommand nope: it takes add, answer, ratify, overturn, record or show, else --all"},
		{[]string{"--bogus"}, "decision does not take --bogus"},
		{[]string{"add"}, "decision add needs <text>, the question"},
		{[]string{"add", "  "}, "decision add needs <text>, the question"},
		{[]string{"add", "--bogus"}, "decision add does not take --bogus"},
		{[]string{"add", "Q?", "--class", ""}, "decision add takes one --class with a value"},
		{[]string{"answer", "--bogus"}, "decision answer does not take --bogus"},
		{[]string{"answer"}, "decision answer needs <id> <text>"},
		{[]string{"answer", "x-1", "Yes"}, `decision answer: "x-1" is no question id (q-<n>)`},
		{[]string{"answer", "q-1"}, "decision answer needs <id> <text>, the answer"},
		{[]string{"answer", "q-1", "A.", "--provisional"}, "decision answer --provisional needs --by <handle>, who answers in the person's place"},
		{[]string{"answer", "q-1", "A.", "--by", "agent-1"}, "decision answer --by needs --provisional: an answer by someone else is provisional until the person ratifies it"},
		{[]string{"answer", "q-1", "A.", "--provisional", "--by", " "}, "decision answer takes one --by with a value"},
		{[]string{"ratify", "--bogus"}, "decision ratify does not take --bogus"},
		{[]string{"ratify"}, "decision ratify needs <id>"},
		{[]string{"ratify", "q-1", "q-2"}, "decision ratify takes one <id>, not q-2"},
		{[]string{"overturn", "--bogus"}, "decision overturn does not take --bogus"},
		{[]string{"overturn"}, "decision overturn needs <id> <text>"},
		{[]string{"overturn", "q-1"}, "decision overturn needs <id> <text>, the person's answer"},
		{[]string{"overturn", "q-1", " "}, "decision overturn needs <id> <text>, the person's answer"},
		{[]string{"show", "--bogus"}, "decision show does not take --bogus"},
		{[]string{"show"}, "decision show needs <id>"},
		{[]string{"show", "q-1", "q-2"}, "decision show takes one <id>, not q-2"},
		{[]string{"record", "q-1", "--option"}, "decision record --option needs a value"},
		{[]string{"record", "q-1", "--option=  "}, "decision record --option needs a value"},
		{[]string{"record", "q-1", "--bogus"}, "decision record does not take --bogus"},
		{[]string{"record", "--title", "T"}, "decision record needs <id>"},
		{[]string{"record", "q-1", "q-2", "--title", "T"}, "decision record takes one <id>, not q-2"},
		{[]string{"record", "q-1", "--title", "T", "--supersedes", "x"}, `decision record --supersedes takes a decision record's number, not "x"`},
		{[]string{"record", "q-1", "--title", "T", "--supersedes", "0"}, `decision record --supersedes takes a decision record's number, not "0"`},
		{[]string{"record", "q-1", "--none", "--title", "T"}, "decision record --none writes no record, so it takes no --title, --option, --consequences or --supersedes"},
		{[]string{"record", "q-1", "--none", "--consequences", "C"}, "decision record --none writes no record, so it takes no --title, --option, --consequences or --supersedes"},
		{[]string{"record", "q-1", "--none", "--supersedes", "1"}, "decision record --none writes no record, so it takes no --title, --option, --consequences or --supersedes"},
		{[]string{"record", "q-1", "--none", "--option", "O"}, "decision record --none writes no record, so it takes no --title, --option, --consequences or --supersedes"},
		{[]string{"record", "q-1"}, "decision record needs --title <title>, the decision's, or --none"},
		{[]string{"record", "q-1", "--title", "!!"}, "decision record --title needs a letter or a digit, which the record's file is named by"},
	} {
		_, err := Parse(test.args, false, false)
		var usage out.UsageError
		if !errors.As(err, &usage) || err.Error() != test.want {
			t.Errorf("Parse(%q) = %v, want the usage error %q", test.args, err, test.want)
		}
	}
}

func TestParseReads(t *testing.T) {
	for _, test := range []struct {
		args []string
		want Query
	}{
		{nil, Query{}},
		{[]string{"--all"}, Query{All: true}},
		{[]string{"add", "Which", "way?", "--item", "p1-a"}, Query{Sub: "add", Text: "Which way?", Item: "p1-a"}},
		{[]string{"add", "Which", "way?", "--class", "spec"}, Query{Sub: "add", Text: "Which way?", Class: "spec"}},
		{[]string{"answer", "q-2", " Left ", "now"}, Query{Sub: "answer", ID: "q-2", Text: "Left  now"}},
		{[]string{"answer", "q-2", "Left", "--provisional", "--by", "agent-1"}, Query{Sub: "answer", ID: "q-2", Text: "Left", Provisional: true, By: "agent-1"}},
		{[]string{"ratify", "q-3"}, Query{Sub: "ratify", ID: "q-3"}},
		{[]string{"overturn", "q-3", "Right,", "then"}, Query{Sub: "overturn", ID: "q-3", Text: "Right, then"}},
		{[]string{"show", "q-3"}, Query{Sub: "show", ID: "q-3"}},
		{[]string{"record", "q-1", "--none"}, Query{Sub: "record", ID: "q-1", None: true}},
		{
			[]string{"record", "q-1", "--title", " A  title ", "--option", "One", "--option=Two  too", "--consequences", " C ", "--supersedes", "3"},
			Query{Sub: "record", ID: "q-1", Title: "A title", Options: []string{"One", "Two too"}, Consequences: "C", Supersedes: 3},
		},
		{
			[]string{"record", "q-1", "--title", "T", "--", "--option"},
			Query{},
		},
	} {
		c, err := Parse(test.args, true, true)
		if len(test.args) > 3 && test.args[len(test.args)-2] == "--" {
			// After "--" --option is no flag: a second <id>.
			if err == nil || err.Error() != "decision record takes one <id>, not --option" {
				t.Errorf("Parse(%q) = %v, want --option read as an id", test.args, err)
			}
			continue
		}
		if err != nil || !c.JSON || !c.Quiet || c.Sub != test.want.Sub || c.All != test.want.All || c.ID != test.want.ID ||
			c.Text != test.want.Text || c.Item != test.want.Item || c.Class != test.want.Class || c.Provisional != test.want.Provisional ||
			c.By != test.want.By || c.Title != test.want.Title || !slices.Equal(c.Options, test.want.Options) ||
			c.Consequences != test.want.Consequences || c.Supersedes != test.want.Supersedes || c.None != test.want.None {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", test.args, c, err, test.want)
		}
	}
}

// The list: the open questions, or with --all every one, each answered one
// with its answer and its decision, then the nudge.
func TestList(t *testing.T) {
	p, _ := over(t, nil, map[string]string{asks: questions(
		"id: q-1\nquestion: \"Open?\"\nitem: p1-a",
		"id: q-2\nquestion: \"Recorded?\"\nanswer: \"Yes.\"\ndecision: 7",
		"id: q-3\nquestion: \"Item only?\"\nanswer: \"Yes.\"\ndecision: none",
		"id: q-10\nquestion: \"Not yet?\"\nanswer: \"No.\"",
	)})
	code, stdout, stderr, err := call(t, p, "--all")
	want := "Open questions:\n  q-1   Open?  (p1-a)\n\nAnswered questions:\n" +
		"  q-2   Recorded?\n        → Yes.\n          decision 7\n" +
		"  q-3   Item only?\n        → Yes.\n          no decision (--none)\n" +
		"  q-10  Not yet?\n        → No.\n" +
		"\nAnswered and recorded nowhere: itos decision record q-10 (--title <title>, or --none)\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("--all: %d, %v, stderr %q\n got %q\nwant %q", code, err, stderr, stdout, want)
	}
	code, stdout, _, err = call(t, p)
	want = "Open questions:\n  q-1  Open?  (p1-a)\n\nAnswered and recorded nowhere: itos decision record q-10 (--title <title>, or --none)\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("the open ones: %d, %v\n got %q\nwant %q", code, err, stdout, want)
	}
	code, stdout, _, err = call(t, p, "--json", "--all")
	var listed struct{ Questions []map[string]any }
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &listed) != nil || len(listed.Questions) != 4 ||
		listed.Questions[0]["id"] != "q-1" || listed.Questions[3]["id"] != "q-10" {
		t.Errorf("--json --all: %d, %v, %s", code, err, stdout)
	}
}

// With no open question the list says so, and the nudge follows; with none
// at all, --all says there are none; --json lists none as [].
func TestListNone(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Done?\"\nanswer: \"Yes.\"")})
	code, stdout, _, err := call(t, p)
	want := "No open questions.\n\nAnswered and recorded nowhere: itos decision record q-1 (--title <title>, or --none)\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("none open: %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--all")
	want = "No open questions.\n\nAnswered questions:\n  q-1  Done?\n       → Yes.\n\nAnswered and recorded nowhere: itos decision record q-1 (--title <title>, or --none)\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("none open, --all: %d, %v, %q", code, err, stdout)
	}
	commit(t, tracker, map[string]string{asks: "questions: []\n"})
	if code, stdout, _, err = call(t, p, "--all"); code != 0 || err != nil || stdout != "No questions.\n" {
		t.Errorf("none, --all: %d, %v, %q", code, err, stdout)
	}
	if code, stdout, _, err = call(t, p); code != 0 || err != nil || stdout != "No open questions.\n" {
		t.Errorf("none: %d, %v, %q", code, err, stdout)
	}
	if code, stdout, _, err = call(t, p, "--json"); code != 0 || err != nil || !strings.Contains(stdout, `"questions": []`) {
		t.Errorf("none, --json: %d, %v, %q", code, err, stdout)
	}
}

// show prints the question, its status, item and decision, then its answer;
// an id no question has is refused, exit 1.
func TestShow(t *testing.T) {
	p, _ := over(t, nil, map[string]string{asks: questions(
		"id: q-1\nquestion: \"Which\n  way?\"\nitem: p1-a\nanswer: \"Left.\\n\"\ndecision: 4",
		"id: q-2\nquestion: \"Open?\"",
	)})
	code, stdout, _, err := call(t, p, "show", "q-1")
	if want := "q-1: Which way?\nanswered, about p1-a, decision 4\n\nLeft.\n"; code != 0 || err != nil || stdout != want {
		t.Errorf("show q-1: %d, %v\n got %q\nwant %q", code, err, stdout, want)
	}
	code, stdout, _, err = call(t, p, "show", "q-2")
	if want := "q-2: Open?\nopen\n"; code != 0 || err != nil || stdout != want {
		t.Errorf("show q-2: %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--json", "show", "q-2")
	if code != 0 || err != nil || !strings.Contains(stdout, `"ok": true`) || !strings.Contains(stdout, `"id": "q-2"`) {
		t.Errorf("show q-2 --json: %d, %v, %q", code, err, stdout)
	}
	code, stdout, stderr, err := call(t, p, "show", "q-9")
	if code != 1 || err != nil || stdout != "" || stderr != "there is no question q-9\n  fix: itos decision --all lists the questions\n" {
		t.Errorf("show q-9: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(t, p, "--json", "show", "q-9")
	var refused struct {
		OK       bool
		Problems []out.Problem
	}
	if code != 1 || err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &refused) != nil || refused.OK ||
		len(refused.Problems) != 1 || refused.Problems[0].Rule != "ask-no-question" {
		t.Errorf("show q-9 --json: %d, %v, %q, %q", code, err, stdout, stderr)
	}
}

// A refusal with no fix prints its message alone.
func TestRefuseWithNoFix(t *testing.T) {
	var stdout, stderr strings.Builder
	code, err := CLI{}.refuse(&stdout, &stderr, []out.Problem{{Rule: "r", Message: "no"}})
	if code != 1 || err != nil || stdout.Len() != 0 || stderr.String() != "no\n" {
		t.Errorf("refuse: %d, %v, %q, %q", code, err, stdout.String(), stderr.String())
	}
}

// A config that does not load, and questions the tracker cannot load, are
// the errors of every subcommand, nothing printed.
func TestErrors(t *testing.T) {
	broken := errors.New("broken")
	p := ports{err: broken}
	for _, args := range [][]string{nil, {"show", "q-1"}, {"add", "Q"}, {"answer", "q-1", "A"}, {"ratify", "q-1"}, {"overturn", "q-1", "A"}, {"record", "q-1", "--none"}} {
		if code, stdout, stderr, err := call(t, p, args...); code != 0 || !errors.Is(err, broken) || stdout != "" || stderr != "" {
			t.Errorf("%q with no config: %d, %v, %q, %q", args, code, err, stdout, stderr)
		}
	}
	p, _ = over(t, nil, nil)
	p.tracker = unloadable{&trackerfake.Tracker{}, broken, ""}
	for _, args := range [][]string{nil, {"show", "q-1"}, {"add", "Q"}, {"answer", "q-1", "A"}, {"ratify", "q-1"}, {"overturn", "q-1", "A"}, {"record", "q-1", "--none"}} {
		if code, _, _, err := call(t, p, args...); code != 0 || !errors.Is(err, broken) {
			t.Errorf("%q with questions not loaded: %d, %v", args, code, err)
		}
	}
}

// unloadable is a tracker whose Load fails, of every document or of those
// whose name ends with suffix.
type unloadable struct {
	*trackerfake.Tracker
	err    error
	suffix string
}

func (u unloadable) Load(ctx context.Context, names []string) ([]trackerport.Doc, error) {
	if strings.HasSuffix(names[0], u.suffix) {
		return nil, u.err
	}
	return u.Tracker.Load(ctx, names)
}

// add asks a question with the next id, about the item --item names, and
// commits the file alone; under --json the question and the commit; under
// --quiet nothing.
func TestAdd(t *testing.T) {
	p, tracker := over(t, map[string]string{registry: registryText}, nil)
	code, stdout, stderr, err := call(t, p, "add", "Which", "way?", "--item", "p1-a")
	if code != 0 || err != nil || stdout != "q-1 asked about p1-a: committed 0000000 docs: ask q-1\n" || stderr != "[1] docs: ask q-1\n" {
		t.Errorf("add: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "id: q-1") || !strings.Contains(text, "item: \"p1-a\"") || !strings.Contains(text, "Which way?") {
		t.Errorf("the questions are %q", text)
	}
	code, stdout, _, err = call(t, p, "--json", "add", "Again?")
	var added struct {
		OK       bool
		Question map[string]any
		Commit   any
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &added) != nil || !added.OK || added.Question["id"] != "q-2" ||
		added.Commit != strings.Repeat("0", 39)+"2" {
		t.Errorf("add --json: %d, %v, %s", code, err, stdout)
	}
	if code, stdout, stderr, err = call(t, p, "--quiet", "add", "Quiet?"); code != 0 || err != nil || stdout != "" || stderr != "" {
		t.Errorf("add --quiet: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = call(t, p, "add", "Plain?")
	if code != 0 || err != nil || stdout != "q-4 asked: committed 0000000 docs: ask q-4\n" {
		t.Errorf("add with no item: %d, %v, %q", code, err, stdout)
	}
}

// The commit's body says what was asked, on one line, the item it names.
func TestAddBody(t *testing.T) {
	p, tracker := over(t, map[string]string{registry: registryText}, nil)
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	if code, _, _, err := call(t, p, "add", "Which\n  way?", "--item", "p1-a"); code != 0 || err != nil {
		t.Fatalf("add: %d, %v", code, err)
	}
	if want := []string{"docs: ask q-1", `Ask q-1 about p1-a ("Which way?"), with itos decision add.`}; !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q, want %q", seen.paragraphs, want)
	}
}

// seeing is the tracker's fake keeping the paragraphs of its last
// transaction.
type seeing struct {
	*trackerfake.Tracker
	paragraphs []string
}

func (s *seeing) Transact(ctx context.Context, tx trackerport.Transaction) (string, error) {
	s.paragraphs = tx.Paragraphs
	return s.Tracker.Transact(ctx, tx)
}

// flat is the paragraphs, each unwrapped onto one line.
func flat(paragraphs []string) []string {
	lines := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		lines[i] = strings.Join(strings.Fields(p), " ")
	}
	return lines
}

// add refuses an item the registry does not have, or no registry, exit 1,
// nothing written; a registry it cannot read is the error.
func TestAddItem(t *testing.T) {
	p, _ := over(t, nil, nil)
	code, stdout, stderr, err := call(t, p, "add", "Q?", "--item", "p1-a")
	if code != 1 || err != nil || stdout != "" || !strings.HasPrefix(stderr, "no work registry at "+registry+"\n") {
		t.Errorf("no registry: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p, tracker := over(t, map[string]string{registry: registryText}, nil)
	code, _, stderr, err = call(t, p, "add", "Q?", "--item", "p1-b")
	if code != 1 || err != nil || stderr != "no item p1-b in "+registry+", so no question can name it\n  fix: itos work list --all names every item; --item may be left out\n" {
		t.Errorf("unknown item: %d, %v, %q", code, err, stderr)
	}
	if doc(t, tracker, asks) != "" {
		t.Error("a refused add wrote the questions")
	}
	p, _ = over(t, map[string]string{registry: "items: [\n"}, nil)
	if code, _, _, err = call(t, p, "add", "Q?", "--item", "p1-a"); code != 0 || err == nil {
		t.Errorf("a registry that does not parse: %d, %v", code, err)
	}
}

// A file with changes no commit holds, and a commit a hook refuses, are
// refused, exit 1; hooks not ready are the error.
func TestAddWriteRefused(t *testing.T) {
	p, tracker := over(t, nil, nil)
	tracker.Edit(asks, "questions: []\n")
	code, _, stderr, err := call(t, p, "add", "Q?")
	if code != 1 || err != nil || !strings.HasPrefix(stderr, asks+" has changes no commit holds") {
		t.Errorf("dirty: %d, %v, %q", code, err, stderr)
	}
	p, tracker = over(t, nil, nil)
	tracker.Refuse = true
	code, stdout, stderr, err := call(t, p, "add", "Q?")
	if code != 1 || err != nil || stdout != "" || !strings.Contains(stderr, "refused\nthe commit of "+asks+" failed (git exited 1)") {
		t.Errorf("refused: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p, tracker = over(t, nil, nil)
	tracker.Unready = errors.New("no hooks")
	if code, _, _, err = call(t, p, "add", "Q?"); code != 0 || err == nil || err.Error() != "no hooks" {
		t.Errorf("unready: %d, %v", code, err)
	}
}

// Under a stealth config nothing is committed: the line says so, --json's
// commit is null; the lock is held while the command writes, and one held
// already is the error.
func TestStealth(t *testing.T) {
	p, tracker := over(t, nil, nil)
	tracker.Stealth = true
	code, stdout, _, err := call(t, p, "add", "Q?")
	if code != 0 || err != nil || stdout != "q-1 asked: the questions are the stealth config's, so nothing is committed\n" {
		t.Errorf("stealth add: %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--json", "answer", "q-1", "A.")
	if code != 0 || err != nil || !strings.Contains(stdout, `"commit": null`) {
		t.Errorf("stealth answer --json: %d, %v, %q", code, err, stdout)
	}
	held, err := tracker.Hold(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "Q?"}, {"answer", "q-1", "A"}, {"ratify", "q-1"}, {"overturn", "q-1", "A"}, {"record", "q-1", "--none"}} {
		if code, _, _, err = call(t, p, args...); code != 0 || err == nil {
			t.Errorf("%q with the lock held: %d, %v", args, code, err)
		}
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	p.tracker = stuck{tracker}
	code, stdout, stderr, err := call(t, p, "add", "Q?")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "q-2 asked") || stderr != "itos: the lock cannot be given back: stuck\n" {
		t.Errorf("a lock that cannot be given back: %d, %v, %q, %q", code, err, stdout, stderr)
	}
}

// stuck is a tracker whose lock cannot be given back.
type stuck struct{ *trackerfake.Tracker }

func (s stuck) Hold(context.Context) (trackerport.Held, error) { return stuckHeld{}, nil }

type stuckHeld struct{}

func (stuckHeld) Release(context.Context) error { return errors.New("stuck") }

// answer keeps the answer beside the question and commits the file alone; a
// question already answered, or none with the id, is refused.
func TestAnswer(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Which\n  way?\"", "id: q-2\nquestion: \"Done?\"\nanswer: \"Yes\n  indeed.\"")})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	code, stdout, _, err := call(t, p, "answer", "q-1", "Left,", "then", "right.")
	if code != 0 || err != nil || stdout != "q-1 answered: committed 0000000 docs: answer q-1\n" {
		t.Errorf("answer: %d, %v, %q", code, err, stdout)
	}
	if want := []string{"docs: answer q-1", `Answer q-1 ("Which way?") with "Left, then right.", by itos decision answer.`}; !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "Left, then right.") {
		t.Errorf("the questions are %q", text)
	}
	code, _, stderr, err := call(t, p, "answer", "q-2", "No.")
	if code != 1 || err != nil || stderr != "q-2 is already answered: Yes indeed.\n  fix: itos decision add asks a new question\n" {
		t.Errorf("answered: %d, %v, %q", code, err, stderr)
	}
	if code, _, stderr, err = call(t, p, "answer", "q-9", "No."); code != 1 || err != nil || !strings.HasPrefix(stderr, "there is no question q-9\n") {
		t.Errorf("no question: %d, %v, %q", code, err, stderr)
	}
	tracker.Refuse = true
	if code, _, _, err = call(t, p, "answer", "q-1", "Again."); code != 1 || err != nil {
		t.Errorf("refused: %d, %v", code, err)
	}
}

// delegating is the ports of a project whose config delegates the classes,
// over the committed questions.
func delegating(t *testing.T, classes string, committed map[string]string) (ports, *trackerfake.Tracker) {
	t.Helper()
	t.Chdir(t.TempDir())
	src, err := treefake.Trees{Files: map[string]map[string]string{"worktree": nil}}.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, treefake.Trees{}, src, "itos.yaml", "version: 1\nwork: { delegated: "+classes+" }\n", false)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &trackerfake.Tracker{}
	commit(t, tracker, committed)
	return ports{cfg: cfg, tracker: tracker}, tracker
}

// add --class keeps the class beside the question, and the commit's body
// names it; answer --provisional --by keeps the answer with provisional and
// who gave it, and its body says so; the answer is refused for a question
// with no class, or of a class the config does not delegate, and a plain
// answer is refused on a provisional one, naming ratify and overturn.
func TestAnswerProvisional(t *testing.T) {
	p, tracker := delegating(t, "[spec]", map[string]string{asks: questions("id: q-1\nquestion: \"Plain?\"", "id: q-2\nquestion: \"Reviewed?\"\nclass: review")})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	code, stdout, _, err := call(t, p, "add", "Labels", "or Projects?", "--class", "spec")
	if code != 0 || err != nil || stdout != "q-3 asked: committed 0000000 docs: ask q-3\n" {
		t.Errorf("add --class: %d, %v, %q", code, err, stdout)
	}
	if want := []string{"docs: ask q-3", `Ask q-3 ("Labels or Projects?"), of the class spec, with itos decision add.`}; !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "    question: \"Labels or Projects?\"\n    class: \"spec\"\n") {
		t.Errorf("the questions are %q", text)
	}
	for id, want := range map[string]string{
		"q-1": "q-1 has no class, so it is the person's to answer\n  fix: a question an answering agent may settle is asked with itos decision add --class <class>, a class work.delegated lists\n",
		"q-2": "q-2 is of the class review, which work.delegated does not list, so it is the person's to answer\n  fix: list review in work.delegated for an answering agent to settle it, or leave it to the person\n",
	} {
		code, stdout, stderr, err := call(t, p, "answer", id, "Yes.", "--provisional", "--by", "agent-1")
		if code != 1 || err != nil || stdout != "" || stderr != want {
			t.Errorf("answer %s --provisional: %d, %v, %q, %q", id, code, err, stdout, stderr)
		}
	}
	if text := doc(t, tracker, asks); strings.Contains(text, "Yes.") {
		t.Errorf("a refused answer was written: %q", text)
	}
	code, stdout, _, err = call(t, p, "answer", "q-3", "Labels.", "--provisional", "--by", "agent-1")
	if code != 0 || err != nil || stdout != "q-3 answered provisionally by agent-1: committed 0000000 docs: answer q-3\n" {
		t.Errorf("answer --provisional: %d, %v, %q", code, err, stdout)
	}
	want := []string{"docs: answer q-3", `Answer q-3 ("Labels or Projects?", of the class spec) with "Labels.", provisionally, by agent-1, through itos decision answer --provisional; work proceeds on it until the person ratifies or overturns it.`}
	if !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "    answer: \"Labels.\"\n    provisional: true\n    by: \"agent-1\"\n") {
		t.Errorf("the questions are %q", text)
	}
	code, stdout, stderr, err := call(t, p, "answer", "q-3", "Projects.")
	if code != 1 || err != nil || stdout != "" || stderr != "q-3 is already answered, provisionally, by agent-1: Labels.\n"+
		"  fix: itos decision ratify q-3 makes that the decision, itos decision overturn q-3 <text> replaces it with the person's\n" {
		t.Errorf("a plain answer on a provisional one: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = call(t, p, "--json", "answer", "q-3", "Projects.", "--provisional", "--by", "agent-2")
	if code != 1 || err != nil || !strings.Contains(stdout, `"rule": "ask-answered"`) {
		t.Errorf("a second provisional answer: %d, %v, %q", code, err, stdout)
	}
}

// ratify clears provisional, keeping the class and who answered; overturn
// replaces the answer with the person's and keeps the agent's as
// overturned; each commits the file alone, its body saying so; a question
// with no provisional answer, open or the person's, or none with the id,
// is refused, nothing written.
func TestSettle(t *testing.T) {
	p, tracker := delegating(t, "[spec]", map[string]string{asks: questions(
		"id: q-1\nquestion: \"Labels?\"\nclass: spec\nanswer: \"Labels.\"\nprovisional: true\nby: agent-1",
		"id: q-2\nquestion: \"Archive?\"\nclass: spec\nanswer: \"No.\"\nprovisional: true\nby: agent-2",
		"id: q-3\nquestion: \"Open?\"\nclass: spec",
		"id: q-4\nquestion: \"Mine?\"\nanswer: \"Yes.\"",
	)})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	code, stdout, _, err := call(t, p, "ratify", "q-1")
	if code != 0 || err != nil || stdout != "q-1 ratified: committed 0000000 docs: ratify q-1\n" {
		t.Errorf("ratify: %d, %v, %q", code, err, stdout)
	}
	if want := []string{"docs: ratify q-1", `Ratify q-1 ("Labels?"): agent-1's answer "Labels." is the decision, by itos decision ratify.`}; !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	code, stdout, _, err = call(t, p, "--json", "overturn", "q-2", "Yes,", "it", "is.")
	var settled struct {
		OK       bool
		Question map[string]any
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &settled) != nil || !settled.OK || settled.Question["answer"] != "Yes, it is." ||
		settled.Question["overturned"] != "No." || settled.Question["by"] != "agent-2" || settled.Question["provisional"] != nil {
		t.Errorf("overturn --json: %d, %v, %s", code, err, stdout)
	}
	want := []string{"docs: overturn q-2", `Overturn q-2 ("Archive?"): agent-2's answer "No." is replaced by the person's, "Yes, it is.", by itos decision overturn.`}
	if !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	text := doc(t, tracker, asks)
	if strings.Contains(text, "provisional") || !strings.Contains(text, "    class: \"spec\"\n    answer: \"Labels.\"\n    by: \"agent-1\"\n") ||
		!strings.Contains(text, "    answer: \"Yes, it is.\"\n    by: \"agent-2\"\n    overturned: \"No.\"\n") {
		t.Errorf("the questions are %q", text)
	}
	for _, args := range [][]string{{"ratify", "q-1"}, {"overturn", "q-2", "Again."}, {"ratify", "q-3"}, {"overturn", "q-4", "No."}} {
		code, stdout, stderr, err := call(t, p, args...)
		want := fmt.Sprintf("%s has no provisional answer, so there is nothing to %s\n  fix: itos decision lists the provisional answers awaiting the person\n", args[1], args[0])
		if code != 1 || err != nil || stdout != "" || stderr != want {
			t.Errorf("%q: %d, %v, %q, %q", args, code, err, stdout, stderr)
		}
	}
	if code, _, stderr, err := call(t, p, "ratify", "q-9"); code != 1 || err != nil || !strings.HasPrefix(stderr, "there is no question q-9\n") {
		t.Errorf("ratify q-9: %d, %v, %q", code, err, stderr)
	}
	if doc(t, tracker, asks) != text {
		t.Error("a refused ratify or overturn wrote the questions")
	}
	p, tracker = delegating(t, "[spec]", map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nclass: spec\nanswer: \"A.\"\nprovisional: true\nby: agent-1")})
	tracker.Refuse = true
	if code, _, _, err := call(t, p, "ratify", "q-1"); code != 1 || err != nil {
		t.Errorf("refused: %d, %v", code, err)
	}
}

// The list holds the provisional answers in a section of their own, oldest
// first, each with its class and who gave it, after the open questions and
// before the answered ones, and the nudge leaves them out; --json gives
// them as provisional, apart from questions; show says how an answer
// stands with the person.
func TestListProvisional(t *testing.T) {
	p, _ := over(t, nil, map[string]string{asks: questions(
		"id: q-1\nquestion: \"Labels?\"\nclass: spec\nanswer: \"Labels.\"\nprovisional: true\nby: agent-1",
		"id: q-2\nquestion: \"Starter?\"",
		"id: q-3\nquestion: \"Archive?\"\nanswer: \"No.\"\nprovisional: true\nby: agent-2",
		"id: q-4\nquestion: \"Mine?\"\nclass: spec\nanswer: \"Yes.\"\nby: agent-1",
		"id: q-5\nquestion: \"Theirs?\"\nclass: spec\nanswer: \"No.\"\nby: agent-1\noverturned: \"Yes.\"\ndecision: none",
	)})
	code, stdout, _, err := call(t, p)
	want := "Open questions:\n  q-2  Starter?\n\nProvisional answers, awaiting the person (itos decision ratify <id>, or overturn <id> <text>):\n" +
		"  q-1  Labels?\n       → Labels.\n         spec, by agent-1\n" +
		"  q-3  Archive?\n       → No.\n         by agent-2\n" +
		"\nAnswered and recorded nowhere: itos decision record q-4 (--title <title>, or --none)\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("the list: %d, %v\n got %q\nwant %q", code, err, stdout, want)
	}
	code, stdout, _, err = call(t, p, "--json", "--all")
	var listed struct{ Questions, Provisional []map[string]any }
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &listed) != nil || len(listed.Questions) != 3 || len(listed.Provisional) != 2 ||
		listed.Questions[0]["id"] != "q-2" || listed.Questions[1]["id"] != "q-4" || listed.Provisional[1]["id"] != "q-3" ||
		listed.Provisional[0]["provisional"] != true || listed.Provisional[0]["class"] != "spec" {
		t.Errorf("--json --all: %d, %v, %s", code, err, stdout)
	}
	for id, want := range map[string]string{
		"q-1": "q-1: Labels?\nanswered, class spec, provisional, by agent-1\n\nLabels.\n",
		"q-4": "q-4: Mine?\nanswered, class spec, ratified, by agent-1\n\nYes.\n",
		"q-5": "q-5: Theirs?\nanswered, class spec, overturned, by the person, no decision (--none)\n\nNo.\n\noverturned, by agent-1:\nYes.\n",
	} {
		if code, stdout, _, err := call(t, p, "show", id); code != 0 || err != nil || stdout != want {
			t.Errorf("show %s: %d, %v\n got %q\nwant %q", id, code, err, stdout, want)
		}
	}
	p, _ = over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Labels?\"\nclass: spec\nanswer: \"Labels.\"\nprovisional: true\nby: agent-1")})
	code, stdout, _, err = call(t, p)
	want = "No open questions.\n\nProvisional answers, awaiting the person (itos decision ratify <id>, or overturn <id> <text>):\n  q-1  Labels?\n       → Labels.\n         spec, by agent-1\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("none open: %d, %v\n got %q\nwant %q", code, err, stdout, want)
	}
	code, _, stderr, err := call(t, p, "record", "q-1", "--none")
	if code != 1 || err != nil || stderr != "q-1's answer is provisional, agent-1's, so it holds no decision to record yet\n"+
		"  fix: itos decision ratify q-1 makes it the decision, itos decision overturn q-1 <text> replaces it\n" {
		t.Errorf("record a provisional answer: %d, %v, %q", code, err, stderr)
	}
}

// record --none marks the question as recorded nowhere, the file committed
// alone; one unanswered, one recorded, or none with the id, is refused.
func TestRecordNone(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions(
		"id: q-1\nquestion: \"Which\n  way?\"\nanswer: \"Left.\"",
		"id: q-2\nquestion: \"Open?\"",
		"id: q-3\nquestion: \"Done?\"\nanswer: \"Yes.\"\ndecision: none",
	)})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	code, stdout, _, err := call(t, p, "record", "q-1", "--none")
	if code != 0 || err != nil || stdout != "q-1 marked as recorded nowhere: committed 0000000 docs: mark q-1 as recorded nowhere\n" {
		t.Errorf("--none: %d, %v, %q", code, err, stdout)
	}
	want := []string{"docs: mark q-1 as recorded nowhere", `Mark q-1 ("Which way?") as recorded nowhere, its answer having concerned its item alone, so itos decision stops naming it. By itos decision record --none.`}
	if !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "decision: none") {
		t.Errorf("the questions are %q", text)
	}
	for id, wantErr := range map[string]string{
		"q-1": "q-1 is already recorded, as no decision (--none)\n  fix: itos decision show q-1 shows it\n",
		"q-2": "q-2 is not answered, so it holds no decision to record\n  fix: itos decision answer q-2 <text> answers it first\n",
		"q-9": "there is no question q-9\n  fix: itos decision --all lists the questions\n",
	} {
		if code, _, stderr, err := call(t, p, "record", id, "--none"); code != 1 || err != nil || stderr != wantErr {
			t.Errorf("record %s --none: %d, %v, %q", id, code, err, stderr)
		}
	}
	code, stdout, _, err = call(t, p, "--json", "record", "q-3", "--title", "T")
	if code != 1 || err != nil || !strings.Contains(stdout, `"rule": "ask-recorded"`) {
		t.Errorf("record q-3 --json: %d, %v, %q", code, err, stdout)
	}
	p, _ = over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
	code, stdout, _, err = call(t, p, "--json", "record", "q-1", "--none")
	var recorded struct {
		OK       bool
		Question map[string]any
		Record   any
		Commit   any
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &recorded) != nil || !recorded.OK || recorded.Record != nil ||
		recorded.Commit == nil || recorded.Question["decision"] != "none" {
		t.Errorf("--none --json: %d, %v, %s", code, err, stdout)
	}
	p, tracker = over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
	tracker.Refuse = true
	if code, _, _, err = call(t, p, "record", "q-1", "--none"); code != 1 || err != nil {
		t.Errorf("--none refused: %d, %v", code, err)
	}
}

// record writes the answer as the next record, dated by the clock, with the
// item and the options, updates the index, and commits the four files
// together; --supersedes sets the old record's status.
func TestRecord(t *testing.T) {
	// The working tree's files are slash-separated; the tracker's fake keys a
	// document by the name it is given, the record's path as the command
	// joins it (filepath.Join).
	old := filepath.Join(decisions, "0001-old.md")
	p, tracker := over(t, map[string]string{decisions + "/0001-old.md": oldRecord}, map[string]string{
		asks: questions("id: q-1\nquestion: \"Which\n  way?\"\nitem: p1-a\nanswer: \"Left.\""),
		old:  oldRecord,
	})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	code, stdout, stderr, err := call(t, p, "record", "q-1", "--title", "Go left", "--option", "Left", "--option", "Right",
		"--consequences", "None.", "--supersedes", "1")
	file := filepath.Join(decisions, "0002-go-left.md")
	if code != 0 || err != nil || stdout != "q-1 recorded as decision 2, docs/decisions/0002-go-left.md: committed 0000000 docs: record q-1 as decision 2\n" {
		t.Errorf("record: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	want := []string{"docs: record q-1 as decision 2", `Record q-1 ("Which way?") as decision 2, Go left, in docs/decisions/0002-go-left.md, by itos decision record. It supersedes decision 1, which leaves the index.`}
	if !slices.Equal(flat(seen.paragraphs), want) {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	text := doc(t, tracker, file)
	for _, part := range []string{"date: 2026-10-22", "# Go left", "Which way?\n\nAsked as q-1, about p1-a.", "- Left\n- Right", "Left.", "None.", "Supersedes ADR-0001."} {
		if !strings.Contains(text, part) {
			t.Errorf("the record lacks %q:\n%s", part, text)
		}
	}
	if text := doc(t, tracker, old); !strings.Contains(text, "status: superseded by ADR-0002") {
		t.Errorf("the old record is %q", text)
	}
	if text := doc(t, tracker, filepath.Join(decisions, "README.md")); !strings.Contains(text, "0002-go-left.md") || strings.Contains(text, "0001-old.md") {
		t.Errorf("the index is %q", text)
	}
	if text := doc(t, tracker, asks); !strings.Contains(text, "decision: 2") {
		t.Errorf("the questions are %q", text)
	}
	if _, err := os.Stat(decisions); err != nil {
		t.Errorf("the records' folder was not made: %v", err)
	}
}

// --json gives the record's path; a title long enough makes the body name
// the folder rather than the file; a question with no item says none.
func TestRecordJSONAndLongTitle(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
	seen := &seeing{Tracker: tracker}
	p.tracker = seen
	title := strings.Repeat("word ", 18)
	code, stdout, _, err := call(t, p, "--json", "record", "q-1", "--title", title)
	var recorded struct {
		Record any
		Commit any
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &recorded) != nil ||
		recorded.Record != "docs/decisions/0001-"+strings.TrimSuffix(strings.Repeat("word-", 18), "-")+".md" || recorded.Commit == nil {
		t.Errorf("record --json: %d, %v, %s", code, err, stdout)
	}
	if len(seen.paragraphs) != 2 || !strings.Contains(seen.paragraphs[1], " in docs/decisions, by itos decision record.") {
		t.Errorf("the commit is %q", seen.paragraphs)
	}
	if text := doc(t, tracker, filepath.Join(decisions, strings.TrimPrefix(recorded.Record.(string), decisions+"/"))); !strings.Contains(text, "Q?\n\nAsked as q-1.") {
		t.Errorf("the record is %q", text)
	}
}

// A path of exactly a body line is the record's own; one past it, the
// folder's.
func TestRecordBodyWidth(t *testing.T) {
	for _, test := range []struct {
		width int
		file  bool
	}{{100, true}, {101, false}} {
		p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
		seen := &seeing{Tracker: tracker}
		p.tracker = seen
		// docs/decisions/0001-<slug>.md, and its comma.
		slug := strings.Repeat("a", test.width-len("docs/decisions/0001-.md,"))
		if code, _, _, err := call(t, p, "record", "q-1", "--title", slug); code != 0 || err != nil {
			t.Fatalf("record: %d, %v", code, err)
		}
		in := " in docs/decisions/0001-" + slug + ".md, by"
		if got := len(seen.paragraphs) == 2 && strings.Contains(strings.Join(strings.Fields(seen.paragraphs[1]), " "), in); got != test.file {
			t.Errorf("a path of %d: the body %q names the file: %v", test.width, seen.paragraphs, got)
		}
	}
}

// --supersedes naming no record is refused, nothing written or made.
func TestRecordSupersedesNone(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
	code, _, stderr, err := call(t, p, "record", "q-1", "--title", "T", "--supersedes", "4")
	if code != 1 || err != nil || stderr != "there is no decision record 4 in "+filepath.FromSlash(decisions)+", so none can be superseded\n  fix: --supersedes takes the number of a record there\n" {
		t.Errorf("supersedes 4: %d, %v, %q", code, err, stderr)
	}
	if text := doc(t, tracker, asks); strings.Contains(text, "decision:") {
		t.Errorf("the questions are %q", text)
	}
	if _, err := os.Stat("docs"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused record made a folder: %v", err)
	}
}

// --supersedes naming a record already superseded is refused, nothing
// written or made: it names the successor its status gives and the fix
// supersedes that one; a status naming none leaves the fix without a number;
// a deprecated record is not refused. --json gives the problem.
func TestRecordSupersedesSuperseded(t *testing.T) {
	for _, test := range []struct {
		status, want string
	}{
		{"superseded by ADR-0002", "decision record 1 is already superseded by ADR-0002, so it cannot be superseded again\n  fix: --supersedes 2 supersedes the record that replaced it\n"},
		{"Superseded", "decision record 1 is already Superseded, so it cannot be superseded again\n  fix: supersede the record that replaced it instead; this one's status does not name it\n"},
	} {
		text := strings.Replace(oldRecord, "status: accepted", "status: "+test.status, 1)
		file := filepath.Join(decisions, "0001-old.md")
		p, tracker := over(t, map[string]string{decisions + "/0001-old.md": text}, map[string]string{
			asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\""),
			file: text,
		})
		code, _, stderr, err := call(t, p, "record", "q-1", "--title", "T", "--supersedes", "1")
		if code != 1 || err != nil || stderr != test.want {
			t.Errorf("%s: %d, %v, %q", test.status, code, err, stderr)
		}
		if got := doc(t, tracker, file); got != text {
			t.Errorf("%s: the old record is %q", test.status, got)
		}
		if got := doc(t, tracker, asks); strings.Contains(got, "decision:") {
			t.Errorf("%s: the questions are %q", test.status, got)
		}
		if _, err := os.Stat("docs"); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: a refused record made a folder: %v", test.status, err)
		}
		code, stdout, _, err := call(t, p, "--json", "record", "q-1", "--title", "T", "--supersedes", "1")
		if code != 1 || err != nil || !strings.Contains(stdout, `"rule": "ask-superseded"`) {
			t.Errorf("%s --json: %d, %v, %q", test.status, code, err, stdout)
		}
	}
	text := strings.Replace(oldRecord, "status: accepted", "status: deprecated", 1)
	p, tracker := over(t, map[string]string{decisions + "/0001-old.md": text}, map[string]string{
		asks:                                    questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\""),
		filepath.Join(decisions, "0001-old.md"): text,
	})
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T", "--supersedes", "1"); code != 0 || err != nil {
		t.Errorf("a deprecated record: %d, %v", code, err)
	}
	if got := doc(t, tracker, filepath.Join(decisions, "0001-old.md")); !strings.Contains(got, "status: superseded by ADR-0002") {
		t.Errorf("the deprecated record is %q", got)
	}
}

// A commit refused, or an error writing, removes the folders made for the
// record, every file as it was.
func TestRecordRefusedUnmakes(t *testing.T) {
	p, tracker := over(t, nil, map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")})
	tracker.Refuse = true
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 1 || err != nil {
		t.Errorf("refused: %d, %v", code, err)
	}
	if _, err := os.Stat("docs"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused record left its folder: %v", err)
	}
	tracker.Refuse = false
	tracker.Unready = errors.New("no hooks")
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || err == nil || err.Error() != "no hooks" {
		t.Errorf("unready: %d, %v", code, err)
	}
	if _, err := os.Stat("docs"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a record that failed left its folder: %v", err)
	}
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || err == nil {
		t.Errorf("unready, docs there: %d, %v", code, err)
	}
	if _, err := os.Stat("docs"); err != nil {
		t.Errorf("a failed record removed a folder it did not make: %v", err)
	}
	if _, err := os.Stat(decisions); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed record left the folder it made: %v", err)
	}
}

// The records' folder that cannot be made, a folder it cannot list, and an
// index it cannot load are the errors, nothing written.
func TestRecordErrors(t *testing.T) {
	committed := map[string]string{asks: questions("id: q-1\nquestion: \"Q?\"\nanswer: \"A.\"")}
	p, tracker := over(t, nil, committed)
	// A file where the folder above the records' would be.
	if err := os.WriteFile("docs", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || err == nil {
		t.Errorf("a folder that cannot be made: %d, %v", code, err)
	}
	if text := doc(t, tracker, asks); strings.Contains(text, "decision:") {
		t.Errorf("the questions are %q", text)
	}
	broken := errors.New("broken")
	p, tracker = over(t, nil, committed)
	p.tracker = unloadable{tracker, broken, "README.md"}
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || !errors.Is(err, broken) {
		t.Errorf("an index that cannot be loaded: %d, %v", code, err)
	}
	p, tracker = over(t, nil, committed)
	p.tracker = unlistable{tracker, broken}
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || !errors.Is(err, broken) {
		t.Errorf("a folder the tracker cannot list: %d, %v", code, err)
	}
	src, err := treefake.Trees{Files: map[string]map[string]string{"worktree": {decisions + "/0001-old.md": oldRecord}}}.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	p, _ = withSource(t, unreadable{src, broken}, committed)
	if code, _, _, err := call(t, p, "record", "q-1", "--title", "T"); code != 0 || !errors.Is(err, broken) {
		t.Errorf("a records' folder that cannot be read: %d, %v", code, err)
	}
}

// unlistable is a tracker that cannot list a folder.
type unlistable struct {
	*trackerfake.Tracker
	err error
}

func (u unlistable) List(context.Context, string) ([]string, error) { return nil, u.err }

// unreadable is a working tree whose folders cannot be listed.
type unreadable struct {
	treeport.Source
	err error
}

func (u unreadable) List(context.Context, string) ([]string, error) { return nil, u.err }
