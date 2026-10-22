package porttest

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// T is what VCSContract asks of the test it runs in: *testing.T is one,
// and so is the recorder that proves each assertion fires.
type T[C any] interface {
	Helper()
	Context() context.Context
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Run(name string, f func(t C)) bool
}

// Layout is the repository VCSContract asks open for: Remotes are the tags
// remote branches point at (refs/remotes/origin/<tag>), and Shallow a
// shallow clone of the history, one commit deep, of which only Shallow is
// asked.
type Layout struct {
	Remotes []string
	Shallow bool
}

// The folders VCSContract locates from the repository's top.
const (
	sub      = "sub"
	worktree = "../wt"
	gitDir   = ".git"
	outside  = "../outside"
)

// VCSContract holds what every VCS does, the fake and the adapter's alike.
// open stands the test at the top of a repository laid out as l asks and
// gives it with the top, absolute:
//
//   - its history is a, b on a, c on b, s on a and m merging s into c, made
//     in that order, each tagged with its name; HEAD is m;
//   - sub is a folder of the work tree, ../wt a linked worktree of it and
//     ../outside a folder in no repository; the git folder is .git.
func VCSContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	main := Layout{Remotes: []string{"b"}}
	resolved := func(t C, v port.VCS, revs ...string) []string {
		t.Helper()
		shas := make([]string, len(revs))
		for i, rev := range revs {
			sha, err := v.Resolve(t.Context(), rev)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", rev, err)
			}
			shas[i] = sha
		}
		return shas
	}
	t.Run("locate", func(t C) {
		v, root := open(t, main)
		in := func(dir, p string) string {
			if filepath.IsAbs(p) {
				return filepath.Clean(p)
			}
			return filepath.Join(root, dir, p)
		}
		common := filepath.Join(root, ".git")
		for _, c := range []struct {
			dir  string
			want port.Location
		}{
			{"", port.Location{Top: root, GitDir: common, Common: common}},
			{sub, port.Location{Top: root, GitDir: common, Common: common}},
			{worktree, port.Location{Top: filepath.Join(root, worktree), GitDir: filepath.Join(common, "worktrees", "wt"), Common: common}},
			{gitDir, port.Location{GitDir: common, Common: common}},
		} {
			l, err := v.Locate(t.Context(), c.dir)
			if err != nil {
				t.Errorf("Locate(%q): %v", c.dir, err)
				continue
			}
			got := port.Location{Top: l.Top, GitDir: in(c.dir, l.GitDir), Common: in(c.dir, l.Common)}
			if got != c.want {
				t.Errorf("Locate(%q) is %+v, read as %+v; want %+v", c.dir, l, got, c.want)
			}
		}
		if l, err := v.Locate(t.Context(), outside); err == nil {
			t.Errorf("Locate outside a repository is %+v", l)
		}
	})
	t.Run("find", func(t C) {
		v, root := open(t, main)
		common := filepath.Join(root, ".git")
		for _, c := range []struct {
			dir  string
			want port.Location
		}{
			{"", port.Location{Top: root, GitDir: common, Common: common}},
			{sub, port.Location{Top: root, GitDir: common, Common: common}},
			{worktree, port.Location{Top: filepath.Join(root, worktree), GitDir: filepath.Join(common, "worktrees", "wt"), Common: common}},
		} {
			if l, ok := v.Find(t.Context(), c.dir); !ok || l != c.want {
				t.Errorf("Find(%q) is %+v, %v; want %+v", c.dir, l, ok, c.want)
			}
		}
		for _, dir := range []string{gitDir, outside} {
			if l, ok := v.Find(t.Context(), dir); ok {
				t.Errorf("Find(%q) finds a work tree: %+v", dir, l)
			}
		}
	})
	t.Run("resolve", func(t C) {
		v, _ := open(t, main)
		m := resolved(t, v, "m")[0]
		for _, rev := range []string{"HEAD", m, "refs/tags/m"} {
			if got, err := v.Resolve(t.Context(), rev); got != m || err != nil {
				t.Errorf("Resolve(%s) is %q, %v; want m's %q", rev, got, err, m)
			}
		}
		if got := resolved(t, v, "refs/remotes/origin/b", "b"); got[0] != got[1] {
			t.Errorf("Resolve: the remote branch is %q, b %q", got[0], got[1])
		}
		if got, err := v.Resolve(t.Context(), "nope"); err == nil {
			t.Errorf("Resolve of no commit is %q", got)
		}
	})
	t.Run("ancestry", func(t C) {
		v, _ := open(t, main)
		for _, c := range []struct {
			ancestor, rev string
			want          bool
		}{{"a", "m", true}, {"s", "m", true}, {"m", "m", true}, {"s", "c", false}, {"m", "a", false}, {"nope", "m", false}, {"a", "nope", false}} {
			if got := v.IsAncestor(t.Context(), c.ancestor, c.rev); got != c.want {
				t.Errorf("IsAncestor(%s, %s) is %v", c.ancestor, c.rev, got)
			}
		}
	})
	t.Run("parents", func(t C) {
		v, _ := open(t, main)
		shas := resolved(t, v, "a", "c", "s")
		if got := v.Parent(t.Context(), "m"); got != shas[1] {
			t.Errorf("Parent(m) is %q; want c's %q", got, shas[1])
		}
		for _, rev := range []string{"a", "nope"} {
			if got := v.Parent(t.Context(), rev); got != port.EmptyTree {
				t.Errorf("Parent(%s) is %q; want the empty tree", rev, got)
			}
		}
		if got, err := v.Parents(t.Context(), "m"); !slices.Equal(got, shas[1:]) || err != nil {
			t.Errorf("Parents(m) are %q, %v; want c's and s's %q", got, err, shas[1:])
		}
		if got, err := v.Parents(t.Context(), "a"); len(got) != 0 || err != nil {
			t.Errorf("Parents(a) are %q, %v; want none", got, err)
		}
		if got, err := v.Parents(t.Context(), "nope"); err == nil {
			t.Errorf("Parents of no commit are %q", got)
		}
	})
	t.Run("first parents", func(t C) {
		v, _ := open(t, main)
		shas := resolved(t, v, "m", "c", "b", "a")
		for _, c := range []struct {
			skip, most int
			want       []string
		}{{0, 10, shas}, {1, 2, shas[1:3]}, {3, 10, shas[3:]}} {
			if got, err := v.FirstParents(t.Context(), "m", c.skip, c.most); !slices.Equal(got, c.want) || err != nil {
				t.Errorf("FirstParents(m, %d, %d) are %q, %v; want %q", c.skip, c.most, got, err, c.want)
			}
		}
		if got, err := v.FirstParents(t.Context(), "nope", 0, 10); err == nil {
			t.Errorf("FirstParents of no commit are %q", got)
		}
	})
	t.Run("ranges", func(t C) {
		v, _ := open(t, main)
		sha := map[string]string{}
		for _, rev := range []string{"a", "b", "c", "s", "m"} {
			sha[rev] = resolved(t, v, rev)[0]
		}
		for _, c := range []struct {
			r    port.Range
			want []string
		}{
			{port.Range{From: "b", To: "m"}, []string{"c", "s", "m"}},
			{port.Range{To: "m"}, []string{"a", "b", "c", "s", "m"}},
			{port.Range{From: port.Unpushed, To: "m"}, []string{"c", "s", "m"}},
			{port.Range{To: "m", Since: "c"}, []string{"s", "m"}},
			{port.Range{From: port.Unpushed, To: "m", Since: "s"}, []string{"c", "m"}},
			{port.Range{From: "m", To: "m"}, nil},
		} {
			var want []string
			for _, rev := range c.want {
				want = append(want, sha[rev])
			}
			if got, err := v.Commits(t.Context(), c.r); !slices.Equal(got, want) || err != nil {
				t.Errorf("Commits(%+v) are %q, %v; want %s %q", c.r, got, err, c.want, want)
			}
			if !v.Readable(t.Context(), c.r) {
				t.Errorf("Readable(%+v) is false", c.r)
			}
		}
		for _, r := range []port.Range{{From: "nope", To: "m"}, {From: "b", To: "nope"}, {From: port.Unpushed, To: "nope"}, {To: "m", Since: "nope"}} {
			if got, err := v.Commits(t.Context(), r); err == nil {
				t.Errorf("Commits(%+v) of no commit are %q", r, got)
			}
			if v.Readable(t.Context(), r) {
				t.Errorf("Readable(%+v) of no commit is true", r)
			}
		}
	})
	t.Run("unpushed base", func(t C) {
		for _, c := range []struct {
			remotes  []string
			to, want string
		}{
			{[]string{"b"}, "m", "b"},
			{[]string{"b"}, "b", "b"},
			{[]string{"c"}, "m", "c"},
			{nil, "m", ""},
			{[]string{"c", "s"}, "m", ""},
			{[]string{"b"}, "nope", ""},
		} {
			v, _ := open(t, Layout{Remotes: c.remotes})
			want := c.want
			if want != "" && want != c.to {
				want = resolved(t, v, want)[0]
			}
			if got := v.UnpushedBase(t.Context(), c.to); got != want {
				t.Errorf("UnpushedBase(%s) with remote branches at %q is %q; want %s %q", c.to, c.remotes, got, c.want, want)
			}
		}
	})
	t.Run("shallow", func(t C) {
		v, _ := open(t, main)
		if v.Shallow(t.Context()) {
			t.Errorf("Shallow: a whole history is shallow")
		}
		v, _ = open(t, Layout{Shallow: true})
		if !v.Shallow(t.Context()) {
			t.Errorf("Shallow: a clone one commit deep is not shallow")
		}
	})
}
