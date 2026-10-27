package push

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
)

// A review thread as GraphQL gives it, with its node id and the login of its
// first comment's author ("" for an author that is gone).
func threadBy(id string, resolved bool, path, line, login string, comments ...string) string {
	author := "null"
	if login != "" {
		author = fmt.Sprintf(`{"login":%q}`, login)
	}
	var nodes []string
	for _, body := range comments {
		nodes = append(nodes, fmt.Sprintf(`{"author":%s,"body":%q}`, author, body))
	}
	return fmt.Sprintf(`{"id":%q,"isResolved":%t,"path":%q,"line":%s,"comments":{"nodes":[%s]}}`, id, resolved, path, line, strings.Join(nodes, ","))
}

// threadsRepo is an item's branch under push.mode pr whose pull request,
// number 7, has these review threads.
func threadsRepo(t *testing.T) (*fake, *pulls) {
	t.Helper()
	f := prRepo(t)
	p := &pulls{open: open7, created: opened, threads: []string{
		threadBy("T1", false, "widget.md", "3", "alice", "Rename this.\nIt clashes.", "second comment"),
		threadBy("T2", true, "gadget.md", "1", "alice", "Fine now."),
		threadBy("T3", false, "old.md", "null", "bob", "  Outdated one.  "),
		threadBy("T4", false, "gone.md", "5", "", "Written by a ghost."),
	}}
	f.gh = p.gh
	return f, p
}

const (
	reply   = "api graphql -f query=" + replyMutation + " -f threadId=T1 -f body=Renamed it."
	resolve = "api graphql -f query=" + resolveMutation + " -f threadId=T1"
)

// reportedThreads is --json's object as a map.
func reportedThreads(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%s: %q", err, stdout)
	}
	return got
}

// push threads says the open threads in GitHub's order, each with its id, its
// place (no line for an outdated one), its first comment's author and the
// first line of that comment, and asks the pull request and its threads and
// nothing else: nothing is fetched, rebased or pushed.
func TestPushThreadsListsTheOpenThreads(t *testing.T) {
	f, _ := threadsRepo(t)
	code, stdout, stderr, err := call(t, f, "threads")
	want := open7 + " has 3 unresolved review threads:\n" +
		"  T1 widget.md:3 alice: Rename this.\n" +
		"  T3 old.md bob: Outdated one.\n" +
		"  T4 gone.md:5 : Written by a ghost.\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if got, want := f.calls(), []string{list, graphql(7)}; !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
	if len(f.pushed) != 0 || len(f.quiet) != 0 {
		t.Errorf("pushed %v, fetched %v", f.pushed, f.quiet)
	}
	if code, stdout, _, _ := call(t, f, "-q", "threads"); code != 0 || stdout != "" {
		t.Errorf("-q: exit %d, %q", code, stdout)
	}

	f, _ = threadsRepo(t)
	code, stdout, stderr, _ = call(t, f, "--json", "threads")
	var wantJSON map[string]any
	json.Unmarshal([]byte(`{"schema":1,"ok":true,"outcome":"threads-listed","pull_request":{"url":"`+open7+`","number":7},"threads":[
		{"id":"T1","path":"widget.md","line":3,"author":"alice","comment":"Rename this."},
		{"id":"T3","path":"old.md","author":"bob","comment":"Outdated one."},
		{"id":"T4","path":"gone.md","line":5,"author":"","comment":"Written by a ghost."}]}`), &wantJSON)
	if got := reportedThreads(t, stdout); code != 0 || !reflect.DeepEqual(got, wantJSON) || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
}

