package porttest

import (
	"errors"
	"slices"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// failureOf is why err, a Failed one, says its command failed; ok false
// when err is no Failed.
func failureOf(err error) (port.Failure, bool) {
	var failed *port.Failed
	if !errors.As(err, &failed) {
		return port.Other, false
	}
	return failed.Failure, true
}

// remoteContract holds the upstream, the remote's refs, a fetch, a push,
// a rebase and a merge.
func remoteContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	origin := Layout{Origin: true}
	t.Run("remotes", func(t C) {
		v, _ := open(t, origin)
		ctx := t.Context()
		if remote, ref, set := v.Upstream(ctx, "main"); remote != "origin" || ref != "refs/heads/main" || !set {
			t.Errorf("Upstream(main), which tracks origin's main, is %s %s %v", remote, ref, set)
		}
		if remote, ref, set := v.Upstream(ctx, "theirs"); remote != "origin" || ref != "refs/heads/theirs" || set {
			t.Errorf("Upstream(theirs), which tracks nothing, is %s %s %v", remote, ref, set)
		}
		if got, ok := v.Symref(ctx, "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/main" || !ok {
			t.Errorf("Symref(refs/remotes/origin/HEAD) is %q, %v; want refs/remotes/origin/main", got, ok)
		}
		if got, ok := v.Symref(ctx, "refs/heads/main"); got != "" || ok {
			t.Errorf("Symref(refs/heads/main), no symbolic ref, is %q, %v", got, ok)
		}
		if got, ok := v.Symref(ctx, "refs/remotes/nowhere/HEAD"); got != "" || ok {
			t.Errorf("Symref of no ref is %q, %v", got, ok)
		}
		if got, err := v.Remotes(ctx); !slices.Equal(got, []string{"down", "origin"}) || err != nil {
			t.Errorf("Remotes are %q, %v; want down and origin", got, err)
		}
		if url, err := v.RemoteURL(ctx, "origin"); url == "" || err != nil {
			t.Errorf("RemoteURL(origin) is %q, %v", url, err)
		}
		if url, err := v.RemoteURL(ctx, "nowhere"); err == nil {
			t.Errorf("RemoteURL of no remote is %q", url)
		}
		v, _ = open(t, Layout{Remotes: []string{"b"}})
		if got, err := v.Remotes(ctx); len(got) != 0 || err != nil {
			t.Errorf("Remotes with remote branches and no remote are %q, %v", got, err)
		}
	})
	t.Run("remote refs", func(t C) {
		v, _ := open(t, origin)
		ctx := t.Context()
		b, theirs := sha(t, v, "b"), sha(t, v, "t")
		refs, err := v.RemoteRefs(ctx, "origin", nil, port.Talk{})
		if err != nil || refs["refs/heads/main"] != b || refs["refs/heads/theirs"] != theirs {
			t.Errorf("RemoteRefs(origin) are %q, %v; want main at b %s and theirs at t %s", refs, err, b, theirs)
		}
		refs, err = v.RemoteRefs(ctx, "origin", []string{"refs/heads/main"}, port.Talk{})
		if len(refs) != 1 || refs["refs/heads/main"] != b || err != nil {
			t.Errorf("RemoteRefs(origin, refs/heads/main) are %q, %v; want main alone", refs, err)
		}
		refs, err = v.RemoteRefs(ctx, "nowhere", nil, port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.NoRepository {
			t.Errorf("RemoteRefs of no remote are %q, %v; want no repository", refs, err)
		}
		refs, err = v.RemoteRefs(ctx, "down", nil, port.Talk{NoPrompt: true})
		if f, ok := failureOf(err); !ok || f != port.Unreachable {
			t.Errorf("RemoteRefs of an unreachable remote are %q, %v; want unreachable", refs, err)
		}
	})
	t.Run("fetch", func(t C) {
		v, _ := open(t, origin)
		ctx := t.Context()
		b, theirs := sha(t, v, "b"), sha(t, v, "t")
		if got, err := v.Fetch(ctx, "origin", "refs/heads/theirs", port.Talk{}); got != theirs || err != nil {
			t.Errorf("Fetch(origin, theirs) is %q, %v; want t's %s", got, err, theirs)
		}
		if got, err := v.Fetch(ctx, "origin", "+refs/heads/main:refs/fetched", port.Talk{}); got != b || err != nil {
			t.Errorf("Fetch(origin, main into refs/fetched) is %q, %v; want b's %s", got, err, b)
		}
		if got, err := v.Ref(ctx, "refs/fetched"); got != b || err != nil {
			t.Errorf("Fetch: refs/fetched is %q, %v; want b's %s", got, err, b)
		}
		if got, err := v.Fetch(ctx, "origin", "refs/heads/nope", port.Talk{}); err == nil {
			t.Errorf("Fetch of a branch origin lacks is %q", got)
		}
		_, err := v.Fetch(ctx, "nowhere", "refs/heads/main", port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.NoRepository {
			t.Errorf("Fetch of no remote: %v; want no repository", err)
		}
		if _, err := v.Fetch(ctx, "origin", "", port.Talk{}); err != nil {
			t.Errorf("Fetch(origin) of its branches: %v", err)
		}
		if got, err := v.Resolve(ctx, "refs/remotes/origin/theirs"); got != theirs || err != nil {
			t.Errorf("Fetch(origin) of its branches: origin/theirs is %q, %v; want t's %s", got, err, theirs)
		}
	})
	t.Run("push", func(t C) {
		v, _ := open(t, origin)
		ctx := t.Context()
		m, a := sha(t, v, "m"), sha(t, v, "a")
		mainOf := func() string {
			refs, _ := v.RemoteRefs(ctx, "origin", []string{"refs/heads/main"}, port.Talk{})
			return refs["refs/heads/main"]
		}
		if err := v.Push(ctx, "origin", []string{m + ":refs/heads/main"}, port.Pushing{Quiet: true}, port.Talk{}); err != nil || mainOf() != m {
			t.Errorf("Push(m to main), forward from b: %v, main at %s; want m %s", err, mainOf(), m)
		}
		err := v.Push(ctx, "origin", []string{a + ":refs/heads/main"}, port.Pushing{}, port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.Refused || mainOf() != m {
			t.Errorf("Push(a to main), back from m: %v, main at %s; want refused, at m", err, mainOf())
		}
		b, theirs := sha(t, v, "b"), sha(t, v, "t")
		for _, lease := range []struct {
			name string
			port.Lease
		}{
			{"a stale lease", port.Lease{Ref: "refs/heads/main", Expect: b}},
			{"a lease on theirs", port.Lease{Ref: "refs/heads/theirs", Expect: theirs}},
		} {
			err = v.Push(ctx, "origin", []string{a + ":refs/heads/main"}, port.Pushing{Lease: &lease.Lease}, port.Talk{})
			if f, ok := failureOf(err); !ok || f != port.Refused || mainOf() != m {
				t.Errorf("Push(a to main) with %s: %v, main at %s; want refused, at m", lease.name, err, mainOf())
			}
		}
		err = v.Push(ctx, "origin", []string{a + ":refs/heads/main"}, port.Pushing{Lease: &port.Lease{Ref: "refs/heads/main", Expect: m}}, port.Talk{})
		if err != nil || mainOf() != a {
			t.Errorf("Push(a to main) with its lease: %v, main at %s; want a %s", err, mainOf(), a)
		}
		err = v.Push(ctx, "origin", []string{m + ":refs/heads/main", a + ":refs/heads/theirs"},
			port.Pushing{Atomic: true, Lease: &port.Lease{Ref: "refs/heads/main", Expect: a}}, port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.Refused || mainOf() != a {
			t.Errorf("Push(m to main, a to theirs) with a lease on main: %v, main at %s; want refused, at a", err, mainOf())
		}
		err = v.Push(ctx, "origin", []string{m + ":refs/heads/new", a + ":refs/heads/theirs"}, port.Pushing{Atomic: true}, port.Talk{})
		refs, _ := v.RemoteRefs(ctx, "origin", []string{"refs/heads/new"}, port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.Refused || refs["refs/heads/new"] != "" {
			t.Errorf("Push atomic of new and theirs back: %v, new at %q; want refused, no new", err, refs["refs/heads/new"])
		}
		err = v.Push(ctx, "nowhere", []string{m + ":refs/heads/main"}, port.Pushing{}, port.Talk{})
		if f, ok := failureOf(err); !ok || f != port.NoRepository {
			t.Errorf("Push to no remote: %v; want no repository", err)
		}
		if err := v.Push(ctx, "origin", []string{"nope:refs/heads/x"}, port.Pushing{NoVerify: true}, port.Talk{}); err == nil {
			t.Errorf("Push of no commit: no error")
		}
	})
	t.Run("rebase", func(t C) {
		v, _ := open(t, Layout{Ours: true})
		ctx := t.Context()
		s, c := sha(t, v, "s"), sha(t, v, "c")
		if err := v.Rebase(ctx, "s", true, port.Talk{}); err != nil {
			t.Errorf("Rebase(s) of ours: %v", err)
		}
		if a, ok, _ := v.File(ctx, "HEAD", "a.txt"); v.Branch(ctx) != "ours" || !v.IsAncestor(ctx, s, "HEAD") ||
			v.IsAncestor(ctx, c, "HEAD") || v.Rebasing(ctx) || a != "o\n" || !ok {
			t.Errorf("Rebase(s) of ours: on %q, s an ancestor %v, c %v, rebasing %v, a.txt %q",
				v.Branch(ctx), v.IsAncestor(ctx, s, "HEAD"), v.IsAncestor(ctx, c, "HEAD"), v.Rebasing(ctx), a)
		}
		v, _ = open(t, Layout{Ours: true})
		o := sha(t, v, "o")
		if err := v.Rebase(ctx, "c", true, port.Talk{}); err != nil || sha(t, v, "HEAD") != o {
			t.Errorf("Rebase(c) of ours, which has c: %v, HEAD at %s; want o's %s", err, sha(t, v, "HEAD"), o)
		}
		if err := v.Rebase(ctx, "nope", true, port.Talk{}); err == nil {
			t.Errorf("Rebase of no commit: no error")
		}
		err := v.Rebase(ctx, "t", true, port.Talk{})
		if _, ok := failureOf(err); !ok || !v.Rebasing(ctx) || !slices.Equal(v.Conflicted(ctx), []string{"a.txt"}) {
			t.Errorf("Rebase(t) of ours, both changing a.txt: %v, rebasing %v, conflicted %q", err, v.Rebasing(ctx), v.Conflicted(ctx))
		}
	})
	t.Run("merge", func(t C) {
		v, _ := open(t, Layout{Ours: true})
		ctx := t.Context()
		s, o := sha(t, v, "s"), sha(t, v, "o")
		if err := v.Merge(ctx, "s", "merge s", true, true, port.Talk{}); err != nil {
			t.Fatalf("Merge(s) into ours: %v", err)
		}
		head := sha(t, v, "HEAD")
		parents, _ := v.Parents(ctx, head)
		shown, _ := v.Show(ctx, head)
		texts, _ := v.Texts(ctx, head, []string{"a.txt", "s.txt"})
		if !slices.Equal(parents, []string{o, s}) || shown.Subject != "merge s" || v.Branch(ctx) != "ours" || sha(t, v, "ours") != head ||
			texts["a.txt"] != "o\n" || texts["s.txt"] != "s\n" || len(v.MergeHeads(ctx)) != 0 {
			t.Errorf("Merge(s) into ours: parents %q, subject %q, on %q, files %q, merge heads %q; want o and s, on ours, a.txt o and s.txt s",
				parents, shown.Subject, v.Branch(ctx), texts, v.MergeHeads(ctx))
		}
		if err := v.Merge(ctx, "c", "merge c", false, true, port.Talk{}); err != nil || sha(t, v, "HEAD") != head {
			t.Errorf("Merge(c), which HEAD has: %v, HEAD at %s; want nothing made", err, sha(t, v, "HEAD"))
		}
		head = sha(t, v, "HEAD")
		if err := v.Merge(ctx, "nope", "merge nope", false, true, port.Talk{}); err == nil || sha(t, v, "HEAD") != head {
			t.Errorf("Merge of no commit: %v, HEAD at %s; want an error and nothing made", err, sha(t, v, "HEAD"))
		}
		v, _ = open(t, Layout{Ours: true})
		c, theirs := sha(t, v, "c"), sha(t, v, "t")
		if err := v.Reset(ctx, "c"); err != nil {
			t.Fatalf("Reset(c): %v", err)
		}
		err := v.Merge(ctx, "t", "merge t", false, false, port.Talk{})
		if parents, _ := v.Parents(ctx, "HEAD"); err != nil || !slices.Equal(parents, []string{c, theirs}) {
			t.Errorf("Merge(t) into c, which a fast-forward would reach: %v, parents %q; want c and t", err, parents)
		}
		if err := v.Reset(ctx, "c"); err != nil {
			t.Fatalf("Reset(c): %v", err)
		}
		if err := v.Merge(ctx, "t", "merge t", true, true, port.Talk{}); err != nil || sha(t, v, "HEAD") != theirs || sha(t, v, "ours") != theirs {
			t.Errorf("Merge(t) into c with ff: %v, HEAD at %s; want t %s, on ours", err, sha(t, v, "HEAD"), theirs)
		}
		v, _ = open(t, Layout{Ours: true})
		err = v.Merge(ctx, "t", "merge t", false, true, port.Talk{})
		if _, ok := failureOf(err); !ok || sha(t, v, "HEAD") != o || !slices.Equal(v.MergeHeads(ctx), []string{theirs}) ||
			!slices.Equal(v.Conflicted(ctx), []string{"a.txt"}) {
			t.Errorf("Merge(t) into ours, both changing a.txt: %v, HEAD at %s, merge heads %q, conflicted %q; want o, t merging, a.txt",
				err, sha(t, v, "HEAD"), v.MergeHeads(ctx), v.Conflicted(ctx))
		}
	})
}
