package check

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/evidence"
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

type fakeGit struct{ checkouts, lookups int }

func (*fakeGit) IndexTree(context.Context) (string, error)       { return "tree", nil }
func (*fakeGit) IndexPaths(context.Context) ([]string, error)    { return nil, nil }
func (*fakeGit) UnstagedPaths(context.Context) ([]string, error) { return nil, nil }
func (g *fakeGit) Snapshot(context.Context, string) (evidence.Checkout, error) {
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
	r.Evidence = &Evidence{Cfg: cfg, Git: git, Store: store{t.TempDir()}, Binding: evidence.ThisBinary(), Mode: "task"}
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
			r.Evidence = &Evidence{Cfg: cfg, Git: git, Store: store{root}, Binding: evidence.ThisBinary(), Mode: "task"}
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
