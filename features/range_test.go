// The steps of ci range's github provider (ci.feature, bug 23): a fake GitHub,
// an httptest server itos is pointed at through GITHUB_API_URL, holding runs
// of one workflow on one branch of one repository. It answers the workflow's
// runs endpoint as GitHub does: with head_sha, that commit's runs; without,
// the list of the branch's runs, newest first, which is every run it holds
// unless a step makes the list say otherwise (stale, as GitHub's was on
// 2026-10-05). Anything else it is asked is a 404.
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// The repository and token the fake GitHub expects, given to itos as Actions
// gives them.
const (
	fakeRepository = "owner/scratch"
	fakeToken      = "fake-token"
)

// One run of the fake GitHub's workflow.
type gitHubRun struct {
	ID         int     `json:"id"`
	HeadSHA    string  `json:"head_sha"`
	HeadBranch string  `json:"head_branch"`
	Status     string  `json:"status"`
	Conclusion *string `json:"conclusion"`
	CreatedAt  string  `json:"created_at"`
	HTMLURL    string  `json:"html_url"`
}

// The fake GitHub and what it was asked.
type fakeGitHub struct {
	server   *httptest.Server
	workflow string
	branch   string
	mu       sync.Mutex
	runs     []gitHubRun // every run, oldest first
	list     []gitHubRun // what the branch's list names, when a step set it; every run when nil
	asked    []string    // each request, its path and query
}

func (g *fakeGitHub) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked = append(g.asked, req.URL.RequestURI())
	want := "/repos/" + fakeRepository + "/actions/workflows/" + g.workflow + "/runs"
	if req.Method != http.MethodGet || req.URL.Path != want || req.Header.Get("Authorization") != "Bearer "+fakeToken {
		http.NotFound(rw, req)
		return
	}
	q := req.URL.Query()
	if branch := q.Get("branch"); branch != "" && branch != g.branch {
		writeRuns(rw, nil)
		return
	}
	var runs []gitHubRun
	if sha := q.Get("head_sha"); sha != "" {
		for _, r := range g.runs {
			if r.HeadSHA == sha {
				runs = append(runs, r)
			}
		}
	} else if g.list != nil {
		runs = slices.Clone(g.list)
	} else {
		runs = slices.Clone(g.runs)
	}
	slices.Reverse(runs) // newest first, as GitHub lists them
	writeRuns(rw, runs)
}

func writeRuns(rw http.ResponseWriter, runs []gitHubRun) {
	if runs == nil {
		runs = []gitHubRun{}
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]any{"total_count": len(runs), "workflow_runs": runs})
}

// A run of the commit, created after every run before it: status completed
// with the conclusion, or the status alone when the conclusion is "".
func (g *fakeGitHub) run(sha, status, conclusion string) gitHubRun {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := len(g.runs) + 1
	r := gitHubRun{ID: id, HeadSHA: sha, HeadBranch: g.branch, Status: status,
		CreatedAt: fmt.Sprintf("2026-10-05T12:%02d:00Z", id), HTMLURL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", fakeRepository, id)}
	if conclusion != "" {
		r.Conclusion = &conclusion
	}
	g.runs = append(g.runs, r)
	return r
}

func initializeRangeSteps(sc *godog.ScenarioContext, w *world) {
	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if w.github != nil {
			w.github.server.Close()
		}
		return ctx, nil
	})
	sc.Step(`^ci\.range asks a fake GitHub for the runs of "([^"]*)" on "([^"]*)"$`, w.rangeAsksFakeGitHub)
	sc.Step(`^three commits on top of the first$`, func() error {
		for _, subject := range []string{"chore: one", "chore: two", "chore: three"} {
			if err := w.commitOnTop(subject); err != nil {
				return err
			}
		}
		return nil
	})
	sc.Step(`^the fake GitHub's list of runs names a green run of the first commit alone$`, func() error {
		g, sha, err := w.fakeGitHubAnd("the first commit")
		if err != nil {
			return err
		}
		g.list = []gitHubRun{g.run(sha, "completed", "success")}
		return nil
	})
	sc.Step(`^the fake GitHub has a (green|failed) run of (the head's parent|the commit before the head's parent|the first commit)$`, func(kind, which string) error {
		g, sha, err := w.fakeGitHubAnd(which)
		if err != nil {
			return err
		}
		g.run(sha, "completed", map[string]string{"green": "success", "failed": "failure"}[kind])
		return nil
	})
	sc.Step(`^the fake GitHub has a run still going of (the head's parent|the commit before the head's parent|the first commit)$`, func(which string) error {
		g, sha, err := w.fakeGitHubAnd(which)
		if err != nil {
			return err
		}
		g.run(sha, "in_progress", "")
		return nil
	})
	sc.Step(`^itos prints where the range of the head starts$`, func() error {
		head, err := w.head()
		if err != nil {
			return err
		}
		return w.itos("ci", "range", "--head", head)
	})
	sc.Step(`^the range starts at (the head's parent|the commit before the head's parent|the first commit)$`, func(which string) error {
		sha, err := w.commitCalled(which)
		if err != nil {
			return err
		}
		return w.rangeFromIs(sha, which)
	})
	// An empty range is also what a provider that asks nobody prints, so with
	// a fake GitHub the range is empty only when itos asked it about every
	// first parent of the head and found none green.
	sc.Step(`^the range is empty$`, func() error {
		if err := w.rangeFromIs("", "empty"); err != nil {
			return err
		}
		return w.askedEveryFirstParent()
	})
}

