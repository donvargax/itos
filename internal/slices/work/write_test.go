package work

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
)

// writableText is a registry every write can change: T-1 and slice-2 in
// progress for a, p1-a and p1-b to do, the queue holding p1-b, a done
// slice, p1-c and p1-e deferred and p1-x dropped.
const writableText = `phases: { 1: null }
queue: [p1-b]
items:
  - { id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [], why: Adopt it. }
  - { id: p1-a, title: An idea, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: Because. }
  - { id: p1-b, title: Another, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: B }
  - { id: slice-1, title: A slice, phase: 1, owner: b, status: done, depends_on: [] }
  - { id: p1-c, title: Put off, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: C, deferred: later }
  - { id: slice-2, title: Doing, phase: 1, owner: a, status: doing, depends_on: [] }
  - { id: p1-e, title: Put off too, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: E, deferred: later }
  - { id: p1-x, title: Dropped, phase: 1, owner: null, status: dropped, depends_on: [], kind: idea, why: X }
  - { id: slice-4, title: Next, phase: 1, owner: null, status: todo, depends_on: [] }
`

// writable is a fixture whose registry is writableText, under a stealth
// config when stealth.
func writable(stealth bool) fixture {
	f := sound()
	f.files["work-items.yaml"] = writableText
	f.stealth = stealth
	return f
}

