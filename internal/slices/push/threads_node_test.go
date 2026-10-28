package push

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
)

// says is a comment of a review thread: its author's login, "" for an author
// that is gone, and its body.
type says struct{ login, body string }

// nodeOf is a review thread's node as GraphQL gives it for the id: in the
// repository, on pull request 7, with the comments.
func nodeOf(id, repository string, resolved bool, path, line string, comments ...says) string {
	var nodes []string
	for _, c := range comments {
		author := "null"
		if c.login != "" {
			author = fmt.Sprintf(`{"login":%q}`, c.login)
		}
		nodes = append(nodes, fmt.Sprintf(`{"author":%s,"body":%q}`, author, c.body))
	}
	return fmt.Sprintf(`{"id":%q,"isResolved":%t,"path":%q,"line":%s,"repository":{"nameWithOwner":%q},`+
		`"pullRequest":{"number":7,"url":%q},"comments":{"nodes":[%s]}}`, id, resolved, path, line, repository, open7, strings.Join(nodes, ","))
}

// graph is a gh that answers the node question by the id it names, and a pull
// request's threads by its number, as GitHub does; any other call is the
// pulls'. An id with no node fails gh as GitHub's answer for one does.
type graph struct {
	*pulls
	nodes map[string]string // the node, by id
	pull  map[string]string // the pull request's answer, by number
}

// notFound is the body gh prints, and the failure it ends with, for an id
// GitHub has no node for.
const notFound = `{"data":{"node":null},"errors":[{"type":"NOT_FOUND","path":["node"],"message":"Could not resolve to a node"}]}`

func (g graph) gh(args []string) (string, error) {
	line := strings.Join(args, " ")
	if i := strings.LastIndex(line, " -f id="); i >= 0 {
		if node, ok := g.nodes[line[i+len(" -f id="):]]; ok {
			return `{"data":{"node":` + node + `}}` + "\n", nil
		}
		return notFound + "\n", &providers.GhFailed{Words: "gh: Could not resolve to a node\n", Err: errors.New("exit status 1")}
	}
	if len(args) > 3 && args[3] == "query="+pullThreadsQuery {
		if answer, ok := g.pull[strings.TrimPrefix(args[len(args)-1], "number=")]; ok {
			return answer + "\n", nil
		}
		return "", &providers.GhFailed{Words: "gh: Could not resolve to a PullRequest\n", Err: errors.New("exit status 1")}
	}
	return g.pulls.gh(args)
}

// nodeRepo is threadsRepo whose gh is a graph.
func nodeRepo(t *testing.T) (*fake, *graph) {
	t.Helper()
	f := prRepo(t)
	g := &graph{pulls: &pulls{open: open7, created: opened}, nodes: map[string]string{}, pull: map[string]string{}}
	f.gh = g.gh
	return f, g
}

func nodeCall(id string) string { return "api graphql -f query=" + nodeQuery + " -f id=" + id }

func pullCall(number int) string {
	return fmt.Sprintf("api graphql -f query=%s -f owner=owner -f name=name -F number=%d", pullThreadsQuery, number)
}

