// The steps of ci range's github provider (ci.feature, bug 23), and of status's
// look at the last green commit through it (status.feature, bug 24): a fake GitHub,
// an httptest server itos is pointed at through GITHUB_API_URL, holding runs
// of the workflows of one repository, each on a branch, the configured one
// unless a step names another (bug 29). It answers a workflow's runs
// endpoint as GitHub does: with head_sha, that commit's runs; without, the
// list of runs, newest first, which is every run it holds unless a step makes
// the list say otherwise (stale, as GitHub's was on 2026-10-05); and with
// branch, only the runs on that branch. It answers a run's jobs endpoint with
// the jobs a step gave it. ci.watch asks it too (watch.feature, slice 85): a
// commit of the watched workflow with no run of its own is answered with the
// watched run, one look a request, the nth request the nth look and the last
// look for every request after it, as the run a push started would answer
// while it goes; a step can have it do something first, when it is first
// asked for the watched run (bug 34), and a run that something adds for the
// commit is the answer (bug 41: the run of a push that another push cancels). Anything else it is asked is a 404, everything a 401 once
// a step makes it refuse the token, and everything a 500 once a step makes it fail (slice 86).
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strconv"
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
	workflow   string  // the workflow it is a run of
	jobs       []watchedJob
}

// The fake GitHub and what it was asked.
type fakeGitHub struct {
	server   *httptest.Server
	workflow string // the workflow ci.range asks about, whose runs run and runOn make
	branch   string
	mu       sync.Mutex
	runs     []gitHubRun  // every run, oldest first
	list     []gitHubRun  // what the branch's list names, when a step set it; every run when nil
	asked    []string     // each request, its path and query
	watched  []watchedRun // the watched run's looks, when ci.watch asks
	looks    int          // how many looks at the watched run were given
	given    []string     // the commits a look at the watched run was given
	refuses  bool         // whether it answers every request 401
	failing  bool         // whether it answers every request 500, a server error
	// What it does when first asked for the watched run, before it answers
	// (bug 34): a change made while the asker waits. An error answers 500.
	firstLook func() error
}

// The fake GitHub's two endpoints: a workflow's runs, and a run's jobs.
var (
	runsPath = regexp.MustCompile(`^/repos/` + regexp.QuoteMeta(fakeRepository) + `/actions/workflows/([^/]+)/runs$`)
	jobsPath = regexp.MustCompile(`^/repos/` + regexp.QuoteMeta(fakeRepository) + `/actions/runs/(\d+)/jobs$`)
)

// The ID of the watched run's first look; its nth is one more for each look
// before it.
const watchedID = 1000

func (g *fakeGitHub) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked = append(g.asked, req.URL.RequestURI())
	if g.refuses {
		http.Error(rw, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
		return
	}
	if g.failing {
		http.Error(rw, `{"message":"Server Error"}`, http.StatusInternalServerError)
		return
	}
	if req.Method != http.MethodGet || req.Header.Get("Authorization") != "Bearer "+fakeToken {
		http.NotFound(rw, req)
		return
	}
	if m := jobsPath.FindStringSubmatch(req.URL.Path); m != nil {
		id, _ := strconv.Atoi(m[1])
		g.writeJobs(rw, req, id)
		return
	}
	m := runsPath.FindStringSubmatch(req.URL.Path)
	if m == nil {
		http.NotFound(rw, req)
		return
	}
	workflow := m[1]
	of := slices.DeleteFunc(slices.Clone(g.runs), func(r gitHubRun) bool { return r.workflow != workflow })
	q := req.URL.Query()
	var runs []gitHubRun
	if sha := q.Get("head_sha"); sha != "" {
		for _, r := range of {
			if r.HeadSHA == sha {
				runs = append(runs, r)
			}
		}
		if len(runs) == 0 && g.watched != nil && workflow == watchedWorkflow {
			if g.looks == 0 && g.firstLook != nil {
				first := g.firstLook
				g.firstLook = nil
				if err := first(); err != nil {
					fmt.Fprintf(os.Stderr, "the fake GitHub's first look failed: %v\n", err)
					http.Error(rw, err.Error(), http.StatusInternalServerError)
					return
				}
				for _, r := range g.runs {
					if r.HeadSHA == sha && r.workflow == workflow {
						runs = append(runs, r)
					}
				}
			}
			if len(runs) == 0 {
				runs = []gitHubRun{g.look(sha)}
			}
		}
	} else if g.list != nil {
		runs = slices.Clone(g.list)
	} else {
		runs = of
	}
	if branch := q.Get("branch"); branch != "" {
		runs = slices.DeleteFunc(runs, func(r gitHubRun) bool { return r.HeadBranch != branch })
	}
	slices.Reverse(runs) // newest first, as GitHub lists them
	writeRuns(rw, runs)
}

