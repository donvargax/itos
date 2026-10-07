package providers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v7/internal/git"
)

func success() *string { s := "success"; return &s }

// The github provider's choice of run, which no conformance case reaches
// without a network: the newest success, whatever the order and however many
// failed, cancelled or running runs are newer. T-008 runs it, in place of
// tools/selftest/ci-range.ts, which held the TypeScript to the same list.
func TestFirstGreen(t *testing.T) {
	failure, cancelled := "failure", "cancelled"
	runs := []WorkflowRun{
		{HeadSHA: "old", Conclusion: success(), CreatedAt: "2000-01-01T08:00:00Z"},
		{HeadSHA: "red", Conclusion: &failure, CreatedAt: "2000-01-09T14:20:00Z"},
		{HeadSHA: "new", Conclusion: success(), CreatedAt: "2000-01-09T13:33:00Z"},
		{HeadSHA: "gone", Conclusion: &cancelled, CreatedAt: "2000-01-09T14:33:00Z"},
		{HeadSHA: "busy", CreatedAt: "2000-01-09T14:35:00Z"},
	}
	if got := FirstGreen(runs); got != "new" {
		t.Errorf("FirstGreen = %q, want the newest success", got)
	}
	if got := FirstGreen([]WorkflowRun{{HeadSHA: "red", Conclusion: &failure}}); got != "" {
		t.Errorf("FirstGreen with no success = %q", got)
	}
}

// A repository for the walk, the working folder for the test: c0, c1, a side
// branch off c1 (s1) merged after c2 (m), then the head, c3. Its first
// parents from the head's parent are m, c2, c1 and c0; s1 is none of them.
func walkRepository(t *testing.T) map[string]string {
	t.Helper()
	// Away from the repository a hook runs the tests in: the walk's git
	// reads the environment too.
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	dir := t.TempDir()
	t.Chdir(dir)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git.Bin(), append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(name string) string {
		run("commit", "-q", "--allow-empty", "-m", name)
		return run("rev-parse", "HEAD")
	}
	shas := map[string]string{}
	run("init", "-q", "-b", "main")
	shas["c0"], shas["c1"] = commit("c0"), commit("c1")
	run("checkout", "-q", "-b", "side")
	shas["s1"] = commit("s1")
	run("checkout", "-q", "main")
	shas["c2"] = commit("c2")
	run("merge", "-q", "--no-ff", "-m", "m", "side")
	shas["m"] = run("rev-parse", "HEAD")
	shas["c3"] = commit("c3")
	return shas
}

// A fake GitHub answering each commit's runs from conclusions, by full SHA:
// "" a run still going, "none" no run; it records each request.
type walkServer struct {
	runs   map[string]string
	answer string // when set, the body of every answer
	status int
	asked  []*http.Request
}

func (s *walkServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.asked = append(s.asked, r)
	if s.status != 0 {
		w.WriteHeader(s.status)
	}
	if s.answer != "" {
		_, _ = w.Write([]byte(s.answer))
		return
	}
	sha := r.URL.Query().Get("head_sha")
	conclusion, ok := s.runs[sha]
	if !ok || conclusion == "none" {
		_, _ = w.Write([]byte(`{"workflow_runs":[]}`))
		return
	}
	c := "null"
	if conclusion != "" {
		c = `"` + conclusion + `"`
	}
	fmt.Fprintf(w, `{"workflow_runs":[{"head_sha":%q,"status":"completed","conclusion":%s,"created_at":"x"}]}`, sha, c)
}

func (s *walkServer) askedFor() []string {
	var shas []string
	for _, r := range s.asked {
		shas = append(shas, r.URL.Query().Get("head_sha"))
	}
	return shas
}

