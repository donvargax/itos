package porttest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// The fake's remotes.

func (v VCS) Upstream(_ context.Context, branch string) (string, string, bool) {
	if up, ok := v.Tracking[branch]; ok {
		return up[0], up[1], true
	}
	return "origin", "refs/heads/" + branch, false
}

func (v VCS) Remotes(context.Context) ([]string, error) {
	if _, ok := v.Located[""]; !ok {
		return nil, errNoRepository
	}
	return slices.Sorted(maps.Keys(v.Remote)), nil
}

func (v VCS) RemoteURL(_ context.Context, remote string) (string, error) {
	if r, ok := v.Remote[remote]; ok {
		return r.URL, nil
	}
	return "", errors.New("no remote " + remote)
}

// reach is the remote by name: a Failed error, as git's would read, when
// there is none or it is out of reach.
func (v VCS) reachRemote(command, remote string) (*Remote, error) {
	r, ok := v.Remote[remote]
	switch {
	case !ok:
		return nil, &port.Failed{Command: command, Code: 128, Failure: port.NoRepository,
			Words: "fatal: '" + remote + "' does not appear to be a git repository\n"}
	case r.Down:
		return nil, &port.Failed{Command: command, Code: 128, Failure: port.Unreachable,
			Words: "fatal: unable to access '" + r.URL + "': Could not resolve host\n"}
	}
	return r, nil
}

// matches is whether a ref is one of the patterns as git ls-remote matches
// them: the whole ref, or its end from a slash.
func matches(ref string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ref == p || strings.HasSuffix(ref, "/"+p) {
			return true
		}
	}
	return false
}

func (v VCS) RemoteRefs(_ context.Context, remote string, refs []string, _ port.Talk) (map[string]string, error) {
	r, err := v.reachRemote("git ls-remote "+remote, remote)
	if err != nil {
		return nil, err
	}
	listed := maps.Clone(r.Refs)
	if main, ok := r.Refs["refs/heads/main"]; ok {
		listed["HEAD"] = main
	}
	return v.refsOf(listed, func(ref string) bool { return matches(ref, refs) }), nil
}

// Fetch sets the refspec's destination, a + before it dropped, to the
// remote's commit; with no refspec, refs/remotes/<remote>/<branch> to each
// branch's.
func (v *VCS) Fetch(_ context.Context, remote, refspec string, _ port.Talk) (string, error) {
	command := "git fetch " + remote + " " + refspec
	r, err := v.reachRemote(command, remote)
	if err != nil {
		return "", err
	}
	v.Named = maps.Clone(v.Named)
	if refspec == "" {
		for ref, sha := range r.Refs {
			if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
				v.Named["refs/remotes/"+remote+"/"+branch] = sha
			}
		}
		return "", nil
	}
	src, dst, _ := strings.Cut(strings.TrimPrefix(refspec, "+"), ":")
	sha, ok := r.Refs[src]
	if !ok {
		return "", &port.Failed{Command: command, Code: 128, Words: "fatal: couldn't find remote ref " + src + "\n"}
	}
	if dst != "" {
		v.Named[dst] = sha
	}
	c, _ := v.commit(sha)
	return c.Name, nil
}

// Push sets each refspec's destination on the remote to its source's
// object, a branch only forward and a tag only where it has none, except
// the ref a lease names, which also may go where it is at the commit the
// lease expects: a refusal, as git words it, fails the push, and the refs it
// set stay unless it is atomic.
func (v *VCS) Push(_ context.Context, remote string, specs []string, p port.Pushing, _ port.Talk) error {
	command := "git push " + remote + " " + strings.Join(specs, " ")
	r, err := v.reachRemote(command, remote)
	if err != nil {
		return err
	}
	refs := maps.Clone(r.Refs)
	var refused []string
	for _, spec := range specs {
		src, dst, _ := strings.Cut(spec, ":")
		object, ok := v.Named[src]
		if !ok {
			c, found := v.commit(src)
			if !found {
				return &port.Failed{Command: command, Code: 1, Words: "error: src refspec " + src + " does not match any\n"}
			}
			object = c.Name
		}
		at, there := refs[dst]
		forward := !there || at == object || !strings.HasPrefix(dst, "refs/tags/") && v.IsAncestor(context.Background(), at, object)
		forward = forward || p.Lease != nil && p.Lease.Ref == dst && p.Lease.Expect == at
		if !forward {
			refused = append(refused, dst)
			continue
		}
		refs[dst] = object
		if !p.Atomic {
			r.Refs = maps.Clone(r.Refs)
			r.Refs[dst] = object
		}
	}
	if len(refused) > 0 {
		return &port.Failed{Command: command, Code: 1, Failure: port.Refused,
			Words: " ! [rejected] " + strings.Join(refused, ", ") + "\nerror: failed to push some refs\n"}
	}
	r.Refs = refs
	return nil
}

