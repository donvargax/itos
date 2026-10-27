package hook

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/shell"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

var (
	zero = strings.Repeat("0", 40)
	a    = strings.Repeat("a", 40)
	b    = strings.Repeat("b", 40)
	c    = strings.Repeat("c", 40)
	gone = strings.Repeat("e", 40)
)

// pushing is a repository of a, its root, b on it, both docs commits
// touching Markdown, and c on a, which touches code, with the config text;
// the push's ref lines are input, given on stdin.
func pushing(t *testing.T, text, input string) *ports {
	t.Helper()
	p := over(t, world{head: map[string]string{}, index: map[string]string{}, work: map[string]string{}, text: text})
	p.VCS.History = []vcsfake.Commit{
		{Name: a, Message: "docs: a\n", Tree: map[string]string{"a.md": "a\n"}},
		{Name: b, Parents: []string{a}, Message: "docs: b\n", Tree: map[string]string{"a.md": "a\n", "b.md": "b\n"}},
		{Name: c, Parents: []string{a}, Message: "docs: c\n", Tree: map[string]string{"a.md": "a\n", "c.go": "package c\n"}},
	}
	p.VCS.Named = map[string]string{"HEAD": b}
	stdin(t, input)
	return p
}

// stdin is a file holding input, standing in for os.Stdin until the test
// ends.
func stdin(t *testing.T, input string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(file, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = saved
		f.Close()
	})
}

// line is git's ref line for a push of local over remote.
func line(local, remote string) string { return "HEAD " + local + " refs/heads/main " + remote + "\n" }

// refsOf are the refs git's lines say are pushed.
func refsOf(h Handler, input string) []pushedRef {
	return h.pushedRefs(context.Background(), pushLines(input))
}

// A deleted ref sends nothing, so it runs nothing; a new branch is judged by
// the commits on no remote branch, and runs per_base from the remote commits
// they grow from, nothing when it adds none, and the whole run when it has no
// such commit.
func TestPushedRefs(t *testing.T) {
	ctx := t.Context()
	h := NewHandler(pushing(t, "version: 1\n", ""))
	refs := refsOf(h, "(delete) "+zero+" refs/heads/old abc\n\n")
	bases, whole := h.pushBases(ctx, refs)
	if len(refs) != 0 || len(bases) != 0 || whole {
		t.Errorf("deleted branch: %v %q %v", refs, bases, whole)
	}
	refs = refsOf(h, "refs/heads/new abc refs/heads/new "+zero+"\n")
	bases, whole = h.pushBases(ctx, refs)
	if len(bases) != 0 || !whole {
		t.Errorf("new branch: %q %v", bases, whole)
	}
	if len(refs) != 1 || refs[0].pushedRange() != (span{vcsport.Unpushed, "abc"}) {
		t.Errorf("new branch's range: %v", refs)
	}
	refs = refsOf(h, line(b, a)+line(b, a)+line(c, b)+"short")
	bases, whole = h.pushBases(ctx, refs)
	if len(refs) != 3 || !slices.Equal(bases, []string{a, b}) || whole || refs[0].pushedRange() != (span{a, b}) {
		t.Errorf("updates: %v %q %v", refs, bases, whole)
	}
	refs = refsOf(h, line(c, gone))
	if bases, whole = h.pushBases(ctx, refs); len(bases) != 0 || !whole {
		t.Errorf("a remote commit the clone lacks: %q %v", bases, whole)
	}
}

