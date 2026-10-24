package evidence

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/donvargax/itos/v8/internal/config"
)

// put writes a file of the rig's working folder, its folders made.
func (r *rig) put(t *testing.T, name, text string, mode os.FileMode) {
	t.Helper()
	full := filepath.Join(r.work, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}

// inputsOf are the declared inputs as the key binds them, path:state.
func (r *rig) inputsOf(t *testing.T, inputs ...string) ([]string, bound) {
	t.Helper()
	b, err := r.bind(t.Context(), r.request("true", config.Evidence{Inputs: inputs}), "tree")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, in := range b.Inputs {
		got = append(got, in.Path+":"+in.State)
	}
	return got, b
}

// A folder is every file under it git does not track, in order, a .git left
// out; a tracked file is the tree's; a missing path is absent.
func TestAFolderIsEveryUntrackedFileUnderIt(t *testing.T) {
	r := newRig(t)
	r.put(t, "gen/sub/a.txt", "a", 0o755)
	r.put(t, "gen/kept.txt", "kept", 0o644)
	r.put(t, "gen/.git/config", "nested checkout", 0o644)
	r.put(t, "gen/sub/.git", "gitdir: x", 0o644)
	r.git.indexed = []string{"gen/kept.txt"}
	got, b := r.inputsOf(t, "gen", "missing", "gen/kept.txt", "gen/kept.txt/under")
	if want := []string{"gen:dir", "gen/sub/a.txt:file", "gen/kept.txt/under:absent", "missing:absent"}; !slices.Equal(got, want) {
		t.Errorf("inputs %v, want %v", got, want)
	}
	if runtime.GOOS != "windows" && !b.Inputs[1].Exec {
		t.Errorf("an executable file is not hashed as one: %+v", b.Inputs[1])
	}
}

// A link is hashed by its target and copied as a link.
func TestALinkIsHashedByItsTarget(t *testing.T) {
	r := newRig(t)
	if err := os.Symlink("a.txt", filepath.Join(r.work, "link")); err != nil {
		t.Skipf("no links here: %v", err)
	}
	got, b := r.inputsOf(t, "link")
	if !slices.Equal(got, []string{"link:link"}) || b.copies[0].target != "a.txt" {
		t.Fatalf("%v %+v", got, b.copies)
	}
	first := b.Key()
	if err := os.Remove(filepath.Join(r.work, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b.txt", filepath.Join(r.work, "link")); err != nil {
		t.Fatal(err)
	}
	if _, again := r.inputsOf(t, "link"); again.Key() == first {
		t.Error("another target, the same key")
	}
	dest := t.TempDir()
	if err := copyInputs(dest, r.work, b.copies); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(dest, "link")); err != nil || target != "a.txt" {
		t.Errorf("the copied link: %q, %v", target, err)
	}
	if err := copyInputs(dest, r.work, b.copies); err == nil {
		t.Error("made a link over a link")
	}
	deep := []copying{{path: "x/y/link", state: "link", target: "t"}}
	if err := copyInputs(t.TempDir(), r.work, deep); err != nil {
		t.Errorf("a link below a folder: %v", err)
	}
}

// What git does not track is copied into the checkout as it is: folders
// made, files with their executable bit.
func TestInputsAreCopiedIntoTheCheckout(t *testing.T) {
	r := newRig(t)
	r.put(t, "gen/sub/run.sh", "echo", 0o755)
	r.put(t, "gen/plain.txt", "plain", 0o644)
	r.put(t, "empty/.keep", "", 0o644)
	_, b := r.inputsOf(t, "gen", "empty")
	dest := t.TempDir()
	if err := copyInputs(dest, r.work, b.copies); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"gen/sub/run.sh": "echo", "gen/plain.txt": "plain"} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	if runtime.GOOS != "windows" {
		run, _ := os.Stat(filepath.Join(dest, "gen", "sub", "run.sh"))
		plain, _ := os.Stat(filepath.Join(dest, "gen", "plain.txt"))
		if run.Mode().Perm()&0o100 == 0 || plain.Mode().Perm()&0o111 != 0 {
			t.Errorf("modes %v %v", run.Mode(), plain.Mode())
		}
	}
}

// A copy that cannot be made says so: a file that is not there, a folder or
// a file in the way.
func TestACopyThatCannotBeMade(t *testing.T) {
	work := t.TempDir()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	inTheWay := t.TempDir()
	if err := os.Mkdir(filepath.Join(inTheWay, "f"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		dest string
		list []copying
	}{
		"a missing file":        {t.TempDir(), []copying{{path: "missing", state: "file"}}},
		"a file below a file":   {blocker, []copying{{path: "f", state: "file"}}},
		"a folder in the way":   {inTheWay, []copying{{path: "f", state: "file"}}},
		"a folder below a file": {blocker, []copying{{path: "d", state: "dir"}}},
		"a link below a file":   {blocker, []copying{{path: "d/l", state: "link", target: "t"}}},
	} {
		if err := copyInputs(c.dest, work, c.list); err == nil {
			t.Errorf("%s was copied", name)
		}
	}
}

// What cannot be read cannot be keyed: a named pipe is no file, a link whose
// target cannot be read, a file or a folder that is not readable.
func TestWhatCannotBeReadCannotBeKeyed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no named pipes and no modes to make here")
	}
	r := newRig(t)
	if out, err := exec.Command("mkfifo", filepath.Join(r.work, "pipe")).CombinedOutput(); err != nil {
		t.Skipf("mkfifo: %v: %s", err, out)
	}
	r.put(t, "locked/inside/file", "x", 0o644)
	r.put(t, "unreadable", "x", 0o000)
	r.put(t, "locked/file", "x", 0o644)
	if err := os.Chmod(filepath.Join(r.work, "locked"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(r.work, "locked"), 0o755) })
	denied := os.Geteuid() != 0
	for input, refused := range map[string]bool{"pipe": true, "unreadable": denied, "locked": denied, "locked/file": denied} {
		if _, err := r.bind(t.Context(), r.request("true", config.Evidence{Inputs: []string{input}}), "tree"); (err != nil) != refused {
			t.Errorf("%s: %v", input, err)
		}
	}
	// A link that is no link when it is read: readFile's Readlink fails.
	info, err := os.Lstat(filepath.Join(r.work, "unreadable"))
	if err != nil {
		t.Fatal(err)
	}
	var b bound
	if err := readFile(&b, "x", filepath.Join(r.work, "unreadable"), linkInfo{info}); err == nil {
		t.Error("a link was read from a file")
	}
	if _, err := hashFile(filepath.Join(r.work, "no-such-file")); err == nil {
		t.Error("hashed a file that is not there")
	}
	if _, err := hashFile(r.work); err == nil {
		t.Error("hashed a folder")
	}
}

// linkInfo is a file said to be a link.
type linkInfo struct{ fs.FileInfo }

func (linkInfo) Mode() fs.FileMode { return fs.ModeSymlink }

// A tool the PATH finds but whose content cannot be read cannot be keyed.
func TestAToolThatCannotBeReadCannotBeKeyed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a mode that refuses a read needs a user it applies to")
	}
	r := newRig(t)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(bin, "tool"), []byte("x"), 0o111); err != nil {
		t.Fatal(err)
	}
	if _, err := r.bind(t.Context(), r.request("true", config.Evidence{Tools: []string{"tool"}}), "tree"); err == nil {
		t.Error("a tool that cannot be read was keyed")
	}
}