// Rebase replays the commits of HEAD that onto lacks, oldest first, onto
// it, each against its first parent: a path whose content a commit changes
// where the replay has moved it on is a conflict, which stops the rebase
// there, HEAD detached at the commits replayed so far. HEAD with onto among
// its commits is left as it is.
func (v *VCS) Rebase(ctx context.Context, onto string, _ bool, _ port.Talk) error {
	base, ok := v.commit(onto)
	if !ok {
		return &port.Failed{Command: "git rebase " + onto, Code: 128, Words: "fatal: invalid upstream '" + onto + "'\n"}
	}
	if v.IsAncestor(ctx, onto, "HEAD") {
		return nil
	}
	replayed, _ := v.Log(ctx, port.Range{From: onto, To: "HEAD"})
	branch, at, tree := v.OnBranch, base.Name, base.Tree
	for _, e := range replayed {
		c, _ := v.commit(e.SHA)
		next, conflicts := v.carry(ctx, c, tree)
		if len(conflicts) > 0 {
			v.Named = maps.Clone(v.Named)
			v.Named["HEAD"], v.OnBranch, v.InRebase, v.Unmerged = at, "", true, conflicts
			v.Index, v.Work = nil, nil
			return &port.Failed{Command: "git rebase " + onto, Code: 1, Words: "CONFLICT (content): Merge conflict in " + conflicts[0] + "\n"}
		}
		made := c
		made.Name, made.Parents, made.Tree = v.newName(), []string{at}, next
		v.History = append(v.History, made)
		at, tree = made.Name, next
	}
	v.moveHead(branch, at)
	v.Index, v.Work = nil, nil
	return nil
}

// carry is the tree the change a commit makes against its first parent
// makes of tree, and the paths it conflicts in: a path whose content the
// commit changes where tree has moved it on.
func (v VCS) carry(ctx context.Context, c Commit, tree map[string]string) (next map[string]string, conflicts []string) {
	parent, _ := v.tree(v.Parent(ctx, c.Name))
	next = maps.Clone(tree)
	for _, p := range differ(parent, c.Tree) {
		mine, in := c.Tree[p]
		now, there := tree[p]
		was, had := parent[p]
		switch {
		case there == had && now == was, there == in && now == mine:
			if in {
				next[p] = mine
			} else {
				delete(next, p)
			}
		default:
			conflicts = append(conflicts, p)
		}
	}
	return next, conflicts
}

// CherryPick makes a commit on onto with the commit's author, date and
// message and the tree carry gives, without a note; a Conflicts error, and
// nothing made, when it does not apply.
func (v *VCS) CherryPick(ctx context.Context, commit, onto string) (string, error) {
	c, ok := v.commit(commit)
	base, found := v.commit(onto)
	if !ok || !found {
		return "", &port.Failed{Command: "git merge-tree " + commit + " " + onto, Code: 128, Words: "fatal: not a valid object name\n"}
	}
	next, conflicts := v.carry(ctx, c, base.Tree)
	if len(conflicts) > 0 {
		return "", &port.Conflicts{Paths: conflicts}
	}
	made := c
	made.Name, made.Parents, made.Tree, made.Note = v.newName(), []string{base.Name}, next, ""
	v.History = append(v.History, made)
	return made.Name, nil
}

// Merge makes a commit on HEAD with rev as its second parent and the message,
// its tree each path as the side that changed it since their newest common
// commit has it: a path both changed apart is a conflict, which leaves the
// merge in progress and makes nothing. HEAD that has rev is left as it is.
func (v *VCS) Merge(ctx context.Context, rev, message string, _ bool, _ port.Talk) error {
	theirs, ok := v.commit(rev)
	ours, found := v.commit("HEAD")
	if !ok || !found {
		return &port.Failed{Command: "git merge " + rev, Code: 1, Words: "merge: " + rev + " - not something we can merge\n"}
	}
	if v.IsAncestor(ctx, theirs.Name, ours.Name) {
		return nil
	}
	mine, _ := v.reach(ours.Name)
	yours, _ := v.reach(theirs.Name)
	var base map[string]string
	for _, c := range v.History {
		if mine[c.Name] && yours[c.Name] {
			base = c.Tree
		}
	}
	mineChanged, yoursChanged, apart := differ(base, ours.Tree), differ(base, theirs.Tree), differ(ours.Tree, theirs.Tree)
	tree := map[string]string{}
	var conflicts []string
	for _, p := range union(base, ours.Tree, theirs.Tree) {
		side := ours.Tree
		switch {
		case !slices.Contains(mineChanged, p):
			side = theirs.Tree
		case slices.Contains(yoursChanged, p) && slices.Contains(apart, p):
			conflicts = append(conflicts, p)
			continue
		}
		if text, there := side[p]; there {
			tree[p] = text
		}
	}
	if len(conflicts) > 0 {
		v.MergeHead, v.Unmerged = []string{theirs.Name}, conflicts
		return &port.Failed{Command: "git merge " + rev, Code: 1, Words: "CONFLICT (content): Merge conflict in " + conflicts[0] + "\n"}
	}
	made := Commit{Name: v.newName(), Parents: []string{ours.Name, theirs.Name}, Message: message + "\n", Tree: tree}
	v.History = append(v.History, made)
	v.moveHead(v.OnBranch, made.Name)
	v.Index, v.Work = nil, nil
	return nil
}

// Reset moves HEAD, and its branch, to the commit, and drops the index and
// the work tree's changes.
func (v *VCS) Reset(_ context.Context, rev string) error {
	c, ok := v.commit(rev)
	if !ok {
		return &port.Failed{Command: "git reset --hard " + rev, Code: 128, Words: "fatal: ambiguous argument '" + rev + "'\n"}
	}
	v.moveHead(v.OnBranch, c.Name)
	v.Index, v.Work = nil, nil
	return nil
}

// newName is a name no commit of the history has.
func (v VCS) newName() string {
	return fmt.Sprintf("n-%d", len(v.History))
}

// moveHead sets HEAD, and the branch when it is on one, to the commit.
func (v *VCS) moveHead(branch, commit string) {
	v.Named = maps.Clone(v.Named)
	v.Named["HEAD"] = commit
	if branch != "" {
		v.Named["refs/heads/"+branch] = commit
	}
}
