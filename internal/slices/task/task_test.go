package task

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/record"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/ledger"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The config the tests read, but where one says otherwise: the ledger in
// tasks/phase-{group}.yaml, T-<n> its ids, the registry beside it.
const configText = "version: 1\nledger:\n  group: { numeric: true }\n  files: \"tasks/phase-{group}.yaml\"\n  id: \"T-\\\\d+\"\nwork: { registry: tasks/work-items.yaml }\n"

// The ledger and the registry the tests read: a task that passes, one that
// fails, one waiting for the push, one with no checks whose item is done
// and one whose item is not, and in group 2 one more.
var files = map[string]string{
	"tasks/phase-1.yaml": `- { id: T-1, type: chore, title: Passes, done_when: [{ run: "true" }] }
- { id: T-2, type: docs, title: Fails, done_when: [{ run: "true" }, { fails: "true" }] }
- { id: T-3, type: chore, title: Waits, done_when: [{ run: "true", after: push }] }
- { id: T-4, type: chore, title: Done item }
- { id: T-5, type: chore, title: In review }
`,
	"tasks/phase-2.yaml": "- { id: T-6, type: chore, title: Later }\n",
	"tasks/work-items.yaml": `phases: { 1: null, 2: null }
items:
  - { id: T-4, title: Done item, phase: 1, owner: null, status: done, depends_on: [] }
  - { id: T-5, title: In review, phase: 1, owner: null, status: doing, depends_on: [] }
`,
}

