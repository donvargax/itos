package attempt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The commits of the fake's history, each a full SHA: the base the brief
// was computed at; past it on the item's branch, a test commit, a feat
// commit, and a merge of main's feat commit, which is not the item's.
var (
	base    = strings.Repeat("b", 40)
	tested  = strings.Repeat("1", 40)
	feat    = strings.Repeat("2", 40)
	onMain  = strings.Repeat("3", 40)
	merged  = strings.Repeat("4", 40)
	request = "sha256:" + strings.Repeat("a", 64)
)

const itosYAML = "version: 1\nwork:\n  registry: work-items.yaml\ntests:\n  scenario: { root: features, id: \"ID-[A-Z]+-\\\\d+\" }\n"

const registry = "phases: { 1: null }\nitems:\n  - { id: slice-9, phase: 1, owner: null, status: todo, depends_on: [] }\n"

// ports are the attempt command's ports over the fakes: the config over a
// fake repository and trees, the requests brief gives by role and base,
// the files, the journal and what was recorded, and errors to fail with.
type ports struct {
	v          *vcsfake.VCS
	trees      treefake.Trees
	tweak      func(*config.Loaded)
	configErr  error
	requests   map[string]string
	problems   []out.Problem
	requestErr error
	asked      []string
	files      map[string]string
	journal    string
	journalErr error
	recordErr  error
}

func (p *ports) Config(ctx context.Context) (*config.Loaded, error) {
	if p.configErr != nil {
		return nil, p.configErr
	}
	src, err := p.trees.At(ctx, "worktree")
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(p.v, p.trees, src, "itos.yaml")
	if err == nil && p.tweak != nil {
		p.tweak(cfg)
	}
	return cfg, err
}

func (p *ports) Request(_ context.Context, id, role, at string) (string, []out.Problem, error) {
	p.asked = append(p.asked, id+" "+role+" "+at)
	return p.requests[role+" "+at], p.problems, p.requestErr
}

func (p *ports) ReadFile(_ context.Context, path string) ([]byte, error) {
	text, ok := p.files[path]
	if !ok {
		return nil, errors.New("open " + path + ": no such file or directory")
	}
	return []byte(text), nil
}

func (p *ports) Journal(context.Context, string) ([]byte, error) {
	return []byte(p.journal), p.journalErr
}

func (p *ports) Record(_ context.Context, _ string, line []byte) error {
	if p.recordErr != nil {
		return p.recordErr
	}
	p.journal += string(line)
	return nil
}

// over is the ports of a clone on main whose remote's branch slice-9 is at
// tip, and whose trees at each commit past the base hold the feature file
// given for it: a feature with the one scenario, its tags those given.
func over(tip string, features map[string]string) *ports {
	common := "/work/.git"
	v := &vcsfake.VCS{
		Root:    "/work",
		Located: map[string]vcsport.Location{"": {Top: "/work", GitDir: common, Common: common}},
		History: []vcsfake.Commit{
			{Name: base, Message: "test: a tagged scenario\n"},
			{Name: onMain, Parents: []string{base}, Message: "feat: main's own\n"},
			{Name: tested, Parents: []string{base}, Message: "test: add the steps\n"},
			{Name: merged, Parents: []string{tested, onMain}, Message: "Merge main into slice-9\n"},
			{Name: feat, Parents: []string{merged}, Message: "feat: say the next step\n"},
		},
		Named:    map[string]string{"HEAD": base, "refs/heads/main": base},
		OnBranch: "main",
		Remote:   map[string]*vcsfake.Remote{"origin": {URL: "/remote", Refs: map[string]string{"refs/heads/main": base}}},
	}
	if tip != "" {
		v.Remote["origin"].Refs["refs/heads/slice-9"] = tip
	}
	files := map[string]map[string]string{"worktree": {"itos.yaml": itosYAML, "work-items.yaml": registry}}
	for sha, tags := range features {
		files[sha] = map[string]string{"features/a.feature": "Feature: A\n\n  @ID-A-01 " + tags + "\n  Scenario: one\n    When it runs\n"}
	}
	return &ports{
		v:        v,
		trees:    treefake.Trees{Common: common, Files: files},
		requests: map[string]string{"drafter " + base: request, "implementer " + base: request},
		files:    map[string]string{},
	}
}

