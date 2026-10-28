package providers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
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
	cfg := &config.Loaded{VCS: vcs.Git{}}
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
				{"id":2,"html_url":"new","status":"in_progress","conclusion":null,"created_at":"2026-10-03T11:00:00Z","run_started_at":"2026-10-03T11:05:00Z"}]}`))
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
	if run.Started != "2026-10-03T11:05:00Z" || run.Created != "2026-10-03T11:00:00Z" {
		t.Fatalf("the run's times: %+v", run)
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

// The repository's visibility is its endpoint's visibility, else its private
// bool, asked at GITHUB_API_URL with the token; an answer naming neither, or
// a refusal, tells none.
func TestVisibilityReadsTheRepository(t *testing.T) {
	answer, status := "", http.StatusOK
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+" "+r.Header.Get("Authorization"))
		w.WriteHeader(status)
		w.Write([]byte(answer))
	}))
	defer server.Close()
	env := map[string]string{"GITHUB_TOKEN": "t", "GITHUB_REPOSITORY": "o/n", "GITHUB_API_URL": server.URL + "/api/v3"}
	watch, ok, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: func(name string) string { return env[name] }})
	if !ok || err != nil || watch.Visibility == nil {
		t.Fatalf("ok %v, err %v", ok, err)
	}
	for body, want := range map[string]string{
		`{"visibility":"internal","private":true}`: "internal",
		`{"visibility":"public","private":true}`:   "public",
		`{"private":true}`:                         "private",
		`{"private":false}`:                        "public",
	} {
		answer = body
		if got, err := watch.Visibility(); got != want || err != nil {
			t.Errorf("%s: %q, %v", body, got, err)
		}
	}
	if asked[0] != "/api/v3/repos/o/n Bearer t" {
		t.Errorf("asked %v", asked)
	}
	answer = `{"full_name":"o/n"}`
	if got, err := watch.Visibility(); got != "" || err == nil || !strings.Contains(err.Error(), "o/n") {
		t.Errorf("no visibility named: %q, %v", got, err)
	}
	answer, status = `{"message":"Not Found"}`, http.StatusNotFound
	if got, err := watch.Visibility(); got != "" || err == nil {
		t.Errorf("a 404: %q, %v", got, err)
	}
}

func TestRunOfReadsEachJobsIDAndItsSteps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/workflows/"):
			w.Write([]byte(`{"workflow_runs":[{"id":2,"html_url":"new","status":"completed","conclusion":"failure"}]}`))
		default:
			w.Write([]byte(`{"jobs":[
				{"id":77,"name":"ci","status":"completed","conclusion":"failure","steps":[
					{"name":"Set up job","conclusion":"success"},
					{"name":"Run the unit tests","conclusion":"failure"},
					{"name":"Complete job","conclusion":null}]},
				{"id":78,"name":"platform","status":"completed","conclusion":"success"}]}`))
		}
	}))
	defer server.Close()
	was := GitHubAPI
	GitHubAPI = server.URL
	defer func() { GitHubAPI = was }()

	run, _, err := GitHub{Repository: "o/n", Token: "t", Workflow: "ci.yml"}.RunOf("abc")
	if err != nil || len(run.Jobs) != 2 {
		t.Fatalf("read %+v, err %v", run, err)
	}
	ci, platform := run.Jobs[0], run.Jobs[1]
	if ci.ID != 77 || platform.ID != 78 || len(ci.Steps) != 3 || ci.Steps[1] != (Step{Name: "Run the unit tests", Conclusion: "failure"}) ||
		ci.Steps[2].Conclusion != "" || platform.Steps != nil {
		t.Fatalf("read %+v", run.Jobs)
	}
}

func TestAJobsFailedStepIsTheFirstThatConcludedNeitherSuccessNorSkipped(t *testing.T) {
	for name, tc := range map[string]struct {
		steps []Step
		want  string
	}{
		"none listed":           {nil, ""},
		"all succeeded":         {[]Step{{"a", "success"}, {"b", "success"}}, ""},
		"a failure":             {[]Step{{"a", "success"}, {"b", "failure"}, {"c", "skipped"}}, "b"},
		"the first of two":      {[]Step{{"a", "failure"}, {"b", "cancelled"}}, "a"},
		"skipped is passed":     {[]Step{{"a", "skipped"}, {"b", "cancelled"}}, "b"},
		"a timeout ends cancel": {[]Step{{"a", "success"}, {"b", "cancelled"}}, "b"},
		"no conclusion yet":     {[]Step{{"a", ""}, {"b", "failure"}}, "b"},
		"only unconcluded":      {[]Step{{"a", ""}}, ""},
	} {
		if got := (Job{Steps: tc.steps}).FailedStep(); got != tc.want {
			t.Errorf("%s: %q, not %q", name, got, tc.want)
		}
	}
}

// logServer answers a job's log with a redirect to a path that serves the
// text, as GitHub answers with a redirect to a short-lived address.
func logServer(t *testing.T, serve http.HandlerFunc) *GitHub {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/n/actions/jobs/9/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t" || r.Header.Get("Accept") != "application/vnd.github+json" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/download/9", http.StatusFound)
	})
	mux.HandleFunc("/download/9", serve)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return &GitHub{Repository: "o/n", Token: "t", API: server.URL}
}

func TestJobLogFollowsGitHubsRedirectToTheText(t *testing.T) {
	g := logServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("2026-10-23T10:00:00.1234567Z one\r\n2026-10-23T10:00:00.1234568Z two\r\n"))
	})
	text, err := g.JobLog(9)
	if err != nil || text != "2026-10-23T10:00:00.1234567Z one\r\n2026-10-23T10:00:00.1234568Z two\r\n" {
		t.Fatalf("read %q, err %v", text, err)
	}
}

func TestJobLogIsAnErrorForAnythingButTheText(t *testing.T) {
	for name, serve := range map[string]http.HandlerFunc{
		"past its retention":   func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusGone) },
		"a redirect to a 404":  func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"a redirect to itself": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/download/9", http.StatusFound) },
		"an answer cut short": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("cut"))
		},
	} {
		if text, err := logServer(t, serve).JobLog(9); err == nil {
			t.Errorf("%s: read %q", name, text)
		}
	}
	if _, err := logServer(t, func(http.ResponseWriter, *http.Request) {}).JobLog(8); err == nil {
		t.Error("a job GitHub does not know: no error")
	}
}

func TestJobLogRefusesALogBeyondTheLimit(t *testing.T) {
	was := LogLimit
	LogLimit = 5
	defer func() { LogLimit = was }()
	g := logServer(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("12345")) })
	if text, err := g.JobLog(9); err != nil || text != "12345" {
		t.Fatalf("a log of the limit's size: %q, %v", text, err)
	}
	g = logServer(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("123456")) })
	if text, err := g.JobLog(9); err == nil {
		t.Fatalf("a log beyond the limit: %q", text)
	}
}

func TestJobLogIsAnErrorWhenGitHubCannotBeAsked(t *testing.T) {
	if _, err := (GitHub{Repository: "o/n", API: "http://[::1"}).JobLog(9); err == nil {
		t.Error("an address that is none: no error")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	if _, err := (GitHub{Repository: "o/n", API: server.URL}).JobLog(9); err == nil {
		t.Error("no network: no error")
	}
}

func TestALookIsAnErrorWhenGitHubCannotBeAsked(t *testing.T) {
	if _, _, err := (GitHub{Repository: "o/n", Workflow: "ci.yml", API: "http://[::1"}).RunOf("abc"); err == nil || kind.Of(err) == kind.Temporary {
		t.Errorf("an address that is none: %v", err)
	}
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	if _, _, err := (GitHub{Repository: "o/n", Workflow: "ci.yml", API: server.URL}).RunOf("abc"); kind.Of(err) != kind.Temporary {
		t.Errorf("no network: %v", err)
	}
}

func TestTheGitHubWatcherReadsJobLogs(t *testing.T) {
	env := func(name string) string {
		return map[string]string{"GITHUB_TOKEN": "t", "GITHUB_REPOSITORY": "o/n"}[name]
	}
	watcher, ok, err := WatchProvider(githubWatchConfig(), WatchSetup{Env: env})
	if !ok || err != nil || watcher.Log == nil || watcher.Annotations == nil {
		t.Fatalf("ok %v, err %v, log %v, annotations %v", ok, err, watcher.Log, watcher.Annotations)
	}
}

func TestJobAnnotationsAsksOnePageOf100AndKeepsWhatItNeeds(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.RequestURI()+" "+r.Header.Get("Authorization"))
		w.Write([]byte(`[{"path":"a_test.go","start_line":7,"end_line":9,"annotation_level":"failure","title":"t","message":"one\ntwo"},
			{"path":".github","start_line":1,"annotation_level":"warning","message":null}]`))
	}))
	t.Cleanup(server.Close)
	got, err := (GitHub{Repository: "o/n", Token: "t", API: server.URL}).JobAnnotations(9)
	want := []Annotation{{Path: "a_test.go", Line: 7, Level: "failure", Message: "one\ntwo"}, {Path: ".github", Line: 1, Level: "warning"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("read %+v, err %v", got, err)
	}
	if !reflect.DeepEqual(asked, []string{"/repos/o/n/check-runs/9/annotations?per_page=100 Bearer t"}) {
		t.Fatalf("asked %q", asked)
	}
}

func TestJobAnnotationsIsAnErrorForAnythingButTheList(t *testing.T) {
	for name, serve := range map[string]http.HandlerFunc{
		"a refusal":      func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) },
		"not a list":     func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"message":"x"}`)) },
		"a server error": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
	} {
		server := httptest.NewServer(serve)
		got, err := (GitHub{Repository: "o/n", API: server.URL}).JobAnnotations(9)
		server.Close()
		if err == nil || got != nil {
			t.Errorf("%s: read %+v, err %v", name, got, err)
		}
	}
}
