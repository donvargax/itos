package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v6/internal/git"
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
// a path after --.
func TestAmends(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"--amend", "--no-edit"}, true},
		{[]string{"-m", "x", "--amend"}, true},
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
		if got := stagesData([]string{staged}); got != want {
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
