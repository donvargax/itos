package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"syscall"
	"testing"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// A failed file operation is said in Node's words for the errors Node
// names, and is the error itself otherwise.
func TestNodeError(t *testing.T) {
	for errno, want := range map[syscall.Errno]string{
		syscall.ENOENT:  "ENOENT: no such file or directory, open 'a.yaml'",
		syscall.ENOTDIR: "ENOTDIR: not a directory, open 'a.yaml'",
		syscall.EACCES:  "EACCES: permission denied, open 'a.yaml'",
		syscall.EISDIR:  "EISDIR: illegal operation on a directory, read",
	} {
		if got := nodeError(&os.PathError{Op: "open", Path: "a.yaml", Err: errno}, "open", "a.yaml"); got.Error() != want {
			t.Errorf("%v: %q, want %q", errno, got, want)
		}
	}
	for _, err := range []error{&os.PathError{Op: "open", Path: "a.yaml", Err: syscall.EINVAL}, errors.New("no errno")} {
		if got := nodeError(err, "open", "a.yaml"); got != err {
			t.Errorf("%v: %v", err, got)
		}
	}
}

// failingTexts is the VCS fake whose Texts fails.
type failingTexts struct{ *vcsfake.VCS }

func (failingTexts) Texts(context.Context, string, []string) (map[string]string, error) {
	return nil, errors.New("cat-file failed")
}

// A tree the VCS holds, over the VCS fake: a tree it cannot list is the
// VCS's error, and texts it cannot read are none; a path the top cannot be
// made relative to is read as the top.
func TestTreesOverTheVCSFake(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	v := &vcsfake.VCS{Root: dir, Index: map[string]string{"a.md": "a\n", "sub/b.md": "b\n"},
		Located: map[string]vcsport.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}}}
	nope, err := Trees{VCS: v}.At(t.Context(), "nope")
	if err != nil {
		t.Fatal(err)
	}
	if listed, err := nope.List(t.Context(), "sub"); err == nil {
		t.Errorf("a tree that names nothing lists %q", listed)
	}
	all := func(string) bool { return true }
	if got := (Trees{VCS: failingTexts{v}}).Texts(t.Context(), "index", "", all); len(got) != 0 {
		t.Errorf("texts that cannot be read: %q", got)
	}
	v.Located[""] = vcsport.Location{Top: "relative", GitDir: ".git", Common: ".git"}
	index, err := Trees{VCS: v}.At(t.Context(), "index")
	if err != nil {
		t.Fatal(err)
	}
	if listed, err := index.List(t.Context(), "sub"); err != nil || !reflect.DeepEqual(listed, []string{"a.md"}) {
		t.Errorf("a top no path is relative to lists %q, %v", listed, err)
	}
}

// A working folder that is gone is At's error.
func TestTreesAtAGoneWorkingFolder(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a removed working folder is getcwd's error on linux; other systems differ or refuse to remove it")
	}
	dir := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	v := &vcsfake.VCS{Root: dir, Located: map[string]vcsport.Location{"": {Top: dir, GitDir: ".git", Common: ".git"}}}
	if _, err := (Trees{VCS: v}).At(t.Context(), "index"); err == nil {
		t.Error("a gone working folder opens the index")
	}
}
