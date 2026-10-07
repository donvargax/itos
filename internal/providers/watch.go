package providers

// The ci.watch providers (slice 51, features/watch.feature): one look at the
// CI run of a commit, which itos push and itos ci watch repeat every
// ci.watch.interval seconds until the run completes. github reads the run of
// ci.watch.github.workflow through GitHub's API, as the range provider beside
// it reads the last green one. The same provider gives itos status one look at
// the last nightly's run (slice 72): ci.watch.github.nightly_workflow's newest
// run on the branch. The command provider, which ran ci.watch.command and
// ci.watch.nightly_command, a repository's own commands, on a push and on
// itos go, was removed in v5.0.0 (slice 85). A run a newer push cancelled is
// no verdict (bug 41): the watch finds the newer run among the workflow's
// runs on the branch (Watcher.Runs) and follows it.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/kind"
	"github.com/donvargax/itos/v6/internal/value"
)

// Run is one look at a CI run: its address, its status (queued,
// in_progress or completed), its conclusion once completed (success,
// failure, cancelled…), and its jobs, each the same. Its ID, the commit it
// ran on, its branch and when it was made say which run it is, for finding
// the run that superseded a cancelled one; --json leaves them out.
type Run struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	Jobs       []Job  `json:"jobs"`
	ID         int64  `json:"-"`
	HeadSHA    string `json:"-"`
	Branch     string `json:"-"`
	Created    string `json:"-"`
}

// Job is one job of a run, as Run is.
type Job struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
}

// Done is whether the run has finished with a conclusion recorded: GitHub
// can say completed a moment before it records the conclusion, and a blank
// one is no result yet.
func (r Run) Done() bool { return r.Status == "completed" && r.Conclusion != "" }

// Done is whether the job has finished with a conclusion recorded.
func (j Job) Done() bool { return j.Status == "completed" && j.Conclusion != "" }

// Watch looks once at the run of a commit, given by its full SHA: found is
// false while there is none yet. An error ends the watch (exit 3), unless it
// is of kind.Temporary, a look that failed in a way the next may not (no
// network, a server error, a rate limit): the watch goes on, until it gives
// up (exit 75).
type Watch func(sha string) (run Run, found bool, err error)

// Runs lists the workflow's runs on a branch, newest first, their jobs left
// out: where a watch looks for the run that superseded a cancelled one. An
// error is as Watch's.
type Runs func(branch string) ([]Run, error)

// Watcher is ci.watch's provider: Look at a commit's run, and the Runs on a
// branch.
type Watcher struct {
	Look Watch
	Runs Runs
}

// WatchSetup is what the github provider needs from outside the config: the
// environment, the URL of the remote the commit was pushed to, and how to
// ask gh for its token.
type WatchSetup struct {
	Env       Env
	RemoteURL string
	GhToken   func() (string, error)
}

// WatchProvider is the provider ci.watch names; ok is false for none, which
// watches nothing. An error is a provider that cannot look at all, said
// before any request: github with no token, or no repository to ask about.
func WatchProvider(cfg *config.Loaded, s WatchSetup) (watch Watcher, ok bool, err error) {
	w := cfg.CI.Watch
	if w.Provider == "none" {
		return Watcher{}, false, nil
	}
	g, err := watchGitHub(cfg, "ci.watch", s)
	if err != nil {
		return Watcher{}, false, err
	}
	g.Workflow = w.GitHub.Workflow
	return Watcher{Look: g.RunOf, Runs: g.RunsOn}, true, nil
}

// Nightly looks once at the last nightly's run: found is false when there
// is none yet.
type Nightly func() (run Run, found bool, err error)

// NightlyProvider is the look at the last nightly's run ci.watch's provider
// gives, on the branch: ok is false when the provider names no nightly (none,
// or github with no ci.watch.github.nightly_workflow). An error is a provider
// that cannot look, as WatchProvider's.
func NightlyProvider(cfg *config.Loaded, branch string, s WatchSetup) (nightly Nightly, ok bool, err error) {
	w := cfg.CI.Watch
	if w.Provider == "none" || w.GitHub.NightlyWorkflow == "" {
		return nil, false, nil
	}
	g, err := watchGitHub(cfg, "ci.watch", s)
	if err != nil {
		return nil, false, err
	}
	g.Workflow, g.Branch = w.GitHub.NightlyWorkflow, branch
	return g.NewestRun, true, nil
}

