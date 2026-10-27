package work

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// prDone is a fixture under push.mode pr whose HEAD is on main, the remote's
// HEAD names main, which it has, and GitHub's repository is owner/scratch;
// T-1's own commit, c-1a, is on origin/main; gh lists the pull requests.
func prDone(t *testing.T, pulls string) (ports, *[][]string, *[]string) {
	t.Helper()
	f := closable(false)
	f.config = configText + "push:\n  mode: pr\n"
	p := over(t, f)
	v := vcsOf(p)
	v.OnBranch = "main"
	v.Remote = map[string]*vcsfake.Remote{"origin": {URL: "u", Refs: map[string]string{"refs/heads/main": "c-4a"}}}
	v.Named["refs/remotes/origin/main"] = "c-1a"
	calls, heads := &[][]string{}, &[]string{}
	p.pr = pr{head: "main", repository: "owner/scratch", ghOut: pulls, ghCalls: calls, headCalls: heads}
	return p, calls, heads
}

const (
	mergedPull = `[{"state":"MERGED","mergedAt":"2026-10-24T10:00:00Z","number":7,"url":"https://github.example/o/r/pull/7"}]`
	openPull   = `[{"state":"OPEN","mergedAt":null,"number":7,"url":"https://github.example/o/r/pull/7"}]`
	closedPull = `[{"state":"CLOSED","mergedAt":null,"number":7,"url":"https://github.example/o/r/pull/7"}]`
)

