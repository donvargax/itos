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
	*VCS
	fault string
}

var errBroken = errors.New("broken")

func (b broken) Folder(ctx context.Context) string {
	if b.fault == "folder elsewhere" {
		return "elsewhere"
	}
	return b.VCS.Folder(ctx)
}

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
	case b.fault == "locate an empty folder" && err != nil && dir == "":
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
	case b.fault == "resolve tags unpeeled":
		if object, ok := b.Named["refs/tags/"+rev]; ok {
			return object, nil
		}
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

func (b broken) UnpushedBases(ctx context.Context, to string) ([]string, bool) {
	bases, adds := b.VCS.UnpushedBases(ctx, to)
	switch {
	case b.fault == "unpushed bases not independent" && len(bases) > 0:
		bases = append(bases, b.History[0].Name)
	case b.fault == "unpushed bases adds never":
		adds = false
	case b.fault == "unpushed bases adds always":
		adds = true
	case b.fault == "unpushed bases of nothing add nothing" && len(bases) == 0 && adds && !b.VCS.Readable(ctx, port.Range{From: port.Unpushed, To: to}):
		adds = false
	case b.fault == "unpushed bases lose one" && len(bases) > 1:
		bases = bases[:1]
	}
	return bases, adds
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
		{"locate top", map[string][]string{"locate": {`Locate("") is`}, "init": {"Locate after Init is"}}},
		{"locate worktree common", map[string][]string{"locate": {`Locate("../wt") is`}}},
		{"locate git folder top", map[string][]string{"locate": {`Locate(".git") is`}}},
		{"locate outside", map[string][]string{"locate": {"Locate outside a repository"}}},
		{"find git folder", map[string][]string{"find": {`Find(".git") finds`}}},
		{"find relative", map[string][]string{"find": {`Find("") is`, `Find("sub") is`, `Find("../wt") is`}}},
		{"find outside", map[string][]string{"find": {`Find("../outside") finds`}}},
		{"resolve head", map[string][]string{"resolve": {"Resolve(HEAD) is"}, "cherry pick": {"CherryPick(s onto b) moved HEAD"}, "reset": {"Reset(b): HEAD at"}, "commit": {"Commit of c.txt alone: parents"}, "commit file": {"CommitFile moved HEAD"}, "rebase": {"Rebase(c) of ours, which has c"},
			"merge": {"Merge(s) into ours: parents", "Merge(t) into c with ff", "Merge(t) into ours, both changing"}}},
		{"resolve nothing", map[string][]string{"resolve": {"Resolve of no commit"}}},
		{"resolve no remote", map[string][]string{"resolve": {"Resolve(refs/remotes/origin/b): "}, "fetch": {"Fetch(origin) of its branches: origin/theirs"}}},
		{"resolve remote as head", map[string][]string{"resolve": {"Resolve: the remote branch is"}, "fetch": {"Fetch(origin) of its branches: origin/theirs"}}},
		{"ancestor not itself", map[string][]string{"ancestry": {"IsAncestor(m, m)"}}},
		{"ancestry by first parents", map[string][]string{"ancestry": {"IsAncestor(s, m)"}}},
		{"ancestry of nothing", map[string][]string{"ancestry": {"IsAncestor(nope, m)", "IsAncestor(a, nope)"}}},
		{"ancestry of any", map[string][]string{"ancestry": {"IsAncestor(s, c)", "IsAncestor(m, a)"}, "rebase": {"Rebase(s) of ours: on"}}},
		{"parent last", map[string][]string{"parents": {"Parent(m)"}}},
		{"parent no empty tree", map[string][]string{"parents": {"Parent(a)", "Parent(nope)"}}},
		{"parents first only", map[string][]string{"parents": {"Parents(m)"},
			"merge": {"Merge(s) into ours: parents", "Merge(t) into c, which a fast-forward"}}},
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
		{"unpushed bases not independent", map[string][]string{"unpushed bases": {"UnpushedBases(m) with remote branches at [\"b\"]", "UnpushedBases(m) with remote branches at [\"c\"]", "UnpushedBases(m) with remote branches at [\"c\" \"s\"]"}}},
		{"unpushed bases adds never", map[string][]string{"unpushed bases": {"UnpushedBases(m) with remote branches at [\"b\"]", "UnpushedBases(m) with remote branches at [\"c\"]", "UnpushedBases(m) with remote branches at []", "UnpushedBases(m) with remote branches at [\"c\" \"s\"]", "UnpushedBases(nope)"}}},
		{"unpushed bases adds always", map[string][]string{"unpushed bases": {"UnpushedBases(b)"}}},
		{"unpushed bases of nothing add nothing", map[string][]string{"unpushed bases": {"UnpushedBases(nope)"}}},
		{"unpushed bases lose one", map[string][]string{"unpushed bases": {"UnpushedBases(m) with remote branches at [\"c\" \"s\"]"}}},
		{"shallow always", map[string][]string{"shallow": {"Shallow: a whole history"}}},
		{"shallow never", map[string][]string{"shallow": {"Shallow: a clone one commit deep"}}},
		{"show subject as first line", map[string][]string{"show": {"Show(c) is"}}},
		{"show message trimmed", map[string][]string{"show": shown("Show(%s) is"), "commit": {"Commit of c.txt alone: parents", "Commit of the index: paths"}, "commit file": {"CommitFile with no parent"}}},
		{"show note of another ref", map[string][]string{"show": {"Show(b) is"}}},
		{"show no note", map[string][]string{"show": {"Show(c) is"}, "notes": {"Note(a, Task: T-2)", "Note(c, Task: T-3)"}}},
		{"show no author", map[string][]string{"show": shown("Show(%s) is"), "commit file": {"CommitFile with no parent"}}},
		{"show merge never", map[string][]string{"show": {"Show(m) is"}}},
		{"show short whole", map[string][]string{"show": shown("Show(%s): the short name")}},
		{"show of nothing", map[string][]string{"show": {"Show of no commit"}}},
		{"show of a merge fails", map[string][]string{"show": {"Show(m): "}, "log": {"Show(m): "}, "merge": {"Merge(s) into ours: parents"}}},
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
		{"staged against the empty tree", map[string][]string{"staged": {"Staged(HEAD) are"}, "commit": {"Commit of c.txt alone left"}}},
		{"staged of nothing", map[string][]string{"staged": {"Staged against no commit"}}},
		{"staged own as staged", map[string][]string{"staged": {"StagedOwnPaths(c, s) are"}}},
		{"staged own of nothing", map[string][]string{"staged": {"StagedOwnPaths on no commit"}}},
		{"changes unstaged only", map[string][]string{"work tree": {"Changes are"}}},
		{"changes untracked listed", map[string][]string{"work tree": {"Changes are"}}},
		{"changes of a clean tree", map[string][]string{"work tree": {"Changes of a clean work tree"}, "reset": {"Reset(b): HEAD at"}}},
		{"path status untracked hidden", map[string][]string{"work tree": {"PathStatus(u.txt)"}, "staging": {"Untrack(new.txt)"}}},
		{"path status of every path", map[string][]string{"work tree": {"PathStatus(c.txt)", "PathStatus(u.txt)", "PathStatus(s.txt)"}, "staging": {"Add(u.txt)", "Unstage(a.txt)", "Untrack(new.txt)", "Restore(c.txt)", "Unstage(b.txt)", "Add(b.txt)"}}},
		{"tracked by the work tree", map[string][]string{"work tree": {"Tracked(u.txt)"}, "staging": {"Untrack(new.txt)"}}},
		{"tracked by HEAD", map[string][]string{"work tree": {"Tracked(new.txt)", "Tracked(b.txt)"}, "staging": {"Add(u.txt)"}}},
		{"branch never", map[string][]string{"state": {"Branch in a clean work tree", "Branch in a merge"}, "cherry pick": {"CherryPick(s onto b) moved HEAD"}, "reset": {"Reset(b): HEAD at"}, "commit": {"Commit of c.txt alone: parents"}, "commit file": {"CommitFile moved HEAD"}, "rebase": {"Rebase(s) of ours: on"}, "merge": {"Merge(s) into ours: parents"}}},
		{"branch always main", map[string][]string{"state": {"Branch in a rebase"}, "rebase": {"Rebase(s) of ours: on"}, "merge": {"Merge(s) into ours: parents"}}},
		{"rebasing never", map[string][]string{"state": {"Rebasing in a rebase"}, "rebase": {"Rebase(t) of ours"}}},
		{"rebasing when conflicted", map[string][]string{"state": {"Rebasing in a merge"}}},
		{"conflicted none", map[string][]string{"state": {"Conflicted in a merge", "Conflicted in a rebase"}, "rebase": {"Rebase(t) of ours"},
			"merge": {"Merge(t) into ours, both changing"}}},
		{"merge heads none", map[string][]string{"state": {"MergeHeads in a merge"}, "merge": {"Merge(t) into ours, both changing"}}},
		{"merge heads in a rebase", map[string][]string{"state": {"MergeHeads in a rebase"}}},
		{"file deleted as empty", map[string][]string{"files at a tree": {"File(m, x.md)"}, "the index's files": {"File(the index, b.txt)"}, "commit a deletion": {"Commit of b.txt's deletion: paths"}}},
		{"file of nothing", map[string][]string{"files at a tree": {"File of no commit"}}},
		{"files unfiltered", map[string][]string{"files at a tree": {`Files(a, "x.md")`, `Files(m, "nope")`}}},
		{"files of nothing", map[string][]string{"files at a tree": {"Files of no commit"}}},
		{"texts of a missing file left out", map[string][]string{"files at a tree": {"Texts(m) of a file m lacks"}}},
		{"texts of the head", map[string][]string{"files at a tree": {"Texts of no commit"}, "the index's files": {"Texts(the index, a.txt)"}, "commit file": {"CommitFile on the first"}, "cherry pick": {"CherryPick(s onto b): "}}},
		{"patch against nothing", map[string][]string{"patches": {"Patch(c)"}}},
		{"patch of nothing", map[string][]string{"patches": {"Patch of no commit"}}},
		{"shown notes always", map[string][]string{"patches": {"Shown(c) without notes"}}},
		{"shown of nothing", map[string][]string{"patches": {"Shown of no commit"}}},
		{"ref peeled", map[string][]string{"refs": {"Ref(refs/tags/r)", "Refs(refs/tags/) name refs/tags/r "}}},
		{"ref of nothing", map[string][]string{"refs": {"Ref of no ref"}}},
		{"refs unpeeled", map[string][]string{"refs": {"Refs(refs/tags/) name refs/tags/r^{}"}}},
		{"refs unfiltered", map[string][]string{"refs": {"Refs(refs/tags/) are", "Refs(refs/nope/)"}}},
		{"symref never", map[string][]string{"remotes": {"Symref(refs/remotes/origin/HEAD)"}}},
		{"symref plain", map[string][]string{"remotes": {"Symref(refs/heads/main)"}}},
		{"symref short", map[string][]string{"remotes": {"Symref(refs/remotes/origin/HEAD)"}}},
		{"symref of nothing", map[string][]string{"remotes": {"Symref of no ref"}}},
		{"upstream always set", map[string][]string{"remotes": {"Upstream(theirs)"}}},
		{"upstream never set", map[string][]string{"remotes": {"Upstream(main)"}}},
		{"remotes of remote branches", map[string][]string{"remotes": {"Remotes with remote branches"}}},
		{"remote url of nothing", map[string][]string{"remotes": {"RemoteURL of no remote"}}},
		{"remote refs unfiltered", map[string][]string{"remote refs": {"RemoteRefs(origin, refs/heads/main)"}}},
		{"remote refs failing plainly", map[string][]string{"remote refs": {"RemoteRefs of no remote", "RemoteRefs of an unreachable remote"}}},
		{"fetch no destination", map[string][]string{"fetch": {"Fetch: refs/fetched is"}}},
		{"fetch of a missing branch", map[string][]string{"fetch": {"Fetch of a branch origin lacks"}}},
		{"fetch without branches", map[string][]string{"fetch": {"Fetch(origin) of its branches: origin/theirs"}}},
		{"fetch failure unknown", map[string][]string{"fetch": {"Fetch of no remote"}}},
		{"push forced", map[string][]string{"push": {"Push(a to main), back", "Push(a to main) with a stale lease", "Push(a to main) with a lease on theirs",
			"Push(m to main, a to theirs) with a lease on main", "Push atomic", "Push of no commit"}}},
		{"push lease ignored", map[string][]string{"push": {"Push(a to main) with its lease", "Push(m to main, a to theirs) with a lease on main"}}},
		{"push lease forces", map[string][]string{"push": {"Push(a to main) with a stale lease", "Push(a to main) with a lease on theirs",
			"Push(m to main, a to theirs) with a lease on main", "Push atomic"}}},
		{"push not atomic", map[string][]string{"push": {"Push(m to main, a to theirs) with a lease on main", "Push atomic"}}},
		{"push nowhere fine", map[string][]string{"push": {"Push to no remote"}}},
		{"rebase stops nothing", map[string][]string{"rebase": {"Rebase of no commit", "Rebase(t) of ours"}}},
		{"rebase detaches", map[string][]string{"rebase": {"Rebase(s) of ours: on"}}},
		{"commit of everything", map[string][]string{"commit": {"Commit of c.txt alone: parents", "Commit of c.txt alone left", "Commit of the index:"}, "commit a deletion": {"Commit of b.txt's deletion: paths"}}},
		{"commit paragraphs joined", map[string][]string{"commit": {"Commit of c.txt alone: parents"}}},
		{"commit of nothing", map[string][]string{"commit": {"Commit of nothing staged"}}},
		{"add of nothing", map[string][]string{"staging": {"Add of a file nothing has"}}},
		{"unstage untracks", map[string][]string{"staging": {"Unstage(a.txt)", "Unstage(b.txt)", "Add(b.txt)"}}},
		{"untrack deletes", map[string][]string{"staging": {"Untrack(new.txt)"}}},
		{"restore index only", map[string][]string{"staging": {"Restore(c.txt)", "Restore of a file HEAD lacks"}}},
		{"restore of nothing", map[string][]string{"staging": {"Restore of a file HEAD lacks"}}},
		{"note appended", map[string][]string{"notes": {"Note(c, Task: T-3)"}}},
		{"note of nothing", map[string][]string{"notes": {"Note of no commit"}}},
		{"commit file on HEAD", map[string][]string{"commit file": {"CommitFile with no parent"}}},
		{"commit file alone", map[string][]string{"commit file": {"CommitFile on the first", "CommitFile on no commit"}}},
		{"commit file on nothing", map[string][]string{"commit file": {"CommitFile on no commit"}}},
		{"commit file moves head", map[string][]string{"commit file": {"CommitFile moved HEAD"}}},
		{"init nothing", map[string][]string{"init": {"Locate after Init is"}}},
		{"ignored nothing", map[string][]string{"ignored": {"NotIgnored(debug.log)", "NotIgnored(../outside/x)"}}},
		{"ignored untracked", map[string][]string{"ignored": {"NotIgnored(u.txt)"}}},
		{"file of the first parent", map[string][]string{"files at a tree": {"File(m, a.txt)"}}},
		{"file of the index as HEAD", map[string][]string{"the index's files": {"File(the index, new.txt)", "File(the index, b.txt)"}}},
		{"files of the index as HEAD", map[string][]string{"the index's files": {"Files(the index)"}}},
		{"texts trimmed", map[string][]string{"files at a tree": {"Texts(m, a.txt s.txt)"}, "the index's files": {"Texts(the index, a.txt)"}, "commit file": {"CommitFile on the first"}, "cherry pick": {"CherryPick(s onto b): "},
			"merge": {"Merge(s) into ours: parents"}}},
		{"shown without notes", map[string][]string{"patches": {`Shown(b, c) with notes does not say "Task: T-1"`}}},
		{"ref short", map[string][]string{"refs": {"Ref(refs/tags/m)", "Refs(refs/tags/) name refs/tags/r "}, "fetch": {"Fetch: refs/fetched is"}}},
		{"resolve tags unpeeled", map[string][]string{"refs": {"Resolve(r)"}}},
		{"remotes none", map[string][]string{"remotes": {"Remotes are"}}},
		{"remotes outside fine", map[string][]string{"init": {"Remotes in no repository"}}},
		{"remote url empty", map[string][]string{"remotes": {"RemoteURL(origin)"}}},
		{"remote refs failing", map[string][]string{"remote refs": {"RemoteRefs(origin) are", "RemoteRefs of no remote", "RemoteRefs of an unreachable remote"}}},
		{"down reached as missing", map[string][]string{"remote refs": {"RemoteRefs of an unreachable remote"}}},
		{"fetch gives nothing", map[string][]string{"fetch": {"Fetch(origin, theirs)", "Fetch(origin, main into refs/fetched)"}}},
		{"fetch of branches fails", map[string][]string{"fetch": {"Fetch(origin) of its branches: broken", "Fetch(origin) of its branches: origin/theirs"}}},
		{"push nothing", map[string][]string{"push": {"Push(m to main)", "Push(a to main), back", "Push(a to main) with a stale lease",
			"Push(a to main) with a lease on theirs", "Push(a to main) with its lease", "Push(m to main, a to theirs) with a lease on main",
			"Push atomic", "Push to no remote", "Push of no commit"}}},
		{"push of nothing fine", map[string][]string{"push": {"Push of no commit"}}},
		{"rebase refuses", map[string][]string{"rebase": {"Rebase(s) of ours: broken", "Rebase(c) of ours, which has c"}}},
		{"rebase resets", map[string][]string{"rebase": {"Rebase(c) of ours", "Rebase(t) of ours"}}},
		{"rebase of nothing fine", map[string][]string{"rebase": {"Rebase of no commit"}}},
		{"merge refuses", map[string][]string{"merge": {"Merge(s) into ours: broken"}}},
		{"merge detaches", map[string][]string{"merge": {"Merge(s) into ours: parents", "Merge(t) into c with ff"}}},
		{"merge of what HEAD has", map[string][]string{"merge": {"Merge(c), which HEAD has"}}},
		{"merge of nothing fine", map[string][]string{"merge": {"Merge of no commit"}}},
		{"merge fast-forwards", map[string][]string{"merge": {"Merge(t) into c, which a fast-forward"}}},
		{"merge never fast-forwards", map[string][]string{"merge": {"Merge(t) into c with ff"}}},
		{"merge stops nothing", map[string][]string{"merge": {"Merge(t) into ours, both changing"}}},
		{"commit of only refused", map[string][]string{"commit": {"Commit of c.txt alone: broken"}, "commit a deletion": {"Commit of b.txt's deletion: broken"}}},
		{"commit of only kept", map[string][]string{"commit a deletion": {"Commit of b.txt's deletion: paths"}}},
		{"add of a deletion kept", map[string][]string{"staging": {"Add(b.txt)"}}},
		{"commit file of nothing fails", map[string][]string{"commit file": {"CommitFile with no parent: broken"}}},
		{"init fails", map[string][]string{"init": {"Init: broken"}}},
		{"locate an empty folder", map[string][]string{"init": {"Locate in no repository is"}}},
		{"folder elsewhere", map[string][]string{"locate": {"Folder is \"elsewhere\""}}},
		{"cherry pick moves head", map[string][]string{"cherry pick": {"CherryPick(s onto b) moved HEAD"}}},
		{"cherry pick keeps the note", map[string][]string{"cherry pick": {"CherryPick(c onto s): "}}},
		{"cherry pick conflict is a failure", map[string][]string{"cherry pick": {"CherryPick(t onto s), both changing"}}},
		{"cherry pick of nothing conflicts", map[string][]string{"cherry pick": {"CherryPick(nope onto b) is", "CherryPick(s onto nope) is"}}},
		{"cherry pick onto the head", map[string][]string{"cherry pick": {"CherryPick(s onto b): ", "CherryPick(c onto s): ", "CherryPick(s onto nope) is"}}},
		{"reset of nothing fine", map[string][]string{"reset": {"Reset of no commit"}}},
		{"reset keeps changes", map[string][]string{"reset": {"Reset(b): HEAD at"}}},
		{"reset leaves the branch", map[string][]string{"reset": {"Reset(b): HEAD at"}}},
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