// The next look at the watched run, as a run of the commit.
func (g *fakeGitHub) look(sha string) gitHubRun {
	n := min(g.looks, len(g.watched)-1)
	g.looks++
	g.given = append(g.given, sha)
	w := g.watched[n]
	r := gitHubRun{ID: watchedID + n, HeadSHA: sha, HeadBranch: g.branch, Status: w.Status,
		CreatedAt: "2026-10-05T13:00:00Z", HTMLURL: w.URL, workflow: watchedWorkflow, jobs: w.Jobs}
	if w.Conclusion != "" {
		conclusion := w.Conclusion
		r.Conclusion = &conclusion
	}
	return r
}

// The jobs of the run with the ID, as GitHub lists them: a conclusion only
// once a job completed.
func (g *fakeGitHub) writeJobs(rw http.ResponseWriter, req *http.Request, id int) {
	var jobs []watchedJob
	switch {
	case id >= watchedID && id-watchedID < len(g.watched):
		jobs = g.watched[id-watchedID].Jobs
	case id >= 1 && id <= len(g.runs):
		jobs = g.runs[id-1].jobs
	default:
		http.NotFound(rw, req)
		return
	}
	type job struct {
		Name       string  `json:"name"`
		Status     string  `json:"status"`
		Conclusion *string `json:"conclusion"`
	}
	listed := []job{}
	for _, j := range jobs {
		one := job{Name: j.Name, Status: j.Status}
		if j.Conclusion != "" {
			conclusion := j.Conclusion
			one.Conclusion = &conclusion
		}
		listed = append(listed, one)
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]any{"total_count": len(listed), "jobs": listed})
}

func writeRuns(rw http.ResponseWriter, runs []gitHubRun) {
	if runs == nil {
		runs = []gitHubRun{}
	}
	rw.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(rw).Encode(map[string]any{"total_count": len(runs), "workflow_runs": runs})
}

// A run of the watched workflow for the commit on the configured branch, at
// the address with the jobs, created after every run before it: completed
// with the conclusion. The caller holds g.mu.
func (g *fakeGitHub) addRun(sha, url, conclusion string, jobs []watchedJob) {
	id := len(g.runs) + 1
	g.runs = append(g.runs, gitHubRun{ID: id, HeadSHA: sha, HeadBranch: g.branch, Status: "completed",
		Conclusion: &conclusion, CreatedAt: fmt.Sprintf("2026-10-05T12:%02d:00Z", id), HTMLURL: url,
		workflow: watchedWorkflow, jobs: jobs})
}

// A run of the commit on the configured branch, created after every run
// before it: status completed with the conclusion, or the status alone when
// the conclusion is "".
func (g *fakeGitHub) run(sha, status, conclusion string) gitHubRun {
	return g.runOn(g.branch, sha, status, conclusion)
}

// A run of the commit on the branch, as run makes one.
func (g *fakeGitHub) runOn(branch, sha, status, conclusion string) gitHubRun {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := len(g.runs) + 1
	r := gitHubRun{ID: id, HeadSHA: sha, HeadBranch: branch, Status: status,
		CreatedAt: fmt.Sprintf("2026-10-05T12:%02d:00Z", id), HTMLURL: fmt.Sprintf("https://github.com/%s/actions/runs/%d", fakeRepository, id),
		workflow: g.workflow}
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
	// A push to another branch is proved by its own green run (bug 29).
	sc.Step(`^the fake GitHub has a green run of (the head's parent|the commit before the head's parent|the first commit) on the branch "([^"]*)"$`, func(which, branch string) error {
		g, sha, err := w.fakeGitHubAnd(which)
		if err != nil {
			return err
		}
		g.runOn(branch, sha, "completed", "success")
		return nil
	})
	// status's look at the last green commit walks the remote's branch as
	// fetched (status.feature, bug 24), so its runs are of the remote's commits.
	sc.Step(`^the fake GitHub has a green run of the remote's commit "([^"]*)"$`, func(header string) error {
		if w.github == nil {
			return fmt.Errorf("no fake GitHub: start one first")
		}
		sha, err := w.remoteCommit(header)
		if err != nil {
			return err
		}
		w.github.run(sha, "completed", "success")
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

// ci.range's provider is github, asking about the workflow, and itos runs as
// Actions runs it, given the repository, a token and the API's address: the
// fake GitHub's, whose runs are on the branch. The config names no branch:
// v5.0.0 removed ci.range.github.branch (slice 85), a run counting on any
// branch since bug 29.
func (w *world) rangeAsksFakeGitHub(workflow, branch string) error {
	g := w.fakeGitHub()
	g.workflow, g.branch = workflow, branch
	w.config.rangeGitHub = workflow
	return w.writeConfig()
}

// The scenario's fake GitHub, started the first time a step asks for it, and
// itos run as Actions runs it, given the repository, a token and the API's
// address: the fake GitHub's. Its runs are of ci.yml on main until a step says
// otherwise.
func (w *world) fakeGitHub() *fakeGitHub {
	if w.github != nil {
		return w.github
	}
	w.github = &fakeGitHub{workflow: watchedWorkflow, branch: "main"}
	w.github.server = httptest.NewServer(w.github)
	w.vars = append(w.vars,
		"GITHUB_API_URL="+w.github.server.URL,
		"GITHUB_REPOSITORY="+fakeRepository,
		"GITHUB_TOKEN="+fakeToken)
	return w.github
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
