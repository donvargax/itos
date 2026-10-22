package porttest

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// The fake's commits, index and repository, which change its fields.

// stage sets the index and the work tree apart from HEAD's, so a change to
// one is not read as the other's.
func (v *VCS) stage() {
	v.Work = maps.Clone(v.files())
	v.Index = maps.Clone(v.index())
}

// message is a commit's message as git writes it: the paragraphs a blank
// line apart, else the message; trailing blank lines dropped and one line
// break at its end.
func message(c port.Commit) string {
	text := c.Message
	if len(c.Paragraphs) > 0 {
		text = strings.Join(c.Paragraphs, "\n\n")
	}
	return strings.TrimRight(text, "\n") + "\n"
}

// Commit makes a commit of the index on HEAD, or of HEAD's files with
// Only's set as the work tree has them, those entries of the index set
// alike; nothing to commit fails, as git does.
func (v *VCS) Commit(_ context.Context, c port.Commit, _ port.Talk) error {
	v.stage()
	tree := maps.Clone(v.Index)
	if len(c.Only) > 0 {
		tree = maps.Clone(v.head())
		for _, p := range c.Only {
			for _, t := range []map[string]string{tree, v.Index} {
				if text, ok := v.Work[p]; ok {
					t[p] = text
				} else {
					delete(t, p)
				}
			}
		}
	}
	if len(differ(v.head(), tree)) == 0 {
		return &port.Failed{Command: "git commit", Code: 1, Words: "nothing to commit\n"}
	}
	made := Commit{Name: v.newName(), Message: message(c), Author: "t", Email: "t@t", Date: "1767225660 +0000", Tree: tree}
	if head, ok := v.commit("HEAD"); ok {
		made.Parents = []string{head.Name}
	}
	v.History = append(v.History, made)
	v.moveHead(v.OnBranch, made.Name)
	return nil
}

// Add sets the paths' entries of the index to the work tree's, taking out
// a tracked one it lacks; a path neither has fails.
func (v *VCS) Add(_ context.Context, paths []string, _ port.Talk) error {
	v.stage()
	for _, p := range paths {
		text, ok := v.Work[p]
		_, tracked := v.Index[p]
		switch {
		case ok:
			v.Index[p] = text
		case tracked:
			delete(v.Index, p)
		default:
			return &port.Failed{Command: "git add -- " + p, Code: 128, Words: "fatal: pathspec '" + p + "' did not match any files\n"}
		}
	}
	return nil
}

func (v *VCS) Unstage(_ context.Context, paths []string) error {
	v.stage()
	head := v.head()
	for _, p := range paths {
		if text, ok := head[p]; ok {
			v.Index[p] = text
		} else {
			delete(v.Index, p)
		}
	}
	return nil
}

func (v *VCS) Untrack(_ context.Context, paths []string) error {
	v.stage()
	for _, p := range paths {
		delete(v.Index, p)
	}
	return nil
}

// Restore sets the paths to HEAD's; one HEAD lacks fails, nothing changed.
func (v *VCS) Restore(_ context.Context, paths []string) error {
	head := v.head()
	for _, p := range paths {
		if _, ok := head[p]; !ok {
			return &port.Failed{Command: "git checkout HEAD -- " + p, Code: 1, Words: "error: pathspec '" + p + "' did not match any file(s) known to git\n"}
		}
	}
	v.stage()
	for _, p := range paths {
		v.Index[p], v.Work[p] = head[p], head[p]
	}
	return nil
}

// at is where in History the commit rev names is.
func (v VCS) where(rev string) (int, bool) {
	c, ok := v.commit(rev)
	if !ok {
		return 0, false
	}
	return slices.IndexFunc(v.History, func(h Commit) bool { return h.Name == c.Name }), true
}

func (v *VCS) Note(_ context.Context, rev, text string) error {
	i, ok := v.where(rev)
	if !ok {
		return errors.New(rev + " names no commit")
	}
	v.History = slices.Clone(v.History)
	v.History[i].Note = strings.TrimRight(text, "\n") + "\n"
	return nil
}

func (v *VCS) CommitFile(_ context.Context, parent, path, text, message, name, email string) (string, error) {
	made := Commit{Name: v.newName(), Message: strings.TrimRight(message, "\n") + "\n", Author: name, Email: email,
		Date: "1767225660 +0000", Tree: map[string]string{}}
	if parent != "" {
		c, ok := v.commit(parent)
		if !ok {
			return "", errors.New(parent + " names no commit")
		}
		made.Parents, made.Tree = []string{c.Name}, maps.Clone(c.Tree)
	}
	made.Tree[path] = text
	v.History = append(v.History, made)
	return made.Name, nil
}

// Init locates the folder at Root as the top of a repository, when it is in
// none.
func (v *VCS) Init(context.Context) error {
	if _, ok := v.Located[""]; !ok {
		v.Located = maps.Clone(v.Located)
		if v.Located == nil {
			v.Located = map[string]port.Location{}
		}
		v.Located[""] = port.Location{Top: v.Root, GitDir: ".git", Common: ".git"}
	}
	return nil
}

// NotIgnored is whether the path is in a repository, within the work tree
// and not among Ignored.
func (v VCS) NotIgnored(_ context.Context, path string) bool {
	_, ok := v.Located[""]
	return ok && path != ".." && !strings.HasPrefix(path, "../") && !slices.Contains(v.Ignored, path)
}