// result is a result's JSON with the keys given, each a JSON value as
// written, in the contract's order and any other after.
func result(fields map[string]string) string {
	var parts []string
	for _, key := range append(append([]string{}, keys...), "commits") {
		if v, ok := fields[key]; ok {
			parts = append(parts, fmt.Sprintf("%q:%s", key, v))
		}
	}
	return "{" + strings.Join(parts, ",") + "}\n"
}

// finished is a done result of the role at attempt n, claiming head.
func finished(role string, n int, head string) map[string]string {
	return map[string]string{"schema": "1", "item": `"slice-9"`, "role": `"` + role + `"`, "request": `"` + request + `"`,
		"base": `"` + base + `"`, "attempt": fmt.Sprint(n), "outcome": `"done"`, "head": `"` + head + `"`}
}

func submit(t *testing.T, p *ports, text string) Answer {
	t.Helper()
	p.files["result.json"] = text
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9", File: "result.json", Path: "result.json"})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	return a
}

// refusedFor is the one problem a is refused for, whose rule is rule.
func refusedFor(t *testing.T, a Answer, rule string, says ...string) out.Problem {
	t.Helper()
	if len(a.Refused) != 1 || a.Refused[0].Rule != rule {
		t.Fatalf("refused %+v, taken %+v, want %s", a.Refused, a.Taken, rule)
	}
	for _, s := range says {
		if !strings.Contains(a.Refused[0].Message, s) {
			t.Errorf("the %s message %q does not say %q", rule, a.Refused[0].Message, s)
		}
	}
	return a.Refused[0]
}

// records are the journal's lines, each decoded.
func records(t *testing.T, p *ports) []map[string]any {
	t.Helper()
	var found []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(p.journal), "\n") {
		if line == "" {
			continue
		}
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("journal line %q: %v", line, err)
		}
		found = append(found, r)
	}
	return found
}

// A drafter's done result, its head the remote's tip holding the item's
// scenario @wip, is accepted and recorded, its notes kept; the same result
// with other notes is already, nothing recorded; another outcome at the
// same attempt, or an attempt not the next, is out of order.
func TestDrafterAccepted(t *testing.T) {
	p := over(tested, map[string]string{tested: "@wip @slice-9"})
	fields := finished(drafter, 1, tested)
	fields["notes"] = `"The spec is pushed."`
	a := submit(t, p, result(fields))
	if a.Taken == nil || a.Taken.Already || a.Taken.Attempt != 1 || *a.Taken.Head != tested {
		t.Fatalf("taken %+v, refused %+v", a.Taken, a.Refused)
	}
	got := records(t, p)
	want := map[string]any{"schema": 1.0, "role": drafter, "attempt": 1.0, "request": request, "base": base, "outcome": "done",
		"head": tested, "notes": "The spec is pushed.", "verdict": accepted, "sha256": digest([]byte(result(fields)))}
	if len(got) != 1 || fmt.Sprint(got[0]) != fmt.Sprint(want) {
		t.Errorf("journal %v", got)
	}
	if p.v.Named["refs/remotes/origin/slice-9"] != tested {
		t.Errorf("the item's branch was not fetched: %v", p.v.Named)
	}
	fields["notes"] = `"Sent twice."`
	if a := submit(t, p, result(fields)); a.Taken == nil || !a.Taken.Already || len(records(t, p)) != 1 {
		t.Errorf("again: %+v, journal %q", a, p.journal)
	}
	fields["outcome"], fields["head"] = `"blocked"`, "null"
	refusedFor(t, submit(t, p, result(fields)), "attempt-out-of-order", "expected attempt 2")
	fields["attempt"] = "3"
	refusedFor(t, submit(t, p, result(fields)), "attempt-out-of-order", "slice-9's drafter attempt 3", "expected attempt 2")
	if len(records(t, p)) != 1 {
		t.Errorf("an out-of-order result was recorded: %q", p.journal)
	}
}