// The github range provider walks the head's first parents from its parent,
// asking each commit's own runs, and starts at the first with a green run:
// a failed, cancelled or unfinished run is passed over, the head's own run
// and a merged branch's commits are never asked about (bug 23), and the runs
// are asked for by commit alone, on any branch, though a branch is set (bug 29).
func TestNearestGreenWalksTheFirstParents(t *testing.T) {
	shas := walkRepository(t)
	s := &walkServer{runs: map[string]string{
		shas["c3"]: "success", shas["m"]: "failure", shas["c2"]: "", shas["s1"]: "success", shas["c1"]: "success",
	}}
	server := httptest.NewServer(s)
	defer server.Close()
	defer func(api string) { GitHubAPI = api }(GitHubAPI)
	GitHubAPI = "http://127.0.0.1:1/not-asked"

	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "a b&c", API: server.URL + "/"}
	if got := g.NearestGreen(shas["c3"]); got != shas["c1"] {
		t.Fatalf("NearestGreen = %q, want c1 %q; asked %v", got, shas["c1"], s.askedFor())
	}
	if want := []string{shas["m"], shas["c2"], shas["c1"]}; !slices.Equal(s.askedFor(), want) {
		t.Fatalf("asked %v, want m, c2, c1 %v", s.askedFor(), want)
	}
	asked := s.asked[0]
	if want := "/repos/o/r/actions/workflows/ci.yml/runs?head_sha=" + shas["m"] + "&per_page=20"; asked.URL.RequestURI() != want {
		t.Errorf("asked %s, want %s", asked.URL.RequestURI(), want)
	}
	if asked.Header.Get("Authorization") != "Bearer t" || asked.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers %v", asked.Header)
	}

	s.asked, g.Token = nil, ""
	s.runs[shas["c1"]] = "cancelled"
	if got := g.NearestGreen(shas["c3"]); got != "" {
		t.Fatalf("no green first parent: NearestGreen = %q", got)
	}
	if want := []string{shas["m"], shas["c2"], shas["c1"], shas["c0"]}; !slices.Equal(s.askedFor(), want) {
		t.Fatalf("asked %v, want every first parent %v", s.askedFor(), want)
	}
	if s.asked[0].Header.Get("Authorization") != "" {
		t.Errorf("without a token, no authorization: %v", s.asked[0].Header)
	}
}

// Past FirstParentsAsked first parents the provider gives up, which runs
// everything.
func TestNearestGreenAsksABoundedNumberOfFirstParents(t *testing.T) {
	shas := walkRepository(t)
	s := &walkServer{runs: map[string]string{shas["c1"]: "success"}}
	server := httptest.NewServer(s)
	defer server.Close()
	defer func(n int) { FirstParentsAsked = n }(FirstParentsAsked)
	FirstParentsAsked = 2

	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "main", API: server.URL}
	if got := g.NearestGreen(shas["c3"]); got != "" || len(s.asked) != 2 {
		t.Fatalf("NearestGreen = %q after %v, want none after two", got, s.askedFor())
	}
	FirstParentsAsked = 3
	if got := g.NearestGreen(shas["c3"]); got != shas["c1"] {
		t.Fatalf("NearestGreen = %q, want c1", got)
	}
}

