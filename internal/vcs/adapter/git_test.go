package adapter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// gitIn runs git in dir, its commits dated date when it is not empty.
func gitIn(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	if out, err := gitCmd(dir, date, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s", strings.Join(args, " "), out)
	}
}

// gitCmd is git run in dir as gitIn runs it.
func gitCmd(dir, date string, args ...string) *exec.Cmd {
	cmd := exec.Command(git.Bin(), append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if date != "" {
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	}
	return cmd
}

// write writes a file of dir, its content a line.
func write(t *testing.T, dir, name, line string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// history makes porttest.VCSContract's history in a new repository at dir,
// its commits a second apart, each tagged.
func history(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, "", "", "init", "-q", "-b", "main", dir)
	at := func(second int) string { return fmt.Sprintf("2026-01-01T00:00:0%dZ", second) }
	commit := func(name string, second int, paragraphs ...string) {
		if len(paragraphs) == 0 {
			paragraphs = []string{name}
		}
		args := []string{"commit", "-q", "--allow-empty"}
		for _, p := range append(paragraphs, "the body of "+name) {
			args = append(args, "-m", p)
		}
		gitIn(t, dir, at(second), args...)
		gitIn(t, dir, "", "tag", name)
	}
	change := func(files ...string) {
		for i := 0; i < len(files); i += 2 {
			write(t, dir, files[i], files[i+1])
			gitIn(t, dir, "", "add", files[i])
		}
	}
	change("a.txt", "a", "x.md", "x")
	commit("a", 1)
	change("a.txt", "b", "b.txt", "b")
	commit("b", 2)
	gitIn(t, dir, "", "rm", "-q", "x.md")
	change("c.txt", "c")
	commit("c", 3, "c\nwrapped")
	gitIn(t, dir, "", "notes", "--ref="+port.NotesRef, "add", "-m", "Task: T-1", "c")
	gitIn(t, dir, "", "notes", "add", "-m", "elsewhere", "b")
	gitIn(t, dir, "", "checkout", "-q", "-b", "side", "a")
	change("s.txt", "s")
	commit("s", 4)
	gitIn(t, dir, "", "checkout", "-q", "main")
	gitIn(t, dir, "", "merge", "-q", "--no-ff", "--no-commit", "side")
	change("a.txt", "m")
	commit("m", 5)
	for i, name := range []string{"t", "o"} {
		branch := map[string]string{"t": "theirs", "o": "ours"}[name]
		gitIn(t, dir, "", "checkout", "-q", "-b", branch, "c")
		change("a.txt", name)
		commit(name, 6+i)
	}
	gitIn(t, dir, "", "checkout", "-q", "main")
}

// leave sets the work tree's state as l asks, in the repository at dir.
func leave(t *testing.T, dir string, l porttest.Layout) {
	t.Helper()
	if l.CommentChar != "" {
		gitIn(t, dir, "", "config", "core.commentChar", l.CommentChar)
	}
	switch {
	case l.Dirty:
		write(t, dir, "a.txt", "staged")
		write(t, dir, "new.txt", "new")
		gitIn(t, dir, "", "add", "a.txt", "new.txt")
		gitIn(t, dir, "", "rm", "-q", "b.txt")
		write(t, dir, "c.txt", "changed")
		write(t, dir, "u.txt", "u")
	case l.Merging:
		stopped(t, dir, "merge", "-q", "theirs")
	case l.Rebasing:
		gitIn(t, dir, "", "checkout", "-q", "ours")
		stopped(t, dir, "rebase", "-q", "theirs")
	}
}

// stopped runs a git command that a conflict stops.
func stopped(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := gitCmd(dir, "", args...).CombinedOutput(); err == nil {
		t.Fatalf("git %s was not stopped: %s", strings.Join(args, " "), out)
	}
}

// laidOut stands the test at the top of a new repository laid out as
// porttest.VCSContract asks, with no git setting from outside and no
// repository above it.
func laidOut(t *testing.T, l porttest.Layout) (port.VCS, string) {
	t.Helper()
	for _, name := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_COMMON_DIR",
		"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", base)
	repo := filepath.Join(base, "repo")
	history(t, repo)
	if l.Shallow {
		clone := filepath.Join(base, "clone")
		gitIn(t, base, "", "clone", "-q", "--depth", "1", "file://"+filepath.ToSlash(repo), clone)
		t.Chdir(clone)
		return Git{}, clone
	}
	for _, tag := range l.Remotes {
		gitIn(t, repo, "", "update-ref", "refs/remotes/origin/"+tag, tag)
	}
	for _, dir := range []string{filepath.Join(repo, "sub"), filepath.Join(base, "outside")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "", "worktree", "add", "-q", "--detach", filepath.Join(base, "wt"))
	leave(t, repo, l)
	t.Chdir(repo)
	return Git{}, repo
}

