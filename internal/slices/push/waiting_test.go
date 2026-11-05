package push

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/engine/reviewers"
	"github.com/donvargax/itos/v8/internal/providers"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// rebasedCommit is the head the host makes when it rebases the pull request
// onto main: rb000, above origin's main.
func rebasedCommit(f *fake) vcsfake.Commit {
	return vcsfake.Commit{Name: "rb000", Parents: []string{up}, Message: "test: more\n", Tree: maps.Clone(f.History[1].Tree)}
}

// node is a review or a comment as GraphQL gives it.
func node(login, body string) string {
	return fmt.Sprintf(`{"author":{"login":%q},"body":%q}`, login, body)
}

// answerOf is the pull request's answer with the review and comment nodes.
func answerOf(reviews, comments []string) string {
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[%s]},"comments":{"nodes":[%s]},"reviewThreads":{"nodes":[]}}}}}`,
		strings.Join(reviews, ","), strings.Join(comments, ","))
}

// runningBy and doneBy are the review-state comments of the login: its code
// review still running, and both reviews completed.
func runningBy(login string) string { return node(login, codexState("running", "completed")) }
func doneBy(login string) string    { return node(login, codexState("completed", "completed")) }

// askedBy is a comment asking the login to review the head again, as itos
// writes it.
func askedBy(author, login, head string) string {
	return node(author, "@codex review\n\n<!-- itos-review-request:v1 "+login+" "+head+" -->")
}

// askedForSecurity is the comment the security review's request is read from: the
// marker alone.
func askedForSecurity(author, login, head string) string {
	return node(author, "<!-- itos-review-request:v1 "+login+" "+head+" -->")
}

// waitingLines are the lines of the output that say what --ready waits on,
// without the address and the head they all begin with.
func waitingLines(output, address, head string) []string {
	var lines []string
	for _, l := range strings.Split(output, "\n") {
		if rest, ok := strings.CutPrefix(l, "Waiting on "+address+" at "+short(head)+": "); ok {
			lines = append(lines, rest)
		}
	}
	return lines
}

// comments are the pr comment calls of the run.
func comments(f *fake) []string {
	var calls []string
	for _, c := range f.calls() {
		if strings.HasPrefix(c, "pr comment ") {
			calls = append(calls, c)
		}
	}
	return calls
}

// --ready says what it waits on when the wait starts and after each look that
// changes a part, in the config's order, and nothing while nothing changes: the
// states are read before the run is watched, and the run's success comes with
// the first ask's; a login with neither a verdict nor a state reads waiting, one
// with a state not completed running, one with a verdict completed.
func TestPushReadySaysWhatItWaitsOn(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a, b]", "ci: { watch: { provider: github, interval: 2, timeout: 30 } }\n")
	watching(f, "success")
	p.open, p.draft = open7, true
	p.graph = func(ask int) string {
		switch {
		case ask < 4:
			return answerOf(nil, []string{runningBy("b")})
		case ask == 4:
			return answerOf([]string{node("a", "")}, []string{runningBy("b")})
		}
		return answerOf([]string{node("a", "")}, []string{doneBy("b")})
	}
	clocked(t)
	code, stdout, stderr, _ := call(t, f, "--ready")
	head := f.head(t)
	want := []string{
		"run waiting, a waiting, b running.",
		"run succeeded, a waiting, b running.",
		"run succeeded, a completed, b running.",
		"run succeeded, a completed, b completed.",
	}
	if got := waitingLines(stdout, open7, head); code != 0 || stderr != "" || !reflect.DeepEqual(got, want) || p.asks != 5 {
		t.Errorf("exit %d, asks %d\n%s%s\nwant %q", code, p.asks, stdout, stderr, want)
	}
	// The first line is before the run is watched, the next after the reviews are asked.
	if run, first := strings.Index(stdout, "CI run:"), strings.Index(stdout, "run waiting"); first < 0 || first > run {
		t.Errorf("the first line is not before the run is watched:\n%s", stdout)
	}

	// Where the watch's progress goes: stderr under --json, nowhere under -q.
	for _, c := range []struct {
		arg            string
		stdout, stderr bool
	}{{"--json", false, true}, {"-q", false, false}} {
		f, p := draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { provider: github, interval: 2, timeout: 30 } }\n")
		watching(f, "success")
		p.open, p.draft, p.reviews = open7, true, []string{"a"}
		clocked(t)
		code, stdout, stderr, _ := call(t, f, c.arg, "--ready")
		if code != 0 || (len(waitingLines(stdout, open7, head)) > 0) != c.stdout || (len(waitingLines(stderr, open7, head)) > 0) != c.stderr {
			t.Errorf("%s: exit %d\n%s%s", c.arg, code, stdout, stderr)
		}
	}

	// A login whose reviews are all refusals reads waiting.
	f, p = draftRepo(t, "draft: true, review_by: [a]")
	p.open, p.draft = open7, true
	p.graph = func(int) string { return answerOf([]string{node("a", quota)}, nil) }
	clocked(t)
	code, stdout, _, _ = call(t, f, "--ready")
	if got := waitingLines(stdout, open7, f.head(t)); code != temporary || !reflect.DeepEqual(got, []string{"a waiting."}) {
		t.Errorf("refused: exit %d\n%s", code, stdout)
	}
}

// With no run to watch there is no first look: the first ask comes before the
// first line, and no question more.
func TestPushReadySaysWhatItWaitsOnWithNoRun(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a]")
	p.open, p.draft = open7, true
	p.graph = func(ask int) string {
		if ask < 2 {
			return answerOf(nil, nil)
		}
		return answerOf([]string{node("a", "")}, nil)
	}
	clocked(t)
	code, stdout, _, _ := call(t, f, "--ready")
	want := []string{"a waiting.", "a completed."}
	if got := waitingLines(stdout, open7, f.head(t)); code != 0 || !reflect.DeepEqual(got, want) || p.asks != 2 {
		t.Errorf("exit %d, asks %d\n%s\nwant %q", code, p.asks, stdout, want)
	}

	// With no run and no login there is nothing to wait on.
	f, p = draftRepo(t, "draft: true")
	p.open, p.draft = open7, true
	if _, stdout, _, _ := call(t, f, "--ready"); strings.Contains(stdout, "Waiting on") {
		t.Errorf("waiting on nothing:\n%s", stdout)
	}
}

// The line names the head GitHub made when it rebased the pull request.
func TestPushReadySaysWhatItWaitsOnAtTheRebasedHead(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { provider: github, interval: 2, timeout: 5 } }\n")
	watching(f, "success")
	f.History = append(f.History, rebasedCommit(f))
	p.open, p.draft, p.heads, p.reviews = open7, true, []string{"rb000"}, []string{"a"}
	f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
	f.on(item, "nw000")
	clocked(t)
	code, stdout, _, _ := call(t, f, "--ready")
	if got := waitingLines(stdout, open7, "rb000"); code != 0 || !reflect.DeepEqual(got, []string{"run waiting, a completed.", "run succeeded, a completed."}) {
		t.Errorf("exit %d\n%s", code, stdout)
	}

	// The first look is at that head: a request for it reads asked, one for the head
	// pushed does not.
	for request, want := range map[string]string{"rb000": "run waiting, a asked.", "nw000": "run waiting, a waiting."} {
		f, p := draftRepo(t, "draft: true, review_by: [a]", "ci: { watch: { provider: github, interval: 2, timeout: 5 } }\n")
		watching(f, "success")
		f.History = append(f.History, rebasedCommit(f))
		p.open, p.draft, p.heads = open7, true, []string{"rb000"}
		p.graph = func(ask int) string {
			if ask == 1 {
				return answerOf(nil, []string{askedBy("someone", "a", request)})
			}
			return answerOf([]string{node("a", "")}, nil)
		}
		f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
		f.on(item, "nw000")
		clocked(t)
		_, stdout, _, _ := call(t, f, "--ready")
		if got := waitingLines(stdout, open7, "rb000"); len(got) == 0 || got[0] != want {
			t.Errorf("a request for %s: %q, want %q first\n%s", request, got, want, stdout)
		}
	}
}

// The table of a login's states in the line (bug-166), the first that holds, read
// at the first look while the run goes and again once it is over: completed; running;
// asked, for either review, for a request naming the login and the head; no review
// requested, with no state and no request, for a login push.pr.reviewers names; else
// waiting. after is the state --ready ends on where it differs: it asks for a review
// a state of another commit leaves unasked, which then reads asked.
func TestWaitingLineStates(t *testing.T) {
	nobody := "no review requested (itos review request " + item + ")"
	cursor := func(login, head string) string {
		return node("someone", "@cursor review\n\n<!-- itos-review-request:v1 "+login+" "+head+" -->")
	}
	elsewhere := func(login string) string { return codexFor(login, "completed", otherSHA, "completed", otherSHA) }
	const named = ", reviewers: [{ login: a, kind: codex }, { login: cur, kind: bugbot }, { login: \"bot[bot]\", kind: codex }]"
	for name, c := range map[string]struct {
		login    string
		reviews  []string
		comments []string
		want     string
		after    string
	}{
		"a review":                                {login: "a", reviews: []string{node("a", "")}, want: "completed"},
		"a review beside a request":               {login: "a", reviews: []string{node("a", "")}, comments: []string{askedBy("x", "a", "@head")}, want: "completed"},
		"a state completed":                       {login: "a", comments: []string{doneBy("a")}, want: "completed"},
		"a state completed and a request":         {login: "a", comments: []string{askedBy("x", "a", "@head"), doneBy("a")}, want: "completed"},
		"a state running":                         {login: "a", comments: []string{runningBy("a")}, want: "running"},
		"a state running and a request":           {login: "a", comments: []string{askedBy("x", "a", "@head"), runningBy("a")}, want: "running"},
		"the code request":                        {login: "a", comments: []string{askedBy("x", "a", "@head")}, want: "asked"},
		"the security request":                    {login: "a", comments: []string{askedForSecurity("x", "a", "@head")}, want: "asked"},
		"a request naming the bot":                {login: "a", comments: []string{askedBy("x", "a[bot]", "@head")}, want: "asked"},
		"a state of another commit and a request": {login: "a", comments: []string{elsewhere("a"), askedBy("x", "a", "@head")}, want: "asked"},
		"a review not started and its request":    {login: "a", comments: []string{notStartedBy("a"), askedForSecurity("x", "a", "@head")}, want: "asked"},
		"a state of another commit":               {login: "a", comments: []string{elsewhere("a")}, want: "waiting", after: "asked"},
		"a review not started":                    {login: "a", comments: []string{notStartedBy("a")}, want: "waiting", after: "asked"},
		"nothing":                                 {login: "a", want: nobody},
		"a request of another head":               {login: "a", comments: []string{askedBy("x", "a", otherSHA)}, want: nobody},
		"a request of another login":              {login: "a", comments: []string{askedBy("x", "b", "@head")}, want: nobody},
		"a request's line with no marker":         {login: "a", comments: []string{node("x", "@codex review")}, want: nobody},
		"named with [bot], written without":       {login: "bot", want: nobody},
		"named without, written with [bot]":       {login: "a[bot]", want: nobody},
		"not named, nothing":                      {login: "c", want: "waiting"},
		"not named, the code request":             {login: "c", comments: []string{askedBy("x", "c", "@head")}, want: "asked"},
		"not named, the security request":         {login: "c", comments: []string{askedForSecurity("x", "c", "@head")}, want: "asked"},
		"not named, a state of another commit":    {login: "c", comments: []string{elsewhere("c")}, want: "waiting", after: "asked"},
		"bugbot, its request":                     {login: "cur", comments: []string{cursor("cur", "@head")}, want: "asked"},
		"bugbot, a request not of its kind":       {login: "cur", comments: []string{askedBy("x", "cur", "@head")}, want: nobody},
		"bugbot, nothing":                         {login: "cur", want: nobody},
		"bugbot, a state of another commit":       {login: "cur", comments: []string{elsewhere("cur")}, want: "waiting", after: "asked"},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, fmt.Sprintf("draft: true, review_by: [%q], ready_wait: 1%s", c.login, named), github)
			watching(f, "success")
			p.open, p.draft = open7, true
			p.graph = func(int) string { return answerOf(c.reviews, c.comments) }
			clocked(t)
			_, stdout, stderr, _ := call(t, f, "--ready")
			after := c.after
			if after == "" {
				after = c.want
			}
			want := []string{"run waiting, " + c.login + " " + c.want + ".", "run succeeded, " + c.login + " " + after + "."}
			got := waitingLines(stdout, open7, f.head(t))
			if len(got) < 2 || got[0] != want[0] || got[len(got)-1] != want[1] {
				t.Errorf("%q, want %q first and last\n%s%s", got, want, stdout, stderr)
			}
		})
	}
}

// The first look judges nothing and costs one question: its failing changes the word
// alone, to not read, the next ask reading the states, or judging the failure as it
// did. A stop before that ask gives the word.
func TestWaitingLineWhereTheFirstLookFails(t *testing.T) {
	for name, firstLook := range map[string]func() (string, error){
		"gh fails":            func() (string, error) { return "", refused("HTTP 502") },
		"an answer not of it": func() (string, error) { return "{}", nil },
	} {
		t.Run(name, func(t *testing.T) {
			f, p := draftRepo(t, "draft: true, review_by: [a]", github)
			watching(f, "success")
			p.open, p.draft, p.reviews = open7, true, []string{"a"}
			pass, looked := f.gh, false
			f.gh = func(args []string) (string, error) {
				if strings.Join(args[:2], " ") == "api graphql" && !looked {
					looked = true
					return firstLook()
				}
				return pass(args)
			}
			code, stdout, stderr, _ := call(t, f, "--ready")
			want := []string{"run waiting, a not read.", "run succeeded, a completed."}
			if got := waitingLines(stdout, open7, f.head(t)); code != 0 || !reflect.DeepEqual(got, want) || p.asks != 1 || strings.Contains(stderr, "failed") {
				t.Errorf("exit %d, asks %d, %q\n%s%s", code, p.asks, got, stdout, stderr)
			}
		})
	}

	f, p := draftRepo(t, "draft: true, review_by: [a]", github)
	watching(f, "success")
	p.open, p.draft = open7, true
	p.fail = map[string]error{"api graphql": refused("HTTP 502")}
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != missing || r.Outcome != "pull-request-failed" || !strings.Contains(stderr, ": run waiting, a not read.") ||
		strings.Count(stderr, "gh api graphql failed: HTTP 502") != 1 {
		t.Errorf("failing still: exit %d, %s\n%s", code, r.Outcome, stderr)
	}
}

// The stop before a draft is marked reads the states too, with the one question its
// wait asks, for it alone; with no login it asks none.
func TestWaitingLineAtTheStopBeforeTheMarking(t *testing.T) {
	stopped := func(t *testing.T, pr string) (*fake, *pulls) {
		t.Helper()
		f, p := draftRepo(t, pr, "ci: { watch: { provider: github, interval: 10, timeout: 100 } }\n")
		f.watcher, f.watchOK = providers.Watcher{HeadRuns: func(string) ([]providers.Run, error) {
			return []providers.Run{{URL: "https://ci/g", Status: "in_progress", Event: "pull_request"}}, nil
		}}, true
		p.open, p.draft = open7, true
		clocked(t)
		return f, p
	}
	for name, c := range map[string]struct {
		graph func(int) string
		fail  bool
		want  string
	}{
		"a request":      {graph: func(int) string { return answerOf(nil, []string{askedBy("x", "a", "@head")}) }, want: "asked"},
		"nothing":        {graph: func(int) string { return answerOf(nil, nil) }, want: "waiting"},
		"a look failing": {fail: true, want: "not read"},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := stopped(t, "draft: true, review_by: [a], ready_wait: 25")
			p.graph = c.graph
			if c.fail {
				p.fail = map[string]error{"api graphql": refused("HTTP 502")}
			}
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			want := ": run waiting, a " + c.want + "; auto-merge is not turned on."
			if r := reported(t, stdout); code != temporary || r.Outcome != "ready-still-waiting" || !strings.Contains(stderr, want) ||
				strings.Contains(strings.Join(f.calls(), "\n"), "pr ready") {
				t.Errorf("exit %d, %s, want %q\n%s", code, r.Outcome, want, stderr)
			}
			if !c.fail && p.asks != 1 {
				t.Errorf("%d asks, want 1", p.asks)
			}
		})
	}

	f, p := stopped(t, "draft: true, ready_wait: 25")
	if code, _, stderr, _ := call(t, f, "--json", "--ready"); code != temporary || p.asks != 0 || !strings.Contains(stderr, ": run waiting; auto-merge") {
		t.Errorf("no login: exit %d, asks %d\n%s", code, p.asks, stderr)
	}
}

// A request --ready has just posted reads asked at once: at the line after it, and at
// a stop in the same look.
func TestWaitingLineSaysAskedAtTheLookThatPosts(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a], ready_wait: 1")
	p.open, p.draft = open7, true
	c := clocked(t)
	p.graph = func(int) string {
		c.at = c.at.Add(2 * time.Second)
		return answerOf(nil, []string{codexFor("a", "completed", otherSHA, "completed", otherSHA)})
	}
	code, stdout, stderr, _ := call(t, f, "--ready")
	if got, want := waitingLines(stdout, open7, f.head(t)), []string{"a waiting.", "a asked."}; code != temporary || !reflect.DeepEqual(got, want) ||
		p.asks != 1 || !strings.Contains(stderr, ": a asked; auto-merge is not turned on.") {
		t.Errorf("exit %d, asks %d, %q\n%s%s", code, p.asks, got, stdout, stderr)
	}
}

// stalling is a pull request whose reviews by a are asked each 10 seconds, the
// answer the function gives for the ask and whether a pr comment call has been
// made, review_stall seconds and ci.watch.timeout 100.
func stalling(t *testing.T, config string, answer func(ask int, commented bool, head string) string) (*fake, *pulls, *clock) {
	t.Helper()
	pr := "draft: true, review_by: [a, b]"
	if config != "" {
		pr += ", " + config
	}
	f, p := draftRepo(t, pr, "ci: { watch: { interval: 10, timeout: 100 } }\n")
	p.open, p.draft = open7, true
	p.graph = func(ask int) string {
		return answer(ask, len(comments(f)) > 0, f.head(t))
	}
	return f, p, clocked(t)
}

// A review that reads running for push.pr.review_stall seconds, counted from
// the first look that read it so, is asked for again with one comment, "@codex
// review" and the marker line with the login and the head; one that still
// reads running as long after the request ends the wait, 75, review-stalled,
// having asked once and turned on no auto-merge. The asks are at 0, 10, ...
// seconds: the request at the fourth, the stall at the seventh.
func TestPushReadyAsksAStalledReviewAgainOnce(t *testing.T) {
	f, p, c := stalling(t, "review_stall: 30", func(int, bool, string) string {
		return answerOf([]string{node("b", "")}, []string{runningBy("a")})
	})
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	r := reported(t, stdout)
	head := f.head(t)
	body := "@codex review\n\n<!-- itos-review-request:v1 a " + head + " -->"
	asked := "Asked a to review " + short(head) + " again on " + open7 + ": its review read running for 30 seconds (push.pr.review_stall).\n"
	stalled := "itos: the review by a of " + short(head) + " on " + open7 + " stalled: still running 30 seconds (push.pr.review_stall) " +
		"after it was asked again, so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n"
	if code != temporary || r.OK || r.Outcome != "review-stalled" || r.PR == nil || r.PR.AutoMerge || p.asks != 7 ||
		!reflect.DeepEqual(comments(f), []string{"pr comment slice-9 --repo owner/name --body=" + body}) ||
		!strings.Contains(stderr, asked) || !strings.HasSuffix(stderr, stalled) || strings.Count(stderr, "Asked ") != 1 ||
		strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") || len(c.slept) != 6 {
		t.Errorf("exit %d, asks %d, slept %v\n%s%s", code, p.asks, c.slept, stdout, stderr)
	}
	// The request is before the ask that follows it and after the fourth.
	calls, graphs := f.calls(), 0
	for _, call := range calls {
		if strings.HasPrefix(call, "pr comment") {
			break
		}
		if strings.HasPrefix(call, "api graphql") {
			graphs++
		}
	}
	if graphs != 4 {
		t.Errorf("the request came after %d asks, not 4", graphs)
	}

	// Plain: the request is said where the watch speaks, the stall on stderr.
	f, _, _ = stalling(t, "review_stall: 30", func(int, bool, string) string {
		return answerOf([]string{node("b", "")}, []string{runningBy("a")})
	})
	code, stdout, stderr, _ = call(t, f, "--ready")
	if code != temporary || !strings.Contains(stdout, asked) || strings.Contains(stderr, "Asked") || !strings.HasSuffix(stderr, stalled) {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
}

// The request is not posted, and the stall is not judged, where push.pr.review_stall
// is left out, 0 or below, the login has no state comment, or its review
// completed in time; a request for another head, or login, counts for nothing,
// one for this head and login, by anyone, is asked already and not posted again.
func TestPushReadyStallCases(t *testing.T) {
	const other = "ffffffffffffffffffffffffffffffffffffffff"
	for name, c := range map[string]struct {
		config   string
		answer   func(ask int, commented bool, head string) string
		code     int
		outcome  string
		comments int
		asks     int
	}{
		"left out": {"", func(int, bool, string) string { return answerOf([]string{node("b", "")}, []string{runningBy("a")}) }, temporary, "review-timeout", 0, 11},
		"zero": {"review_stall: 0", func(int, bool, string) string { return answerOf([]string{node("b", "")}, []string{runningBy("a")}) },
			temporary, "review-timeout", 0, 11},
		"negative": {"review_stall: -30", func(int, bool, string) string { return answerOf([]string{node("b", "")}, []string{runningBy("a")}) },
			temporary, "review-timeout", 0, 11},
		"no state comment": {"review_stall: 30", func(int, bool, string) string { return answerOf([]string{node("b", "")}, []string{doneBy("c")}) },
			temporary, "review-timeout", 0, 11},
		"completed in time": {"review_stall: 30", func(ask int, _ bool, _ string) string {
			if ask < 3 {
				return answerOf([]string{node("b", "")}, []string{runningBy("a")})
			}
			return answerOf([]string{node("b", "")}, []string{doneBy("a")})
		}, 0, "pushed", 0, 3},
		"asked already": {"review_stall: 30", func(_ int, _ bool, head string) string {
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a", head), runningBy("a")})
		}, temporary, "review-stalled", 0, 4},
		"asked already, as a bot": {"review_stall: 30", func(_ int, _ bool, head string) string {
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a[bot]", head), runningBy("a")})
		}, temporary, "review-stalled", 0, 4},
		"another head": {"review_stall: 30", func(int, bool, string) string {
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a", other), runningBy("a")})
		}, temporary, "review-stalled", 1, 7},
		"another login": {"review_stall: 30", func(_ int, _ bool, head string) string {
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "ab", head), runningBy("a")})
		}, temporary, "review-stalled", 1, 7},
		// A security request is not the stall's: the code review is still asked
		// for, and its clock starts from the running review, not from that request.
		"a security request": {"review_stall: 30", func(_ int, _ bool, head string) string {
			return answerOf([]string{node("b", "")}, []string{askedForSecurity("someone", "a", head), runningBy("a")})
		}, temporary, "review-stalled", 1, 7},
		// What GitHub shows after the request counts from the request, not from
		// the first look.
		"our request seen": {"review_stall: 30", func(_ int, commented bool, head string) string {
			if commented {
				return answerOf([]string{node("b", "")}, []string{askedBy("me", "a", head), runningBy("a")})
			}
			return answerOf([]string{node("b", "")}, []string{runningBy("a")})
		}, temporary, "review-stalled", 1, 7},
		// Reading anything but running starts the count again: running at the
		// first two asks, not at the third, running from the fourth.
		"running again": {"review_stall: 30", func(ask int, _ bool, _ string) string {
			if ask == 3 {
				return answerOf([]string{node("b", "")}, nil)
			}
			return answerOf([]string{node("b", "")}, []string{runningBy("a")})
		}, temporary, "review-stalled", 1, 10},
	} {
		t.Run(name, func(t *testing.T) {
			f, p, _ := stalling(t, c.config, c.answer)
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			r := reported(t, stdout)
			if code != c.code || r.Outcome != c.outcome || len(comments(f)) != c.comments || p.asks != c.asks {
				t.Errorf("exit %d, %s, %d comments, %d asks; want %d, %s, %d, %d\n%s%s",
					code, r.Outcome, len(comments(f)), p.asks, c.code, c.outcome, c.comments, c.asks, stdout, stderr)
			}
			if c.code == 0 && (!r.PR.AutoMerge || !strings.Contains(strings.Join(f.calls(), "\n"), "pr merge")) {
				t.Errorf("auto-merge is not on:\n%s", strings.Join(f.calls(), "\n"))
			}
		})
	}
}

// Every running login past review_stall is asked in the config's order, and
// the stall names the first to be judged so.
func TestPushReadyAsksEachStalledLogin(t *testing.T) {
	f, p, _ := stalling(t, "review_stall: 30", func(int, bool, string) string {
		return answerOf(nil, []string{runningBy("b"), runningBy("a")})
	})
	code, _, stderr, _ := call(t, f, "--ready")
	head := f.head(t)
	want := []string{
		"pr comment slice-9 --repo owner/name --body=@codex review\n\n<!-- itos-review-request:v1 a " + head + " -->",
		"pr comment slice-9 --repo owner/name --body=@codex review\n\n<!-- itos-review-request:v1 b " + head + " -->",
	}
	if code != temporary || !reflect.DeepEqual(comments(f), want) || p.asks != 7 || !strings.Contains(stderr, "the review by a of ") {
		t.Errorf("exit %d, asks %d\n%s\n%s", code, p.asks, strings.Join(comments(f), "\n"), stderr)
	}
}

// A request for the head GitHub made when it rebased the pull request names
// that head.
func TestPushReadyAsksForTheRebasedHead(t *testing.T) {
	f, p := draftRepo(t, "draft: true, review_by: [a], review_stall: 1", "ci: { watch: { provider: github, interval: 2, timeout: 20 } }\n")
	watching(f, "success")
	f.History = append(f.History, rebasedCommit(f))
	p.open, p.draft, p.heads = open7, true, []string{"rb000"}
	p.graph = func(int) string { return answerOf(nil, []string{runningBy("a")}) }
	f.Remote["origin"].Refs["refs/heads/slice-9"] = mine
	f.on(item, "nw000")
	clocked(t)
	call(t, f, "--ready")
	want := []string{"pr comment slice-9 --repo owner/name --body=@codex review\n\n<!-- itos-review-request:v1 a rb000 -->"}
	if !reflect.DeepEqual(comments(f), want) {
		t.Errorf("%q", comments(f))
	}
}

// A gh pr comment that fails ends the wait as any gh call that fails does.
func TestPushReadyWhereTheRequestFails(t *testing.T) {
	f, p, _ := stalling(t, "review_stall: 30", func(int, bool, string) string {
		return answerOf(nil, []string{runningBy("a")})
	})
	p.fail = map[string]error{"pr comment": &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}}
	code, stdout, _, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != missing || r.Outcome != "pull-request-failed" || p.asks != 4 || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d, asks %d\n%s", code, p.asks, stdout)
	}
	if code, _, stderr, _ := call(t, f, "--ready"); code != missing || !strings.Contains(stderr, "gh pr comment failed: HTTP 502") {
		t.Errorf("exit %d\n%s", code, stderr)
	}
}

// ci.watch.timeout bounds the whole wait: with review_stall past it the
// timeout comes first.
func TestPushReadyStallPastTheTimeout(t *testing.T) {
	f, _, c := stalling(t, "review_stall: 100", func(int, bool, string) string {
		return answerOf(nil, []string{runningBy("a")})
	})
	code, stdout, _, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-timeout" || len(comments(f)) != 0 || c.at.Sub(time.Unix(1_000_000, 0)) != 100*time.Second {
		t.Errorf("exit %d, %s, %d comments\n%s", code, r.Outcome, len(comments(f)), stdout)
	}
}

// notStartedBy is the login's review-state comment with its code review completed
// for the head and its security review null of no commit, the case of bug-157.
func notStartedBy(login string) string { return codexFor(login, "completed", "@head", "", "") }

// A review not started is asked once more review_stall seconds after its one
// request and ends the wait review_stall seconds after that, each at the first look
// that has it: the looks are at 0, 10, ... seconds, so at the fourth and seventh.
func TestPushReadyAsksOnceMoreThenStops(t *testing.T) {
	f, p, c := stalling(t, "review_stall: 30", func(_ int, _ bool, head string) string {
		return answerOf([]string{node("b", "")}, []string{askedForSecurity("someone", "a", head), notStartedBy("a")})
	})
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	head := f.head(t)
	line := "pr comment slice-9 --repo owner/name --body=@codex security review"
	mark := "pr comment slice-9 --repo owner/name --body=<!-- itos-review-request:v1 a " + head + " -->"
	stop := "itos: the security review by a of " + head + " on " + open7 + " has not started 30 seconds (push.pr.review_stall) after the request " +
		", so auto-merge is not turned on; look at the reviewer, then itos push --ready again.\n"
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-not-started" || p.asks != 7 || len(c.slept) != 6 ||
		!reflect.DeepEqual(comments(f), []string{line, mark}) || !strings.HasSuffix(stderr, stop) {
		t.Errorf("exit %d, %s, asks %d, %q\n%s%s", code, r.Outcome, p.asks, comments(f), stdout, stderr)
	}
}

// A review's status reading anything but not started starts the first look's clock
// again, and a review with no request for the head is judged by nothing: the
// request of another head, or of the other review, is none.
func TestPushReadyNotStartedCases(t *testing.T) {
	for name, c := range map[string]struct {
		answer   func(ask int, commented bool, head string) string
		outcome  string
		comments int
		asks     int
	}{
		"a request of the other review only, of the head": {func(_ int, _ bool, head string) string {
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a", head), codexFor("a", "completed", head, "", head)})
		}, "review-timeout", 0, 11},
		"a request of another head": {func(int, bool, string) string {
			return answerOf([]string{node("b", "")}, []string{askedForSecurity("someone", "a", "ffff"), codexFor("a", "completed", "@head", "", "@head")})
		}, "review-timeout", 0, 11},
		// Running at the second and third looks, so the clock restarts at the fourth:
		// the second request comes three looks after it, at the seventh, not the fourth.
		"started, then null again": {func(ask int, _ bool, head string) string {
			state := notStartedBy("a")
			if ask == 2 || ask == 3 {
				state = codexFor("a", "completed", "@head", "running", "@head")
			}
			return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a", head), askedForSecurity("someone", "a", head), state})
		}, "review-not-started", 2, 10},
	} {
		t.Run(name, func(t *testing.T) {
			f, p, _ := stalling(t, "review_stall: 30", c.answer)
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			if r := reported(t, stdout); code != temporary || r.Outcome != c.outcome || len(comments(f)) != c.comments || p.asks != c.asks {
				t.Errorf("exit %d, %s, %d comments, %d asks\n%s%s", code, r.Outcome, len(comments(f)), p.asks, stdout, stderr)
			}
		})
	}
}

// The code review not started comes before a review under way: a running security
// review does not stop the wait as a stall while the code review's second request
// is due, and the stop names the code review.
func TestPushReadyJudgesTheCodeReviewBeforeTheStall(t *testing.T) {
	f, p, _ := stalling(t, "review_stall: 30", func(_ int, _ bool, head string) string {
		return answerOf([]string{node("b", "")}, []string{askedBy("someone", "a", head), codexFor("a", "", "@head", "running", "@head")})
	})
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-not-started" || p.asks != 7 || len(comments(f)) != 1 ||
		!strings.Contains(stderr, "the code review by a of ") {
		t.Errorf("exit %d, %s, asks %d, %q\n%s%s", code, r.Outcome, p.asks, comments(f), stdout, stderr)
	}
}

// ci.watch.timeout comes first: at a look where it has passed and two requests are
// past review_stall, the outcome is review-timeout, nothing asked.
func TestPushReadyNotStartedPastTheTimeout(t *testing.T) {
	f, p, c := stalling(t, "review_stall: 30", nil)
	p.graph = func(int) string {
		sha := f.head(t)
		request := "<!-- itos-review-request:v1 a " + sha + " -->"
		return answerAt([]string{node("b", "")}, []string{
			timedNode("someone", request, c.ago(190), c.ago(190)),
			timedNode("someone", request, c.ago(150), c.ago(150)),
			notStartedBy("a"),
		}, c.ago(200))
	}
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != temporary || r.Outcome != "review-timeout" || p.asks != 1 || len(comments(f)) != 0 {
		t.Errorf("exit %d, %s, asks %d, %q\n%s%s", code, r.Outcome, p.asks, comments(f), stdout, stderr)
	}
}

// gh pr comment failing on the second request is any gh call failing: exit 3,
// pull-request-failed, nothing merged.
func TestPushReadyWhereTheSecondRequestFails(t *testing.T) {
	f, p, _ := stalling(t, "review_stall: 30", func(_ int, _ bool, head string) string {
		return answerOf([]string{node("b", "")}, []string{askedForSecurity("someone", "a", head), notStartedBy("a")})
	})
	p.fail = map[string]error{"pr comment": &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}}
	code, stdout, stderr, _ := call(t, f, "--json", "--ready")
	if r := reported(t, stdout); code != missing || r.Outcome != "pull-request-failed" || p.asks != 4 ||
		!strings.Contains(stderr, "gh pr comment failed: HTTP 502") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
		t.Errorf("exit %d, %s, asks %d\n%s%s", code, r.Outcome, p.asks, stdout, stderr)
	}
}

// The marker's comment failing is the request not made, the line staying posted and
// no address given.
func TestPushReadyWhereTheSecurityMarkerFails(t *testing.T) {
	for name, c := range map[string]struct {
		answer func(head string) []string
		asks   int
	}{
		"the first request":  {func(string) []string { return []string{notStartedBy("a")} }, 1},
		"the second request": {func(head string) []string { return []string{askedForSecurity("someone", "a", head), notStartedBy("a")} }, 4},
	} {
		t.Run(name, func(t *testing.T) {
			f, p, _ := stalling(t, "review_stall: 30", func(_ int, _ bool, head string) string {
				return answerOf([]string{node("b", "")}, c.answer(head))
			})
			p.failBody = "--body=<!--"
			code, stdout, stderr, _ := call(t, f, "--json", "--ready")
			head := f.head(t)
			want := []string{
				"pr comment slice-9 --repo owner/name --body=@codex security review",
				"pr comment slice-9 --repo owner/name --body=<!-- itos-review-request:v1 a " + head + " -->",
			}
			if r := reported(t, stdout); code != missing || r.Outcome != "pull-request-failed" || p.asks != c.asks ||
				!reflect.DeepEqual(comments(f), want) || strings.Contains(stderr, "Asked ") ||
				!strings.Contains(stderr, "gh pr comment failed: HTTP 502") || strings.Contains(strings.Join(f.calls(), "\n"), "pr merge") {
				t.Errorf("exit %d, %s, asks %d, %q\n%s%s", code, r.Outcome, p.asks, comments(f), stdout, stderr)
			}
		})
	}
}

// A request posted counts until GitHub lists it, and then once, known by its
// address; one with no address cannot be known, and counts beside those listed.
func TestRequestsCountAPostedRequestOnce(t *testing.T) {
	const head = "0123456789abcdef0123456789abcdef01234567"
	req := reviewers.CodexSecurity("a")
	listedAt, postedAt := time.Unix(1_000, 0), time.Unix(1_001, 0)
	mark := req.Bodies(head)[1]
	listed := []providers.Authored{{Body: mark, URL: "u1", Created: listedAt}}
	for name, c := range map[string]struct {
		comments []providers.Authored
		posted   []requestMade
		want     []requestMade
	}{
		"not listed yet":    {nil, []requestMade{{"u1", postedAt}}, []requestMade{{"u1", postedAt}}},
		"listed":            {listed, []requestMade{{"u1", postedAt}}, []requestMade{{"u1", listedAt}}},
		"another's listed":  {listed, []requestMade{{"u2", postedAt}}, []requestMade{{"u1", listedAt}, {"u2", postedAt}}},
		"no address":        {[]providers.Authored{{Body: mark}}, []requestMade{{"", postedAt}}, []requestMade{{"", time.Time{}}, {"", postedAt}}},
		"two with one name": {nil, []requestMade{{"u1", listedAt}, {"u1", postedAt}}, []requestMade{{"u1", listedAt}, {"u1", postedAt}}},
	} {
		rv := newReviewing(1)
		rv.asked[req] = c.posted
		if got := rv.requests(req, c.comments, head); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
		}
	}
}

// The latest request is by its time, whatever the order given, and the one given
// last of those with the same time or with none.
func TestLatestRequest(t *testing.T) {
	first, later := time.Unix(1_000, 0), time.Unix(2_000, 0)
	for _, c := range []struct {
		made []requestMade
		want string
	}{
		{nil, ""},
		{[]requestMade{{"a", first}, {"b", later}}, "b"},
		{[]requestMade{{"a", later}, {"b", first}}, "a"},
		{[]requestMade{{"a", later}, {"b", later}}, "b"},
		{[]requestMade{{"a", time.Time{}}, {"b", time.Time{}}}, "b"},
		{[]requestMade{{"a", time.Time{}}, {"b", first}}, "b"},
	} {
		if got := latest(c.made).address; got != c.want {
			t.Errorf("%+v: %q, want %q", c.made, got, c.want)
		}
	}
}