// With every thread resolved it says none is open, and --json gives an empty
// list, not none.
func TestPushThreadsWithNoneOpen(t *testing.T) {
	f, p := threadsRepo(t)
	p.threads = []string{threadBy("T2", true, "gadget.md", "1", "alice", "Fine now.")}
	code, stdout, _, _ := call(t, f, "threads")
	if code != 0 || stdout != open7+" has no unresolved review threads.\n" {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	_, stdout, _, _ = call(t, f, "--json", "threads")
	if got, ok := reportedThreads(t, stdout)["threads"].([]any); !ok || len(got) != 0 {
		t.Errorf("--json: %s", stdout)
	}
}

// push thread posts the reply, then resolves, as two calls in that order after
// the two that ask; --reply alone leaves the thread open and --resolve alone
// posts no reply; the id of a resolved thread is a thread of the pull request
// too.
func TestPushThreadAnswersTheThread(t *testing.T) {
	for name, c := range map[string]struct {
		args  []string
		calls []string
		says  string
		json  string
	}{
		"both": {[]string{"thread", "T1", "--reply", "Renamed it.", "--resolve"}, []string{reply, resolve},
			"Replied to T1 on " + open7 + ".\nResolved T1 on " + open7 + ".\n", `{"id":"T1","replied":true,"resolved":true}`},
		"reply": {[]string{"thread", "T1", "--reply=Renamed it."}, []string{reply},
			"Replied to T1 on " + open7 + ".\n", `{"id":"T1","replied":true,"resolved":false}`},
		"resolve": {[]string{"thread", "--resolve", "T1"}, []string{resolve},
			"Resolved T1 on " + open7 + ".\n", `{"id":"T1","replied":false,"resolved":true}`},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := threadsRepo(t)
			code, stdout, stderr, err := call(t, f, c.args...)
			if code != 0 || err != nil || stdout != c.says || stderr != "" {
				t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
			}
			if got, want := f.calls(), append([]string{list, graphql(7)}, c.calls...); !reflect.DeepEqual(got, want) {
				t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
			}
			f, _ = threadsRepo(t)
			code, stdout, stderr, _ = call(t, f, append([]string{"--json"}, c.args...)...)
			var want map[string]any
			json.Unmarshal([]byte(`{"schema":1,"ok":true,"outcome":"thread-answered","pull_request":{"url":"`+open7+`","number":7},"thread":`+c.json+`}`), &want)
			if got := reportedThreads(t, stdout); code != 0 || !reflect.DeepEqual(got, want) || stderr != "" {
				t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}

	f, _ := threadsRepo(t)
	if code, stdout, _, _ := call(t, f, "thread", "T2", "--resolve"); code != 0 || stdout != "Resolved T2 on "+open7+".\n" {
		t.Errorf("a resolved thread: exit %d\n%s", code, stdout)
	}
}

// An id that is none of the pull request's threads is refused, exit 1, before
// anything is posted.
func TestPushThreadRefusesAnIdThatIsNoThreadOfThePullRequest(t *testing.T) {
	f, _ := threadsRepo(t)
	code, stdout, stderr, _ := call(t, f, "thread", "T9", "--reply", "x", "--resolve")
	if code != policy || stdout != "" || stderr != "itos push: T9 is not a review thread of "+open7+"; itos push threads lists its open ones.\n" {
		t.Errorf("exit %d\n%s%s", code, stdout, stderr)
	}
	if got, want := f.calls(), []string{list, graphql(7)}; !reflect.DeepEqual(got, want) {
		t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
	}
	f, _ = threadsRepo(t)
	code, stdout, stderr, _ = call(t, f, "--json", "thread", "T9", "--resolve")
	got := reportedThreads(t, stdout)
	if _, thread := got["thread"]; code != policy || got["outcome"] != "no-such-thread" || got["ok"] != false || got["pull_request"] == nil || thread ||
		!strings.Contains(stderr, "T9 is not a review thread") {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
}

// Both commands refuse, exit 1 and before gh is asked, where they have nothing
// to do, and say nothing was posted; no repository to name, and no open pull
// request, are told once gh is asked.
func TestPushThreadsRefusalsAskGhNothing(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*fake)
		code    int
		outcome string
		says    string
		calls   int
	}{
		"trunk": {func(f *fake) { f.cfg = loaded(t, "version: 1\n") }, policy, "threads-without-pr-mode",
			"itos push: review threads are an item's pull request's, and this config does not set push.mode: pr; nothing was posted.\n", 0},
		"no config": {func(f *fake) { f.found = false }, policy, "threads-without-pr-mode", "push.mode: pr", 0},
		"detached":  {func(f *fake) { f.OnBranch = "" }, policy, "detached", "itos push: HEAD is not on a branch; check out the item's branch first. Nothing was posted.\n", 0},
		"main": {func(f *fake) { f.on("main", mine) }, policy, "threads-on-main",
			"itos push: HEAD is on main, which has no pull request; run it from the item's branch. Nothing was posted.\n", 0},
		"no repository": {func(f *fake) { f.repository = "" }, missing, "no-github-repository",
			"itos push: origin names no GitHub repository to find the pull request of slice-9 in; nothing was posted.\n", 0},
		"no pull request": {func(f *fake) { f.gh = answers(opened, "") }, policy, "no-pull-request",
			"itos push: slice-9 has no open pull request; itos push opens it. Nothing was posted.\n", 1},
	} {
		for _, args := range [][]string{{"threads"}, {"thread", "T1", "--reply", "x", "--resolve"}} {
			t.Run(name+" "+args[0], func(t *testing.T) {
				f, _ := threadsRepo(t)
				c.set(f)
				code, stdout, stderr, _ := call(t, f, args...)
				if code != c.code || stdout != "" || !strings.Contains(stderr, c.says) || len(f.ghCalls) != c.calls {
					t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
				}
				code, stdout, stderr, _ = call(t, f, append([]string{"--json"}, args...)...)
				got := reportedThreads(t, stdout)
				if code != c.code || got["outcome"] != c.outcome || got["ok"] != false || !strings.Contains(stderr, c.says) {
					t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
				}
				for _, key := range []string{"remote", "branch", "pull_request", "threads", "thread"} {
					if _, has := got[key]; has {
						t.Errorf("--json has %q: %s", key, stdout)
					}
				}
			})
		}
	}
}

// A gh that fails, or an answer that is not what was asked for, exits 3 naming
// the call and gh's words, with no word of a push; a reply that fails posts
// no resolve, and a resolve that fails after the reply says the reply was
// posted.
func TestPushThreadsWhereGhFails(t *testing.T) {
	// failing is p's gh, failing the calls that have the word in them.
	failing := func(p *pulls, word, words string) func([]string) (string, error) {
		return func(args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), word) {
				return "", &providers.GhFailed{Words: words + "\n", Err: errors.New("exit status 1")}
			}
			return p.gh(args)
		}
	}
	for name, c := range map[string]struct {
		word, words string
		args        []string
		says        string
		calls       int
	}{
		"pr list":     {"pr list", "HTTP 401: Bad credentials", []string{"threads"}, "itos push: gh pr list failed: HTTP 401: Bad credentials\n", 1},
		"threads ask": {"query=", "HTTP 502", []string{"thread", "T1", "--resolve"}, "itos push: gh api graphql failed: HTTP 502\n", 2},
	} {
		t.Run(name, func(t *testing.T) {
			f, p := threadsRepo(t)
			f.gh = failing(p, c.word, c.words)
			code, stdout, stderr, _ := call(t, f, c.args...)
			if code != missing || stdout != "" || stderr != c.says || len(f.ghCalls) != c.calls || strings.Contains(stderr, "pushed") {
				t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
			}
		})
	}

	t.Run("no gh", func(t *testing.T) {
		f, _ := threadsRepo(t)
		f.gh = func([]string) (string, error) { return "", providers.ErrNoGh }
		code, _, stderr, _ := call(t, f, "threads")
		if code != missing || stderr != "itos push: gh pr list failed: gh is not on the PATH\n" {
			t.Errorf("exit %d\n%s", code, stderr)
		}
	})
	t.Run("not a list", func(t *testing.T) {
		f, _ := threadsRepo(t)
		f.gh = func([]string) (string, error) { return "no", nil }
		code, _, stderr, _ := call(t, f, "threads")
		if code != missing || !strings.HasPrefix(stderr, "itos push: gh pr list failed: its answer is not a list of pull requests") {
			t.Errorf("exit %d\n%s", code, stderr)
		}
	})
	t.Run("the reply fails", func(t *testing.T) {
		f, p := threadsRepo(t)
		f.gh = failing(p, "addPullRequestReviewThreadReply", "HTTP 403")
		code, stdout, stderr, _ := call(t, f, "--json", "thread", "T1", "--reply", "Renamed it.", "--resolve")
		got := reportedThreads(t, stdout)
		if code != missing || got["outcome"] != "pull-request-failed" || got["ok"] != false ||
			!reflect.DeepEqual(got["thread"], map[string]any{"id": "T1", "replied": false, "resolved": false}) ||
			stderr != "itos push: gh api graphql failed: HTTP 403\n" {
			t.Errorf("exit %d\n%s%s", code, stdout, stderr)
		}
		if got := f.calls(); len(got) != 3 || got[2] != reply {
			t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
		}
	})
	t.Run("the resolve fails after the reply", func(t *testing.T) {
		f, p := threadsRepo(t)
		f.gh = failing(p, "resolveReviewThread", "HTTP 403")
		code, stdout, stderr, _ := call(t, f, "--json", "thread", "T1", "--reply", "Renamed it.", "--resolve")
		got := reportedThreads(t, stdout)
		if code != missing || !reflect.DeepEqual(got["thread"], map[string]any{"id": "T1", "replied": true, "resolved": false}) ||
			stderr != "Replied to T1 on "+open7+".\nitos push: gh api graphql failed: HTTP 403\n" {
			t.Errorf("exit %d\n%s%s", code, stdout, stderr)
		}
	})
}

