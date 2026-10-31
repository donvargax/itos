package push

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/providers"
)

const (
	cleanWords = "GraphQL: Pull request Pull request is in clean status (enablePullRequestAutoMerge)"
	otherSHA   = "ffffffffffffffffffffffffffffffffffffffff"
)

// refused is a gh that failed with the words.
func refused(words string) error {
	return &providers.GhFailed{Words: words + "\n", Err: errors.New("exit status 1")}
}

// ago is the time that many seconds before the clock's start, as GitHub writes
// it, so that it does not move as the clock does.
func (c *clock) ago(seconds int) string {
	return time.Unix(1_000_000, 0).Add(-time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339)
}

// timedNode is a comment as GraphQL gives it with the times it was made and last
// edited, left out when empty.
func timedNode(login, body, created, updated string) string {
	return fmt.Sprintf(`{"author":{"login":%q},"body":%q,"createdAt":%q,"updatedAt":%q}`, login, body, created, updated)
}

// answerAt is the pull request's answer with the review and comment nodes and
// the time it was last marked ready, none when empty.
func answerAt(reviews, comments []string, ready string) string {
	timeline := ""
	if ready != "" {
		timeline = fmt.Sprintf(`"timelineItems":{"nodes":[{"createdAt":%q}]},`, ready)
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{%s"reviews":{"nodes":[%s]},"comments":{"nodes":[%s]},"reviewThreads":{"nodes":[]}}}}}`,
		timeline, strings.Join(reviews, ","), strings.Join(comments, ","))
}

// push.pr.ready_wait ends one --ready call, counted from its start, with the
// outcome ready-still-waiting, exit 75, saying how long it waited, what it waits
// on and the full SHA of the head: the asks at 0, 2, 4 and 5 seconds, the last
// sleeps shortened to the bound. ci.watch.timeout still ends the wait when it
// comes first or at the same time, and 0 is no bound.
func TestPushReadyStopsAtReadyWait(t *testing.T) {
	for name, c := range map[string]struct {
		config  string
		timeout string
		outcome string
		asks    int
		slept   string
	}{
		"the bound":                  {"ready_wait: 5", "100", "ready-still-waiting", 4, "[2s 2s 1s]"},
		"the bound nearer":           {"ready_wait: 3", "6", "ready-still-waiting", 3, "[2s 1s]"},
		"the timeout nearer":         {"ready_wait: 7", "6", "review-timeout", 4, "[2s 2s 2s]"},
		"both at once":               {"ready_wait: 6", "6", "review-timeout", 4, "[2s 2s 2s]"},
		"no bound":                   {"ready_wait: 0", "6", "review-timeout", 4, "[2s 2s 2s]"},
		"a bound shorter than a ask": {"ready_wait: 0.5", "100", "ready-still-waiting", 2, "[500ms]"},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, "draft: true, review_by: [a], "+c.config, "ci: { watch: { interval: 2, timeout: "+c.timeout+" } }\n")
			p.open, p.draft = open7, true
			clk := clocked(t)
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			r := reported(t, stdout)
			if code != temporary || r.OK || r.Outcome != c.outcome || r.PR == nil || r.PR.AutoMerge || p.asks != c.asks || fmt.Sprint(clk.slept) != c.slept ||
				strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
				t.Fatalf("exit %d, asks %d, slept %v\n%s%s", code, p.asks, clk.slept, stdout, stderr)
			}
			seconds := strings.TrimPrefix(c.config, "ready_wait: ")
			want := "itos: still waiting after " + seconds + " seconds (push.pr.ready_wait) on " + open7 + " at " + f.head(t) +
				": a waiting; auto-merge is not turned on. itos push --ready again goes on waiting.\n"
			if bounded := c.outcome == "ready-still-waiting"; bounded != strings.HasSuffix(stderr, want) || bounded == strings.Contains(stderr, "ci.watch.timeout") {
				t.Errorf("stderr\n%s\nwant\n%s", stderr, want)
			}
		})
	}
}

// limits judges the two limits of a wait: the deadline of ci.watch.timeout and
// the bound of push.pr.ready_wait, only the one that passed first when both
// have, and the time to the nearer one.
func TestLimits(t *testing.T) {
	c := clocked(t)
	at := func(seconds int) time.Time { return c.at.Add(time.Duration(seconds) * time.Second) }
	for name, k := range map[string]struct {
		deadline, bound         int
		hasBound                bool
		timedOut, bounded       bool
		left                    time.Duration
		hasBoundZero, deadlines bool
	}{
		"no bound, ahead":          {deadline: 10, left: 10 * time.Second},
		"no bound, passed":         {deadline: -3, timedOut: true, left: -3 * time.Second},
		"no bound, now":            {deadline: 0, timedOut: true},
		"both ahead, bound nearer": {deadline: 10, bound: 4, hasBound: true, left: 4 * time.Second},
		"both ahead, timeout near": {deadline: 3, bound: 4, hasBound: true, left: 3 * time.Second},
		"the timeout passed":       {deadline: -2, bound: 4, hasBound: true, timedOut: true, left: -2 * time.Second},
		"the bound passed":         {deadline: 5, bound: -2, hasBound: true, bounded: true, left: -2 * time.Second},
		"the bound passed first":   {deadline: -1, bound: -5, hasBound: true, bounded: true, left: -5 * time.Second},
		"the timeout passed first": {deadline: -5, bound: -1, hasBound: true, timedOut: true, left: -5 * time.Second},
		"both at once":             {deadline: 0, bound: 0, hasBound: true, timedOut: true},
		"the bound is now":         {deadline: 5, bound: 0, hasBound: true, bounded: true},
	} {
		t.Run(name, func(t *testing.T) {
			r := &pushRun{}
			if k.hasBound {
				r.bound = at(k.bound)
			}
			timedOut, bounded, left := r.limits(at(k.deadline))
			if timedOut != k.timedOut || bounded != k.bounded || left != k.left {
				t.Errorf("limits = %t, %t, %s; want %t, %t, %s", timedOut, bounded, left, k.timedOut, k.bounded, k.left)
			}
		})
	}
}

// With a provider, GitHub's rebase that has not shown by push.pr.ready_wait
// ends the call, rebase waiting, with the head pushed; and the run that goes on
// past it ends it too, run waiting, with the head GitHub made. Both are the
// real clock's, which the watch keeps.
func TestPushReadyStopsAtReadyWaitWhileWatching(t *testing.T) {
	const settings = "ci: { watch: { provider: github, interval: 0.02, timeout: 30 } }\n"
	behind := func(t *testing.T, heads ...string) (*fake, *pulls) {
		f, p := draftRepo(t, "draft: true, ready_wait: 0.2", settings)
		f.History = append(f.History, rebasedCommit(f))
		p.open, p.draft, p.heads = open7, true, heads
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.on(item, "nw000")
		return f, p
	}
	f, _ := behind(t, mine)
	f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		t.Error("a run was looked at")
		return providers.Run{}, false, nil
	}}, true
	began := time.Now()
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	want := "itos: still waiting after 0.2 seconds (push.pr.ready_wait) on " + open7 + " at nw000: rebase waiting; auto-merge is not turned on. itos push --ready again goes on waiting.\n"
	if code != temporary || r.Outcome != "ready-still-waiting" || r.CI != "" || r.PR == nil || r.PR.AutoMerge || !strings.HasSuffix(stderr, want) || time.Since(began) > 10*time.Second ||
		strings.Contains(stderr, "ci.watch.timeout") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("rebase waiting: exit %d\n%s%s", code, stdout, stderr)
	}

	f, _ = behind(t, "rb000")
	f.watcher, f.watchOK = providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		return providers.Run{URL: "https://ci/1", Status: "in_progress", Jobs: []providers.Job{{Name: "ci", Status: "in_progress"}}}, true, nil
	}}, true
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	r = reported(t, stdout)
	want = "itos: still waiting after 0.2 seconds (push.pr.ready_wait) on " + open7 + " at rb000: run waiting; auto-merge is not turned on. itos push --ready again goes on waiting.\n"
	if code != temporary || r.Outcome != "ready-still-waiting" || r.CI != "" || r.PR == nil || r.PR.AutoMerge || !strings.HasSuffix(stderr, want) ||
		strings.Contains(stderr, "ci.watch.timeout") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("run waiting: exit %d\n%s%s", code, stdout, stderr)
	}
}

// With no ci.watch provider the head GitHub made is read once after the rebase
// was asked for, with no wait: it is the head judged when it has main's head
// (the review states are compared with it, the merge pinned to it), and when it
// has not, --ready stops at once, rebase waiting, having asked GitHub for
// nothing more and merged nothing. A gh pr view that fails is any gh call that
// fails.
func TestPushReadyWithNoProviderReadsTheRebasedHeadOnce(t *testing.T) {
	behind := func(t *testing.T, heads ...string) (*fake, *pulls) {
		f, p := draftRepo(t, "draft: true, review_by: [a]")
		f.History = append(f.History, rebasedCommit(f))
		p.open, p.draft, p.heads = open7, true, heads
		p.graph = func(int) string { return answerOf(nil, []string{doneBy("a")}) }
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.on(item, "nw000")
		return f, p
	}
	view := "pr view slice-9 --repo owner/name --json headRefOid"
	update := "pr update-branch slice-9 --repo owner/name --rebase"

	f, _ := behind(t, "rb000")
	code, stdout, stderr, _ := call(t, f, "--ready")
	if got, want := f.calls(), append([]string{list, ready, update, view, graphql(7), activity}, merged("rb000")...); code != 0 || !reflect.DeepEqual(got, want) ||
		!strings.Contains(stdout, "Waiting on "+open7+" at rb000: a not asked.\n") {
		t.Errorf("exit %d\n%s%s\n%s", code, stdout, stderr, strings.Join(f.calls(), "\n"))
	}

	f, _ = behind(t, mine)
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	want := "itos: still waiting on " + open7 + " at nw000: rebase waiting; GitHub has not rebased it onto origin/main yet, so auto-merge is not turned on. itos push --ready again goes on waiting.\n"
	if code != temporary || r.Outcome != "ready-still-waiting" || r.PR == nil || r.PR.AutoMerge || stderr != want ||
		!reflect.DeepEqual(f.calls(), []string{list, ready, update, view}) {
		t.Errorf("not rebased: exit %d\n%s%s\n%s", code, stdout, stderr, strings.Join(f.calls(), "\n"))
	}

	f, p := behind(t, "rb000")
	p.fail = map[string]error{"pr view": refused("HTTP 502")}
	if code, _, stderr, _ := call(t, f, "--ready"); code != missing || !strings.Contains(stderr, "gh pr view failed: HTTP 502") || len(f.calls()) != 4 {
		t.Errorf("gh pr view fails: exit %d\n%s", code, stderr)
	}
}

// mergeRepo is a --ready with nothing to wait on whose pr merge calls are
// answered by the function first.
func mergeRepo(t *testing.T, merging func(args []string) error) (*fake, *pulls) {
	t.Helper()
	f, p := draftRepo(t, "draft: true")
	p.open, p.draft = open7, true
	inner := f.gh
	f.gh = func(args []string) (string, error) {
		if strings.Join(args[:2], " ") == "pr merge" {
			if err := merging(args); err != nil {
				return "", err
			}
		}
		return inner(args)
	}
	return f, p
}

// autoRefused refuses the calls that ask for auto-merge, with the words.
func autoRefused(words string) func([]string) error {
	return func(args []string) error {
		if strings.Contains(strings.Join(args, " "), "--auto") {
			return refused(words)
		}
		return nil
	}
}

// Every merge call carries the head judged. GitHub's refusal of auto-merge as
// clean (the pull request is mergeable already) is answered by one direct merge
// by rebase, pinned the same, and then the pull request is read back: merged,
// since the call that succeeded merged it, unless the answer says state.
func TestPushReadyMergesAPullRequestGitHubRefusesAsClean(t *testing.T) {
	f, _ := mergeRepo(t, autoRefused(cleanWords))
	code, stdout, stderr, _ := call(t, f, "--ready")
	head := f.head(t)
	direct := "pr merge slice-9 --repo owner/name --rebase --match-head-commit " + head
	want := []string{list, ready, graphql(7), merged(head)[0], direct, readBack}
	if code != 0 || stderr != "" || !reflect.DeepEqual(f.calls(), want) ||
		!strings.HasSuffix(stdout, "Merged "+open7+" by rebase at "+short(head)+": its checks and reviews had passed, and GitHub merged it at once.\n") {
		t.Errorf("exit %d\n%s%s\n%s", code, stdout, stderr, strings.Join(f.calls(), "\n"))
	}

	f, _ = mergeRepo(t, autoRefused(cleanWords))
	_, stdout, _, _ = call(t, f, "--json", "--ready")
	if r := reported(t, stdout); r.PR == nil || r.PR.Merged == nil || !*r.PR.Merged || r.PR.AutoMerge || r.Outcome != "pushed" {
		t.Errorf("--json: %s", stdout)
	}

	// The read-back after the direct merge says what GitHub did.
	f, p := mergeRepo(t, autoRefused(cleanWords))
	p.view = `{"state":"OPEN","mergedAt":null,"autoMergeRequest":{"mergeMethod":"REBASE"}}`
	_, stdout, _, _ = call(t, f, "--json", "--ready")
	if r := reported(t, stdout); r.PR == nil || r.PR.Merged == nil || *r.PR.Merged || !r.PR.AutoMerge {
		t.Errorf("auto-merge on after a direct merge: %s", stdout)
	}

	// A second refusal is pull-request-failed, naming the direct merge, after
	// the pull request was read for a merge at the head judged.
	f, p = mergeRepo(t, func(args []string) error {
		if strings.Contains(strings.Join(args, " "), "--auto") {
			return refused(cleanWords)
		}
		return refused("HTTP 405")
	})
	p.view = `{"state":"OPEN","mergedAt":null,"headRefOid":"@head"}`
	code, _, stderr, _ = call(t, f, "--ready")
	read := "pr view slice-9 --repo owner/name --json state,mergedAt,headRefOid"
	if calls := f.calls(); code != missing || !strings.Contains(stderr, "gh pr merge failed: HTTP 405") || calls[len(calls)-1] != read || len(calls) != 6 {
		t.Errorf("a second refusal: exit %d\n%s\n%s", code, stderr, strings.Join(calls, "\n"))
	}

	// Any other refusal of the --auto call merges nothing itself.
	f, _ = mergeRepo(t, autoRefused("GraphQL: Pull request Auto merge is not allowed for this repository (enablePullRequestAutoMerge)"))
	code, _, stderr, _ = call(t, f, "--ready")
	if calls := f.calls(); code != missing || !strings.Contains(stderr, "Auto merge is not allowed for this repository") || len(calls) != 5 ||
		strings.Count(strings.Join(calls, "\n"), "pr merge") != 1 || calls[4] != read {
		t.Errorf("another refusal: exit %d\n%s\n%s", code, stderr, strings.Join(calls, "\n"))
	}
	// Under --json its words go to stderr, the one object being stdout's.
	f, _ = mergeRepo(t, autoRefused("HTTP 405"))
	code, stdout, stderr, _ = call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != missing || r.Outcome != "pull-request-failed" || strings.Count(stderr, "gh pr merge failed: HTTP 405") != 1 {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
	// A gh that did not run says no words, and is no clean status.
	f, _ = mergeRepo(t, func([]string) error { return errors.New("killed") })
	if code, _, stderr, _ = call(t, f, "--ready"); code != missing || !strings.Contains(stderr, "gh pr merge failed: killed") ||
		strings.Count(strings.Join(f.calls(), "\n"), "pr merge") != 1 {
		t.Errorf("a gh that did not run: exit %d\n%s", code, stderr)
	}
}

// A merge call that fails because an auto-merge turned on earlier merged the
// pull request just before is no failure when the pull request is merged at the
// head judged, by its state or its mergedAt: it says so, exit 0, merged in
// --json. A pull request not merged, merged at another head, or that cannot be
// read leaves the failure as it was.
func TestPushReadyWhereTheMergeCallFailsAfterTheMerge(t *testing.T) {
	const mergedWords = "GraphQL: Pull Request is not mergeable"
	for name, c := range map[string]struct {
		view   string // with %s the head judged
		fail   bool   // whether gh pr view fails
		merged bool
	}{
		"merged":                   {view: `{"state":"MERGED","mergedAt":"2026-10-27T10:00:00Z","headRefOid":"@head"}`, merged: true},
		"merged by its state":      {view: `{"state":"MERGED","headRefOid":"@head"}`, merged: true},
		"merged by its mergedAt":   {view: `{"state":"CLOSED","mergedAt":"2026-10-27T10:00:00Z","headRefOid":"@head"}`, merged: true},
		"merged at another head":   {view: `{"state":"MERGED","mergedAt":"2026-10-27T10:00:00Z","headRefOid":"` + otherSHA + `"}`},
		"open":                     {view: `{"state":"OPEN","mergedAt":null,"headRefOid":"@head"}`},
		"an empty mergedAt":        {view: `{"state":"OPEN","mergedAt":"","headRefOid":"@head"}`},
		"no state at all":          {view: `{"headRefOid":"@head"}`},
		"unreadable":               {view: `nope`},
		"gh pr view that fails":    {fail: true},
		"no head, merged":          {view: `{"state":"MERGED"}`},
		"the head, and no merge":   {view: `{"mergedAt":null,"headRefOid":"@head"}`},
		"the head of a merge only": {view: `{"mergedAt":"2026-10-27T10:00:00Z","headRefOid":"@head"}`, merged: true},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := mergeRepo(t, autoRefused(mergedWords))
			p.view = c.view
			if c.fail {
				p.fail = map[string]error{"pr view": refused("HTTP 502")}
			}
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			r := reported(t, stdout)
			if c.merged {
				if code != 0 || !r.OK || r.Outcome != "pushed" || r.PR == nil || r.PR.Merged == nil || !*r.PR.Merged || r.PR.AutoMerge {
					t.Errorf("exit %d\n%s%s", code, stdout, stderr)
				}
				f, p := mergeRepo(t, autoRefused(mergedWords))
				p.view = c.view
				code, stdout, _, _ = call(t, f, "--ready")
				if want := "Merged " + open7 + " by rebase at " + short(f.head(t)) + ": GitHub had merged it already.\n"; code != 0 || !strings.HasSuffix(stdout, want) {
					t.Errorf("plain: exit %d\n%s", code, stdout)
				}
				return
			}
			if code != missing || r.OK || r.Outcome != "pull-request-failed" || r.PR == nil || r.PR.Merged != nil && *r.PR.Merged ||
				!strings.Contains(stderr, "gh pr merge failed: "+mergedWords) {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
}

// After the merge call that succeeded the pull request is read once: merged
// (state MERGED, or a mergedAt) or auto-merge on (an autoMergeRequest) are
// success, merged first; neither, or an answer that is no pull request, or a gh
// pr view that fails, is pull-request-failed. An answer without state reads as
// the call asked.
func TestPushReadyReadsTheMergeBack(t *testing.T) {
	const auto = `"autoMergeRequest":{"mergeMethod":"REBASE"}`
	for name, c := range map[string]struct {
		view   string
		merged bool
		auto   bool
		says   string
	}{
		"no state, as asked":       {view: ``, auto: true},
		"no state, an object":      {view: `{}`, auto: true},
		"merged":                   {view: `{"state":"MERGED","mergedAt":"2026-10-27T10:00:00Z","autoMergeRequest":null}`, merged: true},
		"merged by its state":      {view: `{"state":"MERGED","mergedAt":null}`, merged: true},
		"merged by its mergedAt":   {view: `{"state":"OPEN","mergedAt":"2026-10-27T10:00:00Z"}`, merged: true},
		"merged, auto-merge on":    {view: `{"state":"MERGED","mergedAt":"2026-10-27T10:00:00Z",` + auto + `}`, merged: true},
		"auto-merge on":            {view: `{"state":"OPEN","mergedAt":null,` + auto + `}`, auto: true},
		"auto-merge on, no merge":  {view: `{"state":"OPEN",` + auto + `}`, auto: true},
		"neither":                  {view: `{"state":"OPEN","mergedAt":null,"autoMergeRequest":null}`, says: "gh pr view failed: the pull request is neither merged nor set to merge"},
		"neither, no request":      {view: `{"state":"OPEN"}`, says: "gh pr view failed: the pull request is neither merged nor set to merge"},
		"neither, an empty merge":  {view: `{"state":"OPEN","mergedAt":""}`, says: "gh pr view failed: the pull request is neither merged nor set to merge"},
		"not a pull request":       {view: `nope`, says: "gh pr view failed: its answer is not a pull request's merge state: "},
		"a gh pr view that fails":  {view: `fail`, says: "gh pr view failed: HTTP 502"},
		"state without a mergedAt": {view: `{"state":"CLOSED"}`, says: "the pull request is neither merged nor set to merge"},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := mergeRepo(t, func([]string) error { return nil })
			p.view = c.view
			if c.view == "fail" {
				p.view, p.fail = "", map[string]error{"pr view": refused("HTTP 502")}
			}
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			r := reported(t, stdout)
			if c.says != "" {
				if code != missing || r.Outcome != "pull-request-failed" || r.PR == nil || r.PR.AutoMerge || !strings.Contains(stderr, c.says) {
					t.Errorf("exit %d\n%s%s", code, stdout, stderr)
				}
				return
			}
			if code != 0 || r.PR == nil || r.PR.Merged == nil || *r.PR.Merged != c.merged || r.PR.AutoMerge != (c.auto && !c.merged) {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
}

// push.pr.ready_wait 0 is no bound: a run is watched to its end and the pull
// request merged.
func TestPushReadyWithNoBoundWatchesTheRun(t *testing.T) {
	f, p := draftRepo(t, "draft: true, ready_wait: 0", github)
	asked := watching(f, "success")
	p.open, p.draft = open7, true
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != 0 || r.PR == nil || !r.PR.AutoMerge || r.CI != "success" || !reflect.DeepEqual(*asked, []string{f.head(t)}) {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
}

// codexFor is Codex's review-state comment of the login whose reviews have the
// statuses and the commits.
func codexFor(login, code, codeSHA, security, securitySHA string) string {
	return node(login, codexStateAt(code, codeSHA, security, securitySHA))
}

// A review-state comment counts for the head alone, both its reviews of it: the
// verdict needs both completed and of the head, running means a review of the
// head not completed, and a state for another commit is neither, asked for the
// head unless one of its reviews that names the head is under way.
func TestReviewStatesAreOfTheHead(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	for name, c := range map[string]struct {
		comment                                 string
		completed, running, elsewhere, underway bool
	}{
		"both of the head, completed":        {comment: codexStateAt("completed", head, "completed", head), completed: true},
		"both of the head, code running":     {comment: codexStateAt("running", head, "completed", head), running: true, underway: true},
		"both of the head, security running": {comment: codexStateAt("completed", head, "running", head), running: true, underway: true},
		"both of another commit":             {comment: codexStateAt("completed", otherSHA, "completed", otherSHA), completed: true, elsewhere: true},
		"code of the head, security other":   {comment: codexStateAt("completed", head, "completed", otherSHA), completed: true, elsewhere: true},
		"security of the head, code other":   {comment: codexStateAt("completed", otherSHA, "completed", head), completed: true, elsewhere: true},
		"code of the head running":           {comment: codexStateAt("running", head, "completed", otherSHA), underway: true},
		"security of the head running":       {comment: codexStateAt("completed", otherSHA, "running", head), underway: true},
		"another commit running":             {comment: codexStateAt("running", otherSHA, "running", otherSHA), elsewhere: true},
		"no commit":                          {comment: "<!-- codex-review-state:v1 {\"code_review\":{\"status\":\"completed\"},\"security_review\":{\"status\":\"completed\"}} -->", completed: true, elsewhere: true},
	} {
		t.Run(name, func(t *testing.T) {
			s := pullState{pull: providers.PullState{Comments: []providers.Authored{{Login: "a", Body: c.comment}}}}
			st := s.pull.StateOf("a")
			if !st.Stated || st.Completed() != c.completed || s.running("a", head) != c.running || s.elsewhere("a", head) != c.elsewhere || underway(st, head) != c.underway {
				t.Errorf("completed %t, running %t, elsewhere %t, underway %t", st.Completed(), s.running("a", head), s.elsewhere("a", head), underway(st, head))
			}
			missing, _ := s.pull.Verdicts([]string{"a"}, head)
			if got := len(missing) == 0; got != (c.completed && !c.elsewhere) {
				t.Errorf("a verdict: %t", got)
			}
		})
	}
	// No comment, no state; its edits are the comment's updatedAt.
	s := pullState{pull: providers.PullState{Comments: []providers.Authored{{Login: "a", Body: "other"}}}}
	if st := s.pull.StateOf("a"); st.Stated || s.running("a", head) || s.elsewhere("a", head) {
		t.Errorf("a state from no state comment: %+v", st)
	}
	edited := time.Unix(1_234, 0)
	s = pullState{pull: providers.PullState{Comments: []providers.Authored{{Login: "a", Body: codexStateAt("running", head, "running", head), Updated: edited}}}}
	if st := s.pull.StateOf("a"); !st.Updated.Equal(edited) {
		t.Errorf("updated: %s", st.Updated)
	}
}

// askingFor is the pull request of a --ready whose review_by login a has the
// answer the function gives, the other, b, a review; the clock is the
// wait's.
func askingFor(t *testing.T, config string, answer func(ask int, c *clock, head string, commented bool) string) (*fake, *pulls, *clock) {
	t.Helper()
	f, p := draftRepo(t, "draft: true, review_by: [a, b]"+config, "ci: { watch: { interval: 10, timeout: 100 } }\n")
	p.open, p.draft = open7, true
	c := clocked(t)
	p.graph = func(ask int) string { return answer(ask, c, f.head(t), len(comments(f)) > 0) }
	return f, p, c
}

// A login whose review-state comment is for another commit is asked once for the
// head, at the look that reads it, whether or not push.pr.review_stall is set,
// and then waited for to ci.watch.timeout. It is not asked when a comment with
// its request for the head is on the pull request, when this --ready asked
// already, when it has no review-state comment, or when one of its reviews that
// names the head is under way.
func TestPushReadyAsksForTheHeadOnce(t *testing.T) {
	stateFor := func(sha string) string { return codexFor("a", "completed", sha, "completed", sha) }
	reviewOfB := node("b", "")
	for name, c := range map[string]struct {
		config   string
		answer   func(ask int, c *clock, head string, commented bool) string
		comments int
		outcome  string
	}{
		"another commit": {"", func(_ int, _ *clock, _ string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{stateFor(otherSHA)})
		}, 1, "review-timeout"},
		"with review_stall set": {", review_stall: 30", func(_ int, _ *clock, _ string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{stateFor(otherSHA)})
		}, 1, "review-timeout"},
		"asked on the pull request": {"", func(_ int, _ *clock, head string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{askedBy("someone", "a", head), stateFor(otherSHA)})
		}, 0, "review-timeout"},
		"asked, and listed once posted": {"", func(_ int, _ *clock, head string, commented bool) string {
			if commented {
				return answerOf([]string{reviewOfB}, []string{askedBy("me", "a", head), stateFor(otherSHA)})
			}
			return answerOf([]string{reviewOfB}, []string{stateFor(otherSHA)})
		}, 1, "review-timeout"},
		"asked for another head": {"", func(_ int, _ *clock, _ string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{askedBy("someone", "a", otherSHA), stateFor(otherSHA)})
		}, 1, "review-timeout"},
		"no state comment": {"", func(_ int, _ *clock, _ string, _ bool) string { return answerOf([]string{reviewOfB}, nil) }, 0, "review-timeout"},
		"a review of the head under way": {"", func(_ int, _ *clock, head string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{codexFor("a", "running", head, "completed", otherSHA)})
		}, 0, "review-timeout"},
		"a state for the head": {"", func(_ int, _ *clock, head string, _ bool) string {
			return answerOf([]string{reviewOfB}, []string{stateFor(head)})
		}, 0, "pushed"},
	} {
		t.Run(name, func(t *testing.T) {
			f, p, _ := askingFor(t, c.config, c.answer)
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			r := reported(t, stdout)
			if r.Outcome != c.outcome || len(comments(f)) != c.comments {
				t.Fatalf("exit %d, %s, %d comments, %d asks\n%s%s", code, r.Outcome, len(comments(f)), p.asks, stdout, stderr)
			}
			if c.comments == 0 {
				return
			}
			head := f.head(t)
			body := "pr comment slice-9 --repo owner/name --body=@codex review\n\n<!-- itos-review-request:v1 a " + head + " -->"
			asked := "Asked a to review " + short(head) + " on " + open7 + ": its review state is for another commit.\n"
			calls := f.calls()
			if !reflect.DeepEqual(comments(f), []string{body}) || strings.Count(stderr, "Asked ") != 1 || !strings.Contains(stderr, asked) ||
				!reflect.DeepEqual(calls[:5], []string{list, ready, graphql(7), activity, body}) {
				t.Errorf("the request: %v\n%s", calls, stderr)
			}
		})
	}
}

// The new head's request is posted at the look that reads the old state even
// when that look is past ci.watch.timeout (the pull request was marked ready
// long ago), then the wait ends review-timeout.
func TestPushReadyAsksForTheHeadBeforeJudgingTheTimeout(t *testing.T) {
	f, p, c := askingFor(t, "", func(_ int, c *clock, _ string, _ bool) string {
		return answerAt(nil, []string{codexFor("a", "completed", otherSHA, "completed", otherSHA)}, c.ago(200))
	})
	code, stdout, _, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-timeout" || len(comments(f)) != 1 || p.asks != 1 || len(c.slept) != 0 {
		t.Errorf("exit %d, %d comments, %d asks, slept %v\n%s", code, len(comments(f)), p.asks, c.slept, stdout)
	}
}

// Once asked for the head, a review that then reads running for it is stalled
// when it has read so review_stall seconds after the request, as usual.
func TestPushReadyStallsAfterTheRequestForTheHead(t *testing.T) {
	f, p, _ := askingFor(t, ", review_stall: 30", func(ask int, _ *clock, head string, _ bool) string {
		if ask < 4 {
			return answerOf([]string{node("b", "")}, []string{codexFor("a", "completed", otherSHA, "completed", otherSHA)})
		}
		return answerOf([]string{node("b", "")}, []string{codexFor("a", "running", head, "completed", head)})
	})
	code, stdout, _, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-stalled" || len(comments(f)) != 1 || p.asks != 4 {
		t.Errorf("exit %d, %d comments, %d asks\n%s", code, len(comments(f)), p.asks, stdout)
	}
}

// pagedAnswer is the pull request's answer whose comments are one page, with
// the cursor of the next, none when empty.
func pagedAnswer(reviews, comments []string, next string) string {
	pages := `"pageInfo":{"hasNextPage":false,"endCursor":"end"},`
	if next != "" {
		pages = fmt.Sprintf(`"pageInfo":{"hasNextPage":true,"endCursor":%q},`, next)
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[%s]},"comments":{%s"nodes":[%s]},"reviewThreads":{"nodes":[]}}}}}`,
		strings.Join(reviews, ","), pages, strings.Join(comments, ","))
}

