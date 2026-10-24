package evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	privport "github.com/donvargax/itos/v8/internal/store/private/port"
	treefake "github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// load is a config over the files given, as the working tree holds them.
func load(t *testing.T, text string) *config.Loaded {
	t.Helper()
	trees := treefake.Trees{Files: map[string]map[string]string{"worktree": {}}}
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

// fakeGit is the repository: a tree, the paths its index holds, the paths
// that differ in the working folder, and a checkout it makes in a folder of
// the test's, or refuses.
type fakeGit struct {
	t           *testing.T
	indexedErr  error
	badCheckout bool
	tree        string
	treeErr     error
	indexed     []string
	unstaged    []string
	unstagedErr error
	snapshotErr error
	checkouts   int
	removed     int
	checkout    string
}

func (g *fakeGit) IndexTree(context.Context) (string, error) { return g.tree, g.treeErr }
func (g *fakeGit) IndexPaths(context.Context) ([]string, error) {
	return g.indexed, g.indexedErr
}
func (g *fakeGit) UnstagedPaths(context.Context) ([]string, error) {
	return g.unstaged, g.unstagedErr
}
func (g *fakeGit) Snapshot(context.Context, string) (Checkout, error) {
	if g.snapshotErr != nil {
		return Checkout{}, g.snapshotErr
	}
	g.checkouts++
	g.checkout = filepath.Join(g.t.TempDir(), "checkout")
	if err := os.MkdirAll(g.checkout, 0o755); err != nil {
		g.t.Fatal(err)
	}
	if g.badCheckout {
		// A checkout whose folder is a file: nothing can be copied into it.
		if err := os.RemoveAll(g.checkout); err != nil {
			g.t.Fatal(err)
		}
		if err := os.WriteFile(g.checkout, nil, 0o644); err != nil {
			g.t.Fatal(err)
		}
	}
	return Checkout{Dir: g.checkout, Env: append(os.Environ(), "IN_CHECKOUT=yes"), Remove: func() error {
		g.removed++
		return os.RemoveAll(g.checkout)
	}}, nil
}

// diskStore is the private store over a folder, with a lock it can refuse.
type diskStore struct {
	root    string
	holdErr error
	pathErr error
	held    int
}

func (s *diskStore) Path(_ context.Context, name string) (string, error) {
	return filepath.Join(s.root, name), s.pathErr
}
func (s *diskStore) Read(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (s *diskStore) Write(context.Context, string, []byte) error        { return nil }
func (s *diskStore) NotIgnored(context.Context, string) bool            { return false }
func (s *diskStore) Hold(context.Context, string) (privport.Held, error) {
	if s.holdErr != nil {
		return nil, s.holdErr
	}
	s.held++
	return released{}, nil
}

type released struct{}

func (released) Release(context.Context) error { return nil }

// rig is an engine over the fakes, the folder it keeps records in and the
// working folder its declared inputs are read from.
type rig struct {
	*Engine
	git   *fakeGit
	store *diskStore
	log   *bytes.Buffer
	work  string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	git := &fakeGit{t: t, tree: "tree-one"}
	store := &diskStore{root: filepath.Join(t.TempDir(), "itos")}
	work := t.TempDir()
	return &rig{
		Engine: &Engine{
			Cfg: load(t, "version: 1\n"), Git: git, Store: store, Work: work, Mode: "task",
			Binding: Binding{OS: "plan9", Arch: "mips", Version: "1.2.3"},
			Now:     func() time.Time { return time.Date(2026, 10, 24, 1, 2, 3, 0, time.UTC) },
		},
		git: git, store: store, log: &bytes.Buffer{}, work: work,
	}
}

func (r *rig) request(command string, decl config.Evidence) Request {
	return Request{Command: command, Decl: decl, Log: r.log}
}

// records are the complete records in the store.
func (r *rig) records(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(r.store.root, Folder))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if e.IsDir() && !strings.Contains(e.Name(), ".") {
			got = append(got, e.Name())
		}
	}
	return got
}

