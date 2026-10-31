package push

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// updateRepo is prRepo under push.pr.update merge (slice-213).
func updateRepo(t *testing.T) *fake {
	t.Helper()
	f := prRepo(t)
	f.cfg.Push.PR.Update = "merge"
	return f
}

// parentsOf are the commit's parents.
func (f *fake) parentsOf(t *testing.T, commit string) []string {
	t.Helper()
	parents, err := f.Parents(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	return parents
}

// mergeOf adds a commit with the parents, its tree theirs together, and puts
// the item's branch on it.
func (f *fake) mergeOf(name, subject string, parents ...string) {
	tree := map[string]string{}
	for _, c := range f.History {
		if slices.Contains(parents, c.Name) {
			maps.Copy(tree, c.Tree)
		}
	}
	f.History = append(f.History, vcsfake.Commit{Name: name, Parents: parents, Message: subject + "\n", Tree: tree})
	f.on(item, name)
}

// A first push whose branch main has moved past merges main's head into it,
// rebasing nothing, and pushes the merge alone to the item's branch; the clone
// ends at it, the pull request is opened and GitHub never asked to update it.
func TestMergeUpdateMergesMainIntoAFirstPush(t *testing.T) {
	f := updateRepo(t)
	code, stdout, stderr, _ := call(t, f)
	head := pushedTo(f, "refs/heads/slice-9")
	if code != 0 || len(f.pushed) != 1 || fmt.Sprint(f.atomic) != "[true]" || head == "" || f.head(t) != head {
		t.Fatalf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	if parents := f.parentsOf(t, head); !slices.Equal(parents, []string{mine, up}) || f.subjectOf(t, head) != "Merge main into slice-9" {
		t.Errorf("the head pushed has the parents %q and the subject %q", parents, f.subjectOf(t, head))
	}
	for _, call := range f.calls() {
		if strings.HasPrefix(call, "pr update-branch") {
			t.Errorf("gh was asked to update the pull request: %s", call)
		}
	}
}

// A take on the branch goes to main first, in a push of its own, carried onto
// main's head; the clone's branch is reset to the work without it, starting at
// the commit below the take rather than at main's head, and main's head, the
// take on it, is merged in; each commit sent is said.
func TestMergeUpdateSendsATakeToMainFirst(t *testing.T) {
	f := updateRepo(t)
	f.on(item, "wk000")
	code, stdout, stderr, _ := call(t, f)
	if code != 0 || len(f.pushed) != 2 || fmt.Sprint(f.atomic) != "[false true]" {
		t.Fatalf("exit %d, pushed %v %v\n%s%s", code, f.pushed, f.atomic, stdout, stderr)
	}
	taken, head := f.pushed[0][0], pushedTo(f, "refs/heads/slice-9")
	taken, to, _ := strings.Cut(taken, ":")
	merged := f.parentsOf(t, head)
	if to != "refs/heads/main" || f.parentOf(t, taken) != up || f.subjectOf(t, taken) != "docs: take" || len(merged) != 2 || merged[1] != taken ||
		f.parentOf(t, merged[0]) != base || f.subjectOf(t, merged[0]) != "feat: work" || f.head(t) != head {
		t.Errorf("main got %s %s, the item's branch %s with the parents %q", to, taken, head, merged)
	}
	want := fmt.Sprintf("Sent %s docs: take to origin/main.\nSent %s feat: work to origin/slice-9.\nPushed ", taken, merged[0])
	if !strings.HasPrefix(stdout, want) {
		t.Errorf("stdout\n%s\nwant it to start with\n%s", stdout, want)
	}
}

// A commit the remote's item branch has that the clone lacks is merged in, the
// clone's own commit kept under its SHA; a branch with nothing of its own past
// the remote's is fast-forwarded to it, nothing pushed, and one that cannot be
// is reported, exit 3.
func TestMergeUpdateMergesTheRemoteItemBranchIn(t *testing.T) {
	f := updateRepo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = base
	f.Remote["origin"].Refs["refs/heads/slice-9"] = up
	code, stdout, stderr, _ := call(t, f)
	head := pushedTo(f, "refs/heads/slice-9")
	if code != 0 || len(f.pushed) != 1 || !slices.Equal(f.parentsOf(t, head), []string{mine, up}) ||
		f.subjectOf(t, head) != "Merge origin/slice-9 into slice-9" {
		t.Errorf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	f = updateRepo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = base
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "nw000"
	code, stdout, stderr, _ = call(t, f, "--json")
	if code != 0 || len(f.pushed) != 0 || f.head(t) != "nw000" || reported(t, stdout).Outcome != "nothing-to-push" {
		t.Errorf("a fast-forward: exit %d, pushed %v, HEAD %s\n%s%s", code, f.pushed, f.head(t), stdout, stderr)
	}
	f = updateRepo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = base
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "nw000"
	f.resetErr = errors.New("locked")
	if code, _, stderr, _ := call(t, f); code != missing || stderr != "itos: cannot fast-forward slice-9 to origin/slice-9: locked\n" {
		t.Errorf("a fast-forward that fails: exit %d\n%s", code, stderr)
	}
}

// A branch main has, with no item's branch or one main has too, is nothing to
// push.
func TestMergeUpdateWithNothingToPush(t *testing.T) {
	for name, tip := range map[string]string{"no item's branch": "", "an item's branch main has": base} {
		f := updateRepo(t)
		f.Remote["origin"].Refs["refs/heads/main"] = mine
		if tip != "" {
			f.Remote["origin"].Refs["refs/heads/slice-9"] = tip
		}
		code, stdout, stderr, _ := call(t, f)
		if code != 0 || len(f.pushed) != 0 || stdout != "Nothing to push: slice-9 adds nothing to origin/main.\n" {
			t.Errorf("%s: exit %d\n%s%s", name, code, stdout, stderr)
		}
	}
}

// A merge that conflicts is left in progress, nothing of the item's pushed and
// no gh called, saying the paths and what to run; registry commits sent to
// main before stay there, and the report says so.
func TestMergeUpdateStopsOnAConflict(t *testing.T) {
	f := updateRepo(t)
	f.History[1].Tree["mine.md"] = "theirs\n"
	code, stdout, stderr, _ := call(t, f, "--json")
	if code != policy || reported(t, stdout).Outcome != "merge-conflict" || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
		!slices.Equal(f.MergeHeads(t.Context()), []string{up}) || f.head(t) != mine ||
		!strings.HasPrefix(stderr, "itos push: the merge of main into slice-9 stopped on a conflict in mine.md; origin/slice-9 was not pushed.\n") ||
		!strings.Contains(stderr, "(itos commit or git commit) with a typed first line") || !strings.Contains(stderr, "Then itos push again") {
		t.Errorf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	f = updateRepo(t)
	f.History[1].Tree["work.md"] = "theirs\n"
	f.on(item, "wk000")
	code, _, stderr, _ = call(t, f)
	taken := pushedTo(f, "refs/heads/main")
	if code != policy || len(f.pushed) != 1 || f.parentOf(t, f.head(t)) != base ||
		!strings.HasPrefix(stderr, fmt.Sprintf("Sent %s docs: take to origin/main.\nitos push: the merge of main into slice-9 stopped on a conflict in work.md;", taken)) {
		t.Errorf("after a take: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}

// A merge git refuses to run is merge-failed, exit 1; one git cannot run, or
// that leaves no HEAD, exits 3; nothing is pushed.
func TestMergeUpdateWhereTheMergeFails(t *testing.T) {
	for _, c := range []struct {
		name   string
		merge  func(f *fake) func() error
		code   int
		stderr string
	}{
		{"refused", func(*fake) func() error {
			return func() error { return &vcsport.Failed{Command: "git merge", Code: 128, Words: "fatal: no\n"} }
		}, policy, "itos push: the merge of main into slice-9 did not run (git's message above); origin/slice-9 was not pushed.\n"},
		{"no git", func(*fake) func() error { return func() error { return errors.New("no git") } }, missing, "itos: cannot run git: no git\n"},
		{"no HEAD", func(f *fake) func() error {
			return func() error { f.Named["HEAD"] = "gone"; return nil }
		}, missing, "itos: cannot resolve HEAD after the merge: HEAD names no commit\n"},
	} {
		f := updateRepo(t)
		f.merge = c.merge(f)
		if code, _, stderr, _ := call(t, f); code != c.code || stderr != c.stderr || len(f.pushed) != 0 {
			t.Errorf("%s: exit %d, pushed %v\n%s", c.name, code, f.pushed, stderr)
		}
	}
}

// Commits that cannot be listed, no GitHub repository, a push of the take to
// main that fails and a branch that cannot be reset after it end the push.
func TestMergeUpdateStops(t *testing.T) {
	f := updateRepo(t)
	f.commitsErr = errors.New("unreadable")
	if code, _, stderr, _ := call(t, f); code != missing || stderr != "itos: cannot list the commits slice-9 adds to origin/main: unreadable\n" {
		t.Errorf("unlisted: exit %d\n%s", code, stderr)
	}
	f = updateRepo(t)
	f.on(item, "wk000")
	f.repository = ""
	if code, _, stderr, _ := call(t, f); code != missing || !strings.Contains(stderr, "names no GitHub repository") || len(f.pushed) != 0 {
		t.Errorf("no repository: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = updateRepo(t)
	f.on(item, "wk000")
	f.pushErr = &vcsport.Failed{Code: 128, Failure: vcsport.Unreachable}
	if code, _, stderr, _ := call(t, f); code != temporary || !strings.Contains(stderr, "the push to origin/main failed") || len(f.pushed) != 1 {
		t.Errorf("main's push: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = updateRepo(t)
	f.on(item, "wk000")
	f.resetErr = errors.New("locked")
	if code, _, stderr, _ := call(t, f); code != missing || !strings.Contains(stderr, "could not be set to") || len(f.pushed) != 1 {
		t.Errorf("the reset: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = updateRepo(t)
	f.pushErr = &vcsport.Failed{Code: 1, Failure: vcsport.Refused}
	if code, _, _, _ := call(t, f); code != policy || len(f.ghCalls) != 0 {
		t.Errorf("the item's push: exit %d", code)
	}
}

// A branch of registry commits alone goes to main: under its own SHAs when
// they start at main's head, else carried onto it, the clone reset to the
// commit below them; no pull request either way.
func TestMergeUpdateOfTakesAlone(t *testing.T) {
	f := updateRepo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = base
	f.on(item, "tk000")
	if code, _, stderr, _ := call(t, f); code != 0 || fmt.Sprint(f.pushed) != "[[tk000:refs/heads/main]]" || len(f.ghCalls) != 0 {
		t.Errorf("on main's head: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = updateRepo(t)
	f.on(item, "tk000")
	code, _, stderr, _ := call(t, f)
	taken := pushedTo(f, "refs/heads/main")
	if code != 0 || len(f.pushed) != 1 || f.parentOf(t, taken) != up || f.head(t) != base || len(f.ghCalls) != 0 {
		t.Errorf("carried: exit %d, pushed %v, HEAD %s\n%s", code, f.pushed, f.head(t), stderr)
	}
}

// Registry commits that start at main's head stay on the item's branch and go
// to main with it, under their own SHAs, in one atomic push, as slice-192 sends
// them, nothing merged.
func TestMergeUpdateSendsATakeOnMainsHeadWithTheWork(t *testing.T) {
	f := updateRepo(t)
	f.Remote["origin"].Refs["refs/heads/main"] = base
	f.on(item, "wk000")
	code, stdout, stderr, _ := call(t, f)
	if code != 0 || fmt.Sprint(f.pushed) != "[[tk000:refs/heads/main wk000:refs/heads/slice-9]]" || fmt.Sprint(f.atomic) != "[true]" || f.head(t) != "wk000" {
		t.Errorf("exit %d, pushed %v %v\n%s%s", code, f.pushed, f.atomic, stdout, stderr)
	}
}

// Under merge only a registry commit a merge among the commits descends from
// is refused (merge-in-history); one above a merge goes to main, and the
// merge stays on the item's branch under its own SHA, main merged into it.
func TestMergeUpdateRefusesARegistryCommitBelowAMerge(t *testing.T) {
	f := updateRepo(t)
	f.mergeOf("mg000", "chore: merge", "tk000", mine)
	code, _, stderr, _ := call(t, f)
	if code != policy || !strings.Contains(stderr, "tk000 docs: take is the registry's and goes to main, and mg000 chore: merge is a merge") || len(f.pushed) != 0 {
		t.Errorf("below: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = updateRepo(t)
	f.mergeOf("mg000", "chore: merge", mine, "nw000")
	f.addOn("id000", "mg000", "docs: add p1-gadget", map[string]string{registryFile: "gadget\n"})
	code, stdout, stderr, _ := call(t, f)
	idea, head := pushedTo(f, "refs/heads/main"), pushedTo(f, "refs/heads/slice-9")
	if code != 0 || len(f.pushed) != 2 || f.parentOf(t, idea) != up || !slices.Equal(f.parentsOf(t, head), []string{"mg000", idea}) {
		t.Errorf("above: exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
}

// A merge of main that brought a take alone, made and not pushed (a push that
// failed after itos made it), is no registry commit under merge: it goes to the
// item's branch as it is, and nothing to main. Under rebase such a merge is
// the registry's, refused as merge-in-history, as before.
func TestMergeUpdateKeepsAMergeOfATakeOnTheItemsBranch(t *testing.T) {
	for _, update := range []string{"merge", "rebase"} {
		f := updateRepo(t)
		f.cfg.Push.PR.Update = update
		f.Remote["origin"].Refs["refs/heads/main"] = "tk000"
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.mergeOf("mg000", "Merge main into slice-9", mine, "tk000")
		code, stdout, stderr, _ := call(t, f)
		switch {
		case update == "merge" && (code != 0 || fmt.Sprint(f.pushed) != "[[mg000:refs/heads/slice-9]]" || f.head(t) != "mg000"),
			update == "rebase" && (code != policy || !strings.Contains(stderr, "mg000 Merge main into slice-9 is a merge")):
			t.Errorf("%s: exit %d, pushed %v\n%s%s", update, code, f.pushed, stdout, stderr)
		}
	}
}

// --onto-main, the rebase merge replaces, is refused before anything is
// fetched or pushed.
func TestMergeUpdateRefusesOntoMain(t *testing.T) {
	f := updateRepo(t)
	code, stdout, stderr, _ := call(t, f, "--json", "--onto-main")
	if code != policy || reported(t, stdout).Outcome != "onto-main-under-merge" || len(f.pushed) != 0 || len(f.quiet)+len(f.ghCalls) != 0 ||
		!strings.Contains(stderr, "push.pr.update is merge, so itos push merges main in") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
}
