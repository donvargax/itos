package draft

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
	domain "github.com/donvargax/itos/v8/internal/draft"
	privateport "github.com/donvargax/itos/v8/internal/store/private/port"
	privatefake "github.com/donvargax/itos/v8/internal/store/private/port/porttest"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// fake is draft's ports over the VCS's and the private store's fakes, the
// rest answering as its fields say: each err field the error its method
// gives.
type fake struct {
	*vcsfake.VCS
	store *privatefake.Store

	holdErr, releaseErr error
	entries             map[string]TreeEntry
	entriesErr          error
	blobs               map[string]string
	blobErr             error
	numstat             string
	numstatErr          error
	applied             map[string]string
	applyErr            error
	index               *index
	hooksErr            error
	self                string
	selfErr             error
	exit                int
	execErr             error
	ran                 [][]string
	runCode             int
	reserved            []string
	commitErr           error
	restoreErr          error
	untrackErr          error
	// then is done after Exec, RunLine or Blob runs, when set.
	then func()

	line        Line
	globalsErr  error
	read        []string
	readErr     error
	addKind     string
	addErr      error
	promoteKind string
	promoteErr  error
	taskErr     error
	cfgErr      error
	sound       Sound
	soundErr    error
	minted      string
	mintErr     error
	mintedFor   []string
}

func (f *fake) Hold(ctx context.Context, name string) (privateport.Held, error) {
	if f.holdErr != nil {
		return nil, f.holdErr
	}
	held, err := f.store.Hold(ctx, name)
	if err != nil || f.releaseErr == nil {
		return held, err
	}
	return stuck{f.releaseErr}, nil
}

// stuck is a lock that cannot be given back.
type stuck struct{ err error }

func (s stuck) Release(context.Context) error { return s.err }

func (f *fake) TreeEntries(_ context.Context, _ string, paths []string) (map[string]TreeEntry, error) {
	if f.entriesErr != nil {
		return nil, f.entriesErr
	}
	entries := map[string]TreeEntry{}
	for _, p := range paths {
		if e, ok := f.entries[p]; ok {
			entries[p] = e
		}
	}
	return entries, nil
}

func (f *fake) Blob(_ context.Context, object string) (string, error) {
	f.after()
	return f.blobs[object], f.blobErr
}

func (f *fake) PatchFiles(context.Context, string) (string, error) { return f.numstat, f.numstatErr }

// Apply sets the files of applied in the index and the work tree.
func (f *fake) Apply(context.Context, string) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	head, _ := f.VCS.Resolve(context.Background(), "HEAD")
	tree := f.VCS.History[len(f.VCS.History)-1].Tree
	for _, c := range f.VCS.History {
		if c.Name == head {
			tree = c.Tree
		}
	}
	f.VCS.Index, f.VCS.Work = maps.Clone(tree), maps.Clone(tree)
	for p, text := range f.applied {
		f.VCS.Index[p], f.VCS.Work[p] = text, text
	}
	return nil
}

func (f *fake) Scratch(context.Context, string) Index { return f.index }

func (f *fake) HooksReady(context.Context, string) error { return f.hooksErr }

func (f *fake) Self(context.Context) (string, error) { return f.self, f.selfErr }

func (f *fake) Exec(_ context.Context, self string, args []string, w io.Writer) (int, error) {
	f.ran = append(f.ran, append([]string{self}, args...))
	io.WriteString(w, "ran\n")
	f.after()
	return f.exit, f.execErr
}

func (f *fake) RunLine(_ context.Context, args []string, w io.Writer, reserved string) int {
	f.ran = append(f.ran, args)
	f.reserved = append(f.reserved, reserved)
	io.WriteString(w, "ran here\n")
	f.after()
	return f.runCode
}

func (f *fake) Getwd(context.Context) (string, error) { return os.Getwd() }

func (f *fake) CreateTemp(_ context.Context, dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

func (f *fake) Save(_ context.Context, file string, drafts domain.File) error {
	return domain.Save(file, drafts)
}

func (f *fake) after() {
	if f.then != nil {
		f.then()
	}
}

func (f *fake) Commit(ctx context.Context, c vcsport.Commit, talk vcsport.Talk) error {
	if f.commitErr != nil {
		return f.commitErr
	}
	return f.VCS.Commit(ctx, c, talk)
}

func (f *fake) Restore(ctx context.Context, paths []string) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	return f.VCS.Restore(ctx, paths)
}

