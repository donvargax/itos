package porttest

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/store/tree/port"
)

// T is what TreesContract asks of the test it runs in: *testing.T is one,
// and so is the recorder that proves each assertion fires.
type T[C any] interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Run(name string, f func(t C)) bool
}

// The paths TreesContract reads.
const (
	first  = "tasks/phase-1.yaml"
	second = "tasks/phase-2.yaml"
	done   = "tasks/done/phase-0.yaml"
	aside  = ".git/itos/tasks/phase-1.yaml"
	gone   = "tasks/gone.yaml"
)

// TreesContract holds what every tree reader does, the fake and the
// adapter's alike. open stands the test at the top of a repository when
// repository, laid out so:
//
//   - HEAD commits tasks/done/phase-0.yaml "zero\n" and tasks/phase-1.yaml
//     "one\n";
//   - the index stages tasks/phase-1.yaml "staged\n" and tasks/phase-2.yaml
//     "staged two\n" over it;
//   - the working tree changes tasks/phase-1.yaml to "worktree\n";
//   - the git folder, .git, holds itos/tasks/phase-1.yaml "aside\n".
//
// Else it stands the test in a folder in no repository holding
// tasks/phase-1.yaml "worktree\n" alone. A read's error beside these, and a
// path git would quote, stay the adapter's tests'.
func TreesContract[C T[C]](t C, open func(t C, repository bool) port.Trees) {
	t.Helper()
	at := func(t C, trees port.Trees, tree string) port.Source {
		t.Helper()
		s, err := trees.At(t.Context(), tree)
		if err != nil {
			t.Fatalf("At(%s): %v", tree, err)
		}
		return s
	}
	all := func(string) bool { return true }
	t.Run("reads", func(t C) {
		trees := open(t, true)
		for _, c := range []struct {
			tree, text string
			names      []string
			staged     bool
		}{
			{"worktree", "worktree\n", []string{"done", "phase-1.yaml", "phase-2.yaml"}, true},
			{"index", "staged\n", []string{"phase-1.yaml", "phase-2.yaml"}, true},
			{"HEAD", "one\n", []string{"phase-1.yaml"}, false},
		} {
			s := at(t, trees, c.tree)
			if got := s.Tree(t.Context()); got != c.tree {
				t.Errorf("Tree: At(%s) is the tree %q", c.tree, got)
			}
			if text, err := s.Read(t.Context(), first); text != c.text || err != nil {
				t.Errorf("Read: %s at %s is %q, %v; want %q", first, c.tree, text, err, c.text)
			}
			if names, err := s.List(t.Context(), "tasks"); !slices.Equal(names, c.names) || err != nil {
				t.Errorf("List: tasks at %s is %q, %v; want %q", c.tree, names, err, c.names)
			}
			if !s.Has(t.Context(), first) {
				t.Errorf("Has: %s lacks %s", c.tree, first)
			}
			if got := s.Has(t.Context(), second); got != c.staged {
				t.Errorf("a staged file: %s has %s is %v", c.tree, second, got)
			}
		}
	})
	t.Run("folders", func(t C) {
		trees := open(t, true)
		for _, c := range []struct {
			tree string
			want bool
		}{{"worktree", true}, {"index", false}, {"HEAD", true}} {
			if got := at(t, trees, c.tree).Has(t.Context(), "tasks"); got != c.want {
				t.Errorf("a folder: %s has tasks is %v", c.tree, got)
			}
		}
	})
	t.Run("missing", func(t C) {
		trees := open(t, true)
		worktree := at(t, trees, "worktree")
		// Node's words, which what itos prints quotes; a folder's code is the
		// platform's.
		if _, err := worktree.Read(t.Context(), gone); err == nil || err.Error() != "ENOENT: no such file or directory, open '"+gone+"'" {
			t.Errorf("a missing file at the worktree: %v", err)
		}
		if _, err := worktree.List(t.Context(), "gone"); err == nil || !strings.HasSuffix(err.Error(), ", scandir 'gone'") {
			t.Errorf("a missing folder at the worktree: %v; want an error naming scandir 'gone'", err)
		}
		for _, tree := range []string{"index", "HEAD"} {
			name := tree
			if tree == "index" {
				name = "the index"
			}
			s := at(t, trees, tree)
			if _, err := s.Read(t.Context(), gone); err == nil || err.Error() != name+" holds no "+gone {
				t.Errorf("a missing file at %s: %v", tree, err)
			}
			if _, err := s.List(t.Context(), "gone"); err == nil || err.Error() != name+" holds no folder gone" {
				t.Errorf("a missing folder at %s: %v", tree, err)
			}
		}
		for _, tree := range []string{"worktree", "index", "HEAD"} {
			if at(t, trees, tree).Has(t.Context(), gone) {
				t.Errorf("Has: %s has the missing %s", tree, gone)
			}
		}
	})
	t.Run("git folder", func(t C) {
		trees := open(t, true)
		for _, tree := range []string{"index", "HEAD"} {
			s := at(t, trees, tree)
			if !s.Has(t.Context(), aside) {
				t.Errorf("the git folder: %s lacks %s", tree, aside)
			}
			if text, err := s.Read(t.Context(), aside); text != "aside\n" || err != nil {
				t.Errorf("the git folder: %s reads %s as %q, %v", tree, aside, text, err)
			}
			if names, err := s.List(t.Context(), ".git/itos/tasks"); !slices.Equal(names, []string{"phase-1.yaml"}) || err != nil {
				t.Errorf("the git folder: %s lists .git/itos/tasks as %q, %v", tree, names, err)
			}
		}
	})
	t.Run("texts", func(t C) {
		trees := open(t, true)
		for _, c := range []struct {
			tree string
			keep func(string) bool
			want map[string]string
		}{
			{"index", all, map[string]string{done: "zero\n", first: "staged\n", second: "staged two\n"}},
			{"HEAD", all, map[string]string{done: "zero\n", first: "one\n"}},
			{"index", func(p string) bool { return p == second }, map[string]string{second: "staged two\n"}},
			{"worktree", all, map[string]string{}},
			{"no-such-commit", all, map[string]string{}},
		} {
			if got := trees.Texts(t.Context(), c.tree, "tasks", c.keep); !maps.Equal(got, c.want) {
				t.Errorf("Texts: tasks at %s is %q; want %q", c.tree, got, c.want)
			}
		}
	})
	t.Run("no repository", func(t C) {
		trees := open(t, false)
		if text, err := at(t, trees, "worktree").Read(t.Context(), first); text != "worktree\n" || err != nil {
			t.Errorf("no repository's worktree reads %s as %q, %v", first, text, err)
		}
		for _, tree := range []string{"index", "HEAD"} {
			if _, err := trees.At(t.Context(), tree); err == nil || err.Error() != tree+" cannot be read outside a git repository" {
				t.Errorf("no repository's %s: %v", tree, err)
			}
		}
		if got := trees.Texts(t.Context(), "HEAD", "tasks", all); len(got) != 0 {
			t.Errorf("no repository's texts: %q", got)
		}
	})
}
