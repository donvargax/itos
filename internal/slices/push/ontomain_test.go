package push

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The commits beside prRepo's: rb000, mine as a rebase onto origin's main
// leaves it (the same message, the author and date it had), and rc000 above it.
const (
	rbased = "rb000"
	above  = "rc000"
)

// ontoRepo is prRepo with the item's branch pushed already, at mine, and the
// commits a rebase of it onto origin's main and one above that make.
func ontoRepo(t *testing.T) *fake {
	t.Helper()
	f := prRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = mine
	tree := map[string]string{"README.md": "start\n", "up.md": "up.md\n", "mine.md": "mine.md\n"}
	more := map[string]string{"README.md": "start\n", "up.md": "up.md\n", "mine.md": "mine.md\n", "rc.md": "rc\n"}
	f.History = append(f.History,
		vcsfake.Commit{Name: rbased, Parents: []string{up}, Message: "docs: mine\n", Tree: tree},
		vcsfake.Commit{Name: above, Parents: []string{rbased}, Message: "docs: above\n", Tree: more})
	return f
}

// subjectsOf are the subjects of the commits of rev that origin's main lacks,
// oldest first.
func subjectsOf(t *testing.T, f *fake, rev string) []string {
	t.Helper()
	entries, err := f.Log(t.Context(), vcsport.Range{From: up, To: rev})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Subject)
	}
	return got
}

