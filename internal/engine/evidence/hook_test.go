package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	own := r.request("cd /; sleep 5; true", config.Evidence{})
	own.Seconds = 600
	cap := 0.05
	// Far above what a shell takes to start, even on a slow runner, so the run
	// that must finish is not racing the cap; the cap that stops a run is the
	// short one.
	roomy := 2.0
	capped := own
	capped.RunSeconds = &cap
	started := time.Now()
	o := r.Run(t.Context(), capped)
	if took := time.Since(started); took > 4*time.Second {
		t.Errorf("the run took %v", took)
	}
	if o.Err != nil || o.OK || !o.TimedOut || o.Info.Recorded || o.Info.Reason != ReasonInconclusive || len(r.records(t)) != 0 {
		t.Fatalf("a run the cap stops: %+v, records %v", o, r.records(t))
	}
	quick := r.request("echo done", config.Evidence{})
	quick.Seconds = 600
	quick.RunSeconds = &roomy
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

// A process a check leaves running keeps the output it inherited open. The
// run's output is a file, so that holds nobody: the run returns when the shell
// does, with the cap or without, and not when the process exits or, as through
// a pipe, after the shell's grace. The sleeper outlasts the grace (10 s), and
// the margin is generous; it starts in a folder of its own so that no
// checkout is its working folder.
func TestAProcessACheckLeavesRunningDoesNotHoldTheRun(t *testing.T) {
	for name, command := range map[string]string{
		"a check that exits":               "cd /; sleep 14 & echo out",
		"a check the cap stops, mid-sleep": "cd /; echo out; sleep 14; true",
	} {
		t.Run(name, func(t *testing.T) {
			r := hookRig(t)
			cap := 0.3
			req := r.request(command, config.Evidence{})
			req.RunSeconds = &cap
			started := time.Now()
			o := r.Run(t.Context(), req)
			if took := time.Since(started); took > 5*time.Second {
				t.Errorf("the run took %v", took)
			}
			if o.Err != nil || strings.TrimSpace(o.Output) != "out" || o.TimedOut != strings.Contains(name, "cap") {
				t.Errorf("%+v", o)
			}
		})
	}
}

// A quiet run's output is a file, whose end is kept, and which is gone with
// the run; a verbose run's, or one with no file to be had, goes through
// memory.
func TestACapturesTailIsTheEndOfItsOutput(t *testing.T) {
	text := strings.Repeat("0123456789", OutputTail/10+10) + "end"
	quiet := newCapture(Request{})
	if quiet.file == nil {
		t.Fatal("no file for a quiet run")
	}
	name := quiet.file.Name()
	if _, err := quiet.w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	got, cut := quiet.text()
	if len(got) != OutputTail || !cut || !strings.HasSuffix(got, "end") || got != text[len(text)-OutputTail:] {
		t.Errorf("a long output: %d bytes, cut %v", len(got), cut)
	}
	if _, err := os.Stat(name); err == nil {
		t.Error("the output file is left")
	}
	short := newCapture(Request{})
	if _, err := short.w.Write([]byte("short")); err != nil {
		t.Fatal(err)
	}
	if got, cut := short.text(); got != "short" || cut {
		t.Errorf("a short output: %q, cut %v", got, cut)
	}
	closed := newCapture(Request{})
	if _, err := closed.w.Write([]byte("lost")); err != nil {
		t.Fatal(err)
	}
	if err := closed.file.Close(); err != nil {
		t.Fatal(err)
	}
	if got, cut := closed.text(); got != "" || cut {
		t.Errorf("an output file that cannot be read: %q, cut %v", got, cut)
	}
	if _, err := os.Stat(closed.file.Name()); err == nil {
		t.Error("the unreadable output file is left")
	}
	var log strings.Builder
	verbose := newCapture(Request{Verbose: true, Log: &log})
	if _, err := verbose.w.Write([]byte("seen")); err != nil {
		t.Fatal(err)
	}
	if got, cut := verbose.text(); verbose.file != nil || got != "seen" || cut || log.String() != "seen" {
		t.Errorf("a verbose output: %q, cut %v, log %q", got, cut, log.String())
	}
	gone := filepath.Join(t.TempDir(), "gone")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, gone)
	}
	memory := newCapture(Request{})
	if _, err := memory.w.Write([]byte("kept")); err != nil {
		t.Fatal(err)
	}
	if got, cut := memory.text(); memory.file != nil || got != "kept" || cut {
		t.Errorf("no file to be had: %q, cut %v", got, cut)
	}
}
