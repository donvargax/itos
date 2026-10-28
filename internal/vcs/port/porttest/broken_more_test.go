package porttest

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// The broken fake's files at a tree, patches, refs, remotes, commits and
// repository.

func (b broken) File(ctx context.Context, tree, path string) (string, bool, error) {
	switch {
	case b.fault == "file of the first parent" && tree == "m":
		tree = b.VCS.Parent(ctx, tree)
	case b.fault == "file of the index as HEAD" && tree == port.Index:
		tree = "HEAD"
	}
	text, ok, err := b.VCS.File(ctx, tree, path)
	switch {
	case b.fault == "file deleted as empty" && err == nil && !ok:
		return "", true, nil
	case b.fault == "file of nothing" && err != nil:
		return "", false, nil
	}
	return text, ok, err
}

func (b broken) Files(ctx context.Context, tree, dir string) ([]string, error) {
	switch b.fault {
	case "files of the index as HEAD":
		if tree == port.Index {
			tree = "HEAD"
		}
	case "files unfiltered":
		dir = ""
	case "files of nothing":
		if _, ok := b.at(tree); !ok {
			return nil, nil
		}
	}
	return b.VCS.Files(ctx, tree, dir)
}

func (b broken) Texts(ctx context.Context, tree string, paths []string) (map[string]string, error) {
	switch b.fault {
	case "texts of a missing file left out":
		files, _ := b.at(tree)
		var there []string
		for _, p := range paths {
			if _, ok := files[p]; ok {
				there = append(there, p)
			}
		}
		paths = there
	case "texts of the head":
		tree = "HEAD"
	}
	texts, err := b.VCS.Texts(ctx, tree, paths)
	if b.fault == "texts trimmed" {
		for p, text := range texts {
			texts[p] = strings.TrimRight(text, "\n")
		}
	}
	return texts, err
}

func (b broken) Patch(ctx context.Context, rev string) (string, error) {
	c, ok := b.commit(rev)
	switch {
	case b.fault == "patch against nothing" && ok:
		return patch(nil, c.Tree), nil
	case b.fault == "patch of nothing" && !ok:
		return "", nil
	}
	return b.VCS.Patch(ctx, rev)
}

func (b broken) Shown(ctx context.Context, revs []string, notes bool) (string, error) {
	switch b.fault {
	case "shown notes always":
		notes = true
	case "shown without notes":
		notes = false
	case "shown of nothing":
		var there []string
		for _, rev := range revs {
			if _, ok := b.commit(rev); ok {
				there = append(there, rev)
			}
		}
		revs = there
	}
	return b.VCS.Shown(ctx, revs, notes)
}

func (b broken) Ref(ctx context.Context, ref string) (string, error) {
	sha, err := b.VCS.Ref(ctx, ref)
	switch {
	case b.fault == "ref short" && err == nil:
		return sha[:len(sha)-1], nil
	case b.fault == "ref peeled" && err == nil:
		return b.VCS.Resolve(ctx, ref)
	case b.fault == "ref of nothing" && err != nil:
		return "", nil
	}
	return sha, err
}

func (b broken) Symref(ctx context.Context, ref string) (string, bool) {
	target, ok := b.VCS.Symref(ctx, ref)
	switch {
	case b.fault == "symref never":
		return "", false
	case b.fault == "symref plain" && !ok && ref == "refs/heads/main":
		return ref, true
	case b.fault == "symref short" && ok:
		return strings.TrimPrefix(target, "refs/remotes/"), true
	case b.fault == "symref of nothing" && !ok && ref != "refs/heads/main":
		return ref, true
	}
	return target, ok
}

func (b broken) Refs(ctx context.Context, prefix string) (map[string]string, error) {
	refs, err := b.VCS.Refs(ctx, prefix)
	switch b.fault {
	case "refs unpeeled":
		maps.DeleteFunc(refs, func(ref, _ string) bool { return strings.HasSuffix(ref, "^{}") })
	case "refs unfiltered":
		return b.VCS.Refs(ctx, "")
	}
	return refs, err
}

func (b broken) Upstream(ctx context.Context, branch string) (string, string, bool) {
	remote, ref, set := b.VCS.Upstream(ctx, branch)
	switch b.fault {
	case "upstream always set":
		return remote, ref, true
	case "upstream never set":
		return "origin", "refs/heads/" + branch, false
	}
	return remote, ref, set
}