// A miss runs in a checkout and records; the same check again answers from
// the record, running nothing, with the recorded exit and output.
func TestMissRecordsAndRepeatHits(t *testing.T) {
	r := newRig(t)
	marks := filepath.ToSlash(filepath.Join(t.TempDir(), "runs"))
	command := "echo ran >> " + marks + "; echo 'the output'; exit 3"
	first := r.Run(t.Context(), r.request(command, config.Evidence{}))
	if first.OK || first.Code != 3 || first.Info.Hit || !first.Info.Recorded || first.Info.Reason != "" {
		t.Fatalf("a miss: %+v", first)
	}
	if first.Info.Source != Source || first.Info.Tree != "tree-one" || len(first.Info.Key) != 64 || first.Info.RecordedAt != "2026-10-24T01:02:03Z" {
		t.Errorf("the miss's evidence: %+v", first.Info)
	}
	if first.Info.Provenance["itos"] != "1.2.3" || first.Info.Provenance["mode"] != "task" {
		t.Errorf("provenance: %v", first.Info.Provenance)
	}
	if r.git.checkouts != 1 || r.git.removed != 1 || len(r.records(t)) != 1 {
		t.Errorf("checkouts %d removed %d records %v", r.git.checkouts, r.git.removed, r.records(t))
	}
	if r.store.held != 1 {
		t.Errorf("the lock was taken %d times", r.store.held)
	}
	r.log.Reset()
	req := r.request(command, config.Evidence{})
	req.Verbose = true
	again := r.Run(t.Context(), req)
	if again.OK || again.Code != 3 || !again.Info.Hit || again.Info.Recorded || again.Info.Key != first.Info.Key {
		t.Fatalf("a hit: %+v", again)
	}
	if again.Info.RecordedAt != first.Info.RecordedAt || again.Info.Provenance["os"] != "plan9" {
		t.Errorf("a hit says where the record came from: %+v", again.Info)
	}
	out := r.log.String()
	if !strings.Contains(out, "cached evidence") || !strings.Contains(out, "exit 3") || !strings.Contains(out, "the output\n") {
		t.Errorf("a hit prints %q", out)
	}
	text, err := os.ReadFile(marks)
	if err != nil || string(text) != "ran\n" {
		t.Errorf("the check ran %q times: %v", text, err)
	}
	if r.git.checkouts != 1 {
		t.Errorf("a hit made a checkout")
	}
	if len(r.Entries()) != 2 || r.Entries()[1].Command != command || !r.Entries()[1].Hit {
		t.Errorf("entries: %+v", r.Entries())
	}
}

// The check runs in the checkout, with the environment the checkout gives,
// and a non-verbose miss names its command in its status line.
func TestMissRunsInTheCheckout(t *testing.T) {
	r := newRig(t)
	command := "test \"$IN_CHECKOUT\" = yes && pwd > where"
	if o := r.Run(t.Context(), r.request(command, config.Evidence{})); !o.OK {
		t.Fatalf("the check failed: %+v", o)
	}
	if _, err := os.Stat(filepath.Join(r.work, "where")); err == nil {
		t.Error("the check ran in the working folder")
	}
	if !strings.Contains(r.log.String(), command+": running") {
		t.Errorf("the status line: %q", r.log.String())
	}
}

