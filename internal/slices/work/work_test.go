package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	"github.com/donvargax/itos/v8/internal/tests"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are work's ports over the fakes: a config or the error loading it,
// the working tree, the identity provider, the tracker's fake, the id
// ItemID gives, past the registry's ids it was given, or its error; what
// TaskChecks says of a task's checks (a failure, the evidence looked up), or
// its error, each task checked kept in checked as its id, during called as
// they run; and the watcher
// Watcher gives, or its error.
type ports struct {
	cfg        *config.Loaded
	err        error
	worktree   treeport.Source
	identity   providers.Identity
	tracker    trackerport.Tracker
	minted     string
	mintErr    error
	given      *[]string
	failure    string
	failureErr error
	evidence   []evidence.Entry
	// failureCode and failureTimedOut are the failure's exit code and
	// whether its timeout stopped it.
	failureCode     int
	failureTimedOut bool
	checked         *[]string
	during          func()
	watcher         providers.Watcher
	watcherErr      error
	// pr is what push.mode pr asks of the world: the branch the remote's HEAD
	// names or its error, the GitHub repository, and gh's stdout or its error,
	// each call of gh kept in ghCalls and each of the remote's in headCalls.
	pr
}

// pr are the answers the ports give under push.mode pr.
type pr struct {
	head       string
	headErr    error
	repository string
	ghOut      string
	ghErr      error
	ghCalls    *[][]string
	headCalls  *[]string
}

func (p ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p ports) Worktree(context.Context) treeport.Source { return p.worktree }

func (p ports) Identity(context.Context, *config.Loaded) providers.Identity { return p.identity }