func (b broken) Remotes(ctx context.Context) ([]string, error) {
	remotes, err := b.VCS.Remotes(ctx)
	switch {
	case b.fault == "remotes none":
		return nil, err
	case b.fault == "remotes outside fine" && err != nil:
		return nil, nil
	}
	if b.fault == "remotes of remote branches" && len(b.Remote) == 0 {
		for ref := range b.Named {
			if strings.HasPrefix(ref, "refs/remotes/origin/") {
				return []string{"origin"}, nil
			}
		}
	}
	return remotes, err
}

func (b broken) RemoteURL(ctx context.Context, remote string) (string, error) {
	url, err := b.VCS.RemoteURL(ctx, remote)
	switch {
	case b.fault == "remote url of nothing" && err != nil:
		return "", nil
	case b.fault == "remote url empty":
		return "", err
	}
	return url, err
}

func (b broken) RemoteRefs(ctx context.Context, remote string, refs []string, talk port.Talk) (map[string]string, error) {
	found, err := b.VCS.RemoteRefs(ctx, remote, refs, talk)
	var failed *port.Failed
	switch {
	case b.fault == "remote refs failing" && refs == nil:
		return nil, errBroken
	case b.fault == "down reached as missing" && errors.As(err, &failed) && failed.Failure == port.Unreachable:
		failed.Failure = port.NoRepository
	case b.fault == "remote refs unfiltered":
		return b.VCS.RemoteRefs(ctx, remote, nil, talk)
	case b.fault == "remote refs failing plainly" && err != nil:
		return nil, errBroken
	}
	return found, err
}

func (b broken) Fetch(ctx context.Context, remote, refspec string, talk port.Talk) (string, error) {
	switch {
	case b.fault == "fetch no destination":
		refspec, _, _ = strings.Cut(strings.TrimPrefix(refspec, "+"), ":")
	case b.fault == "fetch of a missing branch" && strings.HasSuffix(refspec, "nope"):
		return "", nil
	case b.fault == "fetch without branches" && refspec == "":
		return "", nil
	case b.fault == "fetch of branches fails" && refspec == "":
		return "", errBroken
	}
	sha, err := b.VCS.Fetch(ctx, remote, refspec, talk)
	if b.fault == "fetch gives nothing" {
		sha = ""
	}
	var failed *port.Failed
	if b.fault == "fetch failure unknown" && errors.As(err, &failed) {
		failed.Failure = port.Other
	}
	return sha, err
}

func (b broken) Push(ctx context.Context, remote string, specs []string, p port.Pushing, talk port.Talk) error {
	switch {
	case b.fault == "push lease ignored":
		p.Lease = nil
	case b.fault == "push forced" || b.fault == "push lease forces" && p.Lease != nil:
		r, ok := b.Remote[remote]
		if !ok {
			break
		}
		for _, spec := range specs {
			src, dst, _ := strings.Cut(spec, ":")
			r.Refs[dst] = src
		}
		return nil
	case b.fault == "push not atomic":
		p.Atomic = false
	case b.fault == "push nowhere fine" && b.Remote[remote] == nil:
		return nil
	case b.fault == "push nothing":
		return nil
	case b.fault == "push of nothing fine" && strings.HasPrefix(specs[0], "nope:"):
		return nil
	}
	return b.VCS.Push(ctx, remote, specs, p, talk)
}

func (b broken) Rebase(ctx context.Context, onto string, quiet bool, talk port.Talk) error {
	before := *b.VCS
	_, known := b.commit(onto)
	switch {
	case b.fault == "rebase of nothing fine" && !known:
		return nil
	case b.fault == "rebase resets" && b.IsAncestor(ctx, onto, "HEAD"):
		c, _ := b.commit(onto)
		b.moveHead(b.OnBranch, c.Name)
		return nil
	}
	err := b.VCS.Rebase(ctx, onto, quiet, talk)
	switch {
	case b.fault == "rebase refuses" && err == nil:
		return errBroken
	case b.fault == "rebase stops nothing" && err != nil:
		*b.VCS = before
		return nil
	case b.fault == "rebase detaches" && err == nil:
		b.OnBranch = ""
	}
	return err
}

func (b broken) Commit(ctx context.Context, c port.Commit, talk port.Talk) error {
	switch b.fault {
	case "commit of only refused":
		if len(c.Only) > 0 {
			return errBroken
		}
	case "commit of only kept":
		c.Only = slices.DeleteFunc(slices.Clone(c.Only), func(p string) bool { _, ok := b.files()[p]; return !ok })
	case "commit of everything":
		c.Only = nil
	case "commit paragraphs joined":
		if len(c.Paragraphs) > 0 {
			c.Message, c.Paragraphs = strings.Join(c.Paragraphs, "\n"), nil
		}
	}
	err := b.VCS.Commit(ctx, c, talk)
	if b.fault == "commit of nothing" && err != nil {
		return nil
	}
	return err
}

