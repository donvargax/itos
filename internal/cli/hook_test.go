package cli

import (
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/git"
	trees "github.com/donvargax/itos/v8/internal/store/tree/adapter"
)

// A deleted branch sends nothing, so it runs nothing; a new one is judged by
// the commits on no remote branch and runs the whole run; neither asks git.
func TestPushedRefs(t *testing.T) {
	zero := "0000000000000000000000000000000000000000"
	refs := pushedRefs("(delete) " + zero + " refs/heads/old abc\n\n")
	bases, whole := pushBases(refs)
	if len(refs) != 0 || len(bases) != 0 || whole {
		t.Errorf("deleted branch: %v %q %v", refs, bases, whole)
	}
	refs = pushedRefs("refs/heads/new abc refs/heads/new " + zero + "\n")
	bases, whole = pushBases(refs)
	if len(bases) != 0 || !whole {
		t.Errorf("new branch: %q %v", bases, whole)
	}
	if len(refs) != 1 || refs[0].pushedRange() != (span{git.Unpushed, "abc"}) {
		t.Errorf("new branch's range: %v", refs)
	}
}

// A staged range command's problem lines are its `  - …` lines, read as
// JavaScript's /^\s+- (.*\S)/ reads them.
func TestProblemLine(t *testing.T) {
	var got []string
	for _, l := range []string{"  - one  ", "- not indented", "\t- two\r", "  -   ", "    - three - more"} {
		if m := problemLine.FindStringSubmatch(l); m != nil {
			got = append(got, m[1])
		}
	}
	if want := []string{"one", "two", "three - more"}; !slices.Equal(got, want) {
		t.Errorf("problem lines: %q", got)
	}
}

// An amend is --amend among git commit's options, never a value of one, nor
// a path after --; a long option that takes no value takes no argument after
// it as one.
func TestAmends(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"--amend", "--no-edit"}, true},
		{[]string{"-m", "x", "--amend"}, true},
		{[]string{"--allow-empty", "--amend"}, true},
		{[]string{"-am", "x", "--amend"}, true},
		{[]string{"-m", "--amend"}, false},
		{[]string{"-m--amend"}, false},
		{[]string{"--message", "--amend"}, false},
		{[]string{"--message=--amend"}, false},
		{[]string{"-C", "--amend"}, false},
		{[]string{"-q", "--", "--amend"}, false},
		{[]string{"--amend", "--no-amend"}, false},
		{[]string{"-m", "x"}, false},
	} {
		if got := readGitArgs(c.args).amend; got != c.want {
			t.Errorf("readGitArgs(%q).amend = %v", c.args, got)
		}
	}
}

// A staged tree whose config or ledger cannot be read names no task, and is
// no error: the header lint has judged the footer by then.
func TestStagedTasksOfAnUnreadableTree(t *testing.T) {
	notesRepo(t)
	writeFile(t, "itos.yaml", "version: [\n")
	gitIn(t, "add", "itos.yaml")
	if named, err := stagedTasks([]string{"T-1"}); named != nil || err != nil {
		t.Errorf("a broken staged config: %v, %v", named, err)
	}
	writeFiles(t, map[string]string{
		"itos.yaml":          "version: 1\nledger: { files: \"tasks/phase-{group}.yaml\", id: \"T-\\\\d+\" }\n",
		"tasks/phase-1.yaml": "a: 1\n",
	})
	gitIn(t, "add", "itos.yaml", "tasks/phase-1.yaml")
	if named, err := stagedTasks([]string{"T-1"}); named != nil || err != nil {
		t.Errorf("a staged ledger that is no list: %v, %v", named, err)
	}
}

// A decision record or the index, in the folder work.decisions names, is
// itos's data the hook checks when staged (slice 74); a file beside them
// that is neither, or one in a folder below, is not.
func TestStagesDecisions(t *testing.T) {
	notesRepo(t)
	writeFile(t, "itos.yaml", "version: 1\ncommits: { types: [docs] }\nwork: { registry: work-items.yaml }\n")
	for staged, want := range map[string]bool{
		"docs/decisions/0001-use-go.md":     true,
		"docs/decisions/README.md":          true,
		"docs/decisions/notes.txt":          false,
		"docs/decisions/old/0001-use-go.md": false,
		"docs/0001-use-go.md":               false,
	} {
		if got := stagesData(trees.Worktree, []string{staged}); got != want {
			t.Errorf("stagesData(%q) = %v", staged, got)
		}
	}
}

// The registry, the questions' file (slice 130) and each kind's smoke set
// are itos's data the hook checks when staged; another file beside them is
// not.
func TestStagesNamedData(t *testing.T) {
	notesRepo(t)
	writeFile(t, "itos.yaml", "version: 1\ncommits: { types: [docs] }\nwork: { registry: work-items.yaml }\n"+
		"tests: { scenario: { root: features, smoke: { file: features/smoke.yaml } } }\n")
	for staged, want := range map[string]bool{
		"work-items.yaml":     true,
		"asks.yaml":           true,
		"features/smoke.yaml": true,
		"notes.yaml":          false,
	} {
		if got := stagesData(trees.Worktree, []string{staged}); got != want {
			t.Errorf("stagesData(%q) = %v", staged, got)
		}
	}
}

// A ledger file is itos's data the hook checks when staged: a file in the
// ledger's folder that its pattern does not name is not, nor is one the
// pattern names in another folder.
func TestStagesLedgerFiles(t *testing.T) {
	notesRepo(t)
	writeFile(t, "itos.yaml", "version: 1\ncommits: { types: [docs] }\nledger: { files: \"tasks/phase-{group}.yaml\" }\n")
	for staged, want := range map[string]bool{
		"tasks/phase-1.yaml": true,
		"tasks/notes.md":     false,
		"docs/phase-1.yaml":  false,
	} {
		if got := stagesData(trees.Worktree, []string{staged}); got != want {
			t.Errorf("stagesData(%q) = %v", staged, got)
		}
	}
}

