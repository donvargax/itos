package ci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	cirun "github.com/donvargax/itos/v8/internal/ci"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/plan"
	"github.com/donvargax/itos/v8/internal/providers"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The repository's two commits: start, and head, which changes README.md
// and main.go after it.
const (
	start = "1111111111111111111111111111111111111111"
	head  = "2222222222222222222222222222222222222222"
)

// ports are the ci command's ports over the fakes: a repository of two
// commits, the config or its failure, whether it is the stealth one, the
// environment, what the RunPlan port answers, each plan it was given kept,
// and the watcher Watcher gives, its error, or none.
type ports struct {
	*vcsfake.VCS
	cfg     *config.Loaded
	err     error
	stealth bool
	env     map[string]string
	code    int
	runErr  error
	ran     []*plan.Plan
	options []cirun.Options
	watcher providers.Watcher
	none    bool
	wErr    error
	remote  string
}

func (p *ports) Stealth(context.Context) bool { return p.stealth }

func (p *ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p *ports) Env(_ context.Context, name string) string { return p.env[name] }

func (p *ports) RunPlan(_ context.Context, _ *config.Loaded, pl *plan.Plan, o cirun.Options) (int, error) {
	p.ran, p.options = append(p.ran, pl), append(p.options, o)
	return p.code, p.runErr
}

func (p *ports) Watcher(_ context.Context, _ *config.Loaded, remote string) (providers.Watcher, bool, error) {
	p.remote = remote
	return p.watcher, !p.none, p.wErr
}

// over is the ports of the config text, with a ledger of one task, over a
// repository whose start has the ledger and whose head adds files, HEAD
// and main naming head.
func over(t *testing.T, text string, files map[string]string) *ports {
	t.Helper()
	startTree := map[string]string{"tasks/phase-1.yaml": "- { id: T-9, title: Nine, done_when: [] }\n"}
	headTree := maps.Clone(startTree)
	maps.Copy(headTree, files)
	v := &vcsfake.VCS{
		Located:  map[string]vcsport.Location{"": {Top: "/r", GitDir: ".git", Common: ".git"}},
		History:  []vcsfake.Commit{{Name: start, Message: "chore: start", Tree: startTree}, {Name: head, Parents: []string{start}, Message: "chore: next", Tree: headTree}},
		Named:    map[string]string{"HEAD": head, "refs/heads/main": head},
		OnBranch: "main",
	}
	tree := treefake.Trees{Common: ".git", Files: map[string]map[string]string{"worktree": headTree}}
	src, err := tree.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(v, tree, src, "itos.yaml", text+"ledger: { files: \"tasks/phase-{group}.yaml\" }\n", false)
	if err != nil {
		t.Fatal(err)
	}
	return &ports{VCS: v, cfg: cfg, env: map[string]string{}}
}

