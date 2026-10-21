package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
)

// folder stands the test in a new folder with no repository above it,
// holding tasks/phase-1.yaml "worktree\n", and staged "staged\n" when
// repository.
func folder(t *testing.T, repository bool) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	file := filepath.Join("tasks", "phase-1.yaml")
	if err := os.MkdirAll("tasks", 0o755); err != nil {
		t.Fatal(err)
	}
	if repository {
		if err := os.WriteFile(file, []byte("staged\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "tasks"}} {
			if out, err := exec.Command(git.Bin(), args...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %s", args, out)
			}
		}
	}
	if err := os.WriteFile(file, []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Under ReadingFrom the data is read from the tree given, through the
// adapter, and the working tree again after; Texts reads the tree's files
// at once.
func TestReadingFromReadsTheTreeGiven(t *testing.T) {
	folder(t, true)
	index, err := At("index")
	if err != nil {
		t.Fatal(err)
	}
	err = ReadingFrom(index, func() error {
		if tree := Current().Tree(); tree != "index" {
			t.Errorf("the current tree is %q", tree)
		}
		if !Has("tasks/phase-1.yaml") {
			t.Error("the index lacks tasks/phase-1.yaml")
		}
		if names, err := List("tasks"); err != nil || !reflect.DeepEqual(names, []string{"phase-1.yaml"}) {
			t.Errorf("List = %q, %v", names, err)
		}
		text, err := Read("tasks/phase-1.yaml")
		if text != "staged\n" {
			t.Errorf("Read = %q", text)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if Current() != Worktree || Current().Tree() != "worktree" {
		t.Errorf("the current source after ReadingFrom is %v", Current())
	}
	if text, err := Read("tasks/phase-1.yaml"); err != nil || text != "worktree\n" {
		t.Errorf("Read after = %q, %v", text, err)
	}
	if got := Texts("index", "tasks", func(string) bool { return true }); !reflect.DeepEqual(got, map[string]string{"tasks/phase-1.yaml": "staged\n"}) {
		t.Errorf("Texts = %q", got)
	}
}

// Outside a repository a tree git holds cannot be opened.
func TestAtOutsideARepository(t *testing.T) {
	folder(t, false)
	if _, err := At("index"); err == nil || err.Error() != "index cannot be read outside a git repository" {
		t.Errorf("At(index) = %v", err)
	}
}
