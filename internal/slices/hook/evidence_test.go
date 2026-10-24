package hook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/evidence"
	privfake "github.com/donvargax/itos/v8/internal/store/private/port/porttest"
)

// checkoutsOf is the repository as the evidence engine asks it: the tree the
// hook captured, and a folder for each checkout it makes.
type checkoutsOf struct {
	t         *testing.T
	checkouts int
}

func (*checkoutsOf) IndexTree(context.Context) (string, error)       { return "captured", nil }
func (*checkoutsOf) Head(context.Context) (string, error)            { return "head", nil }
func (*checkoutsOf) IndexPaths(context.Context) ([]string, error)    { return nil, nil }
func (*checkoutsOf) UnstagedPaths(context.Context) ([]string, error) { return nil, nil }
func (g *checkoutsOf) Snapshot(context.Context, string) (evidence.Checkout, error) {
	g.checkouts++
	dir := g.t.TempDir()
	return evidence.Checkout{Dir: dir, Env: os.Environ(), Remove: func() error { return os.RemoveAll(dir) }}, nil
}

// evidenceWorld is a hook over a ledger of one task, T-1, whose checks are
// the given YAML; marks is a file each check appends a line to when it runs.
type evidenceWorld struct {
	*snapshotPorts
	git   *checkoutsOf
	marks string
}

func newEvidenceWorld(t *testing.T, status, checks string) *evidenceWorld {
	t.Helper()
	p := staging(t, nil, "docs: x\n\nTask: T-1\n")
	marks := filepath.ToSlash(filepath.Join(t.TempDir(), "marks"))
	checks = strings.ReplaceAll(checks, "MARKS", marks)
	p.trees.Files["index"]["tasks/phase-1.yaml"] = "- { id: T-1, type: docs, title: One, done_when: " + checks + " }\n"
	p.trees.Files["index"]["tasks/work-items.yaml"] = "items:\n  - { id: T-1, title: One, phase: 1, owner: null, status: " + status + ", depends_on: [] }\n"
	git := &checkoutsOf{t: t}
	p.engine = &evidence.Engine{
		Cfg: p.cfg, Git: git, Store: &privfake.Store{Folder: t.TempDir()}, Work: t.TempDir(),
		Binding: evidence.ThisBinary(), Mode: "hook", Hook: true,
	}
	snapshot := Snapshot{Env: os.Environ(), Remove: func() error { return nil }}
	return &evidenceWorld{snapshotPorts: &snapshotPorts{ports: p, snapshot: snapshot}, git: git, marks: marks}
}

func (w *evidenceWorld) attempt(t *testing.T) (Answer, string, error) {
	t.Helper()
	var stderr strings.Builder
	answer, err := NewHandler(w).tasksRule(t.Context(), w.cfg, "docs: x\n\nTask: T-1\n", Out{Stderr: &stderr})
	return answer, stderr.String(), err
}

func (w *evidenceWorld) runs(t *testing.T) int {
	t.Helper()
	text, err := os.ReadFile(w.marks)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(text), "\n")
}

const optedIn = `[{ run: "echo ran >> MARKS", cost: static, evidence: {} }]`

// A check that opts in and misses runs in the checkout the hook made before
// any check, which the engine takes as its own: one checkout in all, nothing
// printed, and the next attempt, a hit, makes none and says so.
func TestTasksRuleMakesOneCheckoutForAMissAndNoneForAHit(t *testing.T) {
	w := newEvidenceWorld(t, "done", optedIn)
	answer, stderr, err := w.attempt(t)
	if err != nil || answer.Code != 0 || stderr != "" || w.runs(t) != 1 || w.snapshots != 1 || w.git.checkouts != 0 {
		t.Fatalf("a miss: %+v, %v, stderr %q, %d runs, %d snapshots, %d own checkouts", answer, err, stderr, w.runs(t), w.snapshots, w.git.checkouts)
	}
	answer, stderr, err = w.attempt(t)
	if err != nil || answer.Code != 0 || w.runs(t) != 1 || w.snapshots != 1 || w.git.checkouts != 0 {
		t.Fatalf("a hit: %+v, %v, %d runs, %d snapshots, %d own checkouts", answer, err, w.runs(t), w.snapshots, w.git.checkouts)
	}
	if !strings.Contains(stderr, "T-1") || !strings.Contains(stderr, "cached evidence") {
		t.Errorf("a hit printed %q", stderr)
	}
}