// pullAnswer is GitHub's answer about pull request 7 with the review threads.
func pullAnswer(threads ...string) string {
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"number":7,"url":%q,"reviewThreads":{"nodes":[%s]}}}}}`,
		open7, strings.Join(threads, ","))
}

// Wherever HEAD is, push thread asks for the node by its id and for no pull
// request: on the item's branch, on main and on a detached HEAD.
var heads = map[string]func(*fake){
	"item's branch": func(*fake) {},
	"main":          func(f *fake) { f.on("main", mine) },
	"detached":      func(f *fake) { f.OnBranch = "" },
}

// push thread with neither --reply nor --resolve shows the thread: where it is
// and whether it is resolved, then each comment, oldest first, its author and
// its body indented, a \r\n read as a \n, an author who is gone as ghost; and
// --json gives each body as GitHub does.
func TestPushThreadShowsTheThread(t *testing.T) {
	for name, set := range heads {
		t.Run(name, func(t *testing.T) {
			f, g := nodeRepo(t)
			g.nodes["T1"] = nodeOf("T1", "owner/name", false, "widget.md", "3",
				says{"alice", "Rename this.\nIt clashes."}, says{"", "Gone."}, says{"bob", "one\r\ntwo\r\n\r\nthree"})
			set(f)
			code, stdout, stderr, err := call(t, f, "thread", "T1")
			want := "T1 widget.md:3 on " + open7 + ", unresolved:\n" +
				"alice:\n  Rename this.\n  It clashes.\n" +
				"ghost:\n  Gone.\n" +
				"bob:\n  one\n  two\n  \n  three\n"
			if code != 0 || err != nil || stdout != want || stderr != "" {
				t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
			}
			if got, want := f.calls(), []string{nodeCall("T1")}; !reflect.DeepEqual(got, want) {
				t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
			}
			if len(f.pushed) != 0 || len(f.quiet) != 0 {
				t.Errorf("pushed %v, fetched %v", f.pushed, f.quiet)
			}
			if code, stdout, _, _ := call(t, f, "-q", "thread", "T1"); code != 0 || stdout != "" {
				t.Errorf("-q: exit %d, %q", code, stdout)
			}

			f, g = nodeRepo(t)
			g.nodes["T1"] = nodeOf("T1", "owner/name", false, "widget.md", "3",
				says{"alice", "Rename this."}, says{"", "Gone."}, says{"bob", "one\r\ntwo"})
			set(f)
			code, stdout, stderr, _ = call(t, f, "--json", "thread", "T1")
			wantJSON := `{"schema":1,"ok":true,"outcome":"thread-shown","pull_request":{"url":"` + open7 + `","number":7},"thread":` +
				`{"id":"T1","path":"widget.md","line":3,"resolved":false,"comments":[{"author":"alice","body":"Rename this."},` +
				`{"author":"ghost","body":"Gone."},{"author":"bob","body":"one\r\ntwo"}]}}`
			if code != 0 || stderr != "" || !jsonEqual(t, stdout, wantJSON) {
				t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
}

// jsonEqual is whether the two texts are the same JSON value.
func jsonEqual(t *testing.T, got, want string) bool {
	t.Helper()
	return reflect.DeepEqual(reportedThreads(t, got), reportedThreads(t, want))
}

// A resolved thread says so, an outdated one (no line) names only its file and
// leaves the line out of --json, and a thread with no comment says only where
// it is, its comments an empty list.
func TestPushThreadShowsAResolvedOutdatedThread(t *testing.T) {
	f, g := nodeRepo(t)
	g.nodes["T2"] = nodeOf("T2", "owner/name", true, "old.md", "null", says{"alice", "Fine now."})
	g.nodes["T3"] = nodeOf("T3", "owner/name", false, "empty.md", "2")
	code, stdout, _, _ := call(t, f, "thread", "T2")
	if want := "T2 old.md on " + open7 + ", resolved:\nalice:\n  Fine now.\n"; code != 0 || stdout != want {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	code, stdout, _, _ = call(t, f, "--json", "thread", "T2")
	want := `{"schema":1,"ok":true,"outcome":"thread-shown","pull_request":{"url":"` + open7 + `","number":7},"thread":` +
		`{"id":"T2","path":"old.md","resolved":true,"comments":[{"author":"alice","body":"Fine now."}]}}`
	if code != 0 || !jsonEqual(t, stdout, want) {
		t.Errorf("--json: exit %d\n%s", code, stdout)
	}
	code, stdout, _, _ = call(t, f, "--json", "thread", "T3")
	want = `{"schema":1,"ok":true,"outcome":"thread-shown","pull_request":{"url":"` + open7 + `","number":7},"thread":` +
		`{"id":"T3","path":"empty.md","line":2,"resolved":false,"comments":[]}}`
	if code != 0 || !jsonEqual(t, stdout, want) {
		t.Errorf("--json: exit %d\n%s", code, stdout)
	}
}

// push thread posts the reply, then resolves, as two calls in that order after
// the one that asks for the node; --reply alone leaves the thread open and
// --resolve alone posts no reply; a resolved thread is answered too, and so is
// one whose repository differs from push's only in case. The pull request is
// the node's, whatever branch HEAD is on.
func TestPushThreadAnswersTheThread(t *testing.T) {
	for name, set := range heads {
		for how, c := range map[string]struct {
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
			t.Run(name+" "+how, func(t *testing.T) {
				f, g := nodeRepo(t)
				g.nodes["T1"] = nodeOf("T1", "Owner/Name", true, "widget.md", "3", says{"alice", "Rename this."})
				set(f)
				code, stdout, stderr, err := call(t, f, c.args...)
				if code != 0 || err != nil || stdout != c.says || stderr != "" {
					t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
				}
				if got, want := f.calls(), append([]string{nodeCall("T1")}, c.calls...); !reflect.DeepEqual(got, want) {
					t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
				}
				f, g = nodeRepo(t)
				g.nodes["T1"] = nodeOf("T1", "owner/name", false, "widget.md", "3", says{"alice", "Rename this."})
				set(f)
				code, stdout, stderr, _ = call(t, f, append([]string{"--json"}, c.args...)...)
				want := `{"schema":1,"ok":true,"outcome":"thread-answered","pull_request":{"url":"` + open7 + `","number":7},"thread":` + c.json + `}`
				if code != 0 || stderr != "" || !jsonEqual(t, stdout, want) {
					t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
				}
			})
		}
	}
}

// An id GitHub has no node for, a node that is no review thread (null, or
// with no id) and a thread of another repository are refused, exit 1, naming
// the repository, with nothing posted and only the node asked for.
func TestPushThreadRefusesAnIdThatIsNoThreadOfTheRepository(t *testing.T) {
	for name, node := range map[string]string{
		"no node":        "",
		"null":           "null",
		"not a thread":   "{}",
		"other repo":     nodeOf("T9", "other/name", false, "widget.md", "3", says{"alice", "Rename this."}),
		"other owner":    nodeOf("T9", "other/name", false, "widget.md", "3"),
		"a longer name":  nodeOf("T9", "owner/names", false, "widget.md", "3"),
		"a shorter name": nodeOf("T9", "owner/nam", false, "widget.md", "3"),
	} {
		for _, args := range [][]string{{"thread", "T9", "--reply", "x", "--resolve"}, {"thread", "T9"}} {
			t.Run(name+" "+strings.Join(args[2:], ""), func(t *testing.T) {
				f, g := nodeRepo(t)
				if node != "" {
					g.nodes["T9"] = node
				}
				code, stdout, stderr, _ := call(t, f, args...)
				if code != policy || stdout != "" || stderr != "itos push: T9 is not a review thread of owner/name; "+
					"itos push threads lists a pull request's open ones. Nothing was posted.\n" {
					t.Errorf("exit %d\n%s%s", code, stdout, stderr)
				}
				if got, want := f.calls(), []string{nodeCall("T9")}; !reflect.DeepEqual(got, want) {
					t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
				}
				code, stdout, stderr, _ = call(t, f, append([]string{"--json"}, args...)...)
				got := reportedThreads(t, stdout)
				if code != policy || got["outcome"] != "no-such-thread" || got["ok"] != false || !strings.Contains(stderr, "T9 is not a review thread") {
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

// A node question that fails for any reason but an id with no node exits 3,
// naming the call and gh's words, with no word of a push: another error in the
// body, one on another path or of another type, an answer that is no JSON, a
// gh that is missing.
func TestPushThreadWhereTheNodeQuestionFails(t *testing.T) {
	failure := &providers.GhFailed{Words: "HTTP 502\n", Err: errors.New("exit status 1")}
	for name, c := range map[string]struct {
		stdout string
		err    error
		says   string
	}{
		"gh fails":           {"", failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"another path":       {`{"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`, failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"a longer path":      {`{"errors":[{"type":"NOT_FOUND","path":["node","id"]}]}`, failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"no path":            {`{"errors":[{"type":"NOT_FOUND"}]}`, failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"another type":       {`{"errors":[{"type":"FORBIDDEN","path":["node"]}]}`, failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"no JSON":            {"oops", failure, "itos push: gh api graphql failed: HTTP 502\n"},
		"an answer, no JSON": {"oops", nil, "itos push: gh api graphql failed: its answer is not a review thread's\n"},
		"no gh":              {"", providers.ErrNoGh, "itos push: gh api graphql failed: gh is not on the PATH\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := nodeRepo(t)
			f.gh = func([]string) (string, error) { return c.stdout, c.err }
			code, stdout, stderr, _ := call(t, f, "thread", "T1", "--resolve")
			if code != missing || stdout != "" || stderr != c.says || len(f.ghCalls) != 1 || strings.Contains(stderr, "pushed") {
				t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
			}
			code, stdout, _, _ = call(t, f, "--json", "thread", "T1")
			if got := reportedThreads(t, stdout); code != missing || got["outcome"] != "pull-request-failed" || got["ok"] != false {
				t.Errorf("--json: exit %d\n%s", code, stdout)
			}
		})
	}
}

