package cli

import (
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/shell"
)

// hookPorts bind the hook slice to the real code: the trees git holds,
// the working tree's config and files, config check's task flags and the
// config's shell.
func TestHookPortsBindTheRealCode(t *testing.T) {
	gitConfigRepo(t, "version: 1\nwork: { registry: work-items.yaml }\n")
	writeFile(t, "a.md", "a\n")
	gitIn(t, "add", "a.md")
	var p hookPorts
	if index, err := p.Trees(t.Context()).At(t.Context(), "index"); err != nil || !index.Has(t.Context(), "a.md") {
		t.Errorf("Trees = %v, want the index git holds", err)
	}
	cfg, err := p.LoadConfig(t.Context())
	if err != nil || cfg.Work.Registry != "work-items.yaml" {
		t.Fatalf("LoadConfig = %+v, %v, want the working tree's config", cfg, err)
	}
	if text, err := p.Read(t.Context(), "a.md"); text != "a\n" || err != nil {
		t.Errorf("Read = %q, %v", text, err)
	}
	if flags := p.TaskFlags(t.Context()); !slices.Contains(flags, "--config") {
		t.Errorf("TaskFlags = %q", flags)
	}
	var stdout strings.Builder
	if r := p.Shell(t.Context(), cfg, "echo out; exit 3", shell.Options{Stdout: &stdout, Stderr: io.Discard}); r.Code != 3 || stdout.String() != "out\n" {
		t.Errorf("Shell = %+v, stdout %q", r, stdout.String())
	}
}

// The hook judges a merge being made in a real repository as verify judges
// it made (bug 30): by its own changes, none passing it whatever its
// message, and some judging it as its first line's type, refused when that
// is none of the commit types.
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
	writeFile(t, "src.js", "src\n")
	git("add", "src.js")
	if code, out := hook("docs: merge the topic\n"); code != 1 || !strings.Contains(out, "docs commits may not touch src.js") ||
		strings.Contains(out, "topic.md") {
		t.Errorf("a docs merge with a change of its own: exit %d\n%s", code, out)
	}
	if code, out := hook("chore: merge the topic\n"); code != 0 {
		t.Errorf("a chore merge with a change of its own: exit %d\n%s", code, out)
	}
}

// A push that would replace the remote's commit with one that does not
// build on it is refused in a real repository, git judging the ancestry.
func TestPrePushRefusesARewrite(t *testing.T) {
	gitConfigRepo(t, "version: 1\n")
	as := []string{"-c", "user.name=t", "-c", "user.email=t@t"}
	git := func(args ...string) string { return strings.TrimSpace(gitIn(t, append(as, args...)...)) }
	git("commit", "-q", "--allow-empty", "-m", "a")
	a := git("rev-parse", "HEAD")
	git("commit", "-q", "--allow-empty", "-m", "b")
	b := git("rev-parse", "HEAD")
	c := git("commit-tree", "HEAD^{tree}", "-p", a, "-m", "c")
	t.Setenv("PRE_COMMIT_TO_REF", c)
	t.Setenv("PRE_COMMIT_FROM_REF", b)
	t.Setenv("PRE_COMMIT_REMOTE_BRANCH", "refs/heads/main")
	var stderr strings.Builder
	code, err := runHook([]string{"pre-push", "origin", "url"}, Out{Stdout: io.Discard, Stderr: &stderr})
	if err != nil || code != ExitPolicy || !strings.Contains(stderr.String(), "refs/heads/main's commit "+b[:7]) {
		t.Errorf("hook pre-push of a rewrite = %d, %v:\n%s", code, err, stderr.String())
	}
	t.Setenv("PRE_COMMIT_TO_REF", b)
	t.Setenv("PRE_COMMIT_FROM_REF", a)
	stderr.Reset()
	if code, err := runHook([]string{"pre-push", "origin", "url"}, Out{Stdout: io.Discard, Stderr: &stderr}); code != 0 || err != nil {
		t.Errorf("hook pre-push of an update = %d, %v:\n%s", code, err, stderr.String())
	}
}
