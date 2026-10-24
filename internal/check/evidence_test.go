package check

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/ledger"
	privport "github.com/donvargax/itos/v8/internal/store/private/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// A check that opts in and one that does not are two checks, as are two
// declarations; the order a declaration is written in is nothing.
func TestKeyHoldsTheEvidenceDeclaration(t *testing.T) {
	cfg := load(t, "version: 1\n")
	plain := ledger.Check{Run: text("x")}
	empty := ledger.Check{Run: text("x"), Evidence: &config.Evidence{}}
	a := ledger.Check{Run: text("x"), Evidence: &config.Evidence{Inputs: []string{"a", "b"}}}
	reordered := ledger.Check{Fails: text("x"), Evidence: &config.Evidence{Inputs: []string{"b", "a"}}}
	other := ledger.Check{Run: text("x"), Evidence: &config.Evidence{Env: []string{"E"}}}
	if Key(cfg, plain) == Key(cfg, empty) || Key(cfg, empty) == Key(cfg, a) || Key(cfg, a) == Key(cfg, other) {
		t.Errorf("checks that differ share a key: %q %q %q %q", Key(cfg, plain), Key(cfg, empty), Key(cfg, a), Key(cfg, other))
	}
	if Key(cfg, a) != Key(cfg, reordered) {
		t.Errorf("one declaration, two keys: %q %q", Key(cfg, a), Key(cfg, reordered))
	}
	if strings.Contains(Key(cfg, plain), "evidence") || strings.Count(Key(cfg, plain), "\x00") != 1 {
		t.Errorf("a check without evidence has the key it always had: %q", Key(cfg, plain))
	}
}

type fakeGit struct {
	checkouts, lookups int
	snapshotErr        error
}

func (*fakeGit) IndexTree(context.Context) (string, error)       { return "tree", nil }
func (*fakeGit) IndexPaths(context.Context) ([]string, error)    { return nil, nil }
func (*fakeGit) UnstagedPaths(context.Context) ([]string, error) { return nil, nil }
func (g *fakeGit) Snapshot(context.Context, string) (evidence.Checkout, error) {
	if g.snapshotErr != nil {
		return evidence.Checkout{}, g.snapshotErr
	}
	g.checkouts++
	return evidence.Checkout{Dir: "", Remove: func() error { return nil }}, nil
}

type store struct{ root string }

func (s store) Path(_ context.Context, name string) (string, error) {
	return filepath.Join(s.root, name), nil
}
func (store) Read(context.Context, string) ([]byte, bool, error)  { return nil, false, nil }
func (store) Write(context.Context, string, []byte) error         { return nil }
func (store) NotIgnored(context.Context, string) bool             { return false }
func (store) Hold(context.Context, string) (privport.Held, error) { return held{}, nil }

type held struct{}

func (held) Release(context.Context) error { return nil }

// A Runner with an engine looks a check that opts in up, and runs every
// other as it did; a check that reads the exit status of a run already made
// reports that run's evidence, not recorded again.
func TestRunnerLooksUpTheChecksThatOptIn(t *testing.T) {
	cfg := load(t, "version: 1\n")
	count := filepath.ToSlash(filepath.Join(t.TempDir(), "runs"))
	command := "echo ran >> " + count + "; exit 3"
	git := &fakeGit{}
	var stdout bytes.Buffer
	r := NewRunner(cfg, &stdout, &stdout)
	r.Evidence = &evidence.Engine{Cfg: cfg, Git: git, Store: store{t.TempDir()}, Binding: evidence.ThisBinary(), Mode: "task"}
	opted := ledger.Check{Run: text(command), Evidence: &config.Evidence{}}
	got, info := r.RunInfo(opted)
	if got != Fail || info == nil || info.Hit || !info.Recorded || info.Source != "local" {
		t.Fatalf("the run: %v %+v", got, info)
	}
	again, shared := r.RunInfo(ledger.Check{Fails: text(command), Evidence: &config.Evidence{}})
	if again != Pass || shared == nil || shared.Hit || shared.Recorded || shared.Key != info.Key {
		t.Errorf("the check that read the run: %v %+v", again, shared)
	}
	plain, none := r.RunInfo(ledger.Check{Run: text("true")})
	if plain != Pass || none != nil {
		t.Errorf("a check that did not opt in: %v %+v", plain, none)
	}
	fresh := NewRunner(cfg, &stdout, &stdout)
	fresh.Evidence = r.Evidence
	hit, hitInfo := fresh.RunInfo(opted)
	if hit != Fail || hitInfo == nil || !hitInfo.Hit {
		t.Errorf("the next invocation: %v %+v", hit, hitInfo)
	}
	runs, _ := os.ReadFile(count)
	if string(runs) != "ran\n" || git.checkouts != 1 {
		t.Errorf("ran %q, %d checkouts: one for the miss, none for the hit or the plain check", runs, git.checkouts)
	}
	without := NewRunner(cfg, &stdout, &stdout)
	if _, info := without.RunInfo(opted); info != nil {
		t.Errorf("a Runner with no engine looked a check up: %+v", info)
	}
}

