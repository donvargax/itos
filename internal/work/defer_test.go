package work

import (
	"strings"
	"testing"
)

// Slice 98: work defer writes the reason as the item's deferred key, a
// folded text in a block mapping, keeping the item's status, owner and place
// in the queue, and puts the reason first in the commit's body; work resume
// removes the key and nothing else, the registry back as it was.
func TestDeferAndResume(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nqueue:\n  - i\nitems:\n" +
		"  - id: i\n    title: I\n    phase: 1\n    owner: null\n    status: todo\n    kind: idea\n" +
		"  - { id: s, title: S, phase: 1, status: todo, kind: slice }\n"
	r := registryOf(t, text)
	change, found, err := Defer(cfg, r, text, "i", "Waits for the next major release.")
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	want := "phases: { 1: null }\nqueue:\n  - i\nitems:\n" +
		"  - id: i\n    title: I\n    phase: 1\n    owner: null\n    status: todo\n    kind: idea\n" +
		"    deferred: >\n      Waits for the next major release.\n" +
		"  - { id: s, title: S, phase: 1, status: todo, kind: slice }\n"
	if change.Text != want || change.Header != "docs: defer i" ||
		!strings.HasPrefix(change.Body, "Waits for the next major release. Set i") ||
		change.Item.At("deferred") != "Waits for the next major release.\n" || change.Item.At("status") != "todo" {
		t.Errorf("defer i:\n got %q\nwant %q\n%+v", change.Text, want, change)
	}
	after := registryOf(t, change.Text)
	if found, err := Issues(cfg, after, "work-items.yaml"); err != nil || len(found) > 0 {
		t.Errorf("after the defer: %v, %v", found, err)
	}
	if p := Propose(after, "p"); len(p.Deferred) != 1 || len(p.Ideas) != 0 {
		t.Errorf("proposed: %+v", p)
	}
	resumed, found, err := Resume(cfg, after, change.Text, "i")
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	if resumed.Text != text || resumed.Header != "docs: resume i" || resumed.Item.Has("deferred") ||
		!strings.Contains(resumed.Body, `"Waits for the next major release."`) {
		t.Errorf("resume i:\n got %q\nwant %q\n%+v", resumed.Text, text, resumed)
	}
}

// In a flow mapping the reason is a quoted text on the item's line.
func TestDeferFlowItem(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n  - { id: s, title: S, phase: 1, status: todo, kind: slice }\n"
	change, found, err := Defer(cfg, registryOf(t, text), text, "s", "Later.")
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	want := "phases: { 1: null }\nitems:\n  - { id: s, title: S, phase: 1, status: todo, kind: slice, deferred: \"Later.\" }\n"
	if change.Text != want {
		t.Errorf("defer s:\n got %q\nwant %q", change.Text, want)
	}
}

func TestDeferRefuses(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: d, title: D, phase: 1, status: done }\n" +
		"  - { id: x, title: X, phase: 1, status: dropped }\n" +
		"  - { id: a, title: A, phase: 1, status: doing }\n" +
		"  - { id: l, title: L, phase: 1, status: todo, deferred: 'Not now.' }\n" +
		"  - { id: t, title: T, phase: 1, status: todo }\n"
	r := registryOf(t, text)
	for _, c := range []struct{ id, rule, says string }{
		{"z", "work-unknown-item", "z"},
		{"d", "work-defer-done", "done"},
		{"x", "work-defer-dropped", "dropped"},
		{"a", "work-defer-status", "doing"},
		{"l", "work-defer-deferred", "Not now."},
	} {
		_, found, err := Defer(cfg, r, text, c.id, "A reason.")
		if err != nil || len(found) != 1 || found[0].Rule != c.rule || !strings.Contains(found[0].Message, c.says) {
			t.Errorf("defer %s: %+v, %v", c.id, found, err)
		}
	}
	for _, c := range []struct{ id, rule string }{{"z", "work-unknown-item"}, {"t", "work-resume-not-deferred"}} {
		_, found, err := Resume(cfg, r, text, c.id)
		if err != nil || len(found) != 1 || found[0].Rule != c.rule {
			t.Errorf("resume %s: %+v, %v", c.id, found, err)
		}
	}
}
