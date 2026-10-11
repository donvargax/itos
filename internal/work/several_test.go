package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/value"
)

const severalText = "phases: { 1: null }\nqueue:\n  - c\nitems:\n" +
	"  - { id: a, title: A, phase: 1, status: todo, kind: idea }\n" +
	"  - { id: b, title: B, phase: 1, status: todo, kind: idea, tags: [plugin] }\n" +
	"  - { id: c, title: C, phase: 1, status: todo }\n" +
	"  - { id: d, title: D, phase: 1, status: done }\n" +
	"  - { id: e, title: E, phase: 1, status: todo, depends_on: [a] }\n"

// itemsOf is the ids of the items a change of several gives.
func itemsOf(s Several) []string {
	ids := make([]string, len(s.Items))
	for i, item := range s.Items {
		ids[i] = value.String(item.At("id"))
	}
	return ids
}

// Slice 118: work edit with several ids makes the change to each, an item
// already so left out of the header and body, and one commit for all.
func TestEditEach(t *testing.T) {
	cfg := addConfigWith(t, "tags: [plugin, stealth], ")
	r := registryOf(t, severalText)
	tags := []string{"plugin"}
	s, refused, found, err := EditEach(cfg, r, severalText, []string{"a", "b", "c"}, Edits{Tags: &tags}, nil)
	if err != nil || refused != nil || found != nil {
		t.Fatal(err, refused, found)
	}
	if s.Header != "docs: edit a, c" || !slices.Equal(s.Changed, []string{"a", "c"}) || s.Unchanged ||
		s.Body != `Change, with itos work edit, a ("A"): its tags; c ("C"): its tags.` {
		t.Errorf("edit a b c: %q %q %q", s.Header, s.Body, s.Changed)
	}
	if got := itemsOf(s); !slices.Equal(got, []string{"a", "b", "c"}) || !s.Each[1].Unchanged {
		t.Errorf("the items are %q, %+v", got, s.Each)
	}
	if !strings.Contains(s.Text, "id: a, title: A, phase: 1, status: todo, kind: idea, tags: [plugin]") ||
		!strings.Contains(s.Text, "id: c, title: C, phase: 1, status: todo, tags: [plugin]") {
		t.Errorf("the text:\n%s", s.Text)
	}
	// One item changed: the commit is that item's own.
	s, _, _, _ = EditEach(cfg, r, severalText, []string{"b", "c"}, Edits{Tags: &tags}, nil)
	if s.Header != "docs: edit c" || s.Body != `Change c ("C"): its tags, with itos work edit.` {
		t.Errorf("edit b c: %q %q", s.Header, s.Body)
	}
	// None changed: nothing to write.
	s, _, _, _ = EditEach(cfg, r, severalText, []string{"b"}, Edits{Tags: &tags}, nil)
	if !s.Unchanged || s.Header != "" {
		t.Errorf("edit b: %+v", s)
	}
}

// One id refused refuses the whole, and every id refused is named, each
// judged on the registry as the ids before it left it.
func TestEachRefusesEveryID(t *testing.T) {
	cfg := addConfigWith(t, "tags: [plugin, stealth], ")
	r := registryOf(t, severalText)
	tags := []string{"plugin"}
	s, refused, found, err := EditEach(cfg, r, severalText, []string{"a", "nope", "b", "gone"}, Edits{Tags: &tags}, nil)
	if err != nil || s.Text != "" || !slices.Equal(refused, []string{"nope", "gone"}) || len(found) != 2 ||
		found[0].Rule != "work-unknown-item" || !strings.Contains(found[1].Message, "gone") {
		t.Errorf("edit a nope b gone: %+v %q %+v %v", s, refused, found, err)
	}
	// Dropping a before e, which depends on it, is refused; e before a is not.
	if _, refused, _, _ := DropEach(cfg, r, severalText, []string{"a", "e"}, "Gone."); !slices.Equal(refused, []string{"a"}) {
		t.Errorf("drop a e: refused %q", refused)
	}
	s, refused, found, err = DropEach(cfg, r, severalText, []string{"e", "a"}, "Gone.")
	if err != nil || refused != nil || found != nil {
		t.Fatal(err, refused, found)
	}
	if !strings.Contains(s.Text, "id: a, title: A, phase: 1, status: dropped") || !slices.Equal(s.Queue, nil) {
		t.Errorf("drop e a:\n%s", s.Text)
	}
}