// A reply that fails posts no resolve, and a resolve that fails after the
// reply says the reply was posted.
func TestPushThreadWhereTheAnswerFails(t *testing.T) {
	failing := func(g *graph, word string) func([]string) (string, error) {
		return func(args []string) (string, error) {
			if strings.Contains(strings.Join(args, " "), word) {
				return "", &providers.GhFailed{Words: "HTTP 403\n", Err: errors.New("exit status 1")}
			}
			return g.gh(args)
		}
	}
	t.Run("the reply fails", func(t *testing.T) {
		f, g := nodeRepo(t)
		g.nodes["T1"] = nodeOf("T1", "owner/name", false, "widget.md", "3")
		f.gh = failing(g, "addPullRequestReviewThreadReply")
		code, stdout, stderr, _ := call(t, f, "--json", "thread", "T1", "--reply", "Renamed it.", "--resolve")
		got := reportedThreads(t, stdout)
		if code != missing || got["outcome"] != "pull-request-failed" || got["ok"] != false ||
			!reflect.DeepEqual(got["thread"], map[string]any{"id": "T1", "replied": false, "resolved": false}) ||
			stderr != "itos push: gh api graphql failed: HTTP 403\n" {
			t.Errorf("exit %d\n%s%s", code, stdout, stderr)
		}
		if got := f.calls(); len(got) != 2 || got[1] != reply {
			t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
		}
	})
	t.Run("the resolve fails after the reply", func(t *testing.T) {
		f, g := nodeRepo(t)
		g.nodes["T1"] = nodeOf("T1", "owner/name", false, "widget.md", "3")
		f.gh = failing(g, "resolveReviewThread")
		code, stdout, stderr, _ := call(t, f, "--json", "thread", "T1", "--reply", "Renamed it.", "--resolve")
		got := reportedThreads(t, stdout)
		if code != missing || !reflect.DeepEqual(got["thread"], map[string]any{"id": "T1", "replied": true, "resolved": false}) ||
			stderr != "Replied to T1 on "+open7+".\nitos push: gh api graphql failed: HTTP 403\n" {
			t.Errorf("exit %d\n%s%s", code, stdout, stderr)
		}
	})
}