func (b broken) Add(ctx context.Context, paths []string, talk port.Talk) error {
	if b.fault == "add of a deletion kept" {
		paths = slices.DeleteFunc(slices.Clone(paths), func(p string) bool { _, ok := b.files()[p]; return !ok && b.Tracked(ctx, p) })
	}
	err := b.VCS.Add(ctx, paths, talk)
	if b.fault == "add of nothing" && err != nil {
		return nil
	}
	return err
}

func (b broken) Unstage(ctx context.Context, paths []string) error {
	if b.fault == "unstage untracks" {
		return b.VCS.Untrack(ctx, paths)
	}
	return b.VCS.Unstage(ctx, paths)
}

func (b broken) Untrack(ctx context.Context, paths []string) error {
	err := b.VCS.Untrack(ctx, paths)
	if b.fault == "untrack deletes" {
		for _, p := range paths {
			delete(b.Work, p)
		}
	}
	return err
}

func (b broken) Restore(ctx context.Context, paths []string) error {
	switch b.fault {
	case "restore index only":
		return b.VCS.Unstage(ctx, paths)
	case "restore of nothing":
		var there []string
		for _, p := range paths {
			if _, ok := b.head()[p]; ok {
				there = append(there, p)
			}
		}
		paths = there
	}
	return b.VCS.Restore(ctx, paths)
}

func (b broken) Note(ctx context.Context, rev, text string) error {
	switch {
	case b.fault == "note appended":
		if c, ok := b.commit(rev); ok {
			text = c.Note + text
		}
	case b.fault == "note of nothing":
		if _, ok := b.commit(rev); !ok {
			return nil
		}
	}
	return b.VCS.Note(ctx, rev, text)
}

func (b broken) CommitFile(ctx context.Context, parent, path, text, message, name, email string) (string, error) {
	switch b.fault {
	case "commit file of nothing fails":
		if parent == "" {
			return "", errBroken
		}
	case "commit file on HEAD":
		if parent == "" {
			parent = "HEAD"
		}
	case "commit file alone":
		parent = ""
	case "commit file on nothing":
		if _, ok := b.commit(parent); !ok {
			parent = ""
		}
	}
	sha, err := b.VCS.CommitFile(ctx, parent, path, text, message, name, email)
	if b.fault == "commit file moves head" && err == nil {
		b.moveHead(b.OnBranch, sha)
	}
	return sha, err
}

func (b broken) Init(ctx context.Context) error {
	switch b.fault {
	case "init nothing":
		return nil
	case "init fails":
		return errBroken
	}
	return b.VCS.Init(ctx)
}

func (b broken) NotIgnored(ctx context.Context, path string) bool {
	switch b.fault {
	case "ignored nothing":
		return true
	case "ignored untracked":
		_, tracked := b.index()[path]
		return tracked && b.VCS.NotIgnored(ctx, path)
	}
	return b.VCS.NotIgnored(ctx, path)
}

func (b broken) CherryPick(ctx context.Context, commit, onto string) (string, error) {
	if b.fault == "cherry pick onto the head" {
		onto = "HEAD"
	}
	made, err := b.VCS.CherryPick(ctx, commit, onto)
	var conflicts *port.Conflicts
	switch {
	case b.fault == "cherry pick moves head" && err == nil:
		b.moveHead(b.OnBranch, made)
	case b.fault == "cherry pick keeps the note" && err == nil:
		c, _ := b.commit(commit)
		i, _ := b.where(made)
		b.History[i].Note = c.Note
	case b.fault == "cherry pick conflict is a failure" && errors.As(err, &conflicts):
		return "", errBroken
	case b.fault == "cherry pick of nothing conflicts" && err != nil && !errors.As(err, &conflicts):
		return "", &port.Conflicts{Paths: []string{"a.txt"}}
	}
	return made, err
}

func (b broken) Reset(ctx context.Context, rev string) error {
	index, work, main := b.Index, b.Work, b.Named["refs/heads/main"]
	err := b.VCS.Reset(ctx, rev)
	switch {
	case b.fault == "reset of nothing fine" && err != nil:
		return nil
	case b.fault == "reset keeps changes" && err == nil:
		b.Index, b.Work = index, work
	case b.fault == "reset leaves the branch" && err == nil:
		b.Named["refs/heads/main"] = main
	}
	return err
}