func TestDropEach(t *testing.T) {
	cfg := addConfigWith(t, "tags: [plugin], ")
	r := registryOf(t, severalText)
	s, _, found, err := DropEach(cfg, r, severalText, []string{"b", "c"}, " Merged into slice 9. ")
	if err != nil || found != nil {
		t.Fatal(err, found)
	}
	want := `Set b ("B") and c ("C") to dropped, with itos work drop: they stay in the registry, so their ids are never given again. Merged into slice 9.`
	if s.Header != "docs: drop b, c" || s.Body != want || strings.Contains(s.Text, "queue:\n  - c") {
		t.Errorf("drop b c: %q %q\n%s", s.Header, s.Body, s.Text)
	}
}

func TestDeferAndResumeEach(t *testing.T) {
	cfg := addConfigWith(t, "tags: [plugin], ")
	r := registryOf(t, severalText)
	s, _, found, err := DeferEach(cfg, r, severalText, []string{"a", "c"}, "Later.")
	if err != nil || found != nil {
		t.Fatal(err, found)
	}
	want := `Set a ("A") and c ("C") deferred, with itos work defer: each keeps its status, its owner and its place in the queue. Later.`
	if s.Header != "docs: defer a, c" || s.Body != want {
		t.Errorf("defer a c: %q %q", s.Header, s.Body)
	}
	after := registryOf(t, s.Text)
	s, _, found, err = ResumeEach(cfg, after, s.Text, []string{"c", "a"})
	if err != nil || found != nil {
		t.Fatal(err, found)
	}
	want = `Lift the deferral, with itos work resume, of c ("C"), deferred with the reason "Later."; a ("A"), deferred with the reason "Later.".`
	if s.Header != "docs: resume c, a" || s.Body != want || s.Text != severalText {
		t.Errorf("resume c a: %q %q\n%s", s.Header, s.Body, s.Text)
	}
	if _, refused, _, _ := ResumeEach(cfg, r, severalText, []string{"a", "c"}); !slices.Equal(refused, []string{"a", "c"}) {
		t.Errorf("resume of items not deferred: refused %q", refused)
	}
}

// work queue with several ids puts them together, in the order given.
func TestQueueEach(t *testing.T) {
	text := queueText
	r := registryOf(t, text)
	for _, c := range []struct {
		ids  []string
		at   Place
		want []string
		body string
	}{
		{[]string{"c", "a"}, Place{Top: true}, []string{"c", "a", "b"}, `Put c ("C") and a ("A") first in the queue, in that order, with itos work queue.`},
		{[]string{"i", "a"}, Place{Before: "c"}, []string{"b", "i", "a", "c"}, `Put i ("I") and a ("A") before c in the queue, in that order, with itos work queue.`},
		{[]string{"a", "i"}, Place{After: "b"}, []string{"b", "a", "i", "c"}, `Put a ("A") and i ("I") after b in the queue, in that order, with itos work queue.`},
		{[]string{"c", "a"}, Place{After: "b"}, []string{"b", "c", "a"}, `Put c ("C") and a ("A") after b in the queue, in that order, with itos work queue.`},
		{[]string{"b", "a"}, Place{Remove: true}, []string{"c"}, `Take b ("B") and a ("A") out of the queue, with itos work queue.`},
	} {
		s, refused, found, err := QueueEach(r, text, c.ids, c.at)
		if err != nil || refused != nil || found != nil {
			t.Fatalf("%q %+v: %v %q %+v", c.ids, c.at, err, refused, found)
		}
		got := queueAfter(t, Change{Text: s.Text})
		if !slices.Equal(got, c.want) || !slices.Equal(s.Queue, c.want) || s.Body != c.body ||
			s.Header != "docs: queue "+strings.Join(c.ids, ", ") || !slices.Equal(s.Changed, c.ids) ||
			!slices.Equal(itemsOf(s), c.ids) {
			t.Errorf("%q %+v: the queue is %q (%q), %q %q", c.ids, c.at, got, s.Queue, s.Header, s.Body)
		}
	}
	// Already so: nothing to write.
	if s, _, _, _ := QueueEach(r, text, []string{"b", "c"}, Place{Top: true}); !s.Unchanged || s.Changed != nil {
		t.Errorf("b c already first: %+v", s)
	}
	for _, c := range []struct {
		ids     []string
		at      Place
		refused []string
		rules   []string
	}{
		{[]string{"a", "z", "d"}, Place{Top: true}, []string{"z", "d"}, []string{"work-unknown-item", "work-queue-done"}},
		{[]string{"a", "b"}, Place{Before: "b"}, []string{"b"}, []string{"work-queue-itself"}},
		{[]string{"a", "i"}, Place{After: "d"}, []string{"a", "i"}, []string{"work-queue-not-queued"}},
		{[]string{"z", "d"}, Place{Remove: true}, []string{"z"}, []string{"work-unknown-item"}},
	} {
		s, refused, found, err := QueueEach(r, text, c.ids, c.at)
		rules := make([]string, len(found))
		for i, p := range found {
			rules[i] = p.Rule
		}
		if err != nil || s.Text != "" || !slices.Equal(refused, c.refused) || !slices.Equal(rules, c.rules) {
			t.Errorf("%q %+v: %v %q %q", c.ids, c.at, err, refused, rules)
		}
	}
}

