package ci

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/donvargax/itos/v8/internal/providers"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The pull request's head, a commit on start with head's message and tree.
const pullHead = "3333333333333333333333333333333333333333"

const (
	pullURL = "https://github.example/o/n/pull/7"
	runURL  = "https://ci.example/runs/5"
)

// trustText is a config under push.mode pr whose ci has a whole step and a
// prose one.
const trustText = "version: 1\npush: { mode: pr }\nci: { steps: [echo whole], prose: { paths: [\"**/*.md\"], steps: [echo prose] } }\n"

// gitHub is a fake GitHub holding one merged pull request of the head, whose
// head's run has the conclusion, and keeping what it was asked.
type gitHub struct {
	mu         sync.Mutex
	asked      []string
	merged     bool   // whether the pull request merged
	mergedAs   string // the commit it merged as
	conclusion string
	down       bool
}

func (g *gitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked = append(g.asked, r.URL.RequestURI())
	if g.down || r.Header.Get("Authorization") != "Bearer tok" {
		http.Error(w, "no", http.StatusInternalServerError)
		return
	}
	switch {
	case r.URL.Path == "/repos/o/n/commits/"+head+"/pulls":
		mergedAt := "null"
		if g.merged {
			mergedAt = `"2026-10-26T10:00:00Z"`
		}
		fmt.Fprintf(w, `[{"number":7,"html_url":%q,"merged_at":%s,"merge_commit_sha":%q,"head":{"sha":%q}}]`, pullURL, mergedAt, g.mergedAs, pullHead)
	case r.URL.Path == "/repos/o/n/actions/workflows/ci.yml/runs" && r.URL.Query().Get("head_sha") == pullHead:
		fmt.Fprintf(w, `{"workflow_runs":[{"id":5,"html_url":%q,"status":"completed","conclusion":%q,"created_at":"2026-10-26T10:00:00Z","head_sha":%q}]}`, runURL, g.conclusion, pullHead)
	case r.URL.Path == "/repos/o/n/actions/runs/5/jobs":
		fmt.Fprint(w, `{"jobs":[{"id":51,"name":"ci","status":"completed","conclusion":"success"}]}`)
	default:
		http.NotFound(w, r)
	}
}

func (g *gitHub) requests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.asked)
}

// trusting is the ports of a range start..HEAD whose head commit, main.go
// added, a merged pull request is listed to have merged as HEAD from a head
// that is a commit of the clone with the same message and tree; the fake
// GitHub answers as Actions would be asked.
func trusting(t *testing.T, text string) (*ports, *gitHub) {
	t.Helper()
	p := over(t, text, map[string]string{"main.go": "package main\n"})
	g := &gitHub{merged: true, mergedAs: head, conclusion: "success"}
	server := httptest.NewServer(g)
	t.Cleanup(server.Close)
	p.env = map[string]string{providers.APIEnv: server.URL, "GITHUB_REPOSITORY": "o/n", "GITHUB_TOKEN": "tok"}
	v := p.VCS
	tip := v.History[1]
	v.History = append(v.History, vcsfake.Commit{Name: pullHead, Parents: []string{start}, Message: tip.Message, Tree: tip.Tree})
	return p, g
}

// ci plan of a range that is a merged pull request's commits, whose head's
// run passed, plans the prose steps and says the trust last, and --json names
// it; the pull request is asked once.
func TestPlanTrustsAMergedPullRequestsGreenHead(t *testing.T) {
	p, g := trusting(t, trustText)
	code, stdout, stderr, err := call(t, p, "plan", start, "HEAD")
	want := "late  echo prose\nTrusted: the pull request " + pullURL + " merged its head 3333333 as 2222222, the same commits with the same trees but for the work registry and the ledger, and the head's CI run passed: " + runURL + ". Only the prose steps run, and the named tasks' static and `prose: true` checks.\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("plan = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if asked := g.requests(); len(asked) != 3 || !strings.HasPrefix(asked[0], "/repos/o/n/commits/"+head+"/pulls") ||
		!strings.Contains(asked[1], "head_sha="+pullHead) {
		t.Errorf("GitHub was asked %v", asked)
	}

	code, stdout, _, err = call(t, p, "--json", "plan", start, "HEAD")
	got := compact(stdout)
	if code != 0 || err != nil || !strings.Contains(got, `"prose":true`) ||
		!strings.HasSuffix(got, `"trusted":{"pull_request":"`+pullURL+`","head":"`+pullHead+`","run":"`+runURL+`"}}`) {
		t.Errorf("--json plan = exit %d, %v, stdout %q", code, err, stdout)
	}
}