// A new ref is judged from the remote commits its own commits grow from
// (slice 182): per_base once for each, whatever other refs bring, nothing
// when it adds no commit, whole when it adds some and grows from none; a
// range git cannot read adds with no base, so it runs whole.
func TestPushBasesOfNewRefs(t *testing.T) {
	ctx := t.Context()
	d := strings.Repeat("d", 40)
	p := pushing(t, "version: 1\n", "")
	p.VCS.History = append(p.VCS.History, vcsfake.Commit{Name: d, Parents: []string{b, c}, Message: "docs: d\n"})
	h := NewHandler(p)
	newRef := func(sha string) string { return "refs/heads/new " + sha + " refs/heads/new " + zero + "\n" }
	for _, tc := range []struct {
		name    string
		remotes map[string]string
		input   string
		bases   []string
		whole   bool
	}{
		{"no remote branch", nil, newRef(b), nil, true},
		{"from one", map[string]string{"x": a}, newRef(b), []string{a}, false},
		{"adding nothing", map[string]string{"x": b}, newRef(b), nil, false},
		{"from two", map[string]string{"x": b, "y": c}, newRef(d), []string{b, c}, false},
		{"from one of two, the older left out", map[string]string{"x": a, "y": b}, newRef(d), []string{b}, false},
		{"beside an update of the same base", map[string]string{"x": a}, newRef(b) + line(c, a), []string{a}, false},
		{"beside an update", map[string]string{"x": a}, newRef(b) + line(c, b), []string{a, b}, false},
		{"beside one that wants whole", nil, newRef(b) + line(c, a), []string{a}, true},
		{"adding nothing beside one that adds", map[string]string{"x": b}, newRef(b) + newRef(d), []string{b}, false},
		{"a range that cannot be read", map[string]string{"x": a}, newRef(gone), nil, true},
	} {
		p.VCS.Named = map[string]string{"HEAD": b}
		for name, sha := range tc.remotes {
			p.VCS.Named["refs/remotes/origin/"+name] = sha
		}
		bases, whole := h.pushBases(ctx, refsOf(h, tc.input))
		slices.Sort(bases)
		want := slices.Clone(tc.bases)
		slices.Sort(want)
		if !slices.Equal(bases, want) || whole != tc.whole {
			t.Errorf("%s: bases %q, whole %v; want %q, %v", tc.name, bases, whole, tc.bases, tc.whole)
		}
	}
}

// git's lines are split on spaces, a carriage return after the last field
// not part of it, and a line may be short; a line deletes a ref when its
// local SHA is zeros or missing, and a ref exists when its remote SHA is
// neither.
func TestPushLines(t *testing.T) {
	got := pushLines("HEAD " + b + " refs/heads/x " + zero + "\r\n\r\n(delete) " + zero + " refs/tags/t " + a + "\nshort\nrefs/heads/y " + b + " refs/heads/y " + a)
	want := []pushLine{
		{b, "refs/heads/x", zero},
		{zero, "refs/tags/t", a},
		{"", "", ""},
		{b, "refs/heads/y", a},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pushLines = %q; want %q", got, want)
	}
	for i, wantDeleted := range []bool{false, true, true, false} {
		if got[i].deleted() != wantDeleted {
			t.Errorf("line %d deleted = %v", i, !wantDeleted)
		}
	}
	for i, wantExisting := range []bool{false, true, false, true} {
		if got[i].existing() != wantExisting {
			t.Errorf("line %d existing = %v", i, !wantExisting)
		}
	}
}

// A tag the remote has is never updated, whatever the new commit builds on
// and whether or not the clone has the old one, and deleting it is refused
// too (slice 182); a new tag and every other ref are not judged by it. The
// hook refuses the first such line, naming the tag and the commits, before it
// reads the config or runs anything.
func TestMovedTag(t *testing.T) {
	tag := func(local, remote string) string { return "refs/tags/v1 " + local + " refs/tags/v1 " + remote + "\n" }
	for input, want := range map[string]bool{
		tag(c, b):                        true,
		tag(b, a):                        true,
		tag(b, gone):                     true,
		tag(zero, a):                     true,
		line(b, a) + tag(b, a):           true,
		tag(b, zero):                     false,
		"HEAD " + b + " refs/tags/v1 \n": false,
		line(c, b):                       false,
		"(delete) " + zero + " refs/heads/x " + a + "\n":              false,
		"refs/heads/refs/tags/v1 " + b + " refs/heads/v1 " + a + "\n": false,
		"": false,
	} {
		if _, got := movedTag(pushLines(input)); got != want {
			t.Errorf("movedTag(%q) = %v; want %v", input, got, want)
		}
	}
	l, _ := movedTag(pushLines(line(b, a) + tag(c, b) + tag(b, a)))
	if l.remote != b {
		t.Errorf("the first moved tag is %+v", l)
	}
	for input, want := range map[string]string{
		tag(c, b): "itos: the push would move the remote's tag refs/tags/v1 (at " + b[:7] + ") to " + c[:7] +
			"; a tag the remote has is never moved or deleted, so the push is refused, and hooks.pre_push's commands did not run.\n" +
			"Tag the new commit with a new name instead.\n",
		tag(zero, b): "itos: the push would delete the remote's tag refs/tags/v1 (at " + b[:7] +
			"); a tag the remote has is never moved or deleted, so the push is refused, and hooks.pre_push's commands did not run.\n",
	} {
		p := pushing(t, pushRules, input)
		p.loadErr = os.ErrNotExist
		code, stdout, stderr, err := hook(t, p, "pre-push", "origin", "url")
		if err != nil || code != policy || stdout != "" || stderr != want || len(p.ran) != 0 {
			t.Errorf("%q = %d, %v\n%s%s\nwant\n%s", input, code, err, stdout, stderr, want)
		}
	}
	p := pushing(t, pushRules, tag(c, gone))
	if _, _, stderr, _ := hook(t, p, "pre-push", "origin", "url"); strings.Contains(stderr, "itos push") {
		t.Errorf("a tag moved onto a commit not built on it is told to run itos push:\n%s", stderr)
	}
}