// The header names the ids while they fit the header lint's 100
// characters, else counts them.
func TestSeveralHeader(t *testing.T) {
	ids := []string{"p1-work-edit-several-ids", "p1-tag-this-registry", "p1-push-proof-stable-site"}
	if got := severalHeader("drop", ids); got != "docs: drop "+strings.Join(ids, ", ") {
		t.Errorf("three ids: %q", got)
	}
	if got := severalHeader("drop", append(ids, "p1-fourteen-ch")); len(got) != 100 {
		t.Errorf("100 characters: %q", got)
	}
	ids = append(ids, "p1-fifteen-char")
	long := "docs: drop " + strings.Join(ids, ", ")
	if len(long) != 101 || severalHeader("drop", ids) != "docs: drop 4 items" {
		t.Errorf("%d characters: %q", len(long), severalHeader("drop", ids))
	}
	if got := severalHeader("drop", []string{strings.Repeat("é", 89)}); !strings.HasSuffix(got, "é") {
		t.Errorf("100 UTF-16 units fit: %q", got)
	}
}

// A writer's error, or a text it leaves that does not read, ends the change
// with nothing to write; a text the queue cannot be written into is the
// queue's error.
func TestEachErrors(t *testing.T) {
	cfg := addConfigWith(t, "tags: [plugin], ")
	r := registryOf(t, severalText)
	failing := func(Registry, string, string) (Change, []out.Problem, error) {
		return Change{}, nil, errors.New("cannot edit")
	}
	if _, _, _, err := each(cfg, r, severalText, []string{"a"}, failing); err == nil || err.Error() != "cannot edit" {
		t.Errorf("a failing writer: %v", err)
	}
	unreadable := func(Registry, string, string) (Change, []out.Problem, error) {
		return Change{Text: "items: 5\n"}, nil, nil
	}
	if _, _, _, err := each(cfg, r, severalText, []string{"a"}, unreadable); err == nil {
		t.Error("a text that does not read was taken")
	}
	if _, _, _, err := QueueEach(registryOf(t, queueText), "{ queue: [b, c] }\n", []string{"a", "c"}, Place{Top: true}); err == nil {
		t.Error("a flow registry was written")
	}
	if _, refused, found, err := DeferEach(cfg, r, severalText, []string{"d", "a"}, "Later."); err != nil ||
		!slices.Equal(refused, []string{"d"}) || len(found) != 1 || found[0].Rule != "work-defer-done" {
		t.Errorf("defer d a: %q %+v %v", refused, found, err)
	}
}
