package push

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// addOn adds a commit on the parent, its tree the parent's with the files set,
// and gives HEAD, on the item's branch, to it.
func (f *fake) addOn(name, parent, subject string, set map[string]string) {
	var tree map[string]string
	for _, c := range f.History {
		if c.Name == parent {
			tree = maps.Clone(c.Tree)
		}
	}
	maps.Copy(tree, set)
	f.History = append(f.History, vcsfake.Commit{Name: name, Parents: []string{parent}, Message: subject + "\n", Tree: tree})
	f.on(item, name)
}

// parentOf is the first parent of the commit.
func (f *fake) parentOf(t *testing.T, commit string) string {
	t.Helper()
	parents, err := f.Parents(t.Context(), commit)
	if err != nil || len(parents) != 1 {
		t.Fatalf("Parents(%s) = %v, %v", commit, parents, err)
	}
	return parents[0]
}

// subjectOf is the subject of the commit.
func (f *fake) subjectOf(t *testing.T, commit string) string {
	t.Helper()
	shown, err := f.Show(t.Context(), commit)
	if err != nil {
		t.Fatal(err)
	}
	return shown.Subject
}

// pushedTo is the commit the push sent to the ref, in the push that sent it.
func pushedTo(f *fake, ref string) string {
	for _, specs := range f.pushed {
		for _, spec := range specs {
			if sha, to, _ := strings.Cut(spec, ":"); to == ref {
				return sha
			}
		}
	}
	return ""
}

