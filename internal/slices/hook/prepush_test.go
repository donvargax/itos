package hook

import (
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

// A deleted branch sends nothing, so it runs nothing; a new one is judged
// by the commits on no remote branch and runs the whole run.
func TestPushedRefs(t *testing.T) {
	h := NewHandler(pushing(t, "version: 1\n", ""))
	refs := h.pushedRefs(t.Context(), "(delete) "+zero+" refs/heads/old abc\n\n")
	bases, whole := pushBases(refs)
	if len(refs) != 0 || len(bases) != 0 || whole {
		t.Errorf("deleted branch: %v %q %v", refs, bases, whole)
	}
	refs = h.pushedRefs(t.Context(), "refs/heads/new abc refs/heads/new "+zero+"\n")
	bases, whole = pushBases(refs)
	if len(bases) != 0 || !whole {
		t.Errorf("new branch: %q %v", bases, whole)
	}
	if len(refs) != 1 || refs[0].pushedRange() != (span{vcsport.Unpushed, "abc"}) {
		t.Errorf("new branch's range: %v", refs)
	}
	refs = h.pushedRefs(t.Context(), line(b, a)+line(b, a)+line(c, b)+"short")
	bases, whole = pushBases(refs)
	if len(refs) != 3 || !slices.Equal(bases, []string{a, b}) || whole || refs[0].pushedRange() != (span{a, b}) {
		t.Errorf("updates: %v %q %v", refs, bases, whole)
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
		r, ok := h.rewrites(t.Context(), h.pushedRefs(t.Context(), input))
		if ok != (want != "") || r.remote != want {
			t.Errorf("rewrites(%q) = %+v, %v; want %q", input, r, ok, want)
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
	p.VCS.Named["refs/remotes/origin/main"] = a
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
	a1, err := h.verifyPush(t.Context(), p.cfg, h.pushedRefs(t.Context(), line(a, zero)), Out{})
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
