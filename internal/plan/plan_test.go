package plan

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/check"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/store/tree/adapter"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	file := filepath.Join(t.TempDir(), "itos.yaml")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(vcs.Git{}, adapter.Trees{VCS: vcs.Git{}}, adapter.Worktree, file)
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
static  cc mutation check --since a1 --fail-uncovered --json   (code proof)
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
	if got := lines(p); got != "static  cc   (code proof)\nlate  lint\n" {
		t.Errorf("plan:\n%s", got)
	}
}

// NamedTests cuts a plan to its run of named tests (slice 132): the merged
// run alone, with no other step, no task check and no code proof, the ends
// kept; nothing at all when the plan has no such run.
func TestNamedTests(t *testing.T) {
	proofCfg := load(t, scratch+"proof:\n  code:\n    paths: [\"**\"]\n    check: \"cc {base}\"\n")
	p, err := Make(proofCfg, Input{Known: true, Smoke: []string{"ID-S-01"}, Tasks: []ledger.Task{task},
		Proof: &Step{Command: "cc a1", Proof: true}})
	if err != nil {
		t.Fatal(err)
	}
	p.Ends = Ends{From: "a1", To: "b2"}
	cut := p.NamedTests()
	if got := lines(cut); got != "late  e2e --grep '@(?:ID-S-01)|@ID-A-'\n" {
		t.Errorf("cut plan:\n%s", got)
	}
	if got := strings.Join(cut.Steps, ","); got != "e2e --grep '@(?:ID-S-01)|@ID-A-'" || cut.Ends != p.Ends ||
		len(cut.Checks) != 0 || len(cut.Tasks) != 0 {
		t.Errorf("steps %q, ends %+v, checks %d, tasks %v", got, cut.Ends, len(cut.Checks), cut.Tasks)
	}
	none, err := Make(load(t, "version: 1\nci:\n  steps: [lint]\n"), Input{Known: true, Tasks: []ledger.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if cut := none.NamedTests(); len(cut.Order) != 0 || len(cut.Steps) != 0 {
		t.Errorf("a plan with no run of named tests cut to %d items", len(cut.Order))
	}
	if got := lines(nightlyPlan(load(t, scratch)).NamedTests()); got != "late  e2e\n" {
		t.Errorf("the nightly cut:\n%s", got)
	}
}

// codeProof is the step a range's code proof is when the config has a
// proof.code and the range touches its paths, and none of that otherwise,
// with the line that says why: for the first reason that holds, in this
// order, no start (the range as typed, whatever commits.since gives it, or
// no resolved start), a read that failed, no touched path (slice 155).
func TestCodeProof(t *testing.T) {
	withProof := load(t, `version: 1
proof:
  code:
    paths: ["{cmd,internal}/**/*.go"]
    check: "cc mutation check --since {base} --fail-uncovered --json"
`)
	none := load(t, "version: 1\n")
	ends := Ends{From: "a1", To: "b2"}
	const sha = "1111111111111111111111111111111111111111"
	touching := []string{"internal/cli/x.go"}
	got, why, err := codeProof(withProof, sha, touching, nil, ends)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Command != "cc mutation check --since a1 --fail-uncovered --json" || !got.Proof || why != "" {
		t.Errorf("a touched range gives %+v, %q", got, why)
	}
	failed := errors.New("the read failed")
	for _, c := range []struct {
		name    string
		cfg     *config.Loaded
		from    string
		changed []string
		readErr error
		ends    Ends
		want    string
	}{
		{"no proof.code", none, sha, touching, nil, ends, ""},
		{"no proof.code and no start", none, "", nil, nil, Ends{}, ""},
		{"no touched path", withProof, sha, []string{"README.md"}, nil, ends, proofNoPath},
		{"no changed path", withProof, sha, nil, nil, ends, proofNoPath},
		{"an unread range", withProof, sha, nil, failed, ends, proofUnreadable},
		{"an unread range with a path", withProof, sha, touching, failed, ends, proofUnreadable},
		{"no start typed", withProof, "", nil, nil, Ends{From: "a1", To: "b2"}, proofNoStart},
		{"an all-zeros start, which commits.since fills", withProof, "0000000000000000000000000000000000000000", nil, failed, ends, proofNoStart},
		{"no start resolved", withProof, sha, touching, nil, Ends{To: "b2"}, proofNoStart},
		{"no start resolved and an unread range", withProof, sha, nil, failed, Ends{To: "b2"}, proofNoStart},
	} {
		got, why, err := codeProof(c.cfg, c.from, c.changed, c.readErr, c.ends)
		if err != nil || got != nil || why != c.want {
			t.Errorf("%s: got %+v, %q, %v; want none and %q", c.name, got, why, err, c.want)
		}
	}
	// A glob itos cannot read is config check's finding, before a plan reads
	// one (proof.Touches is the only other reading of the paths, and its own
	// error is unit-tested beside it).
}

// gitRange is a scratch repository as the current folder, away from the
// user's git config and with no maintenance, holding two commits; it gives
// the range from the first to the second, which touches internal/x.go.
func gitRange(t *testing.T) (from, to string) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	for name, value := range map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_COUNT": "2",
		"GIT_CONFIG_KEY_0": "maintenance.auto", "GIT_CONFIG_VALUE_0": "false",
		"GIT_CONFIG_KEY_1": "gc.auto", "GIT_CONFIG_VALUE_1": "0",
		"GIT_AUTHOR_NAME": "itos", "GIT_AUTHOR_EMAIL": "itos@example.com",
		"GIT_COMMITTER_NAME": "itos", "GIT_COMMITTER_EMAIL": "itos@example.com",
	} {
		t.Setenv(name, value)
	}
	t.Chdir(t.TempDir())
	run := func(args ...string) string {
		t.Helper()
		got, err := exec.Command(git.Bin(runtime.GOOS), args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), got)
		}
		return strings.TrimSpace(string(got))
	}
	commit := func(path, message string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(message+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("add", "--", path)
		run("commit", "-q", "-m", message)
		return run("rev-parse", "HEAD")
	}
	run("init", "-q")
	return commit("README.md", "docs: start"), commit("internal/x.go", "feat: x")
}

