package rewrite

import (
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// commit is a commit of the history, on parents, with the author, the date and
// the message a rebase keeps when it replays one.
func commit(name, message, author, email, date string, parents ...string) porttest.Commit {
	return porttest.Commit{Name: name, Parents: parents, Message: message, Author: author, Email: email, Date: date,
		Tree: map[string]string{name: name}}
}

// repository has main at m2, which grew from m1, and the old head o2 on m1
// with two commits of the branch, o1 and o2; replaying them onto m2 gives n1
// and n2, each the counterpart of the old one unless n2 is another commit.
func repository(n2 porttest.Commit) *porttest.VCS {
	return &porttest.VCS{
		History: []porttest.Commit{
			commit("m1", "docs: main\n", "m", "m@m", "1 +0000"),
			commit("o1", "docs: one\n", "a", "a@a", "2 +0000", "m1"),
			commit("o2", "docs: two\n", "a", "a@a", "3 +0000", "o1"),
			commit("m2", "docs: main again\n", "m", "m@m", "4 +0000", "m1"),
			commit("n1", "docs: one\n", "a", "a@a", "2 +0000", "m2"),
			n2,
		},
		Named: map[string]string{"refs/heads/main": "m2"},
	}
}

func subjects(entries []vcsport.Entry) []string {
	var got []string
	for _, e := range entries {
		got = append(got, e.Subject)
	}
	return got
}

func TestDroppedCountsACommitWithTheSameAuthorDateAndMessageAsItsCounterpart(t *testing.T) {
	v := repository(commit("n2", "docs: two\n", "a", "a@a", "3 +0000", "n1"))
	dropped, err := Dropped(t.Context(), v, "m2", "o2", "n2")
	if err != nil || len(dropped) != 0 {
		t.Fatalf("Dropped(m2, o2, n2) = %v, %v; want none", subjects(dropped), err)
	}
}

func TestDroppedNamesAnOldCommitWithNoCounterpart(t *testing.T) {
	for _, c := range []struct {
		name string
		n2   porttest.Commit
		want []string
	}{
		{"a reworded message", commit("n2", "docs: write two\n", "a", "a@a", "3 +0000", "n1"), []string{"docs: two"}},
		{"another author", commit("n2", "docs: two\n", "b", "a@a", "3 +0000", "n1"), []string{"docs: two"}},
		{"another email", commit("n2", "docs: two\n", "a", "b@a", "3 +0000", "n1"), []string{"docs: two"}},
		{"another date", commit("n2", "docs: two\n", "a", "a@a", "5 +0000", "n1"), []string{"docs: two"}},
		{"another zone", commit("n2", "docs: two\n", "a", "a@a", "3 +0100", "n1"), []string{"docs: two"}},
	} {
		dropped, err := Dropped(t.Context(), repository(c.n2), "m2", "o2", "n2")
		if got := subjects(dropped); err != nil || len(got) != 1 || got[0] != c.want[0] {
			t.Errorf("%s: Dropped(m2, o2, n2) = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestDroppedListsTheOldCommitsOldestFirst(t *testing.T) {
	v := repository(commit("n2", "docs: two\n", "a", "a@a", "3 +0000", "n1"))
	dropped, err := Dropped(t.Context(), v, "m2", "o2", "m2")
	if got := subjects(dropped); err != nil || len(got) != 2 || got[0] != "docs: one" || got[1] != "docs: two" {
		t.Fatalf("Dropped(m2, o2, m2) = %v, %v; want one, then two", got, err)
	}
}

func TestDroppedCountsOnlyTheCommitsMainLacks(t *testing.T) {
	v := repository(commit("n2", "docs: two\n", "a", "a@a", "3 +0000", "n1"))
	dropped, err := Dropped(t.Context(), v, "m2", "m2", "o2")
	if err != nil || len(dropped) != 0 {
		t.Fatalf("Dropped(m2, m2, o2) = %v, %v; want none: main's own head is no commit of the branch", subjects(dropped), err)
	}
	dropped, err = Dropped(t.Context(), v, "", "m2", "o2")
	if got := subjects(dropped); err != nil || len(got) != 1 || got[0] != "docs: main again" {
		t.Fatalf("Dropped(, m2, o2) = %v, %v; want main again: with no main, all of the old head's history counts", got, err)
	}
}

func TestDroppedFailsWhenARangeCannotBeRead(t *testing.T) {
	v := repository(commit("n2", "docs: two\n", "a", "a@a", "3 +0000", "n1"))
	for _, c := range [][2]string{{"nope", "n2"}, {"o2", "nope"}} {
		if dropped, err := Dropped(t.Context(), v, "m2", c[0], c[1]); err == nil {
			t.Errorf("Dropped(m2, %s, %s) = %v, nil; want an error", c[0], c[1], subjects(dropped))
		}
	}
}
