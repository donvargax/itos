package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// The branch and its upstream, origin and the branch's own name until one
// is set; tracked changes, staged or not, and never untracked files; and a
// rebase stopped on a conflict, seen in progress with its path unmerged
// until it is aborted.
func TestState(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	commitFile(t, dir, "a")
	if got := Branch(); got != "main" {
		t.Fatalf("Branch = %q, want main", got)
	}
	if remote, ref, set := Upstream("main"); remote != "origin" || ref != "refs/heads/main" || set {
		t.Fatalf("with no upstream, Upstream = %q %q %v", remote, ref, set)
	}
	gitIn(t, dir, "config", "branch.main.remote", "up")
	gitIn(t, dir, "config", "branch.main.merge", "refs/heads/trunk")
	if remote, ref, set := Upstream("main"); remote != "up" || ref != "refs/heads/trunk" || !set {
		t.Fatalf("Upstream = %q %q %v, want up refs/heads/trunk true", remote, ref, set)
	}

	if err := os.WriteFile(filepath.Join(dir, "new"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Changed(); len(got) != 0 {
		t.Fatalf("an untracked file counts as a change: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Changed(); !slices.Equal(got, []string{" M a"}) {
		t.Fatalf("Changed = %q, want [\" M a\"]", got)
	}
	gitIn(t, dir, "add", "a")
	if got := Changed(); !slices.Equal(got, []string{"M  a"}) {
		t.Fatalf("staged, Changed = %q, want [\"M  a\"]", got)
	}
	gitIn(t, dir, "commit", "-q", "-m", "a on main")

	gitIn(t, dir, "checkout", "-q", "-b", "other", "HEAD~1")
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "commit", "-q", "-am", "a on other")
	if Rebasing() || len(Conflicted()) > 0 {
		t.Fatal("a rebase is seen before one started")
	}
	if exec.Command(Bin(), "-c", "user.name=t", "-c", "user.email=t@t", "rebase", "main").Run() == nil {
		t.Fatal("the rebase did not stop on its conflict")
	}
	if !Rebasing() || !slices.Equal(Conflicted(), []string{"a"}) {
		t.Fatalf("stopped: Rebasing = %v, Conflicted = %q", Rebasing(), Conflicted())
	}
	gitIn(t, dir, "rebase", "--abort")
	if Rebasing() || len(Conflicted()) > 0 || Branch() != "other" {
		t.Fatalf("aborted: Rebasing = %v, Conflicted = %q, Branch = %q", Rebasing(), Conflicted(), Branch())
	}
	gitIn(t, dir, "checkout", "-q", "--detach")
	if got := Branch(); got != "" {
		t.Fatalf("detached, Branch = %q", got)
	}
}
