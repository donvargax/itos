package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A merge's own paths (bug 30): none for a clean merge, even of a file both
// sides changed; a path no parent has, a line no parent has, and a path git
// quotes (a byte above ASCII in it, since windows takes no quote in a file's
// name) for a merge that adds them; the same reading of the index for the
// merge being made, which MERGE_HEAD names until it is committed; and (bug
// 158) every path the second parent changed for a merge made with -s ours.
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
	if got, err := here.OwnPaths(clean); err != nil || len(got) != 0 {
		t.Fatalf("a clean merge's own paths = %q, %v", got, err)
	}
	if got, err := here.Parents(clean); err != nil || len(got) != 2 || got[0] != main {
		t.Fatalf("Parents = %q, %v, want main's first", got, err)
	}
	if got, err := here.Parents(gitIn(t, dir, "rev-list", "--max-parents=0", "HEAD")); err != nil || len(got) != 0 {
		t.Fatalf("a root commit's Parents = %q, %v", got, err)
	}

	gitIn(t, dir, "reset", "-q", "--hard", main)
	gitIn(t, dir, "merge", "-q", "--no-ff", "--no-commit", "topic")
	if got := here.MergeHeads(); len(got) != 1 || got[0] != gitIn(t, dir, "rev-parse", "topic") {
		t.Fatalf("MergeHeads = %q", got)
	}
	parents := append([]string{main}, here.MergeHeads()...)
	if got, err := here.StagedOwnPaths(parents); err != nil || len(got) != 0 {
		t.Fatalf("the clean merge being made's own paths = %q, %v", got, err)
	}
	write("new.js", "new\n")
	write("señal.md", "q\n")
	write("f", strings.Replace(strings.Replace(lines, "2\n", "two\n", 1), "9\n", "nine\n5½\n", 1))
	gitIn(t, dir, "add", "new.js", "señal.md", "f")
	want := []string{"f", "new.js", "señal.md"}
	if got, err := here.StagedOwnPaths(parents); err != nil || !slices.Equal(got, want) {
		t.Fatalf("the merge being made's own paths = %q, %v, want %q", got, err, want)
	}
	gitIn(t, dir, "commit", "-q", "-m", "Merge branch 'topic'")
	if got := here.MergeHeads(); len(got) != 0 {
		t.Fatalf("MergeHeads after the commit = %q", got)
	}
	if got, err := here.OwnPaths(gitIn(t, dir, "rev-parse", "HEAD")); err != nil || !slices.Equal(got, want) {
		t.Fatalf("the merge's own paths = %q, %v, want %q", got, err, want)
	}

	gitIn(t, dir, "reset", "-q", "--hard", main)
	gitIn(t, dir, "merge", "-q", "--no-ff", "-s", "ours", "-m", "Merge branch 'topic'", "topic")
	if got, err := here.OwnPaths(gitIn(t, dir, "rev-parse", "HEAD")); err != nil || !slices.Equal(got, []string{"f"}) {
		t.Fatalf("a merge that keeps its first parent's tree has own paths %q, %v, want f", got, err)
	}
	if got, err := here.OwnPaths("nope"); err == nil {
		t.Fatalf("the own paths of no commit are %q", got)
	}
}

// Bug 158: a merge's own paths are where its tree differs from git's merge
// of its two parents. None for a clean merge of neighbouring lines, which
// the dense combined diff shows as a hunk; a conflict resolved by taking a
// side (-X theirs, where merge-tree exits 1) is the merge's own; a merge of
// two parents with no merge base is read by --cc; and a merge-tree that
// fails is an error.
func TestOwnPathsAgainstGitsMerge(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lines := "1\n2\n3\n4\n5\n"
	write(lines)
	gitIn(t, dir, "add", "f")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	side := func(branch, from, to string) {
		t.Helper()
		gitIn(t, dir, "checkout", "-q", "-b", branch, "main")
		write(strings.Replace(lines, from, to, 1))
		gitIn(t, dir, "commit", "-q", "-am", branch)
	}
	side("two", "2\n", "two\n")
	side("four", "4\n", "four\n")
	side("other-two", "2\n", "TWO\n")

	gitIn(t, dir, "checkout", "-q", "four")
	gitIn(t, dir, "merge", "-q", "--no-ff", "-m", "Merge branch 'two'", "two")
	head := gitIn(t, dir, "rev-parse", "HEAD")
	if got, err := here.combinedPaths(head); err != nil || !slices.Equal(got, []string{"f"}) {
		t.Fatalf("the combined diff's paths of a merge of neighbouring lines = %q, %v, want f", got, err)
	}
	if got, err := here.OwnPaths(head); err != nil || len(got) != 0 {
		t.Fatalf("a clean merge of neighbouring lines has own paths %q, %v", got, err)
	}

	gitIn(t, dir, "checkout", "-q", "other-two")
	gitIn(t, dir, "merge", "-q", "--no-ff", "-X", "theirs", "-m", "Merge branch 'two'", "two")
	taken := gitIn(t, dir, "rev-parse", "HEAD")
	if got, err := here.OwnPaths(taken); err != nil || !slices.Equal(got, []string{"f"}) {
		t.Fatalf("a conflict resolved by taking a side has own paths %q, %v, want f", got, err)
	}

	gitIn(t, dir, "checkout", "-q", "--orphan", "unrelated")
	gitIn(t, dir, "rm", "-q", "-r", "--cached", "f")
	if err := os.WriteFile(filepath.Join(dir, "g"), []byte("g\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "g")
	gitIn(t, dir, "commit", "-q", "-m", "root")
	gitIn(t, dir, "checkout", "-q", "-f", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "--allow-unrelated-histories", "-m", "Merge branch 'unrelated'", "unrelated")
	if got, err := here.OwnPaths(gitIn(t, dir, "rev-parse", "HEAD")); err != nil || len(got) != 0 {
		t.Fatalf("a clean merge with no merge base has own paths %q, %v", got, err)
	}

	blob := gitIn(t, dir, "rev-parse", "main~1:f")
	object := filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	if got, err := here.OwnPaths(taken); err == nil {
		t.Fatalf("the own paths of a merge whose base lost a file are %q", got)
	}
}
