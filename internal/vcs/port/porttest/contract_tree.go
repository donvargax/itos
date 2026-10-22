package porttest

import (
	"maps"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// treeContract holds the files at a tree, the patches and the refs.
func treeContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	main := Layout{Remotes: []string{"b"}}
	t.Run("files at a tree", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		if text, ok, err := v.File(ctx, "m", "a.txt"); text != "m\n" || !ok || err != nil {
			t.Errorf("File(m, a.txt) is %q, %v, %v; want m", text, ok, err)
		}
		if text, ok, err := v.File(ctx, "m", "x.md"); ok || err != nil {
			t.Errorf("File(m, x.md), which m deleted, is %q, %v, %v", text, ok, err)
		}
		if text, ok, err := v.File(ctx, "nope", "a.txt"); err == nil {
			t.Errorf("File of no commit is %q, %v", text, ok)
		}
		for _, c := range []struct {
			tree, dir string
			want      []string
		}{
			{"m", "", []string{"a.txt", "b.txt", "c.txt", "s.txt"}},
			{"a", "x.md", []string{"x.md"}},
			{"m", "nope", nil},
		} {
			if got, err := v.Files(ctx, c.tree, c.dir); !slices.Equal(got, c.want) || err != nil {
				t.Errorf("Files(%s, %q) are %q, %v; want %q", c.tree, c.dir, got, err, c.want)
			}
		}
		if got, err := v.Files(ctx, "nope", ""); err == nil {
			t.Errorf("Files of no commit are %q", got)
		}
		want := map[string]string{"a.txt": "m\n", "s.txt": "s\n"}
		if got, err := v.Texts(ctx, "m", []string{"a.txt", "s.txt"}); !maps.Equal(got, want) || err != nil {
			t.Errorf("Texts(m, a.txt s.txt) are %q, %v; want %q", got, err, want)
		}
		if got, err := v.Texts(ctx, "m", []string{"a.txt", "x.md"}); err == nil {
			t.Errorf("Texts(m) of a file m lacks are %q", got)
		}
		if got, err := v.Texts(ctx, "nope", []string{"a.txt"}); err == nil {
			t.Errorf("Texts of no commit are %q", got)
		}
	})
	t.Run("the index's files", func(t C) {
		v, _ := open(t, Layout{Dirty: true})
		ctx := t.Context()
		if text, ok, err := v.File(ctx, port.Index, "new.txt"); text != "new\n" || !ok || err != nil {
			t.Errorf("File(the index, new.txt) is %q, %v, %v; want new", text, ok, err)
		}
		if text, ok, err := v.File(ctx, port.Index, "b.txt"); ok || err != nil {
			t.Errorf("File(the index, b.txt), deleted there, is %q, %v, %v", text, ok, err)
		}
		want := []string{"a.txt", "c.txt", "new.txt", "s.txt"}
		if got, err := v.Files(ctx, port.Index, ""); !slices.Equal(got, want) || err != nil {
			t.Errorf("Files(the index) are %q, %v; want %q", got, err, want)
		}
		if got, err := v.Texts(ctx, port.Index, []string{"a.txt"}); got["a.txt"] != "staged\n" || err != nil {
			t.Errorf("Texts(the index, a.txt) are %q, %v; want staged", got, err)
		}
	})
	t.Run("patches", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		p, err := v.Patch(ctx, "c")
		if err != nil || !strings.Contains(p, "c.txt") || !strings.Contains(p, "x.md") || strings.Contains(p, "a.txt") {
			t.Errorf("Patch(c), which deletes x.md and adds c.txt, is %q, %v", p, err)
		}
		if p, err := v.Patch(ctx, "nope"); err == nil {
			t.Errorf("Patch of no commit is %q", p)
		}
		shown, err := v.Shown(ctx, []string{"b", "c"}, true)
		for _, want := range []string{"the body of b", "the body of c", "Task: T-1", "c.txt"} {
			if err != nil || !strings.Contains(shown, want) {
				t.Errorf("Shown(b, c) with notes does not say %q: %q, %v", want, shown, err)
			}
		}
		if shown, err := v.Shown(ctx, []string{"c"}, false); err != nil || strings.Contains(shown, "Task: T-1") || !strings.Contains(shown, "the body of c") {
			t.Errorf("Shown(c) without notes is %q, %v", shown, err)
		}
		if shown, err := v.Shown(ctx, []string{"c", "nope"}, false); err == nil {
			t.Errorf("Shown of no commit is %q", shown)
		}
	})
	t.Run("refs", func(t C) {
		v, _ := open(t, main)
		ctx := t.Context()
		shas := map[string]string{}
		for _, name := range []string{"a", "c", "m"} {
			shas[name] = sha(t, v, name)
		}
		if got, err := v.Ref(ctx, "refs/tags/m"); got != shas["m"] || err != nil {
			t.Errorf("Ref(refs/tags/m) is %q, %v; want m's %q", got, err, shas["m"])
		}
		tag, err := v.Ref(ctx, "refs/tags/r")
		if err != nil || tag == "" || tag == shas["c"] {
			t.Errorf("Ref(refs/tags/r), an annotated tag, is %q, %v; want the tag, not c's %q", tag, err, shas["c"])
		}
		if got := sha(t, v, "r"); got != shas["c"] {
			t.Errorf("Resolve(r) is %q; want c's %q", got, shas["c"])
		}
		if got, err := v.Ref(ctx, "refs/heads/nope"); err == nil {
			t.Errorf("Ref of no ref is %q", got)
		}
		refs, err := v.Refs(ctx, "refs/tags/")
		for ref, want := range map[string]string{"refs/tags/a": shas["a"], "refs/tags/m": shas["m"], "refs/tags/r": tag, "refs/tags/r^{}": shas["c"]} {
			if refs[ref] != want || err != nil {
				t.Errorf("Refs(refs/tags/) name %s %q, %v; want %q", ref, refs[ref], err, want)
			}
		}
		if refs["refs/heads/main"] != "" || refs["refs/tags/a^{}"] != "" {
			t.Errorf("Refs(refs/tags/) are %q", refs)
		}
		if refs, err := v.Refs(ctx, "refs/nope/"); len(refs) != 0 || err != nil {
			t.Errorf("Refs(refs/nope/) are %q, %v", refs, err)
		}
	})
}
