package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/donvargax/itos/v4/internal/config"
	"github.com/donvargax/itos/v4/internal/git"
	"github.com/donvargax/itos/v4/internal/shell"
	"github.com/donvargax/itos/v4/internal/value"
)

// Range is a ci.range provider: the start commit it proposes for a push's
// range to the head, before RangeStart holds it to the head's ancestry. ""
// means "run everything", and so does every way a lookup can fail: a provider
// never errors, since a range that cannot be found is the whole history.
type Range func(head string) string

// Env is how a provider reads the environment (os.Getenv in the binary).
type Env func(string) string

// RangeProvider is the provider itos.yaml's ci.range names
// (providers.ts's rangeProvider): `none`, `command` (its stdout's first
// line, through the config's shell, its stderr on stderr) or `github` (the
// head's nearest first parent with a green run of the workflow, asking the
// API at GITHUB_API_URL). Each is a function of the config's ci.range and the
// environment, so a provider over another forge's API is one more case beside
// github's.
func RangeProvider(cfg *config.Loaded, env Env, stderr io.Writer) Range {
	r := cfg.CI.Range
	switch r.Provider {
	case "none":
		return func(string) string { return "" }
	case "command":
		command := ""
		if r.Command != nil {
			command = *r.Command
		}
		return func(string) string { return FirstLine(cfg, command, stderr) }
	}
	return GitHub{
		Repository: env(r.GitHub.RepositoryEnv),
		Token:      firstSet(env, r.GitHub.TokenEnv),
		Workflow:   r.GitHub.Workflow,
		Branch:     r.GitHub.Branch,
		API:        env(APIEnv),
	}.NearestGreen
}

// firstSet is the first of the named variables that is set and not empty.
func firstSet(env Env, names []string) string {
	for _, name := range names {
		if v := env(name); v != "" {
			return v
		}
	}
	return ""
}

// FirstLine is a command run through the config's shell, its stdin closed
// and its stderr on stderr, and the first line of its output, trimmed as
// JavaScript trims; "" when it fails or prints nothing. The command
// providers, ci.range's and work.identity's, read their answer this way.
func FirstLine(cfg *config.Loaded, command string, stderr io.Writer) string {
	var stdout bytes.Buffer
	if !shell.Run(cfg, command, shell.Options{Stdout: &stdout, Stderr: stderr}).OK() {
		return ""
	}
	line, _, _ := strings.Cut(value.Trim(stdout.String()), "\n")
	return value.Trim(line)
}

// RangeStart is where a CI run's range starts (ci-scope.ts's rangeStart). A
// newer push cancels a waiting run, so a push to main is checked from the
// head of the last green run, not from the push before it: the newest run
// covers every cancelled one's commits. A pull request keeps its base, and
// the provider is not asked. With no start, or one that is not an ancestor
// of the head (a rewritten history, a commit the repository does not have),
// the start is empty, and empty runs everything.
func RangeStart(head, base string, provider Range) string {
	if base != "" {
		return base
	}
	green := provider(head)
	if green == "" || head == "" {
		return ""
	}
	if git.Succeeds("merge-base", "--is-ancestor", green, head) {
		return green
	}
	return ""
}

// GitHubAPI is where the github provider asks when it is given no address of
// its own (GitHub.API); a test points it elsewhere.
var GitHubAPI = "https://api.github.com"

// APIEnv is the variable that gives the GitHub API's address, as GitHub
// Actions sets it, a GitHub Enterprise server's own in its runs: ci.range's
// and ci.watch's github providers ask there when it is set.
const APIEnv = "GITHUB_API_URL"

// FirstParentsAsked is how many of the head's first parents the github range
// provider asks about, nearest first, before it gives up and runs everything;
// a test lowers it.
var FirstParentsAsked = 100

// Timeout is how long the github provider waits for the API. Node's fetch
// waits however long it takes; past this the lookup fails, which runs
// everything, rather than holding the CI run until its job's own timeout.
var Timeout = time.Minute

// GitHub is the github provider's question: the workflow's runs on a branch
// of a repository ("owner/name"), asked with a token when there is one, of the
// API at API, else at GitHubAPI.
type GitHub struct {
	Repository, Token, Workflow, Branch string
	API                                 string
}

// api is the address the provider asks, with no slash at its end.
func (g GitHub) api() string {
	if g.API != "" {
		return strings.TrimRight(g.API, "/")
	}
	return GitHubAPI
}