// Under push.mode pr, work done refuses, nothing written, whatever it cannot
// ask or finds: HEAD not on main, a remote that cannot be asked, one without
// a main, no GitHub repository, a gh that fails, and the item's pull
// requests, each with its rule and exit; --json gives the rule as a problem.
func TestDonePRRefuses(t *testing.T) {
	const pull = "https://github.example/o/r/pull/7"
	for _, c := range []struct {
		name, id string
		set      func(*ports)
		code     int
		rule     string
		message  string
		fix      string
	}{
		{"HEAD on another branch", "T-1", func(p *ports) { vcsOf(*p).OnBranch = "T-1" }, 1, "work-done-not-main",
			"push.mode is pr, so work done runs on main, where its close goes, and HEAD is on T-1",
			"switch to main with git switch main, bring it up to date with git pull, then run itos work done T-1 again"},
		{"HEAD on no branch", "T-1", func(p *ports) { vcsOf(*p).OnBranch = "" }, 1, "work-done-not-main",
			"push.mode is pr, so work done runs on main, where its close goes, and HEAD is on no branch", "switch to main with git switch main"},
		{"a remote HEAD that names no branch", "T-1", func(p *ports) { p.head = "" }, 3, "work-done-no-main",
			"push.mode is pr, so work done runs on main, and origin's HEAD names no branch origin has",
			"set the remote's default branch, then run itos work done T-1 again"},
		{"a remote HEAD that names a branch it lacks", "T-1", func(p *ports) { p.head = "trunk" }, 3, "work-done-no-main",
			"origin's HEAD names no branch origin has", ""},
		{"a remote out of reach", "T-1", func(p *ports) {
			p.headErr = &vcsport.Failed{Command: "git ls-remote", Code: 128, Failure: vcsport.Unreachable, Words: "fatal: could not resolve host\n"}
		}, 75, "work-done-remote-failed", "push.mode is pr, so work done runs on main, and reading origin's HEAD failed",
			"fix that, then run itos work done T-1 again"},
		{"a remote that is no repository", "T-1", func(p *ports) {
			p.headErr = &vcsport.Failed{Command: "git ls-remote", Code: 128, Failure: vcsport.NoRepository, Words: "fatal: not a repository\n"}
		}, 3, "work-done-remote-failed", "reading origin's HEAD failed", ""},
		{"a git that does not run", "T-1", func(p *ports) { p.headErr = errors.New("no git") }, 3, "work-done-remote-failed",
			"reading origin's HEAD failed", ""},
		{"a listing of its branch that fails", "T-1", func(p *ports) { vcsOf(*p).Remote["origin"].Down = true }, 75, "work-done-remote-failed",
			"reading origin's HEAD failed", ""},
		{"no GitHub repository", "T-1", func(p *ports) { p.repository = "" }, 3, "work-done-no-github-repository",
			"push.mode is pr, so work done asks for T-1's pull request, and origin names no GitHub repository to ask in",
			"set GITHUB_REPOSITORY to owner/name, or point the remote at a GitHub repository, then run itos work done T-1 again"},
		{"no gh on the PATH", "T-1", func(p *ports) { p.ghErr = providers.ErrNoGh }, 3, "work-done-pull-request-failed",
			"looking for T-1's pull request failed: gh pr list failed: gh is not on the PATH", "fix that, then run itos work done T-1 again"},
		{"a gh that fails", "T-1", func(p *ports) {
			p.ghErr = &providers.GhFailed{Words: "HTTP 401: Bad credentials\n", Err: errors.New("exit status 1")}
		}, 3, "work-done-pull-request-failed", "gh pr list failed: HTTP 401: Bad credentials", ""},
		{"an answer that is no list", "T-1", func(p *ports) { p.ghOut = "{}" }, 3, "work-done-pull-request-failed",
			"gh pr list failed: its answer is not a list of pull requests: json: cannot unmarshal object", ""},
		{"an open pull request", "T-1", func(p *ports) { p.ghOut = openPull }, 1, "work-done-pr-open",
			"T-1's pull request " + pull + " is open, so its work is not on origin/main and T-1 is not done",
			"wait for it to merge, then run itos work done T-1 again"},
		{"an open pull request and a merged one", "T-1", func(p *ports) {
			p.ghOut = strings.Replace(mergedPull, "]", ",", 1) + openPull[1:]
		}, 1, "work-done-pr-open", "is open", ""},
		{"a merged pull request the clone has not pulled", "slice-2", func(p *ports) { p.ghOut = mergedPull }, 1, "work-done-pr-merged",
			"slice-2's pull request " + pull + " has merged, but HEAD has no commit of slice-2 that origin/main has, and origin's main has moved since this clone last pulled it",
			"bring main up to date with git pull, then run itos work done slice-2 again"},
		{"a merged pull request and a main the clone has pulled", "slice-2", func(p *ports) {
			p.ghOut = mergedPull
			vcsOf(*p).Remote["origin"].Refs["refs/heads/main"] = "c-1a"
		}, 1, "work-done-pr-merged",
			"slice-2's pull request " + pull + " has merged, but HEAD has no commit of slice-2 that origin/main has, though this clone has pulled origin's main as it is now, so the pull request did not merge into main",
			"push its work again from the branch slice-2 with itos push, which opens a new pull request, then run itos work done slice-2 again"},
		{"a merged pull request and a HEAD behind main", "slice-2", func(p *ports) {
			p.ghOut = mergedPull
			vcsOf(*p).Named["HEAD"] = "c-3a"
			vcsOf(*p).Named["refs/remotes/origin/main"] = "c-4a"
		}, 1, "work-done-pr-merged", "and origin's main has moved since this clone last pulled it",
			"bring main up to date with git pull, then run itos work done slice-2 again"},
		{"a merged pull request and a commit on main here alone", "T-1", func(p *ports) {
			p.ghOut = mergedPull
			delete(vcsOf(*p).Named, "refs/remotes/origin/main")
		}, 1, "work-done-pr-merged", "has merged, but HEAD has no commit of T-1 that origin/main has", ""},
		{"a pull request closed without merging", "slice-2", func(p *ports) { p.ghOut = closedPull }, 1, "work-done-pr-closed",
			"slice-2's pull request " + pull + " was closed without merging, so its work is not on origin/main and slice-2 is not done",
			"push its work again from the branch slice-2 with itos push, which opens a new pull request, then run itos work done slice-2 again"},
		{"a merged pull request and a closed one", "slice-2", func(p *ports) {
			p.ghOut = strings.Replace(closedPull, "]", ",", 1) + mergedPull[1:]
		}, 1, "work-done-pr-merged", "has merged", ""},
		{"a first closed pull request", "slice-2", func(p *ports) {
			p.ghOut = `[{"state":"CLOSED","mergedAt":null,"url":"https://github.example/o/r/pull/9"},` + closedPull[1:]
		}, 1, "work-done-pr-closed", "https://github.example/o/r/pull/9 was closed", ""},
		{"no pull request", "slice-2", func(p *ports) { p.ghOut = "[]" }, 1, "work-done-no-pull-request",
			"origin/main has no commit of slice-2 and slice-2 has no pull request, so its work has not landed",
			"push its work from the branch slice-2 with itos push, which opens its pull request, then run itos work done slice-2 again"},
		{"a pull request of a state it does not know", "slice-2", func(p *ports) {
			p.ghOut = `[{"state":"DRAFT","mergedAt":null,"url":"https://github.example/o/r/pull/9"}]`
		}, 1, "work-done-no-pull-request", "has no pull request", ""},
		{"a scenario still @wip", "slice-3", nil, 1, "work-done-wip", "is still @wip at HEAD", ""},
	} {
		p, calls, _ := prDone(t, "[]")
		if c.set != nil {
			c.set(&p)
		}
		code, stdout, stderr, err := run(t, p, "done", c.id)
		if code != c.code || err != nil || stdout != "" || !strings.Contains(stderr, c.message) || !strings.Contains(stderr, "fix: "+c.fix) ||
			registryIn(t, p) != doneText {
			t.Errorf("%s: work done %s = %d, %v, %q, %q", c.name, c.id, code, err, stdout, stderr)
		}
		if c.rule == "work-done-wip" && len(*calls) != 0 {
			t.Errorf("%s: gh was called: %q", c.name, *calls)
		}
		p, _, _ = prDone(t, "[]")
		if c.set != nil {
			c.set(&p)
		}
		code, stdout, _, err = run(t, p, "--json", "done", c.id)
		o := object(t, stdout)
		problems, _ := o["problems"].([]any)
		if code != c.code || err != nil || o["ok"] != false || len(problems) != 1 || problems[0].(map[string]any)["rule"] != c.rule {
			t.Errorf("%s: work --json done %s = %d, %v, %v", c.name, c.id, code, err, o)
		}
	}
}

