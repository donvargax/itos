package providers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// The request the provider makes, and every failure read as no green run.
func TestLastGreenRun(t *testing.T) {
	var asked *http.Request
	answer, status := `{"workflow_runs":[{"head_sha":"abc","conclusion":"success","created_at":"x"}]}`, 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	defer server.Close()
	defer func(api string) { GitHubAPI = api }(GitHubAPI)
	GitHubAPI = server.URL

	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "a b&c"}
	if got := g.LastGreenRun(); got != "abc" {
		t.Errorf("LastGreenRun = %q, want abc", got)
	}
	if want := "/repos/o/r/actions/workflows/ci.yml/runs?branch=a%20b%26c&per_page=50"; asked.URL.RequestURI() != want {
		t.Errorf("asked %s, want %s", asked.URL.RequestURI(), want)
	}
	if asked.Header.Get("Authorization") != "Bearer t" || asked.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers %v", asked.Header)
	}

	asked = nil
	g.Token = ""
	g.LastGreenRun()
	if asked == nil || asked.Header.Get("Authorization") != "" {
		t.Errorf("without a token, no authorization: %v", asked)
	}

	for _, c := range []struct {
		answer string
		status int
	}{{answer, 404}, {"not json", 200}, {`{"workflow_runs":{}}`, 200}, {`{}`, 200}} {
		answer, status = c.answer, c.status
		if got := g.LastGreenRun(); got != "" {
			t.Errorf("%d %s: LastGreenRun = %q, want none", c.status, c.answer, got)
		}
	}

	asked = nil
	if got := (GitHub{Workflow: "ci.yml", Branch: "main"}).LastGreenRun(); got != "" || asked != nil {
		t.Errorf("without a repository nobody is asked: %q, %v", got, asked)
	}
}

// What itos status reads of the last green commit (slice 73): LastGreen
// says why it could not read the runs, where LastGreenRun reads none.
func TestLastGreenSaysWhatWentWrong(t *testing.T) {
	answer, status := `{"workflow_runs":[{"head_sha":"abc","conclusion":"success","created_at":"x"}]}`, 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	defer server.Close()
	defer func(api string) { GitHubAPI = api }(GitHubAPI)
	GitHubAPI = server.URL

	g := GitHub{Repository: "o/r", Token: "t", Workflow: "ci.yml", Branch: "main"}
	if sha, err := g.LastGreen(); sha != "abc" || err != nil {
		t.Errorf("LastGreen = %q, %v, want abc", sha, err)
	}
	answer = `{"workflow_runs":[]}`
	if sha, err := g.LastGreen(); sha != "" || err != nil {
		t.Errorf("no green run: LastGreen = %q, %v, want none and no error", sha, err)
	}
	for _, c := range []struct {
		answer string
		status int
	}{{"{}", 404}, {"not json", 200}} {
		answer, status = c.answer, c.status
		if sha, err := g.LastGreen(); sha != "" || err == nil {
			t.Errorf("%d %s: LastGreen = %q, %v, want an error", c.status, c.answer, sha, err)
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