// watchGitHub is the github provider's repository, token and API, the
// workflow left to the caller: the token from ci.range.github.token_env, else
// gh, the repository from ci.range.github.repository_env, else the remote's
// URL, the API's address from GITHUB_API_URL, else GitHubAPI. Every look
// status and the watch take at GitHub asks there (bug 24).
// key is the config key whose provider asks, named in its errors.
func watchGitHub(cfg *config.Loaded, key string, s WatchSetup) (GitHub, error) {
	r := cfg.CI.Range.GitHub
	token := firstSet(s.Env, r.TokenEnv)
	if token == "" && s.GhToken != nil {
		token, _ = s.GhToken()
	}
	if token == "" {
		return GitHub{}, fmt.Errorf("%s's github provider needs a token, and there is none: set %s (ci.range.github.token_env), or install gh and run gh auth login",
			key, orList(r.TokenEnv))
	}
	repository := s.Env(r.RepositoryEnv)
	if repository == "" {
		repository = GitHubRepository(s.RemoteURL)
	}
	if repository == "" {
		return GitHub{}, fmt.Errorf("%s's github provider cannot tell the GitHub repository from the remote's URL %q: set %s to owner/name",
			key, s.RemoteURL, r.RepositoryEnv)
	}
	return GitHub{Repository: repository, Token: token, API: s.Env(APIEnv)}, nil
}

// orList is names joined as a sentence lists alternatives: A, B or C.
func orList(names []string) string {
	switch len(names) {
	case 0:
		return "a token variable"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// GhToken is the token `gh auth token` prints, trimmed, and the error of a
// gh that is missing or signed out.
func GhToken() (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("gh", "auth", "token")
	cmd.Stdout = &stdout
	err := cmd.Run()
	return value.Trim(stdout.String()), err
}

// gitHubRemote is the owner and name in a GitHub remote's URL: https://,
// ssh:// or scp-like git@github.com:owner/name, .git or not.
var gitHubRemote = regexp.MustCompile(`^(?:[a-z+]+://)?(?:[^@/]+@)?github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$`)

// GitHubRepository is the repository ("owner/name") a GitHub remote's URL
// names, "" for one that is not GitHub's.
func GitHubRepository(remoteURL string) string {
	m := gitHubRemote.FindStringSubmatch(strings.TrimSpace(remoteURL))
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// RunOf is the workflow's newest run for the commit, with its jobs, read
// from the GitHub API with the token: found is false while GitHub has none,
// as just after a push. A server error, a rate limit or no network is
// kind.Temporary, a rate limit a RateLimited; any other refusal (a bad token,
// no such workflow) ends the watch.
func (g GitHub) RunOf(sha string) (Run, bool, error) {
	return g.newestOf(fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?head_sha=%s&per_page=20",
		g.Repository, url.PathEscape(g.Workflow), url.QueryEscape(sha)))
}

// NewestRun is the workflow's newest run on the branch, with its jobs, as
// RunOf reads a commit's: found is false when the workflow has never run
// there.
func (g GitHub) NewestRun() (Run, bool, error) {
	return g.newestOf(fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?branch=%s&per_page=20",
		g.Repository, url.PathEscape(g.Workflow), url.QueryEscape(g.Branch)))
}

// RunsOn is the workflow's runs on the branch, newest first, without their
// jobs: the last 20, enough to hold the push that superseded a cancelled run.
func (g GitHub) RunsOn(branch string) ([]Run, error) {
	return g.list(fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?branch=%s&per_page=20",
		g.Repository, url.PathEscape(g.Workflow), url.QueryEscape(branch)))
}

