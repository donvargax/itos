package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// here runs the real git of the OS the tests run on.
var here = Runner{Platform: runtime.GOOS}

// A path git printed is from the folder it ran in, unless it is absolute.
func TestRunnerIn(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "x")
	r := Runner{Dir: "dir"}
	if got := r.in(abs + "\n"); got != abs {
		t.Errorf("in(%s) = %q", abs, got)
	}
	if got := r.in(".git/MERGE_HEAD\n"); got != filepath.Join("dir", ".git", "MERGE_HEAD") {
		t.Errorf("in(.git/MERGE_HEAD) = %q", got)
	}
}

// gitIn is git's output in dir, away from the repository a hook runs the
// tests in.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	cmd := exec.Command(Bin(runtime.GOOS), append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
	}
	return strings.TrimSpace(string(out))
}

// commitFile commits a file with its name as its text, and gives the commit.
func commitFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", name)
	gitIn(t, dir, "commit", "-q", "-m", name)
	return gitIn(t, dir, "rev-parse", "HEAD")
}

// With no remote every commit is unpushed and no one commit starts them;
// with everything pushed the range is empty, starting at its end; with
// commits on top of a pushed one, that one starts them; and with two remote
// branches merged under them, neither an ancestor of the other, none can,
// though UnpushedBases names both; a range git cannot read adds with no base.
func TestUnpushed(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	first := commitFile(t, dir, "a")
	commitFile(t, dir, "b")
	if got := here.UnpushedBase("HEAD"); got != "" {
		t.Fatalf("with no remote, UnpushedBase = %q, want \"\"", got)
	}
	bases := func(want []string, wantAdds bool) {
		t.Helper()
		got, adds := here.UnpushedBases("HEAD")
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) || adds != wantAdds {
			t.Fatalf("UnpushedBases = %q, %v; want %q, %v", got, adds, want, wantAdds)
		}
	}
	bases(nil, true)
	if got, adds := here.UnpushedBases("no-such-commit"); got != nil || !adds {
		t.Fatalf("UnpushedBases of no commit = %q, %v; want none, true", got, adds)
	}
	gitIn(t, dir, "init", "-q", "--bare", filepath.Join(dir, ".git", "remote.git"))
	gitIn(t, dir, "remote", "add", "origin", filepath.Join(dir, ".git", "remote.git"))
	gitIn(t, dir, "push", "-q", "origin", "HEAD:refs/heads/main")
	if got := here.UnpushedBase("HEAD"); got != "HEAD" {
		t.Fatalf("with everything pushed, UnpushedBase = %q, want HEAD", got)
	}
	bases(nil, false)
	pushed := gitIn(t, dir, "rev-parse", "HEAD")
	commitFile(t, dir, "c")
	if got := here.UnpushedBase("HEAD"); got != pushed {
		t.Fatalf("UnpushedBase = %q, want the pushed %s", got, pushed)
	}
	bases([]string{pushed}, true)
	gitIn(t, dir, "checkout", "-q", "-b", "other", first)
	other := commitFile(t, dir, "d")
	gitIn(t, dir, "push", "-q", "origin", "HEAD:refs/heads/other")
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-edit", "other")
	if got := here.UnpushedBase("HEAD"); got != "" {
		t.Fatalf("growing from two remote branches, UnpushedBase = %q, want \"\"", got)
	}
	bases([]string{pushed, other}, true)
}

// A rename is both its paths (bug 32) in a commit's, with git's rename
// detection on: the old path's deletion and the new path's addition, as the
// commit-msg hook reads the staged ones.
func TestRenameIsBothPaths(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	gitIn(t, dir, "config", "diff.renames", "true")
	commitFile(t, dir, "a")
	gitIn(t, dir, "mv", "a", "b")
	gitIn(t, dir, "commit", "-q", "-m", "rename")
	if got, _ := here.CommitPaths("HEAD"); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("CommitPaths = %q, want [a b]", got)
	}
}