// The comments are read to the last page, each asked for after the cursor of the
// one before: a request for the head that sits past the first hundred is seen,
// and posted again by no run. The reviews and the threads are the first page's,
// a page that has no next stops it, and so does one whose next has no cursor,
// and so, at the 20th, does one that always has.
func TestPushReadyReadsEveryPageOfComments(t *testing.T) {
	stateFor := codexFor("a", "completed", otherSHA, "completed", otherSHA)
	f, p, _ := askingFor(t, "", func(int, *clock, string, bool) string { return "" })
	p.graph = func(ask int) string {
		if ask%2 == 1 {
			return pagedAnswer([]string{node("b", "")}, []string{stateFor}, "c1")
		}
		return pagedAnswer(nil, []string{askedBy("someone", "a", "@head")}, "")
	}
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	calls := f.calls()
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-timeout" || len(comments(f)) != 0 || p.asks != 22 ||
		strings.Contains(calls[2], "cursor=") || !strings.HasSuffix(calls[3], " -f cursor=c1") {
		t.Errorf("exit %d, %s, %d comments, %d asks\n%s%s\n%s", code, r.Outcome, len(comments(f)), p.asks, stdout, stderr, strings.Join(calls[:5], "\n"))
	}

	// A next page with no cursor, or none to follow, is not asked for.
	for name, answer := range map[string]string{
		"no cursor": `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[]},"comments":{"pageInfo":{"hasNextPage":true,"endCursor":""},"nodes":[]},"reviewThreads":{"nodes":[]}}}}}`,
		"no page":   answerOf(nil, nil),
	} {
		f, p, _ = askingFor(t, "", func(int, *clock, string, bool) string { return answer })
		if call(t, f, "--ready"); p.asks != 11 {
			t.Errorf("%s: %d asks, want one a look", name, p.asks)
		}
	}

	// A pull request with no end to its pages is read 20 pages deep at a look (two looks here).
	f, p = draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { interval: 10, timeout: 1 } }\n")
	p.open, p.draft = open7, true
	p.graph = func(ask int) string { return pagedAnswer(nil, nil, fmt.Sprintf("c%d", ask)) }
	clocked(t)
	if code, _, _, _ := call(t, f, "--ready"); code != temporary || p.asks != 40 {
		t.Errorf("no end: exit %d, %d asks, want 20 a look", code, p.asks)
	}
}

