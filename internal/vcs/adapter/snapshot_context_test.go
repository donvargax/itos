package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
)

// contextRepo is a repository with the state a snapshot takes: two commits
// on main with a staged change, a lightweight and an annotated tag, a note, a
// remote with the branch pushed and set as its upstream and origin/HEAD set,
// a remote written without a value, a second branch with an upstream of its
// own, and local config a snapshot leaves behind.
func contextRepo(t *testing.T) (Git, string) {
	t.Helper()
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q", "-b", "main")
	write(t, dir, "a", "one")
	gitIn(t, dir, "", "add", "a")
	gitIn(t, dir, "", "commit", "-qm", "one")
	write(t, dir, "a", "two")
	gitIn(t, dir, "", "commit", "-qam", "two")
	gitIn(t, dir, "", "tag", "light")
	gitIn(t, dir, "", "tag", "-a", "-m", "annotated", "v1")
	gitIn(t, dir, "", "notes", "add", "-m", "a note")
	remote := filepath.Join(alone(t), "remote.git")
	gitIn(t, "", "", "init", "-q", "--bare", "-b", "main", remote)
	gitIn(t, dir, "", "remote", "add", "origin", remote)
	gitIn(t, dir, "", "config", "--add", "remote.origin.fetch", "+refs/tags/*:refs/tags/*")
	gitIn(t, dir, "", "push", "-q", "origin", "main")
	gitIn(t, dir, "", "fetch", "-q", "origin")
	gitIn(t, dir, "", "branch", "-q", "--set-upstream-to=origin/main", "main")
	gitIn(t, dir, "", "remote", "set-head", "origin", "main")
	gitIn(t, dir, "", "branch", "side")
	gitIn(t, dir, "", "config", "branch.side.remote", "origin")
	gitIn(t, dir, "", "config", "branch.side.merge", "refs/heads/side")
	gitIn(t, dir, "", "config", "alias.zz", "status")
	config, err := os.OpenFile(filepath.Join(dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.WriteString("[remote \"other\"]\n\tprune\n"); err != nil {
		t.Fatal(err)
	}
	if err := config.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "b", "staged")
	gitIn(t, dir, "", "add", "b")
	return Git{Dir: dir}, dir
}

// out is a git command's output, trimmed, run in the folder itos runs in.
func (g Git) out(t *testing.T, args ...string) string {
	t.Helper()
	out, err := g.git().Output(args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// in is a git command's output, trimmed, run in the snapshot with its
// environment; "failed" when git fails.
func (g Git) in(t *testing.T, snapshot TaskCheckout, args ...string) string {
	t.Helper()
	cmd := g.git().Command(t.Context(), append([]string{"-C", snapshot.Dir}, args...)...)
	cmd.Env = snapshot.Env
	out, err := cmd.Output()
	if err != nil {
		return "failed"
	}
	return strings.TrimSpace(string(out))
}

// snapshotOf is a snapshot of the index tree of g.
func snapshotOf(t *testing.T, g Git) (TaskCheckout, string) {
	t.Helper()
	tree, err := g.IndexTree(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := g.TaskSnapshot(t.Context(), tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = snapshot.Remove() })
	return snapshot, tree
}

// A snapshot has the source's HEAD on its branch, its history, every ref
// (the symbolic origin/HEAD too, and an annotated tag's object), the config
// of every remote (a value written twice, one written without) and of the
// upstream of the branch it is on and no other, none of the rest of the local
// config, and the tree it was made of as its index, with no commit for it.
func TestTaskSnapshotHasTheSourcesGitContext(t *testing.T) {
	g, _ := contextRepo(t)
	snapshot, tree := snapshotOf(t, g)
	for _, args := range [][]string{
		{"rev-parse", "HEAD"}, {"rev-parse", "HEAD~1"}, {"symbolic-ref", "HEAD"}, {"rev-parse", "--symbolic-full-name", "@{u}"},
		{"symbolic-ref", "refs/remotes/origin/HEAD"}, {"rev-parse", "v1"}, {"rev-parse", "v1^{commit}"}, {"rev-parse", "origin"},
		{"for-each-ref", "--format=%(refname) %(objectname) %(symref)"},
	} {
		if got, want := g.in(t, snapshot, args...), g.out(t, args...); got != want {
			t.Errorf("git %s: snapshot says %q, source %q", strings.Join(args, " "), got, want)
		}
	}
	config := g.in(t, snapshot, "config", "--local", "--list")
	for _, line := range []string{
		"remote.origin.url=" + g.out(t, "config", "--local", "remote.origin.url"), "remote.origin.fetch=+refs/heads/*:refs/remotes/origin/*",
		"branch.main.remote=origin", "branch.main.merge=refs/heads/main",
	} {
		if !strings.Contains(config+"\n", line+"\n") {
			t.Errorf("the snapshot's config lacks %q:\n%s", line, config)
		}
	}
	for _, left := range []string{"alias.zz", "branch.side", "user."} {
		if strings.Contains(config, left) {
			t.Errorf("the snapshot's config has %q:\n%s", left, config)
		}
	}
	if got := g.in(t, snapshot, "config", "--local", "--get", "remote.other.prune"); got != "true" {
		t.Errorf("a remote key written without a value is %q", got)
	}
	if got := g.in(t, snapshot, "config", "--local", "--get-all", "remote.origin.fetch"); got != g.out(t, "config", "--local", "--get-all", "remote.origin.fetch") {
		t.Errorf("a remote key written twice is %q", got)
	}
	if got := g.in(t, snapshot, "write-tree"); got != tree {
		t.Errorf("the snapshot's index is %q, not the tree %q", got, tree)
	}
	if got := g.in(t, snapshot, "diff", "--cached", "--name-only", "HEAD"); got != "b" {
		t.Errorf("git diff --cached HEAD = %q: no commit was made for the proposal", got)
	}
	if got := g.in(t, snapshot, "ls-remote", "--exit-code", "origin"); got == "failed" {
		t.Error("the remote origin cannot be reached from the snapshot")
	}
}

// Whatever a check writes in the snapshot, HEAD, refs, tags, the index, the
// config and objects, stays there: the source's are as they were.
func TestTaskSnapshotWritesStayInTheSnapshot(t *testing.T) {
	g, dir := contextRepo(t)
	tree, err := g.IndexTree(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	state := func() string {
		return strings.Join([]string{
			g.out(t, "for-each-ref"), g.out(t, "symbolic-ref", "HEAD"), g.out(t, "config", "--local", "--list"),
			g.out(t, "ls-files", "--stage"), g.out(t, "count-objects", "-v"), g.out(t, "reflog", "show", "--all"),
		}, "\n--\n")
	}
	before := state()
	snapshot, err := g.TaskSnapshot(t.Context(), tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = snapshot.Remove() })
	var blob string
	for _, args := range [][]string{
		{"update-ref", "refs/heads/probe-branch", "HEAD"}, {"tag", "probe-tag"}, {"update-ref", "refs/remotes/origin/main", "HEAD~1"},
		{"config", "remote.origin.url", "/nowhere"}, {"commit", "-q", "--allow-empty", "-m", "probe"},
		{"read-tree", "--empty"}, {"symbolic-ref", "HEAD", "refs/heads/probe-branch"},
	} {
		cmd := g.git().Command(t.Context(), append([]string{"-C", snapshot.Dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = snapshot.Env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s in the snapshot: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	cmd := g.git().Command(t.Context(), "-C", snapshot.Dir, "hash-object", "-w", "--stdin")
	cmd.Env, cmd.Stdin = snapshot.Env, strings.NewReader("snapshot-private-object")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	blob = strings.TrimSpace(string(out))
	if after := state(); after != before {
		t.Errorf("the source changed:\nbefore\n%s\nafter\n%s", before, after)
	}
	if err := g.git().Command(t.Context(), "-C", dir, "cat-file", "-e", blob).Run(); err == nil {
		t.Error("the source holds an object written in the snapshot")
	}
}

// A detached HEAD is detached at the same commit, an unborn one is unborn on
// the same branch with no ref, and a linked worktree's snapshot has that
// worktree's HEAD, not the main checkout's.
func TestTaskSnapshotHeadInEachState(t *testing.T) {
	t.Run("detached", func(t *testing.T) {
		g, _ := contextRepo(t)
		commit := g.out(t, "rev-parse", "HEAD~1")
		gitIn(t, g.Dir, "", "checkout", "-q", "--detach", commit)
		snapshot, _ := snapshotOf(t, g)
		if got := g.in(t, snapshot, "rev-parse", "HEAD"); got != commit {
			t.Errorf("HEAD = %q, want %q", got, commit)
		}
		if got := g.in(t, snapshot, "symbolic-ref", "-q", "HEAD"); got != "failed" {
			t.Errorf("HEAD is on %q, not detached", got)
		}
		if got := g.in(t, snapshot, "config", "--local", "--get-regexp", `^branch\.`); got != "failed" {
			t.Errorf("a detached HEAD has the upstream of a branch: %q", got)
		}
	})
	t.Run("unborn", func(t *testing.T) {
		dir := alone(t)
		gitIn(t, dir, "", "init", "-q", "-b", "main")
		gitIn(t, dir, "", "symbolic-ref", "HEAD", "refs/heads/fresh")
		write(t, dir, "a", "staged")
		gitIn(t, dir, "", "add", "a")
		g := Git{Dir: dir}
		snapshot, tree := snapshotOf(t, g)
		if got := g.in(t, snapshot, "symbolic-ref", "HEAD"); got != "refs/heads/fresh" {
			t.Errorf("HEAD is on %q", got)
		}
		if got := g.in(t, snapshot, "rev-parse", "--verify", "-q", "HEAD"); got != "failed" {
			t.Errorf("an unborn HEAD names %q", got)
		}
		if got := g.in(t, snapshot, "for-each-ref"); got != "" {
			t.Errorf("refs of a repository with none: %q", got)
		}
		if _, err := os.Stat(filepath.Join(snapshot.Dir, ".git", "packed-refs")); !os.IsNotExist(err) {
			t.Errorf("a packed-refs file for no refs: %v", err)
		}
		if got := g.in(t, snapshot, "write-tree"); got != tree {
			t.Errorf("index %q, want %q", got, tree)
		}
	})
	t.Run("linked worktree", func(t *testing.T) {
		g, dir := contextRepo(t)
		linked := filepath.Join(filepath.Dir(dir), "linked")
		gitIn(t, dir, "", "worktree", "add", "-q", "-b", "wt", linked, "HEAD~1")
		w := Git{Dir: linked}
		snapshot, _ := snapshotOf(t, w)
		if got := w.in(t, snapshot, "symbolic-ref", "HEAD"); got != "refs/heads/wt" {
			t.Errorf("the worktree's snapshot is on %q", got)
		}
		if got, want := w.in(t, snapshot, "rev-parse", "HEAD"), g.out(t, "rev-parse", "HEAD~1"); got != want {
			t.Errorf("HEAD = %q, want the worktree's %q", got, want)
		}
		if got := w.in(t, snapshot, "config", "--local", "--get-regexp", `^branch\.`); got != "failed" {
			t.Errorf("a branch with no upstream has config: %q", got)
		}
		if got := w.in(t, snapshot, "for-each-ref", "--format=%(refname)", "refs/heads/"); !strings.Contains(got, "refs/heads/main") {
			t.Errorf("the worktree's snapshot lacks the other branches: %q", got)
		}
	})
}

// A source with no remote and no ref has no config of that kind to copy: a
// git config that finds no key is not an error.
func TestTaskSnapshotOfARepositoryWithNoRemote(t *testing.T) {
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q", "-b", "main")
	write(t, dir, "a", "one")
	gitIn(t, dir, "", "add", "a")
	gitIn(t, dir, "", "commit", "-qm", "one")
	g := Git{Dir: dir}
	snapshot, _ := snapshotOf(t, g)
	if got := g.in(t, snapshot, "config", "--local", "--get-regexp", `^(remote|branch)\.`); got != "failed" {
		t.Errorf("config without a remote: %q", got)
	}
	if got, want := g.in(t, snapshot, "rev-parse", "HEAD"), g.out(t, "rev-parse", "HEAD"); got != want {
		t.Errorf("HEAD = %q, want %q", got, want)
	}
}

// ownedByContext holds any remote key, and of the branch keys only the
// remote and merge of the branch HEAD is on.
func TestOwnedByContext(t *testing.T) {
	for _, c := range []struct {
		key, branch string
		want        bool
	}{
		{"remote.origin.url", "", true},
		{"remote.origin.pushurl", "refs/heads/main", true},
		{"branch.main.remote", "refs/heads/main", true},
		{"branch.main.merge", "refs/heads/main", true},
		{"branch.main.rebase", "refs/heads/main", false},
		{"branch.side.remote", "refs/heads/main", false},
		{"branch.feature.x.remote", "refs/heads/feature.x", true},
		{"branch.main.remote", "", false},
		{"branch.main.remote", "refs/other/main", false},
		{"alias.zz", "refs/heads/main", false},
		{"", "refs/heads/main", false},
	} {
		if got := ownedByContext(c.key, c.branch); got != c.want {
			t.Errorf("ownedByContext(%q, %q) = %v", c.key, c.branch, got)
		}
	}
}

// Head is HEAD's commit, "unborn" before the branch has one, and an error
// where HEAD names neither: no repository, or a detached HEAD at no object.
func TestHead(t *testing.T) {
	dir := alone(t)
	g := Git{Dir: dir}
	if head, err := g.Head(t.Context()); err == nil {
		t.Errorf("outside a repository HEAD is %q", head)
	}
	gitIn(t, dir, "", "init", "-q", "-b", "main")
	if head, err := g.Head(t.Context()); err != nil || head != "unborn" {
		t.Errorf("unborn HEAD = %q, %v", head, err)
	}
	write(t, dir, "a", "one")
	gitIn(t, dir, "", "add", "a")
	gitIn(t, dir, "", "commit", "-qm", "one")
	want := g.out(t, "rev-parse", "HEAD")
	if head, err := g.Head(t.Context()); err != nil || head != want {
		t.Errorf("HEAD = %q, %v, want %q", head, err, want)
	}
	gitIn(t, dir, "", "checkout", "-q", "--detach")
	if head, err := g.Head(t.Context()); err != nil || head != want {
		t.Errorf("detached HEAD = %q, %v, want %q", head, err, want)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte(strings.Repeat("a", len(want))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if head, err := g.Head(t.Context()); err == nil {
		t.Errorf("a HEAD at no object is %q", head)
	}
}

// A context that cannot be read is a snapshot that cannot be made: no
// checkout is left, and the error is a missing environment (exit 3).
func TestTaskSnapshotContextFailures(t *testing.T) {
	for _, c := range []struct {
		fault   string
		arrange func(t *testing.T, g Git, dir string)
	}{
		{"for-each-ref", func(*testing.T, Git, string) {}},
		{"config --null", func(*testing.T, Git, string) {}},
		{"config --local --add", func(*testing.T, Git, string) {}},
		{"symbolic-ref HEAD", func(*testing.T, Git, string) {}},
		{"symbolic-ref ref", func(*testing.T, Git, string) {}},
		{"packed-refs-directory", func(*testing.T, Git, string) {}},
		{"update-ref", func(t *testing.T, g Git, dir string) {
			gitIn(t, dir, "", "checkout", "-q", "--detach")
		}},
		{"detached at no object", func(t *testing.T, g Git, dir string) {
			gitIn(t, dir, "", "checkout", "-q", "--detach")
			head := g.out(t, "rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte(strings.Repeat("a", len(head))+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.fault, func(t *testing.T) {
			g, dir := contextRepo(t)
			c.arrange(t, g, dir)
			tree, err := g.IndexTree(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			real := g.git().Command(t.Context(), "--version").Path
			useFakeGit(t, "unused")
			t.Setenv(snapshotGitFault, c.fault)
			t.Setenv(snapshotRealGit, real)
			snapshot, err := g.TaskSnapshot(t.Context(), tree)
			if err == nil || kind.Of(err) != kind.Missing || !strings.Contains(err.Error(), "cannot create task-check snapshot") {
				t.Fatalf("setup = %+v, %v", snapshot, err)
			}
			if snapshot.Dir != "" || snapshot.Remove != nil {
				t.Errorf("failed setup returned a usable checkout: %+v", snapshot)
			}
			entries, readErr := os.ReadDir(filepath.Join(dir, ".git", "itos", "snapshots"))
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Errorf("partial resources remain: %v", entries)
			}
		})
	}
}