// over is the config text over the working tree's files, held in the tree
// reader's fake.
func over(t *testing.T, text string, tree map[string]string) *config.Loaded {
	t.Helper()
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": tree}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// with is files with the changes given, a "" text taking the file out.
func with(changes map[string]string) map[string]string {
	tree := map[string]string{}
	for p, text := range files {
		tree[p] = text
	}
	for p, text := range changes {
		if text == "" {
			delete(tree, p)
		} else {
			tree[p] = text
		}
	}
	return tree
}

// ports are task's ports over the fakes: the config, the tracker's fake
// holding the same files, and what each of task add's ports gives,
// recording what Write is handed and how often the lock is given back.
type ports struct {
	cfg        *config.Loaded
	cfgErr     error
	tracker    *porttest.Tracker
	soundErr   error
	soundCode  int
	id         string
	idErr      error
	sha        string
	writeCode  int
	writeErr   error
	written    []record.File
	header     string
	body       string
	registered []string
	released   int
}

func newPorts(t *testing.T, text string, tree map[string]string) *ports {
	tracker := &porttest.Tracker{}
	for p, text := range tree {
		tracker.Edit(p, text)
	}
	return &ports{cfg: over(t, text, tree), tracker: tracker, id: "T-7", sha: "0123456789abcdef"}
}

func (p *ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.cfgErr }

func (p *ports) Sound(ctx context.Context) (Sound, error) {
	s := Sound{Release: func() { p.released++ }, Code: p.soundCode}
	if p.soundErr != nil || p.soundCode != 0 {
		return s, p.soundErr
	}
	registry, text, found, err := record.Sound(ctx, p.tracker, p.cfg)
	if err != nil || len(found) > 0 {
		return s, errors.Join(err, errors.New("the registry is not sound"))
	}
	s.Cfg, s.Registry, s.Text = p.cfg, registry, text
	return s, nil
}

func (p *ports) ItemID(_ context.Context, _ *config.Loaded, registryIDs []string) (string, error) {
	p.registered = registryIDs
	return p.id, p.idErr
}

func (p *ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

func (p *ports) Write(_ context.Context, _ *config.Loaded, files []record.File, header, body string) (string, int, error) {
	p.written, p.header, p.body = files, header, body
	return p.sha, p.writeCode, p.writeErr
}

// line runs the line over p as internal/cli would, with --json and --quiet
// as given: its exit, error, stdout and stderr.
func line(t *testing.T, p *ports, json, quiet bool, args ...string) (int, error, string, string) {
	t.Helper()
	c, err := Parse(args, json, quiet)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, err, stdout.String(), stderr.String()
}

// One task runs verbose, its checks' lines on stdout, or under --json on
// stderr, the table after them.
func TestRunOneTaskVerbose(t *testing.T) {
	p := newPorts(t, configText, files)
	code, err, stdout, stderr := line(t, p, false, false, "T-1")
	if code != 0 || err != nil || stdout != "\nT-1 Passes\n  $ true\n\ndone     T-1  chore    Passes\n" || stderr != "" {
		t.Errorf("task T-1: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, stderr = line(t, p, true, false, "T-1")
	var got struct {
		Tasks []struct {
			ID, Type, Title, Status string
			Group                   any
			Checks                  []struct {
				Command, Result string
				Fails           bool
			}
		}
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || stderr != "\nT-1 Passes\n  $ true\n" ||
		len(got.Tasks) != 1 || got.Tasks[0].ID != "T-1" || got.Tasks[0].Status != "done" || got.Tasks[0].Group != 1.0 ||
		len(got.Tasks[0].Checks) != 1 || got.Tasks[0].Checks[0].Command != "true" || got.Tasks[0].Checks[0].Result != "pass" {
		t.Errorf("--json task T-1: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// Several tasks run quiet: a failing check exits 1, an after: push check
// is pending before the push, a task with no checks done or review by its
// item; --pending shows the tasks not done.
func TestRunSeveralTasks(t *testing.T) {
	p := newPorts(t, configText, files)
	code, err, stdout, stderr := line(t, p, false, false, "--pending")
	want := "\nfailing  T-2  docs     Fails\npending  T-3  chore    Waits\nreview   T-5  chore    In review\nreview   T-6  chore    Later\n"
	if code != refused || err != nil || stdout != want || stderr != "" {
		t.Errorf("task --pending: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, _ = line(t, p, false, false, "--group", "01", "--skip", "T-2,T-3")
	want = "\ndone     T-1  chore    Passes\ndone     T-4  chore    Done item\nreview   T-5  chore    In review\n"
	if code != 0 || err != nil || stdout != want {
		t.Errorf("task --group 01 --skip T-2,T-3: exit %d, %v, stdout %q", code, err, stdout)
	}
	code, err, stdout, _ = line(t, p, false, false, "T-4", "T-6")
	if want = "\ndone     T-4  chore    Done item\nreview   T-6  chore    Later\n"; code != 0 || err != nil || stdout != want {
		t.Errorf("task T-4 T-6: exit %d, %v, stdout %q", code, err, stdout)
	}
	code, err, stdout, _ = line(t, p, true, false, "T-2", "T-5")
	var got struct {
		Tasks []struct {
			ID, Status string
			Checks     []struct {
				Command, Result string
				Fails           bool
			}
		}
	}
	if code != refused || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || len(got.Tasks) != 2 ||
		got.Tasks[0].Status != "failing" || len(got.Tasks[0].Checks) != 2 || got.Tasks[0].Checks[0].Fails ||
		!got.Tasks[0].Checks[1].Fails || got.Tasks[0].Checks[1].Result != "fail" ||
		got.Tasks[1].Status != "review" || got.Tasks[1].Checks == nil || len(got.Tasks[1].Checks) != 0 {
		t.Errorf("--json task T-2 T-5: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// A non-numeric group is matched as written.
func TestRunAGroupByName(t *testing.T) {
	text := strings.Replace(configText, "group: { numeric: true }", "group: { pattern: \"[a-z]+\" }", 1)
	p := newPorts(t, text, map[string]string{"tasks/phase-one.yaml": "- { id: T-1, type: chore, title: One }\n",
		"tasks/phase-two.yaml": "- { id: T-2, type: chore, title: Two }\n"})
	code, err, stdout, _ := line(t, p, false, false, "--phase", "two")
	if code != 0 || err != nil || stdout != "\nreview   T-2  chore    Two\n" {
		t.Errorf("task --phase two: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// An ID not in the ledger is a line each, exit 1; no task matching is the
// usage line, naming the label's own flag beside --group, exit 2.
func TestRunNamesWhatItCannotRun(t *testing.T) {
	p := newPorts(t, configText, files)
	code, err, stdout, stderr := line(t, p, false, false, "T-1", "T-8", "--skip", "T-9", "T-10")
	if code != refused || err != nil || stdout != "" || stderr != "No task T-8 in tasks/\nNo task T-10 in tasks/\n" {
		t.Errorf("task T-1 T-8 T-10: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, stderr = line(t, p, false, false, "--skip", "T-1", "T-1")
	if code != unmatched || err != nil || stdout != "" ||
		stderr != "No matching tasks. Usage: itos task <id>… | --group|--phase <g> [--skip <ids>] | --pending\n" {
		t.Errorf("task --skip T-1 T-1: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	text := strings.Replace(configText, "group: { numeric: true }", "group: { label: group }", 1)
	p = newPorts(t, text, files)
	if code, _, _, stderr = line(t, p, false, false); code != unmatched ||
		stderr != "No matching tasks. Usage: itos task <id>… | --group <g> [--skip <ids>] | --pending\n" {
		t.Errorf("task under the label group: exit %d, stderr %q", code, stderr)
	}
}

// What stops the run is an error, exit 0: the config, the ledger, a task
// whose checks cannot run.
func TestRunErrors(t *testing.T) {
	p := newPorts(t, configText, files)
	p.cfgErr = errors.New("no config")
	if code, err, _, _ := line(t, p, false, false, "T-1"); code != 0 || err != p.cfgErr {
		t.Errorf("no config: exit %d, %v", code, err)
	}
	p = newPorts(t, "version: 1\n", files)
	if _, err, _, _ := line(t, p, false, false, "T-1"); err == nil || !strings.Contains(err.Error(), "ledger") {
		t.Errorf("no ledger: %v", err)
	}
	p = newPorts(t, configText, map[string]string{})
	if _, err, _, _ := line(t, p, false, false, "T-1"); err == nil || !strings.Contains(err.Error(), "tasks") {
		t.Errorf("no ledger folder: %v", err)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/phase-2.yaml": "- { id: T-6, type: chore, title: Bad, done_when: 3 }\n"}))
	if _, err, stdout, _ := line(t, p, false, false, "T-1", "T-6"); err == nil || !strings.Contains(err.Error(), "done_when is not a list") || stdout != "" {
		t.Errorf("a task whose checks cannot run: %v, stdout %q", err, stdout)
	}
}

// task list prints each task's item status or no item, by group with
// --group, and says when there is no registry.
func TestList(t *testing.T) {
	p := newPorts(t, configText, files)
	code, err, stdout, stderr := line(t, p, false, false, "list")
	want := "no item  T-1  1   chore    Passes\nno item  T-2  1   docs     Fails\nno item  T-3  1   chore    Waits\n" +
		"done     T-4  1   chore    Done item\ndoing    T-5  1   chore    In review\nno item  T-6  2   chore    Later\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("task list: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, _ = line(t, p, true, false, "list", "--group", "1")
	var got struct {
		Tasks []struct {
			ID     string
			Checks int
			Status any
		}
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || len(got.Tasks) != 5 ||
		got.Tasks[1].Checks != 2 || got.Tasks[1].Status != nil || got.Tasks[3].Status != "done" {
		t.Errorf("--json task list --group 1: exit %d, %v, stdout %q", code, err, stdout)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/work-items.yaml": ""}))
	code, err, stdout, stderr = line(t, p, false, false, "list", "--group", "2")
	if code != 0 || err != nil || stdout != "no item  T-6  2   chore    Later\n" ||
		stderr != "No work registry at tasks/work-items.yaml, so no task has an item.\n" {
		t.Errorf("task list with no registry: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p.cfgErr = errors.New("no config")
	if _, err, _, _ := line(t, p, false, false, "list"); err != p.cfgErr {
		t.Errorf("no config: %v", err)
	}
	p = newPorts(t, configText, map[string]string{})
	if _, err, stdout, _ := line(t, p, false, false, "list"); err == nil || stdout != "" {
		t.Errorf("no ledger folder: %v, stdout %q", err, stdout)
	}
}

// task next-id is one past the ledger's and the registry's ids.
func TestNextID(t *testing.T) {
	p := newPorts(t, configText, with(map[string]string{"tasks/work-items.yaml": "phases: { 1: null }\nitems:\n  - { id: T-9, title: Nine, phase: 1, owner: null, status: todo, depends_on: [] }\n"}))
	if code, err, stdout, _ := line(t, p, false, false, "next-id"); code != 0 || err != nil || stdout != "T-10\n" {
		t.Errorf("task next-id: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, err, stdout, _ := line(t, p, true, false, "next-id"); code != 0 || err != nil || !strings.Contains(stdout, `"id": "T-10"`) {
		t.Errorf("--json task next-id: exit %d, %v, stdout %q", code, err, stdout)
	}
	p.cfgErr = errors.New("no config")
	if _, err, _, _ := line(t, p, false, false, "next-id"); err != p.cfgErr {
		t.Errorf("no config: %v", err)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/work-items.yaml": "items: [\n"}))
	if _, err, stdout, _ := line(t, p, false, false, "next-id"); err == nil || stdout != "" {
		t.Errorf("a registry that is no YAML: %v, stdout %q", err, stdout)
	}
	p = newPorts(t, configText, map[string]string{})
	if _, err, stdout, _ := line(t, p, false, false, "next-id"); err == nil || stdout != "" {
		t.Errorf("no ledger folder: %v, stdout %q", err, stdout)
	}
}

// The line task add is handed: a chore in group 1 with one check.
var addLine = []string{"add", "--group", "1", "--type", "chore", "--title", "New", "--why", "Because.", "--check", "true"}

// task add writes the task and its item through Write, the registry's lock
// given back after; it says so, or under --json gives the task, its file,
// the item and the commit.
func TestAdd(t *testing.T) {
	p := newPorts(t, configText, files)
	code, err, stdout, stderr := line(t, p, false, false, addLine...)
	if code != 0 || err != nil || stderr != "" ||
		stdout != "T-7 is a new task in tasks/phase-1.yaml, and an item in tasks/work-items.yaml: committed 0123456 docs: add T-7\n" {
		t.Errorf("task add: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if p.released != 1 || !slices.Equal(p.registered, []string{"T-4", "T-5"}) || p.header != "docs: add T-7" ||
		p.body != `Add the task T-7 ("New") to tasks/phase-1.yaml, and its item to tasks/work-items.yaml, with itos task add.` ||
		len(p.written) != 2 || p.written[0].Path != "tasks/phase-1.yaml" || p.written[0].Old != files["tasks/phase-1.yaml"] ||
		!strings.Contains(p.written[0].Text, "id: T-7") || p.written[0].Created ||
		p.written[1].Path != "tasks/work-items.yaml" || p.written[1].Old != files["tasks/work-items.yaml"] ||
		!strings.Contains(p.written[1].Text, "id: T-7") || p.written[1].Created {
		t.Errorf("task add: released %d, registry ids %q, header %q, body %q, wrote %+v", p.released, p.registered, p.header, p.body, p.written)
	}
	p = newPorts(t, configText, files)
	p.sha = ""
	added := slices.Clone(addLine)
	added[2] = "2"
	code, err, stdout, _ = line(t, p, false, false, added...)
	if code != 0 || err != nil ||
		stdout != "T-7 is a new task in tasks/phase-2.yaml, and an item in tasks/work-items.yaml: the ledger and the registry are the stealth config's, so nothing is committed\n" {
		t.Errorf("task add in group 2 under a stealth config: exit %d, %v, stdout %q", code, err, stdout)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/phase-2.yaml": ""}))
	code, err, stdout, _ = line(t, p, false, false, added...)
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "T-7 is a new task in a new tasks/phase-2.yaml, ") || !p.written[0].Created {
		t.Errorf("task add in a new group: exit %d, %v, stdout %q, wrote %+v", code, err, stdout, p.written)
	}
	for _, sha := range []string{"0123456789abcdef", ""} {
		p = newPorts(t, configText, files)
		p.sha = sha
		code, err, stdout, _ = line(t, p, true, true, addLine...)
		var got struct {
			OK     bool
			Task   map[string]any
			File   string
			Item   map[string]any
			Commit any
		}
		var commit any
		if sha != "" {
			commit = sha
		}
		if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || !got.OK || got.Task["id"] != "T-7" ||
			got.File != "tasks/phase-1.yaml" || got.Item["id"] != "T-7" || got.Commit != commit {
			t.Errorf("--json task add committing %q: exit %d, %v, stdout %q", sha, code, err, stdout)
		}
	}
	p = newPorts(t, configText, files)
	if code, err, stdout, stderr = line(t, p, false, true, addLine...); code != 0 || err != nil || stdout != "" || stderr != "" || p.written == nil {
		t.Errorf("--quiet task add: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// What task add's ledger and registry refuse is reported, nothing written,
// exit 1: on stderr, or under --json on stdout.
func TestAddRefusals(t *testing.T) {
	text := configText + "commits: { types: [docs] }\n"
	p := newPorts(t, text, files)
	code, err, stdout, stderr := line(t, p, false, false, addLine...)
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "chore is not a commit type") ||
		!strings.Contains(stderr, "\n  fix: ") || p.written != nil || p.released != 1 {
		t.Errorf("a type commits.types does not list: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, err, stdout, stderr = line(t, p, true, false, addLine...)
	var got struct {
		OK       bool
		Problems []struct{ Rule, Message, Fix string }
	}
	if code != refused || err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &got) != nil || got.OK ||
		len(got.Problems) != 1 || got.Problems[0].Fix == "" {
		t.Errorf("--json, a type commits.types does not list: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p = newPorts(t, configText, files)
	added := slices.Clone(addLine)
	added[2] = "4"
	code, err, stdout, stderr = line(t, p, false, false, added...)
	if code != refused || err != nil || stdout != "" || !strings.Contains(stderr, "is not listed") || p.written != nil {
		t.Errorf("a group the registry does not list: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// What stops task add is an error, the registry's lock given back once it
// is held: the config, the line, the registry not sound (reported by Sound,
// its exit kept), the id, the ledger or the registry not edited in place,
// and the write (its refusal's exit kept).
func TestAddErrors(t *testing.T) {
	p := newPorts(t, configText, files)
	p.cfgErr = errors.New("no config")
	if code, err, _, _ := line(t, p, false, false, addLine...); code != 0 || err != p.cfgErr || p.released != 0 {
		t.Errorf("no config: exit %d, %v, released %d", code, err, p.released)
	}
	p = newPorts(t, configText, files)
	if _, err, _, _ := line(t, p, false, false, append(slices.Clone(addLine), "T-7")...); err == nil || err.Error() != "task add mints the task id; do not pass one" || p.released != 0 {
		t.Errorf("an id given: %v, released %d", err, p.released)
	}
	p.soundCode = refused
	if code, err, stdout, _ := line(t, p, false, false, addLine...); code != refused || err != nil || stdout != "" || p.released != 1 || p.registered != nil {
		t.Errorf("a registry not sound: exit %d, %v, stdout %q, released %d", code, err, stdout, p.released)
	}
	p = newPorts(t, configText, files)
	p.soundErr = errors.New("no lock")
	if code, err, _, _ := line(t, p, false, false, addLine...); code != 0 || err != p.soundErr || p.released != 1 {
		t.Errorf("Sound failing: exit %d, %v, released %d", code, err, p.released)
	}
	p = newPorts(t, configText, files)
	p.idErr = errors.New("no counter")
	if _, err, _, _ := line(t, p, false, false, addLine...); err != p.idErr || p.released != 1 || p.written != nil {
		t.Errorf("no id: %v, released %d", err, p.released)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/phase-1.yaml": "[{ id: T-1, type: chore, title: Flow }]\n"}))
	if _, err, _, _ := line(t, p, false, false, addLine...); err == nil || !strings.Contains(err.Error(), "cannot be edited in place") || p.written != nil {
		t.Errorf("a ledger file not edited in place: %v", err)
	}
	p = newPorts(t, configText, with(map[string]string{"tasks/work-items.yaml": "{ phases: { 1: null }, items: [] }\n"}))
	_, err, _, _ := line(t, p, false, false, addLine...)
	if err == nil || !strings.HasPrefix(err.Error(), "tasks/work-items.yaml cannot be edited in place: ") || kind.Of(err) != kind.Usage || p.written != nil {
		t.Errorf("a registry not edited in place: %v", err)
	}
	p = newPorts(t, configText, files)
	p.writeCode = refused
	if code, err, stdout, _ := line(t, p, false, false, addLine...); code != refused || err != nil || stdout != "" || p.released != 1 {
		t.Errorf("a write refused: exit %d, %v, stdout %q", code, err, stdout)
	}
	p = newPorts(t, configText, files)
	p.writeErr = errors.New("no git")
	if code, err, stdout, _ := line(t, p, false, false, addLine...); code != 0 || err != p.writeErr || stdout != "" || p.released != 1 {
		t.Errorf("a write failing: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// The IDs itos task names as unknown are the words ledger.id matches, every
// word that is no flag with no ledger.id (slice 130), never a flag's value.
func TestUnknownIDs(t *testing.T) {
	tasks := []ledger.Task{{ID: "T-001"}}
	args := []string{"T-001", "OPS-8", "--pending", "T-099", "--skip", "T-9", "--group", "7", "--phase=3"}
	cfg := &config.Loaded{}
	cfg.Ledger.Group.Label = "phase"
	if got, want := unknownIDs(cfg, args, tasks), []string{"OPS-8", "T-099"}; !slices.Equal(got, want) {
		t.Errorf("with no ledger.id: got %q, want %q", got, want)
	}
	id := `T-\d+`
	cfg.Ledger.ID = &id
	if got, want := unknownIDs(cfg, args, tasks), []string{"T-099"}; !slices.Equal(got, want) {
		t.Errorf("with ledger.id %s: got %q, want %q", id, got, want)
	}
}