// A check that does not opt in keeps the hook's shared checkout, and the one
// that opts in beside it runs in a checkout of its own, so a miss costs two.
func TestTasksRuleKeepsPlainChecksInTheSharedCheckout(t *testing.T) {
	w := newEvidenceWorld(t, "done", `[{ run: "echo ran >> MARKS", cost: static, evidence: {} }, { run: "echo plain >> MARKS", cost: static }]`)
	if answer, _, err := w.attempt(t); err != nil || answer.Code != 0 || w.runs(t) != 2 || w.snapshots != 1 || w.git.checkouts != 1 {
		t.Fatalf("%+v, %v, %d runs, %d snapshots, %d own checkouts", answer, err, w.runs(t), w.snapshots, w.git.checkouts)
	}
	if answer, _, err := w.attempt(t); err != nil || answer.Code != 0 || w.runs(t) != 3 || w.snapshots != 2 || w.git.checkouts != 1 {
		t.Fatalf("the next attempt: %+v, %v, %d runs, %d snapshots, %d own checkouts", answer, err, w.runs(t), w.snapshots, w.git.checkouts)
	}
}

// A checkout that cannot be made refuses the commit before any check runs
// when one must, and a hit spares the refusal.
func TestTasksRuleRefusesBeforeAnyCheckRunsOnlyWhenOneMust(t *testing.T) {
	refused := errors.New("checkout setup refused")
	w := newEvidenceWorld(t, "done", `[{ run: "echo plain >> MARKS", cost: static }, { run: "echo ran >> MARKS", cost: static, evidence: {} }]`)
	w.snapshotErr = refused
	if _, _, err := w.attempt(t); !errors.Is(err, refused) || w.runs(t) != 0 {
		t.Fatalf("a plain check: %v, %d runs", err, w.runs(t))
	}
	m := newEvidenceWorld(t, "done", optedIn)
	m.snapshotErr = refused
	if _, _, err := m.attempt(t); !errors.Is(err, refused) || m.runs(t) != 0 {
		t.Fatalf("a miss: %v, %d runs", err, m.runs(t))
	}
	m.snapshotErr = nil
	if _, _, err := m.attempt(t); err != nil || m.runs(t) != 1 {
		t.Fatalf("the miss, run: %v, %d runs", err, m.runs(t))
	}
	m.snapshotErr = refused
	if answer, _, err := m.attempt(t); err != nil || answer.Code != 0 || m.runs(t) != 1 {
		t.Fatalf("a hit: %+v, %v, %d runs", answer, err, m.runs(t))
	}
}

// A failing record judges the commit as a failing run does: a done task
// rejects it, one in progress is reported and lets it through, the recorded
// output among the report.
func TestTasksRuleJudgesAFailingHitAsAFailingRun(t *testing.T) {
	for status, code := range map[string]int{"done": policy, "doing": 0} {
		t.Run(status, func(t *testing.T) {
			w := newEvidenceWorld(t, status, `[{ run: "echo ran >> MARKS; echo the output; exit 1", cost: static, evidence: {} }]`)
			for _, attempt := range []string{"a miss", "a hit"} {
				answer, stderr, err := w.attempt(t)
				if err != nil || answer.Code != code || w.runs(t) != 1 {
					t.Fatalf("%s: %+v, %v, %d runs", attempt, answer, err, w.runs(t))
				}
				if !strings.Contains(stderr, "the output") || !strings.Contains(stderr, "failing T-1") && status == "doing" {
					t.Errorf("%s printed %q", attempt, stderr)
				}
				if hit := strings.Contains(stderr, "cached evidence"); hit != (attempt == "a hit") {
					t.Errorf("%s printed %q", attempt, stderr)
				}
			}
		})
	}
}

