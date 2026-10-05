package work

import (
	"slices"
	"strings"
	"testing"
)

// Slice 78: work drop sets the status dropped, drops the why, takes the
// item out of the queue, and puts the reason first in the commit's body.
func TestDrop(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nqueue:\n  - i\n  - s\nitems:\n" +
		"  - { id: i, title: I, phase: 1, status: todo, kind: idea, why: 'a gap', deferred: 'later' }\n" +
		"  - { id: s, title: S, phase: 1, status: todo, kind: slice }\n"
	r := registryOf(t, text)
	change, found, err := Drop(cfg, r, text, "i", "Superseded by s.")
	if err != nil || len(found) > 0 {
		t.Fatal(err, found)
	}
	want := "phases: { 1: null }\nqueue:\n  - s\nitems:\n" +
		"  - { id: i, title: I, phase: 1, status: dropped, kind: idea, deferred: 'later' }\n" +
		"  - { id: s, title: S, phase: 1, status: todo, kind: slice }\n"
	if change.Text != want || change.Header != "docs: drop i" || !strings.HasPrefix(change.Body, "Superseded by s. Set i") ||
		!slices.Equal(change.Queue, []string{"s"}) || change.Item.Has("why") {
		t.Errorf("drop i:\n got %q\nwant %q\n%+v", change.Text, want, change)
	}
	// The registry after is sound: a dropped idea, deferred or not, is no
	// idea started and no deferred item started.
	after := registryOf(t, change.Text)
	if found, err := Issues(cfg, after, "work-items.yaml"); err != nil || len(found) > 0 {
		t.Errorf("after the drop: %v, %v", found, err)
	}
	// Dropped again, nothing changes.
	if change, _, _ := Drop(cfg, after, change.Text, "i", "Again."); !change.Unchanged {
		t.Errorf("dropped twice: %+v", change)
	}
}

func TestDropRefuses(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: d, title: D, phase: 1, status: done }\n" +
		"  - { id: i, title: I, phase: 1, status: todo, kind: idea }\n" +
		"  - { id: a, title: A, phase: 1, status: doing, depends_on: [i] }\n" +
		"  - { id: b, title: B, phase: 1, status: blocked, depends_on: [i] }\n" +
		"  - { id: j, title: J, phase: 1, status: todo, kind: idea }\n" +
		"  - { id: x, title: X, phase: 1, status: dropped, depends_on: [j] }\n"
	r := registryOf(t, text)
	for _, c := range []struct{ id, rule, says string }{
		{"z", "work-unknown-item", "z"},
		{"d", "work-drop-done", "done"},
		{"i", "work-drop-depended-on", "a, b"},
	} {
		_, found, err := Drop(cfg, r, text, c.id, "Not needed.")
		if err != nil || len(found) != 1 || found[0].Rule != c.rule || !strings.Contains(found[0].Message, c.says) {
			t.Errorf("drop %s: %+v, %v", c.id, found, err)
		}
	}
	// Only a dropped item depends on j: it can go.
	if _, found, err := Drop(cfg, r, text, "j", "Not needed."); err != nil || len(found) > 0 {
		t.Errorf("drop j: %+v, %v", found, err)
	}
}

// work check knows dropped whatever work.statuses lists, and reports an item
// still to do that depends on a dropped one; take, done, promote and queue
// refuse a dropped item.
func TestDroppedElsewhere(t *testing.T) {
	cfg := addConfig(t)
	text := "phases: { 1: null }\nitems:\n" +
		"  - { id: x, title: X, phase: 1, status: dropped, kind: idea }\n" +
		"  - { id: a, title: A, phase: 1, status: todo, depends_on: [x] }\n"
	r := registryOf(t, text)
	found, err := Issues(cfg, r, "work-items.yaml")
	if err != nil || len(found) != 1 || found[0].Rule != "work-dropped-dependency" || !strings.Contains(found[0].Message, "never be done") {
		t.Errorf("check: %+v, %v", found, err)
	}
	if _, problem, _ := Take(r, text, "x", "p", false); problem == nil || problem.Rule != "work-take-dropped" {
		t.Errorf("take: %+v", problem)
	}
	if _, problem, _ := Done(r, text, "x"); problem == nil || problem.Rule != "work-done-dropped" {
		t.Errorf("done: %+v", problem)
	}
	if _, problem, _ := Promote(r, text, "x", "slice-1", "slice", "", nil); problem == nil || problem.Rule != "work-promote-dropped" {
		t.Errorf("promote: %+v", problem)
	}
	if _, problem, _ := Queue(r, text, "x", Place{Top: true}); problem == nil || problem.Rule != "work-queue-dropped" {
		t.Errorf("queue: %+v", problem)
	}
	if p := Propose(r, "p"); len(p.Ideas)+len(p.Next)+len(p.Unowned)+len(p.Doing) != 0 {
		t.Errorf("proposed: %+v", p)
	}
}