// push threads takes no argument and push thread an id with --reply <text>,
// --resolve or both; every other form is a usage error, whatever else is
// given.
func TestParseThreads(t *testing.T) {
	for args, want := range map[string]Query{
		"threads":                             {Threads: true},
		"thread T1 --resolve":                 {Thread: "T1", Resolve: true},
		"thread --resolve T1":                 {Thread: "T1", Resolve: true},
		"thread T1 --reply=text":              {Thread: "T1", Reply: "text"},
		"thread T1 --reply=a=b --resolve":     {Thread: "T1", Reply: "a=b", Resolve: true},
		"thread --reply -dash T1":             {Thread: "T1", Reply: "-dash"},
		"thread T1 --resolve --reply --other": {Thread: "T1", Reply: "--other", Resolve: true},
	} {
		c, err := Parse(strings.Fields(args), false, false, "")
		if err != nil || !reflect.DeepEqual(c.Query, want) {
			t.Errorf("Parse(%s) = %+v, %v; want %+v", args, c.Query, err, want)
		}
	}
	for args, want := range map[string]string{
		"threads T1":                     "push threads takes no arguments (T1)",
		"threads --ready":                "push threads takes no arguments (--ready)",
		"thread":                         "push thread needs a thread's id",
		"thread --resolve":               "push thread needs a thread's id",
		"thread T1":                      "push thread T1 needs --reply <text>, --resolve or both",
		"thread T1 --reply":              "push thread --reply needs the reply's text",
		"thread T1 --reply=":             "push thread --reply needs the reply's text",
		"thread T1 --resolve --reply":    "push thread --reply needs the reply's text",
		"thread T1 T2 --resolve":         "push thread answers one thread, and T2 is a second: T1",
		"thread T1 --resolve --tag v1":   "push thread takes a thread's id, --reply <text> and --resolve, not --tag",
		"thread T1 --resolve=yes":        "push thread takes a thread's id, --reply <text> and --resolve, not --resolve=yes",
		"--no-wait threads":              "push takes no arguments but --no-wait",
		"widgets":                        "push takes no arguments but --no-wait",
		"--reply text":                   "push takes no arguments but --no-wait",
		"--resolve":                      "push takes no arguments but --no-wait",
		"--ready thread T1 --resolve":    "push takes no arguments but --no-wait",
		"--tag v1 threads":               "push takes no arguments but --no-wait",
		"thread T1 --reply x --ref main": "push thread takes a thread's id, --reply <text> and --resolve, not --ref",
	} {
		_, err := Parse(strings.Fields(args), false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want %q", args, err, want)
		}
	}
}