// A different tree, command, timeout or declaration is another key.
func TestAnotherTreeCommandTimeoutOrDeclarationMisses(t *testing.T) {
	r := newRig(t)
	run := func(command string, seconds float64, decl config.Evidence) Info {
		req := r.request(command, decl)
		req.Seconds = seconds
		return r.Run(t.Context(), req).Info
	}
	base := run("true", 0, config.Evidence{})
	for name, info := range map[string]Info{
		"command":     run("true;", 0, config.Evidence{}),
		"timeout":     run("true", 5, config.Evidence{}),
		"declaration": run("true", 0, config.Evidence{Env: []string{"ITOS_EVIDENCE_TEST_UNSET"}}),
	} {
		if info.Hit || info.Key == base.Key {
			t.Errorf("another %s hit or shared the key: %+v", name, info)
		}
	}
	r.git.tree = "tree-two"
	r.tree, r.treeKnown = "", false
	if info := run("true", 0, config.Evidence{}); info.Hit || info.Key == base.Key {
		t.Errorf("another tree hit: %+v", info)
	}
	if got := len(r.records(t)); got != 5 {
		t.Errorf("%d records", got)
	}
}

// What cannot be done never changes the verdict: the check runs, and the
// evidence says why nothing was recorded.
func TestWhatCannotBeDoneRunsUncached(t *testing.T) {
	for _, c := range []struct {
		name       string
		arrange    func(r *rig)
		reason     string
		inCheckout bool
	}{
		{"no index tree", func(r *rig) { r.git.treeErr = errors.New("no repository") }, ReasonSnapshotUnusable, false},
		{"a checkout that cannot be made", func(r *rig) { r.git.snapshotErr = errors.New("a gitlink") }, ReasonSnapshotUnusable, false},
		{"a lock held too long", func(r *rig) { r.store.holdErr = kind.Wrap(kind.Temporary, errors.New("held")) }, ReasonLockTimeout, true},
		{"a lock that cannot be made", func(r *rig) { r.store.holdErr = errors.New("not a directory") }, ReasonStoreUnusable, true},
		{"a store that cannot be written", func(r *rig) {
			if err := os.MkdirAll(r.store.root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(r.store.root, Folder), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, ReasonStoreUnusable, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t)
			c.arrange(r)
			where := filepath.ToSlash(filepath.Join(t.TempDir(), "where"))
			req := r.request("pwd > "+where+"; exit 4", config.Evidence{})
			o := r.Run(t.Context(), req)
			if o.OK || o.Code != 4 || o.Info.Hit || o.Info.Recorded || o.Info.Reason != c.reason {
				t.Errorf("%+v", o)
			}
			ran, err := os.ReadFile(where)
			if err != nil {
				t.Fatal(err)
			}
			if inCheckout := strings.Contains(string(ran), "checkout"); inCheckout != c.inCheckout {
				t.Errorf("the check ran in %q, in the checkout %v", ran, inCheckout)
			}
			if strings.Contains(c.name, "store") && len(r.records(t)) != 0 {
				t.Errorf("records: %v", r.records(t))
			}
		})
	}
}

// An unstaged change is counted and said, once, for the first check looked
// up; a count git cannot give is none.
func TestUnstagedChangesAreCountedAndSaidOnce(t *testing.T) {
	r := newRig(t)
	r.Hint = " Hint."
	r.git.unstaged = []string{"a", "b"}
	r.Run(t.Context(), r.request("true", config.Evidence{}))
	r.Run(t.Context(), r.request("true;", config.Evidence{}))
	n, counted := r.Unstaged()
	if n != 2 || !counted || !r.Used() {
		t.Errorf("unstaged %d %v used %v", n, counted, r.Used())
	}
	if got := strings.Count(r.log.String(), "2 changed paths not staged"); got != 1 || !strings.Contains(r.log.String(), "staged tree. Hint.\n") {
		t.Errorf("said %d times: %q", got, r.log.String())
	}
	one := newRig(t)
	one.git.unstaged = []string{"a"}
	one.Run(t.Context(), one.request("true", config.Evidence{}))
	if !strings.Contains(one.log.String(), "1 changed path not staged") {
		t.Errorf("%q", one.log.String())
	}
	none := newRig(t)
	none.Run(t.Context(), none.request("true", config.Evidence{}))
	if n, counted := none.Unstaged(); n != 0 || !counted || strings.Contains(none.log.String(), "not staged") {
		t.Errorf("no change: %d %v %q", n, counted, none.log.String())
	}
	broken := newRig(t)
	broken.git.unstagedErr = errors.New("git")
	broken.Run(t.Context(), broken.request("true", config.Evidence{}))
	if _, counted := broken.Unstaged(); counted {
		t.Error("a count git could not give")
	}
	unused := newRig(t)
	if unused.Used() {
		t.Error("used before any check")
	}
}