// An after: push check is never looked up or recorded and has no evidence,
// whether it waits for the push or runs because HEAD is pushed. The push
// state is the VCS's, which the test sets: HEAD is on a remote branch, or on
// none.
func TestAnAfterPushCheckIsNeverLookedUpOrRecorded(t *testing.T) {
	history := []porttest.Commit{{Name: "c-a"}}
	for _, c := range []struct {
		name   string
		remote string
		want   Result
	}{{"not pushed", "", Pending}, {"pushed", "c-a", Pass}} {
		t.Run(c.name, func(t *testing.T) {
			cfg := load(t, "version: 1\n")
			named := map[string]string{"HEAD": "c-a"}
			if c.remote != "" {
				named["refs/remotes/origin/main"] = c.remote
			}
			cfg.VCS = &porttest.VCS{History: history, Named: named}
			git, root := &fakeGit{}, t.TempDir()
			var log bytes.Buffer
			r := NewRunner(cfg, &log, &log)
			r.Evidence = &evidence.Engine{Cfg: cfg, Git: git, Store: store{root}, Binding: evidence.ThisBinary(), Mode: "task"}
			after := ledger.Check{Run: text("true"), After: "push", Evidence: &config.Evidence{}}
			got, info := r.RunInfo(after)
			if got != c.want || info != nil {
				t.Errorf("the check: %v %+v, want %v and no evidence", got, info, c.want)
			}
			if git.lookups != 0 || git.checkouts != 0 || r.Evidence.Used() || len(r.Evidence.Entries()) != 0 {
				t.Errorf("looked up: %d trees, %d checkouts, used %v", git.lookups, git.checkouts, r.Evidence.Used())
			}
			if _, err := os.Stat(filepath.Join(root, evidence.Folder)); !os.IsNotExist(err) {
				t.Errorf("the store was touched: %v", err)
			}
		})
	}
}

func hookEngine(cfg *config.Loaded, git *fakeGit, root string) *evidence.Engine {
	return &evidence.Engine{Cfg: cfg, Git: git, Store: store{root}, Binding: evidence.ThisBinary(), Mode: "hook", Hook: true}
}