// For stops at a proof.code glob it cannot read. Loading refuses one
// (config-proof-glob), so it is set after the load: For takes any config,
// not only one that loaded.
func TestForStopsAtAProofGlobItCannotRead(t *testing.T) {
	from, to := gitRange(t)
	cfg := load(t, scratch+`proof:
  code:
    paths: ["{cmd,internal}/**/*.go"]
    check: "cc mutation check --since {base} --fail-uncovered --json"
`)
	if p, err := For(cfg, from, to, Data{}); err != nil || p.Order[1].Step == nil || !p.Order[1].Step.Proof {
		t.Fatalf("the range's plan: %+v, %v", p, err)
	}
	cfg.Proof.Code.Paths = []string{"?src"}
	if p, err := For(cfg, from, to, Data{}); err == nil || p != nil {
		t.Errorf("a proof glob it cannot read gave %+v, %v", p, err)
	}
}

// logFails is a VCS whose nth Log fails.
type logFails struct {
	vcsport.VCS
	n     int
	calls *int
}

func (l logFails) Log(ctx context.Context, r vcsport.Range) ([]vcsport.Entry, error) {
	if *l.calls++; *l.calls == l.n {
		return nil, errors.New("git log gives a record itos cannot read")
	}
	return l.VCS.Log(ctx, r)
}

