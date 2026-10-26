package providers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

const pullsBody = `[
  {"number": 3, "html_url": "https://github.example/o/n/pull/3", "state": "open", "merged_at": null, "merge_commit_sha": "x1", "head": {"sha": "h3", "ref": "a"}},
  {"number": 7, "html_url": "https://github.example/o/n/pull/7", "state": "closed", "merged_at": "2026-10-26T10:00:00Z", "merge_commit_sha": "m7", "head": {"sha": "h7", "ref": "b"}}
]`

// The pull requests of a commit are read from its pulls endpoint with the
// token: the number and address, whether the pull request merged (merged_at
// set), the commit it merged as and its head.
func TestPullsOfReadsTheCommitsPullRequests(t *testing.T) {
	var asked *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r
		_, _ = w.Write([]byte(pullsBody))
	}))
	defer server.Close()
	pulls, err := GitHub{Repository: "o/n", Token: "t", API: server.URL}.PullsOf("abc")
	want := []Pull{
		{Number: 3, URL: "https://github.example/o/n/pull/3", MergeCommit: "x1", Head: "h3"},
		{Number: 7, URL: "https://github.example/o/n/pull/7", Merged: true, MergeCommit: "m7", Head: "h7"},
	}
	if err != nil || len(pulls) != 2 || pulls[0] != want[0] || pulls[1] != want[1] {
		t.Errorf("PullsOf = %+v, %v, want %+v", pulls, err, want)
	}
	if asked.URL.Path != "/repos/o/n/commits/abc/pulls" || asked.Header.Get("Authorization") != "Bearer t" {
		t.Errorf("asked %s with %v", asked.URL.RequestURI(), asked.Header)
	}

	for name, status := range map[string]int{"a refusal": http.StatusForbidden, "no such commit": http.StatusNotFound, "a server error": http.StatusInternalServerError} {
		failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", status) }))
		if pulls, err := (GitHub{Repository: "o/n", API: failing.URL}).PullsOf("abc"); err == nil || pulls != nil {
			t.Errorf("%s: PullsOf = %+v, %v", name, pulls, err)
		}
		failing.Close()
	}
	notJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer notJSON.Close()
	if pulls, err := (GitHub{Repository: "o/n", API: notJSON.URL}).PullsOf("abc"); err == nil || pulls != nil {
		t.Errorf("an object, not a list: PullsOf = %+v, %v", pulls, err)
	}
}

// The look is ci.range's github provider's: the repository and token its
// variables name, the API at GITHUB_API_URL, the workflow ci.range names for
// the run; none for provider none and for no repository, which asks nobody.
func TestMergedPullsProviderAsksAsTheRangeProviderDoes(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.RequestURI()+" "+r.Header.Get("Authorization"))
		if r.URL.Path == "/repos/o/n/commits/abc/pulls" {
			_, _ = w.Write([]byte(pullsBody))
			return
		}
		_, _ = w.Write([]byte(`{"workflow_runs":[{"id":5,"html_url":"u-run","status":"completed","conclusion":"success","created_at":"x","head_sha":"abc"}],"jobs":[]}`))
	}))
	defer server.Close()
	cfg := &config.Loaded{}
	cfg.CI.Range.Provider = "github"
	cfg.CI.Range.GitHub.RepositoryEnv = "REPO"
	cfg.CI.Range.GitHub.TokenEnv = []string{"UNSET", "TOKEN"}
	cfg.CI.Range.GitHub.Workflow = "range.yml"
	env := map[string]string{"REPO": "o/n", "TOKEN": "tok", "GITHUB_API_URL": server.URL}
	look, ok := MergedPullsProvider(cfg, func(name string) string { return env[name] })
	if !ok {
		t.Fatal("no look")
	}
	if pulls, err := look.Pulls("abc"); err != nil || len(pulls) != 2 {
		t.Errorf("Pulls = %+v, %v", pulls, err)
	}
	if run, found, err := look.Run("abc"); err != nil || !found || run.URL != "u-run" {
		t.Errorf("Run = %+v, %v, %v", run, found, err)
	}
	if len(asked) != 3 || asked[0] != "/repos/o/n/commits/abc/pulls Bearer tok" ||
		asked[1] != "/repos/o/n/actions/workflows/range.yml/runs?head_sha=abc&per_page=20 Bearer tok" {
		t.Errorf("asked %q", asked)
	}

	if _, ok := MergedPullsProvider(cfg, func(name string) string { return map[string]string{"TOKEN": "tok"}[name] }); ok {
		t.Error("no repository, and a look")
	}
	cfg.CI.Range.Provider = "none"
	if _, ok := MergedPullsProvider(cfg, func(name string) string { return env[name] }); ok {
		t.Error("provider none, and a look")
	}
}
