package adapter

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// spec is a file at a tree as git show and cat-file name it: `:<path>` in
// the index, `<tree>:<path>` elsewhere.
func spec(tree, path string) string {
	if tree == port.Index {
		return ":" + path
	}
	return tree + ":" + path
}

// File is git show <tree>:<path>; a path git says the tree lacks (missing)
// is not ok, any other failure an error.
func (g Git) File(ctx context.Context, tree, path string) (string, bool, error) {
	text, err := g.output(ctx, port.Talk{}, false, "show", spec(tree, path))
	var failed *port.Failed
	switch {
	case errors.As(err, &failed) && missing(failed.Words):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return text, true, nil
}

// Files is git ls-files --cached for the index, else git ls-tree -r, each
// read NUL-separated.
func (g Git) Files(_ context.Context, tree, dir string) ([]string, error) {
	args := []string{"ls-tree", "-r", "--name-only", tree}
	if tree == port.Index {
		args = []string{"ls-files", "--cached"}
	}
	if dir != "" {
		args = append(args, "--", dir)
	}
	return g.git().Paths(args...)
}

// Texts reads the files in one git cat-file --batch, which takes one name a
// line, a carriage return at its end dropped: a path with a line break in
// it is read alone, with git show.
func (g Git) Texts(ctx context.Context, tree string, paths []string) (map[string]string, error) {
	texts := map[string]string{}
	var batched []string
	var specs bytes.Buffer
	for _, p := range paths {
		if strings.ContainsAny(p, "\r\n") {
			text, ok, err := g.File(ctx, tree, p)
			if err != nil || !ok {
				return nil, errors.Join(err, errors.New(spec(tree, p)+" holds no file"))
			}
			texts[p] = text
			continue
		}
		batched = append(batched, p)
		specs.WriteString(spec(tree, p) + "\n")
	}
	if len(batched) == 0 {
		return texts, nil
	}
	cmd := g.git().Command(ctx, "cat-file", "--batch")
	cmd.Stdin = &specs
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for _, p := range batched {
		text, ok := batchEntry(r)
		if !ok {
			return nil, errors.New(spec(tree, p) + " holds no file")
		}
		texts[p] = text
	}
	return texts, nil
}

// batchEntry is the next object git cat-file --batch printed: its header
// line, "<object> <type> <size>", then its content and a line break; not ok
// for a name it reports missing.
func batchEntry(r *bufio.Reader) (string, bool) {
	header, err := r.ReadString('\n')
	if err != nil {
		return "", false
	}
	fields := strings.Fields(header)
	if len(fields) != 3 {
		return "", false
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", false
	}
	content := make([]byte, size+1)
	if _, err := io.ReadFull(r, content); err != nil {
		return "", false
	}
	return string(content[:size]), true
}

// Patch is git show --patch with no header; its error is git's, worded as
// git.Read words it.
func (g Git) Patch(_ context.Context, rev string) (string, error) {
	return g.git().Read("show", "--no-color", "--patch", "--format=", rev)
}

// Shown is git show --patch of the commits, the itos notes alone shown when
// notes is set; its error is git's, worded as git.Read words it.
func (g Git) Shown(_ context.Context, revs []string, notes bool) (string, error) {
	args := []string{"show", "--no-color", "--patch", "--no-notes"}
	if notes {
		args = append(args, "--notes="+port.NotesRef)
	}
	return g.git().Read(append(args, revs...)...)
}

// Ref is git rev-parse --verify of the ref, unpeeled.
func (g Git) Ref(_ context.Context, ref string) (string, error) {
	out, err := g.git().Output("rev-parse", "--verify", "--quiet", ref)
	if sha := strings.TrimSpace(out); err == nil && sha != "" {
		return sha, nil
	}
	return "", errors.New(ref + " names nothing")
}

// Refs is git for-each-ref under the prefix, an annotated tag's commit its
// *objectname.
func (g Git) Refs(_ context.Context, prefix string) (map[string]string, error) {
	out, err := g.git().Read("for-each-ref", "--format=%(objectname) %(refname) %(*objectname)", prefix)
	if err != nil {
		return nil, err
	}
	refs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			refs[fields[1]] = fields[0]
		}
		if len(fields) == 3 {
			refs[fields[1]+"^{}"] = fields[2]
		}
	}
	return refs, nil
}

// Symref is git symbolic-ref of the ref, quiet.
func (g Git) Symref(_ context.Context, ref string) (string, bool) {
	out, err := g.git().Output("symbolic-ref", "--quiet", ref)
	target := strings.TrimSpace(out)
	return target, err == nil && target != ""
}

// missing is whether git's words say a tree lacks a path: "path '…' does
// not exist in '…'", or "exists on disk, but not in '…'".
func missing(words string) bool {
	return strings.Contains(words, "does not exist in") || strings.Contains(words, "path '")
}