// The hook judges a merge being made as verify judges it made (bug 30): by
// its own changes, none passing it whatever its message, and some judging it
// as its first line's type, refused when that is none of the commit types.
func TestMergeHook(t *testing.T) {
	dir, _ := notesRepo(t)
	as := []string{"-c", "user.name=t", "-c", "user.email=t@t"}
	git := func(args ...string) { gitIn(t, append(as, args...)...) }
	writeFile(t, "itos.yaml", "version: 1\ncommits:\n  types: [docs, chore]\n  scopes:\n    docs: { only: [\"**/*.md\"] }\n")
	writeFile(t, "a.md", "a\n")
	git("add", "itos.yaml", "a.md")
	git("commit", "-q", "-m", "docs: start the rules")
	git("checkout", "-q", "-b", "topic")
	writeFile(t, "topic.md", "topic\n")
	git("add", "topic.md")
	git("commit", "-q", "-m", "docs: add the topic")
	git("checkout", "-q", "-")
	writeFile(t, "main.md", "main\n")
	git("add", "main.md")
	git("commit", "-q", "-m", "docs: add the main line")
	git("merge", "-q", "--no-ff", "--no-commit", "topic")
	msg := filepath.Join(dir, ".git", "bug-30-msg")
	hook := func(message string) (int, string) {
		t.Helper()
		writeFile(t, msg, message)
		code, stdout, stderr := run("hook", "commit-msg", msg)
		return code, stdout + stderr
	}
	if code, out := hook("Merge branch 'topic'\n"); code != 0 {
		t.Errorf("a merge with no change of its own: exit %d\n%s", code, out)
	}
	if code, out := hook("update things\n"); code != 0 {
		t.Errorf("a merge with no change of its own and a header with no type: exit %d\n%s", code, out)
	}
	writeFile(t, "src.js", "src\n")
	git("add", "src.js")
	if code, out := hook("Merge branch 'topic'\n"); code != 1 || !strings.Contains(out, "a merge commit with changes of its own (src.js)") {
		t.Errorf("a merge with a change of its own and no type: exit %d\n%s", code, out)
	}
	if code, out := hook("docs: merge the topic\n"); code != 1 || !strings.Contains(out, "docs commits may not touch src.js") ||
		strings.Contains(out, "topic.md") {
		t.Errorf("a docs merge with a change of its own: exit %d\n%s", code, out)
	}
	if code, out := hook("chore: merge the topic\n"); code != 0 {
		t.Errorf("a chore merge with a change of its own: exit %d\n%s", code, out)
	}
}

// A pushed ref rewrites the remote's when the remote commit it replaces is
// not an ancestor of the commit pushed, or is one this clone lacks (slice
// 128); a ref that adds to the remote's commit, or makes a new branch, does
// not. The hook refuses the first that rewrites, naming the ref and itos
// push, before it reads the config or runs anything.
func TestRewrites(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "itos"}, {"GIT_COMMITTER_NAME", "itos"},
		{"GIT_AUTHOR_EMAIL", "itos@example.com"}, {"GIT_COMMITTER_EMAIL", "itos@example.com"}} {
		t.Setenv(kv[0], kv[1])
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(git.Bin(runtime.GOOS), args...).Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return strings.TrimSpace(string(out))
	}
	run("commit", "-q", "--allow-empty", "-m", "a")
	a := run("rev-parse", "HEAD")
	run("commit", "-q", "--allow-empty", "-m", "b")
	b := run("rev-parse", "HEAD")
	c := run("commit-tree", "HEAD^{tree}", "-p", a, "-m", "c")
	zero := strings.Repeat("0", 40)
	lacked := strings.Repeat("e", 40)
	line := func(local, remote string) string { return "HEAD " + local + " refs/heads/main " + remote + "\n" }
	for input, want := range map[string]string{
		line(b, a):                     "",
		line(b, zero):                  "",
		line(b, b):                     "",
		line(c, b):                     b,
		line(b, a) + line(b, lacked):   lacked,
		"(delete) " + zero + " x " + a: "",
	} {
		r, ok := rewrites(pushedRefs(input))
		if ok != (want != "") || r.remote != want {
			t.Errorf("rewrites(%q) = %+v, %v; want %q", input, r, ok, want)
		}
	}

	t.Setenv("PRE_COMMIT_TO_REF", c)
	t.Setenv("PRE_COMMIT_FROM_REF", b)
	t.Setenv("PRE_COMMIT_REMOTE_BRANCH", "refs/heads/main")
	var stderr strings.Builder
	code, err := hookPrePush(Out{Stdout: io.Discard, Stderr: &stderr})
	if err != nil || code != ExitPolicy || !strings.Contains(stderr.String(), "refs/heads/main's commit "+b[:7]) ||
		!strings.Contains(stderr.String(), "a commit that does not build on it") || !strings.Contains(stderr.String(), "Run itos push") {
		t.Errorf("hook pre-push of a rewrite = %d, %v:\n%s", code, err, stderr.String())
	}
	t.Setenv("PRE_COMMIT_FROM_REF", lacked)
	stderr.Reset()
	if code, _ := hookPrePush(Out{Stdout: io.Discard, Stderr: &stderr}); code != ExitPolicy ||
		!strings.Contains(stderr.String(), "since this clone lacks it") {
		t.Errorf("hook pre-push over a commit the clone lacks = %d:\n%s", code, stderr.String())
	}
}
