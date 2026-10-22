package porttest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// recorder runs VCSContract as testing does, each case on its own
// goroutine, a Fatalf ending it, and keeps what each case reported.
type recorder struct {
	name     string
	failures map[string][]string
}

func (r *recorder) Helper() {}

func (r *recorder) Context() context.Context { return context.Background() }

func (r *recorder) Errorf(format string, args ...any) {
	r.failures[r.name] = append(r.failures[r.name], fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Run(name string, f func(t *recorder)) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(&recorder{name: name, failures: r.failures})
	}()
	<-done
	return len(r.failures[name]) == 0
}

// broken is the fake broken for one fault.
type broken struct {
	VCS
	fault string
}

var errBroken = errors.New("broken")

func (b broken) Locate(ctx context.Context, dir string) (port.Location, error) {
	l, err := b.VCS.Locate(ctx, dir)
	switch {
	case b.fault == "locate misses" && dir == sub:
		return port.Location{}, errBroken
	case b.fault == "locate top" && dir == "":
		l.Top = "elsewhere"
	case b.fault == "locate worktree common" && dir == worktree:
		l.Common = l.GitDir
	case b.fault == "locate git folder top" && dir == gitDir:
		l.Top = b.Root
	case b.fault == "locate outside" && dir == outside:
		return port.Location{Top: b.Root}, nil
	}
	return l, err
}

func (b broken) Find(ctx context.Context, dir string) (port.Location, bool) {
	switch {
	case b.fault == "find git folder" && dir == gitDir:
		return port.Location{GitDir: b.Root}, true
	case b.fault == "find relative":
		l, err := b.VCS.Locate(ctx, dir)
		return l, err == nil && l.Top != ""
	case b.fault == "find outside" && dir == outside:
		return port.Location{Top: b.Root}, true
	}
	return b.VCS.Find(ctx, dir)
}

func (b broken) Resolve(ctx context.Context, rev string) (string, error) {
	switch {
	case b.fault == "resolve head" && rev == "HEAD",
		b.fault == "resolve nothing" && rev == "nope":
		return rev, nil
	case b.fault == "resolve no remote" && strings.HasPrefix(rev, "refs/remotes/"):
		return "", errBroken
	case b.fault == "resolve remote as head" && strings.HasPrefix(rev, "refs/remotes/"):
		return b.VCS.Resolve(ctx, "HEAD")
	}
	return b.VCS.Resolve(ctx, rev)
}

func (b broken) Shallow(ctx context.Context) bool {
	switch b.fault {
	case "shallow always":
		return true
	case "shallow never":
		return false
	}
	return b.VCS.Shallow(ctx)
}

func (b broken) IsAncestor(ctx context.Context, ancestor, rev string) bool {
	_, errA := b.VCS.Resolve(ctx, ancestor)
	_, errR := b.VCS.Resolve(ctx, rev)
	switch {
	case b.fault == "ancestor not itself" && ancestor == rev:
		return false
	case b.fault == "ancestry by first parents":
		shas, _ := b.VCS.FirstParents(ctx, rev, 0, 100)
		a, _ := b.VCS.Resolve(ctx, ancestor)
		return slices.Contains(shas, a)
	case b.fault == "ancestry of nothing" && (errA != nil || errR != nil),
		b.fault == "ancestry of any" && errA == nil && errR == nil:
		return true
	}
	return b.VCS.IsAncestor(ctx, ancestor, rev)
}

func (b broken) Parent(ctx context.Context, rev string) string {
	parents, _ := b.VCS.Parents(ctx, rev)
	switch {
	case b.fault == "parent last" && len(parents) > 1:
		return parents[len(parents)-1]
	case b.fault == "parent no empty tree" && len(parents) == 0:
		return ""
	}
	return b.VCS.Parent(ctx, rev)
}

func (b broken) Parents(ctx context.Context, rev string) ([]string, error) {
	parents, err := b.VCS.Parents(ctx, rev)
	switch {
	case b.fault == "parents first only" && len(parents) > 1:
		return parents[:1], nil
	case b.fault == "parents of nothing" && err != nil:
		return nil, nil
	case b.fault == "parents of a root fail" && err == nil && len(parents) == 0:
		return nil, errBroken
	}
	return parents, err
}