// Outcomes other than done claim nothing and fetch nothing, and count up
// for their role alone.
func TestNotDone(t *testing.T) {
	p := over("", nil)
	for i, outcome := range []string{"blocked", "needs_decision", "spec_defect"} {
		fields := finished(implementer, i+1, "")
		fields["outcome"] = `"` + outcome + `"`
		delete(fields, "head")
		if i == 0 {
			fields["head"] = "null"
			fields["notes"] = "null"
		}
		if a := submit(t, p, result(fields)); a.Taken == nil || a.Taken.Head != nil || a.Taken.Notes != nil {
			t.Fatalf("%s: %+v", outcome, a)
		}
	}
	if _, fetched := p.v.Named["refs/remotes/origin/slice-9"]; fetched || len(records(t, p)) != 3 {
		t.Errorf("fetched %v, journal %q", fetched, p.journal)
	}
	fields := finished(drafter, 1, "")
	fields["outcome"] = `"blocked"`
	delete(fields, "head")
	if a := submit(t, p, result(fields)); a.Taken == nil || a.Taken.Attempt != 1 {
		t.Errorf("the drafter's first: %+v", a)
	}
}

// A malformed result names each key at fault; one naming a role is
// recorded rotated, null for each key at fault and the journal's own
// attempt, once for the same bytes; one naming none is recorded for no
// one.
func TestMalformed(t *testing.T) {
	good := finished(drafter, 1, tested)
	with := func(key, value string) string {
		f := map[string]string{}
		for k, v := range good {
			f[k] = v
		}
		if value == "" {
			delete(f, key)
		} else {
			f[key] = value
		}
		return result(f)
	}
	for _, c := range []struct {
		name, text, says string
		rotated          bool
	}{
		{"not JSON", `{"schema":1,`, "result.json is not one JSON object", false},
		{"no value", `{"schema":}`, "result.json is not one JSON object", false},
		{"not closed", `{"schema":1`, "result.json is not one JSON object", false},
		{"closed as an array", `{"schema":1]`, "result.json is not one JSON object", false},
		{"empty", "", "result.json is not one JSON object", false},
		{"an array", "[1]", "it does not start with {", false},
		{"two objects", "{}{}", "something follows the object", false},
		{"a key twice", strings.Replace(with("notes", `"x"`), `"notes":"x"`, `"role":"drafter","notes":"x"`, 1), "gives the key role twice", false},
		{"notes twice", strings.Replace(with("notes", `"x"`), `"notes":"x"`, `"notes":"x","notes":"y"`, 1), "gives the key notes twice", true},
		{"head twice", strings.Replace(with("notes", `"x"`), `"notes":"x"`, `"head":null`, 1), "gives the key head twice", true},
		{"another key", with("commits", "[]"), "the key commits, which a result does not have", true},
		{"no schema", with("schema", ""), "has no schema", true},
		{"schema 2", with("schema", "2"), "schema is 2, not 1", true},
		{"no item", with("item", `""`), "item is \"\", not an item's id", true},
		{"a role", with("role", `"reviewer"`), "role is \"reviewer\"", false},
		{"request", with("request", `"sha256:x"`), "request is", true},
		{"base", with("base", `"abc"`), "base is \"abc\"", true},
		{"attempt text", with("attempt", `"1"`), "attempt is \"1\", not a positive integer", true},
		{"attempt zero", with("attempt", "0"), "attempt is 0", true},
		{"attempt fraction", with("attempt", "1.5"), "attempt is 1.5", true},
		{"outcome", with("outcome", `"finished"`), "outcome is \"finished\"", true},
		{"no head", with("head", ""), "has no head, which the outcome done needs", true},
		{"short head", with("head", `"abc1234"`), "head is \"abc1234\"", true},
		{"notes", with("notes", "3"), "notes is 3, not a text", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := over(tested, map[string]string{tested: "@wip @slice-9"})
			problem := refusedFor(t, submit(t, p, c.text), "attempt-malformed", c.says)
			if got := len(records(t, p)); got != map[bool]int{true: 1, false: 0}[c.rotated] {
				t.Errorf("%d records: %q", got, p.journal)
			}
			if c.rotated && (!strings.Contains(problem.Message, "recorded as slice-9's drafter attempt 1, rotated") || !strings.Contains(problem.Fix, "as attempt 2")) {
				t.Errorf("rotated: %+v", problem)
			}
			if !c.rotated && (!strings.Contains(problem.Message, "counts for no attempt") || problem.Fix == "") {
				t.Errorf("not rotated: %+v", problem)
			}
		})
	}
	p := over(tested, nil)
	text := strings.Replace(with("outcome", `"blocked"`), `"notes"`, `"x"`, 1)
	problem := refusedFor(t, submit(t, p, text), "attempt-malformed", "gives a head with the outcome blocked, which claims none")
	r := records(t, p)
	if len(r) != 1 || r[0]["head"] != nil || r[0]["outcome"] != "blocked" || r[0]["request"] != request || r[0]["notes"] != nil || r[0]["verdict"] != rotated {
		t.Errorf("rotated %+v, %+v", r, problem)
	}
	refusedFor(t, submit(t, p, text), "attempt-malformed", "the journal holds these bytes already")
	if len(records(t, p)) != 1 {
		t.Errorf("the same bytes were recorded again: %q", p.journal)
	}
	refusedFor(t, submit(t, p, with("schema", "2")), "attempt-malformed", "recorded as slice-9's drafter attempt 2")
	if r := records(t, p); len(r) != 2 || r[1]["attempt"] != 2.0 {
		t.Errorf("the second rotation: %+v", r)
	}
}

