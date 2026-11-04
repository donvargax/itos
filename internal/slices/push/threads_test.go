package push

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
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
// place (no line for an outdated one), its first comment's author and summary,
// that comment whole beneath where it says more, and asks the pull request and
// its threads and nothing else: nothing is fetched, rebased or pushed.
func TestPushThreadsListsTheOpenThreads(t *testing.T) {
	f, _ := threadsRepo(t)
	code, stdout, stderr, err := call(t, f, "threads")
	want := open7 + " has 3 unresolved review threads:\n" +
		"  T1 widget.md:3 alice: Rename this.\n" +
		"    Rename this.\n" +
		"    It clashes.\n" +
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
		{"id":"T1","path":"widget.md","line":3,"author":"alice","comment":"Rename this.","body":"Rename this.\nIt clashes."},
		{"id":"T3","path":"old.md","author":"bob","comment":"Outdated one.","body":"  Outdated one.  "},
		{"id":"T4","path":"gone.md","line":5,"author":"","comment":"Written by a ghost.","body":"Written by a ghost."}]}`), &wantJSON)
	if got := reportedThreads(t, stdout); code != 0 || !reflect.DeepEqual(got, wantJSON) || stderr != "" {
		t.Errorf("--json: exit %d\n%s%s", code, stdout, stderr)
	}
}

// A comment's summary, by the rules of bug 165 and on bodies shaped as Codex's
// security and code findings and Bugbot's are.
func TestCommentSummary(t *testing.T) {
	const (
		security = "<!-- codex-security-review-finding:v1 -->\n\n### 🛡️ Codex Security Review\n\n" +
			"**<sub><sub>![P2 Badge](https://img.shields.io/badge/P2-yellow?style=flat)</sub></sub> Security: Keep the token out**\n\n" +
			"A widget logs its token.\n\nUseful? React with 👍 / 👎.\n"
		code = "**<sub><sub>![P1 Badge](https://img.shields.io/badge/P1-orange?style=flat)</sub></sub>  Limit the retries**\n\n" +
			"A widget that never answers is asked for ever.\n\n<!-- codex-code-review-finding:v1 -->\n"
		bugbot = "### Reviews not pinned\n\n**High Severity**\n\n<!-- DESCRIPTION START -->\nThe review is read at any head.\n<!-- DESCRIPTION END -->\n\n" +
			"<!-- BUGBOT_BUG_ID: 1234 -->\n\n<!-- LOCATIONS START\nwidget.go#L1-L2\nLOCATIONS END -->\n"
	)
	for _, c := range []struct{ body, want string }{
		{security, "### 🛡️ Codex Security Review: ** Security: Keep the token out**"},
		{code, "**  Limit the retries**"},
		{bugbot, "### Reviews not pinned: **High Severity**"},

		{"Rename this.", "Rename this."},
		{"  Rename this.  \nIt clashes.", "Rename this."},
		{"Rename this.\r\nIt clashes.\r\n", "Rename this."},
		{"\n\nRename this.", "Rename this."},

		{"", "(no text)"},
		{" \n\t\n", "(no text)"},
		{"<!-- marker:v1 -->", "(no text)"},
		{"<!-- marker:v1 -->\n<sub>![P1](a.png)</sub>", "(no text)"},
		{"### 🛡️", "(no text)"},
		{"---", "(no text)"},
		{"👍 / 👎", "(no text)"},

		{"---\nRename this.", "Rename this."},
		{"👍\nRename this.", "Rename this."},
		{"42", "42"},
		{"¿Qué?", "¿Qué?"},
		{"<!-- across\nlines -->\nRename this.", "Rename this."},
		{"<!-- across\r\nlines -->\r\nRename this.", "Rename this."},
		{"<!-- one --> Rename <!-- two -->this.", "Rename this."},
		{"Quoting `<!-- marker:v1 -->` here.", "Quoting `` here."},
		{"<!-- never closes\nRename this.", "<!-- never closes"},
		{"Say --reply <text> here.", "Say --reply <text> here."},
		{"2**3 is eight.", "2**3 is eight."},

		{"![shot](a.png)\nRename this.", "Rename this."},
		{"See ![shot](a.png) there.", "See  there."},
		{"<sub>![shot](a.png)</sub> Rename this.", "Rename this."},
		{"<sub><sub>![](a.png)</sub></sub>Rename this.", "Rename this."},
		{"<sub>Rename this.</sub>", "<sub>Rename this.</sub>"},
		{"![not closed\nRename](this.)", "![not closed"},
		{"![text] and (more)", "![text] and (more)"},

		{"# One\nNext.", "# One: Next."},
		{"###### Six\n\nNext.", "###### Six: Next."},
		{"### Title\r\n\r\nNext.\r\nLast.\r\n", "### Title: Next."},
		{"  ### Indented\n  Next.  ", "### Indented: Next."},
		{"#243 broke this.\nNext.", "#243 broke this."},
		{"####### Seven\nNext.", "####### Seven"},
		{"# One", "# One"},
		{"# One\n## Two", "# One"},
		{"# One\n## Two\nNext.", "# One: ## Two"},
		{"# One\n---\n<!-- marker -->\n", "# One"},
		{"Next.\n# One", "Next."},
	} {
		if got := commentSummary(c.body); got != c.want {
			t.Errorf("commentSummary(%q) = %q, not %q", c.body, got, c.want)
		}
	}
}

// What goes beneath a thread's line, and --json's comment and body, through the
// branch's pull request and through --pr.
func TestPushThreadsCommentBeneath(t *testing.T) {
	for name, c := range map[string]struct {
		comments              []string
		line, beneath         string
		wantComment, wantBody string
	}{
		"a \\r\\n is read as a \\n beneath": {
			comments: []string{"Rename this.\r\n\r\nIt clashes.\r\n"},
			line:     "Rename this.", beneath: "    Rename this.\n    \n    It clashes.\n    \n",
			wantComment: "Rename this.", wantBody: "Rename this.\r\n\r\nIt clashes.\r\n",
		},
		"an empty body": {
			comments: []string{""},
			line:     "(no text)", wantComment: "(no text)",
		},
		"no comment": {
			line: "(no text)", wantComment: "(no text)",
		},
		"a body of spaces": {
			comments: []string{" \n "},
			line:     "(no text)", wantComment: "(no text)", wantBody: " \n ",
		},
		"one line": {
			comments: []string{"  Rename this.\r\n"},
			line:     "Rename this.", wantComment: "Rename this.", wantBody: "  Rename this.\r\n",
		},
		"a marker alone": {
			comments: []string{"<!-- marker:v1 -->"},
			line:     "(no text)", beneath: "    <!-- marker:v1 -->\n",
			wantComment: "(no text)", wantBody: "<!-- marker:v1 -->",
		},
	} {
		node := threadBy("T1", false, "widget.md", "3", "alice", c.comments...)
		for path, run := range map[string]func(t *testing.T) (*fake, []string){
			"the branch's pull request": func(t *testing.T) (*fake, []string) {
				f, p := threadsRepo(t)
				p.threads = []string{node}
				return f, []string{"threads"}
			},
			"--pr": func(t *testing.T) (*fake, []string) {
				f, g := nodeRepo(t)
				g.pull["7"] = pullAnswer(node)
				return f, []string{"threads", "--pr", "7"}
			},
		} {
			t.Run(name+", "+path, func(t *testing.T) {
				f, args := run(t)
				code, stdout, _, _ := call(t, f, args...)
				author := "alice"
				if len(c.comments) == 0 {
					author = ""
				}
				want := open7 + " has 1 unresolved review threads:\n  T1 widget.md:3 " + author + ": " + c.line + "\n" + c.beneath
				if code != 0 || stdout != want {
					t.Errorf("exit %d\n%q\nnot\n%q", code, stdout, want)
				}
				_, stdout, _, _ = call(t, f, append([]string{"--json"}, args...)...)
				threads, _ := reportedThreads(t, stdout)["threads"].([]any)
				if len(threads) != 1 {
					t.Fatalf("--json: %s", stdout)
				}
				got, _ := threads[0].(map[string]any)
				if body, has := got["body"]; got["comment"] != c.wantComment || !has || body != c.wantBody {
					t.Errorf("--json: %s", stdout)
				}
			})
		}
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

// Main is found as push finds it: a remote that cannot be asked exits by the
// kind of the failure, and one whose HEAD names no branch it has exits 3,
// before gh is asked.
func TestPushThreadsWhereMainCannotBeFound(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*fake)
		code    int
		outcome string
	}{
		"unreachable": {func(f *fake) { f.headErr = &vcsport.Failed{Failure: vcsport.Unreachable} }, temporary, "fetch-failed"},
		"no default":  {func(f *fake) { f.remoteHead = "" }, missing, "no-default-branch"},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := threadsRepo(t)
			c.set(f)
			code, stdout, _, _ := call(t, f, "--json", "threads")
			if got := reportedThreads(t, stdout); code != c.code || got["outcome"] != c.outcome || len(f.ghCalls) != 0 {
				t.Errorf("exit %d, %d gh calls\n%s", code, len(f.ghCalls), stdout)
			}
		})
	}
}
