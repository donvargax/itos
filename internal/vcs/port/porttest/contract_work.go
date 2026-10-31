package porttest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// sha is the commit a revision names, the case failed when it names none.
func sha[C T[C]](t C, v port.VCS, rev string) string {
	t.Helper()
	s, err := v.Resolve(t.Context(), rev)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", rev, err)
	}
	return s
}

// logContract holds Show, Log and CommentChar.
func logContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	main := Layout{Remotes: []string{"b"}}
	t.Run("show", func(t C) {
		v, _ := open(t, main)
		for i, name := range []string{"a", "b", "c", "s", "m"} {
			want := port.Entry{SHA: sha(t, v, name), Subject: name, Message: name + "\n\nthe body of " + name + "\n",
				Author: "t", Email: "t@t", Date: fmt.Sprintf("%d +0000", 1767225601+i), Merge: name == "m"}
			if name == "c" {
				want.Subject, want.Message, want.Note = "c wrapped", "c\nwrapped\n\nthe body of c\n", "Task: T-1\n"
			}
			got, err := v.Show(t.Context(), name)
			if err != nil {
				t.Errorf("Show(%s): %v", name, err)
				continue
			}
			if got.Short == "" || len(got.Short) >= len(got.SHA) || !strings.HasPrefix(got.SHA, got.Short) {
				t.Errorf("Show(%s): the short name %q is no shorter start of %q", name, got.Short, got.SHA)
			}
			want.Short = got.Short
			if got != want {
				t.Errorf("Show(%s) is %+v; want %+v", name, got, want)
			}
		}
		if got, err := v.Show(t.Context(), "nope"); err == nil {
			t.Errorf("Show of no commit is %+v", got)
		}
	})
	t.Run("log", func(t C) {
		v, _ := open(t, main)
		for _, c := range []struct {
			r     port.Range
			names []string
		}{
			{port.Range{From: "b", To: "m"}, []string{"c", "s", "m"}},
			{port.Range{To: "m", Since: "c"}, []string{"s", "m"}},
			{port.Range{From: port.Unpushed, To: "m"}, []string{"c", "s", "m"}},
		} {
			r := c.r
			var want []port.Entry
			for _, c := range c.names {
				e, err := v.Show(t.Context(), c)
				if err != nil {
					t.Fatalf("Show(%s): %v", c, err)
				}
				want = append(want, e)
			}
			if got, err := v.Log(t.Context(), r); !slices.Equal(got, want) || err != nil {
				t.Errorf("Log(%+v) is %+v, %v; want %+v", r, got, err, want)
			}
		}
		if got, err := v.Log(t.Context(), port.Range{From: "nope", To: "m"}); err == nil {
			t.Errorf("Log of no commit is %+v", got)
		}
	})
	t.Run("comment char", func(t C) {
		v, _ := open(t, main)
		if got := v.CommentChar(t.Context()); got != "#" {
			t.Errorf("CommentChar unset is %q", got)
		}
		v, _ = open(t, Layout{CommentChar: ";"})
		if got := v.CommentChar(t.Context()); got != ";" {
			t.Errorf("CommentChar set to ; is %q", got)
		}
	})
}

// wantPaths checks a list of paths a method gave against what it should be.
func wantPaths[C T[C]](t C, what string, got []string, err error, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) || err != nil {
		t.Errorf("%s are %q, %v; want %q", what, got, err, want)
	}
}