// For stops at a range git reads whose commits the VCS cannot log, for the
// tasks they name or the tests (bug-157): a plan naming neither would leave
// their checks out.
func TestForStopsAtALogItCannotRead(t *testing.T) {
	from, to := gitRange(t)
	cfg := load(t, scratch+"commits:\n  footers:\n    Task: { source: ledger }\n    Scenarios: { source: { tests: scenario } }\n")
	if _, err := For(cfg, from, to, Data{}); err != nil {
		t.Fatalf("the range's plan: %v", err)
	}
	for _, n := range []int{1, 2} {
		calls := 0
		cfg.VCS = logFails{VCS: vcs.Git{}, n: n, calls: &calls}
		if p, err := For(cfg, from, to, Data{}); err == nil || !strings.Contains(err.Error(), "cannot read") || p != nil {
			t.Errorf("log %d failing gave %+v, %v", n, p, err)
		}
	}
}

// A push's plan says whether the config's code proof is scheduled: its row
// marked, or the one line that says why not, last (slice 155). The ranges
// are real: one that touches the proof's paths, one that touches none, one
// the repository cannot diff, and the starts that are none, with
// commits.since given.
func TestForSaysWhyTheProofIsNotScheduled(t *testing.T) {
	from, to := gitRange(t)
	const base = scratch + `proof:
  code:
    paths: ["internal/**"]
    check: "cc since {base}"
`
	cfg := load(t, base)
	elsewhere := load(t, strings.Replace(base, "internal/**", "docs/**", 1))
	since := load(t, strings.Replace(base, "ci:\n", "commits:\n  since: "+from+"\nci:\n", 1))
	const missing = "1234567890123456789012345678901234567890"
	const zeros = "0000000000000000000000000000000000000000"
	for _, c := range []struct {
		name     string
		cfg      *config.Loaded
		from     string
		row, why string
	}{
		{"a touched path", cfg, from, "static  cc since " + from + "   (code proof)\n", ""},
		{"no touched path", elsewhere, from, "", proofNoPath},
		{"a range it cannot diff", cfg, missing, "", proofUnreadable},
		{"no start", cfg, "", "", proofNoStart},
		{"an all-zeros start", cfg, zeros, "", proofNoStart},
		{"a start only commits.since gives", since, "", "", proofNoStart},
		{"an all-zeros start, which commits.since fills", since, zeros, "", proofNoStart},
	} {
		p, err := For(c.cfg, c.from, to, Data{})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := lines(p)
		if c.row != "" && !strings.Contains(got, c.row) || c.row == "" && strings.Contains(got, "cc since") {
			t.Errorf("%s: the plan's rows:\n%s", c.name, got)
		}
		if c.why == "" {
			if p.ProofOmitted != "" {
				t.Errorf("%s: the proof is left out: %q", c.name, p.ProofOmitted)
			}
			continue
		}
		if p.ProofOmitted != c.why || !strings.HasSuffix(got, "\n"+c.why+"\n") || strings.Count(got, "Code proof") != 1 {
			t.Errorf("%s: want %q last, got %q:\n%s", c.name, c.why, p.ProofOmitted, got)
		}
	}
	// No proof.code, or the nightly: neither row nor line.
	if p, err := For(load(t, scratch), from, to, Data{}); err != nil || p.ProofOmitted != "" {
		t.Errorf("a config with no proof.code: %v", err)
	}
	if p := ForNightly(cfg); p.ProofOmitted != "" || strings.Contains(lines(p), "Code proof") {
		t.Errorf("the nightly: %q", lines(p))
	}
}

// ChangedErr is Changed with the error of a read that failed, none for a
// range with no start or end.
func TestChangedErrGivesTheFailureOfTheRead(t *testing.T) {
	from, to := gitRange(t)
	v := vcs.Git{}
	const missing = "1234567890123456789012345678901234567890"
	if paths, err := ChangedErr(v, from, to); err != nil || len(paths) != 1 || paths[0] != "internal/x.go" {
		t.Errorf("a readable range: %q, %v", paths, err)
	}
	if paths, err := ChangedErr(v, missing, to); err == nil || paths != nil {
		t.Errorf("a start the repository lacks: %q, %v", paths, err)
	}
	if got := Changed(v, missing, to); got != nil {
		t.Errorf("Changed gives %q for a range that cannot be read", got)
	}
	for _, r := range [][2]string{{"", to}, {from, ""}} {
		if paths, err := ChangedErr(v, r[0], r[1]); err != nil || paths != nil {
			t.Errorf("%q..%q: %q, %v", r[0], r[1], paths, err)
		}
	}
}