func (p ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

func (p ports) ItemID(_ context.Context, _ *config.Loaded, kind string, registryIDs []string) (string, error) {
	if p.given != nil {
		*p.given = append([]string{kind}, registryIDs...)
	}
	return p.minted, p.mintErr
}

func (p ports) TaskChecks(_ context.Context, _ *config.Loaded, task ledger.Task, stderr io.Writer) (Checked, error) {
	if p.checked != nil {
		*p.checked = append(*p.checked, task.ID)
	}
	if p.during != nil {
		p.during()
	}
	checked := Checked{Evidence: p.evidence}
	if p.failure != "" {
		fmt.Fprintf(stderr, "%s: the check said why\n", task.ID)
		checked.Failure = &judge.Failure{Task: task.ID, Command: "false", Why: p.failure, Output: "why\n", Code: p.failureCode, TimedOut: p.failureTimedOut}
	}
	return checked, p.failureErr
}

func (p ports) Watcher(context.Context, *config.Loaded, string) (providers.Watcher, bool, error) {
	return p.watcher, p.watcherErr == nil, p.watcherErr
}

func (p ports) RemoteHead(_ context.Context, remote string, _ vcsport.Talk) (string, error) {
	if p.headCalls != nil {
		*p.headCalls = append(*p.headCalls, remote)
	}
	return p.head, p.headErr
}

func (p ports) Gh(_ context.Context, args ...string) (string, error) {
	if p.ghCalls != nil {
		*p.ghCalls = append(*p.ghCalls, args)
	}
	return p.ghOut, p.ghErr
}

func (p ports) GitHubRepository(context.Context, *config.Loaded, string) string { return p.repository }

// broken is the VCS fake with its log, a commit's patch, git show's text or a
// diff failing when set, and with fetch, when set, standing in for a fetch of
// a refspec: its error fails it, and it may add to the fake's history the
// commits it brings.
type broken struct {
	*vcsfake.VCS
	log, patch, shown, diff error
	fetch                   func(ctx context.Context, remote, refspec string, talk vcsport.Talk) error
}

func (b broken) Diff(ctx context.Context, from, to string) ([]string, error) {
	if b.diff != nil {
		return nil, b.diff
	}
	return b.VCS.Diff(ctx, from, to)
}

func (b broken) Fetch(ctx context.Context, remote, refspec string, talk vcsport.Talk) (string, error) {
	if b.fetch != nil {
		return "", b.fetch(ctx, remote, refspec, talk)
	}
	return b.VCS.Fetch(ctx, remote, refspec, talk)
}

func (b broken) Log(ctx context.Context, r vcsport.Range) ([]vcsport.Entry, error) {
	if b.log != nil {
		return nil, b.log
	}
	return b.VCS.Log(ctx, r)
}

func (b broken) Patch(ctx context.Context, rev string) (string, error) {
	if b.patch != nil {
		return "", b.patch
	}
	return b.VCS.Patch(ctx, rev)
}

func (b broken) Shown(ctx context.Context, revs []string, notes bool) (string, error) {
	if b.shown != nil {
		return "", b.shown
	}
	return b.VCS.Shown(ctx, revs, notes)
}

// flaky is a source whose reads of one file fail after the first good ones.
type flaky struct {
	treeport.Source
	file string
	good *int
}

func (f flaky) Read(ctx context.Context, p string) (string, error) {
	if p == f.file {
		if *f.good == 0 {
			return "", errors.New(p + " is gone")
		}
		*f.good--
	}
	return f.Source.Read(ctx, p)
}

// configText has a ledger of T-<n> tasks, the Task and Scenarios footers,
// the scenarios in features/, and the registry, the people and an identity
// provider that names nobody.
const configText = `version: 1
ledger:
  files: "tasks/phase-{group}.yaml"
  id: "T-\\d+"
commits:
  footers:
    Task: { source: ledger }
    Scenarios: { source: { tests: scenario }, strip_prefix: "@" }
tests:
  scenario:
    adapter: gherkin
    root: features
    id: "ID-[A-Z]+-\\d+"
    tag_prefix: "@"
    wip_tag: "@wip"
work:
  registry: work-items.yaml
  people: { source: yaml, file: people.yaml }
  identity: { provider: none, hint: say --as }
  tags: [cli]
`

// registryText has a task in progress for a, an idea waiting on it, a done
// slice and a deferred idea.
const registryText = `phases: { 1: null }
items:
  - { id: T-1, title: Adopt, phase: 1, owner: a, status: doing, depends_on: [], why: Adopt it. }
  - { id: p1-a, title: An idea, phase: 1, owner: null, status: todo, depends_on: [T-1], kind: idea, why: Because., tags: [cli] }
  - { id: slice-1, title: A slice, phase: 1, owner: b, status: done, depends_on: [] }
  - { id: p1-c, title: Put off, phase: 1, owner: null, status: todo, depends_on: [], kind: idea, why: C, deferred: later }
`

const ledgerText = "- id: T-1\n  type: chore\n  title: Adopt\n  why: Adopt it.\n  done_when:\n    - run: \"true\"\n"

const featureText = `Feature: Things

  @ID-THING-1 @p1-a
  Scenario: One
    Given nothing

  @wip @ID-THING-22 @p1-a
  Scenario: Two
    Given nothing
`

const asksText = `questions:
  - id: q-1
    question: "Open one,
      on two lines?"
    item: p1-a
  - id: q-12
    question: Answered?
    answer: Yes.
    item: p1-a
  - id: q-3
    question: Another's?
    item: T-1
`

// fixture is what a test's repository holds: the working tree's files, by
// path; the history, oldest first, HEAD the newest, none for a repository
// with no commit; the questions; the files the tracker holds beside the
// registry and the questions, as committed; and the VCS's failures.
type fixture struct {
	files             map[string]string
	history           []vcsfake.Commit
	questions         string
	config            string
	stealth           bool
	tracked           []string
	log, patch, shown error
}

// sound is a fixture with every file, a history whose commits link to the
// items, and the questions.
func sound() fixture {
	return fixture{
		files: map[string]string{
			"itos.yaml": configText, "work-items.yaml": registryText, "people.yaml": "- a\n- b\n",
			"tasks/phase-1.yaml": ledgerText, "features/things.feature": featureText,
			"other.yaml": "phases: { 1: null }\nitems:\n  - { id: x }\n",
		},
		history: []vcsfake.Commit{
			{Name: "c-1a", Message: "chore: start\n\nTask: T-1\n", Tree: map[string]string{"features/things.feature": featureText}},
			{Name: "c-2a", Parents: []string{"c-1a"}, Message: "docs: take p1-a\n", Tree: map[string]string{"features/things.feature": featureText, "a": "a\n"}},
			{Name: "c-3a", Parents: []string{"c-2a"}, Message: "feat: one\n\nScenarios: ID-THING-1\n", Tree: map[string]string{"features/things.feature": featureText, "a": "b\n"}},
			{Name: "c-4a", Parents: []string{"c-3a"}, Message: "chore: other\n\nTask: T-9\n", Tree: map[string]string{"features/things.feature": featureText, "a": "b\n"}},
		},
		questions: asksText,
	}
}

// over is the ports of a repository whose top is an empty folder the test
// stands in, holding f; the identity provider names nobody.
func over(t *testing.T, f fixture) ports {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for name, text := range f.files {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	common := filepath.Join(dir, ".git")
	v := &vcsfake.VCS{Root: dir, Located: map[string]vcsport.Location{"": {Top: dir, GitDir: common, Common: common}}, History: f.history}
	files := map[string]map[string]string{"worktree": f.files}
	if len(f.history) > 0 {
		head := f.history[len(f.history)-1].Name
		v.Named = map[string]string{"HEAD": head}
		files["HEAD"] = f.history[len(f.history)-1].Tree
	}
	trees := treefake.Trees{Common: common, Files: files}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	text := configText
	if f.config != "" {
		text = f.config
	}
	cfg, err := config.FromText(broken{VCS: v, log: f.log, patch: f.patch, shown: f.shown}, trees, src, "itos.yaml", text, f.stealth)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &trackerfake.Tracker{Stealth: f.stealth}
	var changes []trackerport.Change
	if f.questions != "" {
		changes = append(changes, trackerport.Change{Name: cfg.Work.Asks, Text: f.questions, Created: true})
	}
	if registry, ok := f.files[cfg.Work.Registry]; ok {
		changes = append(changes, trackerport.Change{Name: cfg.Work.Registry, Text: registry, Created: true})
	}
	for _, name := range f.tracked {
		changes = append(changes, trackerport.Change{Name: name, Text: f.files[name], Created: true})
	}
	if len(changes) > 0 {
		if _, err := tracker.Transact(t.Context(), trackerport.Transaction{Changes: changes,
			Paragraphs: []string{"chore: start"}, Quiet: true}); err != nil {
			t.Fatal(err)
		}
	}
	nobody := func() providers.Answer { return providers.Answer{Problem: "nobody is asked"} }
	return ports{cfg: cfg, worktree: src, identity: nobody, tracker: tracker}
}

// run is work's line over p: its exit code, stdout, stderr and error.
func run(t *testing.T, p Ports, args ...string) (int, string, string, error) {
	t.Helper()
	json, quiet := false, false
	for len(args) > 0 && (args[0] == "--json" || args[0] == "-q") {
		json, quiet = json || args[0] == "--json", quiet || args[0] == "-q"
		args = args[1:]
	}
	c, err := Parse(args, "", json, quiet)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// object is the --json object on stdout.
func object(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var o map[string]any
	if err := json.Unmarshal([]byte(stdout), &o); err != nil {
		t.Fatalf("stdout is no JSON object: %v\n%s", err, stdout)
	}
	return o
}

// failing is a writer that refuses every write.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("the writer is closed") }

// A config that cannot be read is every subcommand's error, but work check
// of a named file that is not there, found before the config is read.
func TestWorkReportsTheConfigsError(t *testing.T) {
	p := over(t, sound())
	p.cfg, p.err = nil, errors.New("no config")
	for _, line := range [][]string{nil, {"list"}, {"check"}, {"check", "work-items.yaml"}, {"show", "T-1"}} {
		if code, stdout, stderr, err := run(t, p, line...); code != 0 || err == nil || err.Error() != "no config" || stdout+stderr != "" {
			t.Errorf("work %q = %d, %v, %q %q", line, code, err, stdout, stderr)
		}
	}
	if code, _, stderr, err := run(t, p, "check", "nope.yaml"); code != refused || err != nil ||
		stderr != "no work registry at nope.yaml\n" {
		t.Errorf("work check nope.yaml = %d, %v, %q", code, err, stderr)
	}
}

// work check judges the registry, or a file named: sound on stdout unless
// -q, its problems on stderr, exit 1; under --json one object either way.
func TestWorkCheck(t *testing.T) {
	p := over(t, sound())
	if code, stdout, stderr, err := run(t, p, "check"); code != 0 || err != nil || stdout != "work-items.yaml: sound\n" || stderr != "" {
		t.Errorf("work check = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	if code, stdout, _, err := run(t, p, "-q", "check", "work-items.yaml"); code != 0 || err != nil || stdout != "" {
		t.Errorf("work check -q = %d, %v, %q", code, err, stdout)
	}
	code, stdout, stderr, err := run(t, p, "--json", "check")
	if o := object(t, stdout); code != 0 || err != nil || stderr != "" || o["file"] != "work-items.yaml" || o["sound"] != true ||
		len(o["problems"].([]any)) != 0 {
		t.Errorf("work check --json = %d, %v, %v, %q", code, err, o, stderr)
	}
	code, _, stderr, err = run(t, p, "check", "other.yaml")
	if code != refused || err != nil || !strings.HasPrefix(stderr, "x: phase undefined is not listed\n") || strings.Contains(stderr, "fix:") {
		t.Errorf("work check other.yaml = %d, %v, %q", code, err, stderr)
	}
	code, stdout, stderr, err = run(t, p, "--json", "check", "nope.yaml")
	if o := object(t, stdout); code != refused || err != nil || stderr != "" || o["file"] != "nope.yaml" || o["sound"] != false ||
		len(o["problems"].([]any)) != 1 || o["problems"].([]any)[0].(map[string]any)["fix"] != "name the registry's file, or write one at nope.yaml" {
		t.Errorf("work check --json nope.yaml = %d, %v, %v, %q", code, err, o, stderr)
	}
	c, err := Parse([]string{"check"}, "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := c.Run(t.Context(), failing{}, failing{}, NewHandler(p)); code != 0 || err == nil || err.Error() != "the writer is closed" {
		t.Errorf("work check --json to a closed writer = %d, %v", code, err)
	}
	f := sound()
	f.files["work-items.yaml"] = "items: [\n"
	if code, _, _, err := run(t, over(t, f), "check"); code != 0 || err == nil {
		t.Errorf("work check of a registry that does not parse = %d, %v", code, err)
	}
}

// The proposal is what the person can start, once the registry is sound:
// its problems on stderr whatever --json says, exit 1.
func TestWorkProposes(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := run(t, p, "--as", "a")
	if code != 0 || err != nil || stderr != "" || !strings.HasPrefix(stdout, "Working for a.\n") || !strings.Contains(stdout, "T-1") {
		t.Errorf("work --as a = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = run(t, p, "--json", "--as", "a")
	if o := object(t, stdout); code != 0 || err != nil || o["person"] != "a" {
		t.Errorf("work --json --as a = %d, %v, %v", code, err, o)
	}
	f := sound()
	f.files["work-items.yaml"] = "phases: { 1: null }\nitems:\n  - { id: x }\n"
	code, stdout, stderr, err = run(t, over(t, f), "--json")
	if code != refused || err != nil || stdout != "" || stderr == "" || strings.Contains(stderr, "fix:") {
		t.Errorf("work --json over problems = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	f.files["work-items.yaml"] = strings.Replace(registryText, "status: doing", "status: nope", 1)
	for _, g := range []string{"--json", "-q"} {
		code, stdout, stderr, err = run(t, over(t, f), g)
		if code != refused || err != nil || stdout != "" || stderr != "T-1: unknown status \"nope\"\n" {
			t.Errorf("work %s over one problem = %d, %v, %q, %q", g, code, err, stdout, stderr)
		}
	}
	f.files["work-items.yaml"] = "items: [\n"
	if code, _, _, err := run(t, over(t, f)); code != 0 || err == nil {
		t.Errorf("work over a registry that does not parse = %d, %v", code, err)
	}
	p.cfg = nil
	p.err = errors.New("no config")
	if code, _, _, err := run(t, p); code != 0 || err == nil {
		t.Errorf("work with no config = %d, %v", code, err)
	}
}

// Who the session is: --as among the people, else the provider's answer
// when the people list several. One the people do not list, or no answer,
// is said on stderr and the proposal is nobody's, or with --as exit 3.
func TestWorkSaysWhoTheSessionIs(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := run(t, p, "--as", "zed")
	if code != notAmong || err != nil || stdout != "" || stderr != "\"zed\" is not in people.yaml\n" {
		t.Errorf("work --as zed = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = run(t, p)
	if code != 0 || err != nil || stderr != "nobody is asked\n" || !strings.Contains(stdout, "T-1") {
		t.Errorf("work, nobody = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p.identity = func() providers.Answer { return providers.Answer{Handle: "zed"} }
	code, stdout, stderr, err = run(t, p)
	if code != 0 || err != nil || stderr != "zed is not in people.yaml: nothing is theirs yet\n" || !strings.HasPrefix(stdout, "Working for zed.") {
		t.Errorf("work as zed, unlisted = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	p.identity = func() providers.Answer { return providers.Answer{Handle: "b"} }
	code, stdout, stderr, err = run(t, p)
	if code != 0 || err != nil || stderr != "" || !strings.HasPrefix(stdout, "Working for b.") {
		t.Errorf("work as b = %d, %v, %q, %q", code, err, stdout, stderr)
	}
}

// Under a stealth config with no --as nobody is asked: the session owns
// every item; --as still proposes that handle's.
func TestWorkUnderAStealthConfigOwnsEveryItem(t *testing.T) {
	f := sound()
	f.stealth = true
	p := over(t, f)
	f.files[p.cfg.Work.Registry] = registryText
	p = over(t, f)
	p.identity = func() providers.Answer { t.Error("the provider is asked"); return providers.Answer{} }
	code, stdout, _, err := run(t, p, "--json")
	if o := object(t, stdout); code != 0 || err != nil || o["every_item"] != true || o["person"] != nil {
		t.Errorf("work --json under a stealth config = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, p, "--json", "--as", "b")
	if o := object(t, stdout); code != 0 || err != nil || o["every_item"] != nil || o["person"] != "b" {
		t.Errorf("work --json --as b under a stealth config = %d, %v, %v", code, err, o)
	}
}

// The proposal's second reading of the registry failing is its error.
func TestWorkProposalReadsTheRegistryAgain(t *testing.T) {
	p := over(t, sound())
	good := 1
	p.cfg = p.cfg.DataFrom(flaky{Source: p.cfg.Source, file: "work-items.yaml", good: &good})
	if code, _, _, err := run(t, p); code != 0 || err == nil || err.Error() == "" {
		t.Errorf("work, the registry gone = %d, %v", code, err)
	}
	good = 1
	if code, _, _, err := run(t, p, "list"); code != 0 || err != nil {
		t.Errorf("work list, the registry read once = %d, %v", code, err)
	}
}

// work list is the open items, with --all every one, with --tag those
// carrying it; a registry that is not there is said on stderr, exit 1.
func TestWorkList(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := run(t, p, "list")
	if code != 0 || err != nil || stderr != "" || !strings.Contains(stdout, "p1-c") || strings.Contains(stdout, "slice-1") {
		t.Errorf("work list = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, _, err = run(t, p, "list", "--all", "--tag", "cli")
	if code != 0 || err != nil || !strings.Contains(stdout, "p1-a") || strings.Contains(stdout, "T-1") {
		t.Errorf("work list --all --tag cli = %d, %v, %q", code, err, stdout)
	}
	code, stdout, _, err = run(t, p, "--json", "list", "--all")
	o := object(t, stdout)
	if items, _ := o["items"].([]any); code != 0 || err != nil || o["file"] != "work-items.yaml" || len(items) != 4 {
		t.Errorf("work list --json --all = %d, %v, %v", code, err, o)
	}
	f := sound()
	delete(f.files, "work-items.yaml")
	code, stdout, stderr, err = run(t, over(t, f), "--json", "list")
	if code != refused || err != nil || stdout != "" || stderr != "no work registry at work-items.yaml\n" {
		t.Errorf("work list --json, no registry = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	a, err := NewHandler(over(t, f)).Handle(t.Context(), Query{Sub: "list"}, Out{})
	if err != nil || len(a.Found) != 1 || a.Found[0].Fix != "move the registry to work-items.yaml, or set work.registry to where it is" {
		t.Errorf("work list's answer, no registry = %+v, %v", a, err)
	}
	f.files["work-items.yaml"] = "items: [\n"
	if code, _, _, err := run(t, over(t, f), "list"); code != 0 || err == nil {
		t.Errorf("work list of a registry that does not parse = %d, %v", code, err)
	}
}

// work show is the item, the items waiting on it, a task's why from its
// ledger entry, its scenarios at HEAD, the questions naming it and the
// commits that belong to it.
func TestWorkShow(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := run(t, p, "show", "p1-a")
	want := `
Scenarios tagged @p1-a:
  @ID-THING-1   live  things.feature
  @ID-THING-22  wip   things.feature

Questions naming p1-a:
  q-1   open      Open one, on two lines?
  q-12  answered  Answered?

Commits, oldest first:
  c-2 docs: take p1-a
  c-3 feat: one
`
	if code != 0 || err != nil || stderr != "" || !strings.HasPrefix(stdout, "p1-a  An idea\n") || !strings.HasSuffix(stdout, want) {
		t.Errorf("work show p1-a = %d, %v, %q\n%s", code, err, stderr, stdout)
	}
	code, stdout, _, err = run(t, p, "show", "T-1", "--patch")
	if code != 0 || err != nil || !strings.Contains(stdout, "Adopt it.") || !strings.Contains(stdout, "depended on by: p1-a") ||
		!strings.Contains(stdout, "\nScenarios: none tagged @T-1\n") || strings.Contains(stdout, "Questions naming T-1") == false ||
		!strings.Contains(stdout, "Commits, oldest first:\n  c-1 chore: start\n\ncommit c-1a\n\nchore: start") {
		t.Errorf("work show T-1 --patch = %d, %v\n%s", code, err, stdout)
	}
	code, stdout, _, err = run(t, p, "show", "slice-1", "--patch")
	if code != 0 || err != nil || !strings.HasSuffix(stdout, "\nCommits: none yet\n") {
		t.Errorf("work show slice-1 --patch = %d, %v\n%s", code, err, stdout)
	}
	code, stdout, _, err = run(t, p, "show", "p1-c")
	if code != 0 || err != nil || !strings.HasSuffix(stdout, "\nScenarios: none tagged @p1-c\n\nCommits: none yet\n") {
		t.Errorf("work show p1-c = %d, %v\n%s", code, err, stdout)
	}
}

// work show --json is one object; with --patch each commit's message and
// diff, a patch that cannot be read the error before anything is printed.
func TestWorkShowJSON(t *testing.T) {
	p := over(t, sound())
	code, stdout, _, err := run(t, p, "--json", "show", "T-1", "--patch")
	o := object(t, stdout)
	commits, _ := o["commits"].([]any)
	if code != 0 || err != nil || o["ok"] != true || o["ledger_why"] != "Adopt it." || len(commits) != 1 {
		t.Fatalf("work show --json T-1 --patch = %d, %v, %v", code, err, o)
	}
	if c := commits[0].(map[string]any); c["sha"] != "c-1a" || c["short"] != "c-1" || c["header"] != "chore: start" ||
		c["message"] != "chore: start\n\nTask: T-1" || !strings.HasPrefix(c["patch"].(string), "diff features/things.feature\n") {
		t.Errorf("work show --json T-1 --patch's commit = %v", c)
	}
	if item := o["item"].(map[string]any); item["why"] != nil {
		t.Errorf("work show --json T-1's item repeats its ledger why: %v", item)
	}
	code, stdout, _, err = run(t, p, "--json", "show", "p1-a")
	o = object(t, stdout)
	commits, _ = o["commits"].([]any)
	scenarios, _ := o["scenarios"].([]any)
	questions, _ := o["questions"].([]any)
	if _, task := o["ledger_why"]; code != 0 || err != nil || task || len(commits) != 2 || len(scenarios) != 2 || len(questions) != 2 ||
		commits[0].(map[string]any)["patch"] != nil {
		t.Errorf("work show --json p1-a = %d, %v, %v", code, err, o)
	}
	code, stdout, _, err = run(t, p, "--json", "show", "p1-c", "--patch")
	o = object(t, stdout)
	if code != 0 || err != nil || len(o["scenarios"].([]any)) != 0 || len(o["questions"].([]any)) != 0 || len(o["commits"].([]any)) != 0 {
		t.Errorf("work show --json p1-c --patch = %d, %v, %v", code, err, o)
	}
	f := sound()
	f.files["tasks/phase-1.yaml"] = "- id: T-2\n  type: chore\n  title: Other\n  done_when:\n    - run: \"true\"\n"
	code, stdout, _, err = run(t, over(t, f), "--json", "show", "T-1")
	if o := object(t, stdout); code != 0 || err != nil || o["ledger_why"] != nil {
		t.Errorf("work show --json T-1, no ledger entry = %d, %v, %v", code, err, o)
	}
	f.files["tasks/phase-1.yaml"] = "- [\n"
	code, stdout, _, err = run(t, over(t, f), "--json", "show", "T-1")
	if o := object(t, stdout); code != 0 || err != nil || o["ledger_why"] != nil {
		t.Errorf("work show --json T-1, a ledger that cannot be read = %d, %v, %v", code, err, o)
	}
	// With no ledger.id an item of no kind is no task.
	f = sound()
	f.config = strings.Replace(configText, "  id: \"T-\\\\d+\"\n", "", 1)
	code, stdout, _, err = run(t, over(t, f), "--json", "show", "T-1")
	if o := object(t, stdout); code != 0 || err != nil || o["ledger_why"] != nil || o["item"].(map[string]any)["why"] != "Adopt it." {
		t.Errorf("work show --json T-1, no ledger.id = %d, %v, %v", code, err, o)
	}
	if _, task := object(t, stdout)["ledger_why"]; task {
		t.Errorf("work show --json T-1, no ledger.id, is a task: %s", stdout)
	}
	f = sound()
	f.patch = errors.New("no patch")
	code, stdout, _, err = run(t, over(t, f), "--json", "show", "T-1", "--patch")
	if code != 0 || err == nil || err.Error() != "no patch" || stdout != "" {
		t.Errorf("work show --json --patch, a patch that cannot be read = %d, %v, %q", code, err, stdout)
	}
	// Without --patch no patch is read.
	code, stdout, _, err = run(t, over(t, f), "--json", "show", "T-1")
	if o := object(t, stdout); code != 0 || err != nil || len(o["commits"].([]any)) != 1 {
		t.Errorf("work show --json, a patch that cannot be read = %d, %v, %v", code, err, o)
	}
}

// work show refuses a registry that is not there and an id no item has,
// with their fixes, or under --json on stdout, exit 1.
func TestWorkShowRefuses(t *testing.T) {
	p := over(t, sound())
	code, stdout, stderr, err := run(t, p, "show", "nope")
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "nope") {
		t.Errorf("work show nope = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	f := sound()
	delete(f.files, "work-items.yaml")
	code, stdout, stderr, err = run(t, over(t, f), "show", "T-1")
	if code != refused || err != nil || stdout != "" ||
		stderr != "no work registry at work-items.yaml\n  fix: move the registry to work-items.yaml, or set work.registry to where it is\n" {
		t.Errorf("work show, no registry = %d, %v, %q, %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = run(t, over(t, f), "--json", "show", "T-1")
	if o := object(t, stdout); code != refused || err != nil || stderr != "" || o["ok"] != false || len(o["problems"].([]any)) != 1 {
		t.Errorf("work show --json, no registry = %d, %v, %v, %q", code, err, o, stderr)
	}
	f.files["work-items.yaml"] = "items: [\n"
	if code, _, _, err := run(t, over(t, f), "show", "T-1"); code != 0 || err == nil {
		t.Errorf("work show of a registry that does not parse = %d, %v", code, err)
	}
}

// What work show cannot read is its error: the scenarios, the history, the
// questions; git show's text after the commits are listed.
func TestWorkShowReportsWhatItCannotRead(t *testing.T) {
	f := sound()
	f.config = strings.Replace(configText, "    id: \"ID-[A-Z]+-\\\\d+\"\n", "", 1)
	if code, _, _, err := run(t, over(t, f), "show", "p1-a"); code != 0 || err == nil || !strings.Contains(err.Error(), "tests.scenario.id is missing") {
		t.Errorf("work show, scenarios that cannot be read = %d, %v", code, err)
	}
	f = sound()
	f.log = errors.New("no log")
	if code, _, _, err := run(t, over(t, f), "show", "p1-a"); code != 0 || err == nil || err.Error() != "no log" {
		t.Errorf("work show, no log = %d, %v", code, err)
	}
	f = sound()
	f.questions = "questions: [\n"
	if code, _, _, err := run(t, over(t, f), "show", "p1-a"); code != 0 || err == nil {
		t.Errorf("work show, questions that do not parse = %d, %v", code, err)
	}
	f = sound()
	f.shown = errors.New("no show")
	code, stdout, _, err := run(t, over(t, f), "show", "p1-a", "--patch")
	if code != 0 || err == nil || err.Error() != "no show" || !strings.HasSuffix(stdout, "  c-3 feat: one\n") {
		t.Errorf("work show --patch, no git show = %d, %v\n%s", code, err, stdout)
	}
}

// Before a first commit, work show reads the scenarios from the working
// tree and lists no commit.
func TestWorkShowBeforeAFirstCommit(t *testing.T) {
	f := sound()
	f.history = nil
	code, stdout, _, err := run(t, over(t, f), "show", "p1-a", "--patch")
	if code != 0 || err != nil || !strings.Contains(stdout, "@ID-THING-22  wip   things.feature") ||
		!strings.HasSuffix(stdout, "\nCommits: none yet\n") {
		t.Errorf("work show p1-a with no commit = %d, %v\n%s", code, err, stdout)
	}
}

// A commit belongs to an item when itos's registry commit names it in its
// header, or a footer links to it: by its id in a footer of IDs, or by one of
// its scenarios, tagged or not (linksTo); an id in a footer of tests is a
// scenario's, never the item's.
func TestBelongingLinksTheItemsCommits(t *testing.T) {
	cfg := &config.Loaded{}
	cfg.Tests = config.Ordered[config.Kind]{Keys: []string{"scenario"}, Values: map[string]config.Kind{"scenario": {TagPrefix: "@"}}}
	scenarios := []tests.Test{{ID: "@ID-A-01"}, {ID: "ID-A-02"}}
	history := []message.Logged{
		{SHA: "1", Header: "docs: take slice-9"},
		{SHA: "2", Header: "docs: take slice-1"},
		{SHA: "3", Header: "chore: one", Links: []message.Link{{ID: "slice-9"}}},
		{SHA: "4", Header: "chore: two", Links: []message.Link{{ID: "slice-1"}}},
		{SHA: "5", Header: "feat: three", Links: []message.Link{{ID: "ID-A-01", Tests: "scenario"}}},
		{SHA: "6", Header: "feat: four", Links: []message.Link{{ID: "ID-A-02", Tests: "scenario"}}},
		{SHA: "7", Header: "feat: five", Links: []message.Link{{ID: "slice-9", Tests: "scenario"}, {ID: "ID-A-09", Tests: "scenario"}}},
	}
	var got []string
	for _, c := range belonging(cfg, "slice-9", scenarios, history) {
		got = append(got, c.SHA)
	}
	if strings.Join(got, " ") != "1 3 5 6" {
		t.Fatalf("the commits of slice-9 are %v", got)
	}
}