// ci.range's provider is github, asking about the workflow on the branch, and
// itos runs as Actions runs it, given the repository, a token and the API's
// address: the fake GitHub's.
func (w *world) rangeAsksFakeGitHub(workflow, branch string) error {
	w.github = &fakeGitHub{workflow: workflow, branch: branch}
	w.github.server = httptest.NewServer(w.github)
	w.config.rangeGitHub = &[2]string{workflow, branch}
	w.vars = append(w.vars,
		"GITHUB_API_URL="+w.github.server.URL,
		"GITHUB_REPOSITORY="+fakeRepository,
		"GITHUB_TOKEN="+fakeToken)
	return w.writeConfig()
}

// The fake GitHub, and the full SHA of the commit a step names.
func (w *world) fakeGitHubAnd(which string) (*fakeGitHub, string, error) {
	if w.github == nil {
		return nil, "", fmt.Errorf("no fake GitHub: start one first")
	}
	sha, err := w.commitCalled(which)
	return w.github, sha, err
}

// The full SHA of the commit the words name, counted from the head.
func (w *world) commitCalled(which string) (string, error) {
	n := len(w.commits)
	index := map[string]int{
		"the head's parent":                   n - 2,
		"the commit before the head's parent": n - 3,
		"the first commit":                    0,
	}[which]
	if index < 0 || index >= n {
		return "", fmt.Errorf("the repository has no %s: its commits are %v", which, w.commits)
	}
	return w.commits[index], nil
}

// The fake GitHub was asked for the runs of each commit before the head, by
// its full SHA.
func (w *world) askedEveryFirstParent() error {
	if w.github == nil {
		return nil
	}
	asked := w.github.requests()
	for _, sha := range w.commits[:len(w.commits)-1] {
		if !slices.ContainsFunc(asked, func(uri string) bool { return strings.Contains(uri, "head_sha="+sha) }) {
			return fmt.Errorf("the fake GitHub was never asked for the runs of %s; it was asked %v\n%s", sha, asked, w.report())
		}
	}
	return nil
}

func (g *fakeGitHub) requests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.asked)
}

// ci range printed FROM= and the commit, or nothing for an empty range.
func (w *world) rangeFromIs(sha, which string) error {
	if err := w.exitsWith(0); err != nil {
		return err
	}
	if !slices.Contains(strings.Split(w.stdout, "\n"), "FROM="+sha) {
		var asked []string
		if w.github != nil {
			asked = w.github.requests()
		}
		return fmt.Errorf("the range does not start at %s (FROM=%s)\nthe fake GitHub was asked %v\n%s", which, sha, asked, w.report())
	}
	return nil
}
