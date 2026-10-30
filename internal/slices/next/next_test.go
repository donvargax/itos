package next

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the next command's ports over the fakes: a config or the error
// loading it, and the tracker's fake.
type ports struct {
	cfg     *config.Loaded
	err     error
	tracker trackerport.Tracker
}

func (p ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

// configText names the registry, the people, the ledger with its id and the
// scenario kind under features/.
const configText = `version: 1
ledger:
  files: "tasks/phase-{group}.yaml"
  id: "T-\\d+"
work:
  registry: work-items.yaml
  people: { source: yaml, file: people.yaml }
tests:
  scenario:
    adapter: gherkin
    root: features
    id: "ID-[A-Z]+-\\d+"
    tag_prefix: "@"
    wip_tag: "@wip"
`

// bareConfig declares no scenario kind and no ledger.id.
const bareConfig = "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n"

// fixture is what a test's repository holds: the config's text, the
// working tree's files by path, HEAD's files by path (nil for a repository
// with no commit) and the questions ("" for no file).
type fixture struct {
	config    string
	files     map[string]string
	head      map[string]string
	questions string
}

// over is the ports of a repository whose top is an empty folder the test
// stands in, holding f.
func over(t *testing.T, f fixture) ports {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	common := filepath.Join(dir, ".git")
	v := &vcsfake.VCS{Root: dir, Located: map[string]vcsport.Location{"": {Top: dir, GitDir: common, Common: common}}}
	files := map[string]map[string]string{"worktree": f.files}
	if f.head != nil {
		files["HEAD"] = f.head
	}
	trees := treefake.Trees{Common: common, Files: files}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	text := configText
	if f.config != "" {
		text = f.config
	}
	cfg, err := config.FromText(v, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &trackerfake.Tracker{}
	if f.questions != "" {
		change := trackerport.Change{Name: cfg.Work.Asks, Text: f.questions, Created: true}
		if _, err := tracker.Transact(t.Context(), trackerport.Transaction{Changes: []trackerport.Change{change},
			Paragraphs: []string{"chore: start"}, Quiet: true}); err != nil {
			t.Fatal(err)
		}
	}
	return ports{cfg: cfg, tracker: tracker}
}

// run is the next command's line over p: its exit code, stdout, stderr
// and error.
func run(t *testing.T, p ports, args ...string) (int, string, string, error) {
	t.Helper()
	json := len(args) > 0 && args[0] == "--json"
	if json {
		args = args[1:]
	}
	c, err := Parse(args, json)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// item is a registry line of one item.
func item(fields string) string { return "  - { " + fields + " }\n" }

// registryOf is a registry of the items.
func registryOf(items ...string) string {
	return "phases: { 1: null }\nitems:\n" + strings.Join(items, "")
}

// feature is a feature file of one scenario tagged with the tags.
func feature(tags string) string {
	return "Feature: A\n\n  " + tags + "\n  Scenario: One\n    When it runs\n"
}

func TestParse(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "next needs one <id>"},
		{[]string{"a", "b"}, "next takes one <id>: a b"},
		{[]string{"a", "--patch"}, "next does not take --patch"},
	} {
		_, err := Parse(test.args, false)
		var usage out.UsageError
		if !errors.As(err, &usage) || err.Error() != test.want {
			t.Errorf("Parse(%q) = %v, want the usage error %q", test.args, err, test.want)
		}
	}
	if c, err := Parse([]string{"slice-9"}, true); err != nil || c != (CLI{Query: Query{ID: "slice-9"}, JSON: true}) {
		t.Errorf("Parse slice-9 = %+v, %v", c, err)
	}
}

// facts is Facts of an item written as a registry line, of the kind, with
// the dependencies, the open questions, the scenarios and the checks given.
func facts(t *testing.T, line, kind string, deps []Dependency, questions []string, scenarios []tests.Test, checks []ledger.Check) Facts {
	t.Helper()
	raw, err := value.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	f := Facts{Item: raw.(*value.Map), Kind: kind, Dependencies: deps, Scenarios: scenarios, Checks: checks}
	for _, q := range questions {
		f.Questions = append(f.Questions, ask.Question{ID: q, Item: value.String(f.Item.At("id")), Question: "Which?"})
	}
	return f
}

var (
	wip  = []tests.Test{{ID: "@ID-A-01", File: "a.feature"}}
	live = []tests.Test{{ID: "@ID-A-01", File: "a.feature", Live: true}}
	one  = []ledger.Check{{Run: ptr("true")}}
)

func ptr(s string) *string { return &s }

// The first state that holds wins, in the order done, parked, blocked (an
// open question before the registry's status), waiting, idea, needs-spec,
// in-flight; each gives its actor, action, argv, waits_on and reason.
func TestJudge(t *testing.T) {
	undone := []Dependency{{ID: "slice-7", Status: "done"}, {ID: "slice-8", Status: "todo"}, {ID: "gone", Status: nil}}
	for _, test := range []struct {
		name  string
		facts Facts
		want  Next
	}{
		{"done, whatever else holds",
			facts(t, "{ id: slice-9, owner: ana, status: done }", "slice", undone, []string{"q-1"}, nil, nil),
			Next{Item: "slice-9", Kind: "slice", State: "done", Owner: "ana", Actor: "none", Action: "none"}},
		{"dropped, with an open question",
			facts(t, "{ id: slice-9, owner: null, status: dropped }", "slice", nil, []string{"q-1"}, nil, nil),
			Next{Item: "slice-9", Kind: "slice", State: "parked", Actor: "none", Action: "none"}},
		{"deferred, with an undone dependency",
			facts(t, "{ id: slice-9, status: todo, depends_on: [slice-8], deferred: later }", "slice", undone, nil, nil, nil),
			Next{Item: "slice-9", Kind: "slice", State: "parked", Actor: "none", Action: "none"}},
		{"an open question wins over status blocked and a dependency",
			facts(t, "{ id: slice-9, status: blocked, blocked_by: [slice-7] }", "slice", undone, []string{"q-1", "q-3"}, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", WaitsOn: []string{"q-1", "q-3"}, BlockedReason: "decision"}},
		{"status blocked with a list waits on its ids, before a dependency",
			facts(t, "{ id: slice-9, status: blocked, blocked_by: [slice-7, T-1] }", "slice", undone, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", WaitsOn: []string{"slice-7", "T-1"}, BlockedReason: "status"}},
		{"status blocked with prose waits on nothing",
			facts(t, "{ id: slice-9, status: blocked, blocked_by: waiting on the vendor }", "slice", nil, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", BlockedReason: "status"}},
		{"status blocked with no blocked_by waits on nothing",
			facts(t, "{ id: slice-9, status: blocked }", "slice", nil, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", BlockedReason: "status"}},
		{"status blocked with an empty list waits on nothing",
			facts(t, "{ id: slice-9, status: blocked, blocked_by: [] }", "slice", nil, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", BlockedReason: "status"}},
		{"waiting on the dependencies not done, the spec present",
			facts(t, "{ id: slice-9, status: todo }", "slice", undone, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "waiting", Actor: "none", Action: "wait", WaitsOn: []string{"slice-8", "gone"}}},
		{"every dependency done is no wait",
			facts(t, "{ id: slice-9, status: todo }", "slice", []Dependency{{ID: "slice-7", Status: "done"}}, nil, live, nil),
			Next{Item: "slice-9", Kind: "slice", State: "in-flight", Actor: "forge", Action: "observe"}},
		{"an idea",
			facts(t, "{ id: p1-next, kind: idea, status: todo }", "idea", nil, nil, nil, nil),
			Next{Item: "p1-next", Kind: "idea", State: "idea", Actor: "person", Action: "promote"}},
		{"no kind is an idea's step",
			facts(t, "{ id: p1-next, status: todo }", "", nil, nil, nil, nil),
			Next{Item: "p1-next", State: "idea", Actor: "person", Action: "promote"}},
		{"a slice with no scenario needs a spec, doing or not",
			facts(t, "{ id: slice-9, owner: ana, status: doing }", "slice", nil, nil, nil, nil),
			Next{Item: "slice-9", Kind: "slice", State: "needs-spec", Owner: "ana", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}}},
		{"a bug with no scenario needs a spec",
			facts(t, "{ id: bug-3, status: todo }", "bug", nil, nil, nil, nil),
			Next{Item: "bug-3", Kind: "bug", State: "needs-spec", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "bug-3", "--role", "drafter"}}},
		{"a task with no check needs a spec",
			facts(t, "{ id: T-1, status: todo }", "task", nil, nil, nil, nil),
			Next{Item: "T-1", Kind: "task", State: "needs-spec", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "T-1", "--role", "drafter"}}},
		{"a wip scenario is a spec",
			facts(t, "{ id: slice-9, status: todo }", "slice", nil, nil, wip, nil),
			Next{Item: "slice-9", Kind: "slice", State: "in-flight", Actor: "forge", Action: "observe"}},
		{"a bug's scenario is a spec",
			facts(t, "{ id: bug-3, status: todo }", "bug", nil, nil, live, nil),
			Next{Item: "bug-3", Kind: "bug", State: "in-flight", Actor: "forge", Action: "observe"}},
		{"a task's check is a spec",
			facts(t, "{ id: T-1, status: todo }", "task", nil, nil, nil, one),
			Next{Item: "T-1", Kind: "task", State: "in-flight", Actor: "forge", Action: "observe"}},
	} {
		got := Judge(test.facts)
		if !digest.MatchString(got.Revision) {
			t.Errorf("%s: revision %q is not sha256: and 64 hex digits", test.name, got.Revision)
		}
		got.Revision = ""
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", test.name, got, test.want)
		}
	}
}

var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// The revision is the same for the same facts and another for a change to
// any of them: the entry, a dependency's status, an open question, a
// scenario, a check.
func TestRevision(t *testing.T) {
	base := func() Facts {
		return facts(t, "{ id: T-1, title: One, status: todo, depends_on: [T-0] }", "task",
			[]Dependency{{ID: "T-0", Status: "done"}}, []string{"q-1"}, live, one)
	}
	first := Revision(base())
	if first != Revision(base()) {
		t.Errorf("the same facts give %s, then %s", first, Revision(base()))
	}
	seen := map[string]string{"the same facts": first}
	for name, change := range map[string]func(f *Facts){
		"the entry":             func(f *Facts) { f.Item.Set("title", "Another") },
		"a dependency's status": func(f *Facts) { f.Dependencies[0].Status = "todo" },
		"a question gone":       func(f *Facts) { f.Questions = nil },
		"a question's id":       func(f *Facts) { f.Questions[0].ID = "q-2" },
		"a scenario gone":       func(f *Facts) { f.Scenarios = nil },
		"a scenario's liveness": func(f *Facts) { f.Scenarios = wip },
		"a check gone":          func(f *Facts) { f.Checks = nil },
		"a check's command":     func(f *Facts) { f.Checks = []ledger.Check{{Run: ptr("false")}} },
	} {
		f := base()
		change(&f)
		got := Revision(f)
		for other, revision := range seen {
			if got == revision {
				t.Errorf("%s changed gives the revision of %s, %s", name, other, got)
			}
		}
		seen[name] = got
	}
}

// Over a repository, the facts are read where their readers read them: the
// registry from the working tree, the scenarios at HEAD, a task's checks
// from the ledger, the questions from the tracker; the kind from the key,
// else the id.
func TestHandleReadsTheFacts(t *testing.T) {
	registry := registryOf(
		item("id: slice-9, title: Nine, phase: 1, owner: ana, status: todo, depends_on: []"),
		item("id: slice-8, title: Eight, phase: 1, owner: null, status: todo, depends_on: [slice-9, slice-7]"),
		item("id: slice-7, title: Seven, phase: 1, owner: null, status: done, depends_on: []"),
		item("id: bug-3, title: Three, phase: 1, owner: null, status: todo, depends_on: []"),
		item("id: T-1, title: One, phase: 1, owner: null, status: todo, depends_on: []"),
		item("id: T-2, title: Two, phase: 1, owner: null, status: todo, depends_on: []"),
		item("id: p1-x, title: X, phase: 1, owner: null, status: todo, depends_on: []"),
		item("id: slice-6, title: Six, phase: 1, owner: null, status: todo, depends_on: []"),
		item("id: slice-5, title: Five, phase: 1, owner: null, status: todo, depends_on: [gone, slice-7]"),
		item("id: T-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [], kind: idea"),
		item("id: slice-4, title: Four, phase: 1, owner: null, status: todo, depends_on: [unsaid]"),
		item("id: unsaid, title: Unsaid, phase: 1, owner: null, depends_on: []"),
	)
	ledgerText := "- { id: T-1, type: chore, title: One, done_when: [{ run: \"true\" }] }\n- { id: T-2, type: chore, title: Two }\n"
	p := over(t, fixture{
		files: map[string]string{"work-items.yaml": registry, "people.yaml": "- ana\n", "tasks/phase-1.yaml": ledgerText,
			"features/c.feature": feature("@ID-C-01 @slice-9")},
		head:      map[string]string{"features/a.feature": feature("@ID-A-01 @wip @slice-9"), "features/b.feature": feature("@ID-B-01 @slice-90")},
		questions: "questions:\n  - id: q-1\n    item: \"slice-6\"\n    question: \"Which way?\"\n    answer: \"This.\"\n  - id: q-2\n    item: \"slice-6\"\n    question: \"When?\"\n",
	})
	for _, test := range []struct {
		id   string
		want Next
	}{
		{"slice-9", Next{Item: "slice-9", Kind: "slice", State: "in-flight", Owner: "ana", Actor: "forge", Action: "observe"}},
		{"slice-8", Next{Item: "slice-8", Kind: "slice", State: "waiting", Actor: "none", Action: "wait", WaitsOn: []string{"slice-9"}}},
		{"bug-3", Next{Item: "bug-3", Kind: "bug", State: "needs-spec", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "bug-3", "--role", "drafter"}}},
		{"T-1", Next{Item: "T-1", Kind: "task", State: "in-flight", Actor: "forge", Action: "observe"}},
		{"T-2", Next{Item: "T-2", Kind: "task", State: "needs-spec", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "T-2", "--role", "drafter"}}},
		{"p1-x", Next{Item: "p1-x", State: "idea", Actor: "person", Action: "promote"}},
		{"slice-6", Next{Item: "slice-6", Kind: "slice", State: "blocked", Actor: "person", Action: "answer", WaitsOn: []string{"q-2"}, BlockedReason: "decision"}},
		{"slice-5", Next{Item: "slice-5", Kind: "slice", State: "waiting", Actor: "none", Action: "wait", WaitsOn: []string{"gone"}}},
		{"T-9", Next{Item: "T-9", Kind: "idea", State: "idea", Actor: "person", Action: "promote"}},
		{"slice-4", Next{Item: "slice-4", Kind: "slice", State: "waiting", Actor: "none", Action: "wait", WaitsOn: []string{"unsaid"}}},
	} {
		a, err := NewHandler(p).Handle(t.Context(), Query{ID: test.id})
		if err != nil || a.Refused != nil || a.Next == nil {
			t.Fatalf("%s: %+v, %v", test.id, a, err)
		}
		got := *a.Next
		got.Revision = ""
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", test.id, got, test.want)
		}
	}
}

// A task the ledger lacks has no checks; with no ledger.id, a T- id is no
// task and with no scenario kind a slice has no scenario: both need a spec,
// the T- id's kind none.
func TestHandleUnderABareConfig(t *testing.T) {
	p := over(t, fixture{config: bareConfig, files: map[string]string{
		"work-items.yaml": registryOf(item("id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: []"),
			item("id: T-1, title: One, phase: 1, owner: null, status: todo, depends_on: []")),
		"people.yaml": "- ana\n"}, head: map[string]string{"features/a.feature": feature("@ID-A-01 @slice-9")}})
	for id, want := range map[string]Next{
		"slice-9": {Item: "slice-9", Kind: "slice", State: "needs-spec", Actor: "drafter", Action: "specify", Argv: []string{"itos", "brief", "slice-9", "--role", "drafter"}},
		"T-1":     {Item: "T-1", State: "idea", Actor: "person", Action: "promote"},
	} {
		a, err := NewHandler(p).Handle(t.Context(), Query{ID: id})
		if err != nil || a.Next == nil {
			t.Fatalf("%s: %+v, %v", id, a, err)
		}
		got := *a.Next
		got.Revision = ""
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", id, got, want)
		}
	}
	p = over(t, fixture{files: map[string]string{
		"work-items.yaml": registryOf(item("id: T-3, title: Three, phase: 1, owner: null, status: todo, depends_on: []")),
		"people.yaml":     "- ana\n", "tasks/phase-1.yaml": "- { id: T-1, type: chore, title: One }\n"}})
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "T-3"})
	if err != nil || a.Next == nil || a.Next.Kind != "task" || a.Next.State != "needs-spec" {
		t.Errorf("a task the ledger lacks: %+v, %v, want a task needing a spec", a, err)
	}
}

// A registry that is not there, or an id no item has, is refused as work
// show refuses it, exit 1, on stderr or as --json's problems.
func TestRunRefuses(t *testing.T) {
	p := over(t, fixture{files: map[string]string{"people.yaml": "- ana\n"}})
	code, stdout, stderr, err := run(t, p, "slice-9")
	if code != 1 || err != nil || stdout != "" || !strings.Contains(stderr, "work-items.yaml") || !strings.Contains(stderr, "fix: ") {
		t.Errorf("no registry: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p = over(t, fixture{files: map[string]string{"work-items.yaml": registryOf(), "people.yaml": "- ana\n"}})
	code, stdout, stderr, err = run(t, p, "--json", "slice-9")
	if code != 1 || err != nil || stderr != "" {
		t.Fatalf("an unknown id: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	var report struct {
		Schema   int
		OK       bool
		Problems []out.Problem
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil || report.Schema != 1 || report.OK ||
		len(report.Problems) != 1 || report.Problems[0].Rule != "work-unknown-item" || !strings.Contains(report.Problems[0].Message, "slice-9") {
		t.Errorf("an unknown id --json: %v\n%s", err, stdout)
	}
	code, stdout, stderr, err = run(t, p, "slice-9")
	if code != 1 || err != nil || stdout != "" || !strings.Contains(stderr, "no item slice-9") {
		t.Errorf("an unknown id: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// What cannot be read is the error: the config, the registry, a task's
// ledger, the questions.
func TestRunErrors(t *testing.T) {
	broken := errors.New("no config")
	if _, _, _, err := run(t, ports{err: broken}, "slice-9"); !errors.Is(err, broken) {
		t.Errorf("a config that cannot be loaded: %v, want %v", err, broken)
	}
	files := map[string]string{"work-items.yaml": "items: [\n", "people.yaml": "- ana\n"}
	if _, _, _, err := run(t, over(t, fixture{files: files}), "slice-9"); err == nil {
		t.Error("a registry that is not YAML: no error")
	}
	files = map[string]string{"work-items.yaml": registryOf(item("id: T-1, title: One, phase: 1, owner: null, status: todo, depends_on: []")),
		"people.yaml": "- ana\n", "tasks/phase-1.yaml": "- [\n"}
	if _, _, _, err := run(t, over(t, fixture{files: files}), "T-1"); err == nil {
		t.Error("a ledger that is not YAML: no error")
	}
	files = map[string]string{"work-items.yaml": registryOf(item("id: slice-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: []")),
		"people.yaml": "- ana\n"}
	if _, _, _, err := run(t, over(t, fixture{files: files, questions: "questions: [\n"}), "slice-9"); err == nil {
		t.Error("questions that are not YAML: no error")
	}
	noID := strings.Replace(configText, `    id: "ID-[A-Z]+-\\d+"`+"\n", "", 1)
	if _, _, _, err := run(t, over(t, fixture{config: noID, files: files, head: map[string]string{}}), "slice-9"); err == nil ||
		!strings.Contains(err.Error(), "tests.scenario.id is missing") {
		t.Errorf("a scenario kind with no id, whose scenarios cannot be listed: %v", err)
	}
}

// The line, and --json's one object: its keys in order, argv, waits_on and
// blocked_reason only when the state has them, kind null for none.
func TestRunPrints(t *testing.T) {
	p := over(t, fixture{files: map[string]string{
		"work-items.yaml": registryOf(
			item("id: slice-9, title: Nine, phase: 1, owner: ana, status: todo, depends_on: []"),
			item("id: slice-8, title: Eight, phase: 1, owner: null, status: blocked, depends_on: [], blocked_by: [slice-9]"),
			item("id: p1-x, title: X, phase: 1, status: todo, depends_on: []")),
		"people.yaml": "- ana\n"}})
	code, stdout, stderr, err := run(t, p, "slice-9")
	if code != 0 || err != nil || stderr != "" || stdout != "slice-9: needs-spec; next: drafter specify; run: itos brief slice-9 --role drafter\n" {
		t.Errorf("slice-9: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = run(t, p, "slice-8")
	if code != 0 || err != nil || stderr != "" || stdout != "slice-8: blocked; next: person answer\n" {
		t.Errorf("slice-8: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	for id, want := range map[string]string{
		"slice-9": `{"schema":1,"ok":true,"item":"slice-9","kind":"slice","state":"needs-spec","owner":"ana",` +
			`"next":{"actor":"drafter","action":"specify","argv":["itos","brief","slice-9","--role","drafter"]},"revision":"<revision>"}`,
		"slice-8": `{"schema":1,"ok":true,"item":"slice-8","kind":"slice","state":"blocked","owner":null,` +
			`"next":{"actor":"person","action":"answer"},"waits_on":["slice-9"],"blocked_reason":"status","revision":"<revision>"}`,
		"p1-x": `{"schema":1,"ok":true,"item":"p1-x","kind":null,"state":"idea","owner":null,` +
			`"next":{"actor":"person","action":"promote"},"revision":"<revision>"}`,
	} {
		code, stdout, stderr, err := run(t, p, "--json", id)
		if code != 0 || err != nil || stderr != "" {
			t.Fatalf("%s --json: exit %d, %v, stderr %q", id, code, err, stderr)
		}
		got := revisionKey.ReplaceAllString(compact(t, stdout), `"revision":"<revision>"`)
		if got != want {
			t.Errorf("%s --json:\n got %s\nwant %s", id, got, want)
		}
	}
}

// revisionKey is the revision's key and digest in the JSON text.
var revisionKey = regexp.MustCompile(`"revision":"sha256:[0-9a-f]{64}"`)

// compact is the JSON text on one line, as written.
func compact(t *testing.T, text string) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(text)); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, text)
	}
	return b.String()
}