func (b broken) FirstParents(ctx context.Context, rev string, skip, most int) ([]string, error) {
	switch b.fault {
	case "first parents unskipped":
		skip = 0
	case "first parents unbounded":
		most = 100
	case "first parents of nothing":
		if _, err := b.VCS.Resolve(ctx, rev); err != nil {
			return nil, nil
		}
	}
	return b.VCS.FirstParents(ctx, rev, skip, most)
}

func (b broken) Commits(ctx context.Context, r port.Range) ([]string, error) {
	switch {
	case b.fault == "commits newest first":
		shas, err := b.VCS.Commits(ctx, r)
		slices.Reverse(shas)
		return shas, err
	case b.fault == "commits since kept":
		r.Since = ""
	case b.fault == "unpushed as every commit" && r.From == port.Unpushed:
		r.From = ""
	case b.fault == "commits of nothing" && !b.VCS.Readable(ctx, r):
		return nil, nil
	}
	return b.VCS.Commits(ctx, r)
}

func (b broken) Readable(ctx context.Context, r port.Range) bool {
	switch b.fault {
	case "readable always":
		return true
	case "readable never":
		return false
	}
	return b.VCS.Readable(ctx, r)
}

func (b broken) UnpushedBase(ctx context.Context, to string) string {
	unpushed, _ := b.VCS.Commits(ctx, port.Range{From: port.Unpushed, To: to})
	switch {
	case b.fault == "unpushed base resolved" && len(unpushed) == 0:
		sha, _ := b.VCS.Resolve(ctx, to)
		return sha
	case b.fault == "unpushed base not independent" && len(unpushed) > 0:
		return b.VCS.UnpushedBase(ctx, to) + "?"
	case b.fault == "unpushed base without remote" && len(b.remotes()) == 0:
		return b.History[0].Name
	case b.fault == "unpushed base of several" && len(b.remotes()) > 1:
		return b.remotes()[0]
	}
	return b.VCS.UnpushedBase(ctx, to)
}

