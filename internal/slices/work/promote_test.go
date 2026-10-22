package work

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	"github.com/donvargax/itos/v8/internal/value"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// promotableText is a registry with two ideas, p1-b waiting on p1-a, a task,
// a done slice and a dropped idea.
const promotableText = `phases: { 1: null }
items:
  - { id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [], why: Adopt it. }
  - { id: p1-a, title: An idea, phase: 1, owner: null, status: todo, depends_on: [T-1], kind: idea, why: Because. }
  - { id: p1-b, title: Another, phase: 1, owner: null, status: todo, depends_on: [p1-a], kind: idea, why: B }
  - { id: slice-1, title: A slice, phase: 1, owner: b, status: done, depends_on: [] }
  - { id: p1-x, title: Dropped, phase: 1, owner: null, status: dropped, depends_on: [], kind: idea, why: X }
`

// ledgerFile is the ledger's one file, as the ledger names it.
const ledgerFile = "tasks/phase-1.yaml"

// promotable is a fixture whose registry is promotableText and whose ledger
// file the tracker holds as committed, under a stealth config when stealth.
func promotable(stealth bool) fixture {
	f := sound()
	f.files["work-items.yaml"] = promotableText
	f.tracked = []string{ledgerFile}
	f.stealth = stealth
	return f
}

// promoting is the ports over f, the id minted for the promote being id and
// the ids ItemID was given kept in given.
func promoting(t *testing.T, f fixture, id string) (ports, *[]string) {
	t.Helper()
	p := over(t, f)
	given := []string{}
	p.minted, p.given = id, &given
	return p, &given
}

// recorded is a tracker that keeps every transaction it is handed.
type recorded struct {
	trackerport.Tracker
	txs *[]trackerport.Transaction
}

func (r recorded) Transact(ctx context.Context, tx trackerport.Transaction) (string, error) {
	*r.txs = append(*r.txs, tx)
	return r.Tracker.Transact(ctx, tx)
}

// loadsThen is a tracker whose loads of one document fail after the first
// good ones.
type loadsThen struct {
	trackerport.Tracker
	name string
	good *int
}

func (l loadsThen) Load(ctx context.Context, names []string) ([]trackerport.Doc, error) {
	if names[0] == l.name {
		if *l.good == 0 {
			return nil, errors.New(l.name + " is gone")
		}
		*l.good--
	}
	return l.Tracker.Load(ctx, names)
}

// docIn is a document's text in p's tracker, "" when it is not there.
func docIn(t *testing.T, p ports, name string) string {
	t.Helper()
	docs, err := p.tracker.Load(t.Context(), []string{name})
	if err != nil {
		t.Fatal(err)
	}
	return docs[0].Text
}