// --onto-main is read beside --no-wait and --ready; it takes no --tag or
// --ref, whichever comes first, and every forcing argument stays refused,
// which no lease of the caller's opens.
func TestParseOntoMain(t *testing.T) {
	c, err := Parse([]string{"--onto-main", "--no-wait", "--ready"}, false, false, "")
	if want := (Query{OntoMain: true, NoWait: true, Ready: true}); err != nil || !reflect.DeepEqual(c.Query, want) {
		t.Errorf("Parse = %+v, %v", c, err)
	}
	for _, args := range []string{"--onto-main --tag v1", "--tag v1 --onto-main", "--onto-main --ref slice-9", "--ref=slice-9 --onto-main"} {
		_, err := Parse(strings.Fields(args), false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || err.Error() != "push --onto-main rebases an item's branch onto main; it takes no --tag or --ref" {
			t.Errorf("Parse(%s) = %v", args, err)
		}
	}
	for _, force := range []string{"--force", "-f", "--force-with-lease", "--force-with-lease=slice-9:abc", "+slice-9"} {
		_, err := Parse([]string{"--onto-main", force}, false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || !strings.HasPrefix(err.Error(), "push never forces") {
			t.Errorf("Parse(--onto-main %s) = %v", force, err)
		}
	}
	if _, err := Parse([]string{"--onto-main", "--no-verify"}, false, false, ""); err == nil || !strings.Contains(err.Error(), "pre-push hook") {
		t.Errorf("Parse(--onto-main --no-verify) = %v", err)
	}
}

// Outside push.mode pr, with no config file or one that cannot be read, and
// from main, --onto-main is refused, exit 1, before anything is fetched or
// pushed and with no gh call; the outcome says which.
func TestOntoMainRefusals(t *testing.T) {
	trunk := func(f *fake) { f.cfg = loaded(t, "version: 1\n") }
	for name, c := range map[string]struct {
		set     func(*fake)
		outcome string
		says    string
	}{
		"trunk":        {trunk, "onto-main-without-pr", "push.mode is not pr"},
		"no config":    {func(f *fake) { f.found = false }, "onto-main-without-pr", "push.mode is not pr"},
		"a bad config": {func(f *fake) { f.loadErr = errors.New("bad") }, "onto-main-without-pr", "push.mode is not pr"},
		"from main":    {func(f *fake) { f.on("main", mine) }, "onto-main-on-main", "HEAD is on main, which is never rewritten"},
	} {
		t.Run(name, func(t *testing.T) {
			f := ontoRepo(t)
			c.set(f)
			code, stdout, stderr, err := call(t, f, "--onto-main")
			if code != policy || err != nil || stdout != "" || !strings.Contains(stderr, c.says) || !strings.Contains(stderr, "nothing was fetched or pushed") ||
				f.pushed != nil || f.ghCalls != nil || f.Named["refs/remotes/origin/main"] != "" {
				t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
			}
			f = ontoRepo(t)
			c.set(f)
			_, stdout, _, _ = call(t, f, "--json", "--onto-main")
			if r := reported(t, stdout); r.OK || r.Outcome != c.outcome {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
}

// An item's branch main has moved past is rebased onto main and pushed to its
// own branch with a lease on the head the push fetched, the registry's commits
// above main going to main in the same atomic push as ever; GitHub is asked
// for no rebase, the pull request is found and auto-merge is turned on.
func TestOntoMainRebasesOntoMainAndPushesWithALease(t *testing.T) {
	f := ontoRepo(t)
	f.gh = answers(opened, open7)
	code, stdout, stderr, err := call(t, f, "--onto-main")
	head := f.head(t)
	if code != 0 || err != nil || stderr != "" || !strings.HasSuffix(stdout, "Pull request: "+open7+"\n") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if parents, _ := f.Parents(t.Context(), head); len(parents) != 1 || parents[0] != up {
		t.Errorf("HEAD %s grows from %v; want origin's main", head, parents)
	}
	lease := &vcsport.Lease{Ref: "refs/heads/slice-9", Expect: mine}
	if !reflect.DeepEqual(f.leases, []*vcsport.Lease{lease}) || f.Remote["origin"].Refs["refs/heads/slice-9"] != head || f.origin() != up ||
		!reflect.DeepEqual(f.atomic, []bool{true}) {
		t.Errorf("leases %v, origin has %v, atomic %v", f.leases, f.Remote["origin"].Refs, f.atomic)
	}
	for _, call := range f.calls() {
		if strings.Contains(call, "update-branch") || strings.Contains(call, "pr view") {
			t.Errorf("GitHub was asked to rebase: %s", call)
		}
	}
}

// A branch the remote does not have yet is a first push, rebased onto main as
// ever, with no lease; one already up to date with main and the remote's head
// is pushed plainly when it adds to it, and is nothing to push when it adds
// nothing.
func TestOntoMainTakesALeaseOnlyToRewrite(t *testing.T) {
	f := prRepo(t)
	if code, _, stderr, _ := call(t, f, "--onto-main"); code != 0 || !reflect.DeepEqual(f.leases, []*vcsport.Lease{nil}) {
		t.Errorf("a first push: exit %d, leases %v\n%s", code, f.leases, stderr)
	}
	f = ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = rbased
	f.on(item, above)
	if code, _, stderr, _ := call(t, f, "--onto-main"); code != 0 || !reflect.DeepEqual(f.leases, []*vcsport.Lease{nil}) ||
		f.Remote["origin"].Refs["refs/heads/slice-9"] != above {
		t.Errorf("an addition: exit %d, leases %v\n%s", code, f.leases, stderr)
	}
	f = ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = rbased
	f.on(item, rbased)
	code, stdout, stderr, _ := call(t, f, "--onto-main")
	if code != 0 || f.pushed != nil || !strings.HasPrefix(stdout, "Nothing to push: slice-9 is at origin/slice-9. Pull request: ") {
		t.Errorf("nothing to add: exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
}

// A commit the remote's item branch has that HEAD lacks is the item's work
// too: the branch is rebased onto the remote's head first, then onto main, so
// the rewrite drops nothing. A branch already rebased onto main, one the
// person finished after a stop, is not rebased onto the remote's old head: its
// commits are the old ones' counterparts, and no rebase runs at all.
func TestOntoMainTakesTheRemotesItemHeadFirstOnlyForACommitItLacks(t *testing.T) {
	f := ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = "nw000"
	code, _, stderr, _ := call(t, f, "--onto-main")
	head := f.head(t)
	if got := subjectsOf(t, f, head); code != 0 || !reflect.DeepEqual(got, []string{"docs: mine", "test: more"}) ||
		!reflect.DeepEqual(f.leases, []*vcsport.Lease{{Ref: "refs/heads/slice-9", Expect: "nw000"}}) {
		t.Errorf("a commit of the remote's: exit %d, HEAD has %v, leases %v\n%s", code, got, f.leases, stderr)
	}
	f = ontoRepo(t)
	f.rebase = func() error { t.Error("the rbased branch was rbased again"); return nil }
	f.on(item, rbased)
	code, _, stderr, _ = call(t, f, "--onto-main")
	if !reflect.DeepEqual(f.leases, []*vcsport.Lease{{Ref: "refs/heads/slice-9", Expect: mine}}) || code != 0 ||
		f.Remote["origin"].Refs["refs/heads/slice-9"] != rbased {
		t.Errorf("a rbased branch: exit %d, leases %v\n%s", code, f.leases, stderr)
	}
	// A reworded commit has no counterpart: the remote's is still the item's
	// work, and the branch is rebased onto it first.
	f = ontoRepo(t)
	f.History[len(f.History)-2].Message = "docs: reworded\n"
	f.on(item, rbased)
	if code, _, stderr, _ := call(t, f, "--onto-main"); code != 0 || f.Remote["origin"].Refs["refs/heads/slice-9"] != f.head(t) {
		t.Errorf("a reworded commit: exit %d\n%s", code, stderr)
	}
	if got := subjectsOf(t, f, f.head(t)); !slices.Contains(got, "docs: mine") || !slices.Contains(got, "docs: reworded") {
		t.Errorf("a reworded commit: HEAD has %v", got)
	}
}

// A push the remote or the hook refuses exits 1 saying to run itos push
// --onto-main again; one that took a lease says the branch was not
// overwritten, and a lease the remote moved from is that refusal.
func TestOntoMainPushFailures(t *testing.T) {
	refused := &vcsport.Failed{Code: 1, Failure: vcsport.Refused, Words: " ! [rejected] slice-9 (stale info)\n"}
	f := ontoRepo(t)
	f.pushErr = refused
	code, _, stderr, _ := call(t, f, "--onto-main")
	if code != policy || !strings.Contains(stderr, "itos push: the push to origin/slice-9 failed (git's message above); refs/heads/slice-9 was not overwritten.\n") ||
		!strings.Contains(stderr, "run itos push --onto-main again to bring the branch up to main again") || f.ghCalls != nil {
		t.Errorf("a lease: exit %d, gh %v\n%s", code, f.ghCalls, stderr)
	}
	f = ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = rbased
	f.on(item, above)
	f.pushErr = refused
	code, _, stderr, _ = call(t, f, "--onto-main")
	if code != policy || !strings.Contains(stderr, "nothing was forced.\n") || !strings.Contains(stderr, "run itos push --onto-main again to rebase onto it") {
		t.Errorf("no lease: exit %d\n%s", code, stderr)
	}
	// The remote moved after the fetch: the lease no longer holds, the fake
	// refuses as git does, and the other clone's commit is left.
	f = ontoRepo(t)
	f.rebase = func() error {
		f.Remote["origin"].Refs["refs/heads/"+item] = "nw000"
		return f.VCS.Rebase(t.Context(), up, true, vcsport.Talk{})
	}
	code, stdout, stderr, _ := call(t, f, "--json", "--onto-main")
	if r := reported(t, stdout); code != policy || r.OK || r.Outcome != "push-failed" || f.Remote["origin"].Refs["refs/heads/slice-9"] != "nw000" {
		t.Errorf("a lost lease: exit %d, origin has %v\n%s", code, f.Remote["origin"].Refs, stderr)
	}
}

// A rebase that stops names itos push --onto-main again, whichever of the two
// it was (onto the remote's item head, onto main) and whether it stopped on a
// conflict in the work registry or not, and so do the refusals before the
// pull.
func TestOntoMainAdvice(t *testing.T) {
	stop := func(f *fake) func() error {
		return func() error { f.InRebase = true; return &vcsport.Failed{Code: 1} }
	}
	f := ontoRepo(t)
	f.rebase = stop(f)
	code, stdout, stderr, _ := call(t, f, "--onto-main")
	if code != policy || stdout != "" || f.pushed != nil || f.ghCalls != nil ||
		!strings.Contains(stderr, "itos push: the rebase onto origin/main stopped; nothing was pushed.\n") ||
		!strings.Contains(stderr, "git rebase --continue, then itos push --onto-main again;\n") || strings.Contains(stderr, "then itos push again") {
		t.Errorf("onto main: exit %d\n%s%s", code, stdout, stderr)
	}
	f = ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = "nw000"
	f.rebase = stop(f)
	code, _, stderr, _ = call(t, f, "--onto-main")
	if code != policy || !strings.Contains(stderr, "the rebase onto origin/slice-9 stopped") ||
		!strings.Contains(stderr, "then itos push --onto-main again;") {
		t.Errorf("onto the remote's head: exit %d\n%s", code, stderr)
	}
	f = ontoRepo(t)
	f.Remote["origin"].Refs["refs/heads/"+item] = "nw000"
	calls := 0
	f.rebase = func() error {
		if calls++; calls == 1 {
			return f.VCS.Rebase(t.Context(), "nw000", true, vcsport.Talk{})
		}
		return stop(f)()
	}
	if code, _, stderr, _ := call(t, f, "--onto-main"); code != policy || !strings.Contains(stderr, "the rebase onto origin/main stopped") {
		t.Errorf("onto main after the remote's head: exit %d\n%s", code, stderr)
	}
	item := func(owner string) string {
		return "phases: { 1: null }\nitems:\n  - { id: slice-9, title: nine, phase: 1, owner: " + owner + ", status: doing, depends_on: [] }\n"
	}
	for name, theirs := range map[string]string{"taken": item("ana"), "added": "phases: { 1: null }\nitems:\n  - { id: p1-ana, title: a, phase: 1, owner: null, status: todo, depends_on: [] }\n"} {
		f = ontoRepo(t)
		f.rebase = func() error {
			f.History = append(f.History,
				vcsfake.Commit{Name: "rb001", Tree: map[string]string{"tasks/work-items.yaml": item("null")}},
				vcsfake.Commit{Name: "rt001", Tree: map[string]string{"tasks/work-items.yaml": theirs}},
				vcsfake.Commit{Name: "rm001", Tree: map[string]string{"tasks/work-items.yaml": item("bo")}})
			f.Named["REBASE_HEAD^"], f.Named["HEAD"], f.Named["REBASE_HEAD"] = "rb001", "rt001", "rm001"
			f.InRebase, f.Unmerged = true, []string{"./tasks/work-items.yaml"}
			return &vcsport.Failed{Code: 1}
		}
		code, _, stderr, _ := call(t, f, "--onto-main")
		if code != policy || strings.Contains(stderr, "itos push again") || !strings.Contains(stderr, "itos push --onto-main again") ||
			!strings.Contains(stderr, "The conflict is in the work registry") {
			t.Errorf("%s: exit %d\n%s", name, code, stderr)
		}
	}
	for name, c := range map[string]struct {
		set  func(*fake)
		says string
	}{
		"a rebase in progress": {func(f *fake) { f.InRebase = true }, "then itos push --onto-main again.\n"},
		"a change": {func(f *fake) { f.Work = map[string]string{"README.md": "changed\n", "mine.md": "mine.md\n"} },
			"then run itos push --onto-main again:\n"},
		"a remote out of reach": {func(f *fake) { f.Remote["origin"].Down = true }, "Check the network, then run itos push --onto-main again.\n"},
		"no repository": {func(f *fake) { f.refsErr = &vcsport.Failed{Code: 128, Failure: vcsport.NoRepository} },
			"then run itos push --onto-main again.\n"},
	} {
		f = ontoRepo(t)
		c.set(f)
		if code, _, stderr, _ := call(t, f, "--onto-main"); code == 0 || !strings.Contains(stderr, c.says) || f.pushed != nil {
			t.Errorf("%s: exit %d\n%s", name, code, stderr)
		}
	}
}