// A pushed ref rewrites the remote's when the remote commit it replaces is
// not an ancestor of the commit pushed, or is one this clone lacks (slice
// 128); a ref that adds to the remote's commit, or makes a new branch, does
// not. The hook refuses the first that rewrites, naming the ref and itos
// push, before it reads the config or runs anything.
func TestRewrites(t *testing.T) {
	h := NewHandler(pushing(t, "version: 1\n", ""))
	for input, want := range map[string]string{
		line(b, a):                     "",
		line(b, zero):                  "",
		line(b, b):                     "",
		line(c, b):                     b,
		line(b, a) + line(b, gone):     gone,
		"(delete) " + zero + " x " + a: "",
	} {
		refs, answer, refused := h.rewrites(t.Context(), "origin", refsOf(h, input))
		if refused != (want != "") || refused && !strings.Contains(answer.Said, "commit "+short(want)+" on the remote") ||
			!refused && len(refs) != len(refsOf(h, input)) {
			t.Errorf("rewrites(%q) = %+v, %q, %v; want %q", input, refs, answer.Said, refused, want)
		}
	}
	p := pushing(t, "version: 1\n", line(c, b))
	p.loadErr = os.ErrNotExist
	code, _, stderr, err := hook(t, p, "pre-push", "origin", "url")
	if err != nil || code != policy || !strings.Contains(stderr, "refs/heads/main's commit "+b[:7]) ||
		!strings.Contains(stderr, "a commit that does not build on it") || !strings.Contains(stderr, "Run itos push") {
		t.Errorf("a rewrite = %d, %v:\n%s", code, err, stderr)
	}
	p = pushing(t, "version: 1\n", line(c, gone))
	if code, _, stderr, _ := hook(t, p, "pre-push", "origin", "url"); code != policy || !strings.Contains(stderr, "since this clone lacks it") {
		t.Errorf("over a commit the clone lacks = %d:\n%s", code, stderr)
	}
}

// A branch's own rewrite: a topic branch whose commit "docs: b" was rebased
// from a (b1) onto main's new head (m1) as bm, or reworded there as rw.
const (
	m1 = "m0000000"
	b1 = "b0000000"
	bm = "bm000000"
	rw = "rw000000"
	x1 = "x0000000"
)

// rewriting is a repository of a, main's new head m1 on it, the topic's old
// head b1 on a, its rebase bm onto m1, its reword rw and unrelated work x1 on
// a, the clone's origin/main at m1 and the remote's HEAD naming main, which it
// has.
func rewriting(t *testing.T, text string) *ports {
	t.Helper()
	p := pushing(t, text, "")
	commit := func(name, message, author string, parents ...string) vcsfake.Commit {
		return vcsfake.Commit{Name: name, Parents: parents, Message: message, Author: author, Email: author + "@x", Date: "10 +0000",
			Tree: map[string]string{"a.md": "a\n", name + ".md": name + "\n"}}
	}
	p.VCS.History = []vcsfake.Commit{
		{Name: a, Message: "docs: a\n", Tree: map[string]string{"a.md": "a\n"}},
		commit(m1, "docs: m\n", "m", a),
		commit(b1, "docs: b\n", "u", a),
		commit(bm, "docs: b\n", "u", m1),
		commit(rw, "docs: write b\n", "u", m1),
		commit(x1, "docs: x\n", "u", a),
	}
	p.VCS.Named = map[string]string{"HEAD": bm, "refs/remotes/origin/main": m1}
	p.VCS.Remote = map[string]*vcsfake.Remote{"origin": {URL: "origin.git", Refs: map[string]string{"refs/heads/main": m1}}}
	p.head = "main"
	return p
}

