package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The working tree, the index and a commit each hold their own files, read
// through one Source, and Texts reads a tree's files at once; a tree that
// cannot be read has none.
func TestTrees(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "one\n")
	gitIn(t, dir, "add", "tasks")
	gitIn(t, dir, "commit", "-qm", "one")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "staged\n")
	write(t, filepath.Join(dir, "tasks/phase-2.yaml"), "staged two\n")
	gitIn(t, dir, "add", "tasks")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "worktree\n")
	t.Chdir(dir)

	for tree, want := range map[string][]string{
		"worktree": {"worktree\n", "phase-1.yaml phase-2.yaml"},
		"index":    {"staged\n", "phase-1.yaml phase-2.yaml"},
		"HEAD":     {"one\n", "phase-1.yaml"},
	} {
		s, err := At(tree)
		if err != nil {
			t.Fatal(err)
		}
		var text, names string
		err = ReadingFrom(s, func() error {
			got, err := Read("tasks/phase-1.yaml")
			text = got
			listed, _ := List("tasks")
			names = strings.Join(listed, " ")
			return err
		})
		if err != nil || text != want[0] || names != want[1] || Current() != Worktree {
			t.Errorf("%s: %q %q %v", tree, text, names, err)
		}
	}
	head, _ := At("HEAD")
	if _, err := head.Read("tasks/phase-2.yaml"); err == nil || err.Error() != "HEAD holds no tasks/phase-2.yaml" {
		t.Errorf("HEAD read %v", err)
	}
	if _, err := head.List("gone"); err == nil || err.Error() != "HEAD holds no folder gone" {
		t.Errorf("HEAD list %v", err)
	}
	all := func(string) bool { return true }
	if got := Texts("index", "tasks", all); !reflect.DeepEqual(got, map[string]string{
		"tasks/phase-1.yaml": "staged\n", "tasks/phase-2.yaml": "staged two\n",
	}) {
		t.Errorf("index texts %q", got)
	}
	if got := Texts("no-such-commit", "tasks", all); len(got) != 0 {
		t.Errorf("unreadable tree's texts %q", got)
	}
}

// A file in the git folder, which no tree git holds can have, is read where
// it is whatever the source: the stealth mode's config and data.
func TestGitFolderIsReadWhereItIs(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, ".git/itos/tasks/phase-1.yaml"), "aside\n")
	t.Chdir(dir)
	index, err := At("index")
	if err != nil {
		t.Fatal(err)
	}
	if !index.Has(".git/itos/tasks/phase-1.yaml") {
		t.Error("the index does not have the git folder's file")
	}
	if text, err := index.Read(".git/itos/tasks/phase-1.yaml"); err != nil || text != "aside\n" {
		t.Errorf("Read = %q, %v", text, err)
	}
	if names, err := index.List(".git/itos/tasks"); err != nil || !reflect.DeepEqual(names, []string{"phase-1.yaml"}) {
		t.Errorf("List = %v, %v", names, err)
	}
	if index.Has("tasks/phase-1.yaml") {
		t.Error("the index has a file it does not hold")
	}
}
