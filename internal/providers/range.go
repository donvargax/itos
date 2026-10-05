package providers

import (
	"bytes"
	"fmt"
	"io"
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
	sha, _ := g.greenFirstParent(head, 1)
	return sha
}

// LastGreenFrom is the commit itos status names as the last green one (bug
// 24): the first of the head's first parents, the head itself included, with
// a successful run of the workflow on the branch, found by NearestGreen's walk
// and bound, for the reason it gives; "" and no error when none of them has
// one. Unlike the range's, its failures are said.
func (g GitHub) LastGreenFrom(head string) (string, error) {
	return g.greenFirstParent(head, 0)
}

// greenFirstParent is the walk: the commit's first parents, the commit itself
// the first of them, skip of them passed over, then at most FirstParentsAsked
// asked about, nearest first; the first with a green run, "" when none has
// one, and an error when they cannot be listed or one cannot be asked about.
func (g GitHub) greenFirstParent(from string, skip int) (string, error) {
	parents, err := git.Lines("rev-list", "--first-parent", fmt.Sprintf("--skip=%d", skip),
		fmt.Sprintf("--max-count=%d", FirstParentsAsked), from, "--")
	if err != nil {
		return "", fmt.Errorf("the first parents of %s cannot be listed: %w", from, err)
	}
	for _, sha := range parents {
		green, err := g.GreenRunOf(sha)
		if err != nil {
			return "", err
		}
		if green {
			return sha, nil
		}
	}
	return "", nil
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

// LastGreenLook is one look at the commit main last proved, as ci.range's
// provider names it, given the head of the branch as fetched: "" and no
// error when it names none.
type LastGreenLook func(head string) (sha string, err error)

// LastGreenProvider is the look itos status takes at the last green commit
// (slice 73), through ci.range's provider: ok is false for none. command is
// ci.range.command's first line, an error when it fails, the head unused;
// github is LastGreenFrom the head, of ci.range.github's workflow on its
// branch, the repository, token and API's address found as ci.watch's github
// provider finds them (the environment, else the remote's URL and gh), so it
// reads outside CI too. An error is a provider that cannot look at all, said
// before any request.
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
		return func(string) (string, error) {
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
	return g.LastGreenFrom, true, nil
}

// WorkflowRun is what the github provider reads of a run.
type WorkflowRun struct {
	HeadSHA    string  `json:"head_sha"`
	Conclusion *string `json:"conclusion"`
	CreatedAt  string  `json:"created_at"`
}

// FirstGreen is the newest successful run's head commit, whatever order the
// list came in; "" when none succeeded. Nothing reads a list of runs for its
// green one since bug 24; it stays for T-008's check, which runs its test.
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
