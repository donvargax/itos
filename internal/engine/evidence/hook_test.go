package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
)

func hookRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t)
	r.Hook = true
	return r
}

// The hook's engine says nothing of a miss or of the changes it leaves out,
// answers a hit with one line naming the task, and gives the output of a hit
// and of a run for the hook to report.
func TestTheHooksEngineIsQuietOnAMissAndSaysAHit(t *testing.T) {
	r := hookRig(t)
	r.git.unstaged = []string{"a"}
	req := r.request("echo the output; exit 2", config.Evidence{})
	req.Task = "T-001"
	miss := r.Run(t.Context(), req)
	if miss.Err != nil || miss.Code != 2 || miss.OK || !miss.Info.Recorded || miss.Output != "the output\n" {
		t.Fatalf("a miss: %+v", miss)
	}
	if r.log.Len() != 0 {
		t.Errorf("a miss printed %q", r.log.String())
	}
	if _, counted := r.Unstaged(); counted {
		t.Error("the changes left out were counted")
	}
	hit := r.Run(t.Context(), req)
	if !hit.Info.Hit || hit.Code != 2 || hit.Output != "the output\n" || hit.TimedOut {
		t.Fatalf("a hit: %+v", hit)
	}
	if got := r.log.String(); strings.Count(got, "\n") != 1 || !strings.HasPrefix(got, "  T-001 echo the output; exit 2: cached evidence (") {
		t.Errorf("a hit printed %q", got)
	}
	if r.git.checkouts != 1 {
		t.Errorf("%d checkouts", r.git.checkouts)
	}
}

// The hook never runs a check in the working folder: a checkout that cannot
// be made, an input that cannot be copied into it and a key that cannot be
// read refuse the check, running nothing, each with the kind of an
// environment without something.
func TestTheHooksEngineRefusesWhatItCannotRunOnTheTree(t *testing.T) {
	where := filepath.ToSlash(filepath.Join(t.TempDir(), "where"))
	command := "pwd > " + where
	for _, c := range []struct {
		name    string
		arrange func(r *rig)
		decl    config.Evidence
		reason  string
	}{
		{"a checkout that cannot be made", func(r *rig) { r.git.snapshotErr = errors.New("a gitlink") }, config.Evidence{}, ReasonSnapshotUnusable},
		{"an input that cannot be copied", func(r *rig) {
			r.git.badCheckout = true
			if err := os.WriteFile(filepath.Join(r.work, "in.txt"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, config.Evidence{Inputs: []string{"in.txt"}}, ReasonSnapshotUnusable},
		{"a key that cannot be read", func(*rig) {}, config.Evidence{Inputs: []string{"../outside"}}, ReasonUnkeyable},
		{"no index tree", func(r *rig) { r.git.treeErr = errors.New("no repository") }, config.Evidence{}, ReasonSnapshotUnusable},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := hookRig(t)
			c.arrange(r)
			o := r.Run(t.Context(), r.request(command, c.decl))
			if o.Err == nil || kind.Of(o.Err) != kind.Missing || o.Info.Reason != c.reason || o.Info.Recorded || o.OK {
				t.Fatalf("%+v", o)
			}
			if _, err := os.Stat(where); err == nil {
				t.Error("the check ran")
			}
			if r.log.Len() != 0 || len(r.records(t)) != 0 {
				t.Errorf("printed %q, records %v", r.log.String(), r.records(t))
			}
		})
	}
}

// A hook's miss under the cap runs at most the cap and records only a run
// that ended by itself, keyed by the check's own timeout: the cap's stop is
// no result, and a record made without the cap answers a run the cap would
// stop.
func TestAMissRunsUnderTheCapAndIsKeyedByTheOwnTimeout(t *testing.T) {
	r := hookRig(t)
	own := r.request("sleep 5", config.Evidence{})
	own.Seconds = 600
	cap := 0.05
	capped := own
	capped.RunSeconds = &cap
	o := r.Run(t.Context(), capped)
	if o.Err != nil || o.OK || !o.TimedOut || o.Info.Recorded || o.Info.Reason != ReasonInconclusive || len(r.records(t)) != 0 {
		t.Fatalf("a run the cap stops: %+v, records %v", o, r.records(t))
	}
	quick := r.request("echo done", config.Evidence{})
	quick.Seconds = 600
	quick.RunSeconds = &cap
	first := r.Run(t.Context(), quick)
	if !first.OK || first.TimedOut || !first.Info.Recorded {
		t.Fatalf("a run under the cap: %+v", first)
	}
	quick.RunSeconds = nil
	if again := r.Run(t.Context(), quick); !again.Info.Hit || again.Info.Key != first.Info.Key {
		t.Errorf("a record made under the cap, asked without it: %+v", again.Info)
	}
}

// Hit says whether a record would answer, and does nothing else.
func TestHitIsALookupAndNothingMore(t *testing.T) {
	r := hookRig(t)
	req := r.request("true", config.Evidence{})
	if r.Hit(t.Context(), req) {
		t.Error("a hit before any run")
	}
	r.Run(t.Context(), req)
	r.log.Reset()
	checkouts, held := r.git.checkouts, r.store.held
	if !r.Hit(t.Context(), req) {
		t.Error("no hit after the run")
	}
	if r.Hit(t.Context(), r.request("true;", config.Evidence{})) {
		t.Error("a hit for another command")
	}
	if r.log.Len() != 0 || r.git.checkouts != checkouts || r.store.held != held || len(r.Entries()) != 1 {
		t.Errorf("printed %q, checkouts %d, locks %d, entries %d", r.log.String(), r.git.checkouts, r.store.held, len(r.Entries()))
	}
	bad := hookRig(t)
	bad.git.treeErr = errors.New("no repository")
	if bad.Hit(t.Context(), req) {
		t.Error("a hit with no index tree")
	}
	unkeyed := hookRig(t)
	if unkeyed.Hit(t.Context(), unkeyed.request("true", config.Evidence{Inputs: []string{"../outside"}})) {
		t.Error("a hit for a key that cannot be read")
	}
}

// A spare checkout is the first miss's, once, and the next miss makes its
// own; the engine removes the one it used, the caller the one it did not.
func TestTheSpareCheckoutServesTheFirstMissOnly(t *testing.T) {
	r := hookRig(t)
	spare := filepath.Join(t.TempDir(), "spare")
	if err := os.MkdirAll(spare, 0o755); err != nil {
		t.Fatal(err)
	}
	removed := 0
	r.Spare(Checkout{Dir: spare, Env: append(os.Environ(), "IN_CHECKOUT=yes"), Remove: func() error { removed++; return nil }})
	if o := r.Run(t.Context(), r.request("test \"$IN_CHECKOUT\" = yes && echo here > ran-here", config.Evidence{})); !o.OK || o.Info.Reason != "" {
		t.Fatalf("%+v", o)
	}
	if _, err := os.Stat(filepath.Join(spare, "ran-here")); err != nil {
		t.Errorf("the check did not run in the spare: %v", err)
	}
	if removed != 1 || r.git.checkouts != 0 {
		t.Errorf("spare removed %d, checkouts %d", removed, r.git.checkouts)
	}
	r.Run(t.Context(), r.request("true", config.Evidence{}))
	if removed != 1 || r.git.checkouts != 1 {
		t.Errorf("spare removed %d, checkouts %d", removed, r.git.checkouts)
	}
}
