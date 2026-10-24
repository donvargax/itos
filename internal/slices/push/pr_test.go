package push

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The registry, which lists the item the branch is named for; the item; the
// pull request gh opens, and the one it finds open.
const (
	registryFile = "tasks/work-items.yaml"
	item         = "slice-9"
	opened       = "https://github.example/o/r/pull/1"
	open7        = "https://github.example/o/r/pull/7"
)

// The commits beside repo's: tk000, a take (it changes the registry alone)
// and wk000, work, one above the other on base; tk001 a second take above
// tk000; nw000 more work above mine.
func (f *fake) addCommits() {
	start := map[string]string{"README.md": "start\n"}
	take := maps.Clone(start)
	take[registryFile] = "taken\n"
	again := maps.Clone(take)
	again[registryFile] = "taken twice\n"
	work := maps.Clone(take)
	work["work.md"] = "work\n"
	more := maps.Clone(f.History[2].Tree)
	more["more.md"] = "more\n"
	f.History = append(f.History,
		vcsfake.Commit{Name: "tk000", Parents: []string{base}, Message: "docs: take\n", Tree: take},
		vcsfake.Commit{Name: "tk001", Parents: []string{"tk000"}, Message: "docs: take again\n", Tree: again},
		vcsfake.Commit{Name: "wk000", Parents: []string{"tk000"}, Message: "feat: work\n", Tree: work},
		vcsfake.Commit{Name: "nw000", Parents: []string{mine}, Message: "test: more\n", Tree: more})
}

// on puts HEAD, and the branch it is on, at the commit.
func (f *fake) on(branch, commit string) {
	f.OnBranch = branch
	f.Named["HEAD"], f.Named["refs/heads/"+branch] = commit, commit
}

// answers is a gh that lists the open pull request when one is given, [] when
// not, and opens pull requests at the address.
func answers(created, open string) func([]string) (string, error) {
	return func(args []string) (string, error) {
		switch strings.Join(args[:2], " ") {
		case "pr list":
			if open == "" {
				return "[]\n", nil
			}
			return fmt.Sprintf(`[{"number":7,"url":%q}]`, open) + "\n", nil
		case "pr create":
			return "Creating the pull request\n" + created + "\n", nil
		}
		return "", nil
	}
}

// prRepo is repo under push.mode pr: the clone on the item's branch at mine,
// which the registry lists, origin's main at up, and a remote whose HEAD
// names main; gh opens pull requests and the repository is owner/name.
func prRepo(t *testing.T, config ...string) *fake {
	t.Helper()
	f := repo(t)
	text := "version: 1\npush: { mode: pr }\n" + strings.Join(config, "")
	f.cfg = loadedWith(t, text, map[string]string{registryFile: "items:\n  - { id: " + item + " }\n"})
	f.remoteHead, f.repository, f.gh = "main", "owner/name", answers(opened, "")
	f.addCommits()
	f.on(item, mine)
	return f
}

// calls are the gh calls as one line each.
func (f *fake) calls() []string {
	var lines []string
	for _, c := range f.ghCalls {
		lines = append(lines, strings.Join(c, " "))
	}
	return lines
}

