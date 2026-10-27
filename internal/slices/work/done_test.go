package work

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// doneText is a registry work done can close: the task T-1 and the slices
// slice-2, which the queue holds, and slice-3 in progress for a, and the
// slice slice-1 done.
const doneText = `phases: { 1: null }
queue: [slice-2]
items:
  - { id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [], why: Adopt it. }
  - { id: slice-1, title: A slice, phase: 1, owner: b, status: done, depends_on: [] }
  - { id: slice-2, title: Doing, phase: 1, owner: a, status: doing, depends_on: [] }
  - { id: slice-3, title: Specified, phase: 1, owner: a, status: doing, depends_on: [] }
  - { id: T-9, title: Unledgered, phase: 1, owner: a, status: doing, depends_on: [] }
`

// doneFeature has slice-3's scenarios, one live and one still @wip.
const doneFeature = `Feature: Done

  @ID-DONE-1 @slice-3
  Scenario: Live
    Given nothing

  @wip @ID-DONE-2 @slice-3
  Scenario: Not yet
    Given nothing
`

// watching is the config with ci.watch's provider GitHub's, looking at
// once and giving up at once.
const watching = configText + "ci:\n  watch:\n    provider: github\n    interval: 0\n    timeout: 0.01\n"

// staticLedgerText is the ledger whose task's one check is static, so that
// closing it names no late check.
const staticLedgerText = "- id: T-1\n  type: chore\n  title: Adopt\n  why: Adopt it.\n  done_when:\n    - run: \"true\"\n      cost: static\n"

// closable is a fixture whose registry is doneText, slice-3's scenarios in
// the working tree and at HEAD, and whose ledger file the tracker holds as
// committed, under a stealth config when stealth.
func closable(stealth bool) fixture {
	f := sound()
	f.files["work-items.yaml"] = doneText
	f.files["features/done.feature"] = doneFeature
	f.files[ledgerFile] = staticLedgerText
	for i := range f.history {
		f.history[i].Tree = maps.Clone(f.history[i].Tree)
		f.history[i].Tree["features/done.feature"] = doneFeature
	}
	f.tracked = []string{ledgerFile}
	f.stealth = stealth
	return f
}

// vcsOf is the VCS fake p's config reads the repository through.
func vcsOf(p ports) *vcsfake.VCS { return p.cfg.VCS.(broken).VCS }

// txsOf is p with its tracker keeping every transaction it is handed.
func txsOf(p *ports) *[]trackerport.Transaction {
	txs := []trackerport.Transaction{}
	p.tracker = recorded{Tracker: p.tracker, txs: &txs}
	return &txs
}

// bodyOf is a transaction's message on one line.
func bodyOf(tx trackerport.Transaction) string {
	return strings.Join(strings.Fields(strings.Join(tx.Paragraphs, " ")), " ")
}

// notes are what work done says on stderr in a repository with no remote
// and no ci.watch.
const notes = "itos: the repository has no remote, so whether its commits are pushed was not checked\n" +
	"itos: ci.watch.provider is none, so CI was not checked\n"

