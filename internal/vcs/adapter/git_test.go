package adapter

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
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
	cmd := exec.Command(git.Bin(runtime.GOOS), append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
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
	gitIn(t, dir, "", "config", "user.name", "t")
	gitIn(t, dir, "", "config", "user.email", "t@t")
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
	gitIn(t, dir, at(8), "tag", "-a", "r", "-m", "release", "c")
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte("debug.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	sharedOnce sync.Once
	sharedRoot string
	sharedErr  error
)

// shared is a folder for what the package's tests make once and read, removed
// by TestMain.
func shared(t *testing.T) string {
	t.Helper()
	sharedOnce.Do(func() { sharedRoot, sharedErr = os.MkdirTemp("", "itos-adapter-") })
	if sharedErr != nil {
		t.Fatal(sharedErr)
	}
	return sharedRoot
}

// madeOnce makes what the package's tests share under name, with build in a
// folder of its own, once: under the first caller's git settings (alone's,
// the same for every caller). git starts are what the tests spend, and the
// history alone is thirty of them. It is the folder holding what build wrote.
func madeOnce(t *testing.T, name string, build func(t *testing.T, dir string)) string {
	t.Helper()
	dir := filepath.Join(shared(t), name)
	if _, err := os.Stat(filepath.Join(dir, ".made")); err == nil {
		return dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	build(t, dir)
	write(t, dir, ".made", name)
	return dir
}

// historyTemplate is a repository holding history's commits.
func historyTemplate(t *testing.T) string {
	t.Helper()
	return filepath.Join(madeOnce(t, "history", func(t *testing.T, dir string) {
		history(t, filepath.Join(dir, "repo"))
	}), "repo")
}

// copyTree copies the folder from to a new folder to, files and folders
// alone.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, text, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// leave sets the work tree's state as l asks, in the repository at dir.
func leave(t *testing.T, dir string, l porttest.Layout) {
	t.Helper()
	if l.CommentChar != "" {
		gitIn(t, dir, "", "config", "core.commentChar", l.CommentChar)
	}
	switch {
	case l.Ours:
		gitIn(t, dir, "", "checkout", "-q", "ours")
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

// alone is a new folder for a test's repositories, with no git setting
// from outside and no repository above it.
func alone(t *testing.T) string {
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
	return base
}

// Show refuses a revision that gives no commit, as every branch of a
// repository with none does, and PathStatus a folder in no repository.
func TestShowAndPathStatusRefuse(t *testing.T) {
	dir := alone(t)
	t.Chdir(dir)
	got, err := (Git{}).PathStatus(t.Context(), "a")
	var failed *port.Failed
	if !errors.As(err, &failed) || !strings.HasPrefix(failed.Command, "git status") || strings.TrimSpace(failed.Words) == "" {
		t.Errorf("PathStatus outside a repository is %q, %v; want a port.Failed of git status holding git's words", got, err)
	}
	gitIn(t, dir, "", "init", "-q")
	if got, err := (Git{}).Show(t.Context(), "--all"); err == nil {
		t.Errorf("Show of no commit is %+v", got)
	}
}

// HooksPath is the folder git runs hook files from, absolute, the git
// folder's hooks or core.hooksPath's, from Dir when one is given; an error
// outside a repository.
func TestHooksPath(t *testing.T) {
	dir := alone(t)
	t.Chdir(dir)
	if got, err := (Git{}).HooksPath(t.Context()); err == nil {
		t.Errorf("outside a repository: %q", got)
	}
	gitIn(t, dir, "", "init", "-q")
	if got, err := (Git{}).HooksPath(t.Context()); err != nil || got != filepath.Join(dir, ".git", "hooks") {
		t.Errorf("the git folder's: %q, %v", got, err)
	}
	gitIn(t, dir, "", "config", "core.hooksPath", "tools/hooks")
	if got, err := (Git{}).HooksPath(t.Context()); err != nil || got != filepath.Join(dir, "tools", "hooks") {
		t.Errorf("core.hooksPath's: %q, %v", got, err)
	}
	t.Chdir(t.TempDir())
	if got, err := (Git{Dir: dir}).HooksPath(t.Context()); err != nil || got != filepath.Join(dir, "tools", "hooks") {
		t.Errorf("under Dir: %q, %v", got, err)
	}
}

// A rename is both its paths (bug 32) in every list of changed paths, with
// git's rename detection on.
func TestRenameIsBothPaths(t *testing.T) {
	dir := alone(t)
	t.Chdir(dir)
	gitIn(t, dir, "", "init", "-q", "-b", "main")
	gitIn(t, dir, "", "config", "diff.renames", "true")
	write(t, dir, "a", "a")
	gitIn(t, dir, "", "add", "a")
	gitIn(t, dir, "", "commit", "-q", "-m", "a")
	gitIn(t, dir, "", "mv", "a", "b")
	ctx, both := t.Context(), []string{"a", "b"}
	if got, err := (Git{}).Staged(ctx, ""); !slices.Equal(got, both) || err != nil {
		t.Errorf("Staged = %q, %v", got, err)
	}
	gitIn(t, dir, "", "commit", "-q", "-m", "rename")
	if got, err := (Git{}).CommitPaths(ctx, "HEAD"); !slices.Equal(got, both) || err != nil {
		t.Errorf("CommitPaths = %q, %v", got, err)
	}
	if got, err := (Git{}).Diff(ctx, "HEAD^", "HEAD"); !slices.Equal(got, both) || err != nil {
		t.Errorf("Diff = %q, %v", got, err)
	}
	if got, err := (Git{}).Touched(ctx, port.Range{To: "HEAD"}, false); !slices.Equal(got, both) || err != nil {
		t.Errorf("Touched = %q, %v", got, err)
	}
}

// laidOut is the git adapter run at the top of a new repository laid out as
// porttest.VCSContract asks, with no git setting from outside and no
// repository above it, the test standing in another folder.
func laidOut(t *testing.T, l porttest.Layout) (port.VCS, string) {
	t.Helper()
	base := alone(t)
	t.Chdir(base)
	repo := filepath.Join(base, "repo")
	if l.None {
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		return Git{Dir: repo}, repo
	}
	copyTree(t, historyTemplate(t), repo)
	if l.Origin {
		origin := filepath.Join(base, "origin.git")
		gitIn(t, base, "", "init", "-q", "--bare", "-b", "main", origin)
		gitIn(t, repo, "", "push", "-q", origin, "b:refs/heads/main", "t:refs/heads/theirs")
		gitIn(t, repo, "", "remote", "add", "origin", origin)
		// A proxy answers for a host that cannot be reached, so git would
		// not say the host is unreachable; this one goes direct.
		t.Setenv("NO_PROXY", "itos.invalid")
		t.Setenv("no_proxy", "itos.invalid")
		gitIn(t, repo, "", "remote", "add", "down", "https://itos.invalid/x.git")
		gitIn(t, repo, "", "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		gitIn(t, repo, "", "config", "branch.main.remote", "origin")
		gitIn(t, repo, "", "config", "branch.main.merge", "refs/heads/main")
	}
	if l.Shallow {
		clone := filepath.Join(base, "clone")
		gitIn(t, base, "", "clone", "-q", "--depth", "1", "file://"+filepath.ToSlash(repo), clone)
		return Git{Dir: clone}, clone
	}
	if len(l.Remotes) > 0 {
		var refs strings.Builder
		for _, tag := range l.Remotes {
			fmt.Fprintf(&refs, "update refs/remotes/origin/%s %s\n", tag, tag)
		}
		cmd := gitCmd(repo, "", "update-ref", "--stdin")
		cmd.Stdin = strings.NewReader(refs.String())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git update-ref --stdin: %s", out)
		}
	}
	for _, dir := range []string{filepath.Join(repo, "sub"), filepath.Join(base, "outside")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "", "worktree", "add", "-q", "--detach", filepath.Join(base, "wt"))
	leave(t, repo, l)
	return Git{Dir: repo}, repo
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
// a relative folder and no Dir to read it from.
func TestFindFindsNothingWithNoAbsoluteFolder(t *testing.T) {
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

// Locate asks git once per folder from Dir: a repository made after the
// first ask is not seen until the run asks from another folder. A relative
// folder with no Dir is asked every time, as the process may stand anywhere.
func TestLocateAsksOncePerFolder(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	g := Git{Dir: dir}
	if _, err := g.Locate(t.Context(), ""); err == nil {
		t.Fatal("Locate found a repository in an empty folder")
	}
	if _, err := (Git{}).Locate(t.Context(), ""); err == nil {
		t.Fatal("Locate with no Dir found a repository in an empty folder")
	}
	gitIn(t, dir, "", "init", "-q")
	for _, at := range []string{"", ".", dir} {
		if _, err := g.Locate(t.Context(), at); err == nil {
			t.Errorf("Locate(%q) asked git again", at)
		}
	}
	if l, err := g.Locate(t.Context(), "sub"); err != nil || l.Top != dir {
		t.Errorf("Locate(sub) = %+v, %v; want the top %s", l, err, dir)
	}
	if l, err := (Git{}).Locate(t.Context(), ""); err != nil || l.Top != dir {
		t.Errorf("Locate with no Dir = %+v, %v; want the top %s", l, err, dir)
	}
}
