package porttest

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// Commit is a commit of the fake's history: its name and its parents, the
// first first; its message, its itos note and its author's name, email and
// date, as Entry gives them; and its tree, each file's content by path.
type Commit struct {
	Name    string
	Parents []string
	Message string
	Note    string
	Author  string
	Email   string
	Date    string
	Tree    map[string]string
}

// VCS is the repository in memory. Root is the folder the test stands in,
// absolute; Located is what Locate gives from each folder, a folder it
// lacks being in no repository. History is every commit, oldest first, as
// git lists a range whose commits were made in that order; Named names
// commits, or tags of Tags, by full ref name (HEAD, refs/heads/…,
// refs/tags/…), those under refs/remotes/ being the remote branches;
// IsShallow is Shallow's answer.
//
// The work tree's state: OnBranch is the branch HEAD is on, "" when it is
// detached; Index and Work are the index's and the work tree's files,
// HEAD's tree and the index's when nil, a file of Work the index lacks
// being untracked; Unmerged are the paths left in conflict, MergeHead the
// commits a merge in progress brings and InRebase whether a rebase is in
// progress. Comment is core.commentChar, "" when it is not set.
//
// A merge's own paths are those whose content is no parent's, which git
// tells by hunks; the contract's merges tell them apart alike.
//
// Tags are annotated tags' objects, each the commit it names; a ref may
// name one. Remote are the remotes by name, and Tracking each branch's
// upstream as its config sets it, its remote and the remote's ref. Ignored
// are the paths the work tree ignores.
//
// The zero VCS is a folder in no repository. The commands that change the
// repository (committing, the remote's, Init) change its fields, so a test
// holds a *VCS; those that read it change nothing.
type VCS struct {
	Root      string
	Located   map[string]port.Location
	History   []Commit
	Named     map[string]string
	Tags      map[string]string
	IsShallow bool

	OnBranch    string
	Index, Work map[string]string
	Unmerged    []string
	MergeHead   []string
	InRebase    bool
	Comment     string
	Ignored     []string

	Remote   map[string]*Remote
	Tracking map[string][2]string
}

// Remote is a remote of the fake: its address and its refs, each naming a
// commit of the fake's History or an annotated tag of its Tags; Down is a
// remote out of reach.
type Remote struct {
	URL  string
	Refs map[string]string
	Down bool
}

var _ port.VCS = &VCS{}

var errNoRepository = errors.New("not a git repository")

// Folder is Root.
func (v VCS) Folder(context.Context) string { return v.Root }

// Locate is Located[dir].
func (v VCS) Locate(_ context.Context, dir string) (port.Location, error) {
	l, ok := v.Located[dir]
	if !ok {
		return port.Location{}, errNoRepository
	}
	return l, nil
}

// Find is Located[dir] made absolute from Root, when it has a top.
func (v VCS) Find(_ context.Context, dir string) (port.Location, bool) {
	l, ok := v.Located[dir]
	if !ok || l.Top == "" {
		return port.Location{}, false
	}
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return filepath.Clean(p)
		}
		return filepath.Join(v.Root, dir, p)
	}
	return port.Location{Top: l.Top, GitDir: abs(l.GitDir), Common: abs(l.Common)}, true
}

// commit is the commit of History rev names: its own name, a full ref
// name, or a tag's, a branch's or a remote branch's short one; an
// annotated tag names its commit.
func (v VCS) commit(rev string) (Commit, bool) {
	name := rev
	for _, ref := range []string{rev, "refs/tags/" + rev, "refs/heads/" + rev, "refs/remotes/" + rev} {
		if sha, ok := v.Named[ref]; ok {
			name = sha
			break
		}
	}
	if peeled, ok := v.Tags[name]; ok {
		name = peeled
	}
	i := slices.IndexFunc(v.History, func(c Commit) bool { return c.Name == name })
	if i < 0 {
		return Commit{}, false
	}
	return v.History[i], true
}

// Resolve is the name of the commit rev names.
func (v VCS) Resolve(_ context.Context, rev string) (string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return "", errors.New(rev + " names no commit")
	}
	return c.Name, nil
}

func (v VCS) Shallow(context.Context) bool { return v.IsShallow }