// work done closes an item: its line names the close commit, then a task's
// checks taken out of its ledger file and the queue's commit; --json gives
// the item, ci, queue_commit and the commit; -q says nothing.
func TestDoneSaysWhatItDid(t *testing.T) {
	p := over(t, closable(false))
	txs := txsOf(&p)
	code, stdout, stderr, err := run(t, p, "done", "slice-2")
	if code != 0 || err != nil || stdout != "slice-2 is done: committed "+sha+" docs: close slice-2; out of the queue: committed "+sha+" docs: dequeue slice-2\n" ||
		stderr != notes+"[2] docs: close slice-2\n[3] docs: dequeue slice-2\n" {
		t.Errorf("work done slice-2 = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if after := registryIn(t, p); !strings.Contains(after, "queue: []") || !strings.Contains(after, "id: slice-2, title: Doing, phase: 1, owner: a, status: done") {
		t.Errorf("the registry after:\n%s", after)
	}
	if len(*txs) != 2 || len((*txs)[0].Changes) != 1 || (*txs)[0].Changes[0].Name != "work-items.yaml" {
		t.Errorf("the close's transactions: %d, the first %q", len(*txs), bodyOf((*txs)[0]))
	}

	p = over(t, closable(false))
	code, stdout, _, err = run(t, p, "--json", "done", "slice-2")
	o := object(t, stdout)
	if code != 0 || err != nil || o["ok"] != true || o["item"].(map[string]any)["status"] != "done" || o["ci"] != "unwatched" ||
		o["queue_commit"] != "0000000000000000000000000000000000000003" || o["commit"] != "0000000000000000000000000000000000000002" {
		t.Errorf("work --json done slice-2 = %d, %v, %v", code, err, o)
	}
	if code, stdout, stderr, err := run(t, over(t, closable(false)), "-q", "done", "slice-2"); code != 0 || err != nil || stdout != "" || stderr != notes {
		t.Errorf("work -q done slice-2 = %d, %v, %q, %q", code, err, stdout, stderr)
	}

	p = over(t, closable(false))
	txs = txsOf(&p)
	checked := []string{}
	p.checked = &checked
	code, stdout, _, err = run(t, p, "done", "T-1")
	if code != 0 || err != nil || stdout != "T-1 is done: committed "+sha+" docs: close T-1; its checks are out of tasks/phase-1.yaml\n" ||
		!slices.Equal(checked, []string{"T-1"}) {
		t.Errorf("work done T-1 = %d, %v, %q, checked %q", code, err, stdout, checked)
	}
	if ledger := docIn(t, p, ledgerFile); ledger != "- id: T-1\n  type: chore\n  title: Adopt\n  why: Adopt it.\n" {
		t.Errorf("the ledger after:\n%s", ledger)
	}
	if len(*txs) != 1 || len((*txs)[0].Changes) != 2 || !strings.HasSuffix(bodyOf((*txs)[0]), " Its checks are taken out of tasks/phase-1.yaml: "+
		"they were its progress while it was open, and gated this close. The commit before this one, c-4, still holds them.") {
		t.Errorf("the close's transactions: %d, the first %q", len(*txs), bodyOf((*txs)[0]))
	}

	// An id of the ledger's shape that no task has is closed as any item.
	p = over(t, closable(false))
	checked = []string{}
	p.checked = &checked
	code, stdout, _, err = run(t, p, "done", "T-9")
	if code != 0 || err != nil || stdout != "T-9 is done: committed "+sha+" docs: close T-9\n" || len(checked) != 0 {
		t.Errorf("work done T-9 = %d, %v, %q, checked %q", code, err, stdout, checked)
	}

	code, stdout, stderr, err = run(t, over(t, closable(false)), "done", "slice-1")
	if code != 0 || err != nil || stdout != "slice-1 is already done; nothing to change\n" || stderr != "" {
		t.Errorf("work done slice-1 = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = run(t, over(t, closable(false)), "--json", "done", "slice-1")
	if o = object(t, stdout); code != 0 || err != nil || o["ok"] != true || o["commit"] != nil || o["ci"] != nil || o["item"].(map[string]any)["id"] != "slice-1" {
		t.Errorf("work --json done slice-1 = %d, %v, %v", code, err, o)
	}
	if code, stdout, _, err = run(t, over(t, closable(false)), "-q", "done", "slice-1"); code != 0 || err != nil || stdout != "" {
		t.Errorf("work -q done slice-1 = %d, %v, %q", code, err, stdout)
	}
}

// Under a stealth config nothing is committed, the close's body names no
// commit before it, and the registry's lock is given back while the checks
// run and taken again for the write (bug 16).
func TestDoneUnderAStealthConfig(t *testing.T) {
	const nothing = "the registry is the stealth config's, so nothing is committed"
	code, stdout, _, err := run(t, over(t, closable(true)), "done", "slice-2")
	if code != 0 || err != nil || stdout != "slice-2 is done: "+nothing+"; out of the queue: "+nothing+"\n" {
		t.Errorf("work done slice-2 = %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = run(t, over(t, closable(true)), "--json", "done", "slice-2")
	if o := object(t, stdout); code != 0 || err != nil || o["commit"] != nil || o["queue_commit"] != nil || !hasKey(o, "queue_commit") {
		t.Errorf("work --json done slice-2 = %d, %v, %v", code, err, o)
	}

	p := over(t, closable(true))
	txs := txsOf(&p)
	free := false
	p.during = func() {
		held, err := p.tracker.Hold(t.Context())
		if free = err == nil; free {
			_ = held.Release(t.Context())
		}
	}
	code, stdout, _, err = run(t, p, "done", "T-1")
	if code != 0 || err != nil || stdout != "T-1 is done: "+nothing+"; its checks are out of tasks/phase-1.yaml\n" || !free {
		t.Errorf("work done T-1 = %d, %v, %q, the lock free during the checks: %v", code, err, stdout, free)
	}
	if len(*txs) != 1 || strings.Contains(bodyOf((*txs)[0]), "The commit before") {
		t.Errorf("the close's transactions: %d, the first %q", len(*txs), bodyOf((*txs)[0]))
	}
	if held, err := p.tracker.Hold(t.Context()); err != nil {
		t.Errorf("the lock is still held: %v", err)
	} else {
		_ = held.Release(t.Context())
	}

	// Refused once the lock is given back, nothing gives it back twice.
	p = over(t, closable(true))
	code, _, stderr, err := run(t, p, "done", "slice-3")
	if code != refused || err != nil || strings.Contains(stderr, "lock") {
		t.Errorf("work done slice-3 = %d, %v, %q", code, err, stderr)
	}
	if held, err := p.tracker.Hold(t.Context()); err != nil {
		t.Errorf("the lock is still held after a refusal: %v", err)
	} else {
		_ = held.Release(t.Context())
	}
}

// hasKey is whether the object has the key, null or not.
func hasKey(o map[string]any, key string) bool {
	_, ok := o[key]
	return ok
}

// work done refuses, exit 1, nothing written: a registry with no such item,
// a scenario still @wip, commits no remote has, a task's check that fails
// (its output on stderr first); under --json one object.
func TestDoneRefuses(t *testing.T) {
	for _, c := range []struct {
		name, id string
		set      func(*ports)
		stderr   string
	}{
		{"no such item", "nope", nil, "no item nope"},
		{"a scenario still @wip", "slice-3", nil, "slice-3's scenario @ID-DONE-2 (done.feature) is still @wip at HEAD\n" +
			"  fix: implement it and commit it live, then run itos work done slice-3 again\n"},
		{"commits no remote has", "slice-2", func(p *ports) {
			v := vcsOf(*p)
			v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u"}}
			v.Named["refs/remotes/origin/main"] = "c-1a"
		}, "HEAD has 3 commits no remote has, so slice-2 has not landed\n  fix: push them with itos push, then run itos work done slice-2 again\n"},
		{"a commit no remote has", "slice-2", func(p *ports) {
			v := vcsOf(*p)
			v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u"}}
			v.Named["refs/remotes/origin/main"] = "c-3a"
		}, "HEAD has a commit no remote has, so slice-2 has not landed\n"},
		{"its own commit no remote has", "T-1", func(p *ports) {
			vcsOf(*p).Remote = map[string]*vcsfake.Remote{"origin": {URL: "u"}}
		}, "The history up to c-1, the newest commit of T-1, has a commit no remote has, so T-1 has not landed\n"},
		{"a check that fails", "T-1", func(p *ports) { p.failure = "`false` exited 1" },
			notes[:len(notes)-len("itos: ci.watch.provider is none, so CI was not checked\n")] +
				"T-1: the check said why\nT-1's check fails: `false` exited 1\n  fix: itos task T-1 shows what its checks want\n"},
	} {
		p := over(t, closable(false))
		if c.set != nil {
			c.set(&p)
		}
		code, stdout, stderr, err := run(t, p, "done", c.id)
		if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, c.stderr) || registryIn(t, p) != doneText {
			t.Errorf("%s: work done %s = %d, %v, %q, %q", c.name, c.id, code, err, stdout, stderr)
		}
		p = over(t, closable(false))
		if c.set != nil {
			c.set(&p)
		}
		code, stdout, _, err = run(t, p, "--json", "done", c.id)
		if o := object(t, stdout); code != refused || err != nil || o["ok"] != false || len(o["problems"].([]any)) != 1 {
			t.Errorf("%s: work --json done %s = %d, %v, %v", c.name, c.id, code, err, o)
		}
	}

	// The item's own commit pushed, the commits after it are passed over.
	p := over(t, closable(false))
	v := vcsOf(p)
	v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u"}}
	v.Named["refs/remotes/origin/main"] = "c-1a"
	if code, _, stderr, err := run(t, p, "done", "T-1"); code != 0 || err != nil || strings.Contains(stderr, "remote") {
		t.Errorf("work done T-1 pushed = %d, %v, %q", code, err, stderr)
	}
	// A remote whose branch names no commit here cannot say what is pushed:
	// nothing is refused for it.
	p = over(t, closable(false))
	v = vcsOf(p)
	v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u"}}
	v.Named["refs/remotes/origin/main"] = "gone"
	if code, _, _, err := run(t, p, "done", "slice-2"); code != 0 || err != nil {
		t.Errorf("work done slice-2 with an unreadable range = %d, %v", code, err)
	}
}

// What ends work done but a refusal: a check that cannot run, the ledger
// that cannot be read for its close, a registry a writer cannot edit in
// place, a commit that fails or is refused, the queue's commit included.
func TestDoneEnds(t *testing.T) {
	p := over(t, closable(false))
	p.cfg, p.err = nil, errors.New("no config")
	if code, _, _, err := run(t, p, "done", "slice-2"); code != 0 || err == nil || err.Error() != "no config" {
		t.Errorf("no config = %d, %v", code, err)
	}

	f := closable(false)
	f.config = strings.Replace(configText, "    root: features\n", "", 1)
	if code, _, _, err := run(t, over(t, f), "done", "slice-2"); code != 0 || err == nil || !strings.Contains(err.Error(), "tests.scenario.root is missing") {
		t.Errorf("scenarios with no root = %d, %v", code, err)
	}

	f = closable(false)
	f.files[ledgerFile] = "- [\n"
	if code, _, _, err := run(t, over(t, f), "done", "T-1"); code != 0 || err == nil {
		t.Errorf("a ledger that does not parse = %d, %v", code, err)
	}

	p = over(t, closable(false))
	p.failureErr = errors.New("no shell")
	if code, stdout, _, err := run(t, p, "done", "T-1"); code != 0 || err == nil || err.Error() != "no shell" || stdout != "" {
		t.Errorf("a check that cannot run = %d, %v, %q", code, err, stdout)
	}

	p = over(t, closable(false))
	good := 0
	p.tracker = loadsThen{Tracker: p.tracker, name: ledgerFile, good: &good}
	if code, _, _, err := run(t, p, "done", "T-1"); code != 0 || err == nil || err.Error() != ledgerFile+" is gone" {
		t.Errorf("a ledger that cannot be read = %d, %v", code, err)
	}

	f = closable(false)
	f.files["work-items.yaml"] = "{ phases: { 1: null }, items: [{ id: slice-2, title: Doing, phase: 1, owner: a, status: doing, depends_on: [] }] }\n"
	if code, _, _, err := run(t, over(t, f), "done", "slice-2"); code != 0 || kind.Of(err) != kind.Usage ||
		err == nil || !strings.HasPrefix(err.Error(), "work-items.yaml cannot be edited in place: ") {
		t.Errorf("a flow registry = %d, %v", code, err)
	}

	p = over(t, closable(false))
	p.tracker = tracked{Tracker: p.tracker, transact: errors.New("no commit")}
	if code, stdout, _, err := run(t, p, "done", "slice-2"); code != 0 || err == nil || err.Error() != "no commit" || stdout != "" {
		t.Errorf("a commit that fails = %d, %v, %q", code, err, stdout)
	}
	p = over(t, closable(false))
	p.tracker.(*trackerfake.Tracker).Refuse = true
	if code, stdout, stderr, err := run(t, p, "done", "slice-2"); code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "refused\n") {
		t.Errorf("a commit refused = %d, %v, %q, %q", code, err, stdout, stderr)
	}

	flow := "{ phases: { 1: null }, queue: [slice-2], items: [{ id: slice-2, title: Doing, phase: 1, owner: a, status: done, depends_on: [] }] }\n"
	for _, c := range []struct {
		name string
		then afterClose
		code int
		err  string
	}{
		{"the registry gone", afterClose{gone: true}, 0, "open work-items.yaml: file does not exist"},
		{"the registry unreadable", afterClose{err: errors.New("no read")}, 0, "no read"},
		{"a queue not edited in place", afterClose{text: &flow}, 0, "work-items.yaml cannot be edited in place: "},
		{"the queue's commit refused", afterClose{refuse: true}, refused, ""},
		{"the queue's commit failing", afterClose{fail: errors.New("no commit")}, 0, "no commit"},
	} {
		p := over(t, closable(false))
		closed := false
		c.then.Tracker, c.then.closed = p.tracker.(*trackerfake.Tracker), &closed
		p.tracker = c.then
		code, stdout, stderr, err := run(t, p, "done", "slice-2")
		if code != c.code || stdout != "" || (c.err == "") != (err == nil) || (err != nil && !strings.HasPrefix(err.Error(), c.err)) {
			t.Errorf("%s: work done slice-2 = %d, %v, %q, %q", c.name, code, err, stdout, stderr)
		}
		if c.name == "the registry gone" && !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: %v", c.name, err)
		}
	}

	// The registry as the working tree has it, which the queue's change is
	// judged against, cannot be read.
	f = closable(false)
	p = over(t, f)
	p.tracker = afterClose{Tracker: p.tracker.(*trackerfake.Tracker), closed: new(bool), then: func() { f.files["work-items.yaml"] = "items: [\n" }}
	if code, stdout, _, err := run(t, p, "done", "slice-2"); code != 0 || err == nil || stdout != "" {
		t.Errorf("a registry the working tree cannot parse after the close = %d, %v, %q", code, err, stdout)
	}
}

// afterClose is the tracker's fake that, once a commit is made, has then
// done, refuses the next commit or fails it, and loads the registry as gone,
// failing or as text.
type afterClose struct {
	*trackerfake.Tracker
	closed *bool
	then   func()
	refuse bool
	fail   error
	gone   bool
	err    error
	text   *string
}

func (a afterClose) Transact(ctx context.Context, tx trackerport.Transaction) (string, error) {
	if *a.closed && a.fail != nil {
		return "", a.fail
	}
	sha, err := a.Tracker.Transact(ctx, tx)
	if err == nil && !*a.closed {
		*a.closed = true
		a.Tracker.Refuse = a.refuse
		if a.then != nil {
			a.then()
		}
	}
	return sha, err
}

func (a afterClose) Load(ctx context.Context, names []string) ([]trackerport.Doc, error) {
	if !*a.closed || names[0] != "work-items.yaml" {
		return a.Tracker.Load(ctx, names)
	}
	switch {
	case a.gone:
		return []trackerport.Doc{{Name: names[0]}}, nil
	case a.err != nil:
		return nil, a.err
	case a.text != nil:
		return []trackerport.Doc{{Name: names[0], Text: *a.text, There: true}}, nil
	}
	return a.Tracker.Load(ctx, names)
}

// The registry is read and judged again after the checks (bug 34): a change
// made meanwhile is kept, an item closed meanwhile is already done, and a
// registry no longer sound is refused.
func TestDoneReadsTheRegistryAgain(t *testing.T) {
	closedMeanwhile := strings.Replace(doneText, "status: doing, depends_on: [], why: Adopt it.", "status: done, depends_on: []", 1)
	for _, c := range []struct {
		name, text, stdout string
		code               int
	}{
		{"closed meanwhile", closedMeanwhile, "T-1 is already done; nothing to change\n", 0},
		{"no longer sound", "phases: { 1: null }\nitems:\n  - { id: x }\n  - { id: x }\n", "", refused},
	} {
		f := closable(false)
		p := over(t, f)
		p.during = func() {
			f.files["work-items.yaml"] = c.text
			p.tracker.(*trackerfake.Tracker).Edit("work-items.yaml", c.text)
		}
		code, stdout, _, err := run(t, p, "done", "T-1")
		if code != c.code || err != nil || stdout != c.stdout {
			t.Errorf("%s: work done T-1 = %d, %v, %q", c.name, code, err, stdout)
		}
	}

	f := closable(false)
	p := over(t, f)
	flow := "{ phases: { 1: null }, items: [{ id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [] }] }\n"
	p.during = func() {
		f.files["work-items.yaml"] = flow
		p.tracker.(*trackerfake.Tracker).Edit("work-items.yaml", flow)
	}
	if code, _, _, err := run(t, p, "done", "T-1"); code != 0 || err == nil || !strings.HasPrefix(err.Error(), "work-items.yaml cannot be edited in place: ") {
		t.Errorf("a flow registry meanwhile = %d, %v", code, err)
	}

	f = closable(false)
	p = over(t, f)
	renamed := strings.Replace(doneText, "Specified", "Renamed meanwhile", 1)
	p.during = func() {
		f.files["work-items.yaml"] = renamed
		if _, err := p.tracker.Transact(t.Context(), trackerport.Transaction{Changes: []trackerport.Change{{Name: "work-items.yaml", Text: renamed}},
			Paragraphs: []string{"docs: edit slice-3"}, Quiet: true}); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, _, err := run(t, p, "-q", "done", "T-1"); code != 0 || err != nil ||
		registryIn(t, p) != strings.Replace(renamed, "status: doing, depends_on: [], why: Adopt it.", "status: done, depends_on: []", 1) {
		t.Errorf("a registry changed meanwhile = %d, %v:\n%s", code, err, registryIn(t, p))
	}

	// A change no commit holds, made meanwhile, would be swept into the
	// close: it is refused rather than written over.
	f = closable(false)
	p = over(t, f)
	p.during = func() {
		f.files["work-items.yaml"] = renamed
		p.tracker.(*trackerfake.Tracker).Edit("work-items.yaml", renamed)
	}
	if code, stdout, stderr, err := run(t, p, "done", "T-1"); code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "work-items.yaml has changes no commit holds") {
		t.Errorf("a registry edited meanwhile = %d, %v, %q, %q", code, err, stdout, stderr)
	}
}

// look is a watcher whose runs are runs, by head, and whose branch lists
// listed, newest first.
func look(runs map[string]providers.Run, listed ...providers.Run) providers.Watcher {
	return providers.Watcher{
		Look: func(sha string) (providers.Run, bool, error) {
			r, ok := runs[sha]
			return r, ok, nil
		},
		Runs: func(string) ([]providers.Run, error) { return listed, nil },
	}
}

// With ci.watch, work done asks for the judged commit's run: any run that
// passed and covers the item's own newest commit lands it, its address in
// the close's body; else the commit's run, or the newest on its branch that
// has it, waited for; one that failed refuses (exit 1), one that cannot be
// looked at exits 3 and one that never appears 75, under --json with the
// watch's keys.
func TestDoneWaitsForCI(t *testing.T) {
	passed := func(url, head string) providers.Run {
		return providers.Run{URL: url, Status: "completed", Conclusion: "success", HeadSHA: head}
	}
	failed := providers.Run{URL: "f", Status: "completed", Conclusion: "failure", HeadSHA: "c-1a"}
	watched := closable(false)
	watched.config = watching

	p := over(t, watched)
	txs := txsOf(&p)
	p.watcher = look(map[string]providers.Run{"c-1a": passed("u", "c-1a")})
	code, stdout, _, err := run(t, p, "done", "T-1")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "CI passed: u\nT-1 is done: committed ") ||
		len(*txs) != 1 || !strings.Contains(bodyOf((*txs)[0]), " The CI run of c-1, the newest commit of T-1, passed: u.") {
		t.Errorf("its own run passed: %d, %v, %q, %d commits", code, err, stdout, len(*txs))
	}
	p = over(t, watched)
	p.watcher = look(map[string]providers.Run{"c-1a": passed("u", "c-1a")})
	code, stdout, stderr, err := run(t, p, "--json", "done", "T-1")
	if o := object(t, stdout); code != 0 || err != nil || o["ci"] != "success" || o["run"].(map[string]any)["url"] != "u" || !strings.HasSuffix(stderr, "\nCI passed: u\n[2] docs: close T-1\n") {
		t.Errorf("its own run passed, --json: %d, %v, %v, %q", code, err, o, stderr)
	}

	p = over(t, watched)
	p.watcher = look(map[string]providers.Run{"c-1a": passed("u", "c-1a")})
	if code, stdout, stderr, err := run(t, p, "-q", "done", "T-1"); code != 0 || err != nil || stdout != "" || strings.Contains(stderr, "CI passed") {
		t.Errorf("its own run passed, -q: %d, %v, %q, %q", code, err, stdout, stderr)
	}

	p = over(t, watched)
	vcsOf(p).OnBranch = "main"
	txs = txsOf(&p)
	p.watcher = look(map[string]providers.Run{"c-1a": failed}, failed, passed("u2", "c-4a"))
	code, _, _, err = run(t, p, "done", "T-1")
	if code != 0 || err != nil || len(*txs) != 1 || !strings.Contains(bodyOf((*txs)[0]), " The CI run of c-4a, which has c-1, the newest commit of T-1, passed: u2.") {
		t.Errorf("a later run passed: %d, %v, %d commits", code, err, len(*txs))
	}

	p = over(t, watched)
	p.watcher = look(map[string]providers.Run{"c-1a": failed}, failed)
	code, stdout, stderr, err = run(t, p, "done", "T-1")
	if code != refused || err != nil || stdout != "CI run: f\n" || !strings.HasSuffix(stderr, "CI failure: f\n"+
		"The CI run of c-1, the newest commit of T-1, did not pass, so T-1 is not done: f\n"+
		"  fix: fix what failed, push the fix with itos push, then run itos work done T-1 again\n") || registryIn(t, p) != doneText {
		t.Errorf("its run failed: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p = over(t, watched)
	p.watcher = look(map[string]providers.Run{"c-1a": failed}, failed)
	code, stdout, _, err = run(t, p, "--json", "done", "T-1")
	if o := object(t, stdout); code != refused || err != nil || o["ok"] != false || o["ci"] != "failure" || len(o["problems"].([]any)) != 1 {
		t.Errorf("its run failed, --json: %d, %v, %v", code, err, o)
	}

	p = over(t, watched)
	p.watcherErr = errors.New("no token")
	code, stdout, stderr, err = run(t, p, "done", "T-1")
	if code != missing || err != nil || stdout != "" || !strings.HasSuffix(stderr, "itos: no token\n") {
		t.Errorf("no watcher: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p = over(t, watched)
	p.watcherErr = errors.New("no token")
	code, stdout, _, err = run(t, p, "--json", "done", "T-1")
	if o := object(t, stdout); code != missing || err != nil || o["ok"] != false || o["ci"] != "error" || len(o["problems"].([]any)) != 0 {
		t.Errorf("no watcher, --json: %d, %v, %v", code, err, o)
	}

	p = over(t, watched)
	p.watcher = look(nil)
	code, stdout, stderr, err = run(t, p, "done", "slice-2")
	if code != 75 || err != nil || stdout != "" || !strings.HasSuffix(stderr,
		"itos: the CI run of c-4a has not appeared after 0.01 seconds (ci.watch.timeout); itos ci watch c-4a waits for it again\n") {
		t.Errorf("no run: %d, %v, %q, %q", code, err, stdout, stderr)
	}

	// A commit at HEAD that touches only the registry is passed over: the
	// commit before it is judged, by the run on the branch that has it.
	f := closable(false)
	f.config = watching
	last := &f.history[len(f.history)-1]
	last.Tree = maps.Clone(last.Tree)
	last.Tree["work-items.yaml"] = doneText
	p = over(t, f)
	v := vcsOf(p)
	v.Named["c-4a^"] = "c-3a"
	v.OnBranch = "main"
	v.Tracking = map[string][2]string{"main": {"origin", "refs/heads/main"}}
	txs = txsOf(&p)
	p.watcher = look(map[string]providers.Run{"c-4a": passed("u", "c-4a")}, passed("u", "c-4a"))
	code, stdout, _, err = run(t, p, "done", "slice-2")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "No CI run of c-3a; following u, the run of c-4a, which has it\nCI run: u\nCI passed: u\nslice-2 is done: ") ||
		len(*txs) != 2 || !strings.Contains(bodyOf((*txs)[0]),
		" The CI run of c-4a, which has c-3a, the newest commit of HEAD that touches more than work-items.yaml, passed: u.") {
		t.Errorf("a registry commit passed over: %d, %v, %q, %d commits", code, err, stdout, len(*txs))
	}
}

// With no commit at HEAD there is no run to ask for.
func TestDoneOfNoCommit(t *testing.T) {
	f := closable(false)
	f.config = watching
	f.history = nil
	code, stdout, _, err := run(t, over(t, f), "done", "slice-2")
	if code != 0 || err == nil || err.Error() != "HEAD names no commit, so it has no CI run" || stdout != "" {
		t.Errorf("work done slice-2 = %d, %v, %q", code, err, stdout)
	}
}

// The item's own commits are those whose footers name it (slice 127), the
// newest judged; an item with none has no such commit.
func TestOwnCommitIsTheNewestThatNamesTheItem(t *testing.T) {
	p := over(t, closable(false))
	if j := ownCommit(p.cfg, "slice-2", nil); j != (doneCommit{}) {
		t.Errorf("an item with no commit of its own judges %+v", j)
	}
	vcsOf(p).History = append(vcsOf(p).History, vcsfake.Commit{Name: "c-5a", Parents: []string{"c-4a"}, Message: "chore: again\n\nTask: T-1\n"})
	vcsOf(p).Named["HEAD"] = "c-5a"
	if j := ownCommit(p.cfg, "T-1", nil); j != (doneCommit{sha: "c-5a", run: "The CI run of c-5, the newest commit of T-1,", what: "c-5, the newest commit of T-1", own: true}) {
		t.Errorf("T-1 judges %+v", j)
	}
}

// judgedCommit is HEAD, or the newest commit of its first-parent line that
// touches more than the registry; with none, or a history that cannot be
// read, HEAD, and with no HEAD no commit.
func TestJudgedCommitPassesOverRegistryCommits(t *testing.T) {
	ctx := t.Context()
	registryOnly := func(name, parent, text string) vcsfake.Commit {
		c := vcsfake.Commit{Name: name, Message: "docs: " + name + "\n", Tree: map[string]string{"a": "a\n", "tasks/work-items.yaml": text}}
		if parent != "" {
			c.Parents = []string{parent}
		}
		return c
	}
	v := &vcsfake.VCS{}
	if j := judgedCommit(ctx, v, "tasks/work-items.yaml"); j != (doneCommit{run: "HEAD's CI run"}) {
		t.Errorf("no HEAD: %+v", j)
	}
	head := doneCommit{sha: "r-2a", run: "HEAD's CI run", what: "HEAD"}
	v = &vcsfake.VCS{History: []vcsfake.Commit{
		{Name: "r-0a", Message: "chore: start\n", Tree: map[string]string{"a": "a\n"}},
		registryOnly("r-1a", "r-0a", "one\n"), registryOnly("r-2a", "r-1a", "two\n"),
	}, Named: map[string]string{"HEAD": "r-2a", "r-2a^": "r-1a", "r-1a^": "r-0a"}}
	want := doneCommit{sha: "r-0a", run: "The CI run of r-0a, the newest commit of HEAD that touches more than tasks/work-items.yaml",
		what: "r-0a, the newest commit of HEAD that touches more than tasks/work-items.yaml"}
	if j := judgedCommit(ctx, v, "tasks/work-items.yaml"); j != want {
		t.Errorf("registry commits at HEAD: %+v", j)
	}
	if j := judgedCommit(ctx, v, "other.yaml"); j != head {
		t.Errorf("HEAD touching more: %+v", j)
	}
	v.History = v.History[1:]
	v.History[0].Parents = nil
	v.History[0].Tree = map[string]string{"tasks/work-items.yaml": "one\n"}
	v.History[1].Tree = map[string]string{"tasks/work-items.yaml": "two\n"}
	if j := judgedCommit(ctx, v, "tasks/work-items.yaml"); j != head {
		t.Errorf("every commit the registry's: %+v", j)
	}
}

// Only commits that touch the registry and nothing else are the
// registry's: none touched, another path or paths that cannot be read are
// not.
func TestTouchesOnlyRegistry(t *testing.T) {
	for _, c := range []struct {
		files []string
		err   error
		want  bool
	}{
		{[]string{"tasks/work-items.yaml"}, nil, true},
		{[]string{"tasks/./work-items.yaml", "tasks/work-items.yaml"}, nil, true},
		{nil, nil, false},
		{[]string{"tasks/work-items.yaml", "a"}, nil, false},
		{[]string{"tasks/work-items.yaml"}, errors.New("no diff"), false},
	} {
		if got := touchesOnlyRegistry("tasks/work-items.yaml", c.files, c.err); got != c.want {
			t.Errorf("touchesOnlyRegistry(%q, %v) = %v", c.files, c.err, got)
		}
	}
}

// The run that judged a commit is named as the commit's own, unless a head
// that has the commit ran it: then that head's run is named, with the
// commit. No run, a run with no head, the commit's own run and a commit
// with nothing to name it by are the commit's own.
func TestRunOf(t *testing.T) {
	sha, other := strings.Repeat("a", 40), strings.Repeat("b", 40)
	j := doneCommit{sha: sha, run: "HEAD's CI run", what: "HEAD"}
	for _, c := range []struct {
		j    doneCommit
		run  *providers.Run
		want string
	}{
		{j, nil, "HEAD's CI run"},
		{j, &providers.Run{}, "HEAD's CI run"},
		{j, &providers.Run{HeadSHA: sha}, "HEAD's CI run"},
		{doneCommit{sha: sha, run: "HEAD's CI run"}, &providers.Run{HeadSHA: other}, "HEAD's CI run"},
		{j, &providers.Run{HeadSHA: other}, "The CI run of bbbbbbb, which has HEAD,"},
	} {
		if got := c.j.runOf(c.run); got != c.want {
			t.Errorf("%+v.runOf(%+v) = %q, want %q", c.j, c.run, got, c.want)
		}
	}
}

// A task's late checks are CI's: work done names each as left to CI, on
// stdout, or on stderr under --json, which gives them as left_to_ci; -q says
// nothing, and a task with none, or whose check fails, names none.
func TestDoneNamesTheLateChecksLeftToCI(t *testing.T) {
	late := closable(false)
	late.files[ledgerFile] = ledgerText

	code, stdout, _, err := run(t, over(t, late), "done", "T-1")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "T-1's check `true` is left to CI: it is late (ci.cost), so work done does not run it, and the CI run below judges it\nT-1 is done: committed ") {
		t.Errorf("work done T-1 = %d, %v, %q", code, err, stdout)
	}
	code, stdout, stderr, err := run(t, over(t, late), "--json", "done", "T-1")
	o := object(t, stdout)
	left, _ := json.Marshal(o["left_to_ci"])
	if code != 0 || err != nil || string(left) != `[{"command":"true","task":"T-1"}]` || !strings.Contains(stderr, "T-1's check `true` is left to CI: it is late (ci.cost), so work done does not run it, and the CI run below judges it\n") {
		t.Errorf("work --json done T-1 = %d, %v, %v, %q", code, err, o, stderr)
	}
	if code, stdout, stderr, err := run(t, over(t, late), "-q", "done", "T-1"); code != 0 || err != nil || stdout != "" || strings.Contains(stderr, "left to CI") {
		t.Errorf("work -q done T-1 = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	_, stdout, _, _ = run(t, over(t, closable(false)), "--json", "done", "T-1")
	if o := object(t, stdout); o["left_to_ci"] != nil || o["evidence"] != nil {
		t.Errorf("a task with no late check has left_to_ci or evidence: %v", o)
	}

	p := over(t, late)
	p.failure = "`false` exited 1"
	if code, _, stderr, _ := run(t, p, "done", "T-1"); code != refused || strings.Contains(stderr, "left to CI") {
		t.Errorf("a failing check names late ones: %d, %q", code, stderr)
	}
}

// The evidence of the task checks looked up is in --json whatever came of
// the close: the close, a failing check and a CI run that did not pass.
func TestDoneGivesTheEvidenceOfTheChecksLookedUp(t *testing.T) {
	entries := []evidence.Entry{{Command: "true", Info: evidence.Info{Source: "local", Hit: true}}}
	evidenceOf := func(o map[string]any) string {
		text, _ := json.Marshal(o["evidence"])
		return string(text)
	}
	const want = `[{"command":"true","hit":true,"recorded":false,"source":"local"}]`

	p := over(t, closable(false))
	p.evidence = entries
	_, stdout, _, _ := run(t, p, "--json", "done", "T-1")
	if o := object(t, stdout); o["ok"] != true || evidenceOf(o) != want {
		t.Errorf("a close: %v", o)
	}

	p = over(t, closable(false))
	p.evidence, p.failure = entries, "`false` exited 1"
	code, stdout, _, _ := run(t, p, "--json", "done", "T-1")
	o := object(t, stdout)
	problem := o["problems"].([]any)[0].(map[string]any)
	if code != refused || evidenceOf(o) != want || problem["rule"] != "work-done-task-check" || problem["task"] != "T-1" ||
		problem["command"] != "false" || problem["exit"] != float64(0) || problem["timed_out"] != false || problem["output"] != "why\n" {
		t.Errorf("a failing check: %d, %v", code, o)
	}

	p = over(t, closable(false))
	p.failure, p.failureCode, p.failureTimedOut = "`false` ran past its timeout (1s)", 0, true
	_, stdout, _, _ = run(t, p, "--json", "done", "T-1")
	problem = object(t, stdout)["problems"].([]any)[0].(map[string]any)
	if exit, has := problem["exit"]; !has || exit != nil || problem["timed_out"] != true {
		t.Errorf("a check that timed out: %v", problem)
	}
	p = over(t, closable(false))
	p.failure, p.failureCode = "`false` was stopped", -1
	_, stdout, _, _ = run(t, p, "--json", "done", "T-1")
	if exit, has := object(t, stdout)["problems"].([]any)[0].(map[string]any)["exit"]; !has || exit != nil {
		t.Errorf("a check that did not exit by itself has exit %v", exit)
	}

	watched := closable(false)
	watched.config = watching
	p = over(t, watched)
	p.evidence = entries
	failed := providers.Run{URL: "f", Status: "completed", Conclusion: "failure", HeadSHA: "c-1a"}
	p.watcher = look(map[string]providers.Run{"c-1a": failed}, failed)
	code, stdout, _, _ = run(t, p, "--json", "done", "T-1")
	if o := object(t, stdout); code != refused || o["ci"] != "failure" || evidenceOf(o) != want {
		t.Errorf("a CI run that did not pass: %d, %v", code, o)
	}
}

// wipScenarios names an Examples block still @wip by its name, quoted, or with
// none by its line, <file>:<line>, after the scenarios still @wip, all with
// the same rule and fix.
func TestWipScenariosNamesBlocks(t *testing.T) {
	outline := tests.Test{ID: "@ID-A-01", File: "sub/a.feature", Live: true}
	got := wipScenarios([]tests.Test{{ID: "@ID-A-02", File: "b.feature"}, outline}, []tests.WipExample{
		{Test: outline, Name: "waiting on the harness", Line: 12},
		{Test: outline, Line: 20},
	}, "slice-9")
	fix := "implement it and commit it live, then run itos work done slice-9 again"
	want := []out.Problem{
		{Rule: "work-done-wip", Message: "slice-9's scenario @ID-A-02 (b.feature) is still @wip at HEAD", Fix: fix},
		{Rule: "work-done-wip", Message: `slice-9's scenario @ID-A-01 (sub/a.feature) has its Examples block "waiting on the harness" still @wip at HEAD`, Fix: fix},
		{Rule: "work-done-wip", Message: "slice-9's scenario @ID-A-01 (sub/a.feature) has its Examples block sub/a.feature:20 still @wip at HEAD", Fix: fix},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%+v, not %+v", got, want)
	}
}