// list is the runs the API path lists, newest first by created_at, without
// their jobs.
func (g GitHub) list(path string) ([]Run, error) {
	var runs struct {
		WorkflowRuns []struct {
			ID         int64   `json:"id"`
			HTMLURL    string  `json:"html_url"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
			CreatedAt  string  `json:"created_at"`
			HeadSHA    string  `json:"head_sha"`
			HeadBranch string  `json:"head_branch"`
		} `json:"workflow_runs"`
	}
	if err := g.get(path, &runs); err != nil {
		return nil, err
	}
	found := runs.WorkflowRuns
	sort.SliceStable(found, func(i, j int) bool { return found[i].CreatedAt > found[j].CreatedAt })
	listed := make([]Run, 0, len(found))
	for _, r := range found {
		listed = append(listed, Run{URL: r.HTMLURL, Status: r.Status, Conclusion: deref(r.Conclusion),
			ID: r.ID, HeadSHA: r.HeadSHA, Branch: r.HeadBranch, Created: r.CreatedAt})
	}
	return listed, nil
}

// newestOf is the newest of the runs the API path lists, with its jobs.
func (g GitHub) newestOf(path string) (Run, bool, error) {
	found, err := g.list(path)
	if err != nil {
		return Run{}, false, err
	}
	if len(found) == 0 {
		return Run{}, false, nil
	}
	run := found[0]
	var jobs struct {
		Jobs []struct {
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := g.get(fmt.Sprintf("/repos/%s/actions/runs/%d/jobs?per_page=100", g.Repository, run.ID), &jobs); err != nil {
		return Run{}, false, err
	}
	for _, j := range jobs.Jobs {
		run.Jobs = append(run.Jobs, Job{Name: j.Name, Status: j.Status, Conclusion: deref(j.Conclusion)})
	}
	return run, true, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// get reads one GitHub API path into into, with the token when there is one.
func (g GitHub) get(path string, into any) error {
	req, err := http.NewRequest(http.MethodGet, g.api()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	res, err := (&http.Client{Timeout: Timeout}).Do(req)
	if err != nil {
		return kind.Wrap(kind.Temporary, fmt.Errorf("GitHub's API did not answer: %w", err))
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		err := fmt.Errorf("GitHub's API answered %s for %s", res.Status, path)
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		if limited, wait := rateLimit(res.StatusCode, res.Header, body, time.Now()); limited {
			return kind.Wrap(kind.Temporary, &RateLimited{Wait: wait, Err: fmt.Errorf("%w: its rate limit", err)})
		}
		if res.StatusCode >= 500 {
			return kind.Wrap(kind.Temporary, err)
		}
		return err
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		return kind.Wrap(kind.Temporary, fmt.Errorf("GitHub's API answered what is not JSON for %s: %w", path, err))
	}
	return nil
}

// RateLimited is a request GitHub refused for its rate limit (bug 40), and
// Wait, how long it asked to be left before the next: 0 when it said nothing.
// get wraps it as kind.Temporary.
type RateLimited struct {
	Wait time.Duration
	Err  error
}

func (e *RateLimited) Error() string { return e.Err.Error() }

func (e *RateLimited) Unwrap() error { return e.Err }

// Wait is how long err asks a watch to wait before it looks again: a rate
// limit's, else 0.
func Wait(err error) time.Duration {
	var limited *RateLimited
	if errors.As(err, &limited) {
		return limited.Wait
	}
	return 0
}

// rateLimit is whether an answer refusing a request is GitHub's rate limit,
// and how long it asks to be left, at now: a 429 always, and a 403 that says
// so, as GitHub's primary limit and often its secondary one answer, with
// x-ratelimit-remaining 0, a retry-after header, or a message naming the rate
// limit. The wait is retry-after's seconds (or its date), else the time to
// x-ratelimit-reset once x-ratelimit-remaining is 0, the primary limit's
// window; else 0, never below. Any other 403 is a refusal, not a limit.
func rateLimit(status int, h http.Header, body []byte, now time.Time) (bool, time.Duration) {
	retryAfter := strings.TrimSpace(h.Get("Retry-After"))
	exhausted := strings.TrimSpace(h.Get("X-RateLimit-Remaining")) == "0"
	switch {
	case status == http.StatusTooManyRequests:
	case status == http.StatusForbidden && (exhausted || retryAfter != "" ||
		strings.Contains(strings.ToLower(string(body)), "rate limit")):
	default:
		return false, 0
	}
	var wait time.Duration
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil {
		wait = time.Duration(seconds) * time.Second
	} else if at, err := http.ParseTime(retryAfter); err == nil {
		wait = at.Sub(now)
	} else if reset, err := strconv.ParseInt(strings.TrimSpace(h.Get("X-RateLimit-Reset")), 10, 64); err == nil && exhausted {
		wait = time.Unix(reset, 0).Sub(now)
	}
	return true, max(wait, 0)
}