// topic is the ref line for a push of local over the remote's topic branch at
// remote.
func topic(local, remote string) string {
	return "HEAD " + local + " refs/heads/topic " + remote + "\n"
}

// A rewrite of a branch that is not main is accepted when each commit of the
// remote's old head that main lacks has a counterpart in the new head's, and
// returned as a new branch: the same author, author date and message, kept
// by a rebase, is one; a reword is none, and is refused naming the commit.
// Every other rewrite stays refused: main's, a ref that is no branch, a
// remote commit the clone lacks, any when the remote's HEAD cannot be asked
// or names no branch it has, and one over a remote this hook is not told.
func TestOwnRewrites(t *testing.T) {
	for _, c := range []struct {
		name   string
		input  string
		remote string
		set    func(*ports)
		want   string // "" when accepted, else what the refusal says
	}{
		{"a rebase", topic(bm, b1), "origin", nil, ""},
		{"a rebase, main's remote-tracking branch gone", topic(bm, b1), "origin", func(p *ports) {
			delete(p.VCS.Named, "refs/remotes/origin/main")
		}, ""},
		{"a branch main has all of", topic(x1, m1), "origin", nil, ""},
		{"a branch main has all of, main's remote-tracking branch gone", topic(x1, m1), "origin", func(p *ports) {
			delete(p.VCS.Named, "refs/remotes/origin/main")
		}, "  m000000 docs: m"},
		{"a reword", topic(rw, b1), "origin", nil, "  b000000 docs: b\n"},
		{"main's", "HEAD " + bm + " refs/heads/main " + b1 + "\n", "origin", nil, "refs/heads/main's commit b000000"},
		{"a ref that is no branch", "HEAD " + bm + " refs/notes/x " + b1 + "\n", "origin", nil, "refs/notes/x's commit b000000"},
		{"a remote commit the clone lacks", topic(bm, gone), "origin", nil, "since this clone lacks it"},
		{"the remote's HEAD naming none", topic(bm, b1), "origin", func(p *ports) { p.head = "" }, "refs/heads/topic's commit b000000"},
		{"the remote's HEAD that cannot be asked", topic(bm, b1), "origin", func(p *ports) { p.headErr = os.ErrDeadlineExceeded }, "refs/heads/topic's commit b000000"},
		{"the remote's HEAD naming a branch it lacks", topic(bm, b1), "origin", func(p *ports) { p.head = "trunk" }, "refs/heads/topic's commit b000000"},
		{"a remote that cannot be asked", topic(bm, b1), "origin", func(p *ports) { delete(p.VCS.Remote, "origin") }, "refs/heads/topic's commit b000000"},
		{"a remote the hook is not told", topic(bm, b1), "", nil, "refs/heads/topic's commit b000000"},
		{"a commit pushed that cannot be read", topic("nope", b1), "origin", nil, "refs/heads/topic's commit b000000"},
	} {
		p := rewriting(t, "version: 1\n")
		if c.set != nil {
			c.set(p)
		}
		h := NewHandler(p)
		refs, answer, refused := h.rewrites(t.Context(), c.remote, refsOf(h, c.input))
		switch {
		case c.want == "" && (refused || len(refs) != 1 || refs[0].remote != "" || refs[0].base != "" || refs[0].ref == "" || refs[0].local == ""):
			t.Errorf("%s: rewrites = %+v, %q, %v; want accepted as a new branch", c.name, refs, answer.Said, refused)
		case c.want != "" && (!refused || !strings.Contains(answer.Said, c.want) || answer.Code != policy):
			t.Errorf("%s: rewrites = %+v, %d, %q, %v; want refused saying %q", c.name, refs, answer.Code, answer.Said, refused, c.want)
		case c.want != "" && c.name == "main's" && strings.Contains(answer.Said, "It would drop"):
			t.Errorf("%s: refused for dropping commits:\n%s", c.name, answer.Said)
		}
	}
}