// A request that cannot be posted ends the wait as any gh call that fails.
func TestPushReadyWhereTheRequestForTheHeadFails(t *testing.T) {
	f, p, _ := askingFor(t, "", func(int, *clock, string, bool) string {
		return answerOf([]string{node("b", "")}, []string{codexFor("a", "completed", otherSHA, "completed", otherSHA)})
	})
	p.fail = map[string]error{"pr comment": refused("HTTP 502")}
	code, _, stderr, _ := call(t, f, "--ready")
	if code != missing || !strings.Contains(stderr, "gh pr comment failed: HTTP 502") || p.asks != 1 || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d, asks %d\n%s", code, p.asks, stderr)
	}
}

// The review wait counts from when GitHub says the pull request was last marked
// ready or its head pushed, whichever is later, both read at the first ask, so a
// rerun reaches ci.watch.timeout: a clock it does not give counts from the first
// ask, whether it is not asked for, fails, is empty, is of another head or
// cannot be read.
func TestPushReadyCountsTheReviewWaitFromGitHubsTimes(t *testing.T) {
	for name, c := range map[string]struct {
		ready, activity func(c *clock, head string) string
		asks            int
	}{
		"nothing to go by":      {nil, nil, 7},
		"marked ready long ago": {func(c *clock, _ string) string { return c.ago(130) }, nil, 1},
		"pushed long ago": {nil, func(c *clock, head string) string {
			return `[{"after":"` + head + `","timestamp":"` + c.ago(120) + `"}]`
		}, 1},
		"both long ago": {func(c *clock, _ string) string { return c.ago(130) }, func(c *clock, head string) string {
			return `[{"after":"` + head + `","timestamp":"` + c.ago(120) + `"}]`
		}, 1},
		"the ready time later": {func(c *clock, _ string) string { return c.ago(10) }, func(c *clock, head string) string {
			return `[{"after":"` + head + `","timestamp":"` + c.ago(130) + `"}]`
		}, 6},
		"the push later": {func(c *clock, _ string) string { return c.ago(130) }, func(c *clock, head string) string {
			return `[{"after":"` + head + `","timestamp":"` + c.ago(10) + `"}]`
		}, 6},
		"pushed a moment ago": {nil, func(c *clock, head string) string { return `[{"after":"` + head + `","timestamp":"` + c.ago(0) + `"}]` }, 7},
		"another head's push": {nil, func(c *clock, _ string) string {
			return `[{"after":"` + otherSHA + `","timestamp":"` + c.ago(120) + `"}]`
		}, 7},
		"no activity":             {nil, func(*clock, string) string { return `[]` }, 7},
		"an activity unreadable":  {nil, func(*clock, string) string { return `nope` }, 7},
		"a timestamp unreadable":  {nil, func(_ *clock, head string) string { return `[{"after":"` + head + `","timestamp":"yesterday"}]` }, 7},
		"a ready time unreadable": {func(*clock, string) string { return "yesterday" }, nil, 7},
		"the first entry decides": {nil, func(c *clock, head string) string {
			return `[{"after":"` + head + `","timestamp":"` + c.ago(0) + `"},{"after":"` + head + `","timestamp":"` + c.ago(500) + `"}]`
		}, 7},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { interval: 10, timeout: 60 } }\n")
			p.open, p.draft = open7, true
			clk := clocked(t)
			const head = "@head"
			ready := ""
			if c.ready != nil {
				ready = c.ready(clk, head)
			}
			if c.activity != nil {
				p.activity = c.activity(clk, head)
			}
			p.graph = func(int) string { return answerAt(nil, nil, ready) }
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			if r := reported(t, stdout); code != temporary || r.Outcome != "review-timeout" || p.asks != c.asks {
				t.Errorf("exit %d, %s, asks %d, want %d\n%s%s", code, r.Outcome, p.asks, c.asks, stdout, stderr)
			}
			if got := f.calls()[3]; got != activity {
				t.Errorf("the branch's activity was asked as %q", got)
			}
		})
	}
	// An activity that cannot be got is no failure.
	f, p := draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { interval: 10, timeout: 60 } }\n")
	p.open, p.draft, p.fail = open7, true, map[string]error{"api repos": refused("HTTP 404")}
	clocked(t)
	if code, _, stderr, _ := call(t, f, "--ready"); code != temporary || !strings.Contains(stderr, "no review by a on") {
		t.Errorf("an activity that fails: exit %d\n%s", code, stderr)
	}
}

