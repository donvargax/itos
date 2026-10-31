package adapter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// A draft's change is a git patch, which the port leaves to the adapter:
// taken from the work tree or from a draft's copies through an index of
// its own (Scratch), against a commit, and applied to the index and the
// work tree when the draft is promoted. Every pathspec is read as the path
// it is, and an error is git's own words on stderr.

// drafted runs git for a draft in env (itos's own when nil), every pathspec
// literal, and gives its stdout; its error is git's words on stderr when git
// fails.
func (g Git) drafted(ctx context.Context, env []string, args ...string) (string, error) {
	if env == nil {
		env = os.Environ()
	}
	var stdout, stderr strings.Builder
	cmd := g.git().Command(ctx, args...)
	cmd.Env = append(env[:len(env):len(env)], "GIT_LITERAL_PATHSPECS=1")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return stdout.String(), errors.New(strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}

// TreeEntry is one entry of a tree as git ls-tree lists it: its mode, its
// kind (blob, tree, commit) and its object.
type TreeEntry struct{ Mode, Kind, Object string }

// TreeEntries are the commit's entries at the paths, named from the top
// (git ls-tree --full-tree): a path it lacks is not among them.
func (g Git) TreeEntries(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error) {
	return g.treeEntries(ctx, append([]string{"ls-tree", "-z", "--full-tree", commit, "--"}, paths...))
}

// FolderEntries are TreeEntries with the paths read from the folder git
// runs in and named from it (git ls-tree without --full-tree), as a
// project in a subfolder, with its own itos.yaml, names its files.
func (g Git) FolderEntries(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error) {
	return g.treeEntries(ctx, append([]string{"ls-tree", "-z", commit, "--"}, paths...))
}

// treeEntries are the entries git ls-tree -z lists with the arguments.
func (g Git) treeEntries(ctx context.Context, args []string) (map[string]TreeEntry, error) {
	listed, err := g.drafted(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	entries := map[string]TreeEntry{}
	for _, line := range strings.Split(listed, "\x00") {
		meta, name, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			continue
		}
		entries[name] = TreeEntry{Mode: fields[0], Kind: fields[1], Object: fields[2]}
	}
	return entries, nil
}

// Blob is a blob's content (git cat-file blob).
func (g Git) Blob(ctx context.Context, object string) (string, error) {
	return g.drafted(ctx, nil, "cat-file", "blob", object)
}

// PatchFiles are what git apply --numstat -z lists of the patch file: each
// file's added and deleted lines and its path, NUL-separated.
func (g Git) PatchFiles(ctx context.Context, patch string) (string, error) {
	return g.drafted(ctx, nil, "apply", "--numstat", "-z", patch)
}

// Apply applies the patch file to the index and the work tree.
func (g Git) Apply(ctx context.Context, patch string) error {
	_, err := g.drafted(ctx, nil, "apply", "--index", "--whitespace=nowarn", patch)
	return err
}

// Scratch is an index of its own at a file, which a draft's change is
// staged in, apart from the repository's.
type Scratch struct {
	g   Git
	env []string
}

// Scratch is the index at the file index, which git makes when it reads a
// tree into it.
func (g Git) Scratch(index string) Scratch {
	return Scratch{g: g, env: append(os.Environ(), "GIT_INDEX_FILE="+index)}
}

// ReadTree sets the index to a commit's tree.
func (s Scratch) ReadTree(ctx context.Context, rev string) error {
	_, err := s.g.drafted(ctx, s.env, "read-tree", rev)
	return err
}

// AddAll stages the paths as the work tree has them, a file it lacks taken
// out (git add --all).
func (s Scratch) AddAll(ctx context.Context, paths []string) error {
	_, err := s.g.drafted(ctx, s.env, append([]string{"add", "--all", "--"}, paths...)...)
	return err
}

// Remove takes the path out of the index.
func (s Scratch) Remove(ctx context.Context, path string) error {
	_, err := s.g.drafted(ctx, s.env, "update-index", "--force-remove", "--", path)
	return err
}

// Store writes the file's content as a blob, as it is, and gives its
// object.
func (s Scratch) Store(ctx context.Context, file string) (string, error) {
	object, err := s.g.drafted(ctx, s.env, "hash-object", "-w", "--no-filters", "--", file)
	return strings.TrimSpace(object), err
}

// Set sets the path's entry to the object, in the mode.
func (s Scratch) Set(ctx context.Context, mode, object, path string) error {
	_, err := s.g.drafted(ctx, s.env, "update-index", "--add", "--cacheinfo", mode+","+object+","+path)
	return err
}

// Changed are the paths the index changes against base, of those given.
func (s Scratch) Changed(ctx context.Context, base string, paths []string) ([]string, error) {
	listed, err := s.g.drafted(ctx, s.env, append([]string{"diff", "--cached", "-z", "--name-only", "--no-renames", base, "--"}, paths...)...)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range strings.Split(listed, "\x00") {
		if e != "" {
			names = append(names, e)
		}
	}
	return names, nil
}

// Diff is the index's change of the paths against base as a binary patch
// git apply takes.
func (s Scratch) Diff(ctx context.Context, base string, paths []string) (string, error) {
	return s.g.drafted(ctx, s.env, append([]string{"diff", "--cached", "--binary", "--full-index", "--no-renames", "--no-color",
		"--no-ext-diff", base, "--"}, paths...)...)
}