// Two tasks that share a check run it once: the later is a hit on the
// record the earlier made a moment before.
func TestTasksRuleLooksAgainAtEachTasksTurn(t *testing.T) {
	w := newEvidenceWorld(t, "done", optedIn)
	w.trees.Files["index"]["tasks/phase-1.yaml"] += strings.ReplaceAll("- { id: T-2, type: docs, title: Two, done_when: "+optedIn+" }\n", "MARKS", w.marks)
	var stderr strings.Builder
	answer, err := NewHandler(w).tasksRule(t.Context(), w.cfg, "docs: x\n\nTask: T-1, T-2\n", Out{Stderr: &stderr})
	if err != nil || answer.Code != 0 || w.runs(t) != 1 || w.snapshots != 1 || !strings.Contains(stderr.String(), "T-2") {
		t.Fatalf("%+v, %v, %d runs, %d snapshots, stderr %q", answer, err, w.runs(t), w.snapshots, stderr.String())
	}
}

// With no engine bound a check that opts in runs as any other, in the shared
// checkout.
func TestTasksRuleRunsAnOptedInCheckPlainlyWithoutAnEngine(t *testing.T) {
	w := newEvidenceWorld(t, "done", optedIn)
	w.engine = nil
	if answer, _, err := w.attempt(t); err != nil || answer.Code != 0 || w.runs(t) != 1 || w.snapshots != 1 {
		t.Fatalf("%+v, %v, %d runs, %d snapshots", answer, err, w.runs(t), w.snapshots)
	}
}

// A check with snapshot: false runs in the working folder, in the environment
// the hook gives such a check, whether it declares evidence or not: no
// checkout is made for it, so a snapshot that could not be made refuses
// nothing, and the engine is never asked.
func TestTasksRuleRunsASnapshotFalseCheckInTheWorkingFolder(t *testing.T) {
	for _, evidence := range []string{"", ", evidence: {}"} {
		w := newEvidenceWorld(t, "done", `[{ run: "test -e only-in-the-working-folder && test \"$WORK\" = 1 && echo ran >> MARKS", cost: static, snapshot: false`+evidence+` }]`)
		if err := os.WriteFile(filepath.Join(w.cfg.Folder(), "only-in-the-working-folder"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		w.workEnv = append(os.Environ(), "WORK=1")
		w.snapshotErr = errors.New("a snapshot must not be attempted")
		answer, _, err := w.attempt(t)
		if err != nil || answer.Code != 0 || w.runs(t) != 1 || w.snapshots != 0 || w.git.checkouts != 0 || w.engine.Used() {
			t.Errorf("%q: %+v, %v, %d runs, %d snapshots, %d own checkouts, engine used %v", evidence, answer, err, w.runs(t), w.snapshots, w.git.checkouts, w.engine.Used())
		}
	}
}

// Beside a check without it, in either order, each runs in its own place: the
// one with snapshot: false in the working folder and its environment, the
// other in the shared checkout, and the checkout is made once.
func TestTasksRuleRunsEachCheckInItsOwnPlace(t *testing.T) {
	outside := `{ run: "test -e only-in-the-working-folder && test \"$WORK\" = 1 && echo out >> MARKS", cost: static, snapshot: false }`
	inside := `{ run: "test ! -e only-in-the-working-folder && test -z \"$WORK\" && echo in >> MARKS", cost: static }`
	for _, checks := range []string{outside + ", " + inside, inside + ", " + outside} {
		w := newEvidenceWorld(t, "done", "["+checks+"]")
		if err := os.WriteFile(filepath.Join(w.cfg.Folder(), "only-in-the-working-folder"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		w.workEnv = append(os.Environ(), "WORK=1")
		w.snapshot.Dir = t.TempDir()
		answer, stderr, err := w.attempt(t)
		if err != nil || answer.Code != 0 || w.runs(t) != 2 || w.snapshots != 1 {
			t.Errorf("%s: %+v, %v, %d runs, %d snapshots, stderr %q", checks, answer, err, w.runs(t), w.snapshots, stderr)
		}
	}
}