// A result that is not the check's to reuse is run and not recorded: a
// timeout.
func TestATimeoutIsNotRecorded(t *testing.T) {
	r := newRig(t)
	req := r.request("cd /; sleep 5; true", config.Evidence{})
	req.Seconds = 0.05
	started := time.Now()
	o := r.Run(t.Context(), req)
	if took := time.Since(started); took > 4*time.Second {
		t.Errorf("the run took %v, past its timeout and the sleeper's left", took)
	}
	if o.OK || o.Info.Recorded || o.Info.Reason != ReasonInconclusive {
		t.Errorf("%+v", o)
	}
	if len(r.records(t)) != 0 {
		t.Errorf("records %v", r.records(t))
	}
}

// A record found broken is a miss: it is renamed aside under its reason, the
// check runs and records, and the next run hits.
func TestADamagedRecordIsQuarantinedAndRerecorded(t *testing.T) {
	for name, c := range map[string]struct {
		damage func(dir string) error
		reason string
	}{
		"no marker": {func(dir string) error { return os.Remove(filepath.Join(dir, complete)) }, "markerless"},
		"no result": {func(dir string) error { return os.Remove(filepath.Join(dir, resultFile)) }, "corrupt"},
		"a NUL manifest": {func(dir string) error {
			return os.WriteFile(filepath.Join(dir, manifest), []byte("\x00"), 0o600)
		}, "corrupt"},
		"another key": {func(dir string) error {
			return os.WriteFile(filepath.Join(dir, manifest), []byte(`{"key":"x","schema":1}`), 0o600)
		}, "corrupt"},
		"a manifest that is not its key's": {func(dir string) error {
			text, err := os.ReadFile(filepath.Join(dir, manifest))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, manifest), bytes.Replace(text, []byte("tree-one"), []byte("tree-two"), 1), 0o600)
		}, "corrupt"},
		"another schema": {func(dir string) error {
			text, err := os.ReadFile(filepath.Join(dir, manifest))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, manifest), bytes.Replace(text, []byte(`"schema": 1`), []byte(`"schema": 2`), 1), 0o600)
		}, "corrupt"},
		"a regular file": {func(dir string) error {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			return os.WriteFile(dir, []byte("x"), 0o600)
		}, "corrupt"},
		"a folder with nothing in it": {func(dir string) error {
			return errors.Join(os.RemoveAll(dir), os.Mkdir(dir, 0o700), os.WriteFile(filepath.Join(dir, "other"), nil, 0o600))
		}, "markerless"},
	} {
		for _, renames := range []string{"as Unix renames", "as Windows renames over a file"} {
			t.Run(name+" "+renames, func(t *testing.T) {
				if strings.Contains(renames, "Windows") {
					replacesFiles(t)
				}
				r := newRig(t)
				first := r.Run(t.Context(), r.request("true", config.Evidence{}))
				root := filepath.Join(r.store.root, Folder)
				if err := c.damage(filepath.Join(root, first.Info.Key)); err != nil {
					t.Fatal(err)
				}
				again := r.Run(t.Context(), r.request("true", config.Evidence{}))
				if again.Info.Hit || !again.Info.Recorded || again.Info.Reason != "" || !again.OK {
					t.Errorf("%+v", again)
				}
				aside := first.Info.Key + "." + c.reason + "-20261024T010203.000000Z"
				if _, err := os.Lstat(filepath.Join(root, aside)); err != nil {
					t.Errorf("the damaged record is not quarantined as %s: %v", aside, err)
				}
				if got := r.records(t); len(got) != 1 || got[0] != first.Info.Key {
					t.Errorf("records %v", got)
				}
				if third := r.Run(t.Context(), r.request("true", config.Evidence{})); !third.Info.Hit {
					t.Errorf("the record made after the quarantine is not found: %+v", third)
				}
				if left, _ := filepath.Glob(filepath.Join(root, ".tmp-*")); len(left) != 0 {
					t.Errorf("left %v", left)
				}
			})
		}
	}
}