// A stale result is refused, nothing recorded: another item's, both ids
// named; a base the clone lacks once the branch is fetched; one brief
// refuses at the base; a request not brief's at the base.
func TestStale(t *testing.T) {
	p := over(tested, map[string]string{tested: "@wip @slice-9"})
	fields := finished(drafter, 1, tested)
	fields["item"] = `"slice-8"`
	fix := refusedFor(t, submit(t, p, result(fields)), "attempt-stale", "slice-8", "slice-9").Fix
	if fix != "brief again with itos brief slice-9 --role drafter --json, and submit a result of that brief" || len(p.asked) != 0 {
		t.Errorf("fix %q, brief asked %v", fix, p.asked)
	}
	fields = finished(drafter, 1, tested)
	fields["base"] = `"` + strings.Repeat("0", 40) + `"`
	refusedFor(t, submit(t, p, result(fields)), "attempt-stale", "base "+strings.Repeat("0", 40)+" is no commit of the clone")
	if p.v.Named["refs/remotes/origin/slice-9"] != tested {
		t.Error("the branch was not fetched for a base the clone lacks")
	}
	p.problems = []out.Problem{{Rule: "brief-no-spec", Message: "nothing is tagged"}}
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-stale", "itos brief refuses slice-9's drafter at the base "+base+": nothing is tagged")
	p.problems = nil
	p.requests["drafter "+base] = "sha256:" + strings.Repeat("f", 64)
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-stale", "is not the one itos brief slice-9 --role drafter computes at its base")
	if p.journal != "" {
		t.Errorf("a stale result was recorded: %q", p.journal)
	}
}

// A head the remote's branch does not have at its tip is refused, naming
// the tip when there is one; the remote is the upstream's of the branch
// the clone is on.
func TestHeadNotPushed(t *testing.T) {
	p := over("", nil)
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-head-not-pushed", "origin has no branch slice-9")
	p = over(feat, nil)
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-head-not-pushed", "the tip is "+feat)
	p = over("", map[string]string{tested: "@wip @slice-9"})
	p.v.Remote["up"] = &vcsfake.Remote{URL: "/up", Refs: map[string]string{"refs/heads/slice-9": tested}}
	p.v.Tracking = map[string][2]string{"main": {"up", "refs/heads/main"}}
	if a := submit(t, p, result(finished(drafter, 1, tested))); a.Taken == nil || p.v.Named["refs/remotes/up/slice-9"] != tested {
		t.Errorf("from the upstream's remote: %+v, %v", a, p.v.Named)
	}
	if p.journal == "" {
		t.Error("nothing recorded")
	}
}

// A drafter's head with no scenario tagged for the item, or one that is
// live, is refused, naming it.
func TestSpecMissing(t *testing.T) {
	p := over(tested, map[string]string{tested: "@wip @slice-90"})
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-spec-missing", "no scenario is tagged @slice-9 at the head "+tested)
	p = over(tested, map[string]string{tested: "@slice-9"})
	refusedFor(t, submit(t, p, result(finished(drafter, 1, tested))), "attempt-spec-missing", "@ID-A-01", "not @wip")
}