// git answers as the fake does.
func TestGitKeepsTheContract(t *testing.T) {
	porttest.VCSContract(t, laidOut)
}

// The port's names for git's are git's own: internal/git's unpushed range
// and empty tree, which the callers not yet on the port still use.
func TestPortNamesAreGits(t *testing.T) {
	if port.Unpushed != git.Unpushed || port.EmptyTree != git.EmptyTree {
		t.Errorf("port: %q %q; git: %q %q", port.Unpushed, port.EmptyTree, git.Unpushed, git.EmptyTree)
	}
}

// Find refuses a .git file that names no git folder, as git does.
func TestFindRefusesABrokenGitFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	for _, text := range []string{"", "not a gitdir line\n"} {
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if l, ok := (Git{}).Find(t.Context(), dir); ok {
			t.Errorf(".git holding %q: Find = %+v", text, l)
		}
	}
}

// Find walks up as git does: past a folder holding a HEAD file alone,
// which is no git folder, to a .git file naming its git folder relative to
// it, whose empty commondir names none; and from a link, the folder it
// leads to.
func TestFindWalksUpAsGitDoes(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitdir := filepath.Join(dir, "elsewhere", "git")
	for _, d := range []string{filepath.Join(gitdir, "objects"), filepath.Join(gitdir, "refs"), filepath.Join(dir, "sub")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, text := range map[string]string{
		filepath.Join(dir, ".git"):           "gitdir: elsewhere/git\n",
		filepath.Join(gitdir, "HEAD"):        "ref: refs/heads/main\n",
		filepath.Join(gitdir, "commondir"):   "\n",
		filepath.Join(dir, "sub", "HEAD"):    "not a git folder\n",
		filepath.Join(dir, "sub", "objects"): "a file\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := port.Location{Top: dir, GitDir: gitdir, Common: gitdir}
	if l, ok := (Git{}).Find(t.Context(), filepath.Join(dir, "sub")); !ok || l != want {
		t.Errorf("Find(sub) = %+v, %v; want %+v", l, ok, want)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(filepath.Join(dir, "sub"), link); err != nil {
		t.Skipf("no symbolic link: %v", err)
	}
	if l, ok := (Git{}).Find(t.Context(), link); !ok || l != want {
		t.Errorf("Find(link) = %+v, %v; want %+v", l, ok, want)
	}
}

// Find finds nothing when the folder it starts from has no absolute path:
// the working folder cannot be read.
func TestFindFindsNothingWithNoAbsoluteFolder(t *testing.T) {
	was := abs
	abs = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { abs = was })
	if l, ok := (Git{}).Find(t.Context(), "."); ok {
		t.Errorf("Find = %+v", l)
	}
}

// Find refuses a .git file it cannot read.
func TestFindRefusesAnUnreadableGitFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".git")
	if err := os.WriteFile(file, []byte("gitdir: x\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(file); err == nil {
		t.Skip("the file is readable whatever its mode here")
	}
	if l, ok := (Git{}).Find(t.Context(), dir); ok {
		t.Errorf("Find = %+v", l)
	}
}

// Locate asks git once per folder: a repository made after the first ask
// is not seen until the run asks from another folder.
func TestLocateAsksOncePerFolder(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	if _, err := (Git{}).Locate(t.Context(), ""); err == nil {
		t.Fatal("Locate found a repository in an empty folder")
	}
	gitIn(t, dir, "", "init", "-q")
	if _, err := (Git{}).Locate(t.Context(), ""); err == nil {
		t.Error("Locate asked git again")
	}
	if l, err := (Git{}).Locate(t.Context(), "."); err != nil || l.Top != dir {
		t.Errorf("Locate(.) = %+v, %v; want the top %s", l, err, dir)
	}
}