// A registry commit between two commits of the work goes to main, onto
// main's head, and the work goes to the item's branch without it, in its
// order, in one atomic push; the clone's branch ends at the head pushed to the
// item's branch. Each commit sent has its line before the "Pushed" one, main's
// first, and GitHub is not asked to rebase the pull request.
func TestPushItemBranchSendsARegistryCommitBetweenWorkToMain(t *testing.T) {
	f := prRepo(t)
	f.addOn("w1", up, "feat: widget", map[string]string{"w1.md": "w1\n"})
	f.addOn("r1", "w1", "docs: add p1-gadget", map[string]string{registryFile: "gadget\n"})
	f.addOn("w2", "r1", "test: cover the widget", map[string]string{"w2.md": "w2\n"})
	code, stdout, stderr, _ := call(t, f)
	toMain, toItem := pushedTo(f, "refs/heads/main"), pushedTo(f, "refs/heads/slice-9")
	if code != 0 || stderr != "" || len(f.pushed) != 1 || fmt.Sprint(f.atomic) != "[true]" || toMain == "" || toItem == "" {
		t.Fatalf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	if f.parentOf(t, toMain) != up || f.subjectOf(t, toMain) != "docs: add p1-gadget" || f.parentOf(t, toItem) != "w1" ||
		f.subjectOf(t, toItem) != "test: cover the widget" || f.head(t) != toItem || f.Named["refs/heads/slice-9"] != toItem {
		t.Errorf("main got %s, the item's branch %s, HEAD is %s", toMain, toItem, f.head(t))
	}
	want := fmt.Sprintf("Sent %s docs: add p1-gadget to origin/main.\nSent w1 feat: widget to origin/slice-9.\nSent %s test: cover the widget to origin/slice-9.\n"+
		"Pushed ", toMain, toItem)
	if !strings.HasPrefix(stdout, want) {
		t.Errorf("stdout\n%s\nwant it to start with\n%s", stdout, want)
	}
	for _, call := range f.calls() {
		if strings.HasPrefix(call, "pr update-branch") {
			t.Errorf("gh was asked to rebase the pull request: %s", call)
		}
	}
	// -q drops the lines as it drops any, --json gives no key for them.
	f = prRepo(t)
	f.addOn("r1", up, "docs: add p1-gadget", map[string]string{registryFile: "gadget\n"})
	f.addOn("w1", "r1", "feat: widget", map[string]string{"w1.md": "w1\n"})
	if code, stdout, _, _ := call(t, f, "-q"); code != 0 || stdout != "" {
		t.Errorf("-q: exit %d, %q", code, stdout)
	}
	f = prRepo(t)
	f.addOn("r1", up, "docs: add p1-gadget", map[string]string{registryFile: "gadget\n"})
	f.addOn("w1", "r1", "feat: widget", map[string]string{"w1.md": "w1\n"})
	if code, stdout, _, _ := call(t, f, "--json"); code != 0 || strings.Contains(stdout, "Sent") || reported(t, stdout).Outcome != "pushed" {
		t.Errorf("--json: exit %d, %q", code, stdout)
	}
}

// A push says what goes to the item's branch only when something goes to main,
// and the registry's commits that start at main's head, which both branches
// keep under one SHA, are said for both.
func TestPushItemBranchSaysWhatItSends(t *testing.T) {
	f := prRepo(t)
	f.on(item, "wk000")
	code, stdout, _, _ := call(t, f)
	taken := pushedTo(f, "refs/heads/main")
	work := pushedTo(f, "refs/heads/slice-9")
	want := fmt.Sprintf("Sent %s docs: take to origin/main.\nSent %s docs: take to origin/slice-9.\nSent %s feat: work to origin/slice-9.\nPushed ",
		taken, taken, work)
	if code != 0 || !strings.HasPrefix(stdout, want) {
		t.Errorf("exit %d\n%s\nwant it to start with\n%s", code, stdout, want)
	}
	f = prRepo(t)
	f.on(item, "nw000")
	if code, stdout, _, _ := call(t, f); code != 0 || strings.Contains(stdout, "Sent") {
		t.Errorf("work alone: exit %d\n%s", code, stdout)
	}
	f = prRepo(t)
	f.on(item, "tk001")
	_, stdout, _, _ = call(t, f)
	head := f.head(t)
	if !strings.HasPrefix(stdout, fmt.Sprintf("Sent %s docs: take to origin/main.\nSent %s docs: take again to origin/main.\nPushed ", f.parentOf(t, head), head)) {
		t.Errorf("registry commits alone:\n%s", stdout)
	}
}

// A take above the spec commit the remote's item branch has goes to main and
// the work above it to the item's branch, onto the spec commit; the take
// alone goes to main with the item's branch left as it is, no pull request
// asked for, and the clone's branch back at the item's branch.
func TestPushItemBranchSendsATakeAboveThePushedSpecToMain(t *testing.T) {
	f := prRepo(t)
	f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
	f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
	f.addOn("w", "tk", "feat: widget", map[string]string{"w.md": "w\n"})
	code, stdout, stderr, _ := call(t, f)
	toMain, toItem := pushedTo(f, "refs/heads/main"), pushedTo(f, "refs/heads/slice-9")
	if code != 0 || stderr != "" || f.parentOf(t, toMain) != up || f.parentOf(t, toItem) != "sp" || f.head(t) != toItem ||
		strings.Contains(stdout, "docs: specify") || !strings.Contains(stdout, "Sent "+toMain+" docs: take to origin/main.\n") {
		t.Errorf("exit %d, main got %s, the item's branch %s, HEAD is %s\n%s%s", code, toMain, toItem, f.head(t), stdout, stderr)
	}

	f = prRepo(t)
	f.repository = ""
	f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
	f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
	code, stdout, stderr, _ = call(t, f)
	toMain = pushedTo(f, "refs/heads/main")
	if code != 0 || stderr != "" || len(f.pushed) != 1 || len(f.pushed[0]) != 1 || len(f.ghCalls) != 0 || f.head(t) != "sp" ||
		f.Remote["origin"].Refs["refs/heads/slice-9"] != "sp" || f.origin() != toMain ||
		!strings.HasPrefix(stdout, "Sent "+toMain+" docs: take to origin/main.\nPushed ") || !strings.Contains(stdout, " to origin/main.\n") {
		t.Errorf("exit %d, pushed %v, HEAD %s\n%s%s", code, f.pushed, f.head(t), stdout, stderr)
	}
	// A take the remote's item branch has already stays there.
	f = prRepo(t)
	f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
	f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "tk"
	f.addOn("w", "tk", "feat: widget", map[string]string{"w.md": "w\n"})
	if code, _, _, _ := call(t, f); code != 0 || len(f.pushed) != 1 || len(f.pushed[0]) != 1 || pushedTo(f, "refs/heads/slice-9") != "w" {
		t.Errorf("a take on the item's branch: exit %d, pushed %v", code, f.pushed)
	}
}

// A registry commit that goes to main carries its itos note, and one whose
// note cannot be set, or that cannot be carried at all, ends the push before
// anything is pushed, exit 3.
func TestPushItemBranchCarriesNotesAndFailures(t *testing.T) {
	build := func() *fake {
		f := prRepo(t)
		f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
		f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
		f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
		for i := range f.History {
			if f.History[i].Name == "tk" {
				f.History[i].Note = "Item: slice-9\n"
			}
		}
		f.addOn("w", "tk", "feat: widget", map[string]string{"w.md": "w\n"})
		return f
	}
	f := build()
	if code, _, _, _ := call(t, f); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if shown, _ := f.Show(t.Context(), pushedTo(f, "refs/heads/main")); shown.Note != "Item: slice-9\n" {
		t.Errorf("the commit sent to main has the note %q", shown.Note)
	}
	for name, c := range map[string]struct {
		set  func(*fake)
		want string
	}{
		"a note that cannot be set": {func(f *fake) { f.noteErr = errors.New("no notes") }, "itos: cannot carry tk onto "},
		"a registry commit that cannot be carried": {func(f *fake) {
			f.pick = func(string, string) (string, bool, error) { return "", true, errors.New("no merge") }
		}, "itos: cannot carry tk onto "},
		"a commit of the work that cannot be carried": {func(f *fake) {
			f.pick = func(commit, _ string) (string, bool, error) { return "", commit == "w", errors.New("no merge") }
		}, "itos: cannot carry w onto "},
	} {
		f := build()
		c.set(f)
		code, _, stderr, _ := call(t, f)
		if code != missing || !strings.Contains(stderr, c.want) || len(f.pushed) != 0 || f.head(t) != "w" || len(f.ghCalls) != 0 {
			t.Errorf("%s: exit %d, pushed %v\n%s", name, code, f.pushed, stderr)
		}
	}
	// The branch could not be set after the push: the commits are pushed.
	f = build()
	f.resetErr = errors.New("locked")
	code, _, stderr, _ := call(t, f)
	if code != missing || !strings.Contains(stderr, "the commits are pushed, but the branch could not be set to ") || !strings.Contains(stderr, "locked") ||
		len(f.pushed) != 1 || len(f.ghCalls) != 0 {
		t.Errorf("a reset that fails: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = prRepo(t)
	f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
	f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
	f.resetErr = errors.New("locked")
	if code, _, stderr, _ := call(t, f); code != missing || !strings.Contains(stderr, "locked") || len(f.pushed) != 1 {
		t.Errorf("a reset that fails after a push to main alone: exit %d\n%s", code, stderr)
	}
}

// Commits that hold a merge and a registry commit are refused, nothing pushed
// or changed, since taking the registry commit out would make the commits above
// it again on their first parents and drop what the merge brings in; with no
// registry commit among them a merge is pushed as it always was.
func TestPushItemBranchRefusesAMergeAboveARegistryCommit(t *testing.T) {
	build := func(registry bool) *fake {
		f := prRepo(t)
		f.addOn("a1", up, "feat: first", map[string]string{"first.md": "first\n"})
		f.addOn("c1", up, "feat: second", map[string]string{"second.md": "second\n"})
		change := map[string]string{"third.md": "third\n"}
		if registry {
			change = map[string]string{registryFile: "taken\n"}
		}
		f.addOn("r1", "c1", "docs: take", change)
		tree := map[string]string{}
		for _, c := range f.History {
			if c.Name == "a1" {
				maps.Copy(tree, c.Tree)
			}
		}
		tree["second.md"], tree["third.md"] = "second\n", "third\n"
		f.History = append(f.History, vcsfake.Commit{Name: "mg", Parents: []string{"a1", "r1"}, Message: "chore: merge\n", Tree: tree})
		f.on(item, "mg")
		return f
	}
	f := build(true)
	code, stdout, stderr, _ := call(t, f, "--json")
	if code != policy || reported(t, stdout).Outcome != "merge-in-history" || len(f.pushed) != 0 || len(f.ghCalls) != 0 || f.head(t) != "mg" ||
		!strings.Contains(stderr, "r1 docs: take is the registry's and goes to main, and mg chore: merge is a merge") ||
		!strings.Contains(stderr, "Make the history linear first (git rebase origin/main), then itos push again.") {
		t.Errorf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	f = build(false)
	if code, _, stderr, _ := call(t, f); code != 0 || pushedTo(f, "refs/heads/slice-9") != "mg" || pushedTo(f, "refs/heads/main") != "" {
		t.Errorf("a merge and no registry commit: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}

// A registry commit that does not apply onto main's head is refused as the
// rebase onto main refuses one: the work registry named, each item both sides
// changed with its owner on main, and how to drop the commit, never git rebase
// --skip; nothing is pushed, no gh is called and the branch is as it was.
func TestPushItemBranchRefusesARegistryCommitThatMeetsMain(t *testing.T) {
	const (
		unowned = "items:\n  - { id: slice-9, owner: null, status: todo }\n"
		ana     = "items:\n  - { id: slice-9, owner: ana, status: doing }\n"
		bo      = "items:\n  - { id: slice-9, owner: bo, status: doing }\n"
	)
	for name, c := range map[string]struct {
		onMain string
		want   []string
	}{
		"a take": {ana, []string{"tasks/work-items.yaml", "slice-9 is taken: a commit on origin/main made ana its owner first."}},
		"a change": {"items:\n  - { id: slice-9, owner: null, status: todo, title: Changed }\n",
			[]string{"slice-9 was changed on origin/main too."}},
		"another item": {unowned + "  - { id: slice-10 }\n", []string{"changed it near where this commit does."}},
	} {
		f := prRepo(t)
		for i := range f.History {
			if f.History[i].Name == up {
				f.History[i].Tree[registryFile] = c.onMain
			}
		}
		f.addOn("sp", base, "docs: specify", map[string]string{"spec.md": "spec\n", registryFile: unowned})
		f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
		f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: bo})
		code, _, stderr, _ := call(t, f, "--json")
		if code != policy || len(f.pushed) != 0 || len(f.ghCalls) != 0 || f.head(t) != "tk" || f.Rebasing(t.Context()) ||
			strings.Contains(stderr, "rebase --skip") || !strings.Contains(stderr, "git rebase --onto tk^ tk") ||
			!strings.Contains(stderr, "tk docs: take is the registry's and goes to origin/main, where it does not apply") {
			t.Errorf("%s: exit %d, pushed %v\n%s", name, code, f.pushed, stderr)
		}
		for _, want := range c.want {
			if !strings.Contains(stderr, want) {
				t.Errorf("%s: stderr\n%s\nwant %q", name, stderr, want)
			}
		}
	}
	// A conflict in a path that is no registry names the paths.
	f := prRepo(t)
	f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
	f.addOn("tk", "sp", "docs: take", map[string]string{registryFile: "taken\n"})
	f.pick = func(string, string) (string, bool, error) {
		return "", true, &vcsport.Conflicts{Paths: []string{"tasks/phase-1.yaml", "tasks/phase-2.yaml"}}
	}
	code, stdout, stderr, _ := call(t, f)
	if code != policy || stdout != "" || !strings.Contains(stderr, "It conflicts in tasks/phase-1.yaml, tasks/phase-2.yaml with a commit on origin/main.") {
		t.Errorf("a conflict outside the registry: exit %d\n%s%s", code, stdout, stderr)
	}
}

// A commit that is not the registry's and does not apply once the registry
// commits are taken out from under it is refused, outcome needs-registry-
// commit, nothing pushed: it names that commit and the registry commit it
// needs, the latest that changes a path it conflicts in, else the latest, and
// says to make the registry commit on main or fold the two; under --json the
// lines go to stderr.
func TestPushItemBranchRefusesACommitThatNeedsARegistryCommit(t *testing.T) {
	build := func() *fake {
		f := prRepo(t)
		for i := range f.History {
			if f.History[i].Name == up {
				f.History[i].Tree[registryFile] = "s0\n"
			}
		}
		f.addOn("sp", up, "docs: specify", map[string]string{"spec.md": "spec\n"})
		f.Remote["origin"].Refs["refs/heads/slice-9"] = "sp"
		f.addOn("t1", "sp", "docs: first take", map[string]string{registryFile: "t1\n"})
		f.addOn("t2", "t1", "docs: second take", map[string]string{registryFile: "t2\n"})
		f.addOn("hand", "t2", "docs: fix the check", map[string]string{registryFile: "t3\n", "check.md": "c\n"})
		return f
	}
	f := build()
	code, stdout, stderr, _ := call(t, f, "--json")
	if code != policy || reported(t, stdout).Outcome != "needs-registry-commit" || len(f.pushed) != 0 || len(f.ghCalls) != 0 || f.head(t) != "hand" ||
		!strings.Contains(stderr, "hand docs: fix the check does not apply without t2 docs: second take, which is the registry's and goes to origin/main, not the item's branch; nothing was pushed.") ||
		!strings.Contains(stderr, "git switch main, make it again there, itos push), or fold t2 and hand into one commit.") {
		t.Errorf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	f = build()
	code, stdout, stderr, _ = call(t, f)
	if code != policy || stderr == "" || stdout != "" || !strings.Contains(stderr, "itos push: hand docs: fix the check does not apply without") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	// With no path the commit conflicts in changed by a registry commit, the
	// latest is the one it needs.
	f = build()
	f.pick = func(commit, _ string) (string, bool, error) {
		if commit == "hand" {
			return "", true, &vcsport.Conflicts{Paths: []string{"check.md"}}
		}
		return "", false, nil
	}
	if code, _, stderr, _ := call(t, f); code != policy || !strings.Contains(stderr, "does not apply without t2 docs: second take") {
		t.Errorf("a conflict in a path no registry commit changes: exit %d\n%s", code, stderr)
	}
}