// An accepted rewrite is judged as a new branch is: hooks.pre_push's per_base
// runs once from the remote commit the commits on no remote-tracking branch
// grow from, main's head, not from the old head the push replaces; a
// refused one runs nothing and says what it drops, and itos push --onto-main.
func TestOwnRewriteIsJudgedAsANewBranch(t *testing.T) {
	p := rewriting(t, pushRules)
	stdin(t, topic(bm, b1))
	code, stdout, stderr, err := hook(t, p, "pre-push", "origin", "url")
	if code != 0 || err != nil || stdout != "$ test "+m1+"\n" || stderr != "" || !slices.Equal(p.ran, []string{"test " + m1}) {
		t.Errorf("an accepted rewrite: exit %d, %v\n%s%s\nran %q", code, err, stdout, stderr, p.ran)
	}
	p = rewriting(t, pushRules)
	stdin(t, topic(rw, b1))
	code, stdout, stderr, err = hook(t, p, "pre-push", "origin", "url")
	if code != policy || err != nil || stdout != "" || len(p.ran) != 0 ||
		!strings.Contains(stderr, "  b000000 docs: b\n") || !strings.Contains(stderr, "itos push --onto-main") {
		t.Errorf("a reword: exit %d, %v\n%s%s\nran %q", code, err, stdout, stderr, p.ran)
	}
}

// Under pre-commit or prek the one ref line is their environment's, a
// missing local or remote branch and an empty from written as git writes
// them; else stdin's.
func TestPrePushInput(t *testing.T) {
	clean(t)
	stdin(t, "from stdin\n")
	if got, err := prePushInput(); got != "from stdin\n" || err != nil {
		t.Errorf("stdin: %q, %v", got, err)
	}
	t.Setenv("PRE_COMMIT_TO_REF", b)
	t.Setenv("PRE_COMMIT_FROM_REF", "")
	if got, _ := prePushInput(); got != "HEAD "+b+" - "+zero+"\n" {
		t.Errorf("defaults: %q", got)
	}
	t.Setenv("PRE_COMMIT_LOCAL_BRANCH", "refs/heads/x")
	t.Setenv("PRE_COMMIT_REMOTE_BRANCH", "refs/heads/y")
	t.Setenv("PRE_COMMIT_FROM_REF", a)
	if got, _ := prePushInput(); got != "refs/heads/x "+b+" refs/heads/y "+a+"\n" {
		t.Errorf("set: %q", got)
	}
	os.Unsetenv("PRE_COMMIT_TO_REF")
	f, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Stdin = f
	if _, err := prePushInput(); err == nil {
		t.Error("a stdin that cannot be read: no error")
	}
	if _, _, _, err := hook(t, pushing(t, "version: 1\n", ""), "pre-push"); err != nil {
		t.Errorf("an empty push: %v", err)
	}
	os.Stdin = f
	if _, _, _, err := hook(t, &ports{}, "pre-push"); err == nil {
		t.Error("the hook over a stdin that cannot be read: no error")
	}
}

// rules for the push: docs may touch Markdown alone; hooks.pre_push's
// commands.
const pushRules = "version: 1\ncommits:\n  types: [docs]\n  scopes: { docs: { only: [\"**/*.md\"] } }\n" +
	"hooks: { pre_push: { per_base: \"test {base}\", whole: \"test all\" } }\n"