// NearestGreen is where a push's range to the head starts (bug 23): the
// nearest of the head's first parents, from its parent, with a successful run
// of the workflow on the branch, each asked about by its own runs. The head's
// own run is the one asking, and a run that failed, was cancelled or is still
// going is passed over. The branch's list of runs is not read: GitHub served
// it stale on 2026-10-05, naming a run a day old as the newest green one, and
// the range then reached back past commits already proved. Past
// FirstParentsAsked first parents with none green, and on anything that goes
// wrong, there is no start, which runs everything; with no repository nobody
// is asked.
func (g GitHub) NearestGreen(head string) string {
	if g.Repository == "" || head == "" {
		return ""
	}
	parents, err := git.Lines("rev-list", "--first-parent", "--skip=1",
		fmt.Sprintf("--max-count=%d", FirstParentsAsked), head, "--")
	if err != nil {
		return ""
	}
	for _, sha := range parents {
		green, err := g.GreenRunOf(sha)
		if err != nil {
			return ""
		}
		if green {
			return sha
		}
	}
	return ""
}

// GreenRunOf is whether the workflow has a successful run of the commit, given
// by its full SHA, on the branch.
func (g GitHub) GreenRunOf(sha string) (bool, error) {
	var body struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/actions/workflows/%s/runs?branch=%s&head_sha=%s&per_page=20",
		g.Repository, url.PathEscape(g.Workflow), encodeURIComponent(g.Branch), encodeURIComponent(sha))
	if err := g.get(path, &body); err != nil {
		return false, err
	}
	for _, r := range body.WorkflowRuns {
		if r.HeadSHA == sha && r.Conclusion != nil && *r.Conclusion == "success" {
			return true, nil
		}
	}
	return false, nil
}

// LastGreen is the head commit of the workflow's last successful run on the
// branch, as itos status reads it, read from the GitHub API with the token: ""
// and no error when no listed run succeeded. The runs are listed and the
// newest success taken:
// the API's own `status=success` filter can answer with a run far older than
// the newest green one, and the range would then name every task since.
func (g GitHub) LastGreen() (string, error) {
	url := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/runs?branch=%s&per_page=50",
		g.api(), g.Repository, g.Workflow, encodeURIComponent(g.Branch))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	res, err := (&http.Client{Timeout: Timeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("GitHub answered %s for %s's runs", res.Status, g.Workflow)
	}
	var body struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("GitHub's list of %s's runs cannot be read: %w", g.Workflow, err)
	}
	return FirstGreen(body.WorkflowRuns), nil
}

// LastGreenLook is one look at the commit main last proved, as ci.range's
// provider names it: "" and no error when it names none.
type LastGreenLook func() (sha string, err error)

// LastGreenProvider is the look itos status takes at the last green commit
// (slice 73), through ci.range's provider: ok is false for none. command is
// ci.range.command's first line, an error when it fails; github is the last
// green run of ci.range.github's workflow on its branch, the repository and
// token found as ci.watch's github provider finds them (the environment,
// else the remote's URL and gh), so it reads outside CI too. An error is a
// provider that cannot look at all, said before any request.
func LastGreenProvider(cfg *config.Loaded, s WatchSetup) (look LastGreenLook, ok bool, err error) {
	r := cfg.CI.Range
	switch r.Provider {
	case "none":
		return nil, false, nil
	case "command":
		command := ""
		if r.Command != nil {
			command = *r.Command
		}
		return func() (string, error) {
			var stdout bytes.Buffer
			res := shell.Run(cfg, command, shell.Options{Stdout: &stdout, Stderr: s.Stderr, Timeout: Timeout})
			if !res.OK() {
				return "", fmt.Errorf("ci.range.command failed (exit %s): %s", res.Status(), command)
			}
			line, _, _ := strings.Cut(value.Trim(stdout.String()), "\n")
			return value.Trim(line), nil
		}, true, nil
	}
	g, err := watchGitHub(cfg, "ci.range", s)
	if err != nil {
		return nil, false, err
	}
	g.Workflow, g.Branch = r.GitHub.Workflow, r.GitHub.Branch
	return g.LastGreen, true, nil
}

// WorkflowRun is what the github provider reads of a run.
type WorkflowRun struct {
	HeadSHA    string  `json:"head_sha"`
	Conclusion *string `json:"conclusion"`
	CreatedAt  string  `json:"created_at"`
}

// FirstGreen is the newest successful run's head commit, whatever order the
// list came in; "" when none succeeded.
func FirstGreen(runs []WorkflowRun) string {
	var green []WorkflowRun
	for _, r := range runs {
		if r.Conclusion != nil && *r.Conclusion == "success" && r.HeadSHA != "" {
			green = append(green, r)
		}
	}
	sort.SliceStable(green, func(i, j int) bool { return green[i].CreatedAt > green[j].CreatedAt })
	if len(green) == 0 {
		return ""
	}
	return green[0].HeadSHA
}

// encodeURIComponent is JavaScript's: every byte of the UTF-8 percent-encoded
// but letters, digits and -_.!~*'().
func encodeURIComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}
