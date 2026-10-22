package guide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/engine/guides"
	guidetext "github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// ports are the guide command's ports over the fakes: a repository and the
// working tree's files.
type ports struct {
	*vcsfake.VCS
	trees treefake.Trees
}

func (p ports) Trees(context.Context) treeport.Trees { return p.trees }

func (p ports) Worktree(ctx context.Context) treeport.Source {
	src, _ := p.trees.At(ctx, "worktree")
	return src
}

// over is the ports of a repository whose top is a new folder, the current
// one, with no ITOS_CONFIG, holding files in its working tree, which are
// written there too: config.Path and the notes read the folder.
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

// shipped is the guide or procedure by its name, as written.
func shipped(name string) string {
	text, _ := guidetext.Text(name)
	return text
}

// Every refusal is a usage error naming the guides, then the procedures.
func TestParseRefusals(t *testing.T) {
	known := "the guides are coordinate, work; the procedures are brief, commit, decide, land, push, repair"
	for args, want := range map[string]string{
		"":                    "guide needs a name: " + known,
		"review":              "no guide or procedure review: " + known,
		"brief slice-1":       "guide brief takes no argument: slice-1",
		"work slice-1 more":   "guide takes one name: work slice-1 more",
		"coordinate anything": "guide takes one name: coordinate anything",
	} {
		_, err := Parse(strings.Fields(args), false)
		var usage out.UsageError
		if err == nil || err.Error() != want || !errors.As(err, &usage) {
			t.Errorf("Parse(%q) = %v, want the usage error %q", args, err, want)
		}
	}
	if c, err := Parse([]string{"land"}, true); err != nil || c != (CLI{Query: Query{Name: "land"}, JSON: true}) {
		t.Errorf("Parse(land) = %+v, %v", c, err)
	}
}

// A guide and a procedure print as written, needing no config.
func TestRunPrintsAsWritten(t *testing.T) {
	p := over(t, nil)
	for _, name := range []string{"work", "land", "push", "brief"} {
		code, stdout, stderr, err := call(t, p, name)
		if code != 0 || err != nil || stdout != shipped(name) || stderr != "" {
			t.Errorf("guide %s: exit %d, %v, stdout %.60q, stderr %q", name, code, err, stdout, stderr)
		}
	}
}

// --json is the guide's name and text, and the notes' file after the
// coordinator's guide.
func TestRunJSON(t *testing.T) {
	p := over(t, map[string]string{"docs/ORCHESTRATING.md": "Ours.\n"})
	for name, notes := range map[string]any{"push": nil, "coordinate": "docs/ORCHESTRATING.md"} {
		code, stdout, _, err := call(t, p, "--json", name)
		var got map[string]any
		if code != 0 || err != nil || json.Unmarshal([]byte(stdout), &got) != nil {
			t.Fatalf("guide %s --json: exit %d, %v, stdout %s", name, code, err, stdout)
		}
		if got["guide"] != name || got["notes"] != notes || !strings.HasPrefix(got["text"].(string), strings.TrimRight(shipped(name), "\n")) {
			t.Errorf("guide %s --json = %v", name, got)
		}
	}
}

// The coordinator's guide is followed by the repository's notes, as the
// config names them, never this clone's own.
func TestRunCoordinateWithTheConfigsNotes(t *testing.T) {
	p := over(t, map[string]string{
		"itos.yaml":          "version: 1\nguide:\n  orchestrating: NOTES.md\n",
		"NOTES.md":           "Config notes.\n",
		".git/itos/notes.md": "Mine alone.\n",
	})
	code, stdout, stderr, err := call(t, p, "coordinate")
	want := strings.TrimRight(shipped("coordinate"), "\n") + "\n" + guides.Separator + "Config notes.\n"
	if code != 0 || err != nil || stdout != want || stderr != "" {
		t.Errorf("guide coordinate: exit %d, %v, stdout ends %q, stderr %q", code, err, stdout[max(0, len(stdout)-80):], stderr)
	}
}

// The commit procedure is followed by the config's commit policy; with no
// config it prints alone, and with one that cannot be read, alone after a
// warning.
func TestRunCommitPrintsThePolicy(t *testing.T) {
	p := over(t, map[string]string{"itos.yaml": "version: 1\ncommits:\n  types: [feat, deps]\n"})
	code, stdout, stderr, err := call(t, p, "commit")
	head := strings.TrimRight(shipped("commit"), "\n") + "\n\n## The rules the config sets\n\nAs `itos.yaml` sets them;"
	if code != 0 || err != nil || !strings.HasPrefix(stdout, head) || !strings.Contains(stdout, "A commit's type is one of `feat` or `deps`.") || stderr != "" {
		t.Errorf("a config: exit %d, %v, stdout %s, stderr %q", code, err, stdout, stderr)
	}
	p = over(t, map[string]string{"itos.yaml": "version: [\n"})
	code, stdout, stderr, err = call(t, p, "commit")
	if code != 0 || err != nil || stdout != shipped("commit") ||
		!strings.HasPrefix(stderr, "itos: itos.yaml cannot be read, so the commit rules it sets are not printed: ") {
		t.Errorf("a broken config: exit %d, %v, stdout %s, stderr %q", code, err, stdout, stderr)
	}
	p = over(t, nil)
	if code, stdout, stderr, err = call(t, p, "commit"); code != 0 || err != nil || stdout != shipped("commit") || stderr != "" {
		t.Errorf("no config: exit %d, %v, stdout %s, stderr %q", code, err, stdout, stderr)
	}
}

// failing is a writer that refuses every write.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("refused") }

// A guide that cannot be written is the command line's error.
func TestRunReportsAWriteThatFails(t *testing.T) {
	p := over(t, nil)
	c, _ := Parse([]string{"work"}, false)
	if code, err := c.Run(t.Context(), failing{}, &bytes.Buffer{}, NewHandler(p)); code != 0 || err == nil {
		t.Errorf("Run = %d, %v, want the write's error", code, err)
	}
}
