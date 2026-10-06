package git

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// Paths reads a path list NUL-separated (bug 31): a path git would C-quote,
// a letter outside ASCII and, where the platform's git takes them, a quote,
// a backslash, a tab and a line break, comes back as it is named from the
// index, a commit, a range and a tree; empty entries are dropped, and a
// command git refuses is an error. The names are staged in the index alone,
// so no file of the work tree needs them until the reset that writes them
// before Changed is read. Changed puts a rename's old path
// first, as git status --short does.
func TestPaths(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	commitFile(t, dir, "a")

	names := []string{"café.js", "señal/é.md", "with space.md"}
	if runtime.GOOS != "windows" {
		names = append(names, `quote".md`, `back\slash.md`, "tab\there.md", "line\nbreak.md")
	}
	if err := os.WriteFile(filepath.Join(dir, "blob"), []byte("text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blob := gitIn(t, dir, "hash-object", "-w", "blob")
	for _, n := range names {
		gitIn(t, dir, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+n)
	}
	want := slices.Clone(names)
	slices.Sort(want)
	sorted := func(paths []string, err error) []string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(paths)
		return paths
	}
	if got := sorted(Paths("diff", "--cached", "--name-only")); !slices.Equal(got, want) {
		t.Fatalf("staged paths = %q, want %q", got, want)
	}
	if got := sorted(Paths("ls-files", "--cached")); !slices.Equal(got, append([]string{"a"}, want...)) {
		t.Fatalf("index paths = %q, want a and %q", got, want)
	}
	gitIn(t, dir, "commit", "-q", "-m", "names")
	if got := sorted(CommitPaths("HEAD")); !slices.Equal(got, want) {
		t.Fatalf("CommitPaths = %q, want %q", got, want)
	}
	if got := sorted(Paths("log", "--format=", "--name-only", "HEAD~1..HEAD")); !slices.Equal(got, want) {
		t.Fatalf("the range's paths = %q, want %q", got, want)
	}
	if got := sorted(Paths("ls-tree", "-r", "--name-only", "HEAD", "--", "señal")); !slices.Equal(got, []string{"señal/é.md"}) {
		t.Fatalf("the tree's paths under señal = %q", got)
	}
	if got, err := Paths("diff", "--name-only", "HEAD", "HEAD"); err != nil || got != nil {
		t.Fatalf("no change lists %q, %v", got, err)
	}
	if _, err := Paths("diff", "--name-only", "no-such-commit"); err == nil {
		t.Fatal("a command git refuses lists paths")
	}
	if got, err := Paths(); err != nil || got != nil {
		t.Fatalf("no command lists %q, %v", got, err)
	}

	gitIn(t, dir, "reset", "-q", "--hard")
	gitIn(t, dir, "mv", "a", "b é")
	if got := Changed(); !slices.Equal(got, []string{"R  a -> b é"}) {
		t.Fatalf("renamed, Changed = %q, want [\"R  a -> b é\"]", got)
	}
}