// pathsContract holds the changed paths.
func pathsContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	main := Layout{Remotes: []string{"b"}}
	t.Run("commit paths", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		got, err := v.CommitPaths(ctx, "a")
		wantPaths(t, "CommitPaths(a)", got, err, "a.txt", "x.md")
		got, err = v.CommitPaths(ctx, "b")
		wantPaths(t, "CommitPaths(b)", got, err, "a.txt", "b.txt")
		got, err = v.CommitPaths(ctx, "c")
		wantPaths(t, "CommitPaths(c)", got, err, "c.txt", "x.md")
		got, err = v.CommitPaths(ctx, "m")
		wantPaths(t, "CommitPaths(m)", got, err)
		if got, err := v.CommitPaths(ctx, "nope"); err == nil {
			t.Errorf("CommitPaths of no commit are %q", got)
		}
		got, err = v.OwnPaths(ctx, "m")
		wantPaths(t, "OwnPaths(m)", got, err, "a.txt")
		got, err = v.OwnPaths(ctx, "b")
		wantPaths(t, "OwnPaths(b)", got, err)
		if got, err := v.OwnPaths(ctx, "nope"); err == nil {
			t.Errorf("OwnPaths of no commit are %q", got)
		}
	})
	t.Run("diff", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		got, err := v.Diff(ctx, "b", "m")
		wantPaths(t, "Diff(b, m)", got, err, "a.txt", "c.txt", "s.txt", "x.md")
		got, err = v.Diff(ctx, port.EmptyTree, "a")
		wantPaths(t, "Diff(the empty tree, a)", got, err, "a.txt", "x.md")
		got, err = v.Diff(ctx, "m", "m")
		wantPaths(t, "Diff(m, m)", got, err)
		for _, ends := range [][2]string{{"nope", "m"}, {"b", "nope"}} {
			if got, err := v.Diff(ctx, ends[0], ends[1]); err == nil {
				t.Errorf("Diff(%s, %s) of no commit are %q", ends[0], ends[1], got)
			}
		}
	})
	t.Run("touched", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		r := port.Range{From: "a", To: "m"}
		got, err := v.Touched(ctx, r, false)
		wantPaths(t, "Touched(a..m) without merges", got, err, "s.txt", "c.txt", "x.md", "a.txt", "b.txt")
		got, err = v.Touched(ctx, r, true)
		wantPaths(t, "Touched(a..m) with merges", got, err, "a.txt", "s.txt", "c.txt", "x.md", "b.txt")
		if got, err := v.Touched(ctx, port.Range{From: "nope", To: "m"}, false); err == nil {
			t.Errorf("Touched of no commit are %q", got)
		}
	})
	t.Run("staged", func(t C) {
		v, _ := open(t, Layout{Dirty: true})
		ctx := t.Context()
		got, err := v.Staged(ctx, "")
		wantPaths(t, "Staged(HEAD)", got, err, "a.txt", "b.txt", "new.txt")
		got, err = v.Staged(ctx, sha(t, v, "c"))
		wantPaths(t, "Staged(c)", got, err, "a.txt", "b.txt", "new.txt", "s.txt")
		if got, err := v.Staged(ctx, "nope"); err == nil {
			t.Errorf("Staged against no commit are %q", got)
		}
		got, err = v.StagedOwnPaths(ctx, []string{sha(t, v, "c"), sha(t, v, "s")})
		wantPaths(t, "StagedOwnPaths(c, s)", got, err, "a.txt", "b.txt", "new.txt")
		got, err = v.StagedOwnPaths(ctx, []string{sha(t, v, "b"), sha(t, v, "s")})
		wantPaths(t, "StagedOwnPaths(b, s)", got, err, "a.txt", "b.txt", "c.txt", "new.txt", "x.md")
		got, err = v.StagedOwnPaths(ctx, []string{sha(t, v, "m"), sha(t, v, "t")})
		wantPaths(t, "StagedOwnPaths(m, t), in conflict on a.txt", got, err, "a.txt", "b.txt", "new.txt")
		if got, err := v.StagedOwnPaths(ctx, []string{"nope", sha(t, v, "s")}); err == nil {
			t.Errorf("StagedOwnPaths on no commit are %q", got)
		}
	})
}

// stateContract holds the work tree's state.
func stateContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	t.Run("work tree", func(t C) {
		v, _ := open(t, Layout{})
		ctx := t.Context()
		if got := v.Changes(ctx); len(got) != 0 {
			t.Errorf("Changes of a clean work tree are %q", got)
		}
		v, _ = open(t, Layout{Dirty: true})
		if got, want := v.Changes(ctx), []string{"M  a.txt", "D  b.txt", " M c.txt", "A  new.txt"}; !slices.Equal(got, want) {
			t.Errorf("Changes are %q; want %q", got, want)
		}
		for _, c := range []struct {
			path string
			want []string
		}{{"c.txt", []string{" M c.txt"}}, {"u.txt", []string{"?? u.txt"}}, {"s.txt", nil}} {
			if got, err := v.PathStatus(ctx, c.path); !slices.Equal(got, c.want) || err != nil {
				t.Errorf("PathStatus(%s) is %q, %v; want %q", c.path, got, err, c.want)
			}
		}
		for _, c := range []struct {
			path string
			want bool
		}{{"new.txt", true}, {"c.txt", true}, {"u.txt", false}, {"b.txt", false}} {
			if got := v.Tracked(ctx, c.path); got != c.want {
				t.Errorf("Tracked(%s) is %v", c.path, got)
			}
		}
	})
	t.Run("state", func(t C) {
		for _, c := range []struct {
			l          Layout
			name       string
			branch     string
			rebasing   bool
			conflicted []string
			heads      []string
		}{
			{Layout{}, "a clean work tree", "main", false, nil, nil},
			{Layout{Merging: true}, "a merge", "main", false, []string{"a.txt"}, []string{"t"}},
			{Layout{Rebasing: true}, "a rebase", "", true, []string{"a.txt"}, nil},
		} {
			v, _ := open(t, c.l)
			ctx := t.Context()
			if got := v.Branch(ctx); got != c.branch {
				t.Errorf("Branch in %s is %q", c.name, got)
			}
			if got := v.Rebasing(ctx); got != c.rebasing {
				t.Errorf("Rebasing in %s is %v", c.name, got)
			}
			if got := v.Conflicted(ctx); !slices.Equal(got, c.conflicted) {
				t.Errorf("Conflicted in %s are %q", c.name, got)
			}
			var heads []string
			for _, h := range c.heads {
				heads = append(heads, sha(t, v, h))
			}
			if got := v.MergeHeads(ctx); !slices.Equal(got, heads) {
				t.Errorf("MergeHeads in %s are %q; want %q", c.name, got, heads)
			}
		}
	})
}