// replacesFiles makes rename as Windows renames (MoveFileEx replacing the
// destination): a regular file at the destination is replaced, where Unix
// refuses to rename a folder onto it.
func replacesFiles(t *testing.T) {
	t.Helper()
	was := rename
	t.Cleanup(func() { rename = was })
	rename = func(from, to string) error {
		if info, err := os.Lstat(to); err == nil && info.Mode().IsRegular() {
			if err := os.Remove(to); err != nil {
				return err
			}
		}
		return was(from, to)
	}
}

// failingRename is a rename that fails on the calls listed (counting from
// one), doing before on the first, and renames on the others.
func failingRename(t *testing.T, calls []int, before func()) {
	t.Helper()
	was := rename
	t.Cleanup(func() { rename = was })
	call := 0
	rename = func(from, to string) error {
		call++
		if call == 1 && before != nil {
			before()
		}
		if slices.Contains(calls, call) {
			return errors.New("the rename fails")
		}
		return was(from, to)
	}
}

// A rename that fails judges what is at the key: a whole record that another
// writer made is kept as it is and this one dropped, the check answering
// with its own result and recording nothing.
func TestARenameThatFailsKeepsAWholeRecordAtTheKey(t *testing.T) {
	other := newRig(t)
	other.Mode = "ci"
	other.Now = func() time.Time { return time.Date(2026, 10, 23, 9, 8, 7, 0, time.UTC) }
	first := other.Run(t.Context(), other.request("exit 2", config.Evidence{}))
	r := newRig(t)
	root := filepath.Join(r.store.root, Folder)
	failingRename(t, []int{1}, func() {
		if err := os.CopyFS(root, os.DirFS(filepath.Join(other.store.root, Folder))); err != nil {
			t.Fatal(err)
		}
	})
	o := r.Run(t.Context(), r.request("exit 2", config.Evidence{}))
	if o.OK || o.Code != 2 || o.Info.Hit || o.Info.Recorded || o.Info.Reason != ReasonAlreadyRecorded || o.Info.Key != first.Info.Key {
		t.Errorf("%+v", o)
	}
	if o.Info.RecordedAt != "2026-10-23T09:08:07Z" || o.Info.Provenance["itos"] != "1.2.3" || o.Info.Provenance["mode"] != "ci" {
		t.Errorf("the kept record's time and provenance: %+v", o.Info)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != first.Info.Key {
		t.Errorf("the store holds %v, %v", entries, err)
	}
	if again := r.Run(t.Context(), r.request("exit 2", config.Evidence{})); !again.Info.Hit || again.Code != 2 {
		t.Errorf("%+v", again)
	}
}

// A whole record found at the key before the rename is kept without trying
// the rename, whatever an OS would let it replace.
func TestAWholeRecordAtTheKeyBeforeTheRenameIsKept(t *testing.T) {
	other := newRig(t)
	first := other.Run(t.Context(), other.request("exit 2", config.Evidence{}))
	r := newRig(t)
	root := filepath.Join(r.store.root, Folder)
	renames := 0
	was, wasTemp := rename, mkdirTemp
	t.Cleanup(func() { rename, mkdirTemp = was, wasTemp })
	rename = func(string, string) error { renames++; return nil }
	mkdirTemp = func(dir, pattern string) (string, error) {
		must(t, os.CopyFS(root, os.DirFS(filepath.Join(other.store.root, Folder))))
		return os.MkdirTemp(dir, pattern)
	}
	o := r.Run(t.Context(), r.request("exit 2", config.Evidence{}))
	if renames != 0 || o.Info.Reason != ReasonAlreadyRecorded || o.Info.Recorded || o.Info.Key != first.Info.Key {
		t.Errorf("%d renames: %+v", renames, o)
	}
	if left, _ := filepath.Glob(filepath.Join(root, ".tmp-*")); len(left) != 0 {
		t.Errorf("left %v", left)
	}
}

// A rename that fails with a broken record at the key quarantines it and
// renames once more; a rename that fails again, or with nothing at the key to
// judge, leaves no record and no temporary folder, the check unchanged.
func TestARenameThatFailsWithoutAWholeRecordAtTheKey(t *testing.T) {
	for name, c := range map[string]struct {
		fails    []int
		before   func(root, key string)
		recorded bool
		reason   string
		aside    string
	}{
		"a broken record, then a rename that works": {[]int{1}, func(root, key string) {
			must(t, os.MkdirAll(filepath.Join(root, key), 0o700))
		}, true, "", "markerless"},
		"a broken record, then a rename that fails": {[]int{1, 3}, func(root, key string) {
			must(t, os.WriteFile(filepath.Join(root, key), nil, 0o600))
		}, false, ReasonStoreUnusable, "corrupt"},
		"a broken record that cannot be moved aside": {[]int{1, 2}, func(root, key string) {
			must(t, os.WriteFile(filepath.Join(root, key), nil, 0o600))
		}, false, ReasonStoreUnusable, ""},
		"nothing to judge": {[]int{1}, func(string, string) {}, false, ReasonStoreUnusable, ""},
	} {
		t.Run(name, func(t *testing.T) {
			key := newRig(t).Run(t.Context(), newRig(t).request("exit 0", config.Evidence{})).Info.Key
			r := newRig(t)
			root := filepath.Join(r.store.root, Folder)
			failingRename(t, c.fails, func() { c.before(root, key) })
			o := r.Run(t.Context(), r.request("exit 0", config.Evidence{}))
			if !o.OK || o.Info.Hit || o.Info.Recorded != c.recorded || o.Info.Reason != c.reason {
				t.Errorf("%+v", o)
			}
			aside := filepath.Join(root, key+"."+c.aside+"-20261024T010203.000000Z")
			_, err := os.Lstat(aside)
			if (err == nil) != (c.aside != "") {
				t.Errorf("quarantined as %q: %v", c.aside, err)
			}
			if left, _ := filepath.Glob(filepath.Join(root, ".tmp-*")); len(left) != 0 {
				t.Errorf("left %v", left)
			}
			_, err = os.Stat(filepath.Join(root, key, complete))
			if (err == nil) != c.recorded {
				t.Errorf("the record at the key: %v", err)
			}
		})
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The record is written in a temporary folder and renamed, never leaving
// the temporary folder behind, and a writer that waited for the lock looks
// again before it runs.
func TestAWriterThatWaitedLooksAgain(t *testing.T) {
	r := newRig(t)
	first := r.Run(t.Context(), r.request("true", config.Evidence{}))
	entries, err := os.ReadDir(filepath.Join(r.store.root, Folder))
	if err != nil || len(entries) != 1 || entries[0].Name() != first.Info.Key {
		t.Fatalf("the store holds %v, %v", entries, err)
	}
	// A lookup that misses before the lock and hits after it: the record is
	// put in place while the lock is held.
	second := newRig(t)
	second.Store = &appearingStore{diskStore: second.store, put: func() {
		if err := os.CopyFS(second.store.root, os.DirFS(r.store.root)); err != nil {
			t.Fatal(err)
		}
	}}
	o := second.Run(t.Context(), second.request("true", config.Evidence{}))
	if !o.Info.Hit || !o.OK || second.git.checkouts != 0 {
		t.Errorf("a writer that waited ran the check: %+v, %d checkouts", o, second.git.checkouts)
	}
}

// appearingStore puts a record in place when its lock is taken.
type appearingStore struct {
	*diskStore
	put func()
}

func (s *appearingStore) Hold(ctx context.Context, name string) (privport.Held, error) {
	s.put()
	return s.diskStore.Hold(ctx, name)
}

// Output past the tail is cut to its end, and the record says so.
func TestTheOutputIsATail(t *testing.T) {
	var w tail
	w.max = 4
	for _, p := range []string{"ab", "cd", "ef", "gh", "ij"} {
		if n, err := w.Write([]byte(p)); n != 2 || err != nil {
			t.Fatal(n, err)
		}
	}
	if text, cut := w.Text(); text != "ghij" || !cut {
		t.Errorf("%q %v", text, cut)
	}
	short := tail{max: 8}
	_, _ = short.Write([]byte("abc"))
	if text, cut := short.Text(); text != "abc" || cut {
		t.Errorf("%q %v", text, cut)
	}
	exact := tail{max: 3}
	_, _ = exact.Write([]byte("abcd"))
	if text, cut := exact.Text(); text != "bcd" || !cut {
		t.Errorf("%q %v", text, cut)
	}
	r := newRig(t)
	big := fmt.Sprintf("i=0; while [ $i -lt %d ]; do echo 0123456789; i=$((i+1)); done", OutputTail/5)
	o := r.Run(t.Context(), r.request(big, config.Evidence{}))
	rec, ok := r.lookup(t.Context(), o.Info.Key)
	if !ok || !rec.result.Truncated || len(rec.result.Output) != OutputTail {
		t.Errorf("the record's output: %v, %v, %d bytes", ok, rec.result.Truncated, len(rec.result.Output))
	}
}

// A miss is run for each check on its own checkout: two misses make two,
// removed after each run.
func TestEachMissHasACheckoutOfItsOwn(t *testing.T) {
	r := newRig(t)
	r.Run(t.Context(), r.request("true", config.Evidence{}))
	r.Run(t.Context(), r.request("true;", config.Evidence{}))
	if r.git.checkouts != 2 || r.git.removed != 2 {
		t.Errorf("checkouts %d removed %d", r.git.checkouts, r.git.removed)
	}
}

// A declared input that is not a file, a link or a folder cannot be keyed:
// the check runs in the working folder, unrecorded.
func TestAnInputThatCannotBeKeyedRunsInTheWorkingFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no named pipes to make here")
	}
	r := newRig(t)
	if out, err := exec.Command("mkfifo", filepath.Join(r.work, "pipe")).CombinedOutput(); err != nil {
		t.Skipf("mkfifo: %v: %s", err, out)
	}
	where := filepath.ToSlash(filepath.Join(t.TempDir(), "where"))
	o := r.Run(t.Context(), r.request("pwd > "+where, config.Evidence{Inputs: []string{"pipe"}}))
	ran, err := os.ReadFile(where)
	if err != nil || !o.OK || o.Info.Reason != ReasonUnkeyable || o.Info.Recorded || o.Info.Key != "" {
		t.Errorf("%+v, %v", o, err)
	}
	if strings.Contains(string(ran), "checkout") {
		t.Errorf("the check ran in a checkout: %q", ran)
	}
}

// The clock is the real one unless a test gives another.
func TestTheClockIsTheRealOneByDefault(t *testing.T) {
	var e Engine
	if got := e.now(); time.Since(got) > time.Minute || got.Location() != time.UTC {
		t.Errorf("now = %v", got)
	}
}

// What cannot be read of the repository or the store keeps the check from
// being keyed or recorded, never from running.
func TestTheRepositoryOrTheStoreThatCannotBeAsked(t *testing.T) {
	r := newRig(t)
	r.git.indexedErr = errors.New("git")
	o := r.Run(t.Context(), r.request("true", config.Evidence{Inputs: []string{"x"}}))
	if !o.OK || o.Info.Reason != ReasonUnkeyable {
		t.Errorf("an index that cannot be listed: %+v", o)
	}
	r = newRig(t)
	r.store.pathErr = errors.New("no repository")
	o = r.Run(t.Context(), r.request("true", config.Evidence{}))
	if !o.OK || o.Info.Hit || o.Info.Recorded || o.Info.Reason != ReasonStoreUnusable {
		t.Errorf("a store with no place: %+v", o)
	}
	if _, ok := r.lookup(t.Context(), "key"); ok {
		t.Error("a record in a store with no place")
	}
}

// A hit prints an output that does not end in a newline as a line.
func TestAHitEndsTheOutputsLastLine(t *testing.T) {
	r := newRig(t)
	r.Run(t.Context(), r.request("printf 'no newline'", config.Evidence{}))
	r.log.Reset()
	req := r.request("printf 'no newline'", config.Evidence{})
	req.Verbose = true
	r.Run(t.Context(), req)
	if !strings.HasSuffix(r.log.String(), "no newline\n") {
		t.Errorf("%q", r.log.String())
	}
	quiet := newRig(t)
	quiet.Run(t.Context(), quiet.request("true", config.Evidence{}))
	quiet.log.Reset()
	req = quiet.request("true", config.Evidence{})
	req.Verbose = true
	quiet.Run(t.Context(), req)
	if strings.Count(quiet.log.String(), "\n") != 1 {
		t.Errorf("a hit with no output prints %q", quiet.log.String())
	}
}

// A verbose miss lets the output through as it runs, and keeps it.
func TestAVerboseMissLetsTheOutputThrough(t *testing.T) {
	r := newRig(t)
	req := r.request("echo through", config.Evidence{})
	req.Verbose = true
	o := r.Run(t.Context(), req)
	if !strings.Contains(r.log.String(), "  running\nthrough\n") {
		t.Errorf("%q", r.log.String())
	}
	rec, ok := r.lookup(t.Context(), o.Info.Key)
	if !ok || rec.result.Output != "through\n" {
		t.Errorf("the record: %v %+v", ok, rec.result)
	}
}

// A declared input that cannot be copied into the checkout sends the check
// to the working folder, and the checkout is removed.
func TestAnInputThatCannotBeCopiedRunsInTheWorkingFolder(t *testing.T) {
	r := newRig(t)
	r.git.badCheckout = true
	if err := os.WriteFile(filepath.Join(r.work, "in.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := r.Run(t.Context(), r.request("true", config.Evidence{Inputs: []string{"in.txt"}}))
	if !o.OK || o.Info.Recorded || o.Info.Reason != ReasonSnapshotUnusable || r.git.removed != 1 {
		t.Errorf("%+v, removed %d", o, r.git.removed)
	}
}

// A record that cannot be written leaves no temporary folder.
func TestARecordThatCannotBeWrittenLeavesNothing(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a folder that refuses a file needs a user the mode applies to")
	}
	r := newRig(t)
	root := filepath.Join(r.store.root, Folder)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	cleanup := mkdirTemp
	t.Cleanup(func() { mkdirTemp = cleanup })
	mkdirTemp = func(dir, pattern string) (string, error) {
		made, err := os.MkdirTemp(dir, pattern)
		if err == nil {
			err = os.Chmod(made, 0o500)
		}
		return made, err
	}
	o := r.Run(t.Context(), r.request("true", config.Evidence{}))
	if !o.OK || o.Info.Recorded || o.Info.Reason != ReasonStoreUnusable {
		t.Errorf("%+v", o)
	}
	if left, _ := os.ReadDir(root); len(left) != 0 {
		t.Errorf("left %v", left)
	}
	mkdirTemp = func(string, string) (string, error) { return "", errors.New("no temporary folder") }
	o = r.Run(t.Context(), r.request("true;", config.Evidence{}))
	if o.Info.Recorded || o.Info.Reason != ReasonStoreUnusable {
		t.Errorf("%+v", o)
	}
}
