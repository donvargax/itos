package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/ledger"
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