// The stall counts from GitHub's times where it gives them: before a request
// from the last edit of the review-state comment, after one from the request's
// time; ask is the look the request was posted at, 0 for none.
func TestPushReadyCountsTheStallFromGitHubsTimes(t *testing.T) {
	running := func(c *clock, head string, edited int) string {
		return timedNode("a", codexStateAt("running", head, "completed", head), c.ago(edited), c.ago(edited))
	}
	request := func(c *clock, head string, posted int) string {
		return timedNode("someone", "@codex review\n\n<!-- itos-review-request:v1 a "+head+" -->", c.ago(posted), c.ago(posted))
	}
	for name, k := range map[string]struct {
		answer  func(c *clock, head string) []string
		outcome string
		asks    int
		ask     int
	}{
		// Last edited 50 seconds ago, review_stall 60: asked at the second look
		// (10 seconds in), and stalled 60 seconds after the request.
		"asked from the last edit": {func(c *clock, head string) []string { return []string{running(c, head, 50)} }, "review-stalled", 8, 2},
		// No time: asked at the seventh look, and the timeout comes first.
		"asked from the first look": {func(c *clock, head string) []string {
			return []string{timedNode("a", codexStateAt("running", head, "completed", head), "", "")}
		}, "review-timeout", 11, 7},
		"asked long ago":       {func(c *clock, head string) []string { return []string{running(c, head, 30), request(c, head, 120)} }, "review-stalled", 1, 0},
		"asked 20 seconds ago": {func(c *clock, head string) []string { return []string{running(c, head, 30), request(c, head, 20)} }, "review-stalled", 5, 0},
		"asked for another head": {func(c *clock, head string) []string {
			return []string{running(c, head, 30), request(c, otherSHA, 120)}
		}, "review-stalled", 10, 4},
	} {
		t.Run(name, func(t *testing.T) {
			f, p, _ := askingFor(t, ", review_stall: 60", func(_ int, c *clock, head string, _ bool) string {
				return answerOf([]string{node("b", "")}, k.answer(c, head))
			})
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			asked, graphs := 0, 0
			for _, call := range f.calls() {
				if strings.HasPrefix(call, "api graphql") {
					graphs++
				}
				if strings.HasPrefix(call, "pr comment") {
					asked = graphs
				}
			}
			if r := reported(t, stdout); code != temporary || r.Outcome != k.outcome || p.asks != k.asks || asked != k.ask {
				t.Errorf("exit %d, %s, asks %d, asked at %d; want %s, %d, %d\n%s%s", code, r.Outcome, p.asks, asked, k.outcome, k.asks, k.ask, stdout, stderr)
			}
		})
	}
}
