package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A merge's own paths (bug 30): none for a clean merge, even of a file both
// sides changed, and none for one that takes a side; a path no parent has,
// a line no parent has, and a quoted path for a merge that adds them; the
// same reading of the index for the merge being made, which MERGE_HEAD
// names until it is committed.
func TestOwnPaths(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lines := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	write("f", lines)
	gitIn(t, dir, "add", "f")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	gitIn(t, dir, "checkout", "-q", "-b", "topic")
	write("f", strings.Replace(lines, "2\n", "two\n", 1))
	gitIn(t, dir, "commit", "-q", "-am", "topic")
	gitIn(t, dir, "checkout", "-q", "main")
	write("f", strings.Replace(lines, "9\n", "nine\n", 1))
	gitIn(t, dir, "commit", "-q", "-am", "main")
	main := gitIn(t, dir, "rev-parse", "HEAD")

	gitIn(t, dir, "merge", "-q", "--no-ff", "-m", "Merge branch 'topic'", "topic")
	clean := gitIn(t, dir, "rev-parse", "HEAD")
	if got, err := OwnPaths(clean); err != nil || len(got) != 0 {
		t.Fatalf("a clean merge's own paths = %q, %v", got, err)
	}
	if got, err := Parents(clean); err != nil || len(got) != 2 || got[0] != main {
		t.Fatalf("Parents = %q, %v, want main's first", got, err)
	}
	if got, err := Parents(gitIn(t, dir, "rev-list", "--max-parents=0", "HEAD")); err != nil || len(got) != 0 {
		t.Fatalf("a root commit's Parents = %q, %v", got, err)
	}

	gitIn(t, dir, "reset", "-q", "--hard", main)
	gitIn(t, dir, "merge", "-q", "--no-ff", "--no-commit", "topic")
	if got := MergeHeads(); len(got) != 1 || got[0] != gitIn(t, dir, "rev-parse", "topic") {
		t.Fatalf("MergeHeads = %q", got)
	}
	parents := append([]string{main}, MergeHeads()...)
	if got, err := StagedOwnPaths(parents); err != nil || len(got) != 0 {
		t.Fatalf("the clean merge being made's own paths = %q, %v", got, err)
	}
	write("new.js", "new\n")
	write(`a "quoted" name`, "q\n")
	write("f", strings.Replace(strings.Replace(lines, "2\n", "two\n", 1), "9\n", "nine\n5½\n", 1))
	gitIn(t, dir, "add", "new.js", `a "quoted" name`, "f")
	want := []string{`a "quoted" name`, "f", "new.js"}
	if got, err := StagedOwnPaths(parents); err != nil || !slices.Equal(got, want) {
		t.Fatalf("the merge being made's own paths = %q, %v, want %q", got, err, want)
	}
	gitIn(t, dir, "commit", "-q", "-m", "Merge branch 'topic'")
	if got := MergeHeads(); len(got) != 0 {
		t.Fatalf("MergeHeads after the commit = %q", got)
	}
	if got, err := OwnPaths(gitIn(t, dir, "rev-parse", "HEAD")); err != nil || !slices.Equal(got, want) {
		t.Fatalf("the merge's own paths = %q, %v, want %q", got, err, want)
	}

	gitIn(t, dir, "reset", "-q", "--hard", main)
	gitIn(t, dir, "merge", "-q", "--no-ff", "-s", "ours", "-m", "Merge branch 'topic'", "topic")
	if got, err := OwnPaths(gitIn(t, dir, "rev-parse", "HEAD")); err != nil || len(got) != 0 {
		t.Fatalf("a merge that keeps one side's tree has own paths %q, %v", got, err)
	}
}