// The first push of an item's branch rebases it onto origin's main, pushes
// it to the branch of its own name in one atomic push, finds no pull request,
// opens one and turns on auto-merge by rebase, every gh call naming the
// repository; the address is said, and --json gives it.
func TestPushItemBranchOpensItsPullRequest(t *testing.T) {
	f := prRepo(t)
	code, stdout, stderr, err := call(t, f)
	head := f.head(t)
	if parents, _ := f.Parents(t.Context(), head); code != 0 || err != nil || stderr != "" || len(parents) != 1 || parents[0] != up ||
		stdout != "Pushed "+head[:len(head)-1]+" to origin/slice-9. Pull request: "+opened+"\n" {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if f.Remote["origin"].Refs["refs/heads/slice-9"] != head || f.origin() != up || fmt.Sprint(f.pushed) != fmt.Sprint([][]string{{head + ":refs/heads/slice-9"}}) ||
		fmt.Sprint(f.atomic) != "[true]" {
		t.Errorf("origin has %v; pushed %v atomic %v", f.Remote["origin"].Refs, f.pushed, f.atomic)
	}
	want := []string{
		"pr list --repo owner/name --head slice-9 --state open --json number,url",
		"pr create --repo owner/name --base main --head slice-9 --fill",
		"pr merge slice-9 --repo owner/name --auto --rebase",
	}
	if got := f.calls(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("gh was called\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	f = prRepo(t)
	code, stdout, stderr, _ = call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || !r.OK || r.Outcome != "pushed" || r.Branch != "slice-9" || r.PR == nil || r.PR.URL != opened || r.PR.Number != 1 ||
		!r.PR.Created || r.PR.Updated || r.CI != "" || r.Run != nil || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	if code, stdout, _, _ := call(t, prRepo(t), "-q"); code != 0 || stdout != "" {
		t.Errorf("-q: exit %d, %q", code, stdout)
	}
	f = prRepo(t)
	if code, stdout, _, _ := call(t, f, "--no-wait"); code != 0 || !strings.HasSuffix(stdout, "Pull request: "+opened+"\n") || len(f.ghCalls) != 3 {
		t.Errorf("--no-wait: exit %d, %q", code, stdout)
	}
}

// The number of a pull request is the end of its address, 0 when it has
// none, and the address is the last line gh printed.
func TestPushPullRequestNumber(t *testing.T) {
	for address, number := range map[string]int{opened: 1, "https://github.example/o/r/pull/12/": 12, "https://github.example/o/r": 0} {
		f := prRepo(t)
		f.gh = answers(address, "")
		_, stdout, _, _ := call(t, f, "--json")
		if r := reported(t, stdout); r.PR == nil || r.PR.URL != address || r.PR.Number != number {
			t.Errorf("%s: %s", address, stdout)
		}
	}
}

// A pull request already open is named and no other is opened; the push
// merges it by rebase all the same.
func TestPushItemBranchWithAnOpenPullRequest(t *testing.T) {
	f := prRepo(t)
	f.gh = answers(opened, open7)
	code, stdout, _, _ := call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || r.PR == nil || r.PR.URL != open7 || r.PR.Number != 7 || r.PR.Created || r.PR.Updated {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	if got := f.calls(); len(got) != 2 || !strings.HasPrefix(got[1], "pr merge slice-9 ") {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
}

// The registry's commits directly above main go to main, the rest to the
// item's branch, in one atomic push; a commit that is not directly above
// main, touches more than the registry or is on the item's branch already
// ends the commits that go to main.
func TestPushItemBranchSendsTakesToMain(t *testing.T) {
	f := prRepo(t)
	f.on(item, "wk000")
	f.History[len(f.History)-2].Parents = []string{"tk001"}
	f.History[len(f.History)-2].Tree["work.md"] = "work\n"
	f.History[len(f.History)-2].Tree[registryFile] = "taken twice\n"
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	took, _ := f.Parents(t.Context(), head)
	if code != 0 || stderr != "" || len(f.pushed) != 1 || len(f.pushed[0]) != 2 || f.pushed[0][1] != head+":refs/heads/slice-9" ||
		f.pushed[0][0] != took[0]+":refs/heads/main" || fmt.Sprint(f.atomic) != "[true]" {
		t.Fatalf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	if f.origin() != took[0] || f.Remote["origin"].Refs["refs/heads/slice-9"] != head {
		t.Errorf("origin has %v", f.Remote["origin"].Refs)
	}
	// Work first: none of it goes to main, and a take above the work stays
	// with the work.
	f = prRepo(t)
	f.History = append(f.History, vcsfake.Commit{Name: "tk002", Parents: []string{mine}, Message: "docs: take\n",
		Tree: map[string]string{"README.md": "start\n", "mine.md": "mine.md\n", registryFile: "taken\n"}})
	f.on(item, "tk002")
	if code, _, _, _ := call(t, f); code != 0 || len(f.pushed) != 1 || len(f.pushed[0]) != 1 {
		t.Errorf("work then a take: exit %d, pushed %v", code, f.pushed)
	}
	// A take already on the remote's item branch is not sent to main again.
	f = prRepo(t)
	f.on(item, "wk000")
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "tk000"
	f.History = append(f.History, vcsfake.Commit{Name: "wk001", Parents: []string{"wk000"}, Message: "feat: more\n",
		Tree: map[string]string{"README.md": "start\n", registryFile: "taken\n", "work.md": "work\n", "w.md": "w\n"}})
	f.on(item, "wk001")
	if code, _, stderr, _ := call(t, f); code != 0 || len(f.pushed) != 1 || len(f.pushed[0]) != 1 {
		t.Errorf("a take on the item's branch: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}

// A branch whose commits all touch the registry alone goes to main alone:
// the item's branch is not pushed, no gh is called and no repository is
// needed; the commits are not waited for, naming itos ci watch.
func TestPushItemBranchOfTakesAlone(t *testing.T) {
	f := prRepo(t)
	f.repository = ""
	f.on(item, "tk001")
	code, stdout, stderr, _ := call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || !r.OK || r.Outcome != "pushed" || r.Branch != "main" || r.PR != nil || len(f.ghCalls) != 0 || stderr != "" ||
		fmt.Sprint(f.pushed) != fmt.Sprint([][]string{{f.head(t) + ":refs/heads/main"}}) || fmt.Sprint(f.atomic) != "[true]" {
		t.Fatalf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
	}
	if f.Remote["origin"].Refs["refs/heads/slice-9"] != "" || f.origin() != f.head(t) {
		t.Errorf("origin has %v", f.Remote["origin"].Refs)
	}
	f = prRepo(t, "ci: { watch: { provider: github } }\n")
	f.on(item, "tk001")
	f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		t.Error("the commits of the registry alone were waited for")
		return providers.Run{}, false, nil
	}}, true
	code, stdout, _, _ = call(t, f)
	head := f.head(t)
	if code != 0 || !strings.Contains(stdout, "Its commits touch only tasks/work-items.yaml, so its CI run is not waited for; itos ci watch "+head+" waits for it.") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	f = prRepo(t)
	f.on(item, "tk001")
	if code, stdout, _, _ := call(t, f, "--no-wait"); code != 0 || strings.Contains(stdout, "Its commits") || len(f.pushed) != 1 {
		t.Errorf("--no-wait: exit %d\n%s", code, stdout)
	}
	f = prRepo(t)
	f.on(item, "tk001")
	f.pushErr = &vcsport.Failed{Code: 1, Failure: vcsport.Refused}
	if code, _, stderr, _ := call(t, f); code != policy || !strings.Contains(stderr, "the push to origin/main failed") {
		t.Errorf("a refused push: exit %d\n%s", code, stderr)
	}
}

// A branch the remote has with nothing new to push still gets its pull
// request; one main has moved past is rebased by GitHub, never forced, and a
// push that asked for that looks at no run when ci.watch is off or --no-wait
// is given: gh pr view is never called then (the wait for the rebased head is
// TestPushItemBranchWatchesTheRebasedHead's).
func TestPushItemBranchBehindMain(t *testing.T) {
	behind := func(config ...string) *fake {
		f := prRepo(t, config...)
		f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
			t.Error("a run was looked at")
			return providers.Run{}, false, nil
		}}, len(config) > 0
		f.gh = answers(opened, open7)
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.on(item, "nw000")
		return f
	}
	watching := "ci: { watch: { provider: github } }\n"
	f := behind()
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	if code != 0 || stderr != "" || stdout != "Pushed "+head[:len(head)-1]+" to origin/slice-9. Pull request: "+open7+"\n" {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	want := "pr update-branch slice-9 --repo owner/name --rebase"
	if got := f.calls(); len(got) != 3 || got[2] != want || fmt.Sprint(f.pushed) != fmt.Sprint([][]string{{"nw000:refs/heads/slice-9"}}) {
		t.Errorf("gh was called\n%s\npushed %v", strings.Join(got, "\n"), f.pushed)
	}
	f = behind()
	code, stdout, _, _ = call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || r.Outcome != "pushed" || r.PR == nil || !r.PR.Updated || r.PR.Created || r.CI != "" || r.Run != nil {
		t.Errorf("--json: exit %d\n%s", code, stdout)
	}
	f = behind(watching)
	if code, stdout, _, _ := call(t, f, "--no-wait"); code != 0 || strings.Contains(stdout, "GitHub rebased") || len(f.ghCalls) != 3 {
		t.Errorf("--no-wait: exit %d\n%s\n%v", code, stdout, f.calls())
	}
	// Nothing new to push: the pull request is still found, merged and
	// rebased by GitHub, and no run is waited for.
	f = behind(watching)
	f.on(item, mine)
	code, stdout, _, _ = call(t, f, "--json")
	r = reported(t, stdout)
	if code != 0 || r.Outcome != "nothing-to-push" || r.PR == nil || !r.PR.Updated || len(f.pushed) != 0 || len(f.ghCalls) != 3 {
		t.Errorf("nothing to push: exit %d\n%s", code, stdout)
	}
	f = behind(watching)
	f.on(item, mine)
	f.gh = answers(opened, "")
	if code, stdout, _, _ := call(t, f); code != 0 || stdout != "Nothing to push: slice-9 is at origin/slice-9. Pull request: "+opened+"\n" {
		t.Errorf("nothing to push, a pull request opened: exit %d\n%s", code, stdout)
	}
}

// clock is the time the wait for the rebased head sees: it moves only when
// the wait sleeps, or when a test advances it, and each sleep is kept. A
// sleep of no time would never move it, so it fails the test.
type clock struct {
	at    time.Time
	slept []time.Duration
}

// clocked puts a clock in the place of the wait's own for the test.
func clocked(t *testing.T) *clock {
	t.Helper()
	c := &clock{at: time.Unix(1_000_000, 0)}
	wasNow, wasSleep := now, sleep
	now = func() time.Time { return c.at }
	sleep = func(d time.Duration) {
		if d <= 0 {
			t.Fatalf("a sleep of %s", d)
		}
		c.slept = append(c.slept, d)
		c.at = c.at.Add(d)
	}
	t.Cleanup(func() { now, sleep = wasNow, wasSleep })
	return c
}

// rebasing is a push of the item's branch that main has moved past (origin's
// up), whose pull request GitHub is asked to rebase, with ci.watch on github
// and the settings, an interval of 2 seconds and a timeout of 30 unless it
// gives them. gh pr view answers the heads in turn, the last for good; the
// run of every head looked at is a success, and the heads looked at are kept.
// rb000, a commit above up, is the head GitHub makes; nw000, the head pushed,
// and mine, the previous push's, lack up. The clone has rb000 unless gain is
// set, which then makes it arrive with the first quiet fetch.
func rebasing(t *testing.T, settings string, gain bool, heads ...string) (f *fake, looked *[]string) {
	t.Helper()
	if settings == "" {
		settings = "interval: 2, timeout: 30"
	}
	f = prRepo(t, "ci: { watch: { provider: github, "+settings+" } }\n")
	gained := vcsfake.Commit{Name: "rb000", Parents: []string{up}, Message: "test: more\n", Tree: maps.Clone(f.History[1].Tree)}
	if gain {
		f.onQuiet = func() { f.History = append(f.History, gained) }
	} else {
		f.History = append(f.History, gained)
	}
	looked = new([]string)
	f.watcher, f.watchOK = providers.Watcher{Look: func(sha string) (providers.Run, bool, error) {
		*looked = append(*looked, sha)
		return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: "success"}, true, nil
	}}, true
	views := 0
	f.gh = func(args []string) (string, error) {
		if strings.Join(args[:2], " ") != "pr view" {
			return answers(opened, open7)(args)
		}
		head := heads[min(views, len(heads)-1)]
		views++
		return fmt.Sprintf(`{"headRefOid":%q}`+"\n", head), nil
	}
	f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
	f.on(item, "nw000")
	return f, looked
}

// views are the gh pr view calls.
func (f *fake) views() []string {
	var views []string
	for _, c := range f.calls() {
		if strings.HasPrefix(c, "pr view ") {
			views = append(views, c)
		}
	}
	return views
}

// After gh pr update-branch the wait asks gh pr view for the item's head at
// once and each ci.watch.interval, passes over every head without main's head
// (the head pushed, the previous push's, none, one the clone cannot have) and
// watches the run of the first that has it, saying so where the watch's
// progress goes.
func TestPushItemBranchWatchesTheRebasedHead(t *testing.T) {
	c := clocked(t)
	f, looked := rebasing(t, "", false, mine, "nw000", "", "zz999", "rb000")
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	if code != 0 || stderr != "" || fmt.Sprint(*looked) != "[rb000]" || fmt.Sprint(c.slept) != "[2s 2s 2s 2s]" ||
		stdout != "Pushed "+head[:len(head)-1]+" to origin/slice-9. Pull request: "+open7+"\n"+
			"GitHub rebased the pull request onto origin/main: watching the run of rb000, its new head.\n"+
			"CI run: https://ci/1\nCI passed: https://ci/1\n" {
		t.Fatalf("exit %d, looked %v, slept %v\n%s%s", code, *looked, c.slept, stdout, stderr)
	}
	view := "pr view slice-9 --repo owner/name --json headRefOid"
	if views := f.views(); len(views) != 5 || views[0] != view || views[4] != view {
		t.Errorf("gh was called\n%s", strings.Join(f.calls(), "\n"))
	}
	// One quiet fetch of the item's branch for the head the clone lacked, and
	// none for an empty answer.
	if fmt.Sprint(f.quiet) != "[refs/heads/slice-9]" {
		t.Errorf("quiet fetches %v", f.quiet)
	}

	// Whatever update-branch printed, a head that has main's head already is
	// taken at the first answer.
	c = clocked(t)
	f, looked = rebasing(t, "", false, "rb000")
	code, stdout, _, _ = call(t, f)
	if code != 0 || fmt.Sprint(*looked) != "[rb000]" || len(c.slept) != 0 || len(f.views()) != 1 || !strings.Contains(stdout, "watching the run of rb000, its new head.\n") {
		t.Errorf("a head that has main's: exit %d, looked %v, slept %v\n%s", code, *looked, c.slept, stdout)
	}

	// A head the clone lacks arrives with a quiet fetch of the item's branch.
	c = clocked(t)
	f, looked = rebasing(t, "", true, "rb000")
	code, _, _, _ = call(t, f)
	if code != 0 || fmt.Sprint(*looked) != "[rb000]" || len(c.slept) != 0 || fmt.Sprint(f.quiet) != "[refs/heads/slice-9]" {
		t.Errorf("a head fetched: exit %d, looked %v, quiet %v", code, *looked, f.quiet)
	}

	// The progress goes to stderr under --json, which carries the run, and
	// nowhere under -q.
	clocked(t)
	f, _ = rebasing(t, "", false, "rb000")
	code, stdout, stderr, _ = call(t, f, "--json")
	r := reported(t, stdout)
	if code != 0 || r.Outcome != "pushed" || r.CI != "success" || r.Run == nil || r.Run.URL != "https://ci/1" || r.PR == nil || !r.PR.Updated ||
		stderr != "GitHub rebased the pull request onto origin/main: watching the run of rb000, its new head.\nCI run: https://ci/1\nCI passed: https://ci/1\n" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	f, _ = rebasing(t, "", false, "rb000")
	if code, stdout, stderr, _ := call(t, f, "-q"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("-q: exit %d\n%s%s", code, stdout, stderr)
	}

	// The commit the new head is watched under is a long one, shortened.
	clocked(t)
	f, looked = rebasing(t, "", false, "rb00000000")
	f.History = append(f.History, vcsfake.Commit{Name: "rb00000000", Parents: []string{up}, Message: "test: more\n", Tree: f.History[1].Tree})
	if _, stdout, _, _ := call(t, f); fmt.Sprint(*looked) != "[rb00000000]" || !strings.Contains(stdout, "watching the run of rb00000, its new head.\n") {
		t.Errorf("a long head: looked %v\n%s", *looked, stdout)
	}
}

// A head the watch fails on is the watch's: the run of the rebased head that
// failed exits 1, and a provider that cannot be built is as it was, before
// any gh pr view.
func TestPushItemBranchWatchesTheRebasedHeadToItsEnd(t *testing.T) {
	clocked(t)
	f, _ := rebasing(t, "", false, "rb000")
	f.watcher.Look = func(string) (providers.Run, bool, error) {
		return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: "failure"}, true, nil
	}
	if code, stdout, _, _ := call(t, f); code != policy || !strings.Contains(stdout, "watching the run of rb000, its new head.\n") {
		t.Errorf("a run that failed: exit %d\n%s", code, stdout)
	}
	f, _ = rebasing(t, "", false, "rb000")
	f.watchErr = errors.New("no token")
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	if code != missing || len(f.views()) != 0 || strings.Contains(stdout, "GitHub rebased") ||
		stderr != "itos: no token; the commits are pushed, and itos ci watch "+head+" waits for their run\n" {
		t.Errorf("a provider that cannot look: exit %d\n%s%s", code, stdout, stderr)
	}
}

// GitHub never rebasing: the wait gives up ci.watch.timeout after the
// update-branch call, not after the wait began, exiting 75 with the words and
// --json's ci timeout, and no run is looked at.
func TestPushItemBranchGivesUpOnARebaseThatNeverComes(t *testing.T) {
	c := clocked(t)
	f, looked := rebasing(t, "interval: 2, timeout: 5", false, "nw000")
	code, stdout, stderr, _ := call(t, f)
	head := f.head(t)
	if code != temporary || len(*looked) != 0 || fmt.Sprint(c.slept) != "[2s 2s 1s]" || len(f.views()) != 4 ||
		stdout != "Pushed "+head[:len(head)-1]+" to origin/slice-9. Pull request: "+open7+"\n" ||
		stderr != "itos: GitHub has not rebased the pull request onto origin/main after 5 seconds (ci.watch.timeout), so no CI run was watched; its checks are at "+open7+".\n" {
		t.Errorf("exit %d, looked %v, slept %v\n%s%s", code, *looked, c.slept, stdout, stderr)
	}
	clocked(t)
	f, looked = rebasing(t, "interval: 2, timeout: 5", false, "nw000")
	code, stdout, _, _ = call(t, f, "--json")
	r := reported(t, stdout)
	if code != temporary || r.OK || r.Outcome != "pushed" || r.CI != "timeout" || r.Run != nil || len(*looked) != 0 {
		t.Errorf("--json: exit %d\n%s", code, stdout)
	}

	// The time counts from the update-branch call: 3 seconds of it passed
	// before the first answer, so 2 are left, not 5.
	c = clocked(t)
	f, _ = rebasing(t, "interval: 4, timeout: 5", false, "nw000")
	answer := f.gh
	f.gh = func(args []string) (string, error) {
		defer func() {
			if args[1] == "update-branch" {
				c.at = c.at.Add(3 * time.Second)
			}
		}()
		return answer(args)
	}
	code, _, stderr, _ = call(t, f)
	if code != temporary || fmt.Sprint(c.slept) != "[2s]" || !strings.Contains(stderr, "after 5 seconds") {
		t.Errorf("counted from update-branch: exit %d, slept %v\n%s", code, c.slept, stderr)
	}

	// A fraction of a second is written as it is.
	c = clocked(t)
	f, _ = rebasing(t, "interval: 0.5, timeout: 1.5", false, "nw000")
	code, _, stderr, _ = call(t, f)
	if code != temporary || fmt.Sprint(c.slept) != "[500ms 500ms 500ms]" || !strings.Contains(stderr, "after 1.5 seconds") {
		t.Errorf("fractions: exit %d, slept %v\n%s", code, c.slept, stderr)
	}
}

// A gh pr view that fails, or whose answer is no head, is any gh call that
// fails: exit 3, pull-request-failed, naming gh pr view; the branch is pushed.
func TestPushItemBranchWhereGhPrViewFails(t *testing.T) {
	for name, c := range map[string]struct {
		view func() (string, error)
		says string
	}{
		"fails": {func() (string, error) {
			return "", &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}
		}, "gh pr view failed: HTTP 502\n"},
		"no gh":    {func() (string, error) { return "", providers.ErrNoGh }, "gh pr view failed: gh is not on the PATH\n"},
		"not json": {func() (string, error) { return "no\n", nil }, "gh pr view failed: its answer is not a pull request's head: invalid character 'o' in literal null (expecting 'u')\n"},
	} {
		t.Run(name, func(t *testing.T) {
			clocked(t)
			f, looked := rebasing(t, "", false, "rb000")
			answer := f.gh
			f.gh = func(args []string) (string, error) {
				if args[1] == "view" {
					return c.view()
				}
				return answer(args)
			}
			code, _, stderr, _ := call(t, f)
			if code != missing || len(*looked) != 0 || len(f.pushed) != 1 || !strings.Contains(stderr, c.says) {
				t.Errorf("exit %d\n%s", code, stderr)
			}
			f, _ = rebasing(t, "", false, "rb000")
			answer = f.gh
			f.gh = func(args []string) (string, error) {
				if args[1] == "view" {
					return c.view()
				}
				return answer(args)
			}
			_, stdout, _, _ := call(t, f, "--json")
			if r := reported(t, stdout); r.OK || r.Outcome != "pull-request-failed" {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
}

// Not behind main, the push waits for the run of the head it pushed, as a
// trunk push does, and exits with its result.
func TestPushItemBranchWaitsForItsRun(t *testing.T) {
	f := prRepo(t, "ci: { watch: { provider: github } }\n")
	f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: "failure"}, true, nil
	}}, true
	code, stdout, _, _ := call(t, f)
	head := f.head(t)
	if code != policy || !strings.HasPrefix(stdout, "Pushed "+head[:len(head)-1]+" to origin/slice-9. Pull request: "+opened+"\n") ||
		!strings.Contains(stdout, "CI run: https://ci/1") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
}

// A branch that adds nothing to main has nothing to push and no pull
// request.
func TestPushItemBranchAddsNothing(t *testing.T) {
	f := prRepo(t)
	f.on(item, up)
	code, stdout, _, _ := call(t, f, "--json")
	if r := reported(t, stdout); code != 0 || r.Outcome != "nothing-to-push" || r.PR != nil || len(f.ghCalls) != 0 || len(f.pushed) != 0 {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	if code, stdout, _, _ := call(t, f); code != 0 || stdout != "Nothing to push: slice-9 adds nothing to origin/main.\n" {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	f.commitsErr = errors.New("no such range")
	if code, _, stderr, _ := call(t, f); code != missing || !strings.Contains(stderr, "itos: cannot list the commits slice-9 adds to origin/main: no such range") {
		t.Errorf("a range that cannot be read: exit %d\n%s", code, stderr)
	}
}

// A branch named for no item of the registry is refused before anything is
// fetched, naming git branch -m.
func TestPushRefusesABranchThatNamesNoItem(t *testing.T) {
	f := prRepo(t)
	f.on("tidy", mine)
	f.fetchErr = errors.New("nothing is fetched")
	code, stdout, stderr, _ := call(t, f)
	if code != policy || stdout != "" || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
		stderr != "itos push: push.mode is pr, so a branch is named for its item, and tidy is no item of tasks/work-items.yaml; nothing was fetched or pushed.\n"+
			"Rename the branch with git branch -m <item>, then itos push again.\n" {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _, _ = call(t, f, "--json")
	if r := reported(t, stdout); r.OK || r.Outcome != "not-item-branch" || r.Branch != "tidy" {
		t.Errorf("--json: %s", stdout)
	}
	f = prRepo(t)
	f.cfg = loadedWith(t, "version: 1\npush: { mode: pr }\n", nil)
	if code, _, _, _ := call(t, f); code != policy {
		t.Errorf("no registry: exit %d", code)
	}
}

// Main is asked of the remote and must be a branch it has: a HEAD that
// names none, or one that names a branch it lacks, is refused, exit 3,
// before anything is fetched; a remote that cannot be asked, for its HEAD or
// for the branch it names, exits by the kind of git's message.
func TestPushNeedsMainOfTheRemote(t *testing.T) {
	for name, set := range map[string]func(*fake){
		"no branch":     func(f *fake) { f.remoteHead = "" },
		"a missing one": func(f *fake) { f.remoteHead = "trunk" },
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			set(f)
			f.fetchErr = errors.New("nothing is fetched")
			code, stdout, stderr, _ := call(t, f)
			if code != missing || stdout != "" || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
				!strings.Contains(stderr, "itos push: origin's HEAD names no branch origin has") {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			_, stdout, _, _ = call(t, f, "--json")
			if r := reported(t, stdout); r.OK || r.Outcome != "no-default-branch" {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
	for name, c := range map[string]struct {
		err  error
		code int
	}{
		"out of reach":  {&vcsport.Failed{Code: 128, Failure: vcsport.Unreachable, Words: "fatal: no network\n"}, temporary},
		"no repository": {&vcsport.Failed{Code: 128, Failure: vcsport.NoRepository, Words: "fatal: not a repository\n"}, missing},
		"another":       {&vcsport.Failed{Code: 128, Failure: vcsport.Other, Words: "fatal: odd\n"}, software},
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			f.headErr = c.err
			code, _, stderr, _ := call(t, f)
			if code != c.code || !strings.HasPrefix(stderr, "fatal: ") || !strings.Contains(stderr, "reading origin's HEAD failed") || len(f.pushed) != 0 {
				t.Errorf("exit %d\n%s", code, stderr)
			}
			f = prRepo(t)
			f.headErr = c.err
			if _, stdout, _, _ := call(t, f, "--json"); reported(t, stdout).Outcome != "fetch-failed" {
				t.Errorf("--json: %s", stdout)
			}
			// The listing that checks the remote has the branch fails alike.
			f = prRepo(t)
			f.refsErr = c.err
			code, _, stderr, _ = call(t, f)
			if code != c.code || !strings.HasPrefix(stderr, "fatal: ") || !strings.Contains(stderr, "listing origin's main failed") || len(f.pushed) != 0 {
				t.Errorf("a listing that fails: exit %d\n%s", code, stderr)
			}
		})
	}
}

// A branch with work for a pull request needs a GitHub repository to name,
// asked once it is rebased, before anything is pushed.
func TestPushNeedsAGitHubRepository(t *testing.T) {
	f := prRepo(t)
	f.repository = ""
	code, stdout, stderr, _ := call(t, f)
	if code != missing || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
		!strings.Contains(stderr, "itos push: slice-9 has work for a pull request, and origin names no GitHub repository to open it in; nothing was pushed.\n"+
			"Set GITHUB_REPOSITORY to owner/name, or point the remote at a GitHub repository, then itos push again.\n") {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _, _ = call(t, f, "--json")
	if r := reported(t, stdout); r.OK || r.Outcome != "no-github-repository" {
		t.Errorf("--json: %s", stdout)
	}
}

// The item's branch is pushed already when gh is missing or fails: exit 3,
// naming the command and why, and that itos push again finishes it.
func TestPushWhereGhFails(t *testing.T) {
	failing := func(at string, err error, out string) func([]string) (string, error) {
		return func(args []string) (string, error) {
			if strings.Join(args[:2], " ") == at {
				return out, err
			}
			return answers(opened, open7)(args)
		}
	}
	for name, c := range map[string]struct {
		gh   func([]string) (string, error)
		says string
	}{
		"no gh":    {failing("pr list", providers.ErrNoGh, ""), "itos push: origin/slice-9 is pushed, but gh pr list failed: gh is not on the PATH\n"},
		"words":    {failing("pr merge", &providers.GhFailed{Words: " HTTP 401 \n", Err: errors.New("exit status 1")}, ""), "gh pr merge failed: HTTP 401\n"},
		"no words": {failing("pr merge", &providers.GhFailed{Err: errors.New("exit status 1")}, ""), "gh pr merge failed: exit status 1\n"},
		"another":  {failing("pr merge", errors.New("killed"), ""), "gh pr merge failed: killed\n"},
		"not a list": {failing("pr list", nil, "no\n"),
			"gh pr list failed: its answer is not a list of pull requests: invalid character 'o' in literal null (expecting 'u')\n"},
		"create": {func(args []string) (string, error) {
			if args[1] == "create" {
				return "", &providers.GhFailed{Words: "HTTP 403\n", Err: errors.New("exit status 1")}
			}
			return "[]", nil
		}, "gh pr create failed: HTTP 403\n"},
		"no address": {func(args []string) (string, error) {
			if args[1] == "create" {
				return "\n", nil
			}
			return "[]", nil
		}, "gh pr create failed: it printed no address\n"},
		"update-branch": {failing("pr update-branch", &providers.GhFailed{Words: "conflicts\n", Err: errors.New("exit status 1")}, ""),
			"gh pr update-branch failed: conflicts\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f := prRepo(t)
			f.gh = c.gh
			f.on(item, "nw000")
			f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
			code, stdout, stderr, _ := call(t, f)
			if code != missing || stdout != "" || !strings.Contains(stderr, c.says) || len(f.pushed) != 1 ||
				!strings.Contains(stderr, "Fix that, then itos push again: it opens the pull request (gh pr create) and turns on auto-merge (gh pr merge --auto --rebase).\n") {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			f = prRepo(t)
			f.gh = c.gh
			f.on(item, "nw000")
			f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
			_, stdout, _, _ = call(t, f, "--json")
			if r := reported(t, stdout); r.OK || r.Outcome != "pull-request-failed" {
				t.Errorf("--json: %s", stdout)
			}
		})
	}
}

// What stops the pr routine before the push: a fetch of main or of the
// item's branch that fails, a rebase that stops (naming what it went onto),
// a HEAD the rebase left that names no commit, and a push that fails.
func TestPushItemBranchStops(t *testing.T) {
	f := prRepo(t)
	f.fetchErrs = map[string]error{"refs/heads/main": &vcsport.Failed{Code: 128, Failure: vcsport.Unreachable}}
	if code, _, stderr, _ := call(t, f); code != temporary || !strings.Contains(stderr, "fetching origin/main failed: origin cannot be reached") || len(f.pushed) != 0 {
		t.Errorf("main: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.fetchErrs = map[string]error{"refs/heads/slice-9": &vcsport.Failed{Code: 128, Failure: vcsport.NoRepository}}
	if code, _, stderr, _ := call(t, f); code != missing || !strings.Contains(stderr, "fetching origin/slice-9 failed: origin is no repository") || len(f.pushed) != 0 {
		t.Errorf("item: exit %d\n%s", code, stderr)
	}
	stopped := func(onto string) string {
		return "itos push: the rebase onto " + onto + " stopped; nothing was pushed.\n"
	}
	f = prRepo(t)
	f.History[1].Tree["mine.md"] = "theirs\n"
	if code, _, stderr, _ := call(t, f); code != policy || !strings.HasPrefix(stderr, stopped("origin/main")) || len(f.pushed) != 0 {
		t.Errorf("a first push: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "up000"
	f.History[1].Tree["mine.md"] = "theirs\n"
	if code, _, stderr, _ := call(t, f); code != policy || !strings.HasPrefix(stderr, stopped("origin/slice-9")) || len(f.pushed) != 0 {
		t.Errorf("a later push: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.rebase = func() error { f.Named["HEAD"] = "gone"; return nil }
	if code, _, stderr, _ := call(t, f); code != missing || stderr != "itos: cannot resolve HEAD after the rebase: HEAD names no commit\n" {
		t.Errorf("no HEAD: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.pushErr = &vcsport.Failed{Code: 128, Failure: vcsport.Unreachable}
	if code, _, stderr, _ := call(t, f); code != temporary || !strings.Contains(stderr, "the push to origin/slice-9 failed") || len(f.ghCalls) != 0 {
		t.Errorf("a push that fails: exit %d\n%s", code, stderr)
	}
}

// A work commit pushed from main is refused once rebased, naming git
// switch -c, nothing pushed; the registry's commits alone, and nothing to
// push, are not.
func TestPushFromMainUnderPr(t *testing.T) {
	onMain := func(config ...string) *fake {
		f := prRepo(t, config...)
		f.on("main", mine)
		return f
	}
	f := onMain()
	code, stdout, stderr, _ := call(t, f)
	if code != policy || stdout != "" || len(f.pushed) != 0 || len(f.ghCalls) != 0 ||
		stderr != "itos push: push.mode is pr, and this push adds commits to origin/main that touch more than tasks/work-items.yaml; nothing was pushed.\n"+
			"Carry the work to its item's branch with git switch -c <item>, then itos push again from there.\n" {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _, _ = call(t, f, "--json")
	if r := reported(t, stdout); r.OK || r.Outcome != "work-on-main" || r.Branch != "main" {
		t.Errorf("--json: %s", stdout)
	}
	f = onMain()
	f.History[2].Tree = map[string]string{"README.md": "start\n", registryFile: "taken\n"}
	if code, _, stderr, _ := call(t, f); code != 0 || len(f.pushed) != 1 || stderr != "" {
		t.Errorf("the registry alone: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f = onMain()
	f.on("main", up)
	if code, stdout, _, _ := call(t, f); code != 0 || !strings.HasPrefix(stdout, "Nothing to push") {
		t.Errorf("nothing to push: exit %d\n%s", code, stdout)
	}
	f = onMain()
	f.commitsErr = errors.New("no such range")
	if code, _, _, _ := call(t, f); code != policy || len(f.pushed) != 0 {
		t.Errorf("commits that cannot be listed: exit %d", code)
	}
	f = onMain()
	f.remoteHead = "trunk"
	if code, stdout, _, _ := call(t, f, "--json"); code != missing || reported(t, stdout).Outcome != "no-default-branch" {
		t.Errorf("a remote with no main: exit %d\n%s", code, stdout)
	}
}

// Trunk, the mode a config without push.mode has, and a config that cannot
// be read push as they always did: from main, which asks the remote's HEAD
// nothing, to its upstream's branch, or from a branch naming no item, which
// has no upstream and goes to the remote's default branch (bug 136), calling
// no gh.
func TestPushIsTrunkUnlessTheConfigSaysPr(t *testing.T) {
	for name, set := range map[string]func(*fake){
		"trunk":       func(f *fake) { f.cfg = loadedWith(t, "version: 1\npush: { mode: trunk }\n", nil) },
		"no mode":     func(f *fake) { f.cfg = loadedWith(t, "version: 1\n", nil) },
		"no config":   func(f *fake) { f.found = false },
		"unreadable":  func(f *fake) { f.loadErr = errors.New("bad") },
		"a bad value": func(f *fake) { f.cfg = loadedWith(t, "version: 1\n", nil); f.cfg.Push.Mode = "squash" },
	} {
		for _, branch := range []string{"main", "tidy"} {
			t.Run(name+" on "+branch, func(t *testing.T) {
				f := prRepo(t)
				set(f)
				f.on(branch, mine)
				if branch == "main" {
					f.headErr = errors.New("main is not asked")
				}
				code, stdout, stderr, _ := call(t, f)
				if code != 0 || len(f.pushed) != 1 || !strings.HasSuffix(f.pushed[0][0], ":refs/heads/main") || len(f.ghCalls) != 0 ||
					!strings.HasPrefix(stdout, "Pushed ") && !strings.Contains(stderr, "no CI run watched") {
					t.Errorf("exit %d, pushed %v\n%s%s", code, f.pushed, stdout, stderr)
				}
			})
		}
	}
}

// --ref naming main is judged as a push from main, against the remote's
// main: refused when it adds work, allowed when it adds the registry's
// commits alone; any other branch, a tag and a ref the clone lacks are
// pushNamed's as ever.
func TestPushRefUnderPr(t *testing.T) {
	f := prRepo(t)
	code, stdout, stderr, _ := call(t, f, "--ref", "main")
	if code != policy || stdout != "" || len(f.pushed) != 0 || !strings.Contains(stderr, "git switch -c <item>") ||
		!strings.Contains(stderr, "adds commits to origin/main that touch more than tasks/work-items.yaml") {
		t.Errorf("work: exit %d\n%s%s", code, stdout, stderr)
	}
	_, stdout, _, _ = call(t, f, "--json", "--ref", "main")
	if r := reported(t, stdout); r.Outcome != "work-on-main" || r.Branch != "main" {
		t.Errorf("--json: %s", stdout)
	}
	f = prRepo(t)
	f.History[2].Parents = []string{up}
	f.History[2].Tree = map[string]string{"README.md": "start\n", "up.md": "up.md\n", registryFile: "taken\n"}
	if code, _, _, _ := call(t, f, "--ref", "main"); code != 0 || len(f.pushed) != 1 {
		t.Errorf("the registry alone: exit %d, pushed %v", code, f.pushed)
	}
	f = prRepo(t)
	f.Named["refs/heads/ci/windows"] = mine
	if code, _, _, _ := call(t, f, "--ref", "ci/windows"); code != 0 || len(f.pushed) != 1 || f.pushed[0][0] != "refs/heads/ci/windows:refs/heads/ci/windows" {
		t.Errorf("another branch: exit %d, pushed %v", code, f.pushed)
	}
	f = prRepo(t)
	delete(f.Named, "refs/heads/main")
	if code, _, stderr, _ := call(t, f, "--ref", "main"); code != policy || !strings.Contains(stderr, "this clone has no heads/main") {
		t.Errorf("a main the clone lacks: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.fetchErrs = map[string]error{"refs/heads/main": &vcsport.Failed{Code: 128, Failure: vcsport.Unreachable}}
	if code, _, stderr, _ := call(t, f, "--ref", "main"); code != temporary || !strings.Contains(stderr, "fetching origin/main failed") || len(f.pushed) != 0 {
		t.Errorf("a fetch that fails: exit %d\n%s", code, stderr)
	}
	f = prRepo(t)
	f.headErr = &vcsport.Failed{Code: 128, Failure: vcsport.NoRepository}
	if code, _, _, _ := call(t, f, "--ref", "main"); code != missing || len(f.pushed) != 0 {
		t.Errorf("a remote that cannot be asked: exit %d", code)
	}
	// A remote with no main has none for --ref to name: the push goes on as
	// pushNamed has it.
	f = prRepo(t)
	f.remoteHead = "trunk"
	f.Named["refs/heads/ci/windows"] = mine
	if code, _, _, _ := call(t, f, "--ref", "ci/windows"); code != 0 || len(f.pushed) != 1 {
		t.Errorf("a remote with no main: exit %d, pushed %v", code, f.pushed)
	}
	// A tag asks nothing of the remote's HEAD.
	f = prRepo(t)
	f.headErr = errors.New("main is not asked")
	f.Named["refs/tags/v1"] = mine
	if code, _, _, _ := call(t, f, "--tag", "v1"); code != 0 || len(f.pushed) != 1 {
		t.Errorf("a tag: exit %d, pushed %v", code, f.pushed)
	}
}

// The ledger beside the registry (slice 177): a commit is the registry's, for
// the push from main and --ref main, when it touches the registry and every
// other path it touches is a ledger file.
func TestPushFromMainUnderPrTakesTheLedgerBesideTheRegistry(t *testing.T) {
	const layout = "ledger: { files: 'tasks/phase-{group}.yaml' }\n"
	with := func(extra ...string) map[string]string {
		tree := map[string]string{"README.md": "start\n", "up.md": "up.md\n", registryFile: "taken\n"}
		for _, p := range extra {
			tree[p] = p + "\n"
		}
		return tree
	}
	for name, c := range map[string]struct {
		config string
		tree   map[string]string
		pushed bool
	}{
		"the registry alone":                {layout, with(), true},
		"the registry and a ledger file":    {layout, with("tasks/phase-1.yaml"), true},
		"two ledger files":                  {layout, with("tasks/phase-1.yaml", "tasks/phase-2.yaml"), true},
		"the registry and another file":     {layout, with("tasks/phase-1.yaml", "widget.md"), false},
		"a name the ledger does not have":   {layout, with("tasks/notes.yaml"), false},
		"a folder the ledger does not have": {layout, with("other/phase-1.yaml"), false},
		"a folder inside the ledger's":      {layout, with("tasks/sub/phase-1.yaml"), false},
		"no ledger section":                 {"", with("tasks/phase-1.yaml"), false},
		"the ledger alone":                  {layout, map[string]string{"README.md": "start\n", "up.md": "up.md\n", "tasks/phase-1.yaml": "x\n"}, false},
	} {
		for _, args := range [][]string{nil, {"--ref", "main"}} {
			t.Run(name+strings.Join(args, " "), func(t *testing.T) {
				f := prRepo(t, c.config)
				f.on("main", mine)
				f.History[2].Parents = []string{up}
				f.History[2].Tree = c.tree
				code, _, stderr, _ := call(t, f, args...)
				if c.pushed && (code != 0 || len(f.pushed) != 1) || !c.pushed && (code != policy || len(f.pushed) != 0) {
					t.Errorf("exit %d, pushed %v\n%s", code, f.pushed, stderr)
				}
			})
		}
	}
}

// A take directly above main that the remote's item branch has already is not
// sent to main again.
func TestPushItemBranchDoesNotSendATakeItAlreadyHas(t *testing.T) {
	f := prRepo(t)
	taken := map[string]string{"README.md": "start\n", "up.md": "up.md\n", registryFile: "taken\n"}
	worked := maps.Clone(taken)
	worked["work.md"] = "work\n"
	f.History = append(f.History,
		vcsfake.Commit{Name: "tk010", Parents: []string{up}, Message: "docs: take\n", Tree: taken},
		vcsfake.Commit{Name: "wk010", Parents: []string{"tk010"}, Message: "feat: work\n", Tree: worked})
	f.on(item, "wk010")
	f.Remote["origin"].Refs["refs/heads/slice-9"] = "tk010"
	code, _, stderr, _ := call(t, f)
	if code != 0 || len(f.pushed) != 1 || len(f.pushed[0]) != 1 || f.pushed[0][0] != "wk010:refs/heads/slice-9" {
		t.Fatalf("exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}

// Paths that cannot be read are not the registry's, nor are none at all.
func TestRegistryPathsNeedTheRegistryAndPathsThatRead(t *testing.T) {
	cfg := loaded(t, "version: 1\nledger: { files: 'tasks/phase-{group}.yaml' }\n")
	registry := cfg.Work.Registry
	if registryPaths(cfg, []string{registry}, errors.New("unreadable")) || registryPaths(cfg, nil, nil) {
		t.Error("paths that cannot be read, or none, were the registry's")
	}
	if !registryPaths(cfg, []string{"tasks/phase-1.yaml", registry}, nil) {
		t.Error("the registry and a ledger file were not the registry's")
	}
}

// A ledger-only commit is not made the registry's by a commit beside it that
// touches the registry: the push is judged commit by commit.
func TestPushFromMainUnderPrJudgesEachCommit(t *testing.T) {
	f := prRepo(t, "ledger: { files: 'tasks/phase-{group}.yaml' }\n")
	f.on("main", "rg000")
	ledgerTree := map[string]string{"README.md": "start\n", "up.md": "up.md\n", "tasks/phase-1.yaml": "x\n"}
	registryTree := maps.Clone(ledgerTree)
	registryTree[registryFile] = "taken\n"
	f.History = append(f.History,
		vcsfake.Commit{Name: "lg000", Parents: []string{up}, Message: "docs: a check\n", Tree: ledgerTree},
		vcsfake.Commit{Name: "rg000", Parents: []string{"lg000"}, Message: "docs: take\n", Tree: registryTree})
	if code, _, stderr, _ := call(t, f); code != policy || len(f.pushed) != 0 {
		t.Errorf("exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
	f.History[len(f.History)-2].Tree = registryTree
	f.History[len(f.History)-1].Tree = maps.Clone(registryTree)
	f.History[len(f.History)-1].Tree[registryFile] = "taken twice\n"
	if code, _, stderr, _ := call(t, f); code != 0 || len(f.pushed) != 1 {
		t.Errorf("a close and a take: exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}

// On an item's branch a task added directly above main goes to main as a take
// does, and a ledger-only commit above it stays with the work.
func TestPushItemBranchSendsALedgerBesideTheRegistryToMain(t *testing.T) {
	f := prRepo(t, "ledger: { files: 'tasks/phase-{group}.yaml' }\n")
	added := map[string]string{"README.md": "start\n", "up.md": "up.md\n", registryFile: "taken\n", "tasks/phase-1.yaml": "x\n"}
	checked := maps.Clone(added)
	checked["tasks/phase-1.yaml"] = "y\n"
	f.History = append(f.History,
		vcsfake.Commit{Name: "td000", Parents: []string{up}, Message: "docs: add\n", Tree: added},
		vcsfake.Commit{Name: "ck000", Parents: []string{"td000"}, Message: "docs: check\n", Tree: checked})
	f.on(item, "ck000")
	code, _, stderr, _ := call(t, f)
	if code != 0 || len(f.pushed) != 1 || len(f.pushed[0]) != 2 || f.pushed[0][0] != "td000:refs/heads/main" || f.pushed[0][1] != "ck000:refs/heads/slice-9" {
		t.Fatalf("exit %d, pushed %v\n%s", code, f.pushed, stderr)
	}
}