// RunCaptured through an engine answers a check that opts in from a record
// the second time, with the output of the run and its code, and judges it as
// a captured run does: past its cap it fails and says so, though its own
// timeout is the longer, and the run the cap stops leaves no record; a must
// fail check passes by failing; the engine's refusal is the error.
func TestRunCapturedThroughEvidence(t *testing.T) {
	cfg := load(t, "version: 1\n")
	git, root := &fakeGit{}, t.TempDir()
	at := Execution{Evidence: hookEngine(cfg, git, root), Log: &bytes.Buffer{}, Task: "T-1"}
	failing := ledger.Check{Run: text("echo out; exit 3"), Evidence: &config.Evidence{}, Timeout: seconds(30)}
	for _, want := range []string{"miss", "hit"} {
		run, err := RunCaptured(cfg, failing, 10, nil, at)
		if err != nil || run.Result != Fail || run.Code != 3 || run.Output != "out\n" || run.Seconds != 10 || !run.Capped || run.TimedOut {
			t.Fatalf("%s: %+v, %v", want, run, err)
		}
	}
	if git.checkouts != 1 || !strings.Contains(at.Log.(*bytes.Buffer).String(), "T-1 echo out; exit 3: cached evidence") {
		t.Errorf("%d checkouts, log %q", git.checkouts, at.Log.(*bytes.Buffer).String())
	}
	mustFail := ledger.Check{Fails: text("echo out; exit 3"), Evidence: &config.Evidence{}, Timeout: seconds(30)}
	if run, err := RunCaptured(cfg, mustFail, 10, nil, at); err != nil || run.Result != Pass || git.checkouts != 1 {
		t.Errorf("a must-fail check answered by the same record: %+v, %v, %d checkouts", run, err, git.checkouts)
	}
	slow := ledger.Check{Run: text("sleep 5"), Evidence: &config.Evidence{}, Timeout: seconds(30)}
	run, err := RunCaptured(cfg, slow, 0.05, nil, at)
	if err != nil || run.Result != Fail || !run.TimedOut || !run.Capped || run.Seconds != 0.05 {
		t.Fatalf("a run the cap stops: %+v, %v", run, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, evidence.Folder)); len(entries) != 1 {
		t.Errorf("the store holds %d entries, the failing check's record alone expected", len(entries))
	}
	git.snapshotErr = errors.New("a gitlink")
	if _, err := RunCaptured(cfg, ledger.Check{Run: text("true"), Evidence: &config.Evidence{}}, 10, nil, at); !errors.Is(err, git.snapshotErr) {
		t.Errorf("a checkout that cannot be made: %v", err)
	}
}

// A check that does not opt in, or an after: push check, runs where the
// execution puts it, never through the engine.
func TestRunCapturedRunsOnlyOptedInChecksThroughEvidence(t *testing.T) {
	cfg := load(t, "version: 1\n")
	git := &fakeGit{}
	dir := t.TempDir()
	at := Execution{Evidence: hookEngine(cfg, git, t.TempDir()), Dir: dir}
	for _, c := range []ledger.Check{
		{Run: text("echo here > ran")},
		{Run: text("echo here > ran"), Evidence: &config.Evidence{}, After: "push"},
	} {
		if run, err := RunCaptured(cfg, c, 10, nil, at); err != nil || run.Result != Pass {
			t.Fatalf("%+v: %+v, %v", c, run, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "ran")); err != nil {
			t.Errorf("%+v did not run in the execution's folder: %v", c, err)
		}
		if err := os.Remove(filepath.Join(dir, "ran")); err != nil {
			t.Fatal(err)
		}
	}
	if git.checkouts != 0 {
		t.Errorf("%d checkouts", git.checkouts)
	}
}

// Answers is true only for an opted-in check that is not an after: push
// check and has a record.
func TestAnswersIsTrueForARecordedOptedInCheckOnly(t *testing.T) {
	cfg := load(t, "version: 1\n")
	e := hookEngine(cfg, &fakeGit{}, t.TempDir())
	opted := ledger.Check{Run: text("true"), Evidence: &config.Evidence{}}
	if Answers(e, cfg, opted) {
		t.Error("answered before any run")
	}
	at := Execution{Evidence: e, Log: &bytes.Buffer{}}
	if _, err := RunCaptured(cfg, opted, NoCap, nil, at); err != nil {
		t.Fatal(err)
	}
	if !Answers(e, cfg, opted) {
		t.Error("not answered after a run")
	}
	if Answers(nil, cfg, opted) || Answers(e, cfg, ledger.Check{Run: text("true")}) || Answers(e, cfg, ledger.Check{Run: text("true"), Evidence: &config.Evidence{}, After: "push"}) {
		t.Error("answered a check that is not looked up, or with no engine")
	}
}
