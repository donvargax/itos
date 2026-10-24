package adapter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/kind"
)

// IndexTree captures the active index, including git's alternate commit index,
// as one immutable tree for task selection and checkout alike.
func (g Git) IndexTree(ctx context.Context) (string, error) {
	return g.indexTreeThrough(ctx, func() (indexFile, error) {
		return os.CreateTemp("", "itos-index-*")
	})
}

// indexFile permits deterministic disk write/close failures, as tempFile does
// for the ID counter. A real temporary file cannot fail those on demand.
type indexFile interface {
	Name() string
	Write([]byte) (int, error)
	Close() error
}

func (g Git) indexTreeThrough(ctx context.Context, create func() (indexFile, error)) (tree string, err error) {
	defer func() { err = kind.Wrap(kind.Missing, err) }()
	path, err := g.git().Output("rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if len(data) == 0 {
		// An absent/empty index captures an empty proposal. mktree reads only
		// its empty stdin, never an index that could appear after this capture.
		out, err := g.git().Command(ctx, "mktree").Output()
		return strings.TrimSpace(string(out)), err
	}
	// write-tree can refresh cache-tree extensions. Give it a private copy so
	// even that metadata update cannot rewrite the commit's active index.
	file, err := create()
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	cmd := g.git().Command(ctx, "write-tree")
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+file.Name())
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// TaskCheckout owns a checkout, its sibling output captures and its environment.
type TaskCheckout struct {
	Dir, TempDir string
	Env          []string
	Remove       func() error
}

// TaskSnapshot materializes tree without registering a worktree or sharing
// writable Git state. Source objects are read through a file alternate, so
// nested repositories do not inherit source object storage via the environment.
func (g Git) TaskSnapshot(ctx context.Context, tree string) (result TaskCheckout, err error) {
	return g.taskSnapshotThrough(ctx, tree, os.MkdirTemp)
}

func (g Git) taskSnapshotThrough(ctx context.Context, tree string, mkdirTemp func(string, string) (string, error)) (result TaskCheckout, err error) {
	source := g.git()
	common, err := source.Output("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return result, kind.Wrap(kind.Missing, err)
	}
	base := filepath.Join(strings.TrimSpace(common), "itos", "snapshots")
	defer func() {
		if err != nil {
			err = kind.Wrap(kind.Missing, fmt.Errorf("cannot create task-check snapshot at %s: %w", base, err))
		}
	}()
	if err = os.MkdirAll(base, 0o700); err != nil {
		return result, err
	}
	parent, err := mkdirTemp(base, "run-")
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(parent)
		}
	}()
	entries, err := source.Output("ls-tree", "-r", "-z", tree)
	if err != nil {
		return result, err
	}
	for _, entry := range strings.Split(entries, "\x00") {
		if strings.HasPrefix(entry, "160000 ") {
			_, path, _ := strings.Cut(entry, "\t")
			return result, fmt.Errorf("unsupported gitlink at staged path %q", path)
		}
	}
	local, err := source.Output("rev-parse", "--local-env-vars")
	if err != nil {
		return result, err
	}
	names := map[string]bool{}
	for _, name := range strings.Fields(local) {
		names[name] = true
	}
	env := []string{}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !names[name] {
			env = append(env, kv)
		}
	}
	objects, err := source.Output("rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return result, err
	}
	format, err := source.Output("rev-parse", "--show-object-format")
	if err != nil {
		return result, err
	}
	dir := filepath.Join(parent, "tree")
	run := func(args ...string) error {
		cmd := source.Command(ctx, args...)
		cmd.Env = env
		out, e := cmd.CombinedOutput()
		if e != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), e, out)
		}
		return nil
	}
	if err = run("init", "--quiet", "--template=", "--object-format="+strings.TrimSpace(format), dir); err != nil {
		return result, err
	}
	for _, key := range []string{"core.filemode", "core.symlinks"} {
		value := g.Config(ctx, key, false)
		if len(value) > 0 {
			if err = run("-C", dir, "config", "--local", key, value[len(value)-1]); err != nil {
				return result, err
			}
		}
	}
	alternates := filepath.Join(dir, ".git", "objects", "info", "alternates")
	if err = os.WriteFile(alternates, []byte(filepath.ToSlash(strings.TrimSpace(objects))+"\n"), 0o600); err != nil {
		return result, err
	}
	if err = run("-C", dir, "read-tree", tree); err != nil {
		return result, err
	}
	if err = run("-C", dir, "checkout-index", "--all", "--force"); err != nil {
		return result, err
	}
	return TaskCheckout{Dir: dir, TempDir: parent, Env: env, Remove: func() error { return os.RemoveAll(parent) }}, nil
}

// UnstagedPaths are the tracked paths whose working copy differs from the
// index (git diff --name-only): a file edited, deleted or left conflicted.
// Untracked files are not among them.
func (g Git) UnstagedPaths(context.Context) ([]string, error) {
	return g.git().Paths("diff", "--name-only", "--no-renames", "--")
}

// IndexPaths are the paths the index holds (git ls-files --cached), the
// files a snapshot of its tree has and the tree's own: a declared input that
// is one of them is the tree's, not the working folder's.
func (g Git) IndexPaths(context.Context) ([]string, error) {
	return g.git().Paths("ls-files", "--cached", "--")
}

// RevTree is the tree a revision names (git rev-parse <rev>^{tree}).
func (g Git) RevTree(_ context.Context, rev string) (string, error) {
	tree, err := g.git().Read("rev-parse", "--verify", rev+"^{tree}")
	return strings.TrimSpace(tree), err
}

// TreePaths are the paths of the files the revision's tree holds (git
// ls-tree -r --name-only): the files a snapshot of that tree has.
func (g Git) TreePaths(_ context.Context, rev string) ([]string, error) {
	return g.git().Paths("ls-tree", "-r", "--name-only", rev, "--")
}