// What each pushed ref adds is verified first, a failure's report printed
// with how to fix the commits, nothing run; else hooks.pre_push's per_base
// once per remote base, or whole for a new branch, each printed first;
// a command that fails exits 1.
func TestPrePushRuns(t *testing.T) {
	p := pushing(t, pushRules, line(b, a)+line(b, a))
	code, stdout, stderr, err := hook(t, p, "pre-push", "origin", "url")
	if code != 0 || err != nil || stdout != "$ test "+a+"\n" || stderr != "" || !slices.Equal(p.ran, []string{"test " + a}) {
		t.Errorf("an update: exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	p = pushing(t, pushRules, line(b, zero))
	p.run = func(string, shell.Options) shell.Result { return shell.Result{Code: 1} }
	code, stdout, _, err = hook(t, p, "pre-push", "origin", "url")
	if code != policy || err != nil || stdout != "$ test all\n" {
		t.Errorf("a new branch whose run fails: exit %d, %v\n%s", code, err, stdout)
	}
	p = pushing(t, pushRules, line(c, zero))
	p.VCS.Named["refs/remotes/origin/main"] = a
	code, stdout, stderr, err = hook(t, p, "pre-push", "origin", "url")
	if code != policy || err != nil || stdout != "" || len(p.ran) != 0 ||
		!strings.HasSuffix(stderr, "The push is refused: fix the commits above, then push again.\n"+
			"  - the last commit: git commit --amend\n  - an earlier one: git rebase -i "+a[:7]+", marking it reword or edit\n") {
		t.Errorf("a commit that fails: exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	p = pushing(t, "version: 1\ncommits: { types: [docs] }\n", line(b, zero))
	p.VCS.Named["refs/remotes/origin/main"] = a
	if code, stdout, stderr, err := hook(t, p, "pre-push", "origin", "url"); code != 0 || err != nil || stdout+stderr != "" {
		t.Errorf("no hooks.pre_push: exit %d, %v\n%s%s", code, err, stdout, stderr)
	}
	p = pushing(t, "version: 1\nhooks: { pre_push: { per_base: \"x {base}\", whole: \"y\" } }\n", line(c, a))
	if code, _, _, err := hook(t, p, "pre-push", "origin", "url"); code != 0 || err != nil || !slices.Equal(p.ran, []string{"x " + a}) {
		t.Errorf("no commits section: exit %d, %v, ran %q", code, err, p.ran)
	}
	p = pushing(t, pushRules, line(b, a))
	p.loadErr = os.ErrNotExist
	if _, _, _, err := hook(t, p, "pre-push", "origin", "url"); err == nil {
		t.Error("a config that does not load: no error")
	}
}

// A root commit that fails is rebased with --root; a range whose checks
// alone fail says so; commits.since naming no commit here exits 2, as
// verify; a range that cannot be read stops the hook.
func TestVerifyPush(t *testing.T) {
	text := "version: 1\ncommits:\n  types: [docs]\n  scopes: { docs: { only: [\"**/*.md\"] } }\n"
	p := pushing(t, text, "")
	h := NewHandler(p)
	p.VCS.History[0].Tree = map[string]string{"a.go": "package a\n"}
	a1, err := h.verifyPush(t.Context(), p.cfg, refsOf(h, line(a, zero)), Out{})
	if err != nil || a1.Code != policy || !strings.Contains(a1.Said, "git rebase -i --root,") {
		t.Errorf("a root that fails: %+v, %v", a1, err)
	}
	if got := h.fixAdvice(t.Context(), ""); got != "The push is refused: fix what the range checks above report, then push again.\n" {
		t.Errorf("fixAdvice of no commit: %q", got)
	}
	p = pushing(t, text+"  since: "+gone+"\n", line(b, a))
	code, _, stderr, err := hook(t, p, "pre-push", "origin", "url")
	if code != usage || err != nil || !strings.HasPrefix(stderr, "FAIL ") {
		t.Errorf("commits.since naming no commit: exit %d, %v\n%s", code, err, stderr)
	}
	p = pushing(t, text, line(b, a))
	p.VCS.History = p.VCS.History[:1]
	p.VCS.Named = map[string]string{}
	if _, err := h.verifyPush(t.Context(), p.cfg, []pushedRef{{local: b, base: a, remote: a}}, Out{}); err == nil {
		t.Error("a range that cannot be read: no error")
	}
	checked := text + "tests:\n  scenario:\n    root: features\n    range_checks: [{ name: fails, range: \"exit 1\" }]\n"
	p = pushing(t, checked, line(b, a))
	if code, _, stderr, _ := hook(t, p, "pre-push", "origin", "url"); code != policy ||
		!strings.HasSuffix(stderr, "The push is refused: fix what the range checks above report, then push again.\n") {
		t.Errorf("a range check that fails: exit %d\n%s", code, stderr)
	}
}