// reach are the commits of revs and their ancestors, by name; not ok when
// a revision names no commit.
func (v VCS) reach(revs ...string) (map[string]bool, bool) {
	seen := map[string]bool{}
	var walk func(Commit)
	walk = func(c Commit) {
		if seen[c.Name] {
			return
		}
		seen[c.Name] = true
		for _, p := range c.Parents {
			if parent, ok := v.commit(p); ok {
				walk(parent)
			}
		}
	}
	for _, rev := range revs {
		c, ok := v.commit(rev)
		if !ok {
			return nil, false
		}
		walk(c)
	}
	return seen, true
}

func (v VCS) IsAncestor(_ context.Context, ancestor, rev string) bool {
	a, ok := v.commit(ancestor)
	if !ok {
		return false
	}
	reached, ok := v.reach(rev)
	return ok && reached[a.Name]
}

func (v VCS) Parent(_ context.Context, rev string) string {
	if c, ok := v.commit(rev); ok && len(c.Parents) > 0 {
		return c.Parents[0]
	}
	return port.EmptyTree
}

func (v VCS) Parents(_ context.Context, rev string) ([]string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return nil, errors.New(rev + " names no commit")
	}
	return slices.Clone(c.Parents), nil
}

func (v VCS) FirstParents(_ context.Context, rev string, skip, most int) ([]string, error) {
	c, ok := v.commit(rev)
	if !ok {
		return nil, errors.New(rev + " names no commit")
	}
	var shas []string
	for {
		if skip > 0 {
			skip--
		} else if len(shas) < most {
			shas = append(shas, c.Name)
		}
		if len(shas) == most || len(c.Parents) == 0 {
			return shas, nil
		}
		c, _ = v.commit(c.Parents[0])
	}
}

// remotes are the remote branches' commits.
func (v VCS) remotes() []string {
	var tips []string
	for ref, sha := range v.Named {
		if strings.HasPrefix(ref, "refs/remotes/") {
			tips = append(tips, sha)
		}
	}
	return tips
}

// within are the commits of r, by name: those To reaches and the start and
// Since do not; not ok when an end names no commit.
func (v VCS) within(r port.Range) (map[string]bool, bool) {
	in, ok := v.reach(r.To)
	if !ok {
		return nil, false
	}
	var out []string
	switch r.From {
	case "":
	case port.Unpushed:
		out = v.remotes()
	default:
		out = []string{r.From}
	}
	if r.Since != "" {
		out = append(out, r.Since)
	}
	excluded, ok := v.reach(out...)
	if !ok {
		return nil, false
	}
	for name := range excluded {
		delete(in, name)
	}
	return in, true
}

func (v VCS) Commits(_ context.Context, r port.Range) ([]string, error) {
	in, ok := v.within(r)
	if !ok {
		return nil, errors.New("the range cannot be read")
	}
	var shas []string
	for _, c := range v.History {
		if in[c.Name] {
			shas = append(shas, c.Name)
		}
	}
	return shas, nil
}

func (v VCS) Readable(_ context.Context, r port.Range) bool {
	_, ok := v.within(r)
	return ok
}

// UnpushedBase is to when nothing is unpushed, else the one commit outside
// the unpushed ones that they grow from and no other such commit has.
func (v VCS) UnpushedBase(ctx context.Context, to string) string {
	bases, adds := v.UnpushedBases(ctx, to)
	switch {
	case !adds:
		return to
	case len(bases) == 1:
		return bases[0]
	}
	return ""
}

// UnpushedBases are the commits outside the unpushed ones that they grow
// from and no other such commit has, and whether any is unpushed.
func (v VCS) UnpushedBases(_ context.Context, to string) ([]string, bool) {
	in, ok := v.within(port.Range{From: port.Unpushed, To: to})
	if !ok {
		return nil, true
	}
	var bounds []string
	for _, c := range v.History {
		if !in[c.Name] {
			continue
		}
		for _, p := range c.Parents {
			if !in[p] && !slices.Contains(bounds, p) {
				bounds = append(bounds, p)
			}
		}
	}
	var independent []string
	for _, b := range bounds {
		below := false
		for _, other := range bounds {
			reached, _ := v.reach(other)
			below = below || (other != b && reached[b])
		}
		if !below {
			independent = append(independent, b)
		}
	}
	return independent, len(in) > 0
}