// registryIn is the registry's text in p's tracker.
func registryIn(t *testing.T, p ports) string {
	t.Helper()
	docs, err := p.tracker.Load(t.Context(), []string{"work-items.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	return docs[0].Text
}

// sha is the fake tracker's second commit, the one after the fixture's, as
// a line gives it.
const sha = "0000000"

// Each write, one id and several, its line and the commit's words on
// stderr; a change already made writes nothing.
func TestWritesSayWhatTheyDid(t *testing.T) {
	for _, c := range []struct {
		args           []string
		stdout, stderr string
		unchanged      bool
	}{
		{[]string{"take", "slice-4", "--as", "a"}, "slice-4 is in progress for a: committed " + sha + " docs: take slice-4\n", "[2] docs: take slice-4\n", false},
		{[]string{"take", "T-1", "--as", "a"}, "T-1 is already in progress for a; nothing to change\n", "", true},
		{[]string{"add", "p1-new", "--title", "New", "--why", "Because."}, "p1-new is a new idea: committed " + sha + " docs: add p1-new\n", "[2] docs: add p1-new\n", false},
		{[]string{"edit", "p1-a", "--title", "Renamed"}, "p1-a's title changed: committed " + sha + " docs: edit p1-a\n", "[2] docs: edit p1-a\n", false},
		{[]string{"edit", "p1-a", "--refs", ""}, "p1-a is already so; nothing to change\n", "", true},
		{[]string{"edit", "p1-a", "p1-b", "--refs", "x.md"}, "p1-a's refs changed\np1-b's refs changed\ncommitted " + sha + " docs: edit p1-a, p1-b\n", "[2] docs: edit p1-a, p1-b\n", false},
		{[]string{"edit", "p1-a", "p1-b", "--refs", ""}, "p1-a is already so\np1-b is already so\nnothing to change\n", "", true},
		{[]string{"queue", "p1-a", "--top"}, "p1-a is first in the queue: committed " + sha + " docs: queue p1-a\n", "[2] docs: queue p1-a\n", false},
		{[]string{"queue", "p1-a", "--before", "p1-b"}, "p1-a is before p1-b in the queue: committed " + sha + " docs: queue p1-a\n", "[2] docs: queue p1-a\n", false},
		{[]string{"queue", "p1-a", "--after", "p1-b"}, "p1-a is after p1-b in the queue: committed " + sha + " docs: queue p1-a\n", "[2] docs: queue p1-a\n", false},
		{[]string{"queue", "p1-b", "--top"}, "p1-b is already first in the queue; nothing to change\n", "", true},
		{[]string{"queue", "p1-a", "--remove"}, "p1-a is already out of the queue; nothing to change\n", "", true},
		{[]string{"queue", "p1-a", "p1-c", "--top"}, "p1-a and p1-c are first in the queue, in that order\ncommitted " + sha + " docs: queue p1-a, p1-c\n", "[2] docs: queue p1-a, p1-c\n", false},
		{[]string{"queue", "p1-a", "p1-c", "--remove"}, "p1-a and p1-c are already out of the queue\nnothing to change\n", "", true},
		{[]string{"drop", "p1-a", "--why", "Gone."}, "p1-a is dropped: committed " + sha + " docs: drop p1-a\n", "[2] docs: drop p1-a\n", false},
		{[]string{"drop", "p1-x", "--why", "Gone."}, "p1-x is already dropped; nothing to change\n", "", true},
		{[]string{"drop", "p1-x", "p1-a", "--why", "Gone."}, "p1-x is already dropped\np1-a is dropped\ncommitted " + sha + " docs: drop p1-a\n", "[2] docs: drop p1-a\n", false},
		{[]string{"defer", "p1-a", "--why", "Later."}, "p1-a is deferred: committed " + sha + " docs: defer p1-a\n", "[2] docs: defer p1-a\n", false},
		{[]string{"defer", "p1-a", "p1-b", "--why", "Later."}, "p1-a is deferred\np1-b is deferred\ncommitted " + sha + " docs: defer p1-a, p1-b\n", "[2] docs: defer p1-a, p1-b\n", false},
		{[]string{"resume", "p1-c"}, "p1-c is resumed: committed " + sha + " docs: resume p1-c\n", "[2] docs: resume p1-c\n", false},
		{[]string{"resume", "p1-c", "p1-e"}, "p1-c is resumed\np1-e is resumed\ncommitted " + sha + " docs: resume p1-c, p1-e\n", "[2] docs: resume p1-c, p1-e\n", false},
		{[]string{"give-back", "T-1", "--why", "Blocked.", "--as", "a"}, "T-1 is given back, todo again for a: committed " + sha + " docs: give back T-1\n", "[2] docs: give back T-1\n", false},
		{[]string{"give-back", "T-1", "slice-2", "--why", "Blocked.", "--as", "a"}, "T-1 is given back\nslice-2 is given back\ncommitted " + sha + " docs: give back T-1, slice-2\n", "[2] docs: give back T-1, slice-2\n", false},
	} {
		p := over(t, writable(false))
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != 0 || err != nil || stdout != c.stdout || stderr != c.stderr {
			t.Errorf("work %q = %d, %v, %q, %q; want %q, %q", c.args, code, err, stdout, stderr, c.stdout, c.stderr)
		}
		if after := registryIn(t, p); (after == writableText) != c.unchanged {
			t.Errorf("work %q left the registry:\n%s", c.args, after)
		}
		p = over(t, writable(false))
		if code, stdout, stderr, err := run(t, p, append([]string{"-q"}, c.args...)...); code != 0 || err != nil || stdout != "" || stderr != "" {
			t.Errorf("work -q %q = %d, %v, %q, %q", c.args, code, err, stdout, stderr)
		}
	}
}

// Under --json a write is one object: ok, the item or the items in the
// order given, edit's changed, queue's queue, and the commit, null when
// nothing changed or under a stealth config.
func TestWritesAsJSON(t *testing.T) {
	code, stdout, _, err := run(t, over(t, writable(false)), "--json", "edit", "p1-a", "--title", "Renamed", "--tags", "cli")
	o := object(t, stdout)
	if changed, _ := o["changed"].([]any); code != 0 || err != nil || o["ok"] != true || o["item"].(map[string]any)["title"] != "Renamed" ||
		len(changed) != 2 || o["commit"] != "0000000000000000000000000000000000000002" {
		t.Errorf("work --json edit = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, over(t, writable(false)), "--json", "queue", "p1-a", "--top")
	o = object(t, stdout)
	if queue, _ := o["queue"].([]any); code != 0 || err != nil || len(queue) != 2 || queue[0] != "p1-a" || queue[1] != "p1-b" {
		t.Errorf("work --json queue = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, over(t, writable(false)), "--json", "take", "T-1", "--as", "a")
	if o = object(t, stdout); code != 0 || err != nil || o["commit"] != nil || o["item"].(map[string]any)["id"] != "T-1" {
		t.Errorf("work --json take, already so = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, over(t, writable(false)), "--json", "queue", "p1-c", "p1-a", "--after", "p1-b")
	o = object(t, stdout)
	items, _ := o["items"].([]any)
	queue, _ := o["queue"].([]any)
	if code != 0 || err != nil || len(items) != 2 || items[0].(map[string]any)["id"] != "p1-c" ||
		!slices.Equal(queue, []any{"p1-b", "p1-c", "p1-a"}) || o["commit"] == nil {
		t.Errorf("work --json queue of several = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, over(t, writable(false)), "--json", "drop", "p1-a", "p1-b", "--why", "Gone.")
	if o = object(t, stdout); code != 0 || err != nil || o["queue"] != nil || len(o["items"].([]any)) != 2 || o["commit"] == nil {
		t.Errorf("work --json drop of several = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, over(t, writable(true)), "--json", "drop", "p1-a", "p1-b", "--why", "Gone.")
	if o = object(t, stdout); code != 0 || err != nil || o["commit"] != nil {
		t.Errorf("work --json drop of several under a stealth config = %d, %v, %v", code, err, o)
	}
	for _, c := range [][]string{{"edit", "p1-a", "--note", "x"}, {"edit", "p1-a", "p1-b", "--note", "x"}, {"refuse", "nope"}, {"refuse", "nope", "p1-a"}} {
		line := c
		if c[0] == "refuse" {
			line = append([]string{"resume"}, c[1:]...)
		}
		parsed, err := Parse(line, "", true, false)
		if err != nil {
			t.Fatal(err)
		}
		if code, err := parsed.Run(t.Context(), failing{}, failing{}, NewHandler(over(t, writable(false)))); err == nil || err.Error() != "the writer is closed" {
			t.Errorf("work --json %q to a closed writer = %d, %v", line, code, err)
		}
	}
}

// Under a stealth config nothing is committed, the registry's lock held
// from the read to the write; with no --as nobody is asked, every item the
// session's and its owner left as it is.
func TestWritesUnderAStealthConfig(t *testing.T) {
	p := over(t, writable(true))
	code, stdout, stderr, err := run(t, p, "take", "slice-4")
	if code != 0 || err != nil || stderr != "" || stdout != "slice-4 is in progress with no owner recorded: the registry is the stealth config's, so nothing is committed\n" {
		t.Errorf("work take under a stealth config = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if !strings.Contains(registryIn(t, p), "{ id: slice-4, title: Next, phase: 1, owner: null, status: doing,") {
		t.Errorf("the take wrote:\n%s", registryIn(t, p))
	}
	code, stdout, _, err = run(t, p, "give-back", "T-1", "slice-2", "--why", "Blocked.")
	if code != 0 || err != nil || stdout != "T-1 is given back\nslice-2 is given back\nthe registry is the stealth config's, so nothing is committed\n" {
		t.Errorf("work give-back of several under a stealth config = %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = run(t, p, "--json", "add", "p1-new", "--title", "New", "--why", "Because.")
	if o := object(t, stdout); code != 0 || err != nil || o["commit"] != nil || o["item"].(map[string]any)["id"] != "p1-new" {
		t.Errorf("work --json add under a stealth config = %d, %v, %v", code, err, o)
	}
	// Another writer holding the lock is the error, nothing read.
	held, err := p.tracker.Hold(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release(t.Context())
	if code, _, _, err := run(t, p, "resume", "p1-c"); code != 0 || err == nil || err.Error() != "the tracker is held by another writer" {
		t.Errorf("work resume with the lock held = %d, %v", code, err)
	}
}

// work add mints the id of a slice, a task or a bug past the registry's
// ids, an idea keeping its author's; a mint that fails is the error.
func TestWorkAddMints(t *testing.T) {
	p := over(t, writable(false))
	var given []string
	p.minted, p.given = "slice-3", &given
	code, stdout, _, err := run(t, p, "add", "--kind", "slice", "--title", "Two", "--why", "A slice.")
	if code != 0 || err != nil || stdout != "slice-3 is a new slice: committed "+sha+" docs: add slice-3\n" ||
		!slices.Equal(given, []string{"slice", "T-1", "p1-a", "p1-b", "slice-1", "p1-c", "slice-2", "p1-e", "p1-x", "slice-4"}) {
		t.Errorf("work add --kind slice = %d, %v, %q, given %q", code, err, stdout, given)
	}
	if !strings.Contains(registryIn(t, p), "id: slice-3") {
		t.Errorf("the add wrote:\n%s", registryIn(t, p))
	}
	given = nil
	if code, stdout, _, err := run(t, p, "add", "p1-y", "--title", "Y", "--why", "An idea."); code != 0 || err != nil ||
		!strings.HasPrefix(stdout, "p1-y is a new idea: ") || given != nil {
		t.Errorf("work add of an idea = %d, %v, %q, given %q", code, err, stdout, given)
	}
	p = over(t, writable(false))
	p.mintErr = errors.New("no counter")
	if code, stdout, _, err := run(t, p, "add", "--kind", "bug", "--title", "B", "--why", "A bug."); code != 0 || err == nil ||
		err.Error() != "no counter" || stdout != "" || registryIn(t, p) != writableText {
		t.Errorf("work add with a mint that fails = %d, %v, %q", code, err, stdout)
	}
}

// A write refused writes nothing: its problems on stderr with their fixes,
// exit 1, or 3 for a person who is nobody or not among the people; a
// change of several ids refused names every id refused; under --json one
// object.
func TestWritesRefused(t *testing.T) {
	unknown := "no item nope in the registry\n  fix: itos work list --all names every item\n"
	for _, c := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"take", "nope", "--as", "a"}, refused, unknown},
		{[]string{"take", "slice-4"}, notAmong, "nobody is asked\n  fix: pass --as <handle>\n"},
		{[]string{"take", "slice-4", "--as", "zed"}, notAmong, "\"zed\" is not in people.yaml"},
		{[]string{"take", "p1-a", "--as", "a"}, refused, "p1-a is an idea"},
		{[]string{"add", "p1-a", "--title", "New", "--why", "Because."}, refused, "p1-a is already an item: An idea"},
		{[]string{"edit", "nope", "--note", "x"}, refused, unknown},
		{[]string{"edit", "p1-a", "nope", "--note", "x"}, refused, "work edit refuses nope, so nothing is written:\n" + unknown},
		{[]string{"queue", "nope", "--top"}, refused, unknown},
		{[]string{"queue", "p1-a", "nope", "--top"}, refused, "work queue refuses nope, so nothing is written:\n"},
		{[]string{"drop", "slice-1", "--why", "x"}, refused, "slice-1 is done"},
		{[]string{"drop", "p1-a", "slice-1", "--why", "x"}, refused, "work drop refuses slice-1, so nothing is written:\nslice-1 is done"},
		{[]string{"defer", "p1-c", "--why", "x"}, refused, "p1-c is already deferred: later"},
		{[]string{"defer", "p1-a", "p1-c", "--why", "x"}, refused, "work defer refuses p1-c, so nothing is written:\n"},
		{[]string{"resume", "p1-a"}, refused, "p1-a is not deferred"},
		{[]string{"resume", "p1-c", "p1-a"}, refused, "work resume refuses p1-a, so nothing is written:\n"},
		{[]string{"give-back", "T-1", "--why", "x", "--as", "b"}, refused, "T-1 is a's, not b's"},
		{[]string{"give-back", "T-1", "--why", "x", "--as", "zed"}, notAmong, "is not in people.yaml"},
		{[]string{"give-back", "T-1", "slice-2", "--why", "x", "--as", "zed"}, notAmong, "is not in people.yaml"},
		{[]string{"give-back", "T-1", "p1-a", "--why", "x", "--as", "a"}, refused, "work give-back refuses p1-a, so nothing is written:\n"},
	} {
		p := over(t, writable(false))
		code, stdout, stderr, err := run(t, p, c.args...)
		if code != c.code || err != nil || stdout != "" || !strings.HasPrefix(stderr, c.stderr) && !strings.Contains(stderr, c.stderr) {
			t.Errorf("work %q = %d, %v, %q, %q; want %d, %q", c.args, code, err, stdout, stderr, c.code, c.stderr)
		}
		if registryIn(t, p) != writableText {
			t.Errorf("work %q wrote the registry", c.args)
		}
		code, stdout, stderr, err = run(t, over(t, writable(false)), append([]string{"--json"}, c.args...)...)
		if o := object(t, stdout); code != c.code || err != nil || stderr != "" || o["ok"] != false || len(o["problems"].([]any)) == 0 {
			t.Errorf("work --json %q = %d, %v, %v, %q", c.args, code, err, o, stderr)
		}
	}
	_, stdout, _, _ := run(t, over(t, writable(false)), "--json", "resume", "p1-c", "p1-a", "nope")
	if o := object(t, stdout); !slices.Equal(o["refused"].([]any), []any{"p1-a", "nope"}) || len(o["problems"].([]any)) != 2 {
		t.Errorf("work --json resume of several refused = %v", o)
	}
	_, stdout, _, _ = run(t, over(t, writable(false)), "--json", "give-back", "T-1", "slice-2", "--why", "x", "--as", "zed")
	if o := object(t, stdout); o["refused"] != nil || o["problems"].([]any)[0].(map[string]any)["rule"] != "work-no-person" {
		t.Errorf("work --json give-back of several for nobody = %v", o)
	}
}

// The person work take and work give-back act for is the identity
// provider's handle with no --as, which must be among the people.
func TestWritesActForTheProvidersPerson(t *testing.T) {
	says := func(handle string) providers.Identity {
		return func() providers.Answer { return providers.Answer{Handle: handle} }
	}
	p := over(t, writable(false))
	p.identity = says("b")
	if code, stdout, _, err := run(t, p, "take", "slice-4"); code != 0 || err != nil || !strings.HasPrefix(stdout, "slice-4 is in progress for b: ") {
		t.Errorf("work take as the provider's b = %d, %v, %q", code, err, stdout)
	}
	p = over(t, writable(false))
	p.identity = says("r")
	for _, line := range [][]string{{"take", "slice-4"}, {"give-back", "T-1", "--why", "x"}, {"give-back", "T-1", "slice-2", "--why", "x"}} {
		code, stdout, stderr, err := run(t, p, line...)
		if code != notAmong || err != nil || stdout != "" ||
			stderr != "r is not in people.yaml, so cannot own an item\n  fix: add r to people.yaml, or pass --as <handle>\n" {
			t.Errorf("work %q as the provider's r = %d, %v, %q, %q", line, code, err, stdout, stderr)
		}
	}
}

// A registry with problems refuses every write before anything is read or
// minted, exit 1, as work check reports them, with their fixes.
func TestWritesNeedASoundRegistry(t *testing.T) {
	f := writable(false)
	f.files["work-items.yaml"] = "phases: { 1: null }\nitems:\n  - { id: a, title: A, phase: 1, owner: null, status: todo, depends_on: [nope] }\n"
	for _, line := range [][]string{
		{"take", "a", "--as", "a"}, {"add", "--kind", "slice", "--title", "T", "--why", "W"}, {"edit", "a", "--note", "x"},
		{"queue", "a", "b", "--top"}, {"drop", "a", "--why", "x"},
	} {
		p := over(t, f)
		var given []string
		p.given = &given
		code, stdout, stderr, err := run(t, p, line...)
		if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "nope") || !strings.Contains(stderr, "  fix: ") || given != nil {
			t.Errorf("work %q of a registry not sound = %d, %v, %q, %q", line, code, err, stdout, stderr)
		}
	}
}

// tracked is the tracker's fake with its reads, its commits or the lock's
// release failing when set.
type tracked struct {
	trackerport.Tracker
	load, transact, release error
}

func (t tracked) Load(ctx context.Context, names []string) ([]trackerport.Doc, error) {
	if t.load != nil {
		return nil, t.load
	}
	return t.Tracker.Load(ctx, names)
}

func (t tracked) Transact(ctx context.Context, tx trackerport.Transaction) (string, error) {
	if t.transact != nil {
		return "", t.transact
	}
	return t.Tracker.Transact(ctx, tx)
}

func (t tracked) Hold(ctx context.Context) (trackerport.Held, error) {
	if t.release != nil {
		return stuck{t.release}, nil
	}
	return t.Tracker.Hold(ctx)
}

// stuck is a lock that cannot be given back.
type stuck struct{ err error }

func (s stuck) Release(context.Context) error { return s.err }

// What ends a write: a config that cannot be read, a registry the tracker
// cannot read, one a writer cannot edit in place (a usage error), a commit
// that fails or is refused; a lock that cannot be given back is said on
// stderr after everything else.
func TestWritesEnd(t *testing.T) {
	lines := [][]string{
		{"take", "slice-4", "--as", "a"}, {"add", "p1-new", "--title", "T", "--why", "W"}, {"edit", "p1-a", "--note", "x"},
		{"edit", "p1-a", "p1-b", "--note", "x"}, {"queue", "p1-a", "--top"}, {"queue", "p1-a", "p1-c", "--top"},
		{"drop", "p1-a", "--why", "x"}, {"drop", "p1-a", "p1-b", "--why", "x"}, {"defer", "p1-a", "--why", "x"},
		{"defer", "p1-a", "p1-b", "--why", "x"}, {"resume", "p1-c"}, {"resume", "p1-c", "p1-e"},
		{"give-back", "T-1", "--why", "x", "--as", "a"}, {"give-back", "T-1", "slice-2", "--why", "x", "--as", "a"},
	}
	flow := writable(false)
	flow.files["work-items.yaml"] = "{ phases: { 1: null }, queue: [p1-b], items: [" +
		"{ id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [] }, " +
		"{ id: p1-a, title: An idea, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: A }, " +
		"{ id: p1-b, title: Another, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: B }, " +
		"{ id: p1-c, title: Put off, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: C, deferred: later }, " +
		"{ id: slice-2, title: Doing, phase: 1, owner: a, status: doing, depends_on: [] }, " +
		"{ id: p1-e, title: Put off too, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: E, deferred: later }, " +
		"{ id: slice-4, title: Next, phase: 1, owner: null, status: todo, depends_on: [] }] }\n"
	for _, line := range lines {
		p := over(t, writable(false))
		p.cfg, p.err = nil, errors.New("no config")
		if code, _, _, err := run(t, p, line...); code != 0 || err == nil || err.Error() != "no config" {
			t.Errorf("work %q with no config = %d, %v", line, code, err)
		}
		p = over(t, writable(false))
		p.tracker = tracked{Tracker: p.tracker, load: errors.New("no read")}
		if code, _, _, err := run(t, p, line...); code != 0 || err == nil || err.Error() != "no read" {
			t.Errorf("work %q with a registry the tracker cannot read = %d, %v", line, code, err)
		}
		p = over(t, flow)
		if code, _, _, err := run(t, p, line...); code != 0 || err == nil || kind.Of(err) != kind.Usage ||
			!strings.HasPrefix(err.Error(), "work-items.yaml cannot be edited in place: ") {
			t.Errorf("work %q of a flow registry = %d, %v", line, code, err)
		}
		p = over(t, writable(false))
		p.tracker = tracked{Tracker: p.tracker, transact: errors.New("no commit")}
		if code, stdout, _, err := run(t, p, line...); code != 0 || err == nil || err.Error() != "no commit" || stdout != "" {
			t.Errorf("work %q with a commit that fails = %d, %v, %q", line, code, err, stdout)
		}
		p = over(t, writable(false))
		p.tracker.(*trackerfake.Tracker).Refuse = true
		code, stdout, stderr, err := run(t, p, line...)
		if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "refused\n") ||
			!strings.Contains(stderr, "the commit of work-items.yaml failed (git exited 1), so work-items.yaml is as it was\n") {
			t.Errorf("work %q with a commit refused = %d, %v, %q, %q", line, code, err, stdout, stderr)
		}
		p = over(t, writable(false))
		p.tracker = tracked{Tracker: p.tracker, release: errors.New("stuck")}
		code, stdout, stderr, err = run(t, p, line...)
		if code != 0 || err != nil || stdout == "" || !strings.HasSuffix(stderr, "itos: the lock cannot be given back: stuck\n") {
			t.Errorf("work %q with a lock that cannot be given back = %d, %v, %q, %q", line, code, err, stdout, stderr)
		}
	}
}