// The remote is asked for its HEAD without a prompt, so a remote that wants
// credentials fails the question instead of git waiting on a terminal.
func TestDonePRAsksTheRemoteWithoutPrompting(t *testing.T) {
	p, _, _ := prDone(t, mergedPull)
	talks := &[]vcsport.Talk{}
	p.pr.headTalks = talks
	if code, _, stderr, err := run(t, p, "done", "T-1"); err != nil {
		t.Fatalf("work done T-1 = %d, %v, %q", code, err, stderr)
	}
	if len(*talks) != 1 || !(*talks)[0].NoPrompt {
		t.Errorf("the remote's HEAD was asked with %+v, want one question with NoPrompt", *talks)
	}
}

// A remote that cannot be asked gives git's words on stderr, trimmed, or the
// error when git did not run.
func TestDonePRSaysGitsWords(t *testing.T) {
	p, _, _ := prDone(t, "[]")
	p.headErr = &vcsport.Failed{Command: "git ls-remote", Code: 128, Failure: vcsport.Unreachable, Words: "fatal: could not resolve host\n"}
	if _, _, stderr, _ := run(t, p, "done", "T-1"); !strings.HasPrefix(stderr, "fatal: could not resolve host\npush.mode is pr") {
		t.Errorf("a remote out of reach says %q", stderr)
	}
	p, _, _ = prDone(t, "[]")
	p.headErr = errors.New("no git")
	if _, _, stderr, _ := run(t, p, "done", "T-1"); !strings.HasPrefix(stderr, "no git\npush.mode is pr") {
		t.Errorf("a git that does not run says %q", stderr)
	}
}

// Under push.mode pr, work done asks the remote's HEAD, then gh for the
// item's pull requests with every state, naming the repository, and goes on
// as before when none is open and the newest of the item's own commits is on
// the remote's main as last fetched: a merged pull request, or none, closes
// it. The remote is the branch's upstream's.
func TestDonePRCloses(t *testing.T) {
	for _, pulls := range []string{mergedPull, "[]", closedPull} {
		p, calls, heads := prDone(t, pulls)
		vcsOf(p).Tracking = map[string][2]string{"main": {"upstream", "refs/heads/main"}}
		vcsOf(p).Remote["upstream"] = vcsOf(p).Remote["origin"]
		vcsOf(p).Named["refs/remotes/upstream/main"] = "c-1a"
		code, stdout, stderr, err := run(t, p, "done", "T-1")
		if code != 0 || err != nil || !strings.HasPrefix(stdout, "T-1 is done: committed ") || strings.Contains(stderr, "no remote has") {
			t.Errorf("work done T-1 with %s = %d, %v, %q, %q", pulls, code, err, stdout, stderr)
		}
		want := [][]string{{"pr", "list", "--repo", "owner/scratch", "--head", "T-1", "--state", "all", "--json", "state,mergedAt,number,url,headRefOid,mergeCommit"}}
		if len(*calls) != 1 || !slices.Equal((*calls)[0], want[0]) || !slices.Equal(*heads, []string{"upstream"}) {
			t.Errorf("work done T-1 asked gh %q and the remote %q, want %q and upstream", *calls, *heads, want)
		}
	}
	p, _, heads := prDone(t, "[]")
	if code, _, _, err := run(t, p, "done", "slice-1"); code != 0 || err != nil || len(*heads) != 0 {
		t.Errorf("work done slice-1, already done, asked the remote: %d, %v, %q", code, err, *heads)
	}
}

