package adapter

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// committed stands the test at the top of a new repository whose one commit
// holds the files, each its name a line.
func committed(t *testing.T, files ...string) string {
	t.Helper()
	dir := filepath.Join(alone(t), "repo")
	gitIn(t, "", "", "init", "-q", "-b", "main", dir)
	for _, f := range files {
		write(t, dir, f, f)
		gitIn(t, dir, "", "add", "--", f)
	}
	gitIn(t, dir, "", "commit", "-q", "-m", "start")
	t.Chdir(dir)
	return dir
}

// failing makes the git the adapter runs fail the subcommand, and run the
// real git for the rest.
func failing(t *testing.T, subcommand string) {
	t.Helper()
	failingWith(t, subcommand, "injected", 1)
}

// failingWith is failing, the subcommand saying the words and exiting with the
// code.
func failingWith(t *testing.T, subcommand, words string, code int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the failing git is a shell script")
	}
	script := filepath.Join(t.TempDir(), "git")
	text := "#!/bin/sh\nif [ \"$1\" = " + subcommand + " ]; then echo '" + words + "' >&2; exit " + strconv.Itoa(code) + "; fi\nexec \"" +
		git.Bin(runtime.GOOS) + "\" \"$@\"\n"
	if err := os.WriteFile(script, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvGit, script)
}

// A change that does not apply onto the commit is a conflict naming the path,
// worded by its paths, and makes no commit.
func TestCherryPickConflictNamesItsPaths(t *testing.T) {
	dir := committed(t, "a.txt")
	g := Git{}
	first, _ := g.Resolve(t.Context(), "HEAD")
	write(t, dir, "a.txt", "ours")
	gitIn(t, dir, "", "commit", "-q", "-a", "-m", "ours")
	ours, _ := g.Resolve(t.Context(), "HEAD")
	gitIn(t, dir, "", "checkout", "-q", "-b", "side", first)
	write(t, dir, "a.txt", "theirs")
	gitIn(t, dir, "", "commit", "-q", "-a", "-m", "theirs")
	theirs, _ := g.Resolve(t.Context(), "HEAD")
	made, err := g.CherryPick(t.Context(), theirs, ours)
	var conflicts *port.Conflicts
	if !errors.As(err, &conflicts) || made != "" || !slices.Equal(conflicts.Paths, []string{"a.txt"}) ||
		err.Error() != "the change conflicts in a.txt" {
		t.Errorf("CherryPick = %q, %v; want a conflict in a.txt", made, err)
	}
}

// A change carried to another commit, and a branch set to a commit, fail with
// git's words, and git saying a network failed in them (a fetch's or a push's
// words) is no remote failure for these, which reach for none; a merge git
// stops with an exit other than its conflicts' is no conflict.
func TestCherryPickAndResetFailWithGitsWords(t *testing.T) {
	const words = "fatal: unable to access 'https://x/': Could not resolve host: x"
	for _, c := range []struct {
		subcommand string
		code       int
		run        func(g Git, first, second string) error
	}{
		{"reset", 128, func(g Git, first, _ string) error { return g.Reset(t.Context(), first) }},
		{"merge-tree", 128, func(g Git, first, second string) error {
			_, err := g.CherryPick(t.Context(), second, first)
			return err
		}},
		{"commit-tree", 128, func(g Git, first, second string) error {
			_, err := g.CherryPick(t.Context(), second, first)
			return err
		}},
	} {
		t.Run(c.subcommand, func(t *testing.T) {
			dir := committed(t, "a.txt")
			write(t, dir, "b.txt", "b")
			gitIn(t, dir, "", "add", "--", "b.txt")
			gitIn(t, dir, "", "commit", "-q", "-m", "second")
			g := Git{}
			second, _ := g.Resolve(t.Context(), "HEAD")
			first, _ := g.Resolve(t.Context(), "HEAD~")
			failingWith(t, c.subcommand, words, c.code)
			err := c.run(g, first, second)
			var failed *port.Failed
			var conflicts *port.Conflicts
			if !errors.As(err, &failed) || failed.Failure != port.Other || !strings.Contains(failed.Words, "Could not resolve host") || errors.As(err, &conflicts) {
				t.Errorf("a failing %s is %v; want git's failure of no kind", c.subcommand, err)
			}
		})
	}
}

