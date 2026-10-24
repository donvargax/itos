package work

import (
	"strings"
	"testing"
)

// Slice 122: work give-back sets an item in progress back to todo, its owner
// and everything else kept, the reason first in the commit's body; work show
// reads its header back.
func TestGiveBack(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nqueue:\n  - s\nitems:\n" +
		"  - id: s\n    title: S\n    phase: 1\n    owner: p\n    status: doing\n    kind: slice\n" +
		"  - { id: t, title: T, phase: 1, owner: null, status: doing, kind: task }\n"
	r := registryOf(t, text)
	change, found, err := GiveBack(cfg, r, text, "s", "p", false, " Blocked on another project. ")
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	want := strings.Replace(text, "status: doing\n    kind: slice", "status: todo\n    kind: slice", 1)
	if change.Text != want || change.Header != "docs: give back s" ||
		change.Body != `Set s ("S") back to todo, with itos work give-back: it keeps its owner, p. Blocked on another project.` ||
		change.Item.At("status") != "todo" || change.Item.At("owner") != "p" {
		t.Errorf("give back s:\n got %q\nwant %q\n%+v", change.Text, want, change)
	}
	if id, ok := RegistryHeader(change.Header); !ok || id != "s" {
		t.Errorf("%q names %q, %v", change.Header, id, ok)
	}
	// An item nobody owns is anyone's to give back, its body naming no owner.
	nobody, found, err := GiveBack(cfg, r, text, "t", "p", false, "Later.")
	if err != nil || len(found) > 0 || nobody.Body != `Set t ("T") back to todo, with itos work give-back. Later.` {
		t.Errorf("give back t: %+v, %v, %v", nobody, found, err)
	}
	// A session that owns every item gives back anyone's.
	if _, found, err := GiveBack(cfg, r, text, "s", "q", true, "Later."); err != nil || len(found) > 0 {
		t.Errorf("give back s, every item the session's: %v, %v", found, err)
	}
}

func TestGiveBackRefuses(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: o }\nitems:\n" +
		"  - { id: d, title: D, phase: 1, owner: p, status: done }\n" +
		"  - { id: t, title: T, phase: 1, owner: p, status: todo }\n" +
		"  - { id: a, title: A, phase: 1, owner: another, status: doing }\n" +
		"  - { id: g, title: G, phase: 1, owner: null, status: doing }\n"
	r := registryOf(t, text)
	for _, c := range []struct{ id, rule, says string }{
		{"z", "work-unknown-item", "z"},
		{"d", "work-give-back-status", "d is done"},
		{"t", "work-give-back-status", "t is todo"},
		{"a", "work-give-back-owned", "a is another's, not p's"},
		{"g", "work-give-back-owned", "g is o's, not p's"},
	} {
		_, found, err := GiveBack(cfg, r, text, c.id, "p", false, "A reason.")
		if err != nil || len(found) != 1 || found[0].Rule != c.rule || !strings.Contains(found[0].Message, c.says) {
			t.Errorf("give back %s: %+v, %v", c.id, found, err)
		}
	}
	// What work check would find with the item given back is refused: here
	// an item that depends on itself.
	broken := "phases: { 1: null }\nitems:\n  - { id: b, title: B, phase: 1, owner: p, status: doing, depends_on: [b] }\n"
	if _, found, err := GiveBack(cfg, registryOf(t, broken), broken, "b", "p", false, "A reason."); err != nil || len(found) == 0 {
		t.Errorf("give back b, a registry work check refuses: %v, %v", found, err)
	}
	for _, bad := range []string{
		"{ phases: { 1: o }, items: [{ id: a }] ",                                                               // does not parse
		"phases: { 1: o }\nitems:\n  - d\n  - t\n  - a\n",                                                       // the item is no mapping
		"phases: { 1: o }\nitems:\n  - { id: d }\n  - { id: t }\n  - { id: a, status: &s doing, refs: [*s] }\n", // an anchored status
	} {
		if _, _, err := GiveBack(cfg, r, bad, "a", "another", false, "A reason."); err == nil {
			t.Errorf("give back a in %q: no error", bad)
		}
	}
}

// Several ids are each given back, the reason after them in the one body.
func TestGiveBackEach(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: a, title: A, phase: 1, owner: p, status: doing }\n" +
		"  - { id: b, title: B, phase: 1, owner: p, status: doing }\n" +
		"  - { id: c, title: C, phase: 1, owner: p, status: todo }\n"
	r := registryOf(t, text)
	s, refused, found, err := GiveBackEach(cfg, r, text, []string{"a", "b"}, "p", false, "Blocked.")
	if err != nil || len(found) > 0 || s.Header != "docs: give back a, b" ||
		s.Body != `Set a ("A") and b ("B") back to todo, with itos work give-back: each keeps its owner. Blocked.` ||
		strings.Count(s.Text, "status: todo") != 3 {
		t.Errorf("give back a b: %+v, %v, %v, %v", s, refused, found, err)
	}
	if _, refused, found, err := GiveBackEach(cfg, r, text, []string{"a", "c", "z"}, "p", false, "Blocked."); err != nil ||
		strings.Join(refused, " ") != "c z" || len(found) != 2 {
		t.Errorf("give back a c z: %v, %v, %v", refused, found, err)
	}
	// One refused is refused as much as two: nothing is given back.
	if s, refused, found, err := GiveBackEach(cfg, r, text, []string{"a", "c"}, "p", false, "Blocked."); err != nil ||
		strings.Join(refused, " ") != "c" || len(found) != 1 || s.Text != "" || s.Header != "" {
		t.Errorf("give back a c: %+v, %v, %v, %v", s, refused, found, err)
	}
}