// Under push.mode trunk, or with none, work done asks neither the remote's
// HEAD nor gh.
func TestDoneTrunkAsksNoGh(t *testing.T) {
	p := over(t, closable(false))
	calls, heads := &[][]string{}, &[]string{}
	p.pr = pr{ghCalls: calls, headCalls: heads}
	if code, _, _, err := run(t, p, "done", "slice-2"); code != 0 || err != nil || len(*calls)+len(*heads) != 0 {
		t.Errorf("work done slice-2 = %d, %v, gh %q, remote %q", code, err, *calls, *heads)
	}
}

// A merged pull request's merge commit is the item's newest own commit when
// HEAD has it and the item's own commits do not come after it (slice 195);
// else the commit the item's footers name, if any, is judged as before.
func TestWithMergeCommit(t *testing.T) {
	p, _, _ := prDone(t, "[]")
	v := vcsOf(p)
	own := doneCommit{sha: "c-1a", run: "The CI run of c-1, the newest commit of slice-2,", what: "c-1, the newest commit of slice-2", own: true}
	newest := doneCommit{sha: "c-4a", run: "The CI run of c-4a, the newest commit of slice-2,", what: "c-4a, the newest commit of slice-2", own: true}
	merged := func(oid string) *pull {
		m := &pull{MergeCommit: &struct {
			Oid string `json:"oid"`
		}{oid}}
		return m
	}
	for _, c := range []struct {
		name    string
		pull    *pull
		landing doneCommit
		want    doneCommit
	}{
		{"no pull request", nil, own, own},
		{"no merge commit", &pull{}, own, own},
		{"an empty merge commit", merged(""), own, own},
		{"a merge commit HEAD lacks", merged("x-9z"), own, own},
		{"a merge commit and no own commit", merged("c-4a"), doneCommit{}, newest},
		{"a merge commit after the own commit", merged("c-4a"), own, newest},
		{"the own commit as the merge commit", merged("c-1a"), own, doneCommit{sha: "c-1a", run: "The CI run of c-1a, the newest commit of slice-2,", what: "c-1a, the newest commit of slice-2", own: true}},
		{"an own commit after the merge commit", merged("c-2a"), doneCommit{sha: "c-3a", own: true}, doneCommit{sha: "c-3a", own: true}},
	} {
		if got := withMergeCommit(t.Context(), v, "slice-2", c.pull, c.landing); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

// An item with no commit of its own is done by its merged pull request's merge
// commit once origin/main has it, and not before.
func TestDonePRClosesByTheMergeCommit(t *testing.T) {
	const pulls = `[{"state":"MERGED","mergedAt":"2026-10-24T10:00:00Z","number":7,"url":"https://github.example/o/r/pull/7","mergeCommit":{"oid":"c-4a"}}]`
	p, _, _ := prDone(t, pulls)
	vcsOf(p).Named["refs/remotes/origin/main"] = "c-4a"
	code, stdout, stderr, err := run(t, p, "done", "slice-2")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "slice-2 is done: committed ") {
		t.Errorf("work done slice-2 = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p, _, _ = prDone(t, pulls)
	code, _, stderr, err = run(t, p, "done", "slice-2")
	if code != 1 || err != nil || !strings.Contains(stderr, "has merged, but HEAD has no commit of slice-2 that origin/main has") {
		t.Errorf("work done slice-2 with origin/main behind = %d, %v, %q", code, err, stderr)
	}
}
