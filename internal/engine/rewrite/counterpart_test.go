package rewrite

import (
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// Two commits made alike (the same author, message and second) need two
// counterparts: one commit of the new head cannot stand in for both.
func TestDroppedMatchesCounterpartsOneToOne(t *testing.T) {
	history := func(newCommits ...porttest.Commit) *porttest.VCS {
		return &porttest.VCS{History: append([]porttest.Commit{
			commit("m1", "docs: main\n", "m", "m@m", "1 +0000"),
			commit("a1", "docs: same\n", "a", "a@a", "2 +0000", "m1"),
			commit("a2", "docs: same\n", "a", "a@a", "2 +0000", "a1"),
		}, newCommits...)}
	}
	v := history(commit("n1", "docs: same\n", "a", "a@a", "2 +0000", "m1"))
	dropped, err := Dropped(t.Context(), v, "m1", "a2", "n1")
	if got := subjects(dropped); err != nil || len(got) != 1 || got[0] != "docs: same" {
		t.Errorf("one counterpart for two commits: Dropped = %v, %v; want one dropped", got, err)
	}
	v = history(commit("n1", "docs: same\n", "a", "a@a", "2 +0000", "m1"), commit("n2", "docs: same\n", "a", "a@a", "2 +0000", "n1"))
	dropped, err = Dropped(t.Context(), v, "m1", "a2", "n2")
	if err != nil || len(dropped) != 0 {
		t.Errorf("a counterpart for each: Dropped = %v, %v; want none", subjects(dropped), err)
	}
}

// A merge commit is no work to drop, in either head: a rebase leaves it out
// and the commits it brought in are counted.
func TestDroppedSetsMergesAside(t *testing.T) {
	v := &porttest.VCS{History: []porttest.Commit{
		commit("m1", "docs: main\n", "m", "m@m", "1 +0000"),
		commit("a1", "docs: one\n", "a", "a@a", "2 +0000", "m1"),
		commit("s1", "docs: side\n", "a", "a@a", "3 +0000", "m1"),
		commit("g1", "Merge side\n", "a", "a@a", "4 +0000", "a1", "s1"),
		commit("n1", "docs: one\n", "a", "a@a", "2 +0000", "m1"),
		commit("n2", "docs: side\n", "a", "a@a", "3 +0000", "n1"),
		commit("g2", "Merge side\n", "a", "a@a", "4 +0000", "n1", "n2"),
	}}
	if dropped, err := Dropped(t.Context(), v, "m1", "g1", "n2"); err != nil || len(dropped) != 0 {
		t.Errorf("a merge rebased away: Dropped = %v, %v; want none", subjects(dropped), err)
	}
	if dropped, err := Dropped(t.Context(), v, "m1", "n2", "g2"); err != nil || len(dropped) != 0 {
		t.Errorf("a merge in the new head: Dropped = %v, %v; want none", subjects(dropped), err)
	}
	if dropped, err := Dropped(t.Context(), v, "m1", "g1", "n1"); err != nil || len(subjects(dropped)) != 1 || subjects(dropped)[0] != "docs: side" {
		t.Errorf("a merged-in commit missing: Dropped = %v, %v; want it, and not the merge", subjects(dropped), err)
	}
}
