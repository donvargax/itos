package corpus

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The repository and the token the fake GitHub answers for.
const (
	fakeRepository = "owner/name"
	fakeToken      = "fake-token"
)

var (
	runsPath = regexp.MustCompile(`^/repos/` + fakeRepository + `/actions/workflows/([^/]+)/runs$`)
	jobsPath = regexp.MustCompile(`^/repos/` + fakeRepository + `/actions/runs/(\d+)/jobs$`)
)

// fakeJob is one job of a run's look.
type fakeJob struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Conclusion *string `json:"conclusion"`
}

// fakeLook is how a run is seen: its address, its status, its conclusion and its jobs.
type fakeLook struct {
	HTMLURL    *string   `json:"html_url"`
	Status     *string   `json:"status"`
	Conclusion *string   `json:"conclusion"`
	Jobs       []fakeJob `json:"jobs"`
}

// fakeRun is a workflow run the fake holds: one look, or looks given one a request.
type fakeRun struct {
	fakeLook
	Workflow   *string    `json:"workflow"`
	HeadSHA    *string    `json:"head_sha"`
	HeadBranch *string    `json:"head_branch"`
	Looks      []fakeLook `json:"looks"`
}

// fakeGitHub is a case's github: every request refused, or the runs it holds.
type fakeGitHub struct {
	Refuse bool      `json:"refuse"`
	Runs   []fakeRun `json:"runs"`

	mu    sync.Mutex
	seen  []int      // each run's looks given so far
	shown []fakeLook // the look each run last showed, whose jobs its jobs endpoint lists
}

func or(s *string, d string) string {
	if s == nil {
		return d
	}
	return *s
}

// head is the commit a run answers a request for the workflow's runs as, narrowed to the commit
// and the branch when the request names them; false when it does not answer it.
func (r *fakeRun) head(workflow, sha, branch string) (string, bool) {
	head := or(r.HeadSHA, "")
	if head == "*" {
		head = sha
	}
	if or(r.Workflow, "ci.yml") != workflow || (sha != "" && head != sha) {
		return "", false
	}
	if branch != "" && or(r.HeadBranch, "main") != branch {
		return "", false
	}
	return head, true
}

// look is the next look at the ith run: the run itself when it has no looks.
func (g *fakeGitHub) look(i int) fakeLook {
	run := g.Runs[i]
	if len(run.Looks) == 0 {
		return run.fakeLook
	}
	look := run.Looks[min(g.seen[i], len(run.Looks)-1)]
	g.seen[i]++
	return look
}

// list is the workflow's runs a request finds, newest first, as GitHub lists them.
func (g *fakeGitHub) list(workflow, sha, branch string) (int, any) {
	type listed struct {
		ID         int     `json:"id"`
		HeadSHA    string  `json:"head_sha"`
		HeadBranch string  `json:"head_branch"`
		HTMLURL    string  `json:"html_url"`
		Status     string  `json:"status"`
		Conclusion *string `json:"conclusion"`
		CreatedAt  string  `json:"created_at"`
	}
	found := []listed{}
	for i := range g.Runs {
		head, ok := g.Runs[i].head(workflow, sha, branch)
		if !ok {
			continue
		}
		look := g.look(i)
		g.shown[i] = look
		created := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC).Add(-time.Duration(i) * time.Minute)
		found = append(found, listed{
			ID:         i + 1,
			HeadSHA:    head,
			HeadBranch: or(g.Runs[i].HeadBranch, "main"),
			HTMLURL:    strings.ReplaceAll(or(look.HTMLURL, ""), "{sha}", head),
			Status:     or(look.Status, "completed"),
			Conclusion: look.Conclusion,
			CreatedAt:  created.Format("2006-01-02T15:04:05.000Z"),
		})
	}
	return http.StatusOK, map[string]any{"total_count": len(found), "workflow_runs": found}
}

var notFound = map[string]any{"message": "Not Found"}

// jobs are the jobs of the run with the ID, as its last look showed them.
func (g *fakeGitHub) jobs(id int) (int, any) {
	if id < 1 || id > len(g.shown) {
		return http.StatusNotFound, notFound
	}
	jobs := g.shown[id-1].Jobs
	if jobs == nil {
		jobs = []fakeJob{}
	}
	return http.StatusOK, map[string]any{"total_count": len(jobs), "jobs": jobs}
}

// answer is the fake's status and JSON body for one request.
func (g *fakeGitHub) answer(req *http.Request) (int, any) {
	if g.Refuse {
		return http.StatusUnauthorized, map[string]any{"message": "Bad credentials"}
	}
	if req.Method != http.MethodGet || req.Header.Get("Authorization") != "Bearer "+fakeToken {
		return http.StatusNotFound, notFound
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	path := req.URL.EscapedPath()
	if m := jobsPath.FindStringSubmatch(path); m != nil {
		id, _ := strconv.Atoi(m[1])
		return g.jobs(id)
	}
	m := runsPath.FindStringSubmatch(path)
	if m == nil {
		return http.StatusNotFound, notFound
	}
	workflow, err := url.PathUnescape(m[1])
	if err != nil {
		return http.StatusNotFound, notFound
	}
	q := req.URL.Query()
	return g.list(workflow, q.Get("head_sha"), q.Get("branch"))
}

// serveGitHub serves the fake a case's github describes on a port of 127.0.0.1 of its own; its
// address, and what stops it.
func serveGitHub(spec any) (string, func(), error) {
	data, err := json.Marshal(plain(spec))
	if err != nil {
		return "", nil, err
	}
	g := &fakeGitHub{}
	if err := json.Unmarshal(data, g); err != nil {
		return "", nil, fixtureErr("the fake GitHub: %v", err)
	}
	g.seen = make([]int, len(g.Runs))
	g.shown = make([]fakeLook, len(g.Runs))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		status, body := g.answer(req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String(), func() { _ = server.Close() }, nil
}
