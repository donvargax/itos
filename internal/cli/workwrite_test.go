package cli

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/donvargax/itos/v3/internal/config"
	"github.com/donvargax/itos/v3/internal/value"
)

// The rollback of writeCommitted (bug 13): a scratch repository, its files
// committed, as the current folder, away from the user's git config, with a
// fixed author. registry.yaml is the registry, ledger.yaml a ledger file.
func rollbackRepo(t *testing.T) *config.Loaded {
	t.Helper()
	dir := gitConfigRepo(t, "version: 1\n")
	for name, value := range map[string]string{
		"GIT_AUTHOR_NAME": "A", "GIT_AUTHOR_EMAIL": "a@example.com",
		"GIT_COMMITTER_NAME": "A", "GIT_COMMITTER_EMAIL": "a@example.com",
	} {
		t.Setenv(name, value)
	}
	for name, text := range map[string]string{"registry.yaml": "items: []\n", "ledger.yaml": "- id: T-1\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, "add", "--", "itos.yaml", "registry.yaml", "ledger.yaml")
	gitIn(t, "commit", "-q", "-m", "chore: start")
	cfg := &config.Loaded{}
	cfg.Work.Registry = "registry.yaml"
	return cfg
}

func gitIn(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// A commit-msg hook in git's own hooks folder that exits 1.
func refusingHook(t *testing.T) {
	t.Helper()
	gitHook(t, "commit-msg", "#!/bin/sh\necho refused >&2\nexit 1\n")
}

// The hook, by its name, in git's own hooks folder.
func gitHook(t *testing.T, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the hook is a shell script")
	}
	hooks := filepath.Join(".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The registry edited and a new ledger file, as task add writes them.
func rollbackFiles() []written {
	return []written{
		{path: "registry.yaml", old: "items: []\n", text: "items:\n  - id: T-2\n"},
		{path: "new.yaml", text: "- id: T-2\n", created: true},
	}
}

// Every file as it was, the new one gone, and git status reporting nothing.
func asItWas(t *testing.T) {
	t.Helper()
	if text, err := os.ReadFile("registry.yaml"); err != nil || string(text) != "items: []\n" {
		t.Errorf("registry.yaml reads %q (%v)", text, err)
	}
	if _, err := os.Stat("new.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("new.yaml is still there (%v)", err)
	}
	if status := gitIn(t, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Errorf("git status reports:\n%s", status)
	}
}

func writeAndCommit(t *testing.T, cfg *config.Loaded, files []written, body string) (string, int, error, string) {
	t.Helper()
	var stdout, stderr strings.Builder
	sha, code, err := writeCommitted(cfg, files, "docs: add T-2", body, Out{Stdout: &stdout, Stderr: &stderr})
	return sha, code, err, stderr.String()
}

// A commit a hook refuses puts every file back, file and index, the new one
// removed and unstaged, and says so.
func TestRollbackHookRefuses(t *testing.T) {
	cfg := rollbackRepo(t)
	refusingHook(t)
	head := gitIn(t, "rev-parse", "HEAD")
	sha, code, err, stderr := writeAndCommit(t, cfg, rollbackFiles(), "Add T-2.")
	if sha != "" || code != ExitPolicy || err != nil {
		t.Fatalf("got %q, exit %d, %v\n%s", sha, code, err, stderr)
	}
	asItWas(t)
	if want := "the commit of registry.yaml and new.yaml failed (git exited 1), so registry.yaml and new.yaml are as they were"; !strings.Contains(stderr, want) || !strings.Contains(stderr, "refused") {
		t.Errorf("stderr:\n%s", stderr)
	}
	if gitIn(t, "rev-parse", "HEAD") != head {
		t.Error("a commit was made")
	}
}

// A write that fails after the first one puts the first back.
func TestRollbackSecondWriteFails(t *testing.T) {
	cfg := rollbackRepo(t)
	files := rollbackFiles()
	files[1].path = filepath.Join("missing", "new.yaml")
	_, _, err, _ := writeAndCommit(t, cfg, files, "Add T-2.")
	if err == nil || !strings.Contains(err.Error(), filepath.Join("missing", "new.yaml")+" cannot be written") {
		t.Fatalf("the error is %v", err)
	}
	asItWas(t)
}

// A git that cannot start the commit, here because the message is longer
// than any system lets a program be given, puts back what git add staged.
func TestRollbackGitCannotStart(t *testing.T) {
	cfg := rollbackRepo(t)
	_, code, err, stderr := writeAndCommit(t, cfg, rollbackFiles(), strings.Repeat("word ", 1<<20))
	if err == nil || !strings.Contains(err.Error(), "cannot run git") {
		t.Fatalf("exit %d, the error is %v\n%s", code, err, stderr)
	}
	asItWas(t)
}

// git add refusing the new file puts everything back and shows what git
// said.
func TestRollbackGitAddFails(t *testing.T) {
	cfg := rollbackRepo(t)
	if err := os.WriteFile(filepath.Join(".git", "info", "exclude"), []byte("new.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, code, err, stderr := writeAndCommit(t, cfg, rollbackFiles(), "Add T-2.")
	if code != ExitPolicy || err != nil {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr)
	}
	asItWas(t)
	if !strings.Contains(stderr, "ignored") || !strings.Contains(stderr, "git add of new.yaml failed (git exited 1)") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

// A file deleted and the deletion not committed, staged or not, is a change
// no commit holds: refused before anything is written, the message naming
// the file, not the registry.
func TestRefuseUncommittedDeletion(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(map[bool]string{false: "unstaged", true: "staged"}[staged], func(t *testing.T) {
			cfg := rollbackRepo(t)
			if staged {
				gitIn(t, "rm", "-q", "--", "ledger.yaml")
			} else if err := os.Remove("ledger.yaml"); err != nil {
				t.Fatal(err)
			}
			status := gitIn(t, "status", "--porcelain")
			files := []written{
				{path: "ledger.yaml", text: "- id: T-2\n", created: true},
				{path: "registry.yaml", old: "items: []\n", text: "items:\n  - id: T-2\n"},
			}
			_, code, err, stderr := writeAndCommit(t, cfg, files, "Add T-2.")
			if code != ExitPolicy || err != nil {
				t.Fatalf("exit %d, %v\n%s", code, err, stderr)
			}
			if !strings.HasPrefix(stderr, "ledger.yaml has changes no commit holds") || strings.Contains(stderr, "registry") {
				t.Errorf("stderr:\n%s", stderr)
			}
			if _, err := os.Stat("ledger.yaml"); !errors.Is(err, fs.ErrNotExist) {
				t.Error("ledger.yaml was written")
			}
			if now := gitIn(t, "status", "--porcelain"); now != status {
				t.Errorf("git status was\n%s\nand is\n%s", status, now)
			}
		})
	}
}

// A pre-commit hook that rewrites the files it commits, as a formatter does,
// leaves them in the commit and the working tree in its version; git's
// index of them is set to the commit's, so git status reports nothing of
// them (bug 17), and a change staged before stays staged.
func TestCommitLeavesTheIndexAsCommitted(t *testing.T) {
	cfg := rollbackRepo(t)
	gitHook(t, "pre-commit", `#!/bin/sh
git diff --cached --name-only -- '*.yaml' | while IFS= read -r file; do
	echo '# formatted' >>"$file"
	git add -- "$file" || exit 1
done
`)
	if err := os.WriteFile("ledger.yaml", []byte("- id: T-1\n- id: T-3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, "add", "--", "ledger.yaml")
	sha, code, err, stderr := writeAndCommit(t, cfg, rollbackFiles(), "Add T-2.")
	if sha == "" || code != 0 || err != nil {
		t.Fatalf("got %q, exit %d, %v\n%s", sha, code, err, stderr)
	}
	if text := gitIn(t, "show", "HEAD:new.yaml"); text != "- id: T-2\n# formatted\n" {
		t.Errorf("the commit holds new.yaml as %q", text)
	}
	if status := gitIn(t, "status", "--porcelain", "--untracked-files=all"); status != "M  ledger.yaml\n" {
		t.Errorf("git status reports:\n%s", status)
	}
}

// The body of a commit itos makes of its own files never starts a line with
// what the header lint reads as a footer, nor with git's comment char, where
// a plain wrap at the same width would (bug 18): ask's "word:", as in docs:
// answer q-10, work done's run address after its "passed:", and a "#". The
// words are kept, in order, and no line is longer than the width.
func TestCommitBodyWrapsClearOfFooters(t *testing.T) {
	cfg := rollbackRepo(t)
	footer := regexp.MustCompile(`^[\w-]+(?::\s|\s+#)`)
	old := "items: []\n"
	for i, text := range []string{
		"recommendation: majors cost nothing under fix-forward.",
		"passed: https://github.com/donvargax/itos/actions/runs/37228229727.",
		"#12 is the issue it closes.",
	} {
		// The words before the text fill the first line to the width, so a
		// plain wrap starts the second with the text.
		lead := "Set slice-71 (\"itos ask record writes MADR 4 records\") to done, with itos work done. HEAD's CI run"
		lead += strings.Repeat(" x", (bodyWidth-len(lead))/2)
		if len(lead) < bodyWidth {
			lead += "s"
		}
		body := lead + " " + text
		if plain := value.Wrap(body, bodyWidth); !strings.Contains(plain, "\n"+text) {
			t.Fatalf("a plain wrap does not start a line with %q:\n%s", text, plain)
		}
		files := []written{{path: "registry.yaml", old: old, text: strings.Repeat("# edit\n", i+1) + old}}
		if sha, code, err, stderr := writeAndCommit(t, cfg, files, body); sha == "" || code != 0 || err != nil {
			t.Fatalf("got %q, exit %d, %v\n%s", sha, code, err, stderr)
		}
		old = files[0].text
		got := strings.TrimSpace(gitIn(t, "log", "-1", "--format=%b"))
		for _, line := range strings.Split(got, "\n") {
			if footer.MatchString(line) || strings.HasPrefix(line, "#") || len(line) > bodyWidth {
				t.Errorf("the body has the line %q:\n%s", line, got)
			}
		}
		if strings.Join(strings.Fields(got), " ") != body {
			t.Errorf("the body's words are not the given ones, in order:\n%s", got)
		}
	}
}
