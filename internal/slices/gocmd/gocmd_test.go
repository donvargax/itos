package gocmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/guides"
	"github.com/donvargax/itos/v8/internal/engine/status"
	"github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the go command's ports over the fakes: a repository, the
// working tree's files, and where things stand as st, found and err give
// it, the config it was asked for kept in asked.
type ports struct {
	*vcsfake.VCS
	trees treefake.Trees
	st    *status.Standing
	found []out.Problem
	err   error
	asked **config.Loaded
}

func (p ports) Trees(context.Context) treeport.Trees { return p.trees }

func (p ports) Worktree(ctx context.Context) treeport.Source {
	src, _ := p.trees.At(ctx, "worktree")
	return src
}

func (p ports) Standing(_ context.Context, cfg *config.Loaded) (*status.Standing, []out.Problem, error) {
	*p.asked = cfg
	return p.st, p.found, p.err
}

// over is the ports of a repository whose top is a new folder, the current
// one, with no ITOS_CONFIG, holding files in its working tree, which are
// written there too: config.Path and the notes read the folder. Where
// things stand is a person's.
func over(t *testing.T, files map[string]string) ports {
	t.Helper()
	t.Setenv("ITOS_CONFIG", "")
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(top)
	for name, text := range files {
		path := filepath.FromSlash(name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ports{
		VCS:   &vcsfake.VCS{Located: map[string]vcsport.Location{"": {Top: top, GitDir: ".git", Common: ".git"}}},
		trees: treefake.Trees{Files: map[string]map[string]string{"worktree": files}},
		st:    &status.Standing{Person: "ana"},
		asked: new(*config.Loaded),
	}
}

// call is the command's line run over p: its exit, stdout, stderr and
// error; a --json first sets the global flag.
func call(t *testing.T, p ports, args ...string) (int, string, string, error) {
	t.Helper()
	json := len(args) > 0 && args[0] == "--json"
	if json {
		args = args[1:]
	}
	c, err := Parse(args, json)
	if err != nil {
		return 0, "", "", err
	}
	var stdout, stderr bytes.Buffer
	code, err := c.Run(t.Context(), &stdout, &stderr, NewHandler(p))
	return code, stdout.String(), stderr.String(), err
}

// coordinate is the coordinator's guide, as written.
func coordinate() string {
	text, _ := guide.Text(guide.Coordinate)
	return text
}

// printed is where things stand as st prints it.
func printed(st *status.Standing) string {
	var b strings.Builder
	st.Print(&b)
	return b.String()
}

// go takes no arguments: any is a usage error.
func TestParse(t *testing.T) {
	_, err := Parse([]string{"x", "y"}, false)
	var usage out.UsageError
	if err == nil || err.Error() != "go takes no arguments: x y" || !errors.As(err, &usage) {
		t.Errorf("Parse(x y) = %v, want the usage error", err)
	}
	if c, err := Parse(nil, true); err != nil || c != (CLI{JSON: true}) {
		t.Errorf("Parse() = %+v, %v", c, err)
	}
}

// The guide, the repository's notes and this clone's own, then, after a
// line of ---, where things stand for the config read.
func TestRunPrintsTheGuideThenWhereThingsStand(t *testing.T) {
	p := over(t, map[string]string{
		"itos.yaml":             "version: 1\n",
		"docs/ORCHESTRATING.md": "Ours.\n",
		".git/itos/notes.md":    "Mine.\n",
	})
	code, stdout, stderr, err := call(t, p)
	want := strings.TrimRight(coordinate(), "\n") + "\n" + guides.Separator + "Ours.\n" + guides.Separator +
		guides.LocalNotesHeading + "\n\nThese are uncommitted and hold what is true on this machine only (.git/itos/notes.md).\n\nMine.\n" +
		guides.Separator + printed(p.st)
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("go: exit %d, %v, stdout ends %q, stderr %q", code, err, stdout[max(0, len(stdout)-300):], stderr)
	}
	if *p.asked == nil || (*p.asked).Version != 1 {
		t.Errorf("Standing was asked for %+v, want the config read", *p.asked)
	}
}

// --json is the guide's name and text, the notes' files and the status.
func TestRunJSON(t *testing.T) {
	p := over(t, map[string]string{
		"itos.yaml":             "version: 1\n",
		"docs/ORCHESTRATING.md": "Ours.\n",
		".git/itos/notes.md":    "Mine.\n",
	})
	code, stdout, stderr, err := call(t, p, "--json")
	var got struct {
		Guide, Text, Notes string
		Local              string `json:"local_notes"`
		Status             map[string]any
	}
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil || stderr != "" {
		t.Fatalf("go --json: exit %d, %v, stdout %s, stderr %q", code, err, stdout, stderr)
	}
	if got.Guide != "coordinate" || !strings.HasPrefix(got.Text, strings.TrimRight(coordinate(), "\n")) ||
		got.Notes != "docs/ORCHESTRATING.md" || got.Local != ".git/itos/notes.md" || got.Status["person"] != "ana" {
		t.Errorf("go --json = %+v", got)
	}
	p = over(t, nil)
	code, stdout, _, err = call(t, p, "--json")
	var bare map[string]any
	if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &bare) != nil || len(bare) != 3 || bare["guide"] != "coordinate" {
		t.Errorf("go --json with nothing but the guide: exit %d, %v, stdout %.200s", code, err, stdout)
	}
}

