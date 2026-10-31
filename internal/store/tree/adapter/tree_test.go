package adapter

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/store/tree/port/porttest"
	vcs "github.com/donvargax/itos/v8/internal/vcs/adapter"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(git.Bin(runtime.GOOS), append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
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

// notHooked clears what a hook hands the tests it runs: the repository, the index and the
// settings that every git command a test makes, and the adapter's own, would use in place of the
// test's folder.
func notHooked(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

// laidOut stands the test in a new folder laid out as
// porttest.TreesContract asks, a repository when repository, with no git
// setting from outside and no repository above it.
func laidOut(t *testing.T, repository bool) port.Trees {
	t.Helper()
	notHooked(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(t.TempDir())
	if !repository {
		write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "worktree\n")
		return Trees{VCS: vcs.Git{Dir: dir}}
	}
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "tasks/done/phase-0.yaml"), "zero\n")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "one\n")
	gitIn(t, dir, "add", "tasks")
	gitIn(t, dir, "commit", "-qm", "one")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "staged\n")
	write(t, filepath.Join(dir, "tasks/phase-2.yaml"), "staged two\n")
	gitIn(t, dir, "add", "tasks")
	write(t, filepath.Join(dir, "tasks/phase-1.yaml"), "worktree\n")
	write(t, filepath.Join(dir, ".git/itos/tasks/phase-1.yaml"), "aside\n")
	return Trees{VCS: vcs.Git{Dir: dir}}
}

// The adapter reads the disk and git as the fake reads its trees.
func TestTreesKeepTheContract(t *testing.T) {
	porttest.TreesContract(t, laidOut)
}

// Texts and List read a tree's paths as they are named (bug 31): a path git
// would quote, a letter outside ASCII and, where the platform's git takes
// them, a line break and a carriage return, which git cat-file --batch cannot
// be handed, are read from the index and from a commit. The names are staged
// in the index alone, so no file of the work tree needs them.
func TestTextsOfNamesGitQuotes(t *testing.T) {
	notHooked(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "blob"), "text\n")
	out, err := exec.Command(git.Bin(runtime.GOOS), "-C", dir, "hash-object", "-w", "blob").Output()
	if err != nil {
		t.Fatal(err)
	}
	blob := strings.TrimSpace(string(out))
	names := []string{"features/café.feature", "features/plain.feature"}
	if runtime.GOOS != "windows" {
		names = append(names, "features/line\nbreak.feature", "features/cr\r.feature")
	}
	want := map[string]string{}
	for _, n := range names {
		gitIn(t, dir, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+n)
		want[n] = "text\n"
	}
	all := func(string) bool { return true }
	trees := Trees{VCS: vcs.Git{Dir: dir}}
	if got := trees.Texts(t.Context(), "index", "features", all); !reflect.DeepEqual(got, want) {
		t.Errorf("index texts %q, want %q", got, want)
	}
	gitIn(t, dir, "commit", "-qm", "names")
	if got := trees.Texts(t.Context(), "HEAD", "features", all); !reflect.DeepEqual(got, want) {
		t.Errorf("HEAD texts %q, want %q", got, want)
	}
	head, _ := trees.At(t.Context(), "HEAD")
	listed, err := head.List(t.Context(), "features")
	if err != nil || len(listed) != len(names) || !slices.Contains(listed, "café.feature") {
		t.Errorf("HEAD list %q, %v", listed, err)
	}
}

// A project in a subfolder reads its own files of the index and of a
// commit by the names git lists from that folder, where git would read
// those names from the top.
func TestTextsFromASubfolder(t *testing.T) {
	notHooked(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, "features", "one.feature"), "the top's\n")
	write(t, filepath.Join(dir, "app", "features", "one.feature"), "the app's\n")
	gitIn(t, dir, "add", ".")
	all := func(string) bool { return true }
	trees := Trees{VCS: vcs.Git{Dir: filepath.Join(dir, "app")}}
	want := map[string]string{"features/one.feature": "the app's\n"}
	if got := trees.Texts(t.Context(), "index", "features", all); !reflect.DeepEqual(got, want) {
		t.Errorf("index texts %q, want %q", got, want)
	}
	gitIn(t, dir, "commit", "-qm", "two projects")
	if got := trees.Texts(t.Context(), "HEAD", "features", all); !reflect.DeepEqual(got, want) {
		t.Errorf("HEAD texts %q, want %q", got, want)
	}
}

// A file in the git folder, which no tree git holds can have, is read where
// it is whatever the source: the stealth mode's config and data, from the
// folder itos runs in, wherever the process stands.
func TestGitFolderIsReadWhereItIs(t *testing.T) {
	notHooked(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	write(t, filepath.Join(dir, ".git/itos/tasks/phase-1.yaml"), "aside\n")
	t.Chdir(t.TempDir())
	index, err := Trees{VCS: vcs.Git{Dir: dir}}.At(t.Context(), "index")
	if err != nil {
		t.Fatal(err)
	}
	if !index.Has(t.Context(), ".git/itos/tasks/phase-1.yaml") {
		t.Error("the index does not have the git folder's file")
	}
	if text, err := index.Read(t.Context(), ".git/itos/tasks/phase-1.yaml"); err != nil || text != "aside\n" {
		t.Errorf("Read = %q, %v", text, err)
	}
	if names, err := index.List(t.Context(), ".git/itos/tasks"); err != nil || !reflect.DeepEqual(names, []string{"phase-1.yaml"}) {
		t.Errorf("List = %v, %v", names, err)
	}
	if index.Has(t.Context(), "tasks/phase-1.yaml") {
		t.Error("the index has a file it does not hold")
	}
}