// An implementer's first-parent commits past the base: a test commit
// before the feat is red first, main's feat commit merged in counting for
// nothing; a feat with no test commit before it is refused, naming it; a
// scenario left @wip is refused; none past the base judges no order.
func TestImplementer(t *testing.T) {
	p := over(feat, map[string]string{feat: "@slice-9"})
	if a := submit(t, p, result(finished(implementer, 1, feat))); a.Taken == nil {
		t.Errorf("red first: %+v", a.Refused)
	}
	p = over(feat, map[string]string{feat: "@wip @slice-9"})
	refusedFor(t, submit(t, p, result(finished(implementer, 1, feat))), "attempt-still-wip", "@ID-A-01", "still @wip")
	p = over(feat, map[string]string{feat: "@slice-9"})
	p.v.History[2].Message = "docs: add the steps\n"
	refusedFor(t, submit(t, p, result(finished(implementer, 1, feat))), "attempt-not-red-first", "feat: say the next step ("+feat+")")
	p = over(base, map[string]string{base: "@slice-9"})
	if a := submit(t, p, result(finished(implementer, 1, base))); a.Taken == nil {
		t.Errorf("nothing past the base: %+v", a.Refused)
	}
	p = over(tested, map[string]string{tested: "@slice-9"})
	if a := submit(t, p, result(finished(implementer, 1, tested))); a.Taken == nil {
		t.Errorf("no feat past the base: %+v", a.Refused)
	}
}

// broken is a repository whose first-parent history cannot be read.
type broken struct{ *vcsfake.VCS }

func (broken) FirstParents(context.Context, string, int, int) ([]string, error) {
	return nil, errors.New("rev-list failed")
}

// Errors, each with its kind, and the refusals before the result is read.
func TestErrors(t *testing.T) {
	fails := func(name string, p *ports, text string, k kind.Kind, words string) {
		t.Helper()
		p.files["result.json"] = text
		_, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-9", File: "result.json", Path: "result.json"})
		if err == nil || kind.Of(err) != k || !strings.Contains(err.Error(), words) {
			t.Errorf("%s: %v (kind %d), want kind %d saying %q", name, err, kind.Of(err), k, words)
		}
	}
	ok := result(finished(drafter, 1, tested))
	p := over(tested, nil)
	p.configErr = kind.Wrap(kind.Usage, errors.New("no config"))
	fails("config", p, ok, kind.Usage, "no config")
	p = over(tested, nil)
	p.trees.Files["worktree"]["work-items.yaml"] = "items: [\n"
	fails("registry", p, ok, kind.Usage, "did not find expected")
	p = over(tested, nil)
	p.journalErr = errors.New("journal unread")
	fails("journal", p, ok, kind.Unknown, "journal unread")
	p = over(tested, nil)
	p.journal = "{\"schema\":1,\"role\":\"drafter\"}\n\nnot json\n"
	fails("journal line", p, ok, kind.Usage, "line 3 of slice-9's journal of attempts is not a record")
	p = over(tested, nil)
	p.requestErr = errors.New("brief failed")
	fails("request", p, ok, kind.Unknown, "brief failed")
	p = over(tested, nil)
	p.v.Remote["origin"].Down = true
	fails("remote down", p, ok, kind.Temporary, "fetching slice-9's branch from origin")
	p = over(tested, nil)
	delete(p.v.Remote, "origin")
	fails("no remote", p, ok, kind.Missing, "does not appear to be a git repository")
	p = over(tested, nil)
	p.v.Remote["origin"].Down = true
	blocked := finished(drafter, 1, "")
	blocked["outcome"], blocked["head"], blocked["base"] = `"blocked"`, "null", `"`+strings.Repeat("0", 40)+`"`
	fails("remote down for a base the clone lacks", p, result(blocked), kind.Temporary, "fetching slice-9's branch")
	p = over(tested, map[string]string{tested: "@wip @slice-9"})
	p.tweak = func(cfg *config.Loaded) {
		k := cfg.Tests.Values["scenario"]
		bad := "("
		k.ID = &bad
		cfg.Tests.Values["scenario"] = k
	}
	fails("ID pattern", p, ok, kind.Unknown, "error parsing regexp")
	p = over(feat, nil)
	p.tweak = func(cfg *config.Loaded) { cfg.VCS = broken{p.v} }
	fails("first parents", p, result(finished(implementer, 1, feat)), kind.Unknown, "rev-list failed")
	p = over(tested, map[string]string{tested: "@wip @slice-9"})
	p.recordErr = errors.New("disk full")
	fails("record", p, ok, kind.Unknown, "disk full")
	fails("record a rotation", p, `{"role":"drafter"}`, kind.Unknown, "disk full")

	p = over(tested, nil)
	delete(p.trees.Files["worktree"], "work-items.yaml")
	if a := submit(t, p, ok); len(a.Refused) != 1 || a.Refused[0].Rule != "work-registry-missing" {
		t.Errorf("no registry: %+v", a.Refused)
	}
	p = over(tested, nil)
	a, err := NewHandler(p).Handle(t.Context(), Query{ID: "slice-7", File: "x.json", Path: "x.json"})
	if err != nil || len(a.Refused) != 1 || a.Refused[0].Rule != "work-unknown-item" {
		t.Errorf("an unknown item: %+v, %v", a, err)
	}
	a, err = NewHandler(p).Handle(t.Context(), Query{ID: "slice-9", File: "missing.json", Path: "sub/missing.json"})
	if err != nil || len(a.Refused) != 1 || a.Refused[0].Rule != "attempt-malformed" || !strings.Contains(a.Refused[0].Message, "missing.json") || p.journal != "" {
		t.Errorf("an unreadable file: %+v, %v", a, err)
	}
}