// Where things stand is left out, with one line on stderr said before the
// notes' warnings, when there is no config, it cannot be read, the
// registry has problems or status fails; the guide still prints.
func TestRunLeavesWhereThingsStandOut(t *testing.T) {
	notes := map[string]string{".git/itos/notes.md": "Mine.\n"}
	for _, c := range []struct {
		name  string
		files map[string]string
		found []out.Problem
		err   error
		line  string
	}{
		{"no config", nil, nil, nil, "there is no itos config here (itos init makes one)"},
		{"a broken config", map[string]string{"itos.yaml": "version: [\n"}, nil, nil, "itos.yaml: cannot be read: "},
		{"problems", map[string]string{"itos.yaml": "version: 1\n"}, []out.Problem{{Message: "p1-a: no title"}, {Message: "p1-b"}}, nil, "p1-a: no title (itos work check)"},
		{"a failure", map[string]string{"itos.yaml": "version: 1\n"}, nil, errors.New("no registry"), "no registry"},
	} {
		files := map[string]string{"docs/ORCHESTRATING.md/x": "a folder in place of the notes\n"}
		for name, text := range notes {
			files[name] = text
		}
		for name, text := range c.files {
			files[name] = text
		}
		p := over(t, files)
		p.found, p.err = c.found, c.err
		code, stdout, stderr, err := call(t, p)
		lead := "itos: where things stand is not printed: " + c.line
		// The notes' path is the config's with one, else joined in the
		// system's form, and the error's words are the system's.
		warning := strings.Index(stderr, "ORCHESTRATING.md cannot be read, so the guide prints without it: ")
		if code != 0 || err != nil || !strings.HasPrefix(stderr, lead) || warning < len(lead) ||
			!strings.HasSuffix(stdout, "\n\nMine.\n") || strings.Contains(stdout, "# Where things stand") {
			t.Errorf("%s: exit %d, %v, stdout ends %q, stderr %q", c.name, code, err, stdout[max(0, len(stdout)-80):], stderr)
		}
	}
}

// failing is a writer that refuses every write.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("refused") }

// A guide that cannot be written is the command line's error, and where
// things stand is not printed after it.
func TestRunReportsAWriteThatFails(t *testing.T) {
	p := over(t, map[string]string{"itos.yaml": "version: 1\n"})
	c, _ := Parse(nil, false)
	if code, err := c.Run(t.Context(), failing{}, &bytes.Buffer{}, NewHandler(p)); code != 0 || err == nil {
		t.Errorf("Run = %d, %v, want the write's error", code, err)
	}
}