// An idea promoted to a slice or a task: its line names what it became,
// the items and the questions naming it now and the commit; the registry,
// the questions and a task's ledger file committed together, the commit's
// body saying so.
func TestPromoteSaysWhatItDid(t *testing.T) {
	for _, c := range []struct {
		args          []string
		id, stdout    string
		files, body   string
		ledgerEntry   string
		registryNames string
	}{
		{[]string{"promote", "p1-a", "--kind", "slice"}, "slice-7",
			"p1-a is the slice slice-7, named now by p1-b, q-1, q-12: committed " + sha + " docs: promote p1-a to slice-7\n",
			"work-items.yaml asks.yaml", ", and rename it in q-1 and q-12 of asks.yaml.", "", "{ id: slice-7, title: An idea,"},
		{[]string{"promote", "p1-a", "--kind", "slice", "--title", "Renamed"}, "slice-7",
			"p1-a is the slice slice-7, named now by p1-b, q-1, q-12: committed " + sha + " docs: promote p1-a to slice-7\n",
			"work-items.yaml asks.yaml", " of asks.yaml.", "", "{ id: slice-7, title: Renamed,"},
		{[]string{"promote", "p1-b", "--kind", "slice"}, "slice-7",
			"p1-b is the slice slice-7: committed " + sha + " docs: promote p1-b to slice-7\n",
			"work-items.yaml", "with itos work promote.", "", "{ id: slice-7, title: Another,"},
		{[]string{"promote", "p1-a", "--kind", "task", "--type", "build", "--check", "make", "--timeout", "9"}, "T-2",
			"p1-a is the task T-2 of tasks/phase-1.yaml, named now by p1-b, q-1, q-12: committed " + sha + " docs: promote p1-a to T-2\n",
			"tasks/phase-1.yaml work-items.yaml asks.yaml",
			", and write its ledger entry, with its why and checks, in tasks/phase-1.yaml, and rename it in q-1 and q-12 of asks.yaml.",
			"- id: T-2\n  type: build\n  title: An idea\n  why: >\n    Because.\n  done_when:\n    - run: make\n      timeout: 9\n", "{ id: T-2, title: An idea,"},
		{[]string{"promote", "p1-b", "--kind", "task", "--type", "chore", "--check", "true", "--title", "Other", "--why", "Given."}, "T-2",
			"p1-b is the task T-2 of tasks/phase-1.yaml: committed " + sha + " docs: promote p1-b to T-2\n",
			"tasks/phase-1.yaml work-items.yaml", "in tasks/phase-1.yaml.",
			"- id: T-2\n  type: chore\n  title: Other\n  why: >\n    Given.\n  done_when:\n    - run: \"true\"\n", "{ id: T-2, title: Other,"},
	} {
		p, given := promoting(t, promotable(false), c.id)
		var txs []trackerport.Transaction
		p.tracker = recorded{Tracker: p.tracker, txs: &txs}
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != 0 || err != nil || stdout != c.stdout || stderr != "[2] docs: promote "+c.args[1]+" to "+c.id+"\n" {
			t.Errorf("work %q = %d, %v, %q, %q; want %q", c.args, code, err, stdout, stderr, c.stdout)
			continue
		}
		if want := []string{c.args[3], "T-1", "p1-a", "p1-b", "slice-1", "p1-x"}; !slices.Equal(*given, want) {
			t.Errorf("work %q gave ItemID %q", c.args, *given)
		}
		names := make([]string, len(txs[0].Changes))
		for i, change := range txs[0].Changes {
			names[i] = change.Name
		}
		if len(txs) != 1 || strings.Join(names, " ") != c.files {
			t.Errorf("work %q committed %q, want %q", c.args, names, c.files)
		}
		if body := strings.Join(strings.Fields(txs[0].Paragraphs[1]), " "); !strings.HasSuffix(body, c.body) {
			t.Errorf("work %q's body is %q, want it to end %q", c.args, body, c.body)
		}
		if registry := registryIn(t, p.inner()); !strings.Contains(registry, c.registryNames) {
			t.Errorf("work %q wrote the registry:\n%s", c.args, registry)
		}
		if ledger := docIn(t, p.inner(), ledgerFile); ledger != ledgerText+c.ledgerEntry {
			t.Errorf("work %q wrote the ledger:\n%s", c.args, ledger)
		}
		asks := docIn(t, p.inner(), "asks.yaml")
		if renamed := strings.Count(asks, "item: \""+c.id+"\""); renamed != strings.Count(c.stdout, "q-1") {
			t.Errorf("work %q wrote the questions:\n%s", c.args, asks)
		}
		p, _ = promoting(t, promotable(false), c.id)
		if code, stdout, stderr, err := run(t, p, append([]string{"-q"}, c.args...)...); code != 0 || err != nil || stdout != "" || stderr != "" {
			t.Errorf("work -q %q = %d, %v, %q, %q", c.args, code, err, stdout, stderr)
		}
	}
}

// inner is the ports with the tracker a recorded one wraps, for reading what it holds.
func (p ports) inner() ports {
	if r, ok := p.tracker.(recorded); ok {
		p.tracker = r.Tracker
	}
	return p
}