// ci run carries out the trusted plan, its trust in the plan the port is
// given; --tests runs the prose plan's run of named tests, which is none here.
func TestRunCarriesOutATrustedPlan(t *testing.T) {
	p, _ := trusting(t, trustText)
	code, _, _, err := call(t, p, "run", start, "HEAD")
	if code != 0 || err != nil || len(p.ran) != 1 || p.ran[0].Trusted == nil || !p.ran[0].Prose || !slices.Equal(p.ran[0].Steps, []string{"echo prose"}) {
		t.Errorf("run = exit %d, %v, ran %+v", code, err, p.ran)
	}
	code, stdout, _, err := call(t, p, "run", start, "HEAD", "--tests")
	if code != 0 || err != nil || len(p.ran) != 2 || len(p.ran[1].Order) != 0 ||
		stdout != "ci run --tests: no named tests to run (no tests: step, or nothing selected), so nothing ran.\n" {
		t.Errorf("run --tests = exit %d, %v, stdout %q, ran %+v", code, err, stdout, p.ran)
	}
	code, _, _, err = call(t, p, "--json", "run", start, "HEAD")
	if code != 0 || err != nil || len(p.ran) != 3 || p.ran[2].Trusted == nil || !p.options[2].JSON {
		t.Errorf("--json run = exit %d, %v, ran %d", code, err, len(p.ran))
	}
}

// A merged pull request whose head's run failed is not trusted: the whole
// plan, the note last.
func TestPlanSaysWhyAMergedPullRequestIsNotTrusted(t *testing.T) {
	p, g := trusting(t, trustText)
	g.conclusion = "failure"
	code, stdout, _, err := call(t, p, "plan", start, "HEAD")
	want := "late  echo whole\nNot trusted: the pull request " + pullURL + " merged its head 3333333 as 2222222, but the head's CI run " + runURL + " has not passed.\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("plan = exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--json", "plan", start, "HEAD")
	if code != 0 || err != nil || !strings.Contains(compact(stdout), `"prose":false`) || strings.Contains(stdout, "trusted") || strings.Contains(stdout, "Not trusted") {
		t.Errorf("--json plan = exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, _, _, err := call(t, p, "run", start, "HEAD"); code != 0 || err != nil || p.ran[0].Note != strings.TrimSuffix(want[strings.Index(want, "Not trusted"):], "\n") {
		t.Errorf("run = exit %d, %v, ran %+v", code, err, p.ran)
	}
}

// Nothing changes, and GitHub is not asked about a pull request, where the
// rule does not apply: trunk, no push.mode, provider none, no repository, no
// prose steps, the nightly, --whole, an empty range start, and ci scope; and
// a look that fails, an open pull request and one merged as another commit
// say nothing.
func TestPlanAsksGitHubOnlyWhereTheRuleApplies(t *testing.T) {
	for name, c := range map[string]struct {
		text string
		args []string
		env  string
	}{
		"trunk":            {strings.Replace(trustText, "push: { mode: pr }", "push: { mode: trunk }", 1), []string{"plan", start, "HEAD"}, ""},
		"no push mode":     {strings.Replace(trustText, "push: { mode: pr }\n", "", 1), []string{"plan", start, "HEAD"}, ""},
		"provider none":    {strings.Replace(trustText, "prose:", "range: { provider: none }, prose:", 1), []string{"plan", start, "HEAD"}, ""},
		"no repository":    {trustText, []string{"plan", start, "HEAD"}, "GITHUB_REPOSITORY"},
		"no prose steps":   {strings.Replace(trustText, ", prose: { paths: [\"**/*.md\"], steps: [echo prose] }", "", 1), []string{"plan", start, "HEAD"}, ""},
		"the nightly":      {strings.Replace(trustText, "ci: {", "ci: { nightly: { steps: [echo night] },", 1), []string{"plan", "--nightly"}, ""},
		"--whole":          {trustText, []string{"plan", "--whole"}, ""},
		"an empty start":   {trustText, []string{"plan", "", "HEAD"}, ""},
		"an all-zeros one": {trustText, []string{"plan", "0000000000000000000000000000000000000000", "HEAD"}, ""},
	} {
		p, g := trusting(t, c.text)
		if c.env != "" {
			delete(p.env, c.env)
		}
		code, stdout, _, err := call(t, p, c.args...)
		if code != 0 || err != nil || strings.Contains(stdout, "rusted") || strings.Contains(stdout, "echo prose") || len(g.requests()) != 0 {
			t.Errorf("%s: exit %d, %v, stdout %q, GitHub asked %v", name, code, err, stdout, g.requests())
		}
	}

	p, g := trusting(t, trustText)
	if code, stdout, _, err := call(t, p, "scope", start, "HEAD"); code != 0 || err != nil || stdout != "docs_only=false\n" || len(g.requests()) != 0 {
		t.Errorf("scope: exit %d, %v, stdout %q, GitHub asked %v", code, err, stdout, g.requests())
	}

	for name, change := range map[string]func(*gitHub){
		"a look that fails":                    func(g *gitHub) { g.down = true },
		"an open pull request":                 func(g *gitHub) { g.merged = false },
		"a pull request merged as another one": func(g *gitHub) { g.mergedAs = start },
	} {
		p, g := trusting(t, trustText)
		change(g)
		code, stdout, _, err := call(t, p, "plan", start, "HEAD")
		if code != 0 || err != nil || stdout != "late  echo whole\n" || len(g.requests()) != 1 {
			t.Errorf("%s: exit %d, %v, stdout %q, GitHub asked %v", name, code, err, stdout, g.requests())
		}
	}
}