// Anything that goes wrong reads as no green run, and the walk stops there;
// a green run of another commit, as a server ignoring head_sha would list,
// is not the commit's; with no repository nobody is asked.
func TestNearestGreenReadsEveryFailureAsNoGreenRun(t *testing.T) {
	shas := walkRepository(t)
	s := &walkServer{}
	server := httptest.NewServer(s)
	defer server.Close()
	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "main", API: server.URL}
	for _, c := range []struct {
		answer string
		status int
	}{
		{`{"workflow_runs":[{"head_sha":"abc","conclusion":"success"}]}`, 404},
		{"not json", 200},
		{`{"workflow_runs":{}}`, 200},
	} {
		s.answer, s.status, s.asked = c.answer, c.status, nil
		if got := g.NearestGreen(shas["c3"]); got != "" || len(s.asked) != 1 {
			t.Errorf("%d %s: NearestGreen = %q after %d requests, want none after one", c.status, c.answer, got, len(s.asked))
		}
	}
	s.answer, s.status, s.asked = `{"workflow_runs":[{"head_sha":"`+shas["c0"]+`","conclusion":"success"}]}`, 0, nil
	if got := g.NearestGreen(shas["c3"]); got != shas["c0"] || len(s.asked) != 4 {
		t.Errorf("another commit's green run: NearestGreen = %q after %d requests, want c0 after four", got, len(s.asked))
	}
	if got := g.NearestGreen("not-a-commit"); got != "" {
		t.Errorf("a head the repository does not have: NearestGreen = %q", got)
	}
	s.asked = nil
	if got := (GitHub{Workflow: "ci.yml", Branch: "main", API: server.URL}).NearestGreen(shas["c3"]); got != "" || s.asked != nil {
		t.Errorf("without a repository nobody is asked: %q, %v", got, s.askedFor())
	}
}

// What itos status reads of the last green commit (bug 24): the walk of
// NearestGreen, from the head itself, saying why when it cannot list the
// first parents or ask about one, where NearestGreen reads that as none.
func TestLastGreenFromWalksFromTheHeadAndSaysWhatWentWrong(t *testing.T) {
	shas := walkRepository(t)
	s := &walkServer{runs: map[string]string{shas["c3"]: "success", shas["c2"]: "success"}}
	server := httptest.NewServer(s)
	defer server.Close()
	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "main", API: server.URL}
	if sha, err := g.LastGreenFrom(shas["c3"]); sha != shas["c3"] || err != nil {
		t.Errorf("a green head: LastGreenFrom = %q, %v, want c3", sha, err)
	}
	s.runs[shas["c3"]], s.asked = "in_progress", nil
	if sha, err := g.LastGreenFrom(shas["c3"]); sha != shas["c2"] || err != nil {
		t.Errorf("LastGreenFrom = %q, %v, want c2 after c3 and m; asked %v", sha, err, s.askedFor())
	}
	s.runs[shas["c2"]], s.asked = "failure", nil
	if sha, err := g.LastGreenFrom(shas["c3"]); sha != "" || err != nil || len(s.asked) != 5 {
		t.Errorf("no green run: LastGreenFrom = %q, %v after %v, want none and no error after five", sha, err, s.askedFor())
	}
	if sha, err := g.LastGreenFrom("refs/remotes/origin/main"); sha != "" || err == nil || !strings.Contains(err.Error(), "refs/remotes/origin/main") {
		t.Errorf("a head the repository does not have: LastGreenFrom = %q, %v, want an error naming it", sha, err)
	}
	for _, c := range []struct {
		answer string
		status int
	}{{"{}", 404}, {"not json", 200}} {
		s.answer, s.status = c.answer, c.status
		if sha, err := g.LastGreenFrom(shas["c3"]); sha != "" || err == nil {
			t.Errorf("%d %s: LastGreenFrom = %q, %v, want an error", c.status, c.answer, sha, err)
		}
	}
}

// ci.range's provider none reads no last green commit, and github's says
// before asking when it has no token, naming ci.range.
func TestTheLastGreenProviderIsCIRanges(t *testing.T) {
	noEnv := func(string) string { return "" }
	signedOut := func() (string, error) { return "", errors.New("exit 4") }
	setup := WatchSetup{Env: noEnv, GhToken: signedOut, RemoteURL: "git@github.com:o/n.git"}
	cfg := githubWatchConfig()
	if _, ok, err := LastGreenProvider(cfg, setup); ok || err == nil || !strings.HasPrefix(err.Error(), "ci.range's github provider needs a token") {
		t.Fatalf("github with no token: ok %v, err %v", ok, err)
	}
	cfg.CI.Range.Provider = "none"
	if _, ok, err := LastGreenProvider(cfg, setup); ok || err != nil {
		t.Fatalf("provider none: ok %v, err %v", ok, err)
	}
}
