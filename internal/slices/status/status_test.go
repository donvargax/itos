package status

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	trackerfake "github.com/donvargax/itos/v8/internal/store/tracker/port/porttest"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the status command's ports over the fakes: the repository, a
// config or the error loading it, the identity provider, an environment
// where the release server is never asked, and the tracker's fake.
type ports struct {
	*vcsfake.VCS
	cfg      *config.Loaded
	err      error
	identity providers.Identity
	tracker  trackerport.Tracker
}

func (p ports) Config(context.Context) (*config.Loaded, error) { return p.cfg, p.err }

func (p ports) Identity(context.Context, *config.Loaded) providers.Identity { return p.identity }

func (ports) Setup(context.Context) providers.WatchSetup {
	return providers.WatchSetup{Env: func(name string) string {
		if name == "ITOS_NO_UPDATE" {
			return "1"
		}
		return ""
	}}
}

func (ports) Gh(context.Context, ...string) (string, error) { return "", providers.ErrNoGh }

func (p ports) Tracker(context.Context, *config.Loaded) trackerport.Tracker { return p.tracker }

// configText names the registry and the people; ci.watch's provider is
// none, and the questions are the default tasks/asks.yaml.
const configText = "version: 1\nwork:\n  registry: work-items.yaml\n  people: { source: yaml, file: people.yaml }\n"

// registry is a registry of the item one, todo and unowned.
const registry = "phases: { 1: null }\nitems:\n  - { id: one, title: One, phase: 1, owner: null, status: todo, depends_on: [] }\n"

// over is the ports of a repository whose top is an empty folder the test
// stands in, its git common dir common, the working tree holding files and
// the tracker the questions (none when ""); the identity provider names
// nobody.
func over(t *testing.T, files map[string]string, questions string) (p ports, common string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	common = filepath.Join(dir, ".git")
	v := &vcsfake.VCS{Root: dir, Located: map[string]vcsport.Location{"": {Top: dir, GitDir: common, Common: common}}}
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": files}}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(v, trees, src, "itos.yaml", configText, false)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &trackerfake.Tracker{}
	if questions != "" {
		change := trackerport.Change{Name: cfg.Work.Asks, Text: questions, Created: true}
		if _, err := tracker.Transact(t.Context(), trackerport.Transaction{Changes: []trackerport.Change{change},
			Paragraphs: []string{"chore: start"}, Quiet: true}); err != nil {
			t.Fatal(err)
		}
	}
	nobody := func() providers.Answer { return providers.Answer{Problem: "nobody is asked"} }
	return ports{VCS: v, cfg: cfg, identity: nobody, tracker: tracker}, common
}

// run is the status command's line over p: its exit code, stdout, stderr
// and error.
func run(t *testing.T, p ports, args ...string) (int, string, string, error) {
	t.Helper()
	json := len(args) > 0 && args[0] == "--json"
	if json {
		args = args[1:]
	}
	c, err := Parse(args, json)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

func TestParse(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"extra"}, "status takes no arguments: extra"},
		{[]string{"extra", "more"}, "status takes no arguments: extra more"},
		{[]string{"--as", "a", "more"}, "status takes no arguments: more"},
		{[]string{"--as="}, "status takes one --as with a value"},
	} {
		_, err := Parse(test.args, false)
		var usage out.UsageError
		if !errors.As(err, &usage) || err.Error() != test.want {
			t.Errorf("Parse(%q) = %v, want the usage error %q", test.args, err, test.want)
		}
	}
	if c, err := Parse([]string{"--as", "a"}, true); err != nil || c != (CLI{Query: Query{As: "a"}, JSON: true}) {
		t.Errorf("Parse --as a = %+v, %v", c, err)
	}
	if c, err := Parse(nil, false); err != nil || c != (CLI{}) {
		t.Errorf("Parse() = %+v, %v", c, err)
	}
}