// Under --json a promote is one object: ok, the item, the id it was, the
// items rewritten, a task's entry and file, the questions renamed only
// when one named the idea, and the commit, null under a stealth config.
func TestPromoteAsJSON(t *testing.T) {
	p, _ := promoting(t, promotable(false), "slice-7")
	code, stdout, _, err := run(t, p, "--json", "promote", "p1-a", "--kind", "slice")
	o := object(t, stdout)
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if code != 0 || err != nil || o["ok"] != true || o["item"].(map[string]any)["id"] != "slice-7" || o["was"] != "p1-a" ||
		!slices.Equal(o["rewritten"].([]any), []any{"p1-b"}) || !slices.Equal(o["questions"].([]any), []any{"q-1", "q-12"}) ||
		o["commit"] != "0000000000000000000000000000000000000002" || strings.Join(keys, " ") != "commit item ok questions rewritten schema was" {
		t.Errorf("work --json promote of a slice = %d, %v, %v", code, err, o)
	}
	if !strings.HasPrefix(stdout, "{\n  \"schema\": 1,\n  \"ok\": true,\n  \"item\": ") || !strings.Contains(stdout, "\"was\": \"p1-a\",\n  \"rewritten\": [") ||
		!strings.Contains(stdout, "],\n  \"questions\": [") {
		t.Errorf("work --json promote's keys are out of order:\n%s", stdout)
	}
	p, _ = promoting(t, promotable(false), "T-2")
	code, stdout, _, err = run(t, p, "--json", "promote", "p1-b", "--kind", "task", "--type", "build", "--check", "true")
	o = object(t, stdout)
	if code != 0 || err != nil || o["task"].(map[string]any)["id"] != "T-2" || o["file"] != ledgerFile ||
		len(o["rewritten"].([]any)) != 0 || o["questions"] != nil || o["commit"] == nil {
		t.Errorf("work --json promote of a task = %d, %v, %v", code, err, o)
	}
	if !strings.Contains(stdout, "\"rewritten\": [],\n  \"task\": {") || !strings.Contains(stdout, "\"file\": \"tasks/phase-1.yaml\",\n  \"commit\": ") {
		t.Errorf("work --json promote of a task's keys are out of order:\n%s", stdout)
	}
	p, _ = promoting(t, promotable(true), "T-2")
	code, stdout, _, err = run(t, p, "--json", "promote", "p1-a", "--kind", "task", "--type", "build", "--check", "true")
	if o = object(t, stdout); code != 0 || err != nil || o["commit"] != nil || o["file"] != ledgerFile {
		t.Errorf("work --json promote under a stealth config = %d, %v, %v", code, err, o)
	}
	p, _ = promoting(t, promotable(false), "slice-7")
	parsed, err := Parse([]string{"promote", "p1-a", "--kind", "slice"}, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := parsed.Run(t.Context(), failing{}, failing{}, NewHandler(p)); err == nil || err.Error() != "the writer is closed" {
		t.Errorf("work --json promote to a closed writer = %d, %v", code, err)
	}
}

// Under a stealth config nothing is committed, and the line says which
// files are the config's: the ledger's too for a task.
func TestPromoteUnderAStealthConfig(t *testing.T) {
	for _, c := range []struct {
		args       []string
		id, stdout string
	}{
		{[]string{"promote", "p1-a", "--kind", "task", "--type", "build", "--check", "true"}, "T-2",
			"p1-a is the task T-2 of tasks/phase-1.yaml, named now by p1-b, q-1, q-12: the ledger and the registry are the stealth config's, so nothing is committed\n"},
		{[]string{"promote", "p1-b", "--kind", "slice"}, "slice-7",
			"p1-b is the slice slice-7: the registry is the stealth config's, so nothing is committed\n"},
	} {
		p, _ := promoting(t, promotable(true), c.id)
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != 0 || err != nil || stdout != c.stdout || stderr != "" {
			t.Errorf("work %q under a stealth config = %d, %v, %q, %q", c.args, code, err, stdout, stderr)
		}
		if !strings.Contains(registryIn(t, p), "id: "+c.id) {
			t.Errorf("work %q under a stealth config wrote:\n%s", c.args, registryIn(t, p))
		}
	}
}

// What a promote refuses before the id is reserved writes nothing and asks
// ItemID nothing, exit 1: the idea (Promotable), questions that cannot be
// read as itos decision writes them, the task's ledger entry and a file
// with changes no commit holds; under --json one object.
func TestPromoteRefusesBeforeMinting(t *testing.T) {
	typed := promotable(false)
	typed.config = strings.Replace(configText, "commits:\n", "commits:\n  types: [build, chore]\n", 1)
	badAsks := promotable(false)
	badAsks.questions = "questions: [\n"
	slice := []string{"promote", "p1-a", "--kind", "slice"}
	task := []string{"promote", "p1-a", "--kind", "task", "--type", "build", "--check", "true"}
	for _, c := range []struct {
		f      fixture
		args   []string
		edit   string
		stderr string
	}{
		{promotable(false), []string{"promote", "nope", "--kind", "slice"}, "", "no item nope in the registry\n  fix: itos work list --all names every item\n"},
		{promotable(false), []string{"promote", "p1-x", "--kind", "slice"}, "", "p1-x is dropped, taken out of the open work\n"},
		{promotable(false), []string{"promote", "T-1", "--kind", "slice"}, "", "T-1 has no kind, not an idea: only an idea is promoted\n"},
		{badAsks, slice, "", "\n  fix: correct asks.yaml to what itos decision writes, then promote p1-a again\n"},
		{typed, []string{"promote", "p1-a", "--kind", "task", "--type", "docs", "--check", "true"}, "", "docs is not a commit type: commits.types lists build, chore\n"},
		{promotable(false), slice, "asks.yaml", "asks.yaml has changes no commit holds"},
		{promotable(false), task, ledgerFile, "tasks/phase-1.yaml has changes no commit holds"},
	} {
		p, given := promoting(t, c.f, "slice-7")
		if c.edit != "" {
			p.tracker.(*trackerfake.Tracker).Edit(c.edit, docIn(t, p, c.edit)+"# edited\n")
		}
		before := docIn(t, p, ledgerFile)
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != refused || err != nil || stdout != "" || !strings.HasSuffix(stderr, c.stderr) && !strings.Contains(stderr, c.stderr) {
			t.Errorf("work %q = %d, %v, %q, %q; want %q", c.args, code, err, stdout, stderr, c.stderr)
		}
		if len(*given) != 0 || registryIn(t, p) != promotableText || docIn(t, p, ledgerFile) != before {
			t.Errorf("work %q refused asked ItemID %q or wrote:\n%s", c.args, *given, registryIn(t, p))
		}
		p, _ = promoting(t, c.f, "slice-7")
		if c.edit != "" {
			p.tracker.(*trackerfake.Tracker).Edit(c.edit, docIn(t, p, c.edit)+"# edited\n")
		}
		code, stdout, stderr, err = run(t, p, append([]string{"--json"}, c.args...)...)
		if o := object(t, stdout); code != refused || err != nil || stderr != "" || o["ok"] != false || len(o["problems"].([]any)) != 1 {
			t.Errorf("work --json %q = %d, %v, %v, %q", c.args, code, err, o, stderr)
		}
	}
}

// What can only be refused once the id is known is refused after the
// reservation, nothing written: an id an item already has and one ledger.id
// does not match, by work.Promote, and one the ledger already has, by
// ledger.Add.
func TestPromoteRefusesAfterMinting(t *testing.T) {
	ledgered := promotable(false)
	ledgered.files[ledgerFile] = ledgerText + "- { id: T-5, type: chore, title: Five }\n"
	task := []string{"promote", "p1-a", "--kind", "task", "--type", "build", "--check", "true"}
	for _, c := range []struct {
		f          fixture
		args       []string
		id, stderr string
	}{
		{promotable(false), []string{"promote", "p1-a", "--kind", "slice"}, "slice-1", "slice-1 is already an item: A slice\n  fix: give the slice an id no item has\n"},
		{promotable(false), task, "nine", "nine is not a task ID by ledger.id (^(?:T-\\d+)$)\n"},
		{ledgered, task, "T-5", "T-5 is already a task of the ledger, in tasks/phase-1.yaml: Five\n"},
	} {
		p, given := promoting(t, c.f, c.id)
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != refused || err != nil || stdout != "" || !strings.HasPrefix(stderr, c.stderr) {
			t.Errorf("work %q minting %s = %d, %v, %q, %q; want %q", c.args, c.id, code, err, stdout, stderr, c.stderr)
		}
		if len(*given) == 0 || registryIn(t, p) != promotableText || docIn(t, p, ledgerFile) != c.f.files[ledgerFile] {
			t.Errorf("work %q minting %s wrote:\n%s", c.args, c.id, registryIn(t, p))
		}
	}
}

// What ends a promote as an error: a ledger with no folder and the hooks
// not ready, before the id is reserved; a reservation that fails, a
// registry not editable in place (a usage error), a ledger that cannot be
// read once the id is reserved, questions that cannot be read, a commit
// that fails; a commit refused exits 1, every file as it was.
func TestPromoteEnds(t *testing.T) {
	task := []string{"promote", "p1-a", "--kind", "task", "--type", "build", "--check", "true"}
	slice := []string{"promote", "p1-a", "--kind", "slice"}
	noLedger := promotable(false)
	delete(noLedger.files, ledgerFile)
	noLedger.tracked = nil
	p, given := promoting(t, noLedger, "T-2")
	if code, _, _, err := run(t, p, task...); code != 0 || err == nil || !strings.Contains(err.Error(), "the ledger's folder tasks does not exist") || len(*given) != 0 {
		t.Errorf("work %q with no ledger folder = %d, %v, given %q", task, code, err, *given)
	}
	p, given = promoting(t, promotable(false), "slice-7")
	p.tracker.(*trackerfake.Tracker).Unready = errors.New("no hooks")
	if code, _, _, err := run(t, p, slice...); code != 0 || err == nil || err.Error() != "no hooks" || len(*given) != 0 {
		t.Errorf("work %q with the hooks not ready = %d, %v, given %q", slice, code, err, *given)
	}
	p, _ = promoting(t, promotable(false), "slice-7")
	p.mintErr = errors.New("no counter")
	if code, stdout, _, err := run(t, p, slice...); code != 0 || err == nil || err.Error() != "no counter" || stdout != "" || registryIn(t, p) != promotableText {
		t.Errorf("work %q with a reservation that fails = %d, %v, %q", slice, code, err, stdout)
	}
	flow := promotable(false)
	flow.files["work-items.yaml"] = "{ phases: { 1: null }, items: [{ id: p1-a, title: An idea, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }] }\n"
	p, _ = promoting(t, flow, "slice-7")
	if code, _, _, err := run(t, p, slice...); code != 0 || kind.Of(err) != kind.Usage || !strings.HasPrefix(err.Error(), "work-items.yaml cannot be edited in place: ") {
		t.Errorf("work %q of a flow registry = %d, %v", slice, code, err)
	}
	p, _ = promoting(t, promotable(false), "T-2")
	good := 1
	p.tracker = loadsThen{Tracker: p.tracker, name: ledgerFile, good: &good}
	if code, _, _, err := run(t, p, task...); code != 0 || err == nil || err.Error() != "tasks/phase-1.yaml is gone" || good != 0 {
		t.Errorf("work %q with a ledger gone once the id is reserved = %d, %v", task, code, err)
	}
	p, given = promoting(t, promotable(false), "slice-7")
	p.tracker = tracked{Tracker: p.tracker, load: errors.New("no read")}
	if code, _, _, err := run(t, p, slice...); code != 0 || err == nil || err.Error() != "no read" || len(*given) != 0 {
		t.Errorf("work %q with a registry that cannot be read = %d, %v", slice, code, err)
	}
	p, given = promoting(t, promotable(false), "slice-7")
	none := 0
	p.tracker = loadsThen{Tracker: p.tracker, name: "asks.yaml", good: &none}
	if code, _, _, err := run(t, p, slice...); code != 0 || err == nil || err.Error() != "asks.yaml is gone" || len(*given) != 0 {
		t.Errorf("work %q with questions that cannot be read = %d, %v", slice, code, err)
	}
	p, _ = promoting(t, promotable(false), "T-2")
	p.tracker = tracked{Tracker: p.tracker, transact: errors.New("no commit")}
	if code, stdout, _, err := run(t, p, task...); code != 0 || err == nil || err.Error() != "no commit" || stdout != "" {
		t.Errorf("work %q with a commit that fails = %d, %v, %q", task, code, err, stdout)
	}
	p, _ = promoting(t, promotable(false), "T-2")
	p.tracker.(*trackerfake.Tracker).Refuse = true
	code, stdout, stderr, err := run(t, p, task...)
	if code != refused || err != nil || stdout != "" || !strings.HasPrefix(stderr, "refused\n") ||
		!strings.Contains(stderr, "the commit of tasks/phase-1.yaml, work-items.yaml and asks.yaml failed (git exited 1), so tasks/phase-1.yaml, work-items.yaml and asks.yaml are as they were\n") ||
		registryIn(t, p) != promotableText || docIn(t, p, ledgerFile) != ledgerText {
		t.Errorf("work %q with a commit refused = %d, %v, %q, %q", task, code, err, stdout, stderr)
	}
}

// promotedTask refuses an idea with no group, and one with no why when the
// line gives none; the line's --why stands in for the idea's, and a group
// is the idea's phase as a string or a number.
func TestPromotedTaskNeedsAGroupAndAWhy(t *testing.T) {
	p := over(t, promotable(false))
	registry, err := domain.Load(p.cfg, p.cfg.Work.Registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		idea           *value.Map
		why, rule, got string
	}{
		{value.NewMap("id", "p1-a", "title", "Idea", "why", "Specified"), "", "work-promote-no-group", ""},
		{value.NewMap("id", "p1-a", "title", "Idea", "phase", 1.0), "", "work-promote-no-why", ""},
		{value.NewMap("id", "p1-a", "title", "Idea", "phase", 1.0, "why", "  "), "", "work-promote-no-why", ""},
		{value.NewMap("id", "p1-a", "title", "Idea", "phase", 1.0), "Given.", "", "Given."},
		{value.NewMap("id", "p1-a", "title", "Idea", "phase", "1", "why", " Its own. "), "", "", "Its own."},
	} {
		n, added, found, err := promotedTask(p.cfg, registry, c.idea, Promotion{ID: "p1-a", Kind: "task", Type: "build", Why: c.why}, p.tracker.Load)
		switch {
		case err != nil:
			t.Errorf("promotedTask(%v) error: %v", c.idea, err)
		case c.rule == "" && (len(found) != 0 || n.Group != "1" || n.ID != "T-2" || n.Title != "Idea" || added.Path != ledgerFile ||
			n.Why != c.got):
			t.Errorf("promotedTask(%v) = %+v, %+v, %+v, want the task in group 1", c.idea, n, added, found)
		case c.rule != "" && (len(found) != 1 || found[0].Rule != c.rule || !strings.HasPrefix(found[0].Message, "p1-a ")):
			t.Errorf("promotedTask(%v) refused %+v, want %s", c.idea, found, c.rule)
		}
	}
	_, _, found, _ := promotedTask(p.cfg, registry, value.NewMap("id", "p1-a"), Promotion{ID: "p1-a"}, p.tracker.Load)
	if len(found) != 1 || found[0].Message != "p1-a names no phase, so its task has no ledger file" ||
		found[0].Fix != "give p1-a a phase in work-items.yaml, then promote it again" {
		t.Errorf("promotedTask of an idea with no group = %+v", found)
	}
	_, _, found, _ = promotedTask(p.cfg, registry, value.NewMap("id", "p1-a", "phase", 1.0), Promotion{ID: "p1-a"}, p.tracker.Load)
	if len(found) != 1 || found[0].Message != "p1-a has no why for its task's ledger entry" || found[0].Fix != "pass --why <why>" {
		t.Errorf("promotedTask of an idea with no why = %+v", found)
	}
}
