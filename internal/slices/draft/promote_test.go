package draft

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/donvargax/itos/v8/internal/draft"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

var (
	say     = domain.Draft{ID: "say", Message: "docs: say", Paths: []string{"notes.md"}}
	check   = domain.Draft{ID: "check", Command: []string{"work", "check"}}
	makeIt  = domain.Draft{ID: "make", Command: []string{"work", "add", "--kind", "slice"}, Minted: "slice-5"}
	edit    = domain.Draft{ID: "edit", Message: "docs: edit", Paths: []string{"notes.md", "new.md"}, Base: "c1"}
	patched = "the patch\n"
)

// changeOf keeps d's patch beside the list.
func changeOf(t *testing.T, file string, d domain.Draft) {
	t.Helper()
	if err := domain.WriteFile(domain.Patch(file, d.ID), []byte(patched)); err != nil {
		t.Fatal(err)
	}
}

// With no drafts promote says so, unless --quiet; --json says what it
// promoted, none.
func TestPromoteNothing(t *testing.T) {
	f, _, _ := repo(t)
	if code, stdout, _, err := draft(t, f, false, "promote"); code != 0 || err != nil || stdout != "No drafts to promote.\n" {
		t.Errorf("promote: exit %d, %v, stdout %q", code, err, stdout)
	}
	if code, stdout, _, err := draftIn(t, f, "", false, true, "promote"); code != 0 || err != nil || stdout != "" {
		t.Errorf("promote under --quiet: exit %d, %v, stdout %q", code, err, stdout)
	}
	code, stdout, _, err := draft(t, f, true, "promote")
	if code != 0 || err != nil || stdout != "{\n  \"schema\": 1,\n  \"ok\": true,\n  \"promoted\": []\n}\n" {
		t.Errorf("promote under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
}

// promote applies each draft in order from the top, saying each as it is
// promoted, and empties the list.
func TestPromoteEveryKind(t *testing.T) {
	f, dir, file := repo(t)
	keep(t, file, check, makeIt, say)
	changeOf(t, file, say)
	f.self, f.numstat, f.applied = "/itos", "1\t1\tnotes.md\x00", map[string]string{"notes.md": "New.\n"}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	code, stdout, stderr, err := draft(t, f, false, "promote")
	want := "check promoted: itos work check (no commit)\n" +
		"make promoted: itos work add --kind slice (no commit)\n" +
		"say promoted: committed n- docs: say\n"
	if code != 0 || err != nil || stdout != want || stderr != "ran\nran here\n" {
		t.Fatalf("promote: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if here, _ := os.Getwd(); here != dir {
		t.Errorf("promote ran in %s, not the top", here)
	}
	if strings.Join(f.ran[0], " ") != "/itos work check" || strings.Join(f.reserved, " ") != "slice-5" {
		t.Errorf("ran %q, reserving %q", f.ran, f.reserved)
	}
	if drafts := loaded(t, file); len(drafts) != 0 {
		t.Errorf("drafts left: %+v", drafts)
	}
	if _, err := os.Stat(domain.Patch(file, "say")); err == nil {
		t.Error("the patch is left")
	}
	keep(t, file, check)
	if code, stdout, _, err := draft(t, f, true, "promote"); code != 0 || err != nil || !strings.Contains(stdout, "\"promoted\": [\n    \"check\"\n  ]") {
		t.Errorf("promote under --json: exit %d, %v, stdout %q", code, err, stdout)
	}
	keep(t, file, check)
	if code, stdout, _, err := draftIn(t, f, "", false, true, "promote"); code != 0 || err != nil || stdout != "" {
		t.Errorf("promote under --quiet: exit %d, %v, stdout %q", code, err, stdout)
	}
	keep(t, file, check)
	f.Located = map[string]vcsport.Location{"": {Common: f.Located[""].Common}}
	t.Chdir(sub)
	if code, _, _, err := draft(t, f, false, "promote"); code != 0 || err != nil {
		t.Errorf("promote with no top: exit %d, %v", code, err)
	}
	if here, _ := os.Getwd(); here != sub {
		t.Errorf("promote with no top moved to %s", here)
	}
}

// promote refuses while the checkout is busy, nothing applied.
func TestPromoteRefusesABusyCheckout(t *testing.T) {
	f, _, file := repo(t)
	keep(t, file, check)
	f.Work = map[string]string{"notes.md": "New.\n"}
	code, _, stderr, err := draft(t, f, false, "promote")
	if code != refused || err != nil || stderr != "notes.md has a change no commit holds, so no draft is promoted over it\n"+
		"  fix: promote once that work is committed or put back\n" {
		t.Errorf("a change: exit %d, %v, stderr %q", code, err, stderr)
	}
	f.Work, f.Index = map[string]string{"notes.md": "New.\n", "b.md": "B.\n"}, map[string]string{"notes.md": "Old.\n", "b.md": "B?\n"}
	f.InRebase, f.MergeHead = true, []string{"c1"}
	code, stdout, _, err := draft(t, f, true, "promote")
	for _, want := range []string{`"ok": false`, `"promoted": []`, "b.md and notes.md have a change", "a rebase and a merge is in progress"} {
		if code != refused || err != nil || !strings.Contains(stdout, want) {
			t.Errorf("busy under --json: exit %d, %v, stdout %q, not %q", code, err, stdout, want)
		}
	}
	f.Work, f.Index, f.MergeHead = nil, nil, nil
	if code, _, stderr, _ := draft(t, f, false, "promote"); code != refused || !strings.HasPrefix(stderr, "a rebase is in progress") {
		t.Errorf("a rebase: exit %d, stderr %q", code, stderr)
	}
	if len(f.ran) > 0 || len(loaded(t, file)) != 1 {
		t.Error("a busy checkout ran a draft")
	}
}

// The first draft that cannot be promoted stops promote, exit 1, it and
// the ones after it kept; the ones before it stay promoted.
func TestPromoteStops(t *testing.T) {
	for name, c := range map[string]struct {
		drafts []domain.Draft
		set    func(*fake)
		want   string
	}{
		"a command that fails": {drafts: []domain.Draft{check, say}, set: func(f *fake) { f.exit = 3 },
			want: "check cannot be promoted: itos work check exited 3; it and the 1 after it are kept, the tree left as HEAD has it\n" +
				"  fix: redo it (itos draft drop check, then itos draft add) or fix what stopped it, then itos draft promote again\n"},
		"a minting command that fails": {drafts: []domain.Draft{makeIt}, set: func(f *fake) { f.runCode = 1 },
			want: "make cannot be promoted: itos work add --kind slice exited 1; it is kept"},
		"a change that no longer applies": {drafts: []domain.Draft{say}, set: func(f *fake) { f.applyErr = errBroken },
			want: "say cannot be promoted: the change no longer applies to the tree (broken); it is kept"},
		"a commit refused": {drafts: []domain.Draft{say}, set: func(f *fake) {
			f.commitErr, f.numstat = &vcsport.Failed{Code: 1}, "1\t1\tnotes.md\x00"
			f.applied = map[string]string{"notes.md": "New.\n"}
		}, want: "say cannot be promoted: its commit failed (git exited 1); it is kept"},
		"an edit draft with no change": {drafts: []domain.Draft{edit}, set: func(f *fake) { f.index.diff = "" },
			want: "fix: redo it from HEAD (keep what you need of its copies in "},
	} {
		t.Run(name, func(t *testing.T) {
			f, _, file := repo(t)
			keep(t, file, c.drafts...)
			changeOf(t, file, say)
			c.set(f)
			code, _, stderr, err := draft(t, f, false, "promote")
			if code != refused || err != nil || !strings.Contains(stderr, c.want) {
				t.Errorf("exit %d, %v, stderr %q", code, err, stderr)
			}
			if len(loaded(t, file)) != len(c.drafts) {
				t.Error("a draft not promoted left the list")
			}
			if f.Work["notes.md"] != "" && f.Work["notes.md"] != "Old.\n" {
				t.Errorf("notes.md is %q, not as HEAD has it", f.Work["notes.md"])
			}
		})
	}
}

// What stops promote from even trying is an error.
func TestPromoteFails(t *testing.T) {
	for name, c := range map[string]struct {
		drafts []domain.Draft
		set    func(*fake, string)
		want   string
	}{
		"no top to move to": {drafts: []domain.Draft{check}, set: func(f *fake, _ string) {
			f.Located = map[string]vcsport.Location{"": {Top: filepath.Join(f.Located[""].Top, "gone"), Common: f.Located[""].Common}}
		}, want: "gone"},
		"no hooks": {drafts: []domain.Draft{check, say}, set: func(f *fake, _ string) { f.hooksErr = errBroken }, want: "broken"},
		"no binary": {drafts: []domain.Draft{check}, set: func(f *fake, _ string) { f.selfErr = errBroken },
			want: "this itos cannot find its own binary to run check: broken"},
		"no run":     {drafts: []domain.Draft{check}, set: func(f *fake, _ string) { f.execErr = errBroken }, want: "broken"},
		"no patch":   {drafts: []domain.Draft{say}, set: func(_ *fake, file string) { os.Remove(domain.Patch(file, "say")) }, want: "say's patch cannot be read"},
		"no numstat": {drafts: []domain.Draft{say}, set: func(f *fake, _ string) { f.numstatErr = errBroken }, want: "say's patch cannot be read: broken"},
		"no edit":    {drafts: []domain.Draft{edit}, set: func(f *fake, _ string) { f.entriesErr = errBroken }, want: "git ls-tree c1: broken"},
		"no commit": {drafts: []domain.Draft{say}, set: func(f *fake, _ string) { f.commitErr, f.numstat = errBroken, "1\t0\tnotes.md\x00" },
			want: "broken"},
		"no commit, no put back": {drafts: []domain.Draft{say}, set: func(f *fake, _ string) {
			f.commitErr, f.restoreErr, f.numstat = &vcsport.Failed{Code: 1}, errBroken, "1\t0\tnotes.md\x00"
		}, want: "the commit of say failed, and its files cannot be put back: git checkout HEAD -- notes.md: broken"},
		"no list": {drafts: []domain.Draft{check}, set: func(f *fake, file string) { f.then = func() { block(file) } },
			want: "check is promoted, but the list cannot be saved without it, so a promote would apply it again"},
		"no removal": {drafts: []domain.Draft{check}, set: func(_ *fake, file string) { folderAt(t, domain.Patch(file, "check")) },
			want: "check.patch"},
	} {
		t.Run(name, func(t *testing.T) {
			f, _, file := repo(t)
			keep(t, file, c.drafts...)
			changeOf(t, file, say)
			c.set(f, file)
			if code, _, _, err := draft(t, f, false, "promote"); code != 0 || err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("exit %d, %v, want an error naming %q", code, err, c.want)
			}
		})
	}
}

// editPatch makes an edit draft's patch from its copies against its base:
// a copy removed takes its file out, an empty copy of a file the base
// lacks adds nothing, each copy's mode the base's or a file's. A copy it
// cannot read (a NUL byte in its name) and a patch file it cannot make or
// write are errors, the file it could not write removed.
func TestEditPatch(t *testing.T) {
	f, _, file := repo(t)
	h := NewHandler(f)
	ctx := context.Background()
	d := domain.Draft{ID: "edit", Message: "docs: edit", Paths: []string{"notes.md", "new.md", "empty.md", "gone.md"}, Base: "c1"}
	f.entries["gone.md"] = TreeEntry{Mode: "100644", Kind: "blob", Object: "o2"}
	for path, text := range map[string]string{"notes.md": "New.\n", "new.md": "A spec.\n", "empty.md": ""} {
		if err := domain.WriteFile(domain.Copy(file, d.ID, path), []byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	made, why, err := h.editPatch(ctx, file, d)
	if err != nil || why != "" {
		t.Fatalf("editPatch: %q, %v", why, err)
	}
	if text, err := os.ReadFile(made); err != nil || string(text) != patched || filepath.Dir(made) != filepath.Dir(file) {
		t.Errorf("the patch %s holds %q (%v)", made, text, err)
	}
	if strings.Join(f.index.set, ", ") != "100755 object-of-notes.md notes.md, 100644 object-of-new.md new.md" ||
		strings.Join(f.index.removed, ", ") != "gone.md" {
		t.Errorf("set %q, removed %q", f.index.set, f.index.removed)
	}
	for name, c := range map[string]struct {
		set func(*index)
		why string
		err string
	}{
		"its base gone": {set: func(x *index) { x.readErr = errBroken }, why: "the commit its copies were taken from, c1, is gone (broken)"},
		"no change":     {set: func(x *index) { x.diff = "" }, why: "no copy differs from what it was taken from (" + domain.Copies(file, d.ID) + ")"},
		"no removal":    {set: func(x *index) { x.removeErr = errBroken }, err: "git update-index --force-remove gone.md: broken"},
		"no blob":       {set: func(x *index) { x.storeErr = errBroken }, err: "git hash-object edit's copy of notes.md: broken"},
		"no entry":      {set: func(x *index) { x.setErr = errBroken }, err: "git update-index --cacheinfo notes.md: broken"},
		"no diff":       {set: func(x *index) { x.diffErr = errBroken }, err: "git diff --cached --binary: broken"},
	} {
		f.index = &index{diff: patched}
		c.set(f.index)
		made, why, err := h.editPatch(ctx, file, d)
		if made != "" || why != c.why || (c.err == "") != (err == nil) || err != nil && err.Error() != c.err {
			t.Errorf("%s: %q, %q, %v", name, made, why, err)
		}
	}
	if _, _, err := h.editPatch(ctx, filepath.Join(file, "gone", "drafts.yaml"), d); err == nil {
		t.Error("editPatch with no folder for its index gave no error")
	}
	f.index = &index{diff: patched}
	unread := domain.Draft{ID: "edit", Message: "docs: edit", Paths: []string{"bad\x00.md"}, Base: "c1"}
	f.entries["bad\x00.md"] = TreeEntry{Mode: "100644", Kind: "blob", Object: "o3"}
	if made, why, err := h.editPatch(ctx, file, unread); made != "" || why != "" || err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a copy that cannot be read: %q, %q, %v", made, why, err)
	}
	f.tempErr = errBroken
	if made, why, err := h.editPatch(ctx, file, d); made != "" || why != "" || !errors.Is(err, errBroken) {
		t.Errorf("no patch file made: %q, %q, %v", made, why, err)
	}
	f.tempErr, f.tempClosed = nil, true
	if made, why, err := h.editPatch(ctx, file, d); made != "" || why != "" || !errors.Is(err, os.ErrClosed) {
		t.Errorf("a patch file not written: %q, %q, %v", made, why, err)
	}
	if _, err := os.Stat(f.temp); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the patch file not written is left: %v", err)
	}
}

// promote's helpers: the files a patch touches, git's words of a failure,
// and a path as typed in a subfolder.
func TestHelpers(t *testing.T) {
	if got := patchNames("1\t0\ta.md\x00-\t-\tbin\x00\x00bad\x00"); strings.Join(got, ",") != "a.md,bin" {
		t.Errorf("patchNames = %q", got)
	}
	if wordsOf(errBroken) != "broken" || wordsOf(&vcsport.Failed{Words: " no \n"}) != "no" {
		t.Error("wordsOf")
	}
	for _, c := range [][3]string{{"sub", "a.md", filepath.Join("sub", "a.md")}, {"", "a.md", "a.md"}, {"sub", "-", "-"}, {"sub", "", ""}} {
		if got := typed(c[0], c[1]); got != c[2] {
			t.Errorf("typed(%q, %q) = %q", c[0], c[1], got)
		}
	}
	if and(nil) != "" || and([]string{"a", "b", "c"}) != "a, b and c" || oneLine(" a\n b ") != "a b" {
		t.Error("and or oneLine")
	}
}

// An edit draft's patch, made from its copies, is applied and committed
// as a change's, and removed after.
func TestPromoteAnEditDraft(t *testing.T) {
	f, _, file := repo(t)
	keep(t, file, edit)
	f.numstat, f.applied = "1\t1\tnotes.md\x00", map[string]string{"notes.md": "New.\n"}
	if code, stdout, stderr, err := draft(t, f, false, "promote"); code != 0 || err != nil || stdout != "edit promoted: committed n- docs: edit\n" {
		t.Fatalf("promote: exit %d, %v, stdout %q, stderr %q", code, err, stdout, stderr)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(file), ".edit.*")); len(left) > 0 {
		t.Errorf("the patch made is left: %q", left)
	}
}

// topPaths moves itos to the top, an error when it cannot.
func TestTopPathsCannotMove(t *testing.T) {
	f, dir, _ := repo(t)
	top := filepath.Join(dir, "gone")
	f.Located = map[string]vcsport.Location{"": {Top: top}}
	if _, err := NewHandler(f).topPaths(context.Background(), "draft add", []string{filepath.Join(top, "a.md")}, ""); err == nil {
		t.Error("topPaths gave no error for a top it cannot move to")
	}
}

// topPaths cannot read the folder itos runs in: an error.
func TestTopPathsCannotReadWhereItRuns(t *testing.T) {
	f, _, _ := repo(t)
	f.getwdErr = errBroken
	if _, err := NewHandler(f).topPaths(context.Background(), "draft add", []string{"notes.md"}, ""); !errors.Is(err, errBroken) {
		t.Errorf("topPaths gave %v, not Getwd's error", err)
	}
}