func (f *fake) Untrack(ctx context.Context, paths []string) error {
	if f.untrackErr != nil {
		return f.untrackErr
	}
	return f.VCS.Untrack(ctx, paths)
}

func (f *fake) Globals(context.Context, []string) Line { return f.line }

func (f *fake) ApplyGlobals(context.Context, []string) error { return f.globalsErr }

func (f *fake) ReadLine(context.Context, []string) ([]string, error) { return f.read, f.readErr }

func (f *fake) WorkAddKind(context.Context, []string) (string, error) { return f.addKind, f.addErr }

func (f *fake) WorkPromoteKind(context.Context, []string) (string, error) {
	return f.promoteKind, f.promoteErr
}

func (f *fake) TaskAdd(context.Context, *config.Loaded, []string) error { return f.taskErr }

func (f *fake) Config(context.Context) (*config.Loaded, error) { return &config.Loaded{}, f.cfgErr }

func (f *fake) Sound(context.Context) (Sound, error) {
	if f.sound.Release == nil {
		f.sound.Release = func() {}
	}
	return f.sound, f.soundErr
}

func (f *fake) Mint(_ context.Context, _ *config.Loaded, kind string, registryIDs []string) (string, error) {
	f.mintedFor = append(append(f.mintedFor, kind), registryIDs...)
	return f.minted, f.mintErr
}

// index is a scratch index answering as its fields say, recording the
// paths taken out and set.
type index struct {
	readErr, addErr, removeErr, storeErr, setErr, changedErr, diffErr error
	changed                                                           []string
	diff                                                              string
	removed, set                                                      []string
	// then is done after Diff runs, when set.
	then func()
}

func (x *index) ReadTree(context.Context, string) error { return x.readErr }

func (x *index) AddAll(context.Context, []string) error { return x.addErr }

func (x *index) Remove(_ context.Context, path string) error {
	x.removed = append(x.removed, path)
	return x.removeErr
}

func (x *index) Store(_ context.Context, file string) (string, error) {
	return "object-of-" + filepath.Base(file), x.storeErr
}

func (x *index) Set(_ context.Context, mode, object, path string) error {
	x.set = append(x.set, mode+" "+object+" "+path)
	return x.setErr
}

func (x *index) Changed(context.Context, string, []string) ([]string, error) {
	return x.changed, x.changedErr
}

func (x *index) Diff(context.Context, string, []string) (string, error) {
	if x.then != nil {
		x.then()
	}
	return x.diff, x.diffErr
}

// repo is a scratch work tree itos stands at the top of, HEAD holding
// notes.md, and the fake over it: the work tree's folder and the drafts'
// file, whose folder is there.
func repo(t *testing.T) (*fake, string, string) {
	t.Helper()
	dir := t.TempDir()
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	t.Chdir(dir)
	common := filepath.Join(dir, ".git")
	file := filepath.Join(common, config.StealthFolder, domain.Folder, domain.FileName)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	f := &fake{
		VCS: &vcsfake.VCS{
			Located: map[string]vcsport.Location{"": {Top: dir, GitDir: common, Common: common}},
			History: []vcsfake.Commit{{Name: "c1", Message: "docs: start\n", Tree: map[string]string{"notes.md": "Old.\n"}}},
			Named:   map[string]string{"HEAD": "c1"},
		},
		store:   &privatefake.Store{Folder: filepath.Join(common, config.StealthFolder)},
		index:   &index{changed: []string{"notes.md"}, diff: "the patch\n"},
		entries: map[string]TreeEntry{"notes.md": {Mode: "100755", Kind: "blob", Object: "o1"}},
		blobs:   map[string]string{"o1": "Old.\n"},
	}
	return f, dir, file
}

// draft runs the line over f, as itos would with --json when json, from
// the folder origin names.
func draft(t *testing.T, f *fake, json bool, args ...string) (int, string, string, error) {
	t.Helper()
	return draftIn(t, f, "", json, false, args...)
}

func draftIn(t *testing.T, f *fake, origin string, json, quiet bool, args ...string) (int, string, string, error) {
	t.Helper()
	c, err := Parse(args, origin, json, quiet)
	if err != nil {
		t.Fatalf("draft %s: %v", strings.Join(args, " "), err)
	}
	var stdout, stderr strings.Builder
	code, err := c.Run(context.Background(), &stdout, &stderr, NewHandler(f))
	return code, stdout.String(), stderr.String(), err
}