// logged is a repository whose log gives the records log makes of the
// fake's.
type logged struct {
	*vcsfake.VCS
	log func([]vcsport.Entry) ([]vcsport.Entry, error)
}

func (l logged) Log(ctx context.Context, r vcsport.Range) ([]vcsport.Entry, error) {
	found, err := l.VCS.Log(ctx, r)
	if err != nil {
		return nil, err
	}
	return l.log(found)
}

// A range git cannot read, or a log that cannot be read, is the command's
// error; so is a commit past the base the log gives no record of (a
// message holding the byte git log's records are framed by), which is
// never passed over as though it were not there: here the feat commit,
// red first's own question.
func TestLogError(t *testing.T) {
	p := over(feat, map[string]string{feat: "@slice-9"})
	c := claim{v: p.v, id: "slice-9", Result: Result{Base: "gone", Head: &feat}}
	if _, err := c.untested(t.Context()); err == nil {
		t.Error("an unreadable range is no error")
	}
	c.Base = base
	c.v = logged{p.v, func([]vcsport.Entry) ([]vcsport.Entry, error) { return nil, errors.New("log failed") }}
	if _, err := c.untested(t.Context()); err == nil || err.Error() != "log failed" {
		t.Errorf("an unreadable log: %v", err)
	}
	c.v = logged{p.v, func(found []vcsport.Entry) ([]vcsport.Entry, error) {
		var kept []vcsport.Entry
		for _, e := range found {
			if e.SHA != feat {
				kept = append(kept, e)
			}
		}
		return kept, nil
	}}
	if first, err := c.untested(t.Context()); err == nil || kind.Of(err) != kind.Usage || !strings.Contains(err.Error(), feat+" past the base has no record") || first != nil {
		t.Errorf("a commit the log dropped: %v, %v", first, err)
	}
	p.v.History[2].Message = "docs: add the steps\n"
	c.v = logged{p.v, func(found []vcsport.Entry) ([]vcsport.Entry, error) { return found[:0], nil }}
	if _, err := c.untested(t.Context()); err == nil {
		t.Error("a log of no records over commits past the base is no error")
	}
}

