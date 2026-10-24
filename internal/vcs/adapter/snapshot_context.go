package adapter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// unborn is the head Head gives when HEAD's branch has no commit yet.
const unborn = "unborn"

// Head is HEAD's commit, or "unborn" when its branch has no commit yet. A HEAD
// that names neither is an error.
func (g Git) Head(context.Context) (string, error) {
	out, err := g.git().Output("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if sha := strings.TrimSpace(out); err == nil && sha != "" {
		return sha, nil
	}
	if g.git().Succeeds("symbolic-ref", "--quiet", "HEAD") {
		return unborn, nil
	}
	return "", errors.New("HEAD names no commit")
}

// gitContext is what a task snapshot takes of the source's git state, read
// once when the snapshot is made: HEAD as the folder itos runs in has it (the
// ref it is on, else the commit it is detached at; a linked worktree's own),
// every ref git for-each-ref lists there, and the config of the remotes and
// of the upstream of HEAD's branch.
type gitContext struct {
	branch, head string
	refs         []contextRef
	config       [][2]string
}

// contextRef is a ref with the commit or object it names and, for a symbolic
// one (refs/remotes/origin/HEAD), the ref it points at.
type contextRef struct{ name, object, symref string }

// sourceContext reads the source's git context. A context that cannot be read
// is an error, so that a snapshot with a wrong HEAD or none is never made.
func (g Git) sourceContext() (c gitContext, err error) {
	source := g.git()
	// A detached HEAD is symbolic-ref's exit 1.
	branch, _ := source.Output("symbolic-ref", "--quiet", "HEAD")
	c.branch = strings.TrimSpace(branch)
	head, _ := source.Output("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if c.head = strings.TrimSpace(head); c.head == "" && c.branch == "" {
		return c, errors.New("HEAD names no commit")
	}
	refs, err := source.Output("for-each-ref", "--format=%(objectname)%00%(refname)%00%(symref)")
	if err != nil {
		return c, fmt.Errorf("git for-each-ref: %w", err)
	}
	for _, line := range strings.Split(refs, "\n") {
		if fields := strings.Split(line, "\x00"); len(fields) == 3 {
			c.refs = append(c.refs, contextRef{name: fields[1], object: fields[0], symref: fields[2]})
		}
	}
	sort.Slice(c.refs, func(i, j int) bool { return c.refs[i].name < c.refs[j].name })
	config, err := source.Output("config", "--local", "--null", "--get-regexp", `^(remote|branch)\.`)
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return c, fmt.Errorf("git config: %w", err)
	}
	for _, entry := range strings.Split(config, "\x00") {
		key, value, found := strings.Cut(entry, "\n")
		if !found {
			// A key written without a value is true for git.
			value = "true"
		}
		if ownedByContext(key, c.branch) {
			c.config = append(c.config, [2]string{key, value})
		}
	}
	return c, nil
}

// ownedByContext is whether a config key is what a snapshot takes: any
// remote.<name>.* key, and branch.<name>.remote and branch.<name>.merge of
// the branch HEAD is on.
func ownedByContext(key, branch string) bool {
	if strings.HasPrefix(key, "remote.") {
		return true
	}
	name, ok := strings.CutPrefix(branch, "refs/heads/")
	if !ok {
		return false
	}
	rest, ok := strings.CutPrefix(key, "branch."+name+".")
	return ok && (rest == "remote" || rest == "merge")
}

// copyTo gives the snapshot in dir the source's git context, run being how it
// runs git. The refs
// are written as one packed-refs file, each at the value it has now, so that
// a snapshot of a repository with thousands of tags makes no file for each;
// what a check writes after goes to loose files of the snapshot's own.
func (c gitContext) copyTo(dir string, run func(args ...string) error) error {
	if len(c.refs) > 0 {
		var packed strings.Builder
		packed.WriteString("# pack-refs with: sorted \n")
		for _, r := range c.refs {
			packed.WriteString(r.object + " " + r.name + "\n")
		}
		if err := os.WriteFile(filepath.Join(dir, ".git", "packed-refs"), []byte(packed.String()), 0o600); err != nil {
			return err
		}
	}
	for _, r := range c.refs {
		if r.symref == "" {
			continue
		}
		if err := run("-C", dir, "symbolic-ref", r.name, r.symref); err != nil {
			return err
		}
	}
	head := []string{"-C", dir, "symbolic-ref", "HEAD", c.branch}
	if c.branch == "" {
		head = []string{"-C", dir, "update-ref", "--no-deref", "HEAD", c.head}
	}
	if err := run(head...); err != nil {
		return err
	}
	for _, entry := range c.config {
		if err := run("-C", dir, "config", "--local", "--add", entry[0], entry[1]); err != nil {
			return err
		}
	}
	return nil
}
