package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func success() *string { s := "success"; return &s }

// The github provider's choice of run, which no conformance case reaches
// without a network (tools/selftest/ci-range.ts holds the TypeScript's to the
// same list): the newest success, whatever the order and however many failed,
// cancelled or running runs are newer.
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

// The request the TypeScript makes, and every failure read as no green run.
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