// push threads --pr n asks for the pull request by its number and no gh pr
// list, from any branch, and lists its open threads as the branch's do, the
// address from GitHub's answer.
func TestPushThreadsListsAPullRequestByItsNumber(t *testing.T) {
	for name, set := range heads {
		t.Run(name, func(t *testing.T) {
			f, g := nodeRepo(t)
			g.pull["7"] = pullAnswer(
				threadBy("T1", false, "widget.md", "3", "alice", "Rename this.\nIt clashes.", "second comment"),
				threadBy("T2", true, "gadget.md", "1", "alice", "Fine now."),
				threadBy("T3", false, "old.md", "null", "bob", "  Outdated one.  "))
			set(f)
			code, stdout, stderr, err := call(t, f, "threads", "--pr", "7")
			want := open7 + " has 2 unresolved review threads:\n" +
				"  T1 widget.md:3 alice: Rename this.\n" +
				"  T3 old.md bob: Outdated one.\n"
			if code != 0 || err != nil || stdout != want || stderr != "" {
				t.Errorf("exit %d, %v\n%s%s", code, err, stdout, stderr)
			}
			if got, want := f.calls(), []string{pullCall(7)}; !reflect.DeepEqual(got, want) {
				t.Errorf("gh was called\n%s", strings.Join(got, "\n"))
			}
			if len(f.pushed) != 0 || len(f.quiet) != 0 {
				t.Errorf("pushed %v, fetched %v", f.pushed, f.quiet)
			}
			code, stdout, stderr, _ = call(t, f, "--json", "threads", "--pr=7")
			wantJSON := `{"schema":1,"ok":true,"outcome":"threads-listed","pull_request":{"url":"` + open7 + `","number":7},"threads":[` +
				`{"id":"T1","path":"widget.md","line":3,"author":"alice","comment":"Rename this."},` +
				`{"id":"T3","path":"old.md","author":"bob","comment":"Outdated one."}]}`
			if code != 0 || stderr != "" || !jsonEqual(t, stdout, wantJSON) {
				t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
	f, g := nodeRepo(t)
	g.pull["7"] = pullAnswer(threadBy("T2", true, "gadget.md", "1", "alice", "Fine now."))
	code, stdout, _, _ := call(t, f, "threads", "--pr", "7")
	if code != 0 || stdout != open7+" has no unresolved review threads.\n" {
		t.Errorf("none open: exit %d\n%s", code, stdout)
	}
}

// A pull request GitHub does not have is its gh that fails, and an answer that
// is not a pull request's threads with its address is no answer: exit 3.
func TestPushThreadsWhereThePullRequestIsNotAnswered(t *testing.T) {
	for name, c := range map[string]struct {
		answer string
		says   string
	}{
		"not there":       {"", "itos push: gh api graphql failed: gh: Could not resolve to a PullRequest\n"},
		"not JSON":        {"oops", "itos push: gh api graphql failed: its answer is not a pull request's review threads\n"},
		"null":            {`{"data":{"repository":{"pullRequest":null}}}`, "its answer is not a pull request's review threads"},
		"no threads":      {`{"data":{"repository":{"pullRequest":{"number":7,"url":"` + open7 + `"}}}}`, "its answer is not a pull request's review threads"},
		"no address":      {`{"data":{"repository":{"pullRequest":{"number":7,"reviewThreads":{"nodes":[]}}}}}`, "its answer is not a pull request's review threads"},
		"no repository":   {`{"data":{"repository":null}}`, "its answer is not a pull request's review threads"},
		"an empty answer": {`{}`, "its answer is not a pull request's review threads"},
	} {
		t.Run(name, func(t *testing.T) {
			f, g := nodeRepo(t)
			if c.answer != "" {
				g.pull["7"] = c.answer
			}
			code, stdout, stderr, _ := call(t, f, "threads", "--pr", "7")
			if code != missing || stdout != "" || !strings.Contains(stderr, c.says) || strings.Contains(stderr, "pushed") {
				t.Errorf("exit %d\n%s%s", code, stdout, stderr)
			}
			code, stdout, _, _ = call(t, f, "--json", "threads", "--pr", "7")
			if got := reportedThreads(t, stdout); code != missing || got["outcome"] != "pull-request-failed" {
				t.Errorf("--json: exit %d\n%s", code, stdout)
			}
		})
	}
}

// Under trunk, with no config, or with a config that cannot be read, every
// form is refused, exit 1 and before gh is asked.
func TestPushThreadsRefusalsAskGhNothing(t *testing.T) {
	for name, set := range map[string]func(*fake){
		"trunk":     func(f *fake) { f.cfg = loaded(t, "version: 1\n") },
		"no config": func(f *fake) { f.found = false },
		"unreadable": func(f *fake) {
			f.loadErr = errors.New("unreadable")
		},
	} {
		for _, args := range [][]string{{"threads"}, {"threads", "--pr", "7"}, {"thread", "T1"}, {"thread", "T1", "--reply", "x", "--resolve"}} {
			t.Run(name+" "+strings.Join(args, " "), func(t *testing.T) {
				f, _ := nodeRepo(t)
				set(f)
				code, stdout, stderr, _ := call(t, f, args...)
				if code != policy || stdout != "" || len(f.ghCalls) != 0 ||
					stderr != "itos push: review threads are an item's pull request's, and this config does not set push.mode: pr; nothing was posted.\n" {
					t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
				}
				code, stdout, _, _ = call(t, f, append([]string{"--json"}, args...)...)
				got := reportedThreads(t, stdout)
				if code != policy || got["outcome"] != "threads-without-pr-mode" || got["ok"] != false {
					t.Errorf("--json: exit %d\n%s", code, stdout)
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

// push threads alone is the branch's: a detached HEAD and main have no pull
// request and say so, naming --pr, before gh is asked; a branch with no open
// pull request is told once gh pr list says so.
func TestPushThreadsOfTheBranchWithoutAPullRequest(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*fake)
		outcome string
		says    string
		calls   int
	}{
		"detached": {func(f *fake) { f.OnBranch = "" }, "detached",
			"itos push: HEAD is not on a branch; check out the item's branch first, or name the pull request with --pr <n>. Nothing was posted.\n", 0},
		"main": {func(f *fake) { f.on("main", mine) }, "threads-on-main",
			"itos push: HEAD is on main, which has no pull request; run it from the item's branch, or name the pull request with --pr <n>. Nothing was posted.\n", 0},
		"no pull request": {func(f *fake) { f.gh = answers(opened, "") }, "no-pull-request",
			"itos push: slice-9 has no open pull request; itos push opens it. Nothing was posted.\n", 1},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := threadsRepo(t)
			c.set(f)
			code, stdout, stderr, _ := call(t, f, "threads")
			if code != policy || stdout != "" || stderr != c.says || len(f.ghCalls) != c.calls {
				t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
			}
			code, stdout, stderr, _ = call(t, f, "--json", "threads")
			got := reportedThreads(t, stdout)
			if code != policy || got["outcome"] != c.outcome || got["ok"] != false || !strings.Contains(stderr, c.says) {
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

// With no GitHub repository to name, every form exits 3 before gh is asked,
// saying what it could not find.
func TestPushThreadsWithNoRepository(t *testing.T) {
	for name, c := range map[string]struct {
		args []string
		says string
	}{
		"threads":   {[]string{"threads"}, "itos push: origin names no GitHub repository to find the pull request of slice-9 in; nothing was posted.\n"},
		"--pr":      {[]string{"threads", "--pr", "7"}, "itos push: origin names no GitHub repository to find pull request 7 in; nothing was posted.\n"},
		"thread":    {[]string{"thread", "T1"}, "itos push: origin names no GitHub repository to find review thread T1 in; nothing was posted.\n"},
		"an answer": {[]string{"thread", "T1", "--resolve"}, "itos push: origin names no GitHub repository to find review thread T1 in; nothing was posted.\n"},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := nodeRepo(t)
			f.repository = ""
			code, stdout, stderr, _ := call(t, f, c.args...)
			if code != missing || stdout != "" || stderr != c.says || len(f.ghCalls) != 0 {
				t.Errorf("exit %d, %d gh calls\n%s%s", code, len(f.ghCalls), stdout, stderr)
			}
			code, stdout, _, _ = call(t, f, append([]string{"--json"}, c.args...)...)
			if got := reportedThreads(t, stdout); code != missing || got["outcome"] != "no-github-repository" {
				t.Errorf("--json: exit %d\n%s", code, stdout)
			}
		})
	}
}

// push threads takes --pr <n> alone and push thread an id with --reply <text>,
// --resolve, both or neither; every other form is a usage error, whatever else
// is given.
func TestParseThreads(t *testing.T) {
	for args, want := range map[string]Query{
		"threads":                             {Threads: true},
		"threads --pr 7":                      {Threads: true, PullNumber: 7},
		"threads --pr=12":                     {Threads: true, PullNumber: 12},
		"thread T1":                           {Thread: "T1"},
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
	const pr = "push threads --pr needs a pull request's number"
	const only = "push threads takes --pr <n> and nothing else"
	for args, want := range map[string]string{
		"threads T1":                        only + " (T1)",
		"threads --ready":                   only + " (--ready)",
		"threads --pr 7 T1":                 only + " (T1)",
		"threads --pr 7 --ready":            only + " (--ready)",
		"threads --pr":                      pr,
		"threads --pr=":                     pr,
		"threads --pr seven":                pr,
		"threads --pr 0":                    pr,
		"threads --pr=0":                    pr,
		"threads --pr -7":                   pr,
		"threads --pr 99999999999999999999": pr,
		"threads --pr 7 --pr 8":             "push threads --pr names one pull request, and it is given twice",
		"thread":                            "push thread needs a thread's id",
		"thread --resolve":                  "push thread needs a thread's id",
		"thread --reply text":               "push thread needs a thread's id",
		"thread T1 --reply":                 "push thread --reply needs the reply's text",
		"thread T1 --reply=":                "push thread --reply needs the reply's text",
		"thread T1 --resolve --reply":       "push thread --reply needs the reply's text",
		"thread T1 T2 --resolve":            "push thread answers one thread, and T2 is a second: T1",
		"thread T1 --resolve --tag v1":      "push thread takes a thread's id, --reply <text> and --resolve, not --tag",
		"thread T1 --pr 7":                  "push thread takes a thread's id, --reply <text> and --resolve, not --pr",
		"thread T1 --pr=7":                  "push thread takes a thread's id, --reply <text> and --resolve, not --pr=7",
		"thread T1 --resolve=yes":           "push thread takes a thread's id, --reply <text> and --resolve, not --resolve=yes",
		"--no-wait threads":                 "push takes no arguments but --no-wait",
		"--pr 7":                            "push takes no arguments but --no-wait",
		"widgets":                           "push takes no arguments but --no-wait",
		"--reply text":                      "push takes no arguments but --no-wait",
		"--resolve":                         "push takes no arguments but --no-wait",
		"--ready thread T1 --resolve":       "push takes no arguments but --no-wait",
		"--tag v1 threads":                  "push takes no arguments but --no-wait",
		"thread T1 --reply x --ref main":    "push thread takes a thread's id, --reply <text> and --resolve, not --ref",
	} {
		_, err := Parse(strings.Fields(args), false, false, "")
		var u out.UsageError
		if !errors.As(err, &u) || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want %q", args, err, want)
		}
	}
}
