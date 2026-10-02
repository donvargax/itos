package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/donvargax/itos/internal/config"
	"github.com/donvargax/itos/internal/git"
	"github.com/donvargax/itos/internal/shell"
	"github.com/donvargax/itos/internal/value"
)

// Range is a ci.range provider: the start commit it proposes for a push's
// range, before RangeStart holds it to the head's ancestry. "" means "run
// everything", and so does every way a lookup can fail: a provider never
// errors, since a range that cannot be found is the whole history.
type Range func() string

// Env is how a provider reads the environment (os.Getenv in the binary).
type Env func(string) string

// RangeProvider is the provider itos.yaml's ci.range names
// (providers.ts's rangeProvider): `none`, `command` (its stdout's first
// line, through the config's shell, its stderr on stderr) or `github` (the
// last green run of the workflow). Each is a function of the config's
// ci.range and the environment, so a provider over another forge's API is
// one more case beside github's.
func RangeProvider(cfg *config.Loaded, env Env, stderr io.Writer) Range {
	r := cfg.CI.Range
	switch r.Provider {
	case "none":
		return func() string { return "" }
	case "command":
		command := ""
		if r.Command != nil {
			command = *r.Command
		}
		return func() string { return FirstLine(cfg, command, stderr) }
	}
	return GitHub{
		Repository: env(r.GitHub.RepositoryEnv),
		Token:      firstSet(env, r.GitHub.TokenEnv),
		Workflow:   r.GitHub.Workflow,
		Branch:     r.GitHub.Branch,
	}.LastGreenRun
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
	green := provider()
	if green == "" || head == "" {
		return ""
	}
	if git.Succeeds("merge-base", "--is-ancestor", green, head) {
		return green
	}
	return ""
}

// GitHubAPI is where the github provider asks; a test points it elsewhere.
var GitHubAPI = "https://api.github.com"

// Timeout is how long the github provider waits for the API. Node's fetch
// waits however long it takes; past this the lookup fails, which runs
// everything, rather than holding the CI run until its job's own timeout.
var Timeout = time.Minute

// GitHub is the github provider's question: the workflow's runs on a branch
// of a repository ("owner/name"), asked with a token when there is one.
type GitHub struct {
	Repository, Token, Workflow, Branch string
}

// LastGreenRun is the head commit of the workflow's last successful run on
// the branch, read from the GitHub API with the workflow's token
// (`actions: read`). Anything that goes wrong reads as "no green run", which
// runs everything, and with no repository nobody is asked. The runs are
// listed and the newest success taken: the API's own `status=success` filter
// can answer with a run far older than the newest green one, and the range
// would then name every task since.
func (g GitHub) LastGreenRun() string {
	if g.Repository == "" {
		return ""
	}
	url := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/runs?branch=%s&per_page=50",
		GitHubAPI, g.Repository, g.Workflow, encodeURIComponent(g.Branch))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	res, err := (&http.Client{Timeout: Timeout}).Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return ""
	}
	var body struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if json.NewDecoder(res.Body).Decode(&body) != nil {
		return ""
	}
	return FirstGreen(body.WorkflowRuns)
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
