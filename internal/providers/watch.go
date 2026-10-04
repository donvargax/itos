package providers

// The ci.watch providers (slice 51, features/watch.feature): one look at the
// CI run of a commit, which itos push and itos ci watch repeat every
// ci.watch.interval seconds until the run completes. github reads the run of
// ci.watch.github.workflow through GitHub's API, as the range provider beside
// it reads the last green one; command runs ci.watch.command and reads the run
// from its stdout. The same provider gives itos status one look at the last
// nightly's run (slice 72): github reads ci.watch.github.nightly_workflow's
// newest run on the branch, command runs ci.watch.nightly_command, given no
// commit, and reads its stdout as ci.watch.command's.

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
	"slices"
	"sort"
	"strings"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/shell"
	"github.com/donvargax/itos/v3/internal/tests"
	"github.com/donvargax/itos/v3/internal/value"
)

// Run is one look at a CI run: its address, its status (queued,
// in_progress or completed), its conclusion once completed (success,
// failure, cancelled…), and its jobs, each the same.
type Run struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	Jobs       []Job  `json:"jobs"`
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
// is Transient, which the next look may not meet.
type Watch func(sha string) (run Run, found bool, err error)

// Transient is a look that failed in a way the next may not (no network, a
// server error, a rate limit): the watch goes on, and names it if the
// timeout comes first.
type Transient struct{ Err error }

func (t Transient) Error() string { return t.Err.Error() }

// WatchSetup is what the github provider needs from outside the config: the
// environment, the URL of the remote the commit was pushed to, and how to
// ask gh for its token.
type WatchSetup struct {
	Env       Env
	RemoteURL string
	GhToken   func() (string, error)
	Stderr    io.Writer
}

// WatchProvider is the provider ci.watch names; ok is false for none, which
// watches nothing. An error is a provider that cannot look at all, said
// before any request: github with no token, or no repository to ask about.
func WatchProvider(cfg *config.Loaded, s WatchSetup) (watch Watch, ok bool, err error) {
	w := cfg.CI.Watch
	switch w.Provider {
	case "none":
		return nil, false, nil
	case "command":
		command := ""
		if w.Command != nil {
			command = *w.Command
		}
		return commandWatch(cfg, command, s.Stderr), true, nil
	}
	g, err := watchGitHub(cfg, "ci.watch", s)
	if err != nil {
		return nil, false, err
	}
	g.Workflow = w.GitHub.Workflow
	return g.RunOf, true, nil
}

// Nightly looks once at the last nightly's run: found is false when there
// is none yet.
type Nightly func() (run Run, found bool, err error)

// NightlyProvider is the look at the last nightly's run ci.watch's provider
// gives, on the branch: ok is false when the provider names no nightly (none,
// github with no ci.watch.github.nightly_workflow, command with no
// ci.watch.nightly_command). An error is a provider that cannot look, as
// WatchProvider's.
func NightlyProvider(cfg *config.Loaded, branch string, s WatchSetup) (nightly Nightly, ok bool, err error) {
	w := cfg.CI.Watch
	switch w.Provider {
	case "none":
		return nil, false, nil
	case "command":
		if w.NightlyCommand == nil || value.Trim(*w.NightlyCommand) == "" {
			return nil, false, nil
		}
		command := *w.NightlyCommand
		return func() (Run, bool, error) {
			run, err := commandRun(cfg, "ci.watch.nightly_command", command, s.Stderr)
			return run, err == nil, err
		}, true, nil
	}
	if w.GitHub.NightlyWorkflow == "" {
		return nil, false, nil
	}
	g, err := watchGitHub(cfg, "ci.watch", s)
	if err != nil {
		return nil, false, err
	}
	g.Workflow, g.Branch = w.GitHub.NightlyWorkflow, branch
	return g.NewestRun, true, nil
}

// watchGitHub is the github provider's repository and token, the workflow
// left to the caller: the token from ci.range.github.token_env, else gh,
// the repository from ci.range.github.repository_env, else the remote's URL.
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
	return GitHub{Repository: repository, Token: token}, nil
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

// commandWatch runs the command once a look, {sha} filled in as one shell
// word, and reads the run from its stdout: one JSON object, Run's shape. A
// command that fails or prints anything else is an error.
func commandWatch(cfg *config.Loaded, command string, stderr io.Writer) Watch {
	return func(sha string) (Run, bool, error) {
		filled := strings.ReplaceAll(command, "{sha}", tests.ShellWord(sha))
		run, err := commandRun(cfg, "ci.watch.command", filled, stderr)
		return run, err == nil, err
	}
}