// A commit outside every branch stops where git fails to make its index's
// folder, its blob, its entry or its tree, with git's words.
func TestCommitFileStopsWhereGitFails(t *testing.T) {
	for _, step := range []string{"hash-object", "update-index", "write-tree"} {
		t.Run(step, func(t *testing.T) {
			committed(t, "a.txt")
			failing(t, step)
			if sha, err := (Git{}).CommitFile(t.Context(), "", "f", "x\n", "m", "n", "e@x"); err == nil || sha != "" ||
				!strings.Contains(err.Error(), "injected") {
				t.Errorf("CommitFile = %q, %v", sha, err)
			}
		})
	}
	t.Run("the index's folder", func(t *testing.T) {
		committed(t, "a.txt")
		for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
			t.Setenv(name, filepath.Join(t.TempDir(), "gone"))
		}
		if sha, err := (Git{}).CommitFile(t.Context(), "", "f", "x\n", "m", "n", "e@x"); err == nil || sha != "" {
			t.Errorf("CommitFile = %q, %v", sha, err)
		}
	})
}

// A draft's plumbing: a scratch index read from a commit, a path taken out
// of it and the change read against the commit; git's words when the
// commit is no commit, and git's own error when git does not start.
func TestDraftPlumbing(t *testing.T) {
	dir := committed(t, "a.txt", "b.txt")
	ctx, g := t.Context(), Git{}
	if _, err := g.TreeEntries(ctx, "nope", nil); err == nil {
		t.Error("TreeEntries of no commit: no error")
	}
	s := g.Scratch(filepath.Join(t.TempDir(), "index"))
	if err := s.ReadTree(ctx, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Changed(ctx, "HEAD", []string{"a.txt", "b.txt"}); !slices.Equal(got, []string{"a.txt"}) || err != nil {
		t.Errorf("Changed = %q, %v", got, err)
	}
	if got, err := s.Changed(ctx, "nope", nil); err == nil {
		t.Errorf("Changed against no commit = %q", got)
	}
	t.Setenv(EnvGit, filepath.Join(dir, "no-git-here"))
	if got, err := g.Blob(ctx, "HEAD:a.txt"); err == nil {
		t.Errorf("Blob with no git = %q", got)
	}
}

// git init that cannot make the git folder says git's words; a config that
// cannot be set, outside a repository, says them too.
func TestInitAndSetConfigFail(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only folder needs a unix user other than root")
	}
	dir := filepath.Join(alone(t), "locked")
	if err := os.MkdirAll(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	t.Chdir(dir)
	if err := (Git{}).Init(t.Context()); err == nil || !strings.HasPrefix(err.Error(), "git init: ") {
		t.Errorf("Init in a read-only folder: %v", err)
	}
	if err := (Git{}).SetConfig(t.Context(), "a.b", "c"); err == nil || !strings.HasPrefix(err.Error(), "git config --replace-all a.b c: ") {
		t.Errorf("SetConfig outside a repository: %v", err)
	}
	if got, err := (Git{}).Refs(t.Context(), "refs/tags/"); err == nil {
		t.Errorf("Refs outside a repository = %q", got)
	}
	if got, err := (Git{}).Texts(t.Context(), "HEAD", []string{"a"}); err == nil {
		t.Errorf("Texts outside a repository = %q", got)
	}
}

// A command runs in the folder its Talk names, over the adapter's own.
func TestTalkRunsInItsFolder(t *testing.T) {
	dir := committed(t, "a.txt")
	t.Chdir(t.TempDir())
	top, err := (Git{}).output(t.Context(), port.Talk{Dir: dir}, false, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(strings.TrimSpace(top)) != dir {
		t.Errorf("rev-parse in %s = %q, %v", dir, top, err)
	}
}

// A path with a line break is read alone, and is an error when the tree
// lacks it; the rest are read in one batch.
func TestTextsOfAPathWithALineBreak(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no line break in a file's name on windows")
	}
	committed(t, "a.txt", "line\nbreak.txt")
	ctx := t.Context()
	want := map[string]string{"a.txt": "a.txt\n", "line\nbreak.txt": "line\nbreak.txt\n"}
	if got, err := (Git{}).Texts(ctx, "HEAD", []string{"a.txt", "line\nbreak.txt"}); err != nil || len(got) != 2 ||
		got["a.txt"] != want["a.txt"] || got["line\nbreak.txt"] != want["line\nbreak.txt"] {
		t.Errorf("Texts = %q, %v", got, err)
	}
	if got, err := (Git{}).Texts(ctx, "HEAD", []string{"line\nnone.txt"}); err == nil {
		t.Errorf("Texts of a path HEAD lacks = %q", got)
	}
}

// cat-file --batch's entry is refused when its header cannot be read, its
// size is no number, or its content is cut short.
func TestBatchEntryRefuses(t *testing.T) {
	for _, out := range []string{"", "abc blob ten\n", "abc blob 10\nshort"} {
		if text, ok := batchEntry(bufio.NewReader(strings.NewReader(out))); ok {
			t.Errorf("batchEntry(%q) = %q", out, text)
		}
	}
}