// The contract fails a VCS broken for any one of its assertions, each case
// reporting what it asserts and no other case reporting anything: a
// contract that cannot fail proves nothing.
func TestVCSContractFailsEachBrokenVCS(t *testing.T) {
	commits := func(n int) []string {
		var want []string
		for range n {
			want = append(want, "Commits(")
		}
		return want
	}
	readable := func(n int) []string {
		var want []string
		for range n {
			want = append(want, "Readable(")
		}
		return want
	}
	shown := func(format string) []string {
		var want []string
		for _, name := range []string{"a", "b", "c", "s", "m"} {
			want = append(want, fmt.Sprintf(format, name))
		}
		return want
	}
	for _, c := range []struct {
		fault string
		want  map[string][]string
	}{
		{"", nil},
		{"locate misses", map[string][]string{"locate": {`Locate("sub"): `}}},
		{"locate top", map[string][]string{"locate": {`Locate("") is`}}},
		{"locate worktree common", map[string][]string{"locate": {`Locate("../wt") is`}}},
		{"locate git folder top", map[string][]string{"locate": {`Locate(".git") is`}}},
		{"locate outside", map[string][]string{"locate": {"Locate outside a repository"}}},
		{"find git folder", map[string][]string{"find": {`Find(".git") finds`}}},
		{"find relative", map[string][]string{"find": {`Find("") is`, `Find("sub") is`, `Find("../wt") is`}}},
		{"find outside", map[string][]string{"find": {`Find("../outside") finds`}}},
		{"resolve head", map[string][]string{"resolve": {"Resolve(HEAD) is"}}},
		{"resolve nothing", map[string][]string{"resolve": {"Resolve of no commit"}}},
		{"resolve no remote", map[string][]string{"resolve": {"Resolve(refs/remotes/origin/b): "}}},
		{"resolve remote as head", map[string][]string{"resolve": {"Resolve: the remote branch is"}}},
		{"ancestor not itself", map[string][]string{"ancestry": {"IsAncestor(m, m)"}}},
		{"ancestry by first parents", map[string][]string{"ancestry": {"IsAncestor(s, m)"}}},
		{"ancestry of nothing", map[string][]string{"ancestry": {"IsAncestor(nope, m)", "IsAncestor(a, nope)"}}},
		{"ancestry of any", map[string][]string{"ancestry": {"IsAncestor(s, c)", "IsAncestor(m, a)"}}},
		{"parent last", map[string][]string{"parents": {"Parent(m)"}}},
		{"parent no empty tree", map[string][]string{"parents": {"Parent(a)", "Parent(nope)"}}},
		{"parents first only", map[string][]string{"parents": {"Parents(m)"}}},
		{"parents of nothing", map[string][]string{"parents": {"Parents of no commit"}}},
		{"parents of a root fail", map[string][]string{"parents": {"Parents(a)"}}},
		{"first parents unskipped", map[string][]string{"first parents": {"FirstParents(m, 1, 2)", "FirstParents(m, 3, 10)"}}},
		{"first parents unbounded", map[string][]string{"first parents": {"FirstParents(m, 1, 2)"}}},
		{"first parents of nothing", map[string][]string{"first parents": {"FirstParents of no commit"}}},
		{"commits newest first", map[string][]string{"ranges": commits(5)}},
		{"commits since kept", map[string][]string{"ranges": commits(3)}},
		{"unpushed as every commit", map[string][]string{"ranges": commits(2)}},
		{"commits of nothing", map[string][]string{"ranges": commits(4)}},
		{"readable always", map[string][]string{"ranges": readable(4)}},
		{"readable never", map[string][]string{"ranges": readable(6)}},
		{"unpushed base resolved", map[string][]string{"unpushed base": {"UnpushedBase(b)"}}},
		{"unpushed base not independent", map[string][]string{"unpushed base": {"UnpushedBase(m) with remote branches at [\"b\"]", "UnpushedBase(m) with remote branches at [\"c\"]", "UnpushedBase(m) with remote branches at [] ", "UnpushedBase(m) with remote branches at [\"c\" \"s\"]"}}},
		{"unpushed base without remote", map[string][]string{"unpushed base": {"UnpushedBase(m) with remote branches at [] "}}},
		{"unpushed base of several", map[string][]string{"unpushed base": {"UnpushedBase(m) with remote branches at [\"c\" \"s\"]"}}},
		{"shallow always", map[string][]string{"shallow": {"Shallow: a whole history"}}},
		{"shallow never", map[string][]string{"shallow": {"Shallow: a clone one commit deep"}}},
		{"show subject as first line", map[string][]string{"show": {"Show(c) is"}}},
		{"show message trimmed", map[string][]string{"show": shown("Show(%s) is")}},
		{"show note of another ref", map[string][]string{"show": {"Show(b) is"}}},
		{"show no note", map[string][]string{"show": {"Show(c) is"}}},
		{"show no author", map[string][]string{"show": shown("Show(%s) is")}},
		{"show merge never", map[string][]string{"show": {"Show(m) is"}}},
		{"show short whole", map[string][]string{"show": shown("Show(%s): the short name")}},
		{"show of nothing", map[string][]string{"show": {"Show of no commit"}}},
		{"show of a merge fails", map[string][]string{"show": {"Show(m): "}, "log": {"Show(m): "}}},
		{"log newest first", map[string][]string{"log": {"Log({From:b ", "Log({From: To:m Since:c}", "Log({From:--remotes "}}},
		{"log since kept", map[string][]string{"log": {"Log({From: To:m Since:c}"}}},
		{"log of nothing", map[string][]string{"log": {"Log of no commit"}}},
		{"log without notes", map[string][]string{"log": {"Log({From:b ", "Log({From:--remotes "}}},
		{"comment char always #", map[string][]string{"comment char": {"CommentChar set to ;"}}},
		{"comment char unset empty", map[string][]string{"comment char": {"CommentChar unset"}}},
		{"commit paths of a root none", map[string][]string{"commit paths": {"CommitPaths(a) are"}}},
		{"commit paths of a merge against its first parent", map[string][]string{"commit paths": {"CommitPaths(m) are"}}},
		{"commit paths of nothing", map[string][]string{"commit paths": {"CommitPaths of no commit"}}},
		{"own paths against the first parent", map[string][]string{"commit paths": {"OwnPaths(m) are"}}},
		{"own paths of one parent", map[string][]string{"commit paths": {"OwnPaths(b) are"}}},
		{"own paths of nothing", map[string][]string{"commit paths": {"OwnPaths of no commit"}}},
		{"diff from the empty tree fails", map[string][]string{"diff": {"Diff(the empty tree, a) are"}}},
		{"diff without deletions", map[string][]string{"diff": {"Diff(b, m) are"}}},
		{"diff of nothing", map[string][]string{"diff": {"Diff(nope, m) of no commit", "Diff(b, nope) of no commit"}}},
		{"touched oldest first", map[string][]string{"touched": {"Touched(a..m) without merges", "Touched(a..m) with merges"}}},
		{"touched merges always", map[string][]string{"touched": {"Touched(a..m) without merges"}}},
		{"touched merges never", map[string][]string{"touched": {"Touched(a..m) with merges"}}},
		{"touched of nothing", map[string][]string{"touched": {"Touched of no commit"}}},
		{"staged against HEAD always", map[string][]string{"staged": {"Staged(c) are"}}},
		{"staged against the empty tree", map[string][]string{"staged": {"Staged(HEAD) are"}}},
		{"staged of nothing", map[string][]string{"staged": {"Staged against no commit"}}},
		{"staged own as staged", map[string][]string{"staged": {"StagedOwnPaths(c, s) are"}}},
		{"staged own of nothing", map[string][]string{"staged": {"StagedOwnPaths on no commit"}}},
		{"changes unstaged only", map[string][]string{"work tree": {"Changes are"}}},
		{"changes untracked listed", map[string][]string{"work tree": {"Changes are"}}},
		{"changes of a clean tree", map[string][]string{"work tree": {"Changes of a clean work tree"}}},
		{"path status untracked hidden", map[string][]string{"work tree": {"PathStatus(u.txt)"}}},
		{"path status of every path", map[string][]string{"work tree": {"PathStatus(c.txt)", "PathStatus(u.txt)", "PathStatus(s.txt)"}}},
		{"tracked by the work tree", map[string][]string{"work tree": {"Tracked(u.txt)"}}},
		{"tracked by HEAD", map[string][]string{"work tree": {"Tracked(new.txt)", "Tracked(b.txt)"}}},
		{"branch never", map[string][]string{"state": {"Branch in a clean work tree", "Branch in a merge"}}},
		{"branch always main", map[string][]string{"state": {"Branch in a rebase"}}},
		{"rebasing never", map[string][]string{"state": {"Rebasing in a rebase"}}},
		{"rebasing when conflicted", map[string][]string{"state": {"Rebasing in a merge"}}},
		{"conflicted none", map[string][]string{"state": {"Conflicted in a merge", "Conflicted in a rebase"}}},
		{"merge heads none", map[string][]string{"state": {"MergeHeads in a merge"}}},
		{"merge heads in a rebase", map[string][]string{"state": {"MergeHeads in a rebase"}}},
	} {
		r := &recorder{failures: map[string][]string{}}
		VCSContract(r, func(_ *recorder, l Layout) (port.VCS, string) {
			v, root := layout(l)
			return broken{v, c.fault}, root
		})
		for name, got := range r.failures {
			want := c.want[name]
			ok := len(got) == len(want)
			for i := 0; ok && i < len(got); i++ {
				ok = strings.HasPrefix(got[i], want[i])
			}
			if !ok {
				t.Errorf("%q: case %q reported %q; want %q", c.fault, name, got, want)
			}
		}
		for name := range c.want {
			if _, ok := r.failures[name]; !ok {
				t.Errorf("%q: case %q reported nothing; want %q", c.fault, name, c.want[name])
			}
		}
	}
}