// call is the command's line run over p: its exit, stdout, stderr and
// error; a --json or --quiet first sets the global flag.
func call(t *testing.T, p *ports, args ...string) (int, string, string, error) {
	t.Helper()
	asJSON, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "--quiet") {
		asJSON, quiet = asJSON || args[0] == "--json", quiet || args[0] == "--quiet"
		args = args[1:]
	}
	c, err := Parse(args, asJSON, quiet)
	if err != nil {
		t.Fatalf("Parse(%q) = %v", args, err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// compact is a --json object on one line.
func compact(text string) string {
	var b bytes.Buffer
	if json.Compact(&b, []byte(text)) != nil {
		return text
	}
	return b.String()
}

// usageOf is whether err is the usage error saying text.
func usageOf(err error, text string) bool {
	var u out.UsageError
	return errors.As(err, &u) && err.Error() == text
}

const nightlyText = "version: 1\nci: { steps: [step a], nightly: { steps: [echo one, { run: echo two, cost: static }] } }\n"

// ci plan prints the nightly's steps, or a range's, one a line, or the plan
// as JSON with its range; it runs nothing.
func TestPlanPrintsThePlan(t *testing.T) {
	p := over(t, nightlyText, nil)
	code, stdout, stderr, err := call(t, p, "plan", "--nightly")
	if code != 0 || err != nil || stdout != "late  echo one\nstatic  echo two\n" || stderr != "" {
		t.Errorf("plan --nightly = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = call(t, p, "plan", start, "HEAD")
	if code != 0 || err != nil || stdout != "late  step a\n" {
		t.Errorf("plan start HEAD = exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--json", "plan", start, "HEAD")
	if code != 0 || err != nil || !strings.Contains(compact(stdout), `"range":{"from":"`+start+`","to":"`+head+`","readable":true,"nightly":false}`) {
		t.Errorf("--json plan start HEAD = exit %d, %v, stdout %q", code, err, stdout)
	}
	if len(p.ran) != 0 {
		t.Errorf("plan ran %d plans, want none", len(p.ran))
	}
}

// ci plan with no range is a usage error but under a stealth config, which
// plans the unpushed commits up to HEAD.
func TestPlanWithNoRangeTakesTheUnpushedCommitsOfAStealthConfig(t *testing.T) {
	p := over(t, nightlyText, nil)
	if _, _, _, err := call(t, p, "plan"); !usageOf(err, "ci plan needs <from> <to>, --nightly or --whole") {
		t.Errorf("plan = %v, want the range needed", err)
	}
	p.stealth = true
	code, stdout, _, err := call(t, p, "--json", "plan")
	if code != 0 || err != nil || !strings.Contains(compact(stdout), `"range":{"from":"`+vcsport.Unpushed+`","to":"`+head+`"`) {
		t.Errorf("--json plan, stealth = exit %d, %v, stdout %q", code, err, stdout)
	}
}

// A ref that names no commit is refused before the config is read; a
// config that does not load, and data that cannot be read, stop the plan.
func TestPlanRefusesWhatItCannotPlan(t *testing.T) {
	p := over(t, nightlyText, nil)
	p.err = errors.New("the config does not load")
	if _, _, _, err := call(t, p, "plan", "nope", "HEAD"); !usageOf(err, "ci plan: nope is not a commit of this repository") {
		t.Errorf("plan nope HEAD = %v, want the ref refused", err)
	}
	if _, _, _, err := call(t, p, "plan", "--whole", "--data-at", "nope"); !usageOf(err, "ci plan: nope is not a commit of this repository") {
		t.Errorf("plan --data-at nope = %v, want the ref refused", err)
	}
	if _, _, _, err := call(t, p, "plan", "--nightly"); !errors.Is(err, p.err) {
		t.Errorf("plan over a config that does not load = %v, want its error", err)
	}
	p.err = nil
	if _, stdout, _, err := call(t, p, "plan", "--whole", "--data-at", "HEAD"); err == nil || stdout != "" {
		t.Errorf("plan --data-at a tree the store lacks = %q, %v, want its error", stdout, err)
	}
}

// ci run carries the plan out through RunPlan, its exit the run's, with
// itos's streams and flags; a ref that names no commit, or a plan that
// cannot be made, runs nothing.
func TestRunCarriesThePlanOut(t *testing.T) {
	p := over(t, nightlyText, nil)
	p.code = 1
	code, _, _, err := call(t, p, "--json", "run", "--nightly")
	if code != 1 || err != nil || len(p.ran) != 1 || len(p.ran[0].Order) != 2 || !p.options[0].JSON || !p.options[0].Nightly {
		t.Errorf("run --nightly = exit %d, %v, ran %d plans, options %+v", code, err, len(p.ran), p.options)
	}
	p.code, p.runErr = 75, errors.New("a step could not start")
	if code, _, _, err := call(t, p, "run", start, "HEAD"); code != 75 || !errors.Is(err, p.runErr) {
		t.Errorf("run start HEAD = exit %d, %v, want the run's", code, err)
	}
	p.ran = nil
	if _, _, _, err := call(t, p, "run", "nope", "HEAD"); !usageOf(err, "ci run: nope is not a commit of this repository") || p.ran != nil {
		t.Errorf("run nope HEAD = %v, ran %d, want the ref refused", err, len(p.ran))
	}
	p.err = errors.New("the config does not load")
	if _, _, _, err := call(t, p, "run", "--nightly"); !errors.Is(err, p.err) || p.ran != nil {
		t.Errorf("run over a config that does not load = %v, ran %d, want its error", err, len(p.ran))
	}
}

// ci run --tests carries out the plan's run of named tests alone; with
// none it says so, on stderr under --json, and the empty plan runs.
func TestRunTestsSaysWhenThereAreNoNamedTests(t *testing.T) {
	const said = "ci run --tests: no named tests to run (no tests: step, or nothing selected), so nothing ran.\n"
	p := over(t, nightlyText, nil)
	code, stdout, stderr, err := call(t, p, "run", "--nightly", "--tests")
	if code != 0 || err != nil || stdout != said || stderr != "" || len(p.ran) != 1 || len(p.ran[0].Order) != 0 {
		t.Errorf("run --nightly --tests = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = call(t, p, "--json", "run", "--nightly", "--tests")
	if code != 0 || err != nil || stdout != "" || stderr != said {
		t.Errorf("--json run --nightly --tests = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p = over(t, "version: 1\nci: { steps: [], nightly: { steps: [echo one, { tests: scenario }] } }\ntests: { scenario: { root: features, id: \"ID-\\\\d+\", run: { whole: run all } } }\n", nil)
	code, stdout, _, err = call(t, p, "run", "--nightly", "--tests")
	if code != 0 || err != nil || stdout != "" || len(p.ran) != 1 || len(p.ran[0].Order) != 1 {
		t.Errorf("run --nightly --tests with a tests step = exit %d, %v, stdout %q, ran %+v", code, err, stdout, p.ran)
	}
}

// ci scope says whether the range changes prose alone, text or --json,
// after its refs and the config.
func TestScopeSaysWhetherTheRangeIsProse(t *testing.T) {
	text := "version: 1\nci: { steps: [], prose: { paths: [\"**/*.md\"], steps: [echo prose] } }\n"
	prose := over(t, text, map[string]string{"README.md": "read me\n"})
	code, stdout, _, err := call(t, prose, "scope", start, "HEAD")
	if code != 0 || err != nil || stdout != "docs_only=true\n" {
		t.Errorf("scope of prose = exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, over(t, text, map[string]string{"README.md": "read me\n", "main.go": "package main\n"}), "--json", "scope", start, "HEAD")
	if code != 0 || err != nil || compact(stdout) != `{"schema":1,"docs_only":false}` {
		t.Errorf("--json scope of code = exit %d, %v, stdout %q", code, err, stdout)
	}
	if _, _, _, err := call(t, prose, "scope", start, "nope"); !usageOf(err, "ci scope: nope is not a commit of this repository") {
		t.Errorf("scope start nope = %v, want the ref refused", err)
	}
	prose.err = errors.New("the config does not load")
	if _, stdout, _, err := call(t, prose, "scope", start, "HEAD"); !errors.Is(err, prose.err) || stdout != "" {
		t.Errorf("scope over a config that does not load = %q, %v, want its error", stdout, err)
	}
}

// ci range is the pull request's base when given, else commits.since's
// commit with provider none, or one github does not answer; a config that
// does not load stops it.
func TestRangeSaysWhereTheRangeStarts(t *testing.T) {
	p := over(t, "version: 1\ncommits: { since: \""+start+"\" }\nci: { steps: [], range: { provider: none } }\n", nil)
	code, stdout, _, err := call(t, p, "range", "--head", head)
	if code != 0 || err != nil || stdout != "FROM="+start+"\n" {
		t.Errorf("range = exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err = call(t, p, "--json", "range", "--head", head, "--base", "b")
	if code != 0 || err != nil || compact(stdout) != `{"schema":1,"from":"b"}` {
		t.Errorf("--json range --base b = exit %d, %v, stdout %q", code, err, stdout)
	}
	github := over(t, "version: 1\ncommits: { since: \""+start+"\" }\nci: { steps: [], range: { provider: github } }\n", nil)
	github.env = map[string]string{providers.APIEnv: "http://127.0.0.1:1"}
	if _, stdout, _, err := call(t, github, "range", "--head", head); err != nil || stdout != "FROM="+start+"\n" {
		t.Errorf("range, github unanswered = %q, %v, want since", stdout, err)
	}
	p.err = errors.New("the config does not load")
	if _, stdout, _, err := call(t, p, "range", "--head", head); !errors.Is(err, p.err) || stdout != "" {
		t.Errorf("range over a config that does not load = %q, %v, want its error", stdout, err)
	}
}

// watched is a watcher whose look finds the run, completed as it ended.
func watched(conclusion string) providers.Watcher {
	return providers.Watcher{Look: func(string) (providers.Run, bool, error) {
		return providers.Run{URL: "https://ci/1", Status: "completed", Conclusion: conclusion}, true, nil
	}}
}

// ci watch waits for the commit's run with the provider for the branch's
// upstream: exit 0 when it passed, 1 when it did not, under --json its
// object with the commit.
func TestWatchWaitsForTheCommitsRun(t *testing.T) {
	p := over(t, "version: 1\n", nil)
	p.watcher = watched("success")
	p.Tracking = map[string][2]string{"main": {"upstream", "refs/heads/main"}}
	code, stdout, stderr, err := call(t, p, "watch")
	if code != 0 || err != nil || stdout != "CI run: https://ci/1\nCI passed: https://ci/1\n" || stderr != "" || p.remote != "upstream" {
		t.Errorf("watch = exit %d, %v, stdout %q, stderr %q, remote %q", code, err, stdout, stderr, p.remote)
	}
	p.watcher, p.OnBranch = watched("failure"), ""
	code, stdout, stderr, err = call(t, p, "--json", "watch", start)
	if code != 1 || err != nil || p.remote != "origin" || stderr != "CI run: https://ci/1\nCI failure: https://ci/1\n" ||
		!strings.HasPrefix(compact(stdout), `{"schema":1,"ok":false,"commit":"`+start+`","ci":"failure","run":{"url":"https://ci/1"`) {
		t.Errorf("--json watch start = exit %d, %v, stdout %q, stderr %q, remote %q", code, err, stdout, stderr, p.remote)
	}
	code, stdout, _, err = call(t, p, "--quiet", "watch")
	if code != 1 || err != nil || stdout != "" {
		t.Errorf("--quiet watch = exit %d, %v, stdout %q", code, err, stdout)
	}
}

// ci watch outside a repository says so, exit 3; a rev that names no
// commit is a usage error; a config that does not load stops it; a
// provider that cannot look ends it as an error, exit 3; provider none is
// a config error.
func TestWatchRefusesWhatItCannotWatch(t *testing.T) {
	p := over(t, "version: 1\n", nil)
	p.Located = nil
	code, stdout, stderr, err := call(t, p, "watch")
	if code != 3 || err != nil || stdout != "" || stderr != "itos: ci watch runs in a git repository, and this is none\n" {
		t.Errorf("watch outside a repository = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p = over(t, "version: 1\n", nil)
	if _, _, _, err := call(t, p, "watch", "nope"); !usageOf(err, "ci watch: nope is not a commit of this repository") {
		t.Errorf("watch nope = %v, want the rev refused", err)
	}
	p.wErr = errors.New("no token")
	code, stdout, stderr, err = call(t, p, "watch")
	if code != 3 || err != nil || stdout != "" || stderr != "itos: no token\n" {
		t.Errorf("watch, no token = exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = call(t, p, "--json", "watch")
	if code != 3 || err != nil || compact(stdout) != `{"schema":1,"ok":false,"commit":"`+head+`","ci":"error"}` {
		t.Errorf("--json watch, no token = exit %d, %v, stdout %q", code, err, stdout)
	}
	p.wErr, p.none = nil, true
	var invalid *config.Error
	if _, _, _, err := call(t, p, "watch"); !errors.As(err, &invalid) || !strings.Contains(err.Error(), "ci.watch.provider is none") {
		t.Errorf("watch, provider none = %v, want a config error", err)
	}
	p.err = errors.New("the config does not load")
	if _, _, _, err := call(t, p, "watch"); !errors.Is(err, p.err) {
		t.Errorf("watch over a config that does not load = %v, want its error", err)
	}
}
