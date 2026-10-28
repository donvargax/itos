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
