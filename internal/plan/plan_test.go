package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/ledger"
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
  nightly: { steps: [{ tests: scenario, whole: true }, { tasks: done, cost: static }] }
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

// The nightly: its steps in written order, the tasks step only the static
// checks, nothing left to the nightly by the nightly.
func TestMakeNightly(t *testing.T) {
	p, err := Make(load(t, scratch), Input{Nightly: true, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	want := "late  e2e\nstatic  T-1: quick check   (run)\n"
	if got := lines(p); got != want || strings.Join(p.Tasks, ",") != "T-1" {
		t.Errorf("plan:\n%s\ntasks %v", got, p.Tasks)
	}
}

// Bug 28: in a push, a named task's after: push check is pending, whatever
// would otherwise merge it, cover it or leave it out, and the kind's run
// takes no subset from it; the nightly runs it as any other.
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
	nightly, err := Make(load(t, scratch), Input{Nightly: true, Tasks: []ledger.Task{pushed}})
	if err != nil {
		t.Fatal(err)
	}
	if len(nightly.Checks) != 1 {
		t.Fatalf("the nightly plans %d checks", len(nightly.Checks))
	}
	for _, c := range nightly.Checks {
		if c.Action != Run {
			t.Errorf("the nightly leaves %s pending", c.Check.Command())
		}
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
