package plan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
)

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

const scratch = `version: 1
tests:
  scenario:
    root: features
    run: { whole: "e2e", select: "e2e --grep {pattern}", ids_pattern: "@(?:{ids})" }
    recognize: [{ command: "e2e --grep {pattern}", as: pattern }]
ci:
  steps:
    - { run: lint, cost: static }
    - unit
    - { tests: scenario }
  prose: { paths: ["**/*.md"], steps: [lint] }
  cost: { static: ["^lint$", "^quick\\b"], keep_written_order: true }
  covers: [{ by: unit, matches: "^unit \\S+$" }]
  nightly_only: [slow]
  nightly: { steps: [{ tests: scenario, whole: true }, { run: quick check, cost: static }] }
`

func text(s string) *string { return &s }

var task = ledger.Task{ID: "T-1", Title: "One", DoneWhen: []ledger.Check{
	{Run: text("quick check")},
	{Run: text("e2e --grep '@ID-A-'")},
	{Run: text("unit a.test.ts")},
	{Run: text("slow")},
	{Run: text("docs"), Prose: true},
	{Run: text("quick again")},
}}

// lines are a plan's order as `ci plan` prints it.
func lines(p *Plan) string {
	var b strings.Builder
	p.Print(&b)
	return b.String()
}

// A push: the static step and check first, the late steps with the kind's
// run merging the task's subset, then the late checks, covered, left to the
// nightly or run, the static check below a late one late by order.
func TestMakePush(t *testing.T) {
	p, err := Make(load(t, scratch), Input{Known: true, Smoke: []string{"ID-S-01"}, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	want := `static  lint
static  T-1: quick check   (run)
late  unit
late  e2e --grep '@(?:ID-S-01)|@ID-A-'
late  T-1: e2e --grep '@ID-A-'   (merged)
late  T-1: unit a.test.ts   (covered)
late  T-1: slow   (nightly)
late  T-1: docs   (run)
late  T-1: quick again   (run)
`
	if got := lines(p); got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
	if got := p.Order[7].Check.From; got != "default" {
		t.Errorf("docs' cost from %q", got)
	}
	if got := p.Order[8].Check.From; got != "order" {
		t.Errorf("quick again's cost from %q", got)
	}
}

// Prose: the prose steps, the static and prose: true checks; the rest left
// out, and the subset with them, so no run of the kind.
func TestMakeProse(t *testing.T) {
	p, err := Make(load(t, scratch), Input{Prose: true, Known: true, Smoke: []string{"ID-S-01"}, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	want := "static  lint\nstatic  T-1: quick check   (run)\nlate  T-1: docs   (run)\n"
	if got := lines(p); got != want || len(p.LeftOut) != 4 {
		t.Errorf("plan:\n%s\nleft out %d", got, len(p.LeftOut))
	}
}

// ci.keep_step_order (issue #13): the steps as written, then the task's
// checks in their written order, each still with its cost class; a prose
// range keeps the same order over what it runs.
func TestMakeKeepStepOrder(t *testing.T) {
	cfg := load(t, strings.Replace(scratch, "    - { run: lint, cost: static }\n    - unit\n", "    - unit\n    - { run: lint, cost: static }\n", 1)+"  keep_step_order: true\n")
	p, err := Make(cfg, Input{Known: true, Smoke: []string{"ID-S-01"}, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	want := `late  unit
static  lint
late  e2e --grep '@(?:ID-S-01)|@ID-A-'
static  T-1: quick check   (run)
late  T-1: e2e --grep '@ID-A-'   (merged)
late  T-1: unit a.test.ts   (covered)
late  T-1: slow   (nightly)
late  T-1: docs   (run)
late  T-1: quick again   (run)
`
	if got := lines(p); got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
	prose, err := Make(cfg, Input{Prose: true, Known: true, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	want = "static  lint\nstatic  T-1: quick check   (run)\nlate  T-1: docs   (run)\n"
	if got := lines(prose); got != want {
		t.Errorf("prose plan:\n%s\nwant:\n%s", got, want)
	}
}

// The nightly: its steps in the order written, whatever their cost, and no
// task's checks (slice 107).
func TestMakeNightly(t *testing.T) {
	p := nightlyPlan(load(t, scratch))
	want := "late  e2e\nstatic  quick check\n"
	if got := lines(p); got != want || len(p.Tasks) != 0 || len(p.Checks) != 0 {
		t.Errorf("plan:\n%s\ntasks %v, checks %d", got, p.Tasks, len(p.Checks))
	}
	if got := strings.Join(p.Steps, ","); got != "e2e,quick check" {
		t.Errorf("steps %q", got)
	}
	if empty := nightlyPlan(load(t, "version: 1\n")); len(empty.Order) != 0 || len(empty.Steps) != 0 {
		t.Errorf("a config with no nightly plans %d items", len(empty.Order))
	}
}

// ForNightly plans the config's nightly steps alone, its range every commit
// up to HEAD.
func TestForNightly(t *testing.T) {
	p := ForNightly(load(t, scratch))
	if got := strings.Join(p.Steps, ","); got != "e2e,quick check" || p.Ends.From != "" || len(p.Checks) != 0 {
		t.Errorf("steps %q, from %q, checks %d", got, p.Ends.From, len(p.Checks))
	}
}

// Make stops at what it cannot read: a task whose checks it cannot, a config
// with no ci section, a kind of named tests the config does not have (when a
// check is read as one of its runs, and when only the run itself is), and a
// ci.covers pattern that does not compile.
func TestMakeStopsAtWhatItCannotRead(t *testing.T) {
	broken := ledger.Task{ID: "T-9", Err: errors.New("T-9 cannot be read")}
	if _, err := Make(load(t, scratch), Input{Tasks: []ledger.Task{broken}}); err == nil || err.Error() != "T-9 cannot be read" {
		t.Errorf("a task it cannot read: %v", err)
	}
	if _, err := Make(load(t, "version: 1\n"), Input{}); err == nil || !strings.Contains(err.Error(), "ci is missing") {
		t.Errorf("no ci section: %v", err)
	}
	if _, err := TestsKind(load(t, "version: 1\n")); err == nil {
		t.Error("TestsKind of a config with no ci section gave no error")
	}
	missing := func() *config.Loaded {
		cfg := load(t, scratch)
		cfg.CI.Steps[2].Tests = text("missing")
		return cfg
	}
	for name, in := range map[string]Input{
		"a check read as a run": {Known: true, Tasks: []ledger.Task{task}},
		"the run alone":         {Known: true},
	} {
		if _, err := Make(missing(), in); err == nil || !strings.Contains(err.Error(), "tests.missing is missing") {
			t.Errorf("%s, of a kind the config does not have: %v", name, err)
		}
	}
	cfg := load(t, scratch)
	cfg.CI.Covers[0].Matches = "("
	if _, err := Make(cfg, Input{Known: true, Tasks: []ledger.Task{task}}); err == nil {
		t.Error("a ci.covers pattern that does not compile gave no error")
	}
}

// A range it cannot read runs every test; a prose range whose static or
// prose: true check is a run of the kind runs the kind's run for that check
// alone.
func TestMakeUnreadAndProseRuns(t *testing.T) {
	unread, err := Make(load(t, scratch), Input{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(unread.Steps, ","); got != "lint,unit,e2e" {
		t.Errorf("an unread range's steps %q", got)
	}
	prose := ledger.Task{ID: "T-3", Title: "Three", DoneWhen: []ledger.Check{{Run: text("e2e --grep '@ID-C-'"), Prose: true}}}
	p, err := Make(load(t, scratch), Input{Prose: true, Known: true, Tasks: []ledger.Task{prose}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.Steps, ","); got != "lint,e2e --grep '@ID-C-'" || p.Checks[0].Action != Merged {
		t.Errorf("a prose range's steps %q, its check %s", got, p.Checks[0].Action)
	}
}

// Bug 28: in a push, a named task's after: push check is pending, whatever
// would otherwise merge it, cover it or leave it out, and the kind's run
// takes no subset from it.
func TestMakePushPending(t *testing.T) {
	pushed := ledger.Task{ID: "T-2", Title: "Two", DoneWhen: []ledger.Check{
		{Run: text("quick pushed"), After: "push"},
		{Run: text("e2e --grep '@ID-B-'"), After: "push"},
		{Run: text("unit b.test.ts"), After: "push"},
		{Run: text("slow"), After: "push"},
		{Run: text("release check"), After: "push"},
	}}
	p, err := Make(load(t, scratch), Input{Known: true, Smoke: []string{"ID-S-01"}, Tasks: []ledger.Task{pushed}})
	if err != nil {
		t.Fatal(err)
	}
	want := `static  lint
static  T-2: quick pushed   (pending: runs after the push)
late  unit
late  e2e --grep '@(?:ID-S-01)'
late  T-2: e2e --grep '@ID-B-'   (pending: runs after the push)
late  T-2: unit b.test.ts   (pending: runs after the push)
late  T-2: slow   (pending: runs after the push)
late  T-2: release check   (pending: runs after the push)
`
	if got := lines(p); got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
}

// A step's {from} and {to} are filled in after the plan is made (slice 83):
// its cost class and ci.covers read it as ci.steps writes it, and what ci
// plan prints, the covering step a check names included, is what the run
// runs.
func TestFillReadsTheStepAsWritten(t *testing.T) {
	cfg := load(t, `version: 1
ci:
  steps: ["lint {from}", "diff {from} {to}"]
  cost: { static: ["^lint \\{from\\}$"] }
`)
	covered := ledger.Task{ID: "T-1", DoneWhen: []ledger.Check{{Run: text("diff {from} {to}")}}}
	p, err := Make(cfg, Input{Known: true, Tasks: []ledger.Task{covered}})
	if err != nil {
		t.Fatal(err)
	}
	p.fill(Ends{From: "a1", To: "b2"})
	want := `static  lint 'a1'
late  diff 'a1' 'b2'
late  T-1: diff {from} {to}   (covered)
`
	if got := lines(p); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if p.Checks[0].CoveredBy != "diff 'a1' 'b2'" || p.Steps[0] != "lint 'a1'" || p.Ends.To != "b2" {
		t.Errorf("covered by %q, steps %q, ends %+v", p.Checks[0].CoveredBy, p.Steps, p.Ends)
	}
}

// The range's code proof (slice 106, issue #28): a step among the static
// ones, before the run of named tests, its command proof.code.check with
// {base} the range's start; and none of that with no proof.code, a range
// touching none of proof.code.paths, or a range with no start to judge
// from. For reads the paths a range touched, so it is codeProof's callers
// that pass them.
func TestMakeProof(t *testing.T) {
	proofCfg := load(t, scratch+`proof:
  code:
    paths: ["{cmd,internal}/**/*.go"]
    check: "cc mutation check --since {base} --fail-uncovered --json"
`)
	p, err := Make(proofCfg, Input{Known: true, Smoke: []string{"ID-S-01"}, Proof: &Step{
		Command: "cc mutation check --since a1 --fail-uncovered --json", Proof: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := `static  lint
static  cc mutation check --since a1 --fail-uncovered --json
late  unit
late  e2e --grep '@(?:ID-S-01)'
`
	if got := lines(p); got != want {
		t.Errorf("plan:\n%s\nwant:\n%s", got, want)
	}
	if !p.Order[1].Step.Proof || p.Order[1].Step.Cost != check.Static || p.Order[1].Step.From != check.FromExplicit {
		t.Errorf("the proof step: %+v", p.Order[1].Step)
	}
	// No run of named tests to sit before: written after the other steps, and
	// still static, so it runs before them.
	p, err = Make(load(t, "version: 1\nci:\n  steps: [lint]\n"),
		Input{Known: true, Proof: &Step{Command: "cc", Proof: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := lines(p); got != "static  cc\nlate  lint\n" {
		t.Errorf("plan:\n%s", got)
	}
}

// codeProof is the step a range's code proof is when the config has a
// proof.code and the range touches its paths, and none of that otherwise:
// no proof.code, no touched path, or no start to judge from.
func TestCodeProof(t *testing.T) {
	withProof := load(t, `version: 1
proof:
  code:
    paths: ["{cmd,internal}/**/*.go"]
    check: "cc mutation check --since {base} --fail-uncovered --json"
`)
	none := load(t, "version: 1\n")
	ends := Ends{From: "a1", To: "b2"}
	got, err := codeProof(withProof, []string{"internal/cli/x.go"}, ends)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Command != "cc mutation check --since a1 --fail-uncovered --json" || !got.Proof {
		t.Errorf("a touched range gives %+v", got)
	}
	for _, c := range []struct {
		name    string
		cfg     *config.Loaded
		changed []string
		ends    Ends
	}{
		{"no proof.code", none, []string{"internal/cli/x.go"}, ends},
		{"no touched path", withProof, []string{"README.md"}, ends},
		{"an unread range touched nothing", withProof, nil, ends},
		{"no start to judge from", withProof, []string{"internal/cli/x.go"}, Ends{To: "b2"}},
	} {
		got, err := codeProof(c.cfg, c.changed, c.ends)
		if err != nil || got != nil {
			t.Errorf("%s: got %+v, %v; want none", c.name, got, err)
		}
	}
	// A glob itos cannot read is config check's finding, before a plan reads
	// one (proof.Touches is the only other reading of the paths, and its own
	// error is unit-tested beside it).
}

// A range that runs everything, an empty or all-zeros start without
// commits.since, gives an empty start.
func TestEndsOfAnEmptyStart(t *testing.T) {
	cfg := load(t, "version: 1\n")
	for _, from := range []string{"", "0000000000000000000000000000000000000000"} {
		if e := EndsOf(cfg, from, ""); e.From != "" {
			t.Errorf("from %q gives %q", from, e.From)
		}
	}
}
