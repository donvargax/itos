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
// asked. CommentChar sets core.commentChar; Dirty, Merging and Rebasing
// leave the work tree as VCSContract says, and Ours has HEAD on the branch
// ours. Origin adds the remote origin VCSContract says; None is a folder
// in no repository at all, of which only Init is asked.
type Layout struct {
	Remotes                  []string
	Shallow                  bool
	CommentChar              string
	Dirty, Merging, Rebasing bool
	Ours, Origin, None       bool
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
//   - its history is a, b on a, c on b, s on a, m merging s into c, then t
//     and o each on c, made in that order a second apart from
//     2026-01-01T00:00:01Z by t <t@t>, each tagged with its name; HEAD is
//     m on the branch main, t is the branch theirs and o the branch ours;
//   - a holds a.txt (a) and x.md (x); b sets a.txt to b and adds b.txt
//     (b); c deletes x.md and adds c.txt (c); s adds s.txt (s); m sets
//     a.txt to m of its own; t sets it to t and o to o, each file's
//     content a line;
//   - each message is its name, a blank line and "the body of <name>", c's
//     first paragraph two lines, "c" and "wrapped"; c's itos note is
//     "Task: T-1", and b has a note of another ref;
//   - sub is a folder of the work tree, ../wt a linked worktree of it and
//     ../outside a folder in no repository; the git folder is .git;
//   - r is an annotated tag of c; the work tree ignores debug.log.
//
// Dirty stages a.txt set to staged, b.txt deleted and new.txt (new) added,
// then sets c.txt to changed in the work tree and leaves u.txt (u)
// untracked; Merging merges t into m, Rebasing rebases ours onto t, each
// stopped by the conflict on a.txt. Origin is a remote whose main is b and
// theirs t, HEAD's branch main tracking its main, beside the remote down,
// out of reach.
func VCSContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	main := Layout{Remotes: []string{"b"}}
	resolved := func(t C, v port.VCS, revs ...string) []string {
		t.Helper()
		shas := make([]string, len(revs))
		for i, rev := range revs {
			shas[i] = sha(t, v, rev)
		}
		return shas
	}
	t.Run("locate", func(t C) {
		v, root := open(t, main)
		if got := v.Folder(t.Context()); got != root {
			t.Errorf("Folder is %q, not the folder the VCS runs in, %q", got, root)
		}
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
	t.Run("unpushed bases", func(t C) {
		for _, c := range []struct {
			remotes  []string
			to       string
			want     []string
			wantAdds bool
		}{
			{[]string{"b"}, "m", []string{"b"}, true},
			{[]string{"b"}, "b", nil, false},
			{[]string{"c"}, "m", []string{"c"}, true},
			{nil, "m", nil, true},
			{[]string{"c", "s"}, "m", []string{"c", "s"}, true},
			{[]string{"b"}, "nope", nil, true},
		} {
			v, _ := open(t, Layout{Remotes: c.remotes})
			var want []string
			for _, rev := range c.want {
				want = append(want, resolved(t, v, rev)[0])
			}
			got, adds := v.UnpushedBases(t.Context(), c.to)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) || adds != c.wantAdds {
				t.Errorf("UnpushedBases(%s) with remote branches at %q are %q, %v; want %s %q, %v", c.to, c.remotes, got, adds, c.want, want, c.wantAdds)
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
	logContract(t, open)
	pathsContract(t, open)
	stateContract(t, open)
	treeContract(t, open)
	remoteContract(t, open)
	commitContract(t, open)
}
