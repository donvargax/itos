package work

import (
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/value"
)

const queueText = "phases: { 1: null }\n" +
	"queue:\n  - b\n  - c\n\n" +
	"items:\n" +
	"  - { id: a, title: A, phase: 1, owner: null, status: todo }\n" +
	"  - { id: b, title: B, phase: 1, owner: null, status: todo }\n" +
	"  - { id: c, title: C, phase: 1, owner: null, status: todo }\n" +
	"  - { id: d, title: D, phase: 1, owner: null, status: done }\n" +
	"  - { id: i, title: I, phase: 1, owner: null, status: todo, kind: idea }\n"

// queueAfter is the queue the change writes, read back from its text.
func queueAfter(t *testing.T, change Change) []string {
	t.Helper()
	raw, err := value.Parse(change.Text)
	if err != nil {
		t.Fatal(err)
	}
	return Queued(Registry{Queue: value.Prop(raw, "queue")})
}

func TestQueuePlaces(t *testing.T) {
	r := registryOf(t, queueText)
	for _, c := range []struct {
		id   string
		at   Place
		want []string
	}{
		{"a", Place{Top: true}, []string{"a", "b", "c"}},
		{"c", Place{Top: true}, []string{"c", "b"}},
		{"a", Place{Before: "c"}, []string{"b", "a", "c"}},
		{"a", Place{After: "c"}, []string{"b", "c", "a"}},
		{"b", Place{After: "c"}, []string{"c", "b"}},
		{"i", Place{After: "b"}, []string{"b", "i", "c"}},
		{"b", Place{Remove: true}, []string{"c"}},
	} {
		change, problem, err := Queue(r, queueText, c.id, c.at)
		if err != nil || problem != nil {
			t.Fatalf("%s %+v: %v %+v", c.id, c.at, err, problem)
		}
		if got := queueAfter(t, change); !slices.Equal(got, c.want) || !slices.Equal(change.Queue, c.want) {
			t.Errorf("%s %+v: the queue is %q (%q), want %q", c.id, c.at, got, change.Queue, c.want)
		}
		if change.Header != "docs: queue "+c.id {
			t.Errorf("%s: header %q", c.id, change.Header)
		}
	}
	// The registry's other lines are as they were.
	change, _, _ := Queue(r, queueText, "a", Place{Top: true})
	if want := strings.Replace(queueText, "queue:\n", "queue:\n  - a\n", 1); change.Text != want {
		t.Errorf("the text is\n%s\nwant\n%s", change.Text, want)
	}
}

func TestQueueRefusesAndLeavesAsItIs(t *testing.T) {
	r := registryOf(t, queueText)
	for _, c := range []struct {
		id   string
		at   Place
		rule string
	}{
		{"z", Place{Top: true}, "work-unknown-item"},
		{"d", Place{Top: true}, "work-queue-done"},
		{"a", Place{Before: "a"}, "work-queue-itself"},
		{"a", Place{After: "i"}, "work-queue-not-queued"},
	} {
		_, problem, err := Queue(r, queueText, c.id, c.at)
		if err != nil || problem == nil || problem.Rule != c.rule {
			t.Errorf("%s %+v: %v %+v, want %s", c.id, c.at, err, problem, c.rule)
		}
	}
	for _, c := range []struct {
		id string
		at Place
	}{{"b", Place{Top: true}}, {"c", Place{After: "b"}}, {"a", Place{Remove: true}}} {
		change, problem, err := Queue(r, queueText, c.id, c.at)
		if err != nil || problem != nil || !change.Unchanged {
			t.Errorf("%s %+v: %v %+v %+v, want unchanged", c.id, c.at, err, problem, change)
		}
	}
}

func TestQueueStartsAQueueBeforeItems(t *testing.T) {
	text := "phases: { 1: null }\n\nitems:\n  - { id: a, title: A, phase: 1, owner: null, status: todo }\n"
	change, problem, err := Queue(registryOf(t, text), text, "a", Place{Top: true})
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	want := "phases: { 1: null }\n\nqueue:\n  - a\n\nitems:\n  - { id: a, title: A, phase: 1, owner: null, status: todo }\n"
	if change.Text != want {
		t.Errorf("the text is %q, want %q", change.Text, want)
	}
}

func TestUnqueue(t *testing.T) {
	r := registryOf(t, queueText)
	change, ok, err := Unqueue(r, queueText, "b")
	if err != nil || !ok || !slices.Equal(queueAfter(t, change), []string{"c"}) || change.Header != "docs: queue b" {
		t.Errorf("b: %v %v %+v", err, ok, change)
	}
	if _, ok, err := Unqueue(r, queueText, "a"); err != nil || ok {
		t.Errorf("a, not queued: %v %v", err, ok)
	}
}

// work promote renames the idea in the queue too, so the registry stays
// sound.
func TestPromoteRenamesItInTheQueue(t *testing.T) {
	text := strings.Replace(queueText, "  - c\n", "  - c\n  - i\n", 1)
	change, problem, err := Promote(registryOf(t, text), text, "i", "slice-1", "slice", "", nil)
	if err != nil || problem != nil {
		t.Fatal(err, problem)
	}
	if got := queueAfter(t, change); !slices.Equal(got, []string{"b", "c", "slice-1"}) {
		t.Errorf("the queue is %q", got)
	}
}

func TestQueueIssues(t *testing.T) {
	cfg := &config.Loaded{}
	cfg.Work.Statuses = []string{"todo", "doing", "done"}
	for _, c := range []struct {
		queue string
		rules []string
	}{
		{"queue: [b, c]\n", nil},
		{"queue: []\n", nil},
		{"queue:\n", nil},
		{"queue: [b, z]\n", []string{"work-queue-unknown-item"}},
		{"queue: [b, c, b]\n", []string{"work-queue-twice"}},
		{"queue: b\n", []string{"work-queue-not-list"}},
	} {
		text := strings.Replace(queueText, "queue:\n  - b\n  - c\n", c.queue, 1)
		found, err := Issues(cfg, registryOf(t, text), "w.yaml")
		if err != nil {
			t.Fatal(err)
		}
		var rules []string
		for _, p := range found {
			rules = append(rules, p.Rule)
		}
		if !slices.Equal(rules, c.rules) {
			t.Errorf("%q: %q, want %q", c.queue, rules, c.rules)
		}
	}
}

// The proposal is in the queue's order, the unqueued after in the
// registry's.
func TestProposeInQueueOrder(t *testing.T) {
	p := Propose(registryOf(t, queueText), "q")
	var ids []string
	for _, item := range p.Unowned {
		ids = append(ids, value.String(item.At("id")))
	}
	if !slices.Equal(ids, []string{"b", "c", "a"}) {
		t.Errorf("unowned: %q", ids)
	}
}

// Two takes of one item clash, the other side's owner named; an item only
// one side changed does not.
func TestClashes(t *testing.T) {
	base := "items:\n  - { id: a, owner: null, status: todo }\n  - { id: b, owner: null, status: todo }\n"
	theirs := "items:\n  - { id: a, owner: ana, status: doing }\n  - { id: b, owner: null, status: todo }\n"
	mine := "items:\n  - { id: a, owner: bo, status: doing }\n  - { id: b, owner: bo, status: doing }\n"
	got := Clashes(base, theirs, mine)
	if len(got) != 1 || got[0] != (Clash{ID: "a", Owner: "ana", Taken: true}) {
		t.Errorf("%+v", got)
	}
	if got := Clashes(base, ": [", mine); got != nil {
		t.Errorf("an unreadable side: %+v", got)
	}
}
