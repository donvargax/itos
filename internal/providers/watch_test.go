package providers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/kind"
)

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
	if _, _, err := (GitHub{Repository: "o/n", Token: "bad", Workflow: "ci.yml"}).RunOf("abc"); err == nil || kind.Of(err) == kind.Temporary {
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

func TestAServerErrorIsTemporary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()
	_, _, err := GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}.RunOf("abc")
	if kind.Of(err) != kind.Temporary {
		t.Fatalf("a 502 is %v, not temporary", err)
	}
}

func TestTheNightlyIsReadOnlyWhereTheProviderNamesOne(t *testing.T) {
	noEnv := func(string) string { return "" }
	signedOut := func() (string, error) { return "", errors.New("exit 4") }
	setup := WatchSetup{Env: noEnv, GhToken: signedOut, RemoteURL: "git@github.com:o/n.git"}
	cfg := githubWatchConfig()
	if _, ok, err := NightlyProvider(cfg, "main", setup); ok || err != nil {
		t.Fatalf("github with no nightly workflow: ok %v, err %v", ok, err)
	}
	cfg.CI.Watch.GitHub.NightlyWorkflow = "nightly.yml"
	if _, ok, err := NightlyProvider(cfg, "main", setup); ok || err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("github's nightly with no token: ok %v, err %v", ok, err)
	}
	cfg.CI.Watch.Provider = "none"
	if _, ok, err := NightlyProvider(cfg, "main", setup); ok || err != nil {
		t.Fatalf("provider none: ok %v, err %v", ok, err)
	}
}

// itos status's looks at GitHub, the nightly's and the last green commit's,
// ask the API GITHUB_API_URL names, as ci range and ci watch do (bug 24).
func TestStatusLooksAskGitHubAPIURL(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Write([]byte(`{"workflow_runs":[]}`))
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = "http://127.0.0.1:1/not-asked"
	defer func() { GitHubAPI = was }()
	env := map[string]string{"GITHUB_TOKEN": "t", "GITHUB_REPOSITORY": "o/n", "GITHUB_API_URL": server.URL + "/api/v3"}
	setup := WatchSetup{Env: func(name string) string { return env[name] }}
	cfg := githubWatchConfig()
	cfg.CI.Watch.GitHub.NightlyWorkflow = "nightly.yml"
	cfg.CI.Range.Provider, cfg.CI.Range.GitHub.Workflow = "github", "range.yml"
	nightly, ok, err := NightlyProvider(cfg, "main", setup)
	if !ok || err != nil {
		t.Fatalf("nightly: ok %v, err %v", ok, err)
	}
	if _, found, err := nightly(); found || err != nil {
		t.Fatalf("nightly: found %v, err %v", found, err)
	}
	if len(asked) != 1 || asked[0] != "/api/v3/repos/o/n/actions/workflows/nightly.yml/runs" {
		t.Fatalf("the nightly asked %v", asked)
	}
	look, ok, err := LastGreenProvider(cfg, setup)
	if !ok || err != nil {
		t.Fatalf("last green: ok %v, err %v", ok, err)
	}
	shas := walkRepository(t)
	asked = nil
	if sha, err := look(shas["c3"]); sha != "" || err != nil {
		t.Fatalf("last green: %q, %v", sha, err)
	}
	if len(asked) == 0 || asked[0] != "/api/v3/repos/o/n/actions/workflows/range.yml/runs" {
		t.Fatalf("the last green look asked %v", asked)
	}
}

