package guides

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	"github.com/donvargax/itos/v8/internal/guide"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// repo is a repository the VCS fake locates from the folder itos runs in
// (its Root): its top is a new folder, its links resolved, and that folder;
// its git common dir is .git there, holding itos's folder. The test stands
// elsewhere.
func repo(t *testing.T) (string, *vcsfake.VCS) {
	t.Helper()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(top, ".git", "itos"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	return top, &vcsfake.VCS{Root: top, Located: map[string]vcsport.Location{"": {Top: top, GitDir: ".git", Common: ".git"}}}
}

// write writes text to the file at path, its folders made.
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// loaded is the config of text, read as itos.yaml.
func loaded(t *testing.T, text string) Config {
	t.Helper()
	trees := treefake.Trees{}
	src, err := trees.At(t.Context(), "worktree")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromText(&vcsfake.VCS{}, trees, src, "itos.yaml", text, false)
	if err != nil {
		t.Fatal(err)
	}
	return Config{File: "itos.yaml", Loaded: cfg}
}

// text is the guide by its name, as shipped.
func text(name string) string {
	s, _ := guide.Text(name)
	return s
}

// A guide but the coordinator's prints as shipped, and reads no notes, nor
// the config.
func TestComposeAGuideButTheCoordinators(t *testing.T) {
	top, v := repo(t)
	write(t, filepath.Join(top, "docs", "ORCHESTRATING.md"), "Ours.\n")
	var stderr strings.Builder
	g := Compose(t.Context(), v, Config{File: "itos.yaml", Err: errors.New("unread")}, "work", false, &stderr)
	if g != (Guide{Text: text("work")}) || stderr.String() != "" {
		t.Errorf("Compose(work) = %+v, stderr %q", g, stderr.String())
	}
}

// The coordinator's guide is followed by the repository's notes, read from
// the top with no config, and then by this clone's own under their heading,
// each after a line of ---, a newline ending each.
func TestComposeTheCoordinatorsWithBothNotes(t *testing.T) {
	top, v := repo(t)
	write(t, filepath.Join(top, "docs", "ORCHESTRATING.md"), "Ours.")
	write(t, filepath.Join(top, ".git", "itos", "notes.md"), "Mine.")
	if err := os.MkdirAll(filepath.Join(top, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	v.Root = filepath.Join(top, "a", "b")
	v.Located[""] = vcsport.Location{Top: top, GitDir: "../../.git", Common: "../../.git"}
	var stderr strings.Builder
	g := Compose(t.Context(), v, Config{}, guide.Coordinate, true, &stderr)
	local := filepath.Join("..", "..", ".git", "itos", "notes.md")
	want := Guide{
		Text: strings.TrimRight(text(guide.Coordinate), "\n") + "\n" + Separator + "Ours.\n" + Separator +
			LocalNotesHeading + "\n\nThese are uncommitted and hold what is true on this machine only (" +
			filepath.ToSlash(local) + ").\n\nMine.\n",
		Notes: filepath.Join("..", "..", "docs", "ORCHESTRATING.md"),
		Local: local,
	}
	if g != want || stderr.String() != "" {
		t.Errorf("Compose(coordinate) = %+v\nwant %+v\nstderr %q", g, want, stderr.String())
	}
	if g := Compose(t.Context(), v, Config{}, guide.Coordinate, false, &stderr); g.Local != "" || g.Notes != want.Notes {
		t.Errorf("Compose(coordinate) without local = %+v", g)
	}
}

// Notes the config's guide.orchestrating names that are this clone's own
// print once, as the repository's.
func TestComposePrintsTheSameNotesOnce(t *testing.T) {
	top, v := repo(t)
	write(t, filepath.Join(top, ".git", "itos", "notes.md"), "Once.\n")
	g := Compose(t.Context(), v, loaded(t, "version: 1\nguide:\n  orchestrating: .git/itos/notes.md\n"), guide.Coordinate, true, &strings.Builder{})
	want := Guide{
		Text:  strings.TrimRight(text(guide.Coordinate), "\n") + "\n" + Separator + "Once.\n",
		Notes: ".git/itos/notes.md",
	}
	if g != want {
		t.Errorf("Compose = %+v, want %+v", g, want)
	}
}

// The commit policy is the config's sections under a heading naming the
// config; none with no config, and none, after a warning on one line, with
// a config that cannot be read.
func TestCommitPolicy(t *testing.T) {
	var stderr strings.Builder
	if got := CommitPolicy(Config{File: "itos.yaml"}, &stderr); got != "" || stderr.String() != "" {
		t.Errorf("no config: %q, stderr %q", got, stderr.String())
	}
	if got := CommitPolicy(Config{File: "itos.yaml", Err: errors.New(" bad\nworse\n")}, &stderr); got != "" ||
		stderr.String() != "itos: itos.yaml cannot be read, so the commit rules it sets are not printed: bad; worse\n" {
		t.Errorf("unread: %q, stderr %q", got, stderr.String())
	}
	stderr.Reset()
	c := loaded(t, "version: 1\ncommits:\n  types: [feat, deps]\n")
	want := "## The rules the config sets\n\nAs `itos.yaml` sets them; the hooks and CI hold every commit to them.\n\n" +
		strings.Join(agentrules.CommitPolicy(c.Loaded, agentrules.Code), "\n\n") + "\n"
	if got := CommitPolicy(c, &stderr); got != want || !strings.Contains(got, "A commit's type is one of `feat` or `deps`.") || stderr.String() != "" {
		t.Errorf("a config: %q, stderr %q", got, stderr.String())
	}
}

// A config that cannot be read is a warning, and the notes are read from
// their default.
func TestNotesOfAConfigThatCannotBeRead(t *testing.T) {
	top, v := repo(t)
	write(t, filepath.Join(top, "docs", "ORCHESTRATING.md"), "Default.\n")
	var stderr strings.Builder
	text, path := Notes(t.Context(), v, Config{File: "itos.yaml", Err: errors.New("bad")}, &stderr)
	if text != "Default.\n" || path != filepath.Join("docs", "ORCHESTRATING.md") ||
		stderr.String() != "itos: itos.yaml cannot be read, so guide.orchestrating is its default: bad\n" {
		t.Errorf("Notes = %q, %q, stderr %q", text, path, stderr.String())
	}
}

// Outside a repository with no config there are no notes to read; in the
// git folder, where there is no work tree, the default is read from there.
func TestNotesPathWithNoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if got := notesPath(t.Context(), &vcsfake.VCS{}, Config{}, &strings.Builder{}); got != "" {
		t.Errorf("outside a repository = %q", got)
	}
	v := &vcsfake.VCS{Located: map[string]vcsport.Location{"": {GitDir: ".", Common: "."}}}
	if got := notesPath(t.Context(), v, Config{}, &strings.Builder{}); got != filepath.Join("docs", "ORCHESTRATING.md") {
		t.Errorf("in the git folder = %q", got)
	}
}

// Notes are none when no file is named, the file is missing or holds only
// blanks; a file that cannot be read is a warning; a newline ends them.
func TestReadNotes(t *testing.T) {
	dir := t.TempDir()
	var stderr strings.Builder
	write(t, filepath.Join(dir, "blank.md"), " \n\n")
	write(t, filepath.Join(dir, "open.md"), "No newline.")
	if err := os.MkdirAll(filepath.Join(dir, "folder.md", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"": "", filepath.Join(dir, "missing.md"): "", filepath.Join(dir, "blank.md"): ""} {
		if text, got := readNotes("", path, &stderr); text != want || got != "" {
			t.Errorf("readNotes(%q) = %q, %q", path, text, got)
		}
	}
	if stderr.String() != "" {
		t.Errorf("stderr %q", stderr.String())
	}
	open := filepath.Join(dir, "open.md")
	if text, got := readNotes("", open, &stderr); text != "No newline.\n" || got != open {
		t.Errorf("readNotes(open) = %q, %q", text, got)
	}
	folder := filepath.Join(dir, "folder.md")
	if text, got := readNotes("", folder, &stderr); text != "" || got != "" ||
		!strings.HasPrefix(stderr.String(), "itos: "+folder+" cannot be read, so the guide prints without it: ") {
		t.Errorf("readNotes(folder) = %q, %q, stderr %q", text, got, stderr.String())
	}
}

// This clone's own notes are none outside a repository and with no common
// dir; a file named before them that is another, or missing, leaves them
// printed.
func TestLocalNotes(t *testing.T) {
	top, v := repo(t)
	write(t, filepath.Join(top, ".git", "itos", "notes.md"), "Mine.\n")
	write(t, filepath.Join(top, "other.md"), "Other.\n")
	ctx := t.Context()
	if text, path := LocalNotes(ctx, &vcsfake.VCS{}, "", &strings.Builder{}); text != "" || path != "" {
		t.Errorf("outside a repository = %q, %q", text, path)
	}
	bare := &vcsfake.VCS{Located: map[string]vcsport.Location{"": {Top: top}}}
	if text, path := LocalNotes(ctx, bare, "", &strings.Builder{}); text != "" || path != "" {
		t.Errorf("no common dir = %q, %q", text, path)
	}
	local := filepath.Join(".git", "itos", "notes.md")
	for _, printed := range []string{"", "other.md", "missing.md"} {
		if text, path := LocalNotes(ctx, v, printed, &strings.Builder{}); text != "Mine.\n" || path != local {
			t.Errorf("after %q = %q, %q", printed, text, path)
		}
	}
	if text, path := LocalNotes(ctx, v, filepath.Join(top, ".git", "itos", "notes.md"), &strings.Builder{}); text != "" || path != "" {
		t.Errorf("after the same file = %q, %q", text, path)
	}
}

// The way up to the top is a ../ a folder, none at the top or in the git
// folder; outside a repository there is none to give.
func TestUpToTop(t *testing.T) {
	top, v := repo(t)
	if err := os.MkdirAll(filepath.Join(top, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := upToTop(t.Context(), v); got != "" || !ok {
		t.Errorf("at the top = %q, %v", got, ok)
	}
	v.Root = filepath.Join(top, "a", "b")
	if got, ok := upToTop(t.Context(), v); got != "../../" || !ok {
		t.Errorf("from a/b = %q, %v", got, ok)
	}
	inGit := &vcsfake.VCS{Located: map[string]vcsport.Location{"": {GitDir: ".", Common: "."}}}
	if got, ok := upToTop(t.Context(), inGit); got != "" || !ok {
		t.Errorf("in the git folder = %q, %v", got, ok)
	}
	if got, ok := upToTop(t.Context(), &vcsfake.VCS{}); got != "" || ok {
		t.Errorf("outside a repository = %q, %v", got, ok)
	}
}
