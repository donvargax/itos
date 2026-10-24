package adapter

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTaskSnapshotCapturesAlternateIndex(t *testing.T) {
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q")
	write(t, dir, "input", "original")
	gitIn(t, dir, "", "add", "input")
	alternate := filepath.Join(t.TempDir(), "index")
	t.Setenv("GIT_INDEX_FILE", alternate)
	gitIn(t, dir, "", "read-tree", "--empty")
	write(t, dir, "input", "captured")
	gitIn(t, dir, "", "add", "input")
	g := Git{Dir: dir}
	before, err := os.ReadFile(alternate)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := g.IndexTree(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(alternate)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("capturing rewrote the source index")
	}
	write(t, dir, "input", "later")
	gitIn(t, dir, "", "add", "input")
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "cache"))
	t.Setenv("GIT_TRACE", "0")
	snapshot, err := g.TaskSnapshot(t.Context(), tree)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Remove()
	b, err := os.ReadFile(filepath.Join(snapshot.Dir, "input"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "captured\n" {
		t.Errorf("snapshot input = %q", b)
	}
	for _, kv := range snapshot.Env {
		if strings.HasPrefix(kv, "GIT_INDEX_FILE=") {
			t.Error("inherited alternate index")
		}
	}
	for _, name := range []string{"GOCACHE", "GIT_TRACE"} {
		if !slices.Contains(snapshot.Env, name+"="+os.Getenv(name)) {
			t.Errorf("lost nonlocal setting %s", name)
		}
	}
	cmd := g.git().Command(t.Context(), "-C", snapshot.Dir, "rev-parse", "--git-dir")
	cmd.Env = snapshot.Env
	if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != ".git" {
		t.Errorf("private metadata = %s, %v", out, err)
	}
	alternateObjects := filepath.Join(snapshot.Dir, ".git", "objects", "info", "alternates")
	b, err = os.ReadFile(alternateObjects)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != filepath.ToSlash(filepath.Join(dir, ".git", "objects")) {
		t.Errorf("object alternate = %q", b)
	}
	if err := snapshot.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshot.TempDir); !os.IsNotExist(err) {
		t.Errorf("snapshot remained: %v", err)
	}
}

func TestTaskSnapshotSetupErrors(t *testing.T) {
	for _, problem := range []string{"blocked", "gitlink", "unknown-tree"} {
		t.Run(problem, func(t *testing.T) {
			dir := alone(t)
			gitIn(t, dir, "", "init", "-q")
			write(t, dir, "a", "base")
			gitIn(t, dir, "", "add", "a")
			gitIn(t, dir, "", "commit", "-qm", "base")
			g := Git{Dir: dir}
			if problem == "gitlink" {
				head, err := g.Resolve(t.Context(), "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				gitIn(t, dir, "", "update-index", "--add", "--cacheinfo", "160000,"+head+",vendor/module")
			}
			tree, err := g.IndexTree(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(dir, ".git", "itos", "snapshots")
			if problem == "blocked" {
				if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(base, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if problem == "unknown-tree" {
				tree = "not-a-tree"
			}
			_, err = g.TaskSnapshot(t.Context(), tree)
			if err == nil || !strings.Contains(err.Error(), "cannot create task-check snapshot") {
				t.Fatalf("setup error = %v", err)
			}
			if problem == "gitlink" && !strings.Contains(err.Error(), "unsupported gitlink at staged path \"vendor/module\"") {
				t.Errorf("gitlink diagnostic = %v", err)
			}
			if problem != "blocked" {
				entries, err := os.ReadDir(base)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Errorf("partial snapshot remains: %v", entries)
				}
			}
		})
	}
}

func TestUnstagedAndIndexPaths(t *testing.T) {
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q")
	write(t, dir, "kept", "one")
	write(t, dir, "edited", "one")
	write(t, dir, "deleted", "one")
	gitIn(t, dir, "", "add", "--", "kept", "edited", "deleted")
	write(t, dir, "edited", "two")
	write(t, dir, "untracked", "one")
	if err := os.Remove(filepath.Join(dir, "deleted")); err != nil {
		t.Fatal(err)
	}
	g := Git{Dir: dir}
	unstaged, err := g.UnstagedPaths(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(unstaged)
	if !slices.Equal(unstaged, []string{"deleted", "edited"}) {
		t.Errorf("unstaged = %q, want the edited and the deleted path, not the untracked one", unstaged)
	}
	indexed, err := g.IndexPaths(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(indexed)
	if !slices.Equal(indexed, []string{"deleted", "edited", "kept"}) {
		t.Errorf("index paths = %q", indexed)
	}
	if _, err := (Git{Dir: t.TempDir()}).UnstagedPaths(t.Context()); err == nil {
		t.Error("unstaged paths outside a repository")
	}
	if _, err := (Git{Dir: t.TempDir()}).IndexPaths(t.Context()); err == nil {
		t.Error("index paths outside a repository")
	}
}

// A revision's tree and the paths in it are HEAD's, not the index's or the
// working folder's.
func TestRevTreeAndTreePathsReadTheRevision(t *testing.T) {
	dir := alone(t)
	gitIn(t, dir, "", "init", "-q")
	write(t, dir, "kept", "committed")
	gitIn(t, dir, "", "add", "kept")
	gitIn(t, dir, "", "commit", "-q", "-m", "first")
	write(t, dir, "staged", "new")
	gitIn(t, dir, "", "add", "staged")
	write(t, dir, "kept", "edited")
	g := Git{Dir: dir}
	want, err := g.git().Read("rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if tree, err := g.RevTree(t.Context(), "HEAD"); err != nil || tree != strings.TrimSpace(want) {
		t.Errorf("RevTree = %q, %v, want %q", tree, err, want)
	}
	if paths, err := g.TreePaths(t.Context(), "HEAD"); err != nil || !slices.Equal(paths, []string{"kept"}) {
		t.Errorf("TreePaths = %v, %v", paths, err)
	}
	if tree, err := g.RevTree(t.Context(), "nope"); err == nil || tree != "" {
		t.Errorf("RevTree of no revision = %q, %v", tree, err)
	}
	if _, err := g.TreePaths(t.Context(), "nope"); err == nil {
		t.Error("TreePaths of no revision gave no error")
	}
}