// commandRun runs the command, the config key named in its errors, and
// reads the run from its stdout.
func commandRun(cfg *config.Loaded, key, command string, stderr io.Writer) (Run, error) {
	var stdout bytes.Buffer
	res := shell.Run(cfg, command, shell.Options{Stdout: &stdout, Stderr: stderr, Timeout: Timeout})
	if !res.OK() {
		return Run{}, fmt.Errorf("%s failed (exit %s): %s", key, res.Status(), command)
	}
	run, err := ReadRun(stdout.Bytes())
	if err != nil {
		return Run{}, fmt.Errorf("%s printed no run: %s", key, err)
	}
	return run, nil
}

// statuses are the statuses a run or a job may have.
var statuses = []string{"queued", "in_progress", "completed"}

// ReadRun is the run a command printed: one JSON object with a status of
// statuses, a conclusion when it is completed, a url and jobs, each with a
// name and a status, and their conclusion when completed.
func ReadRun(text []byte) (Run, error) {
	var run Run
	dec := json.NewDecoder(bytes.NewReader(text))
	if err := dec.Decode(&run); err != nil {
		return Run{}, fmt.Errorf("not one JSON object of {url, status, conclusion, jobs}: %s", err)
	}
	if dec.More() {
		return Run{}, errors.New("more than one JSON value")
	}
	check := func(what, status, conclusion string) error {
		if !slices.Contains(statuses, status) {
			return fmt.Errorf("%s's status %q is none of queued, in_progress or completed", what, status)
		}
		if status == "completed" && conclusion == "" {
			return fmt.Errorf("%s is completed with no conclusion", what)
		}
		return nil
	}
	if err := check("the run", run.Status, run.Conclusion); err != nil {
		return Run{}, err
	}
	for i, j := range run.Jobs {
		if j.Name == "" {
			return Run{}, fmt.Errorf("job %d has no name", i+1)
		}
		if err := check("the job "+j.Name, j.Status, j.Conclusion); err != nil {
			return Run{}, err
		}
	}
	return run, nil
}

// RunOf is the workflow's newest run for the commit, with its jobs, read
// from the GitHub API with the token: found is false while GitHub has none,
// as just after a push. A server error, a rate limit or no network is
// Transient; any other refusal (a bad token, no such workflow) ends the
// watch.
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

// newestOf is the newest of the runs the API path lists, with its jobs.
func (g GitHub) newestOf(path string) (Run, bool, error) {
	var runs struct {
		WorkflowRuns []struct {
			ID         int64   `json:"id"`
			HTMLURL    string  `json:"html_url"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
			CreatedAt  string  `json:"created_at"`
		} `json:"workflow_runs"`
	}
	if err := g.get(path, &runs); err != nil {
		return Run{}, false, err
	}
	if len(runs.WorkflowRuns) == 0 {
		return Run{}, false, nil
	}
	found := runs.WorkflowRuns
	sort.SliceStable(found, func(i, j int) bool { return found[i].CreatedAt > found[j].CreatedAt })
	newest := found[0]
	run := Run{URL: newest.HTMLURL, Status: newest.Status, Conclusion: deref(newest.Conclusion)}
	var jobs struct {
		Jobs []struct {
			Name       string  `json:"name"`
			Status     string  `json:"status"`
			Conclusion *string `json:"conclusion"`
		} `json:"jobs"`
	}
	if err := g.get(fmt.Sprintf("/repos/%s/actions/runs/%d/jobs?per_page=100", g.Repository, newest.ID), &jobs); err != nil {
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

// get reads one GitHub API path into into.
func (g GitHub) get(path string, into any) error {
	req, err := http.NewRequest(http.MethodGet, GitHubAPI+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	res, err := (&http.Client{Timeout: Timeout}).Do(req)
	if err != nil {
		return Transient{fmt.Errorf("GitHub's API did not answer: %w", err)}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		err := fmt.Errorf("GitHub's API answered %s for %s", res.Status, path)
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			return Transient{err}
		}
		return err
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		return Transient{fmt.Errorf("GitHub's API answered what is not JSON for %s: %w", path, err)}
	}
	return nil
}