func TestNewestRunReadsTheBranchsNewestRun(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.RequestURI())
		switch {
		case strings.Contains(r.URL.Path, "/workflows/"):
			w.Write([]byte(`{"workflow_runs":[
				{"id":3,"html_url":"red","status":"completed","conclusion":"failure","created_at":"2026-10-04T11:44:00Z"},
				{"id":2,"html_url":"green","status":"completed","conclusion":"success","created_at":"2026-10-03T11:44:00Z"}]}`))
		case strings.HasSuffix(r.URL.Path, "/runs/3/jobs"):
			w.Write([]byte(`{"jobs":[{"name":"nightly","status":"completed","conclusion":"failure"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()

	run, found, err := GitHub{Repository: "o/n", Token: "t", Workflow: "nightly.yml", Branch: "main"}.NewestRun()
	if err != nil || !found {
		t.Fatalf("found %v, err %v", found, err)
	}
	if run.URL != "red" || run.Conclusion != "failure" || len(run.Jobs) != 1 || run.Jobs[0].Name != "nightly" {
		t.Fatalf("read %+v", run)
	}
	if !strings.Contains(asked[0], "/repos/o/n/actions/workflows/nightly.yml/runs?branch=main") {
		t.Fatalf("asked %v", asked)
	}
}

// ci.watch's github provider asks the API GITHUB_API_URL names, as ci.range's
// does (bug 23): GitHub Actions sets it, a GitHub Enterprise server's own.
func TestTheGitHubWatchAsksGitHubAPIURL(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Write([]byte(`{"workflow_runs":[]}`))
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = "http://127.0.0.1:1/not-asked"
	defer func() { GitHubAPI = was }()
	env := map[string]string{"GITHUB_TOKEN": "t", "GITHUB_REPOSITORY": "o/n", "GITHUB_API_URL": server.URL + "/api/v3"}
	watch, ok, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: func(name string) string { return env[name] }})
	if !ok || err != nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	if _, found, err := watch.Look("abc"); found || err != nil {
		t.Fatalf("found %v, err %v", found, err)
	}
	if len(asked) != 1 || asked[0] != "/api/v3/repos/o/n/actions/workflows/ci.yml/runs" {
		t.Fatalf("asked %v", asked)
	}
}

// GitHub answers its rate limit 403 or 429 (bug 40); a 403 is one only when
// a header or its message says so, and the wait is retry-after's, else the
// reset's once none remain.
func TestARateLimitIsToldFromARefusal(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	header := func(kv ...string) http.Header {
		h := http.Header{}
		for i := 0; i < len(kv); i += 2 {
			h.Set(kv[i], kv[i+1])
		}
		return h
	}
	for _, c := range []struct {
		name    string
		status  int
		h       http.Header
		body    string
		limited bool
		wait    time.Duration
	}{
		{"none remain, reset in 30s", 403, header("X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "1000030"), "", true, 30 * time.Second},
		{"a reset passed", 403, header("X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "999990"), "", true, 0},
		{"retry-after wins", 403, header("Retry-After", "7", "X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "1003600"), "", true, 7 * time.Second},
		{"retry-after as a date", 403, header("Retry-After", now.Add(time.Minute).UTC().Format(http.TimeFormat)), "", true, time.Minute},
		{"a secondary limit's message", 403, header("X-RateLimit-Remaining", "4000", "X-RateLimit-Reset", "1003600"),
			`{"message":"You have exceeded a secondary rate limit."}`, true, 0},
		{"a 429", 429, header(), "", true, 0},
		{"a refused token", 403, header("X-RateLimit-Remaining", "4999"), `{"message":"Resource not accessible by integration"}`, false, 0},
		{"a 401", 401, header("X-RateLimit-Remaining", "0"), "", false, 0},
	} {
		limited, wait := rateLimit(c.status, c.h, []byte(c.body), now)
		if limited != c.limited || wait != c.wait {
			t.Errorf("%s: limited %v, wait %s; want %v, %s", c.name, limited, wait, c.limited, c.wait)
		}
	}
}

func TestARateLimitedLookIsTemporaryAndSaysItsWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "5")
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()
	_, _, err := GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}.RunOf("abc")
	if kind.Of(err) != kind.Temporary || Wait(err) != 5*time.Second {
		t.Fatalf("a rate limit is %v, wait %s", err, Wait(err))
	}
}
