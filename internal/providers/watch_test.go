package providers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/donvargax/itos/v2/internal/config"
)

func TestReadRunTakesTheRunObject(t *testing.T) {
	run, err := ReadRun([]byte(`{"url":"u","status":"completed","conclusion":"failure","jobs":[{"name":"ci","status":"completed","conclusion":"failure"},{"name":"p","status":"queued"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !run.Done() || run.Conclusion != "failure" || len(run.Jobs) != 2 || !run.Jobs[0].Done() || run.Jobs[1].Done() {
		t.Fatalf("read %+v", run)
	}
}

func TestReadRunRefusesWhatIsNotARun(t *testing.T) {
	for _, text := range []string{
		"not json",
		`{"url":"u","status":"running"}`,
		`{"url":"u","status":"completed"}`,
		`{"url":"u","status":"queued","jobs":[{"status":"queued"}]}`,
		`{"url":"u","status":"queued","jobs":[{"name":"ci","status":"completed"}]}`,
		`{"url":"u","status":"queued"} {}`,
		`[]`,
	} {
		if _, err := ReadRun([]byte(text)); err == nil {
			t.Errorf("%s read as a run", text)
		}
	}
}

func TestGitHubRepositoryReadsAGitHubRemote(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:donvargax/itos.git":          "donvargax/itos",
		"https://github.com/donvargax/itos":          "donvargax/itos",
		"https://github.com/donvargax/itos.git":      "donvargax/itos",
		"ssh://git@github.com/donvargax/itos.git":    "donvargax/itos",
		"https://user@github.com/donvargax/itos.git": "donvargax/itos",
		"/srv/git/itos.git":                          "",
		"https://gitlab.com/donvargax/itos.git":      "",
	} {
		if got := GitHubRepository(url); got != want {
			t.Errorf("GitHubRepository(%q) = %q, not %q", url, got, want)
		}
	}
}

// githubWatchConfig is a loaded config whose ci.watch provider is github.
func githubWatchConfig() *config.Loaded {
	cfg := &config.Loaded{}
	cfg.CI.Watch.Provider = "github"
	cfg.CI.Watch.GitHub.Workflow = "ci.yml"
	cfg.CI.Range.GitHub.RepositoryEnv = "GITHUB_REPOSITORY"
	cfg.CI.Range.GitHub.TokenEnv = []string{"GITHUB_TOKEN", "GH_TOKEN"}
	return cfg
}

func TestTheGitHubWatchNeedsAToken(t *testing.T) {
	noEnv := func(string) string { return "" }
	signedOut := func() (string, error) { return "", errors.New("exit 4") }
	_, ok, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: noEnv, GhToken: signedOut, RemoteURL: "git@github.com:o/n.git"})
	if ok || err == nil || !strings.Contains(err.Error(), "GH_TOKEN") || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	fromGh := func() (string, error) { return "t", nil }
	if _, ok, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: noEnv, GhToken: fromGh, RemoteURL: "git@github.com:o/n.git"}); !ok || err != nil {
		t.Fatalf("with gh's token: ok %v, err %v", ok, err)
	}
	if _, _, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: noEnv, GhToken: fromGh, RemoteURL: "/srv/n.git"}); err == nil {
		t.Fatal("a remote that is not GitHub's gave a repository")
	}
}

func TestRunOfReadsTheNewestRunAndItsJobs(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/workflows/"):
			w.Write([]byte(`{"workflow_runs":[
				{"id":1,"html_url":"old","status":"completed","conclusion":"failure","created_at":"2026-10-03T10:00:00Z"},
				{"id":2,"html_url":"new","status":"in_progress","conclusion":null,"created_at":"2026-10-03T11:00:00Z"}]}`))
		case strings.HasSuffix(r.URL.Path, "/runs/2/jobs"):
			w.Write([]byte(`{"jobs":[{"name":"ci","status":"completed","conclusion":"success"},{"name":"platform","status":"in_progress","conclusion":null}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()

	run, found, err := GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}.RunOf("abc")
	if err != nil || !found {
		t.Fatalf("found %v, err %v", found, err)
	}
	if run.URL != "new" || run.Done() || len(run.Jobs) != 2 || !run.Jobs[0].Done() || run.Jobs[1].Done() {
		t.Fatalf("read %+v", run)
	}
	if !strings.Contains(asked[0], "/repos/o/n/actions/workflows/ci.yml/runs?head_sha=abc") {
		t.Fatalf("asked %v", asked)
	}
	if _, _, err := (GitHub{Repository: "o/n", Token: "bad", Workflow: "ci.yml"}).RunOf("abc"); err == nil || errors.As(err, new(Transient)) {
		t.Fatalf("a refused token: %v", err)
	}
}

func TestRunOfFindsNoRunYet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"workflow_runs":[]}`))
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()
	if _, found, err := (GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}).RunOf("abc"); found || err != nil {
		t.Fatalf("found %v, err %v", found, err)
	}
}

func TestAServerErrorIsTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()
	_, _, err := GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}.RunOf("abc")
	if !errors.As(err, new(Transient)) {
		t.Fatalf("a 502 is %v, not transient", err)
	}
}
