package porttest

import (
	"errors"
	"slices"

	"github.com/donvargax/itos/v8/internal/vcs/port"
)

// commitContract holds committing, the index's changes, notes, a commit
// outside every branch, a new repository and the ignored paths.
func commitContract[C T[C]](t C, open func(t C, l Layout) (port.VCS, string)) {
	t.Helper()
	dirty := Layout{Dirty: true}
	t.Run("commit", func(t C) {
		v, _ := open(t, dirty)
		ctx := t.Context()
		m := sha(t, v, "m")
		only := port.Commit{Paragraphs: []string{"x", "the body"}, Only: []string{"c.txt"}, Quiet: true}
		if err := v.Commit(ctx, only, port.Talk{}); err != nil {
			t.Fatalf("Commit of c.txt alone: %v", err)
		}
		head := sha(t, v, "HEAD")
		shown, _ := v.Show(ctx, head)
		paths, _ := v.CommitPaths(ctx, head)
		parents, _ := v.Parents(ctx, head)
		text, _, _ := v.File(ctx, head, "c.txt")
		if !slices.Equal(parents, []string{m}) || !slices.Equal(paths, []string{"c.txt"}) || shown.Message != "x\n\nthe body\n" ||
			text != "changed\n" || v.Branch(ctx) != "main" || sha(t, v, "main") != head {
			t.Errorf("Commit of c.txt alone: parents %q, paths %q, message %q, c.txt %q, on %q", parents, paths, shown.Message, text, v.Branch(ctx))
		}
		if staged, err := v.Staged(ctx, ""); !slices.Equal(staged, []string{"a.txt", "b.txt", "new.txt"}) || err != nil {
			t.Errorf("Commit of c.txt alone left the index's changes %q, %v; want a.txt b.txt new.txt", staged, err)
		}
		if err := v.Commit(ctx, port.Commit{Message: "y\n"}, port.Talk{}); err != nil {
			t.Fatalf("Commit of the index: %v", err)
		}
		head = sha(t, v, "HEAD")
		shown, _ = v.Show(ctx, head)
		paths, _ = v.CommitPaths(ctx, head)
		if !slices.Equal(paths, []string{"a.txt", "b.txt", "new.txt"}) || shown.Message != "y\n" {
			t.Errorf("Commit of the index: paths %q, message %q", paths, shown.Message)
		}
		if err := v.Commit(ctx, port.Commit{Message: "z\n"}, port.Talk{}); err == nil || sha(t, v, "HEAD") != head {
			t.Errorf("Commit of nothing staged: %v, HEAD moved", err)
		}
	})
	t.Run("commit a deletion", func(t C) {
		v, _ := open(t, dirty)
		ctx := t.Context()
		if err := v.Commit(ctx, port.Commit{Message: "d\n", Only: []string{"b.txt"}}, port.Talk{}); err != nil {
			t.Fatalf("Commit of b.txt's deletion: %v", err)
		}
		paths, _ := v.CommitPaths(ctx, "HEAD")
		if _, ok, err := v.File(ctx, "HEAD", "b.txt"); ok || err != nil || !slices.Equal(paths, []string{"b.txt"}) {
			t.Errorf("Commit of b.txt's deletion: paths %q, b.txt there %v, %v", paths, ok, err)
		}
	})
	t.Run("staging", func(t C) {
		v, _ := open(t, dirty)
		ctx := t.Context()
		status := func(path string) []string {
			lines, _ := v.PathStatus(ctx, path)
			return lines
		}
		if err := v.Add(ctx, []string{"u.txt"}, port.Talk{}); err != nil || !v.Tracked(ctx, "u.txt") || !slices.Equal(status("u.txt"), []string{"A  u.txt"}) {
			t.Errorf("Add(u.txt): %v, status %q", err, status("u.txt"))
		}
		if err := v.Add(ctx, []string{"nope.txt"}, port.Talk{}); err == nil {
			t.Errorf("Add of a file nothing has: no error")
		}
		if err := v.Unstage(ctx, []string{"a.txt"}); err != nil || !slices.Equal(status("a.txt"), []string{" M a.txt"}) {
			t.Errorf("Unstage(a.txt): %v, status %q", err, status("a.txt"))
		}
		if err := v.Untrack(ctx, []string{"new.txt", "nope.txt"}); err != nil || v.Tracked(ctx, "new.txt") || !slices.Equal(status("new.txt"), []string{"?? new.txt"}) {
			t.Errorf("Untrack(new.txt): %v, status %q", err, status("new.txt"))
		}
		if err := v.Restore(ctx, []string{"c.txt"}); err != nil || len(status("c.txt")) != 0 {
			t.Errorf("Restore(c.txt): %v, status %q", err, status("c.txt"))
		}
		if err := v.Restore(ctx, []string{"u.txt"}); err == nil {
			t.Errorf("Restore of a file HEAD lacks: no error")
		}
		if err := v.Unstage(ctx, []string{"b.txt"}); err != nil || !slices.Equal(status("b.txt"), []string{" D b.txt"}) {
			t.Errorf("Unstage(b.txt), which the work tree lacks: %v, status %q", err, status("b.txt"))
		}
		if err := v.Add(ctx, []string{"b.txt"}, port.Talk{}); err != nil || !slices.Equal(status("b.txt"), []string{"D  b.txt"}) {
			t.Errorf("Add(b.txt), which the work tree lacks: %v, status %q", err, status("b.txt"))
		}
	})
	t.Run("notes", func(t C) {
		v, _ := open(t, Layout{})
		ctx := t.Context()
		for _, c := range []struct{ rev, text string }{{"a", "Task: T-2"}, {"c", "Task: T-3"}} {
			err := v.Note(ctx, c.rev, c.text)
			if shown, _ := v.Show(ctx, c.rev); err != nil || shown.Note != c.text+"\n" {
				t.Errorf("Note(%s, %s): %v, the note %q", c.rev, c.text, err, shown.Note)
			}
		}
		if err := v.Note(ctx, "nope", "x"); err == nil {
			t.Errorf("Note of no commit: no error")
		}
	})
	t.Run("commit file", func(t C) {
		v, _ := open(t, Layout{})
		ctx := t.Context()
		m := sha(t, v, "m")
		first, err := v.CommitFile(ctx, "", "counters/slice", "1\n", "reserve", "itos id counter", "c@localhost")
		if err != nil {
			t.Fatalf("CommitFile with no parent: %v", err)
		}
		shown, _ := v.Show(ctx, first)
		parents, _ := v.Parents(ctx, first)
		files, _ := v.Files(ctx, first, "")
		if len(parents) != 0 || !slices.Equal(files, []string{"counters/slice"}) || shown.Author != "itos id counter" ||
			shown.Email != "c@localhost" || shown.Message != "reserve\n" {
			t.Errorf("CommitFile with no parent: parents %q, files %q, %+v", parents, files, shown)
		}
		second, err := v.CommitFile(ctx, first, "counters/task", "2\n", "reserve", "itos id counter", "c@localhost")
		parents, _ = v.Parents(ctx, second)
		texts, _ := v.Texts(ctx, second, []string{"counters/slice", "counters/task"})
		if err != nil || !slices.Equal(parents, []string{first}) || texts["counters/slice"] != "1\n" || texts["counters/task"] != "2\n" {
			t.Errorf("CommitFile on the first: %v, parents %q, files %q", err, parents, texts)
		}
		if sha(t, v, "HEAD") != m || v.Branch(ctx) != "main" {
			t.Errorf("CommitFile moved HEAD")
		}
		if got, err := v.CommitFile(ctx, "nope", "f", "x\n", "m", "n", "e@x"); err == nil {
			t.Errorf("CommitFile on no commit is %q", got)
		}
	})
	t.Run("cherry pick", func(t C) {
		v, _ := open(t, Layout{})
		ctx := t.Context()
		head, b := sha(t, v, "HEAD"), sha(t, v, "b")
		picked, err := v.CherryPick(ctx, "s", "b")
		if err != nil {
			t.Fatalf("CherryPick(s onto b): %v", err)
		}
		was, _ := v.Show(ctx, "s")
		shown, _ := v.Show(ctx, picked)
		parents, _ := v.Parents(ctx, picked)
		texts, _ := v.Texts(ctx, picked, []string{"a.txt", "b.txt", "s.txt"})
		if picked == sha(t, v, "s") || !slices.Equal(parents, []string{b}) || shown.Message != was.Message || shown.Author != was.Author ||
			shown.Email != was.Email || shown.Date != was.Date || texts["a.txt"] != "b\n" || texts["b.txt"] != "b\n" || texts["s.txt"] != "s\n" {
			t.Errorf("CherryPick(s onto b): %s, parents %q, %+v, files %q", picked, parents, shown, texts)
		}
		if sha(t, v, "HEAD") != head || v.Branch(ctx) != "main" || sha(t, v, "main") != head {
			t.Errorf("CherryPick(s onto b) moved HEAD or main")
		}
		picked, err = v.CherryPick(ctx, "c", "s")
		shown, _ = v.Show(ctx, picked)
		files, _ := v.Files(ctx, picked, "")
		if err != nil || shown.Note != "" || !slices.Equal(files, []string{"a.txt", "c.txt", "s.txt"}) {
			t.Errorf("CherryPick(c onto s): %v, note %q, files %q; want c's change on s, with no note", err, shown.Note, files)
		}
		var conflicts *port.Conflicts
		if _, err := v.CherryPick(ctx, "t", "s"); !errors.As(err, &conflicts) || !slices.Equal(conflicts.Paths, []string{"a.txt"}) {
			t.Errorf("CherryPick(t onto s), both changing a.txt: %v; want a conflict in a.txt", err)
		}
		for _, c := range [][2]string{{"nope", "b"}, {"s", "nope"}} {
			if made, err := v.CherryPick(ctx, c[0], c[1]); err == nil || errors.As(err, &conflicts) {
				t.Errorf("CherryPick(%s onto %s) is %q, %v; want an error that is no conflict", c[0], c[1], made, err)
			}
		}
	})
	t.Run("reset", func(t C) {
		v, _ := open(t, dirty)
		ctx := t.Context()
		b := sha(t, v, "b")
		if err := v.Reset(ctx, "b"); err != nil {
			t.Fatalf("Reset(b): %v", err)
		}
		if sha(t, v, "HEAD") != b || sha(t, v, "main") != b || v.Branch(ctx) != "main" || len(v.Changes(ctx)) != 0 {
			t.Errorf("Reset(b): HEAD at %s, main at %s, on %q, changes %q; want b %s, on main, clean",
				sha(t, v, "HEAD"), sha(t, v, "main"), v.Branch(ctx), v.Changes(ctx), b)
		}
		if err := v.Reset(ctx, "nope"); err == nil {
			t.Errorf("Reset of no commit: no error")
		}
	})
	t.Run("init", func(t C) {
		v, root := open(t, Layout{None: true})
		ctx := t.Context()
		if l, err := v.Locate(ctx, ""); err == nil {
			t.Fatalf("Locate in no repository is %+v", l)
		}
		if got, err := v.Remotes(ctx); err == nil {
			t.Errorf("Remotes in no repository are %q", got)
		}
		if err := v.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		if l, err := v.Locate(ctx, ""); err != nil || l.Top != root {
			t.Errorf("Locate after Init is %+v, %v; want the top %s", l, err, root)
		}
	})
	t.Run("ignored", func(t C) {
		v, _ := open(t, dirty)
		ctx := t.Context()
		for _, c := range []struct {
			path string
			want bool
		}{{"u.txt", true}, {"a.txt", true}, {"debug.log", false}, {"../outside/x", false}} {
			if got := v.NotIgnored(ctx, c.path); got != c.want {
				t.Errorf("NotIgnored(%s) is %v", c.path, got)
			}
		}
	})
}