// Parse: attempt submit, an id and a file, the file read from the folder
// the person stood in.
func TestParse(t *testing.T) {
	for args, want := range map[string]string{
		"":                        "attempt needs submit: itos attempt submit <id> <result.json>",
		"take":                    "attempt has no subcommand take: it takes submit, itos attempt submit <id> <result.json>",
		"submit":                  "attempt submit takes an item's id and a result file, not \"\": itos attempt submit <id> <result.json>",
		"submit slice-9":          "attempt submit takes an item's id and a result file, not \"slice-9\": itos attempt submit <id> <result.json>",
		"submit a b c":            "attempt submit takes an item's id and a result file, not \"a b c\": itos attempt submit <id> <result.json>",
		"submit --force a b.json": "attempt submit does not take --force",
	} {
		_, err := Parse(strings.Fields(args), "", false)
		var usage out.UsageError
		if err == nil || err.Error() != want || !errors.As(err, &usage) {
			t.Errorf("Parse(%q) = %v, want the usage error %q", args, err, want)
		}
	}
	if c, err := Parse([]string{"submit", "slice-9", "r.json"}, "sub", true); err != nil || c != (CLI{Query: Query{ID: "slice-9", File: "r.json", Path: filepath.Join("sub", "r.json")}, JSON: true}) {
		t.Errorf("Parse from sub = %+v, %v", c, err)
	}
	abs := filepath.Join(t.TempDir(), "r.json")
	if c, err := Parse([]string{"submit", "slice-9", abs}, "sub", false); err != nil || c.Path != abs || c.File != abs {
		t.Errorf("Parse of an absolute path = %+v, %v", c, err)
	}
}

// Run says the result taken in, one line, or --json's object; a refusal is
// its rule, message and fix on stderr, or the ok-false object; an error is
// the command line's.
func TestRun(t *testing.T) {
	call := func(p *ports, json bool) (int, string, string, error) {
		var stdout, stderr bytes.Buffer
		code, err := CLI{Query: Query{ID: "slice-9", File: "result.json", Path: "result.json"}, JSON: json}.Run(t.Context(), &stdout, &stderr, NewHandler(p))
		return code, stdout.String(), stderr.String(), err
	}
	p := over(tested, map[string]string{tested: "@wip @slice-9"})
	p.files["result.json"] = result(finished(drafter, 1, tested))
	if code, stdout, stderr, err := call(p, false); code != 0 || err != nil || stderr != "" || stdout != "accepted slice-9's drafter attempt 1, done at "+tested+"\n" {
		t.Errorf("text: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if code, stdout, _, err := call(p, false); code != 0 || err != nil || stdout != "already accepted slice-9's drafter attempt 1, done at "+tested+"\n" {
		t.Errorf("already: %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err := call(p, true)
	want := fmt.Sprintf("{\n  \"schema\": 1,\n  \"ok\": true,\n  \"outcome\": \"already\",\n  \"item\": \"slice-9\",\n  \"role\": \"drafter\",\n  \"attempt\": 1,\n  \"request\": %q,\n  \"base\": %q,\n  \"head\": %q\n}\n", request, base, tested)
	if code != 0 || err != nil || stdout != want {
		t.Errorf("json: %d, %v, %s", code, err, stdout)
	}
	fields := finished(drafter, 2, "")
	fields["outcome"] = `"blocked"`
	delete(fields, "head")
	p.files["result.json"] = result(fields)
	if code, stdout, _, err := call(p, false); code != 0 || err != nil || stdout != "accepted slice-9's drafter attempt 2, blocked\n" {
		t.Errorf("no head: %d, %v, %q", code, err, stdout)
	}
	p.files["result.json"] = result(finished(drafter, 5, tested))
	if code, stdout, stderr, err := call(p, false); code != 1 || err != nil || stdout != "" || stderr != "attempt-out-of-order: slice-9's drafter attempt 5 is out of order: expected attempt 3\n  fix: submit the result as attempt 3, the next of slice-9's drafter\n" {
		t.Errorf("refused: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if code, stdout, stderr, err := call(p, true); code != 1 || err != nil || stderr != "" || !strings.HasPrefix(stdout, "{\n  \"schema\": 1,\n  \"ok\": false,\n  \"problems\": [\n    {\n      \"rule\": \"attempt-out-of-order\"") {
		t.Errorf("refused json: %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p.configErr = errors.New("unread")
	if code, _, _, err := call(p, false); code != 0 || err == nil {
		t.Errorf("error: %d, %v", code, err)
	}
}

// The journal's identity of a result: the same optional texts are both
// none or both the same.
func TestSame(t *testing.T) {
	a, b := "a", "b"
	for _, c := range []struct {
		x, y *string
		want bool
	}{{nil, nil, true}, {&a, nil, false}, {nil, &a, false}, {&a, &b, false}, {&a, &a, true}} {
		if got := same(c.x, c.y); got != c.want {
			t.Errorf("same(%v, %v) = %v", c.x, c.y, got)
		}
	}
}