// Where things stand prints for the person --as names: the item, the open
// questions read through the config's tracker and the drafts waiting in the
// git common dir; under --json as one object.
func TestStatusPrintsWhereThingsStand(t *testing.T) {
	p, common := over(t, map[string]string{"work-items.yaml": registry, "people.yaml": "- a\n- b\n"},
		"questions:\n  - id: q-1\n    question: Open?\n  - id: q-2\n    question: Done?\n    answer: Yes.\n")
	file := filepath.Join(common, config.StealthFolder, draft.Folder, draft.FileName)
	if err := draft.Save(file, draft.File{Drafts: []draft.Draft{{ID: "say-new", Message: "docs: say new", Paths: []string{"notes.md"}}}}); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr, err := run(t, p, "--as", "a")
	if code != 0 || err != nil || !strings.HasPrefix(stdout, "# Where things stand") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	for _, want := range []string{"one", "q-1", "Open?", "say-new"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the status does not say %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "q-2") {
		t.Errorf("the status lists an answered question:\n%s", stdout)
	}
	code, stdout, stderr, err = run(t, p, "--json", "--as", "a")
	var st struct {
		Person    string           `json:"person"`
		Next      []map[string]any `json:"next"`
		Questions []map[string]any `json:"questions"`
		Drafts    []map[string]any `json:"drafts"`
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &st) != nil {
		t.Fatalf("--json: exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	if st.Person != "a" || len(st.Next) != 1 || len(st.Questions) != 1 || st.Questions[0]["id"] != "q-1" ||
		len(st.Drafts) != 1 || st.Drafts[0]["id"] != "say-new" {
		t.Errorf("--json = %+v", st)
	}
}

// With no --as among several people the identity provider is asked, its
// problem said on stderr; questions and drafts that cannot be read are
// each a line, the rest still printed.
func TestStatusSaysWhatItCannotRead(t *testing.T) {
	p, common := over(t, map[string]string{"work-items.yaml": registry, "people.yaml": "- a\n- b\n"}, "questions: [\n")
	if err := os.MkdirAll(filepath.Join(common, config.StealthFolder, draft.Folder, draft.FileName, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr, err := run(t, p)
	if code != 0 || err != nil || !strings.Contains(stderr, "nobody is asked") {
		t.Fatalf("exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	for _, want := range []string{"The questions cannot be read: ", "The drafts cannot be read: ", "one"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the status does not say %q:\n%s", want, stdout)
		}
	}
}

// Outside a repository the drafts are not read, not even to say they
// cannot be, and the rest is still read.
func TestStatusOutsideARepositoryListsNoDrafts(t *testing.T) {
	p, _ := over(t, map[string]string{"work-items.yaml": registry, "people.yaml": "- a\n"}, "")
	h := NewHandler(p)
	p.Located = nil
	var stderr strings.Builder
	st, found, missing, err := h.Standing(t.Context(), p.cfg, "a", Out{Stderr: &stderr})
	if err != nil || found != nil || missing || st.Drafts == nil || len(st.Drafts) != 0 || len(st.Next) != 1 ||
		strings.Contains(strings.Join(st.Unread, "\n"), "drafts") {
		t.Errorf("outside a repository: %v %v %v, %+v", err, found, missing, st)
	}
}

// A registry that is not there is its problems, exit 1, on stderr with
// their fixes or under --json on stdout; --as not among the people is exit
// 3, said on stderr; a config that cannot be read is the error.
func TestStatusRefuses(t *testing.T) {
	p, _ := over(t, map[string]string{"people.yaml": "- a\n"}, "")
	code, stdout, stderr, err := run(t, p, "--as", "a")
	if code != refused || err != nil || stdout != "" ||
		stderr != "no work registry at work-items.yaml\n  fix: move the registry to work-items.yaml, or set work.registry to where it is\n" {
		t.Errorf("no registry: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	code, stdout, stderr, err = run(t, p, "--json", "--as", "a")
	var refusal struct {
		OK       *bool         `json:"ok"`
		Problems []out.Problem `json:"problems"`
	}
	if code != refused || err != nil || stderr != "" || json.Unmarshal([]byte(stdout), &refusal) != nil || refusal.OK == nil ||
		*refusal.OK || len(refusal.Problems) != 1 || refusal.Problems[0].Rule != "work-registry-missing" {
		t.Errorf("no registry --json: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p, _ = over(t, map[string]string{"work-items.yaml": registry, "people.yaml": "- a\n"}, "")
	code, stdout, stderr, err = run(t, p, "--as", "zed")
	if code != notAmong || err != nil || stdout != "" || !strings.Contains(stderr, `"zed" is not in people.yaml`) {
		t.Errorf("--as not among the people: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	p.err = errors.New("itos.yaml: cannot be read")
	if code, stdout, stderr, err := run(t, p, "--as", "a"); code != 0 || err != p.err || stdout != "" || stderr != "" {
		t.Errorf("a config that cannot be read: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
}

// A problem with no fix is its message alone.
func TestRefuseSaysAFixOnlyWhenThereIsOne(t *testing.T) {
	var stdout, stderr strings.Builder
	code, err := CLI{}.refuse(&stdout, &stderr, []out.Problem{{Message: "m"}, {Message: "n", Fix: "f"}})
	if code != refused || err != nil || stdout.String() != "" || stderr.String() != "m\nn\n  fix: f\n" {
		t.Errorf("refuse = %d, %v, stdout %q, stderr %q", code, err, stdout.String(), stderr.String())
	}
}