// The plan's JSON gives a step its command, cost and where its cost came
// from, and marks the range's code proof; a check its task, place, command
// and action.
func TestFieldsGiveEachItem(t *testing.T) {
	p, err := Make(load(t, scratch), Input{Known: true, Tasks: []ledger.Task{task},
		Proof: &Step{Command: "cc --since a1", Proof: true}})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := out.Emit(&b, p.Fields(Range{From: "a1", To: "b2", Readable: true})...); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Order []map[string]any
	}
	if err := json.Unmarshal([]byte(b.String()), &got); err != nil {
		t.Fatalf("%v:\n%s", err, b.String())
	}
	step := func(i int) string { s, _ := got.Order[i]["step"].(string); return s }
	if len(got.Order) < 3 || step(0) != "lint" || got.Order[0]["cost"] != "static" || got.Order[0]["proof"] != nil ||
		step(1) != "cc --since a1" || got.Order[1]["proof"] != true {
		t.Errorf("order:\n%s", b.String())
	}
	checks := 0
	for _, item := range got.Order {
		if c, ok := item["check"].(map[string]any); ok && c["task"] == "T-1" && item["action"] != nil {
			checks++
		}
	}
	if checks == 0 {
		t.Errorf("no check of T-1 in the order:\n%s", b.String())
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

// --data-at reads the ledger and the registry at the commit, by the working
// tree's config, whose own source stays the working tree; a commit that
// cannot be read is the error.
func TestDataAtReadsTheCommit(t *testing.T) {
	cfg := load(t, "version: 1\nledger:\n  files: tasks/phase-{group}.yaml\nci:\n  steps: [unit]\n")
	cfg.Trees = porttest.Trees{Common: ".git", Files: map[string]map[string]string{
		"abc": {
			"tasks/phase-1.yaml":    "- { id: T-9, title: Nine, done_when: [] }\n",
			"tasks/work-items.yaml": "items:\n  - { id: T-9, status: todo }\n",
		},
	}}
	d, err := DataAt(cfg, "abc")
	if err != nil || len(d.Tasks) != 1 || d.Tasks[0].ID != "T-9" {
		t.Fatalf("the commit's ledger: %+v, %v", d.Tasks, err)
	}
	if status, ok := d.Statuses.Of("T-9"); !ok || status != "todo" {
		t.Errorf("the commit's registry: %v, %v", status, ok)
	}
	if tree := cfg.Source.Tree(context.Background()); tree != "worktree" {
		t.Errorf("the config's source became %s", tree)
	}
	cfg.Trees = porttest.Trees{}
	if _, err := DataAt(cfg, "abc"); err == nil {
		t.Error("a commit outside a repository is read")
	}
}

// A step that runs a command and declares evidence keeps its declaration in
// the plan; a step written as text, or one that runs tests, has none.
func TestStepsKeepTheirEvidence(t *testing.T) {
	cfg := load(t, `version: 1
tests:
  scenario: { root: features, run: { whole: e2e } }
ci:
  steps:
    - { run: a, evidence: { inputs: [x] } }
    - { run: b }
    - c
    - { tests: scenario }
`)
	steps, err := ciSteps(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Evidence == nil || len(steps[0].Evidence.Inputs) != 1 || steps[0].Evidence.Inputs[0] != "x" {
		t.Errorf("step a: %+v", steps[0].Evidence)
	}
	for i, name := range []string{"b", "c", "tests"} {
		if steps[i+1].Evidence != nil {
			t.Errorf("step %s has evidence: %+v", name, steps[i+1].Evidence)
		}
	}
}

// A step that runs a kind of named tests takes the kind's whole run as its
// command, and has none when the kind declares none: config validation
// refuses such a step, so stepOf is given the config directly.
func TestAStepOfTestsTakesTheKindsWholeRun(t *testing.T) {
	cfg := &config.Loaded{}
	whole := "e2e"
	var withWhole, without config.Kind
	withWhole.Run.Whole = &whole
	cfg.Tests = config.Ordered[config.Kind]{
		Keys:   []string{"scenario", "unit"},
		Values: map[string]config.Kind{"scenario": withWhole, "unit": without},
	}
	for kind, want := range map[string]string{"scenario": "e2e", "unit": "", "missing": ""} {
		step := stepOf(cfg, config.Step{Tests: &kind})
		if step.Command != want || step.Tests != kind {
			t.Errorf("tests: %s: command %q, tests %q, want command %q", kind, step.Command, step.Tests, want)
		}
	}
}

// A range trusted for a merged pull request's green head (slice 188) is
// planned as a prose-only one whatever it touches: the prose steps, no run of
// named tests and no code proof, nor the line that says why there is none;
// the plan prints the trust's line last and its JSON names the trust, which
// no other plan's does. The note of a head not trusted prints before the line
// of a proof left out.
func TestForTrustedPlansTheProseRange(t *testing.T) {
	from, to := gitRange(t)
	cfg := load(t, scratch+`proof:
  code:
    paths: ["internal/**"]
    check: "cc since {base}"
`)
	trust := &Trust{PullRequest: "https://github.example/o/r/pull/7", Head: "h1", Run: "https://ci.example/runs/1", Line: "Trusted: the said."}
	p, err := ForTrusted(cfg, from, to, Data{Smoke: []string{"ID-S-01"}}, trust)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Prose || p.Trusted != trust || p.ProofOmitted != "" || p.Note != "" || !slices.Equal(p.Steps, []string{"lint"}) {
		t.Errorf("the trusted plan: %+v", p)
	}
	if got := lines(p); got != "static  lint\nTrusted: the said.\n" {
		t.Errorf("the trusted plan prints:\n%s", got)
	}
	var b strings.Builder
	if err := out.Emit(&b, p.Fields(Range{From: from, To: to, Readable: true})...); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Prose   bool
		Trusted map[string]string
	}
	if err := json.Unmarshal([]byte(b.String()), &got); err != nil || !got.Prose ||
		!maps.Equal(got.Trusted, map[string]string{"pull_request": trust.PullRequest, "head": "h1", "run": trust.Run}) ||
		strings.Index(b.String(), `"trusted"`) < strings.Index(b.String(), `"not_started"`) {
		t.Errorf("the trusted plan's JSON: %v, %+v\n%s", err, got, b.String())
	}

	plain, err := For(cfg, from, to, Data{})
	if err != nil {
		t.Fatal(err)
	}
	var plainJSON strings.Builder
	if err := out.Emit(&plainJSON, plain.Fields(Range{})...); err != nil {
		t.Fatal(err)
	}
	if plain.Prose || plain.Trusted != nil || strings.Contains(plainJSON.String(), "trusted") || plain.Order[1].Step == nil || !plain.Order[1].Step.Proof {
		t.Errorf("the plan of a range that is not trusted: %+v\n%s", plain, plainJSON.String())
	}

	plain.Note = "Not trusted: the said."
	plain.ProofOmitted = "Code proof not scheduled: the said."
	if got := lines(plain); !strings.HasSuffix(got, "\nNot trusted: the said.\nCode proof not scheduled: the said.\n") {
		t.Errorf("a head not trusted prints:\n%s", got)
	}
	plain.ProofOmitted = ""
	if got := lines(plain); !strings.HasSuffix(got, "\nNot trusted: the said.\n") || !strings.Contains(got, "(code proof)\n") {
		t.Errorf("a head not trusted, the proof scheduled, prints:\n%s", got)
	}
}