// keep writes the drafts as the list.
func keep(t *testing.T, file string, drafts ...domain.Draft) {
	t.Helper()
	if err := domain.Save(file, domain.File{Drafts: drafts}); err != nil {
		t.Fatal(err)
	}
}

func loaded(t *testing.T, file string) []domain.Draft {
	t.Helper()
	drafts, err := domain.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return drafts.Drafts
}

// folderAt puts a folder holding a file where path is, which every OS
// refuses to read, write over or remove as a file.
func folderAt(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "taken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "taken", "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// block puts a folder holding a file where the file at path was, as
// folderAt does, from code that has no test to fail.
func block(path string) {
	_ = os.Remove(path)
	_ = os.MkdirAll(filepath.Join(path, "taken"), 0o700)
	_ = os.WriteFile(filepath.Join(path, "taken", "file"), nil, 0o600)
}

var errBroken = errors.New("broken")

// Outside a repository there are no drafts: exit 3, said on stderr.
func TestNoRepository(t *testing.T) {
	f, _, _ := repo(t)
	f.Located = nil
	for _, args := range [][]string{nil, {"promote"}, {"drop", "x"}} {
		code, stdout, stderr, err := draft(t, f, false, args...)
		if code != missing || err != nil || stdout != "" ||
			stderr != "itos: draft keeps its drafts in the git folder, and this is no git repository\n" {
			t.Errorf("draft %q: exit %d, %v, stdout %q, stderr %q", args, code, err, stdout, stderr)
		}
	}
}

// draft lists the drafts in the order they were added, or says there are
// none; --json lists each as an entry.
func TestList(t *testing.T) {
	f, _, file := repo(t)
	if code, stdout, _, err := draft(t, f, false); code != 0 || err != nil || stdout != "No drafts.\n" {
		t.Errorf("no drafts: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draft(t, f, true); code != 0 || err != nil || !strings.Contains(stdout, `"drafts": []`) {
		t.Errorf("no drafts under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	keep(t, file, domain.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}},
		domain.Draft{ID: "make-it", Command: []string{"work", "add", "--kind", "slice"}, Minted: "slice-5"})
	want := "Drafts, in the order itos draft promote applies them:\n" +
		"  say      docs: say  (notes.md)\n" +
		"  make-it  itos work add --kind slice  (makes slice-5)\n"
	if code, stdout, _, err := draft(t, f, false); code != 0 || err != nil || stdout != want {
		t.Errorf("draft: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draft(t, f, true); code != 0 || err != nil || !strings.Contains(stdout, `"kind": "command"`) ||
		!strings.Contains(stdout, `"minted": "slice-5"`) {
		t.Errorf("draft --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	folderAt(t, file)
	if code, _, _, err := draft(t, f, false); code != 0 || err == nil {
		t.Errorf("a list that cannot be read: exit %d, %v", code, err)
	}
}

// A lock that cannot be taken is an error, nothing read; one that cannot
// be given back is said last; a list that cannot be read under it is an
// error.
func TestTheListsLock(t *testing.T) {
	f, _, file := repo(t)
	f.holdErr = errBroken
	if code, _, _, err := draft(t, f, false, "drop", "x"); code != 0 || !errors.Is(err, errBroken) {
		t.Errorf("a lock not taken: exit %d, %v", code, err)
	}
	f.holdErr, f.releaseErr = nil, errBroken
	code, _, stderr, err := draft(t, f, false, "drop", "x")
	if code != refused || err != nil || !strings.HasSuffix(stderr, "itos: the lock cannot be given back: broken\n") ||
		!strings.HasPrefix(stderr, "there is no draft x\n") {
		t.Errorf("a lock not given back: exit %d, %v, stderr %q", code, err, stderr)
	}
	f.releaseErr, f.store = nil, &privatefake.Store{Folder: f.store.Folder}
	folderAt(t, file)
	if code, _, _, err := draft(t, f, false, "drop", "x"); code != 0 || err == nil {
		t.Errorf("a list that cannot be read: exit %d, %v", code, err)
	}
}

// draft drop takes the draft out and removes what it kept; one not there
// is refused.
func TestDrop(t *testing.T) {
	f, _, file := repo(t)
	keep(t, file, domain.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}},
		domain.Draft{ID: "other", Command: []string{"work", "check"}})
	if err := os.WriteFile(domain.Patch(file, "say"), []byte("patch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := domain.WriteFile(domain.Copy(file, "say", "notes.md"), []byte("copy")); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _, err := draft(t, f, false, "drop", "say"); code != 0 || err != nil || stdout != "say dropped: docs: say  (notes.md)\n" {
		t.Errorf("drop: exit %d, %v, stdout %q", code, err, stdout)
	}
	for _, gone := range []string{domain.Patch(file, "say"), domain.Copies(file, "say")} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s is left", gone)
		}
	}
	if drafts := loaded(t, file); len(drafts) != 1 || drafts[0].ID != "other" {
		t.Errorf("drafts %+v", drafts)
	}
	code, stdout, stderr, err := draft(t, f, false, "drop", "say")
	if code != refused || err != nil || stdout != "" || stderr != "there is no draft say\n  fix: itos draft lists the drafts\n" {
		t.Errorf("drop again: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if code, stdout, _, err := draft(t, f, true, "drop", "say"); code != refused || err != nil || !strings.Contains(stdout, `"rule": "draft-none"`) {
		t.Errorf("drop again under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draft(t, f, true, "drop", "other"); code != 0 || err != nil || !strings.Contains(stdout, `"ok": true`) {
		t.Errorf("drop under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	keep(t, file, domain.Draft{ID: "other", Command: []string{"work", "check"}})
	if code, stdout, _, err := draftIn(t, f, "", false, true, "drop", "other"); code != 0 || err != nil || stdout != "" {
		t.Errorf("drop under --quiet: exit %d, %v, stdout %q", code, err, stdout)
	}
	keep(t, file, domain.Draft{ID: "other", Command: []string{"work", "check"}})
	folderAt(t, domain.Patch(file, "other"))
	if code, _, _, err := draft(t, f, false, "drop", "other"); code != 0 || err == nil {
		t.Errorf("a patch that cannot be removed: exit %d, %v", code, err)
	}
}

// draft add of a change keeps its patch, puts its files back as HEAD has
// them and says what it drafted.
func TestAddAChange(t *testing.T) {
	f, dir, file := repo(t)
	f.Work = map[string]string{"notes.md": "New.\n", "new/deep/file.md": "Made.\n"}
	f.index.changed = []string{"notes.md", "new/deep/file.md"}
	if err := os.MkdirAll(filepath.Join(dir, "new", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new", "deep", "file.md"), []byte("Made.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "new"))
	code, stdout, stderr, err := draftIn(t, f, "", false, false, "add", "say", "-m", "docs: say\n\nWhy.", "../notes.md", "deep")
	if code != 0 || err != nil || stdout != "say drafted: docs: say  (notes.md, new/deep)\n" {
		t.Fatalf("add: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if here, _ := os.Getwd(); here != dir {
		t.Errorf("add left itos in %s, not the top", here)
	}
	if patch, err := os.ReadFile(domain.Patch(file, "say")); err != nil || string(patch) != "the patch\n" {
		t.Errorf("the patch holds %q (%v)", patch, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err == nil {
		t.Error("the folders the new file left empty are there")
	}
	if f.Work["notes.md"] != "Old.\n" {
		t.Errorf("notes.md is %q, not put back", f.Work["notes.md"])
	}
	if drafts := loaded(t, file); len(drafts) != 1 || drafts[0].Message != "docs: say\n\nWhy." {
		t.Errorf("drafts %+v", drafts)
	}
	code, stdout, _, err = draft(t, f, true, "add", "say", "-m", "docs: again", "notes.md")
	if code != refused || err != nil || !strings.Contains(stdout, `"rule": "draft-id-taken"`) {
		t.Errorf("an id taken: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draft(t, f, true, "add", "json", "-m", "docs: json", "notes.md"); code != 0 || err != nil ||
		!strings.Contains(stdout, `"kind": "change"`) {
		t.Errorf("add under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draftIn(t, f, "", false, true, "add", "quiet", "-m", "docs: quiet", "notes.md"); code != 0 || err != nil || stdout != "" {
		t.Errorf("add under --quiet: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// What draft add cannot draft is refused, nothing written; what it cannot
// do is an error.
func TestAddAChangeRefusedOrFailing(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*fake, string)
		refusal string
		err     string
	}{
		"a path outside the work tree": {set: func(*fake, string) {}, err: "draft add: ../out is outside the work tree"},
		"no work tree": {set: func(f *fake, _ string) { f.Located = map[string]vcsport.Location{"": {Common: f.Located[""].Common}} },
			err: "the work tree's top cannot be read: no work tree"},
		"no HEAD":       {set: func(f *fake, _ string) { f.Named = nil }, refusal: "the repository has no commit yet"},
		"no read-tree":  {set: func(f *fake, _ string) { f.index.readErr = errBroken }, err: "git read-tree HEAD: broken"},
		"nothing there": {set: func(f *fake, _ string) { f.index.addErr = errBroken }, refusal: "notes.md cannot be drafted: broken"},
		"no names":      {set: func(f *fake, _ string) { f.index.changedErr = errBroken }, err: "git diff --cached: broken"},
		"no change":     {set: func(f *fake, _ string) { f.index.changed = nil }, refusal: "notes.md has no change against HEAD to draft"},
		"no diff":       {set: func(f *fake, _ string) { f.index.diffErr = errBroken }, err: "git diff --cached --binary: broken"},
		"no patch":      {set: func(_ *fake, file string) { folderAt(t, domain.Patch(file, "say")) }, err: "say.patch"},
		"no list": {set: func(f *fake, file string) { f.index.then = func() { block(file) } },
			err: "drafts.yaml"},
		"no put back": {set: func(f *fake, _ string) { f.restoreErr = &vcsport.Failed{Words: "refused\n"} },
			err: "say is drafted, but its files cannot be put back as HEAD has them: git checkout HEAD -- notes.md: refused"},
		"no scratch": {set: func(_ *fake, file string) {
			if err := os.RemoveAll(filepath.Dir(file)); err != nil {
				t.Fatal(err)
			}
		}, err: domain.Folder},
	} {
		t.Run(name, func(t *testing.T) {
			f, _, file := repo(t)
			c.set(f, file)
			path := "notes.md"
			if name == "a path outside the work tree" {
				path = "../out"
			}
			code, stdout, stderr, err := draft(t, f, false, "add", "say", "-m", "docs: say", path)
			switch {
			case c.refusal != "":
				if code != refused || err != nil || !strings.Contains(stderr, c.refusal) {
					t.Errorf("exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
				}
			case code != 0 || err == nil || !strings.Contains(err.Error(), c.err):
				t.Errorf("exit %d, %v, want an error naming %q", code, err, c.err)
			}
			if name != "no put back" {
				if _, err := os.Stat(domain.Patch(file, "say")); err == nil && name != "no patch" {
					t.Error("the patch is left")
				}
			}
		})
	}
}

// putBack restores the files HEAD has and removes the ones it lacks,
// saying each failure.
func TestPutBack(t *testing.T) {
	f, dir, _ := repo(t)
	h := NewHandler(f)
	f.Work = map[string]string{"notes.md": "New.\n", "made.md": "Made.\n"}
	f.Index = map[string]string{"notes.md": "Old.\n", "made.md": "Made.\n"}
	if err := os.WriteFile(filepath.Join(dir, "made.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.putBack(context.Background(), []string{"notes.md", "made.md", "gone.md"}); err != nil {
		t.Fatal(err)
	}
	if _, tracked := f.Index["made.md"]; tracked || f.Work["notes.md"] != "Old.\n" {
		t.Errorf("index %v, work %v", f.Index, f.Work)
	}
	f.restoreErr, f.untrackErr = errBroken, &vcsport.Failed{Words: " refused \n"}
	folderAt(t, filepath.Join(dir, "made.md"))
	err := h.putBack(context.Background(), []string{"notes.md", "made.md"})
	for _, want := range []string{"git checkout HEAD -- notes.md: broken", "git rm --cached -- made.md: refused", "made.md"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("putBack: %v, not naming %q", err, want)
		}
	}
}

// draft add of a command keeps it, minting the id it would mint.
func TestAddACommand(t *testing.T) {
	f, _, file := repo(t)
	f.line = Line{Rest: []string{"work", "add"}}
	f.read = []string{"work", "add", "--kind", "slice"}
	f.addKind, f.minted = "slice", "slice-5"
	f.sound = Sound{Cfg: &config.Loaded{}}
	code, stdout, _, err := draft(t, f, false, "add", "thing", "--", "work", "add", "--kind", "slice")
	if code != 0 || err != nil || stdout != "thing drafted: itos work add --kind slice  (makes slice-5)\n" {
		t.Errorf("add: exit %d, %v, stdout %q", code, err, stdout)
	}
	f.sound = Sound{Code: refused}
	if code, _, _, err := draft(t, f, false, "add", "unsound", "--", "work", "add", "--kind", "slice"); code != refused || err != nil {
		t.Errorf("an unsound registry: exit %d, %v", code, err)
	}
	f.sound, f.soundErr = Sound{}, errBroken
	if code, _, _, err := draft(t, f, false, "add", "broken", "--", "work", "add", "--kind", "slice"); code != 0 || !errors.Is(err, errBroken) {
		t.Errorf("a registry not read: exit %d, %v", code, err)
	}
	if drafts := loaded(t, file); len(drafts) != 1 || drafts[0].Minted != "slice-5" {
		t.Errorf("drafts %+v", drafts)
	}
}

// draft edit writes a copy of each path as HEAD has it, empty for one HEAD
// lacks, and prints where; HEAD is the draft's base.
func TestEdit(t *testing.T) {
	f, _, file := repo(t)
	code, stdout, stderr, err := draft(t, f, false, "edit", "say", "-m", "docs: say", "notes.md", "new.md", "notes.md")
	want := domain.Copy(file, "say", "notes.md") + "\n" + domain.Copy(file, "say", "new.md") + "\n"
	if code != 0 || err != nil || stdout != want {
		t.Fatalf("edit: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	for path, text := range map[string]string{"notes.md": "Old.\n", "new.md": ""} {
		if got, err := os.ReadFile(domain.Copy(file, "say", path)); err != nil || string(got) != text {
			t.Errorf("the copy of %s holds %q (%v)", path, got, err)
		}
	}
	if drafts := loaded(t, file); len(drafts) != 1 || drafts[0].Base != "c1" || len(drafts[0].Paths) != 2 {
		t.Errorf("drafts %+v", drafts)
	}
	if code, stdout, _, err := draft(t, f, false, "edit", "say", "-m", "docs: say", "notes.md"); code != refused || err != nil || stdout != "" {
		t.Errorf("an id taken: exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err = draft(t, f, true, "edit", "json", "-m", "docs: json", "notes.md")
	if code != 0 || err != nil || !strings.Contains(stdout, `"copies": [`) || !strings.Contains(stdout, `"kind": "change"`) {
		t.Errorf("edit under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// What draft edit cannot copy is refused, nothing written; what it cannot
// do is an error, its copies removed.
func TestEditRefusedOrFailing(t *testing.T) {
	for name, c := range map[string]struct {
		set     func(*fake, string)
		paths   []string
		refusal string
		err     string
	}{
		"outside":  {set: func(*fake, string) {}, paths: []string{"../out"}, err: "draft edit: ../out is outside the work tree"},
		"no HEAD":  {set: func(f *fake, _ string) { f.Named = nil }, refusal: "the repository has no commit yet, and a copy is taken from HEAD"},
		"no tree":  {set: func(f *fake, _ string) { f.entriesErr = errBroken }, err: "git ls-tree c1: broken"},
		"a folder": {set: func(f *fake, _ string) { f.entries["notes.md"] = TreeEntry{Kind: "tree"} }, refusal: "notes.md is no file in HEAD, but a folder"},
		"the top":  {set: func(*fake, string) {}, paths: []string{".", "notes.md", "sub"}, refusal: ". is no file in HEAD"},
		"two":      {set: func(f *fake, _ string) { f.entries["sub"] = TreeEntry{Kind: "commit"} }, paths: []string{".", "sub"}, refusal: ". and sub are no file in HEAD"},
		"no blob":  {set: func(f *fake, _ string) { f.blobErr = errBroken }, err: "git cat-file blob HEAD:notes.md: broken"},
		"no copy":  {set: func(*fake, string) {}, paths: []string{"new", "new/deeper"}, err: "new"},
		"no list":  {set: func(f *fake, file string) { f.then = func() { block(file) } }, err: "drafts.yaml"},
		"no folder": {set: func(_ *fake, file string) {
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), domain.Edits), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, err: domain.Edits},
	} {
		t.Run(name, func(t *testing.T) {
			f, _, file := repo(t)
			c.set(f, file)
			paths := c.paths
			if paths == nil {
				paths = []string{"notes.md"}
			}
			code, _, stderr, err := draft(t, f, false, append([]string{"edit", "say", "-m", "docs: say"}, paths...)...)
			switch {
			case c.refusal != "":
				if code != refused || err != nil || !strings.Contains(stderr, c.refusal) {
					t.Errorf("exit %d, %v, stderr %q", code, err, stderr)
				}
			case code != 0 || err == nil || !strings.Contains(err.Error(), c.err):
				t.Errorf("exit %d, %v, want an error naming %q", code, err, c.err)
			}
			if _, err := os.Stat(domain.Copies(file, "say")); err == nil {
				t.Error("the copies are left")
			}
		})
	}
}
